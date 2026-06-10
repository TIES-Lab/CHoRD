package switch_tiny

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"

	_ "embed"

	"github.com/sarchlab/mgpusim/v4/amd/driver"
	"github.com/sarchlab/mgpusim/v4/amd/insts"
	"github.com/sarchlab/mgpusim/v4/amd/kernels"
)

type MatMulArgs struct {
	A driver.Ptr
	B driver.Ptr
	C driver.Ptr
	M int32
	N int32
	K int32
}
type AttnScoreArgs struct {
	Q        driver.Ptr
	K        driver.Ptr
	Scores   driver.Ptr
	SeqLen   int32
	DHead    int32
	NumHeads int32
}
type SoftmaxArgs struct {
	In       driver.Ptr
	Out      driver.Ptr
	SeqLen   int32
	NumHeads int32
}
type AttnCtxArgs struct {
	Probs    driver.Ptr
	V        driver.Ptr
	Context  driver.Ptr
	SeqLen   int32
	DHead    int32
	NumHeads int32
}
type AddArgs struct {
	A   driver.Ptr
	B   driver.Ptr
	Out driver.Ptr
	N   int32
}
type LayerNormArgs struct {
	In     driver.Ptr
	Gamma  driver.Ptr
	Beta   driver.Ptr
	Out    driver.Ptr
	SeqLen int32
	DModel int32
	Eps    float32
}
type RouterArgs struct {
	In      driver.Ptr
	W       driver.Ptr
	Routes  driver.Ptr
	SeqLen  int32
	DModel  int32
	Experts int32
}
type ExpertFF1Args struct {
	In       driver.Ptr
	Routes   driver.Ptr
	W        driver.Ptr
	Out      driver.Ptr
	SeqLen   int32
	DModel   int32
	DFF      int32
	ExpertID int32
}
type ExpertFF2Args struct {
	In       driver.Ptr
	Routes   driver.Ptr
	W        driver.Ptr
	Out      driver.Ptr
	SeqLen   int32
	DFF      int32
	DModel   int32
	ExpertID int32
}

type layerHost struct {
	Wq, Wk, Wv, Wo, Wrouter, G1, B1, G2, B2 []float32
	ExpertW1                                [][]float32
	ExpertW2                                [][]float32
}
type Benchmark struct {
	driver  *driver.Driver
	context *driver.Context
	gpus    []int

	kQProj, kKProj, kVProj      *insts.HsaCo
	kScores, kSoftmax, kContext *insts.HsaCo
	kOProj                      *insts.HsaCo
	kAdd1, kAdd2                *insts.HsaCo
	kLN1, kLN2                  *insts.HsaCo
	kRouter                     *insts.HsaCo
	kExpertFF1, kExpertFF2      *insts.HsaCo

	SeqLen   int
	DModel   int
	NumHeads int
	DFF      int
	Layers   int
	Experts  int

	BatchTotal       int
	BatchStart       int
	BatchCount       int
	BatchSeed        int64
	BatchUpdateInput bool

	hostX      []float32
	hostOut    []float32
	layersHost []layerHost

	dX, dX2                            driver.Ptr
	dQ, dK, dV                         driver.Ptr
	dScores, dProbs, dContext, dTmpOut driver.Ptr
	dRoutes, dFF1                      driver.Ptr
	dWq, dWk, dWv, dWo                 []driver.Ptr
	dWrouter                           []driver.Ptr
	dG1, dB1, dG2, dB2                 []driver.Ptr
	dExpertW1, dExpertW2               []driver.Ptr
	useUnifiedMemory                   bool
}

func NewBenchmark(driver *driver.Driver) *Benchmark {
	b := new(Benchmark)
	b.driver = driver
	b.context = driver.Init()
	b.SeqLen = 256
	b.DModel = 128
	b.NumHeads = 8
	b.DFF = 256
	b.Layers = 2
	b.Experts = 4
	b.BatchTotal = 1
	b.BatchStart = 0
	b.BatchCount = 0
	b.BatchSeed = 1
	b.BatchUpdateInput = false
	b.loadProgram()
	return b
}
func (b *Benchmark) SelectGPU(gpus []int) { b.gpus = gpus }
func (b *Benchmark) SetUnifiedMemory()    { b.useUnifiedMemory = true }

//go:embed kernels.hsaco
var hsacoBytes []byte

func mustLoadKernel(bytes []byte, name string) *insts.HsaCo {
	k := kernels.LoadProgramFromMemory(bytes, name)
	if k == nil {
		log.Panicf("load %s", name)
	}
	return k
}
func (b *Benchmark) loadProgram() {
	b.kQProj = mustLoadKernel(hsacoBytes, "sw_q_proj")
	b.kKProj = mustLoadKernel(hsacoBytes, "sw_k_proj")
	b.kVProj = mustLoadKernel(hsacoBytes, "sw_v_proj")
	b.kScores = mustLoadKernel(hsacoBytes, "sw_attention_scores")
	b.kSoftmax = mustLoadKernel(hsacoBytes, "sw_softmax")
	b.kContext = mustLoadKernel(hsacoBytes, "sw_attention_context")
	b.kOProj = mustLoadKernel(hsacoBytes, "sw_o_proj")
	b.kAdd1 = mustLoadKernel(hsacoBytes, "sw_add_resid1")
	b.kAdd2 = mustLoadKernel(hsacoBytes, "sw_add_resid2")
	b.kLN1 = mustLoadKernel(hsacoBytes, "sw_layernorm1")
	b.kLN2 = mustLoadKernel(hsacoBytes, "sw_layernorm2")
	b.kRouter = mustLoadKernel(hsacoBytes, "sw_router")
	b.kExpertFF1 = mustLoadKernel(hsacoBytes, "sw_expert_ff1")
	b.kExpertFF2 = mustLoadKernel(hsacoBytes, "sw_expert_ff2")
}
func (b *Benchmark) alloc(bytes uint64, tag string) driver.Ptr {
	if b.useUnifiedMemory {
		return b.driver.AllocateUnifiedMemory(b.context, bytes)
	}
	return b.driver.AllocateMemoryWithTag(b.context, bytes, tag)
}
func roundUp(x, m uint32) uint32 {
	if m == 0 || x%m == 0 {
		return x
	}
	return x + (m - x%m)
}
func (b *Benchmark) launch1D(kernel *insts.HsaCo, total uint32, local uint16, args interface{}) {
	if total == 0 {
		return
	}
	b.driver.LaunchKernel(b.context, kernel, [3]uint32{roundUp(total, uint32(local)), 1, 1}, [3]uint16{local, 1, 1}, args)
}

func (b *Benchmark) launch2D(kernel *insts.HsaCo, globalX, globalY uint32, localX, localY uint16, args interface{}) {
	if globalX == 0 || globalY == 0 {
		return
	}
	if localX == 0 {
		localX = 1
	}
	if localY == 0 {
		localY = 1
	}
	localSize := [3]uint16{localX, localY, 1}
	globalSize := [3]uint32{roundUp(globalX, uint32(localX)), roundUp(globalY, uint32(localY)), 1}
	b.driver.LaunchKernel(b.context, kernel, globalSize, localSize, args)
}

func initMat(rng *rand.Rand, x []float32) {
	for i := range x {
		x[i] = (rng.Float32() - 0.5) * 0.02
	}
}
func initGamma(g, b []float32) {
	for i := range g {
		g[i] = 1.0
		b[i] = 0.0
	}
}
func (b *Benchmark) fillInputForBatch(batch int) {
	seed := b.BatchSeed + int64(batch)*1315423911
	rng := rand.New(rand.NewSource(seed))
	for i := range b.hostX {
		b.hostX[i] = (rng.Float32() - 0.5) * 0.02
	}
}

func (b *Benchmark) Run() {
	if b.DModel%b.NumHeads != 0 {
		log.Panic("DModel must be divisible by NumHeads")
	}
	for _, gpu := range b.gpus {
		b.driver.SelectGPU(b.context, gpu)
		b.driver.CreateCommandQueue(b.context)
	}
	b.initMem()
	start := b.BatchStart
	if start < 0 {
		start = 0
	}
	total := b.BatchTotal
	if total <= 0 {
		total = start + 1
	}
	count := b.BatchCount
	if count <= 0 {
		count = total - start
	}
	if count <= 0 {
		count = 1
	}
	end := start + count
	if end > total {
		end = total
	}
	if end <= start {
		end = start + 1
	}
	if b.BatchUpdateInput {
		b.fillInputForBatch(start)
		b.driver.MemCopyH2D(b.context, b.dX, b.hostX)
	}
	for bi := start; bi < end; bi++ {
		if b.BatchUpdateInput && bi != start {
			b.fillInputForBatch(bi)
			b.driver.MemCopyH2D(b.context, b.dX, b.hostX)
		}
		b.exec()
	}
}

func (b *Benchmark) initMem() {
	// Use a local PRNG instead of math/rand's package-level generator.
	// Go 1.24 makes top-level rand.Seed a no-op by default, so relying on
	// rand.Seed(1)+rand.Float32() makes benchmark weights differ across runs.
	rng := rand.New(rand.NewSource(1))
	b.hostX = make([]float32, b.SeqLen*b.DModel)
	b.hostOut = make([]float32, b.SeqLen*b.DModel)
	initMat(rng, b.hostX)
	b.layersHost = make([]layerHost, b.Layers)
	b.dWq = make([]driver.Ptr, b.Layers)
	b.dWk = make([]driver.Ptr, b.Layers)
	b.dWv = make([]driver.Ptr, b.Layers)
	b.dWo = make([]driver.Ptr, b.Layers)
	b.dWrouter = make([]driver.Ptr, b.Layers)
	b.dG1 = make([]driver.Ptr, b.Layers)
	b.dB1 = make([]driver.Ptr, b.Layers)
	b.dG2 = make([]driver.Ptr, b.Layers)
	b.dB2 = make([]driver.Ptr, b.Layers)
	b.dExpertW1 = make([]driver.Ptr, b.Layers*b.Experts)
	b.dExpertW2 = make([]driver.Ptr, b.Layers*b.Experts)
	for l := 0; l < b.Layers; l++ {
		h := layerHost{Wq: make([]float32, b.DModel*b.DModel), Wk: make([]float32, b.DModel*b.DModel), Wv: make([]float32, b.DModel*b.DModel), Wo: make([]float32, b.DModel*b.DModel), Wrouter: make([]float32, b.DModel*b.Experts), G1: make([]float32, b.DModel), B1: make([]float32, b.DModel), G2: make([]float32, b.DModel), B2: make([]float32, b.DModel), ExpertW1: make([][]float32, b.Experts), ExpertW2: make([][]float32, b.Experts)}
		for _, arr := range [][]float32{h.Wq, h.Wk, h.Wv, h.Wo, h.Wrouter} {
			initMat(rng, arr)
		}
		initGamma(h.G1, h.B1)
		initGamma(h.G2, h.B2)
		for e := 0; e < b.Experts; e++ {
			h.ExpertW1[e] = make([]float32, b.DModel*b.DFF)
			h.ExpertW2[e] = make([]float32, b.DFF*b.DModel)
			initMat(rng, h.ExpertW1[e])
			initMat(rng, h.ExpertW2[e])
		}
		b.layersHost[l] = h
	}
	dh := b.DModel / b.NumHeads
	_ = dh
	b.dX = b.alloc(uint64(b.SeqLen*b.DModel*4), "sw.X")
	b.dX2 = b.alloc(uint64(b.SeqLen*b.DModel*4), "sw.X2")
	b.dQ = b.alloc(uint64(b.SeqLen*b.DModel*4), "sw.Q")
	b.dK = b.alloc(uint64(b.SeqLen*b.DModel*4), "sw.K")
	b.dV = b.alloc(uint64(b.SeqLen*b.DModel*4), "sw.V")
	b.dScores = b.alloc(uint64(b.NumHeads*b.SeqLen*b.SeqLen*4), "sw.Scores")
	b.dProbs = b.alloc(uint64(b.NumHeads*b.SeqLen*b.SeqLen*4), "sw.Probs")
	b.dContext = b.alloc(uint64(b.SeqLen*b.DModel*4), "sw.Context")
	b.dTmpOut = b.alloc(uint64(b.SeqLen*b.DModel*4), "sw.TmpOut")
	b.dRoutes = b.alloc(uint64(b.SeqLen*4), "sw.Routes")
	b.dFF1 = b.alloc(uint64(b.SeqLen*b.DFF*4), "sw.FF1")
	b.driver.MemCopyH2D(b.context, b.dX, b.hostX)
	wMatBytes := uint64(b.DModel * b.DModel * 4)
	wRouterBytes := uint64(b.DModel * b.Experts * 4)
	normVecBytes := uint64(b.DModel * 4)
	expertW1Bytes := uint64(b.DModel * b.DFF * 4)
	expertW2Bytes := uint64(b.DFF * b.DModel * 4)
	for l := 0; l < b.Layers; l++ {
		b.dWq[l] = b.alloc(wMatBytes, fmt.Sprintf("sw.L%d.Wq", l))
		b.dWk[l] = b.alloc(wMatBytes, fmt.Sprintf("sw.L%d.Wk", l))
		b.dWv[l] = b.alloc(wMatBytes, fmt.Sprintf("sw.L%d.Wv", l))
		b.dWo[l] = b.alloc(wMatBytes, fmt.Sprintf("sw.L%d.Wo", l))
		b.dWrouter[l] = b.alloc(wRouterBytes, fmt.Sprintf("sw.L%d.Wrouter", l))
		b.dG1[l] = b.alloc(normVecBytes, fmt.Sprintf("sw.L%d.G1", l))
		b.dB1[l] = b.alloc(normVecBytes, fmt.Sprintf("sw.L%d.B1", l))
		b.dG2[l] = b.alloc(normVecBytes, fmt.Sprintf("sw.L%d.G2", l))
		b.dB2[l] = b.alloc(normVecBytes, fmt.Sprintf("sw.L%d.B2", l))
		for e := 0; e < b.Experts; e++ {
			idx := l*b.Experts + e
			b.dExpertW1[idx] = b.alloc(expertW1Bytes, fmt.Sprintf("sw.L%d.E%d.W1", l, e))
			b.dExpertW2[idx] = b.alloc(expertW2Bytes, fmt.Sprintf("sw.L%d.E%d.W2", l, e))
			b.driver.MemCopyH2D(b.context, b.dExpertW1[idx], b.layersHost[l].ExpertW1[e])
			b.driver.MemCopyH2D(b.context, b.dExpertW2[idx], b.layersHost[l].ExpertW2[e])
		}
		// Keep H2D initialization order deterministic. Go map iteration order is
		// intentionally randomized and can otherwise obscure replay diagnostics.
		b.driver.MemCopyH2D(b.context, b.dWq[l], b.layersHost[l].Wq)
		b.driver.MemCopyH2D(b.context, b.dWk[l], b.layersHost[l].Wk)
		b.driver.MemCopyH2D(b.context, b.dWv[l], b.layersHost[l].Wv)
		b.driver.MemCopyH2D(b.context, b.dWo[l], b.layersHost[l].Wo)
		b.driver.MemCopyH2D(b.context, b.dWrouter[l], b.layersHost[l].Wrouter)
		b.driver.MemCopyH2D(b.context, b.dG1[l], b.layersHost[l].G1)
		b.driver.MemCopyH2D(b.context, b.dB1[l], b.layersHost[l].B1)
		b.driver.MemCopyH2D(b.context, b.dG2[l], b.layersHost[l].G2)
		b.driver.MemCopyH2D(b.context, b.dB2[l], b.layersHost[l].B2)
	}
}

func (b *Benchmark) exec() {
	dh := b.DModel / b.NumHeads
	cur, other := b.dX, b.dX2
	for l := 0; l < b.Layers; l++ {
		b.launch1D(b.kLN1, uint32(b.SeqLen), 64, &LayerNormArgs{In: cur, Gamma: b.dG1[l], Beta: b.dB1[l], Out: other, SeqLen: int32(b.SeqLen), DModel: int32(b.DModel), Eps: 1e-5})
		b.launch2D(b.kQProj, uint32(b.DModel), uint32(b.SeqLen), 8, 8, &MatMulArgs{A: other, B: b.dWq[l], C: b.dQ, M: int32(b.SeqLen), N: int32(b.DModel), K: int32(b.DModel)})
		b.launch2D(b.kKProj, uint32(b.DModel), uint32(b.SeqLen), 8, 8, &MatMulArgs{A: other, B: b.dWk[l], C: b.dK, M: int32(b.SeqLen), N: int32(b.DModel), K: int32(b.DModel)})
		b.launch2D(b.kVProj, uint32(b.DModel), uint32(b.SeqLen), 8, 8, &MatMulArgs{A: other, B: b.dWv[l], C: b.dV, M: int32(b.SeqLen), N: int32(b.DModel), K: int32(b.DModel)})
		b.launch2D(b.kScores, uint32(b.SeqLen), uint32(b.NumHeads*b.SeqLen), 8, 8, &AttnScoreArgs{Q: b.dQ, K: b.dK, Scores: b.dScores, SeqLen: int32(b.SeqLen), DHead: int32(dh), NumHeads: int32(b.NumHeads)})
		b.launch2D(b.kSoftmax, uint32(b.SeqLen), uint32(b.NumHeads), 8, 8, &SoftmaxArgs{In: b.dScores, Out: b.dProbs, SeqLen: int32(b.SeqLen), NumHeads: int32(b.NumHeads)})
		b.launch2D(b.kContext, uint32(b.DModel/b.NumHeads), uint32(b.NumHeads*b.SeqLen), 8, 8, &AttnCtxArgs{Probs: b.dProbs, V: b.dV, Context: b.dContext, SeqLen: int32(b.SeqLen), DHead: int32(dh), NumHeads: int32(b.NumHeads)})
		b.launch2D(b.kOProj, uint32(b.DModel), uint32(b.SeqLen), 8, 8, &MatMulArgs{A: b.dContext, B: b.dWo[l], C: b.dTmpOut, M: int32(b.SeqLen), N: int32(b.DModel), K: int32(b.DModel)})
		b.launch1D(b.kAdd1, uint32(b.SeqLen*b.DModel), 64, &AddArgs{A: cur, B: b.dTmpOut, Out: other, N: int32(b.SeqLen * b.DModel)})
		cur, other = other, cur
		b.launch1D(b.kLN2, uint32(b.SeqLen), 64, &LayerNormArgs{In: cur, Gamma: b.dG2[l], Beta: b.dB2[l], Out: other, SeqLen: int32(b.SeqLen), DModel: int32(b.DModel), Eps: 1e-5})
		b.launch1D(b.kRouter, uint32(b.SeqLen), 64, &RouterArgs{In: other, W: b.dWrouter[l], Routes: b.dRoutes, SeqLen: int32(b.SeqLen), DModel: int32(b.DModel), Experts: int32(b.Experts)})
		for e := 0; e < b.Experts; e++ {
			idx := l*b.Experts + e
			b.launch2D(b.kExpertFF1, uint32(b.DFF), uint32(b.SeqLen), 8, 8, &ExpertFF1Args{In: other, Routes: b.dRoutes, W: b.dExpertW1[idx], Out: b.dFF1, SeqLen: int32(b.SeqLen), DModel: int32(b.DModel), DFF: int32(b.DFF), ExpertID: int32(e)})
			b.launch2D(b.kExpertFF2, uint32(b.DModel), uint32(b.SeqLen), 8, 8, &ExpertFF2Args{In: b.dFF1, Routes: b.dRoutes, W: b.dExpertW2[idx], Out: b.dTmpOut, SeqLen: int32(b.SeqLen), DFF: int32(b.DFF), DModel: int32(b.DModel), ExpertID: int32(e)})
		}
		b.launch1D(b.kAdd2, uint32(b.SeqLen*b.DModel), 64, &AddArgs{A: cur, B: b.dTmpOut, Out: other, N: int32(b.SeqLen * b.DModel)})
		cur, other = other, cur
	}
	b.driver.MemCopyD2H(b.context, b.hostOut, cur)
}

func (b *Benchmark) Verify() {
	if path := os.Getenv("SWITCH_TINY_DUMP_OUTPUT"); path != "" {
		dumpFloat32File(path, b.hostOut)
		log.Printf("switch_tiny output dumped to %s (%d floats)\n", path, len(b.hostOut))
		return
	}

	if path := os.Getenv("SWITCH_TINY_GOLDEN_OUTPUT"); path != "" {
		golden := loadFloat32File(path)
		verifyFloat32Golden(path, golden, b.hostOut)
		log.Printf("Passed golden-output verify against %s\n", path)
		return
	}

	ref := b.cpuForwardOutput()
	const atol = 2e-3
	if len(ref) != len(b.hostOut) {
		log.Panicf("output length mismatch: ref=%d, got=%d", len(ref), len(b.hostOut))
	}
	for i := range ref {
		diff := math.Abs(float64(ref[i] - b.hostOut[i]))
		if diff > atol {
			log.Panicf("Mismatch at output %d, expected %.6f, got %.6f (diff=%.6f)",
				i, ref[i], b.hostOut[i], diff)
		}
	}
	log.Printf("Passed!\n")
}

func dumpFloat32File(path string, values []float32) {
	f, err := os.Create(path)
	if err != nil {
		log.Panicf("failed to create %s: %v", path, err)
	}
	defer f.Close()
	buf := make([]byte, 4*len(values))
	for i, v := range values {
		binary.LittleEndian.PutUint32(buf[i*4:(i+1)*4], math.Float32bits(v))
	}
	if _, err := f.Write(buf); err != nil {
		log.Panicf("failed to write %s: %v", path, err)
	}
}

func loadFloat32File(path string) []float32 {
	buf, err := os.ReadFile(path)
	if err != nil {
		log.Panicf("failed to read %s: %v", path, err)
	}
	if len(buf)%4 != 0 {
		log.Panicf("bad golden file %s: byte length %d is not divisible by 4", path, len(buf))
	}
	values := make([]float32, len(buf)/4)
	for i := range values {
		values[i] = math.Float32frombits(binary.LittleEndian.Uint32(buf[i*4 : (i+1)*4]))
	}
	return values
}

func verifyFloat32Golden(path string, golden []float32, got []float32) {
	if len(golden) != len(got) {
		log.Panicf("golden length mismatch for %s: golden=%d, got=%d", path, len(golden), len(got))
	}
	for i := range golden {
		if math.Float32bits(golden[i]) != math.Float32bits(got[i]) {
			diff := math.Abs(float64(golden[i] - got[i]))
			log.Panicf("Golden mismatch at output %d, expected %.9g, got %.9g (diff=%.9g, expected_bits=0x%08x, got_bits=0x%08x)",
				i, golden[i], got[i], diff, math.Float32bits(golden[i]), math.Float32bits(got[i]))
		}
	}
}

func (b *Benchmark) cpuMatMul(A []float32, B []float32, M, N, K int) []float32 {
	C := make([]float32, M*N)
	for i := 0; i < M; i++ {
		ai := i * K
		ci := i * N
		for j := 0; j < N; j++ {
			var sum float32
			for k := 0; k < K; k++ {
				sum += A[ai+k] * B[k*N+j]
			}
			C[ci+j] = sum
		}
	}
	return C
}

func (b *Benchmark) cpuAdd(a, c []float32) []float32 {
	out := make([]float32, len(a))
	for i := range a {
		out[i] = a[i] + c[i]
	}
	return out
}

func (b *Benchmark) cpuLayerNorm(x []float32, gamma []float32, beta []float32, seq, dm int) []float32 {
	out := make([]float32, len(x))
	const eps = 1e-5
	for i := 0; i < seq; i++ {
		row := x[i*dm : (i+1)*dm]
		var mean float32
		for j := 0; j < dm; j++ {
			mean += row[j]
		}
		mean /= float32(dm)
		var varsum float32
		for j := 0; j < dm; j++ {
			d := row[j] - mean
			varsum += d * d
		}
		varsum /= float32(dm)
		invStd := float32(1.0 / math.Sqrt(float64(varsum+eps)))
		for j := 0; j < dm; j++ {
			n := (row[j] - mean) * invStd
			out[i*dm+j] = n*gamma[j] + beta[j]
		}
	}
	return out
}

func cpuOpenCLExpF32(x float32) float32 {
	// Match the gfx803 V_EXP_F32 lowering observed in kernels.hsaco.linked.s:
	// x is split into xHi/xLo and log2(e) is split into cHi/cLo. The two
	// partial products are accumulated in the same order as the HSACO sequence.
	xHi := math.Float32frombits(math.Float32bits(x) & 0xfffff000)
	xLo := x - xHi
	cLo := math.Float32frombits(0x39a3b295) // 0.00031222837
	cHi := math.Float32frombits(0x3fb8a000) // 1.4423828125

	ySmall := cLo * xLo
	ySmall += cHi * xLo
	yBig := cHi * xHi
	ySmall += cLo * xHi

	y := float32(math.Exp2(float64(yBig))) * float32(math.Exp2(float64(ySmall)))
	if !(x > math.Float32frombits(0xc2aeac50)) { // -87.33655
		y = 0
	}
	if !(x < math.Float32frombits(0x42b17218)) { // 88.72284
		y = float32(math.Inf(1))
	}
	return y
}

func (b *Benchmark) cpuAttention(Q []float32, K []float32, V []float32, seq, dm, heads int) []float32 {
	dh := dm / heads
	probs := make([]float32, heads*seq*seq)
	context := make([]float32, seq*dm)
	scale := float32(1.0 / math.Sqrt(float64(dh)))

	for h := 0; h < heads; h++ {
		for i := 0; i < seq; i++ {
			rowStart := (h*seq + i) * seq
			maxv := float32(-1e30)
			for j := 0; j < seq; j++ {
				var dot float32
				qBase := i*dm + h*dh
				kBase := j*dm + h*dh
				for k := 0; k < dh; k++ {
					dot += Q[qBase+k] * K[kBase+k]
				}
				v := dot * scale
				probs[rowStart+j] = v
				if v > maxv {
					maxv = v
				}
			}
			var sumExp float32
			for j := 0; j < seq; j++ {
				e := cpuOpenCLExpF32(probs[rowStart+j] - maxv)
				probs[rowStart+j] = e
				sumExp += e
			}
			inv := float32(1.0) / sumExp
			for j := 0; j < seq; j++ {
				probs[rowStart+j] *= inv
			}
		}
	}

	for i := 0; i < seq; i++ {
		for h := 0; h < heads; h++ {
			for k := 0; k < dh; k++ {
				var sum float32
				pBase := (h*seq + i) * seq
				for j := 0; j < seq; j++ {
					sum += probs[pBase+j] * V[j*dm+h*dh+k]
				}
				context[i*dm+h*dh+k] = sum
			}
		}
	}
	return context
}

func (b *Benchmark) cpuRouter(x []float32, W []float32, seq, dm, experts int) []int {
	routes := make([]int, seq)
	for row := 0; row < seq; row++ {
		base := row * dm
		best := 0
		bestv := float32(-1e30)
		for e := 0; e < experts; e++ {
			var sum float32
			for k := 0; k < dm; k++ {
				sum += x[base+k] * W[k*experts+e]
			}
			if sum > bestv {
				bestv = sum
				best = e
			}
		}
		routes[row] = best
	}
	return routes
}

func (b *Benchmark) cpuExpertMLP(x []float32, routes []int, w1 [][]float32, w2 [][]float32, seq, dm, dff int) []float32 {
	out := make([]float32, seq*dm)
	ff1 := make([]float32, dff)
	for row := 0; row < seq; row++ {
		e := routes[row]
		abase := row * dm
		for col := 0; col < dff; col++ {
			var sum float32
			for k := 0; k < dm; k++ {
				sum += x[abase+k] * w1[e][k*dff+col]
			}
			if sum < 0 {
				sum = 0
			}
			ff1[col] = sum
		}
		obase := row * dm
		for col := 0; col < dm; col++ {
			var sum float32
			for k := 0; k < dff; k++ {
				sum += ff1[k] * w2[e][k*dm+col]
			}
			out[obase+col] = sum
		}
	}
	return out
}

func (b *Benchmark) cpuForwardOutput() []float32 {
	seq := b.SeqLen
	dm := b.DModel
	heads := b.NumHeads
	dff := b.DFF
	cur := make([]float32, len(b.hostX))
	copy(cur, b.hostX)

	for l := 0; l < b.Layers; l++ {
		h := b.layersHost[l]
		xn1 := b.cpuLayerNorm(cur, h.G1, h.B1, seq, dm)
		Q := b.cpuMatMul(xn1, h.Wq, seq, dm, dm)
		K := b.cpuMatMul(xn1, h.Wk, seq, dm, dm)
		V := b.cpuMatMul(xn1, h.Wv, seq, dm, dm)
		C := b.cpuAttention(Q, K, V, seq, dm, heads)
		O := b.cpuMatMul(C, h.Wo, seq, dm, dm)
		Y1 := b.cpuAdd(cur, O)
		xn2 := b.cpuLayerNorm(Y1, h.G2, h.B2, seq, dm)
		routes := b.cpuRouter(xn2, h.Wrouter, seq, dm, b.Experts)
		FF2 := b.cpuExpertMLP(xn2, routes, h.ExpertW1, h.ExpertW2, seq, dm, dff)
		cur = b.cpuAdd(Y1, FF2)
	}
	return cur
}
