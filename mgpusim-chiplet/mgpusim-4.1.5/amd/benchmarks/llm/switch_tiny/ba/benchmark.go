package switch_tiny

import (
	"fmt"
	"log"
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

func initMat(x []float32) {
	for i := range x {
		x[i] = (rand.Float32() - 0.5) * 0.02
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
	rand.Seed(1)
	b.hostX = make([]float32, b.SeqLen*b.DModel)
	b.hostOut = make([]float32, b.SeqLen*b.DModel)
	initMat(b.hostX)
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
			initMat(arr)
		}
		initGamma(h.G1, h.B1)
		initGamma(h.G2, h.B2)
		for e := 0; e < b.Experts; e++ {
			h.ExpertW1[e] = make([]float32, b.DModel*b.DFF)
			h.ExpertW2[e] = make([]float32, b.DFF*b.DModel)
			initMat(h.ExpertW1[e])
			initMat(h.ExpertW2[e])
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
		for ptr, arr := range map[driver.Ptr][]float32{b.dWq[l]: b.layersHost[l].Wq, b.dWk[l]: b.layersHost[l].Wk, b.dWv[l]: b.layersHost[l].Wv, b.dWo[l]: b.layersHost[l].Wo, b.dWrouter[l]: b.layersHost[l].Wrouter, b.dG1[l]: b.layersHost[l].G1, b.dB1[l]: b.layersHost[l].B1, b.dG2[l]: b.layersHost[l].G2, b.dB2[l]: b.layersHost[l].B2} {
			b.driver.MemCopyH2D(b.context, ptr, arr)
		}
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
	log.Printf("switch_tiny Verify() skipped in benchmark pack (execution-only benchmark)")
}
