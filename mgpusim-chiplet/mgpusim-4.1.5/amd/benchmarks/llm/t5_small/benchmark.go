package t5_small

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
type AttnScoreArgs struct {
	Q        driver.Ptr
	K        driver.Ptr
	Scores   driver.Ptr
	QLen     int32
	KLen     int32
	DHead    int32
	NumHeads int32
}
type SoftmaxArgs struct {
	In       driver.Ptr
	Out      driver.Ptr
	QLen     int32
	KLen     int32
	NumHeads int32
}
type AttnCtxArgs struct {
	Probs    driver.Ptr
	V        driver.Ptr
	Context  driver.Ptr
	QLen     int32
	KLen     int32
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
type ReLUArgs struct {
	X driver.Ptr
	N int32
}

type encLayerHost struct{ Wq, Wk, Wv, Wo, W1, W2, G1, B1, G2, B2 []float32 }
type encLayerDev struct{ dWq, dWk, dWv, dWo, dW1, dW2, dG1, dB1, dG2, dB2 driver.Ptr }
type decLayerHost struct{ WSq, WSk, WSv, SWo, WCq, WCk, WCv, WCo, W1, W2, G1, B1, Gx, Bx, G2, B2 []float32 }
type decLayerDev struct{ dWSq, dWSk, dWSv, dSWo, dWCq, dWCk, dWCv, dWCo, dW1, dW2, dG1, dB1, dGx, dBx, dG2, dB2 driver.Ptr }

type Benchmark struct {
	driver  *driver.Driver
	context *driver.Context
	gpus    []int

	kQProj, kKProj, kVProj *insts.HsaCo
	kScores, kScoresMasked *insts.HsaCo
	kSoftmax, kContext     *insts.HsaCo
	kOProj                 *insts.HsaCo
	kAdd1, kAddX, kAdd2    *insts.HsaCo
	kLN1, kLNX, kLN2       *insts.HsaCo
	kFF1, kReLU, kFF2      *insts.HsaCo
	kLMHead                *insts.HsaCo

	EncSeq   int
	DecSeq   int
	DModel   int
	NumHeads int
	DFF      int
	Layers   int
	Vocab    int

	BatchTotal       int
	BatchStart       int
	BatchCount       int
	BatchSeed        int64
	BatchUpdateInput bool

	hostEnc    []float32
	hostDec    []float32
	hostLogits []float32
	encHost    []encLayerHost
	decHost    []decLayerHost
	hWlm       []float32

	dEncX, dEncX2, dDecX, dDecX2       driver.Ptr
	dQ, dK, dV                         driver.Ptr
	dScores, dProbs, dContext, dTmpOut driver.Ptr
	dFF1, dFF2                         driver.Ptr
	dLogits, dWlm                      driver.Ptr
	encDev                             []encLayerDev
	decDev                             []decLayerDev

	useUnifiedMemory bool
}

func NewBenchmark(driver *driver.Driver) *Benchmark {
	b := new(Benchmark)
	b.driver = driver
	b.context = driver.Init()
	b.EncSeq = 128
	b.DecSeq = 32
	b.DModel = 128
	b.NumHeads = 8
	b.DFF = 256
	b.Layers = 2
	b.Vocab = 64
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
	b.kQProj = mustLoadKernel(hsacoBytes, "t5_q_proj")
	b.kKProj = mustLoadKernel(hsacoBytes, "t5_k_proj")
	b.kVProj = mustLoadKernel(hsacoBytes, "t5_v_proj")
	b.kScores = mustLoadKernel(hsacoBytes, "t5_attention_scores")
	b.kScoresMasked = mustLoadKernel(hsacoBytes, "t5_attention_scores_masked")
	b.kSoftmax = mustLoadKernel(hsacoBytes, "t5_softmax")
	b.kContext = mustLoadKernel(hsacoBytes, "t5_attention_context")
	b.kOProj = mustLoadKernel(hsacoBytes, "t5_o_proj")
	b.kAdd1 = mustLoadKernel(hsacoBytes, "t5_add_resid1")
	b.kAddX = mustLoadKernel(hsacoBytes, "t5_add_resid_x")
	b.kAdd2 = mustLoadKernel(hsacoBytes, "t5_add_resid2")
	b.kLN1 = mustLoadKernel(hsacoBytes, "t5_layernorm1")
	b.kLNX = mustLoadKernel(hsacoBytes, "t5_layernorm_x")
	b.kLN2 = mustLoadKernel(hsacoBytes, "t5_layernorm2")
	b.kFF1 = mustLoadKernel(hsacoBytes, "t5_ff1")
	b.kReLU = mustLoadKernel(hsacoBytes, "t5_relu")
	b.kFF2 = mustLoadKernel(hsacoBytes, "t5_ff2")
	b.kLMHead = mustLoadKernel(hsacoBytes, "t5_lm_head")
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
func initMatWithRNG(rng *rand.Rand, x []float32) {
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
	for i := range b.hostEnc {
		b.hostEnc[i] = (rng.Float32() - 0.5) * 0.02
	}
	for i := range b.hostDec {
		b.hostDec[i] = (rng.Float32() - 0.5) * 0.02
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
		b.driver.MemCopyH2D(b.context, b.dEncX, b.hostEnc)
		b.driver.MemCopyH2D(b.context, b.dDecX, b.hostDec)
	}
	for bi := start; bi < end; bi++ {
		if b.BatchUpdateInput && bi != start {
			b.fillInputForBatch(bi)
			b.driver.MemCopyH2D(b.context, b.dEncX, b.hostEnc)
			b.driver.MemCopyH2D(b.context, b.dDecX, b.hostDec)
		}
		b.exec()
	}
}

func (b *Benchmark) initMem() {
	// Use a local PRNG instead of math/rand's package-level generator.
	// Go 1.24 makes top-level rand.Seed a no-op by default, which can
	// otherwise make benchmark weights differ across runs.
	rng := rand.New(rand.NewSource(1))
	b.hostEnc = make([]float32, b.EncSeq*b.DModel)
	b.hostDec = make([]float32, b.DecSeq*b.DModel)
	b.hostLogits = make([]float32, b.DecSeq*b.Vocab)
	initMatWithRNG(rng, b.hostEnc)
	initMatWithRNG(rng, b.hostDec)
	b.hWlm = make([]float32, b.DModel*b.Vocab)
	initMatWithRNG(rng, b.hWlm)
	b.encHost = make([]encLayerHost, b.Layers)
	b.encDev = make([]encLayerDev, b.Layers)
	for l := 0; l < b.Layers; l++ {
		h := encLayerHost{Wq: make([]float32, b.DModel*b.DModel), Wk: make([]float32, b.DModel*b.DModel), Wv: make([]float32, b.DModel*b.DModel), Wo: make([]float32, b.DModel*b.DModel), W1: make([]float32, b.DModel*b.DFF), W2: make([]float32, b.DFF*b.DModel), G1: make([]float32, b.DModel), B1: make([]float32, b.DModel), G2: make([]float32, b.DModel), B2: make([]float32, b.DModel)}
		initMatWithRNG(rng, h.Wq)
		initMatWithRNG(rng, h.Wk)
		initMatWithRNG(rng, h.Wv)
		initMatWithRNG(rng, h.Wo)
		initMatWithRNG(rng, h.W1)
		initMatWithRNG(rng, h.W2)
		initGamma(h.G1, h.B1)
		initGamma(h.G2, h.B2)
		b.encHost[l] = h
	}
	b.decHost = make([]decLayerHost, b.Layers)
	b.decDev = make([]decLayerDev, b.Layers)
	for l := 0; l < b.Layers; l++ {
		h := decLayerHost{WSq: make([]float32, b.DModel*b.DModel), WSk: make([]float32, b.DModel*b.DModel), WSv: make([]float32, b.DModel*b.DModel), SWo: make([]float32, b.DModel*b.DModel), WCq: make([]float32, b.DModel*b.DModel), WCk: make([]float32, b.DModel*b.DModel), WCv: make([]float32, b.DModel*b.DModel), WCo: make([]float32, b.DModel*b.DModel), W1: make([]float32, b.DModel*b.DFF), W2: make([]float32, b.DFF*b.DModel), G1: make([]float32, b.DModel), B1: make([]float32, b.DModel), Gx: make([]float32, b.DModel), Bx: make([]float32, b.DModel), G2: make([]float32, b.DModel), B2: make([]float32, b.DModel)}
		for _, arr := range [][]float32{h.WSq, h.WSk, h.WSv, h.SWo, h.WCq, h.WCk, h.WCv, h.WCo, h.W1, h.W2} {
			initMatWithRNG(rng, arr)
		}
		initGamma(h.G1, h.B1)
		initGamma(h.Gx, h.Bx)
		initGamma(h.G2, h.B2)
		b.decHost[l] = h
	}
	maxSeq := b.EncSeq
	if b.DecSeq > maxSeq {
		maxSeq = b.DecSeq
	}
	b.dEncX = b.alloc(uint64(b.EncSeq*b.DModel*4), "t5.EncX")
	b.dEncX2 = b.alloc(uint64(b.EncSeq*b.DModel*4), "t5.EncX2")
	b.dDecX = b.alloc(uint64(b.DecSeq*b.DModel*4), "t5.DecX")
	b.dDecX2 = b.alloc(uint64(b.DecSeq*b.DModel*4), "t5.DecX2")
	b.dQ = b.alloc(uint64(maxSeq*b.DModel*4), "t5.Q")
	b.dK = b.alloc(uint64(maxSeq*b.DModel*4), "t5.K")
	b.dV = b.alloc(uint64(maxSeq*b.DModel*4), "t5.V")
	b.dScores = b.alloc(uint64(b.NumHeads*maxSeq*maxSeq*4), "t5.Scores")
	b.dProbs = b.alloc(uint64(b.NumHeads*maxSeq*maxSeq*4), "t5.Probs")
	b.dContext = b.alloc(uint64(maxSeq*b.DModel*4), "t5.Context")
	b.dTmpOut = b.alloc(uint64(maxSeq*b.DModel*4), "t5.TmpOut")
	b.dFF1 = b.alloc(uint64(maxSeq*b.DFF*4), "t5.FF1")
	b.dFF2 = b.alloc(uint64(maxSeq*b.DModel*4), "t5.FF2")
	b.dWlm = b.alloc(uint64(len(b.hWlm)*4), "t5.LMHead.W")
	b.dLogits = b.alloc(uint64(len(b.hostLogits)*4), "t5.Logits")
	b.driver.MemCopyH2D(b.context, b.dEncX, b.hostEnc)
	b.driver.MemCopyH2D(b.context, b.dDecX, b.hostDec)
	b.driver.MemCopyH2D(b.context, b.dWlm, b.hWlm)
	for l := 0; l < b.Layers; l++ {
		d := encLayerDev{}
		d.dWq = b.alloc(uint64(len(b.encHost[l].Wq)*4), fmt.Sprintf("t5.E%d.Wq", l))
		d.dWk = b.alloc(uint64(len(b.encHost[l].Wk)*4), fmt.Sprintf("t5.E%d.Wk", l))
		d.dWv = b.alloc(uint64(len(b.encHost[l].Wv)*4), fmt.Sprintf("t5.E%d.Wv", l))
		d.dWo = b.alloc(uint64(len(b.encHost[l].Wo)*4), fmt.Sprintf("t5.E%d.Wo", l))
		d.dW1 = b.alloc(uint64(len(b.encHost[l].W1)*4), fmt.Sprintf("t5.E%d.W1", l))
		d.dW2 = b.alloc(uint64(len(b.encHost[l].W2)*4), fmt.Sprintf("t5.E%d.W2", l))
		d.dG1 = b.alloc(uint64(len(b.encHost[l].G1)*4), fmt.Sprintf("t5.E%d.G1", l))
		d.dB1 = b.alloc(uint64(len(b.encHost[l].B1)*4), fmt.Sprintf("t5.E%d.B1", l))
		d.dG2 = b.alloc(uint64(len(b.encHost[l].G2)*4), fmt.Sprintf("t5.E%d.G2", l))
		d.dB2 = b.alloc(uint64(len(b.encHost[l].B2)*4), fmt.Sprintf("t5.E%d.B2", l))
		b.encDev[l] = d
		for ptr, arr := range map[driver.Ptr][]float32{d.dWq: b.encHost[l].Wq, d.dWk: b.encHost[l].Wk, d.dWv: b.encHost[l].Wv, d.dWo: b.encHost[l].Wo, d.dW1: b.encHost[l].W1, d.dW2: b.encHost[l].W2, d.dG1: b.encHost[l].G1, d.dB1: b.encHost[l].B1, d.dG2: b.encHost[l].G2, d.dB2: b.encHost[l].B2} {
			b.driver.MemCopyH2D(b.context, ptr, arr)
		}
	}
	for l := 0; l < b.Layers; l++ {
		d := decLayerDev{}
		d.dWSq = b.alloc(uint64(len(b.decHost[l].WSq)*4), fmt.Sprintf("t5.D%d.WSq", l))
		d.dWSk = b.alloc(uint64(len(b.decHost[l].WSk)*4), fmt.Sprintf("t5.D%d.WSk", l))
		d.dWSv = b.alloc(uint64(len(b.decHost[l].WSv)*4), fmt.Sprintf("t5.D%d.WSv", l))
		d.dSWo = b.alloc(uint64(len(b.decHost[l].SWo)*4), fmt.Sprintf("t5.D%d.SWo", l))
		d.dWCq = b.alloc(uint64(len(b.decHost[l].WCq)*4), fmt.Sprintf("t5.D%d.WCq", l))
		d.dWCk = b.alloc(uint64(len(b.decHost[l].WCk)*4), fmt.Sprintf("t5.D%d.WCk", l))
		d.dWCv = b.alloc(uint64(len(b.decHost[l].WCv)*4), fmt.Sprintf("t5.D%d.WCv", l))
		d.dWCo = b.alloc(uint64(len(b.decHost[l].WCo)*4), fmt.Sprintf("t5.D%d.WCo", l))
		d.dW1 = b.alloc(uint64(len(b.decHost[l].W1)*4), fmt.Sprintf("t5.D%d.W1", l))
		d.dW2 = b.alloc(uint64(len(b.decHost[l].W2)*4), fmt.Sprintf("t5.D%d.W2", l))
		d.dG1 = b.alloc(uint64(len(b.decHost[l].G1)*4), fmt.Sprintf("t5.D%d.G1", l))
		d.dB1 = b.alloc(uint64(len(b.decHost[l].B1)*4), fmt.Sprintf("t5.D%d.B1", l))
		d.dGx = b.alloc(uint64(len(b.decHost[l].Gx)*4), fmt.Sprintf("t5.D%d.Gx", l))
		d.dBx = b.alloc(uint64(len(b.decHost[l].Bx)*4), fmt.Sprintf("t5.D%d.Bx", l))
		d.dG2 = b.alloc(uint64(len(b.decHost[l].G2)*4), fmt.Sprintf("t5.D%d.G2", l))
		d.dB2 = b.alloc(uint64(len(b.decHost[l].B2)*4), fmt.Sprintf("t5.D%d.B2", l))
		b.decDev[l] = d
		for ptr, arr := range map[driver.Ptr][]float32{d.dWSq: b.decHost[l].WSq, d.dWSk: b.decHost[l].WSk, d.dWSv: b.decHost[l].WSv, d.dSWo: b.decHost[l].SWo, d.dWCq: b.decHost[l].WCq, d.dWCk: b.decHost[l].WCk, d.dWCv: b.decHost[l].WCv, d.dWCo: b.decHost[l].WCo, d.dW1: b.decHost[l].W1, d.dW2: b.decHost[l].W2, d.dG1: b.decHost[l].G1, d.dB1: b.decHost[l].B1, d.dGx: b.decHost[l].Gx, d.dBx: b.decHost[l].Bx, d.dG2: b.decHost[l].G2, d.dB2: b.decHost[l].B2} {
			b.driver.MemCopyH2D(b.context, ptr, arr)
		}
	}
}

func (b *Benchmark) exec() {
	dh := b.DModel / b.NumHeads
	encCur, encOther := b.dEncX, b.dEncX2
	for l := 0; l < b.Layers; l++ {
		d := b.encDev[l]
		b.launch1D(b.kLN1, uint32(b.EncSeq), 64, &LayerNormArgs{In: encCur, Gamma: d.dG1, Beta: d.dB1, Out: encOther, SeqLen: int32(b.EncSeq), DModel: int32(b.DModel), Eps: 1e-5})
		b.launch2D(b.kQProj, uint32(b.DModel), uint32(b.EncSeq), 8, 8, &MatMulArgs{A: encOther, B: d.dWq, C: b.dQ, M: int32(b.EncSeq), N: int32(b.DModel), K: int32(b.DModel)})
		b.launch2D(b.kKProj, uint32(b.DModel), uint32(b.EncSeq), 8, 8, &MatMulArgs{A: encOther, B: d.dWk, C: b.dK, M: int32(b.EncSeq), N: int32(b.DModel), K: int32(b.DModel)})
		b.launch2D(b.kVProj, uint32(b.DModel), uint32(b.EncSeq), 8, 8, &MatMulArgs{A: encOther, B: d.dWv, C: b.dV, M: int32(b.EncSeq), N: int32(b.DModel), K: int32(b.DModel)})
		b.launch2D(b.kScores, uint32(b.EncSeq), uint32(b.NumHeads*b.EncSeq), 8, 8, &AttnScoreArgs{Q: b.dQ, K: b.dK, Scores: b.dScores, QLen: int32(b.EncSeq), KLen: int32(b.EncSeq), DHead: int32(dh), NumHeads: int32(b.NumHeads)})
		b.launch2D(b.kSoftmax, uint32(b.EncSeq), uint32(b.NumHeads), 8, 8, &SoftmaxArgs{In: b.dScores, Out: b.dProbs, QLen: int32(b.EncSeq), KLen: int32(b.EncSeq), NumHeads: int32(b.NumHeads)})
		b.launch2D(b.kContext, uint32(b.DModel/b.NumHeads), uint32(b.NumHeads*b.EncSeq), 8, 8, &AttnCtxArgs{Probs: b.dProbs, V: b.dV, Context: b.dContext, QLen: int32(b.EncSeq), KLen: int32(b.EncSeq), DHead: int32(dh), NumHeads: int32(b.NumHeads)})
		b.launch2D(b.kOProj, uint32(b.DModel), uint32(b.EncSeq), 8, 8, &MatMulArgs{A: b.dContext, B: d.dWo, C: b.dTmpOut, M: int32(b.EncSeq), N: int32(b.DModel), K: int32(b.DModel)})
		b.launch1D(b.kAdd1, uint32(b.EncSeq*b.DModel), 64, &AddArgs{A: encCur, B: b.dTmpOut, Out: encOther, N: int32(b.EncSeq * b.DModel)})
		encCur, encOther = encOther, encCur
		b.launch1D(b.kLN2, uint32(b.EncSeq), 64, &LayerNormArgs{In: encCur, Gamma: d.dG2, Beta: d.dB2, Out: encOther, SeqLen: int32(b.EncSeq), DModel: int32(b.DModel), Eps: 1e-5})
		b.launch2D(b.kFF1, uint32(b.DFF), uint32(b.EncSeq), 8, 8, &MatMulArgs{A: encOther, B: d.dW1, C: b.dFF1, M: int32(b.EncSeq), N: int32(b.DFF), K: int32(b.DModel)})
		b.launch1D(b.kReLU, uint32(b.EncSeq*b.DFF), 64, &ReLUArgs{X: b.dFF1, N: int32(b.EncSeq * b.DFF)})
		b.launch2D(b.kFF2, uint32(b.DModel), uint32(b.EncSeq), 8, 8, &MatMulArgs{A: b.dFF1, B: d.dW2, C: b.dTmpOut, M: int32(b.EncSeq), N: int32(b.DModel), K: int32(b.DFF)})
		b.launch1D(b.kAdd2, uint32(b.EncSeq*b.DModel), 64, &AddArgs{A: encCur, B: b.dTmpOut, Out: encOther, N: int32(b.EncSeq * b.DModel)})
		encCur, encOther = encOther, encCur
	}

	decCur, decOther := b.dDecX, b.dDecX2
	for l := 0; l < b.Layers; l++ {
		d := b.decDev[l]
		b.launch1D(b.kLN1, uint32(b.DecSeq), 64, &LayerNormArgs{In: decCur, Gamma: d.dG1, Beta: d.dB1, Out: decOther, SeqLen: int32(b.DecSeq), DModel: int32(b.DModel), Eps: 1e-5})
		b.launch2D(b.kQProj, uint32(b.DModel), uint32(b.DecSeq), 8, 8, &MatMulArgs{A: decOther, B: d.dWSq, C: b.dQ, M: int32(b.DecSeq), N: int32(b.DModel), K: int32(b.DModel)})
		b.launch2D(b.kKProj, uint32(b.DModel), uint32(b.DecSeq), 8, 8, &MatMulArgs{A: decOther, B: d.dWSk, C: b.dK, M: int32(b.DecSeq), N: int32(b.DModel), K: int32(b.DModel)})
		b.launch2D(b.kVProj, uint32(b.DModel), uint32(b.DecSeq), 8, 8, &MatMulArgs{A: decOther, B: d.dWSv, C: b.dV, M: int32(b.DecSeq), N: int32(b.DModel), K: int32(b.DModel)})
		b.launch2D(b.kScoresMasked, uint32(b.DecSeq), uint32(b.NumHeads*b.DecSeq), 8, 8, &AttnScoreArgs{Q: b.dQ, K: b.dK, Scores: b.dScores, QLen: int32(b.DecSeq), KLen: int32(b.DecSeq), DHead: int32(dh), NumHeads: int32(b.NumHeads)})
		b.launch2D(b.kSoftmax, uint32(b.DecSeq), uint32(b.NumHeads), 8, 8, &SoftmaxArgs{In: b.dScores, Out: b.dProbs, QLen: int32(b.DecSeq), KLen: int32(b.DecSeq), NumHeads: int32(b.NumHeads)})
		b.launch2D(b.kContext, uint32(b.DModel/b.NumHeads), uint32(b.NumHeads*b.DecSeq), 8, 8, &AttnCtxArgs{Probs: b.dProbs, V: b.dV, Context: b.dContext, QLen: int32(b.DecSeq), KLen: int32(b.DecSeq), DHead: int32(dh), NumHeads: int32(b.NumHeads)})
		b.launch2D(b.kOProj, uint32(b.DModel), uint32(b.DecSeq), 8, 8, &MatMulArgs{A: b.dContext, B: d.dSWo, C: b.dTmpOut, M: int32(b.DecSeq), N: int32(b.DModel), K: int32(b.DModel)})
		b.launch1D(b.kAdd1, uint32(b.DecSeq*b.DModel), 64, &AddArgs{A: decCur, B: b.dTmpOut, Out: decOther, N: int32(b.DecSeq * b.DModel)})
		decCur, decOther = decOther, decCur

		b.launch1D(b.kLNX, uint32(b.DecSeq), 64, &LayerNormArgs{In: decCur, Gamma: d.dGx, Beta: d.dBx, Out: decOther, SeqLen: int32(b.DecSeq), DModel: int32(b.DModel), Eps: 1e-5})
		b.launch2D(b.kQProj, uint32(b.DModel), uint32(b.DecSeq), 8, 8, &MatMulArgs{A: decOther, B: d.dWCq, C: b.dQ, M: int32(b.DecSeq), N: int32(b.DModel), K: int32(b.DModel)})
		b.launch2D(b.kKProj, uint32(b.DModel), uint32(b.EncSeq), 8, 8, &MatMulArgs{A: encCur, B: d.dWCk, C: b.dK, M: int32(b.EncSeq), N: int32(b.DModel), K: int32(b.DModel)})
		b.launch2D(b.kVProj, uint32(b.DModel), uint32(b.EncSeq), 8, 8, &MatMulArgs{A: encCur, B: d.dWCv, C: b.dV, M: int32(b.EncSeq), N: int32(b.DModel), K: int32(b.DModel)})
		b.launch2D(b.kScores, uint32(b.EncSeq), uint32(b.NumHeads*b.DecSeq), 8, 8, &AttnScoreArgs{Q: b.dQ, K: b.dK, Scores: b.dScores, QLen: int32(b.DecSeq), KLen: int32(b.EncSeq), DHead: int32(dh), NumHeads: int32(b.NumHeads)})
		b.launch2D(b.kSoftmax, uint32(b.DecSeq), uint32(b.NumHeads), 8, 8, &SoftmaxArgs{In: b.dScores, Out: b.dProbs, QLen: int32(b.DecSeq), KLen: int32(b.EncSeq), NumHeads: int32(b.NumHeads)})
		b.launch2D(b.kContext, uint32(b.DModel/b.NumHeads), uint32(b.NumHeads*b.DecSeq), 8, 8, &AttnCtxArgs{Probs: b.dProbs, V: b.dV, Context: b.dContext, QLen: int32(b.DecSeq), KLen: int32(b.EncSeq), DHead: int32(dh), NumHeads: int32(b.NumHeads)})
		b.launch2D(b.kOProj, uint32(b.DModel), uint32(b.DecSeq), 8, 8, &MatMulArgs{A: b.dContext, B: d.dWCo, C: b.dTmpOut, M: int32(b.DecSeq), N: int32(b.DModel), K: int32(b.DModel)})
		b.launch1D(b.kAddX, uint32(b.DecSeq*b.DModel), 64, &AddArgs{A: decCur, B: b.dTmpOut, Out: decOther, N: int32(b.DecSeq * b.DModel)})
		decCur, decOther = decOther, decCur

		b.launch1D(b.kLN2, uint32(b.DecSeq), 64, &LayerNormArgs{In: decCur, Gamma: d.dG2, Beta: d.dB2, Out: decOther, SeqLen: int32(b.DecSeq), DModel: int32(b.DModel), Eps: 1e-5})
		b.launch2D(b.kFF1, uint32(b.DFF), uint32(b.DecSeq), 8, 8, &MatMulArgs{A: decOther, B: d.dW1, C: b.dFF1, M: int32(b.DecSeq), N: int32(b.DFF), K: int32(b.DModel)})
		b.launch1D(b.kReLU, uint32(b.DecSeq*b.DFF), 64, &ReLUArgs{X: b.dFF1, N: int32(b.DecSeq * b.DFF)})
		b.launch2D(b.kFF2, uint32(b.DModel), uint32(b.DecSeq), 8, 8, &MatMulArgs{A: b.dFF1, B: d.dW2, C: b.dTmpOut, M: int32(b.DecSeq), N: int32(b.DModel), K: int32(b.DFF)})
		b.launch1D(b.kAdd2, uint32(b.DecSeq*b.DModel), 64, &AddArgs{A: decCur, B: b.dTmpOut, Out: decOther, N: int32(b.DecSeq * b.DModel)})
		decCur, decOther = decOther, decCur
	}
	b.launch2D(b.kLMHead, uint32(b.Vocab), uint32(b.DecSeq), 8, 8, &MatMulArgs{A: decCur, B: b.dWlm, C: b.dLogits, M: int32(b.DecSeq), N: int32(b.Vocab), K: int32(b.DModel)})
	b.driver.MemCopyD2H(b.context, b.hostLogits, b.dLogits)
}

func (b *Benchmark) Verify() {
	ref := b.cpuForwardLogits()
	const atol = 2e-3
	if len(ref) != len(b.hostLogits) {
		log.Panicf("t5_small logits length mismatch: ref=%d, got=%d", len(ref), len(b.hostLogits))
	}
	for i := range ref {
		diff := math.Abs(float64(ref[i] - b.hostLogits[i]))
		if diff > atol {
			log.Panicf("Mismatch at logit %d, expected %.6f, got %.6f (diff=%.6f)",
				i, ref[i], b.hostLogits[i], diff)
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

func (b *Benchmark) cpuLayerNorm(x, gamma, beta []float32, seq, dm int) []float32 {
	out := make([]float32, len(x))
	const eps = 1e-5
	for row := 0; row < seq; row++ {
		base := row * dm
		var mean float32
		for j := 0; j < dm; j++ {
			mean += x[base+j]
		}
		mean *= float32(1.0) / float32(dm)
		var varsum float32
		for j := 0; j < dm; j++ {
			d := x[base+j] - mean
			varsum += d * d
		}
		varsum *= float32(1.0) / float32(dm)
		inv := float32(1.0 / math.Sqrt(float64(varsum+eps)))
		for j := 0; j < dm; j++ {
			out[base+j] = (x[base+j]-mean)*inv*gamma[j] + beta[j]
		}
	}
	return out
}

func (b *Benchmark) cpuReLU(x []float32) []float32 {
	out := make([]float32, len(x))
	for i, v := range x {
		if v < 0 {
			v = 0
		}
		out[i] = v
	}
	return out
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

func (b *Benchmark) cpuAttention(q, k, v []float32, qLen, kLen, dm, heads int, masked bool) []float32 {
	dh := dm / heads
	probs := make([]float32, heads*qLen*kLen)
	context := make([]float32, qLen*dm)
	scale := float32(1.0 / math.Sqrt(float64(dh)))

	for h := 0; h < heads; h++ {
		for i := 0; i < qLen; i++ {
			rowStart := (h*qLen + i) * kLen
			maxv := float32(-1e30)
			for j := 0; j < kLen; j++ {
				var score float32
				if masked && j > i {
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
			for j := 0; j < kLen; j++ {
				e := cpuOpenCLExpF32(probs[rowStart+j] - maxv)
				probs[rowStart+j] = e
				sumExp += e
			}
			inv := float32(1.0) / sumExp
			for j := 0; j < kLen; j++ {
				probs[rowStart+j] *= inv
			}
		}
	}

	for i := 0; i < qLen; i++ {
		for h := 0; h < heads; h++ {
			for kk := 0; kk < dh; kk++ {
				var sum float32
				pBase := (h*qLen + i) * kLen
				for j := 0; j < kLen; j++ {
					sum += probs[pBase+j] * v[j*dm+h*dh+kk]
				}
				context[i*dm+h*dh+kk] = sum
			}
		}
	}
	return context
}

func (b *Benchmark) cpuEncoderForward() []float32 {
	encCur := make([]float32, len(b.hostEnc))
	copy(encCur, b.hostEnc)

	for l := 0; l < b.Layers; l++ {
		h := b.encHost[l]
		norm := b.cpuLayerNorm(encCur, h.G1, h.B1, b.EncSeq, b.DModel)
		q := b.cpuMatMul(norm, h.Wq, b.EncSeq, b.DModel, b.DModel)
		k := b.cpuMatMul(norm, h.Wk, b.EncSeq, b.DModel, b.DModel)
		v := b.cpuMatMul(norm, h.Wv, b.EncSeq, b.DModel, b.DModel)
		ctx := b.cpuAttention(q, k, v, b.EncSeq, b.EncSeq, b.DModel, b.NumHeads, false)
		attnOut := b.cpuMatMul(ctx, h.Wo, b.EncSeq, b.DModel, b.DModel)
		encCur = b.cpuAdd(encCur, attnOut)

		norm = b.cpuLayerNorm(encCur, h.G2, h.B2, b.EncSeq, b.DModel)
		ff1 := b.cpuMatMul(norm, h.W1, b.EncSeq, b.DFF, b.DModel)
		ff1 = b.cpuReLU(ff1)
		ff2 := b.cpuMatMul(ff1, h.W2, b.EncSeq, b.DModel, b.DFF)
		encCur = b.cpuAdd(encCur, ff2)
	}
	return encCur
}

func (b *Benchmark) cpuForwardLogits() []float32 {
	encCur := b.cpuEncoderForward()
	decCur := make([]float32, len(b.hostDec))
	copy(decCur, b.hostDec)

	for l := 0; l < b.Layers; l++ {
		h := b.decHost[l]
		norm := b.cpuLayerNorm(decCur, h.G1, h.B1, b.DecSeq, b.DModel)
		q := b.cpuMatMul(norm, h.WSq, b.DecSeq, b.DModel, b.DModel)
		k := b.cpuMatMul(norm, h.WSk, b.DecSeq, b.DModel, b.DModel)
		v := b.cpuMatMul(norm, h.WSv, b.DecSeq, b.DModel, b.DModel)
		ctx := b.cpuAttention(q, k, v, b.DecSeq, b.DecSeq, b.DModel, b.NumHeads, true)
		attnOut := b.cpuMatMul(ctx, h.SWo, b.DecSeq, b.DModel, b.DModel)
		decCur = b.cpuAdd(decCur, attnOut)

		norm = b.cpuLayerNorm(decCur, h.Gx, h.Bx, b.DecSeq, b.DModel)
		q = b.cpuMatMul(norm, h.WCq, b.DecSeq, b.DModel, b.DModel)
		k = b.cpuMatMul(encCur, h.WCk, b.EncSeq, b.DModel, b.DModel)
		v = b.cpuMatMul(encCur, h.WCv, b.EncSeq, b.DModel, b.DModel)
		ctx = b.cpuAttention(q, k, v, b.DecSeq, b.EncSeq, b.DModel, b.NumHeads, false)
		attnOut = b.cpuMatMul(ctx, h.WCo, b.DecSeq, b.DModel, b.DModel)
		decCur = b.cpuAdd(decCur, attnOut)

		norm = b.cpuLayerNorm(decCur, h.G2, h.B2, b.DecSeq, b.DModel)
		ff1 := b.cpuMatMul(norm, h.W1, b.DecSeq, b.DFF, b.DModel)
		ff1 = b.cpuReLU(ff1)
		ff2 := b.cpuMatMul(ff1, h.W2, b.DecSeq, b.DModel, b.DFF)
		decCur = b.cpuAdd(decCur, ff2)
	}

	return b.cpuMatMul(decCur, b.hWlm, b.DecSeq, b.Vocab, b.DModel)
}
