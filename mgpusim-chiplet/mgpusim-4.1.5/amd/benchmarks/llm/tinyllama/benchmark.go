package tinyllama

import (
	"fmt"
	"log"
	"math"
	"math/rand"

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
type RopeArgs struct {
	X        driver.Ptr
	Cos      driver.Ptr
	Sin      driver.Ptr
	SeqLen   int32
	DModel   int32
	NumHeads int32
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
type RMSNormArgs struct {
	In     driver.Ptr
	Weight driver.Ptr
	Out    driver.Ptr
	SeqLen int32
	DModel int32
	Eps    float32
}
type SwiGLUArgs struct {
	Gate driver.Ptr
	Up   driver.Ptr
	Out  driver.Ptr
	N    int32
}

type layerHost struct {
	Wq, Wk, Wv, Wo    []float32
	WGate, WUp, WDown []float32
	Norm1, Norm2      []float32
}
type layerDev struct {
	dWq, dWk, dWv, dWo   driver.Ptr
	dWGate, dWUp, dWDown driver.Ptr
	dNorm1, dNorm2       driver.Ptr
}

type Benchmark struct {
	driver  *driver.Driver
	context *driver.Context
	gpus    []int

	kRMSNorm1                   *insts.HsaCo
	kQProj, kKProj, kVProj      *insts.HsaCo
	kRopeQ, kRopeK              *insts.HsaCo
	kScores, kSoftmax, kContext *insts.HsaCo
	kOProj                      *insts.HsaCo
	kAdd1                       *insts.HsaCo
	kRMSNorm2                   *insts.HsaCo
	kGate, kUp, kSwiGLU, kDown  *insts.HsaCo
	kAdd2                       *insts.HsaCo

	SeqLen   int
	DModel   int
	NumHeads int
	DFF      int
	Layers   int

	BatchTotal       int
	BatchStart       int
	BatchCount       int
	BatchSeed        int64
	BatchUpdateInput bool

	hostX      []float32
	hostOut    []float32
	layersHost []layerHost

	dX, dX2                             driver.Ptr
	dNorm, dQ, dK, dV                   driver.Ptr
	dRopeCos, dRopeSin                  driver.Ptr
	dScores, dProbs, dContext, dAttnOut driver.Ptr
	dGateBuf, dUpBuf, dFFBuf            driver.Ptr
	dWq, dWk, dWv, dWo                  []driver.Ptr
	dWGate, dWUp, dWDown                []driver.Ptr
	dNorm1W, dNorm2W                    []driver.Ptr

	useUnifiedMemory bool
}

func NewBenchmark(driver *driver.Driver) *Benchmark {
	b := new(Benchmark)
	b.driver = driver
	b.context = driver.Init()
	b.SeqLen = 256
	b.DModel = 128
	b.NumHeads = 8
	b.DFF = 352
	b.Layers = 4
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
		log.Panicf("failed to load %s", name)
	}
	return k
}
func (b *Benchmark) loadProgram() {
	b.kRMSNorm1 = mustLoadKernel(hsacoBytes, "tlm_rmsnorm1")
	b.kQProj = mustLoadKernel(hsacoBytes, "tlm_q_proj")
	b.kKProj = mustLoadKernel(hsacoBytes, "tlm_k_proj")
	b.kVProj = mustLoadKernel(hsacoBytes, "tlm_v_proj")
	b.kRopeQ = mustLoadKernel(hsacoBytes, "tlm_rope_q")
	b.kRopeK = mustLoadKernel(hsacoBytes, "tlm_rope_k")
	b.kScores = mustLoadKernel(hsacoBytes, "tlm_attention_scores_masked")
	b.kSoftmax = mustLoadKernel(hsacoBytes, "tlm_softmax")
	b.kContext = mustLoadKernel(hsacoBytes, "tlm_attention_context")
	b.kOProj = mustLoadKernel(hsacoBytes, "tlm_o_proj")
	b.kAdd1 = mustLoadKernel(hsacoBytes, "tlm_add_resid1")
	b.kRMSNorm2 = mustLoadKernel(hsacoBytes, "tlm_rmsnorm2")
	b.kGate = mustLoadKernel(hsacoBytes, "tlm_ff_gate")
	b.kUp = mustLoadKernel(hsacoBytes, "tlm_ff_up")
	b.kSwiGLU = mustLoadKernel(hsacoBytes, "tlm_swiglu")
	b.kDown = mustLoadKernel(hsacoBytes, "tlm_ff_down")
	b.kAdd2 = mustLoadKernel(hsacoBytes, "tlm_add_resid2")
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

func initMat(x []float32) {
	for i := range x {
		x[i] = (rand.Float32() - 0.5) * 0.02
	}
}
func initNorm(x []float32) {
	for i := range x {
		x[i] = 1.0
	}
}

func initRopeTables(cosTable, sinTable []float32, seqLen, dHead int) {
	half := dHead / 2
	for pos := 0; pos < seqLen; pos++ {
		for pair := 0; pair < half; pair++ {
			theta := float64(pos) / math.Pow(10000.0, float64(2*pair)/float64(dHead))
			idx := pos*half + pair
			cosTable[idx] = float32(math.Cos(theta))
			sinTable[idx] = float32(math.Sin(theta))
		}
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
	if (b.DModel/b.NumHeads)%2 != 0 {
		log.Panic("DHead must be even for RoPE")
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
	rand.Seed(1)
	seq, dm := b.SeqLen, b.DModel
	b.hostX = make([]float32, seq*dm)
	b.hostOut = make([]float32, seq*dm)
	initMat(b.hostX)
	b.layersHost = make([]layerHost, b.Layers)
	for l := 0; l < b.Layers; l++ {
		h := layerHost{Wq: make([]float32, dm*dm), Wk: make([]float32, dm*dm), Wv: make([]float32, dm*dm), Wo: make([]float32, dm*dm), WGate: make([]float32, dm*b.DFF), WUp: make([]float32, dm*b.DFF), WDown: make([]float32, b.DFF*dm), Norm1: make([]float32, dm), Norm2: make([]float32, dm)}
		initMat(h.Wq)
		initMat(h.Wk)
		initMat(h.Wv)
		initMat(h.Wo)
		initMat(h.WGate)
		initMat(h.WUp)
		initMat(h.WDown)
		initNorm(h.Norm1)
		initNorm(h.Norm2)
		b.layersHost[l] = h
	}
	b.dX = b.alloc(uint64(seq*dm*4), "tlm.X")
	b.dX2 = b.alloc(uint64(seq*dm*4), "tlm.X2")
	b.dNorm = b.alloc(uint64(seq*dm*4), "tlm.Norm")
	b.dQ = b.alloc(uint64(seq*dm*4), "tlm.Q")
	b.dK = b.alloc(uint64(seq*dm*4), "tlm.K")
	b.dV = b.alloc(uint64(seq*dm*4), "tlm.V")
	dHead := dm / b.NumHeads
	ropeHalf := dHead / 2
	ropeCos := make([]float32, seq*ropeHalf)
	ropeSin := make([]float32, seq*ropeHalf)
	initRopeTables(ropeCos, ropeSin, seq, dHead)
	b.dWq = make([]driver.Ptr, b.Layers)
	b.dWk = make([]driver.Ptr, b.Layers)
	b.dWv = make([]driver.Ptr, b.Layers)
	b.dWo = make([]driver.Ptr, b.Layers)
	b.dWGate = make([]driver.Ptr, b.Layers)
	b.dWUp = make([]driver.Ptr, b.Layers)
	b.dWDown = make([]driver.Ptr, b.Layers)
	b.dNorm1W = make([]driver.Ptr, b.Layers)
	b.dNorm2W = make([]driver.Ptr, b.Layers)
	b.dRopeCos = b.alloc(uint64(seq*ropeHalf*4), "tlm.RopeCos")
	b.dRopeSin = b.alloc(uint64(seq*ropeHalf*4), "tlm.RopeSin")
	b.dScores = b.alloc(uint64(b.NumHeads*seq*seq*4), "tlm.Scores")
	b.dProbs = b.alloc(uint64(b.NumHeads*seq*seq*4), "tlm.Probs")
	b.dContext = b.alloc(uint64(seq*dm*4), "tlm.Context")
	b.dAttnOut = b.alloc(uint64(seq*dm*4), "tlm.AttnOut")
	b.dGateBuf = b.alloc(uint64(seq*b.DFF*4), "tlm.Gate")
	b.dUpBuf = b.alloc(uint64(seq*b.DFF*4), "tlm.Up")
	b.dFFBuf = b.alloc(uint64(seq*b.DFF*4), "tlm.FF")
	b.driver.MemCopyH2D(b.context, b.dRopeCos, ropeCos)
	b.driver.MemCopyH2D(b.context, b.dRopeSin, ropeSin)
	wMatBytes := uint64(dm * dm * 4)
	wUpBytes := uint64(dm * b.DFF * 4)
	wDownBytes := uint64(b.DFF * dm * 4)
	normBytes := uint64(dm * 4)
	for l := 0; l < b.Layers; l++ {
		b.dWq[l] = b.alloc(wMatBytes, fmt.Sprintf("tlm.L%d.Wq", l))
		b.dWk[l] = b.alloc(wMatBytes, fmt.Sprintf("tlm.L%d.Wk", l))
		b.dWv[l] = b.alloc(wMatBytes, fmt.Sprintf("tlm.L%d.Wv", l))
		b.dWo[l] = b.alloc(wMatBytes, fmt.Sprintf("tlm.L%d.Wo", l))
		b.dWGate[l] = b.alloc(wUpBytes, fmt.Sprintf("tlm.L%d.WGate", l))
		b.dWUp[l] = b.alloc(wUpBytes, fmt.Sprintf("tlm.L%d.WUp", l))
		b.dWDown[l] = b.alloc(wDownBytes, fmt.Sprintf("tlm.L%d.WDown", l))
		b.dNorm1W[l] = b.alloc(normBytes, fmt.Sprintf("tlm.L%d.Norm1", l))
		b.dNorm2W[l] = b.alloc(normBytes, fmt.Sprintf("tlm.L%d.Norm2", l))
		b.driver.MemCopyH2D(b.context, b.dWq[l], b.layersHost[l].Wq)
		b.driver.MemCopyH2D(b.context, b.dWk[l], b.layersHost[l].Wk)
		b.driver.MemCopyH2D(b.context, b.dWv[l], b.layersHost[l].Wv)
		b.driver.MemCopyH2D(b.context, b.dWo[l], b.layersHost[l].Wo)
		b.driver.MemCopyH2D(b.context, b.dWGate[l], b.layersHost[l].WGate)
		b.driver.MemCopyH2D(b.context, b.dWUp[l], b.layersHost[l].WUp)
		b.driver.MemCopyH2D(b.context, b.dWDown[l], b.layersHost[l].WDown)
		b.driver.MemCopyH2D(b.context, b.dNorm1W[l], b.layersHost[l].Norm1)
		b.driver.MemCopyH2D(b.context, b.dNorm2W[l], b.layersHost[l].Norm2)
	}
	b.driver.MemCopyH2D(b.context, b.dX, b.hostX)
}

func (b *Benchmark) exec() {
	seq, dm, dh := b.SeqLen, b.DModel, b.DModel/b.NumHeads
	cur := b.dX
	other := b.dX2
	for l := 0; l < b.Layers; l++ {
		b.launch1D(b.kRMSNorm1, uint32(seq), 64, &RMSNormArgs{In: cur, Weight: b.dNorm1W[l], Out: b.dNorm, SeqLen: int32(seq), DModel: int32(dm), Eps: 1e-5})
		b.launch2D(b.kQProj, uint32(dm), uint32(seq), 8, 8, &MatMulArgs{A: b.dNorm, B: b.dWq[l], C: b.dQ, M: int32(seq), N: int32(dm), K: int32(dm)})
		b.launch2D(b.kKProj, uint32(dm), uint32(seq), 8, 8, &MatMulArgs{A: b.dNorm, B: b.dWk[l], C: b.dK, M: int32(seq), N: int32(dm), K: int32(dm)})
		b.launch2D(b.kVProj, uint32(dm), uint32(seq), 8, 8, &MatMulArgs{A: b.dNorm, B: b.dWv[l], C: b.dV, M: int32(seq), N: int32(dm), K: int32(dm)})
		b.launch2D(b.kRopeQ, uint32(dm), uint32(seq), 8, 8, &RopeArgs{X: b.dQ, Cos: b.dRopeCos, Sin: b.dRopeSin, SeqLen: int32(seq), DModel: int32(dm), NumHeads: int32(b.NumHeads)})
		b.launch2D(b.kRopeK, uint32(dm), uint32(seq), 8, 8, &RopeArgs{X: b.dK, Cos: b.dRopeCos, Sin: b.dRopeSin, SeqLen: int32(seq), DModel: int32(dm), NumHeads: int32(b.NumHeads)})
		b.launch2D(b.kScores, uint32(seq), uint32(b.NumHeads*seq), 8, 8, &AttnScoreArgs{Q: b.dQ, K: b.dK, Scores: b.dScores, SeqLen: int32(seq), DHead: int32(dh), NumHeads: int32(b.NumHeads)})
		b.launch2D(b.kSoftmax, uint32(seq), uint32(b.NumHeads), 8, 8, &SoftmaxArgs{In: b.dScores, Out: b.dProbs, SeqLen: int32(seq), NumHeads: int32(b.NumHeads)})
		b.launch2D(b.kContext, uint32(dm/b.NumHeads), uint32(b.NumHeads*seq), 8, 8, &AttnCtxArgs{Probs: b.dProbs, V: b.dV, Context: b.dContext, SeqLen: int32(seq), DHead: int32(dh), NumHeads: int32(b.NumHeads)})
		b.launch2D(b.kOProj, uint32(dm), uint32(seq), 8, 8, &MatMulArgs{A: b.dContext, B: b.dWo[l], C: b.dAttnOut, M: int32(seq), N: int32(dm), K: int32(dm)})
		b.launch1D(b.kAdd1, uint32(seq*dm), 64, &AddArgs{A: cur, B: b.dAttnOut, Out: other, N: int32(seq * dm)})
		cur, other = other, cur
		b.launch1D(b.kRMSNorm2, uint32(seq), 64, &RMSNormArgs{In: cur, Weight: b.dNorm2W[l], Out: b.dNorm, SeqLen: int32(seq), DModel: int32(dm), Eps: 1e-5})
		b.launch2D(b.kGate, uint32(b.DFF), uint32(seq), 8, 8, &MatMulArgs{A: b.dNorm, B: b.dWGate[l], C: b.dGateBuf, M: int32(seq), N: int32(b.DFF), K: int32(dm)})
		b.launch2D(b.kUp, uint32(b.DFF), uint32(seq), 8, 8, &MatMulArgs{A: b.dNorm, B: b.dWUp[l], C: b.dUpBuf, M: int32(seq), N: int32(b.DFF), K: int32(dm)})
		b.launch1D(b.kSwiGLU, uint32(seq*b.DFF), 64, &SwiGLUArgs{Gate: b.dGateBuf, Up: b.dUpBuf, Out: b.dFFBuf, N: int32(seq * b.DFF)})
		b.launch2D(b.kDown, uint32(dm), uint32(seq), 8, 8, &MatMulArgs{A: b.dFFBuf, B: b.dWDown[l], C: b.dAttnOut, M: int32(seq), N: int32(dm), K: int32(b.DFF)})
		b.launch1D(b.kAdd2, uint32(seq*dm), 64, &AddArgs{A: cur, B: b.dAttnOut, Out: other, N: int32(seq * dm)})
		cur, other = other, cur
	}
	b.driver.MemCopyD2H(b.context, b.hostOut, cur)
}

func (b *Benchmark) Verify() {
	ref := b.cpuForward()
	const atol = 2e-3
	if len(ref) != len(b.hostOut) {
		log.Panicf("tinyllama output length mismatch: ref=%d, got=%d", len(ref), len(b.hostOut))
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

// ===== CPU reference (for optional -verify) =====

func (b *Benchmark) cpuMatMul(a, w []float32, m, n, kdim int) []float32 {
	out := make([]float32, m*n)
	for row := 0; row < m; row++ {
		for col := 0; col < n; col++ {
			var sum float32
			aBase := row * kdim
			for k := 0; k < kdim; k++ {
				sum += a[aBase+k] * w[k*n+col]
			}
			out[row*n+col] = sum
		}
	}
	return out
}

func (b *Benchmark) cpuAdd(a, c []float32) []float32 {
	out := make([]float32, len(a))
	for i := range a {
		out[i] = a[i] + c[i]
	}
	return out
}

func (b *Benchmark) cpuRMSNorm(x, weight []float32, seq, dm int) []float32 {
	out := make([]float32, len(x))
	const eps = 1e-5
	for row := 0; row < seq; row++ {
		base := row * dm
		var mean2 float32
		for j := 0; j < dm; j++ {
			v := x[base+j]
			mean2 += v * v
		}
		mean2 *= float32(1.0) / float32(dm)
		inv := float32(1.0 / math.Sqrt(float64(mean2+eps)))
		for j := 0; j < dm; j++ {
			out[base+j] = x[base+j] * inv * weight[j]
		}
	}
	return out
}

func (b *Benchmark) cpuApplyRoPE(x []float32, cosTable, sinTable []float32, seq, dm, heads int) {
	dh := dm / heads
	halfDim := dh / 2
	for row := 0; row < seq; row++ {
		for col := 0; col < dm; col += 2 {
			inner := col % dh
			if inner&1 != 0 {
				continue
			}
			idx0 := row*dm + col
			idx1 := idx0 + 1
			if idx1 >= (row+1)*dm {
				continue
			}
			pair := inner >> 1
			ropeBase := row*halfDim + pair
			c := cosTable[ropeBase]
			s := sinTable[ropeBase]
			x0 := x[idx0]
			x1 := x[idx1]
			x[idx0] = x0*c + (-(x1 * s))
			x[idx1] = x0*s + x1*c
		}
	}
}

func cpuOpenCLExpF32(x float32) float32 {
	xHi := math.Float32frombits(math.Float32bits(x) & 0xfffff000)
	xLo := x - xHi

	cLo := math.Float32frombits(0x39a3b295)
	cHi := math.Float32frombits(0x3fb8a000)

	// Match the HSACO lowering used by clang for OpenCL exp(float):
	// ySmall = cLo*xLo; ySmall += cHi*xLo; ySmall += cLo*xHi;
	// yBig = cHi*xHi; exp(x)=exp2(yBig)*exp2(ySmall).
	ySmall := cLo * xLo
	ySmall += cHi * xLo
	ySmall += cLo * xHi
	yBig := cHi * xHi

	y := float32(math.Exp2(float64(yBig))) * float32(math.Exp2(float64(ySmall)))

	if !(x > math.Float32frombits(0xc2aeac50)) {
		y = 0
	}
	if !(x < math.Float32frombits(0x42b17218)) {
		y = float32(math.Inf(1))
	}
	return y
}

func (b *Benchmark) cpuMaskedAttention(q, k, v []float32, seq, dm, heads int) []float32 {
	dh := dm / heads
	probs := make([]float32, heads*seq*seq)
	context := make([]float32, seq*dm)
	scale := float32(1.0 / math.Sqrt(float64(dh)))

	for h := 0; h < heads; h++ {
		for i := 0; i < seq; i++ {
			rowStart := (h*seq + i) * seq
			maxv := float32(-1e30)
			for j := 0; j < seq; j++ {
				var score float32
				if j > i {
					score = -1.0e30
				} else {
					qBase := i*dm + h*dh
					kBase := j*dm + h*dh
					var dot float32
					for kk := 0; kk < dh; kk++ {
						dot += q[qBase+kk] * k[kBase+kk]
					}
					score = dot * scale
				}
				probs[rowStart+j] = score
				if score > maxv {
					maxv = score
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
			for kk := 0; kk < dh; kk++ {
				var sum float32
				pBase := (h*seq + i) * seq
				for j := 0; j < seq; j++ {
					sum += probs[pBase+j] * v[j*dm+h*dh+kk]
				}
				context[i*dm+h*dh+kk] = sum
			}
		}
	}
	return context
}

func (b *Benchmark) cpuSwiGLU(gate, up []float32) []float32 {
	out := make([]float32, len(gate))
	for i := range gate {
		g := gate[i]
		sig := float32(1.0) / (float32(1.0) + cpuOpenCLExpF32(-g))
		out[i] = (g * sig) * up[i]
	}
	return out
}

func (b *Benchmark) cpuForward() []float32 {
	seq, dm := b.SeqLen, b.DModel
	dh := dm / b.NumHeads
	ropeHalf := dh / 2
	ropeCos := make([]float32, seq*ropeHalf)
	ropeSin := make([]float32, seq*ropeHalf)
	initRopeTables(ropeCos, ropeSin, seq, dh)

	cur := make([]float32, len(b.hostX))
	copy(cur, b.hostX)

	for l := 0; l < b.Layers; l++ {
		h := b.layersHost[l]
		norm := b.cpuRMSNorm(cur, h.Norm1, seq, dm)
		q := b.cpuMatMul(norm, h.Wq, seq, dm, dm)
		k := b.cpuMatMul(norm, h.Wk, seq, dm, dm)
		v := b.cpuMatMul(norm, h.Wv, seq, dm, dm)
		b.cpuApplyRoPE(q, ropeCos, ropeSin, seq, dm, b.NumHeads)
		b.cpuApplyRoPE(k, ropeCos, ropeSin, seq, dm, b.NumHeads)
		context := b.cpuMaskedAttention(q, k, v, seq, dm, b.NumHeads)
		attnOut := b.cpuMatMul(context, h.Wo, seq, dm, dm)
		cur = b.cpuAdd(cur, attnOut)

		norm = b.cpuRMSNorm(cur, h.Norm2, seq, dm)
		gate := b.cpuMatMul(norm, h.WGate, seq, b.DFF, dm)
		up := b.cpuMatMul(norm, h.WUp, seq, b.DFF, dm)
		ff := b.cpuSwiGLU(gate, up)
		down := b.cpuMatMul(ff, h.WDown, seq, dm, b.DFF)
		cur = b.cpuAdd(cur, down)
	}
	return cur
}
