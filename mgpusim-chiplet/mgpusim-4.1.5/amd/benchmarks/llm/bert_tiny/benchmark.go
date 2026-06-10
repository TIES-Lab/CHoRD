// Package bert implements a simplified BERT *inference* (forward-only) benchmark
// for MGPUSim.
//
// The goal of this benchmark is NOT numerical fidelity to HuggingFace BERT.
// Instead, it provides a BERT-shaped workload with clear, separated kernels so
// that the chiplet planner can optimize:
//   - buffer page placement (per-tag)
//   - per-kernel scheduling / partition hints
//
// Modeled structure:
//  1. Embedding stage (already after lookup):
//     TokEmb + PosEmb + TypeEmb -> LayerNorm
//  2. Encoder stack (repeated Layers times):
//     Q/K/V projections -> Attention scores -> Softmax -> Context
//     -> Output projection -> Residual + LayerNorm
//     -> FFN (MatMul + ReLU + MatMul) -> Residual + LayerNorm
//  3. Pooler on [CLS] token (token 0): Linear(dModel->dModel) + ReLU
//  4. Classifier: Linear(dModel->NumClass) -> logits
//
// All parameters/weights/inputs are randomly generated with a fixed seed.
//
// Build:
//
//	cd amd/benchmarks/llm/bert && make
//	cd amd/samples/bert && go build
package bert_tiny

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

// ===== Kernel argument structs (must match OpenCL signatures) =====

// Common GEMM args: C[M,N] = A[M,K] * B[K,N] (row-major).
// Used for projections, FFN, pooler, classifier.
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

type ReLUArgs struct {
	X driver.Ptr
	N int32
}

type layerDev struct {
	dWq, dWk, dWv, dWo driver.Ptr
	dW1, dW2           driver.Ptr
	dGamma1, dBeta1    driver.Ptr
	dGamma2, dBeta2    driver.Ptr
}

type layerHost struct {
	Wq, Wk, Wv, Wo []float32
	W1, W2         []float32
	Gamma1, Beta1  []float32
	Gamma2, Beta2  []float32
}

// Benchmark is the BERT inference benchmark.
type Benchmark struct {
	driver  *driver.Driver
	context *driver.Context
	gpus    []int

	// kernels (reusing tiny_encoder hsaco names)
	kQProj   *insts.HsaCo
	kKProj   *insts.HsaCo
	kVProj   *insts.HsaCo
	kScores  *insts.HsaCo
	kSoftmax *insts.HsaCo
	kContext *insts.HsaCo
	kOProj   *insts.HsaCo

	kAdd1 *insts.HsaCo
	kLN1  *insts.HsaCo

	kFF1  *insts.HsaCo
	kReLU *insts.HsaCo
	kFF2  *insts.HsaCo

	kAdd2 *insts.HsaCo
	kLN2  *insts.HsaCo

	// model sizes (tunable)
	SeqLen   int
	DModel   int
	NumHeads int
	DFF      int
	Layers   int
	NumClass int

	// batch inference controls
	BatchTotal       int
	BatchStart       int
	BatchCount       int
	BatchSeed        int64
	BatchUpdateInput bool

	// host inputs
	hostTokEmb  []float32
	hostPosEmb  []float32
	hostTypeEmb []float32

	// host weights
	hostEmbGamma []float32
	hostEmbBeta  []float32
	hostWPool    []float32
	hostWCls     []float32
	layersHost   []layerHost

	// host output
	hostLogits []float32

	// device buffers: embedding stage
	dTokEmb   driver.Ptr
	dPosEmb   driver.Ptr
	dTypeEmb  driver.Ptr
	dEmbTmp   driver.Ptr
	dEmbSum   driver.Ptr
	dEmbGamma driver.Ptr
	dEmbBeta  driver.Ptr

	// device buffers: encoder (ping-pong X)
	dX  driver.Ptr
	dX2 driver.Ptr

	dQ, dK, dV       driver.Ptr
	dScores, dProbs  driver.Ptr
	dContext, dOut   driver.Ptr
	dY1, dY1n        driver.Ptr
	dFF1Buf, dFF2Buf driver.Ptr
	dY2              driver.Ptr

	layersDev []layerDev

	// device buffers: pooler + classifier
	dPool   driver.Ptr
	dLogits driver.Ptr
	dWPool  driver.Ptr
	dWCls   driver.Ptr

	useUnifiedMemory bool
}

func NewBenchmark(driver *driver.Driver) *Benchmark {
	b := new(Benchmark)
	b.driver = driver
	b.context = driver.Init()

	// Reasonable small defaults (can be overridden by samples/bert flags).
	b.SeqLen = 64
	b.DModel = 128
	b.NumHeads = 8
	b.DFF = 512
	b.Layers = 2
	b.NumClass = 2
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
		log.Panicf("Failed to load kernel binary: %s", name)
	}
	return k
}

func (b *Benchmark) loadProgram() {
	// NOTE: kernel names in kernels.cl use the bt_* prefix for bert_tiny.
	b.kQProj = mustLoadKernel(hsacoBytes, "bt_q_proj")
	b.kKProj = mustLoadKernel(hsacoBytes, "bt_k_proj")
	b.kVProj = mustLoadKernel(hsacoBytes, "bt_v_proj")

	b.kScores = mustLoadKernel(hsacoBytes, "bt_attention_scores")
	b.kSoftmax = mustLoadKernel(hsacoBytes, "bt_softmax")
	b.kContext = mustLoadKernel(hsacoBytes, "bt_attention_context")

	b.kOProj = mustLoadKernel(hsacoBytes, "bt_o_proj")

	b.kAdd1 = mustLoadKernel(hsacoBytes, "bt_add_resid1")
	b.kLN1 = mustLoadKernel(hsacoBytes, "bt_layernorm1")

	b.kFF1 = mustLoadKernel(hsacoBytes, "bt_ff1")
	b.kReLU = mustLoadKernel(hsacoBytes, "bt_relu")
	b.kFF2 = mustLoadKernel(hsacoBytes, "bt_ff2")

	b.kAdd2 = mustLoadKernel(hsacoBytes, "bt_add_resid2")
	b.kLN2 = mustLoadKernel(hsacoBytes, "bt_layernorm2")
}

func (b *Benchmark) alloc(byteSize uint64, tag string) driver.Ptr {
	if b.useUnifiedMemory {
		return b.driver.AllocateUnifiedMemory(b.context, byteSize)
	}
	return b.driver.AllocateMemoryWithTag(b.context, byteSize, tag)
}

func (b *Benchmark) Run() {
	if b.DModel%b.NumHeads != 0 {
		log.Panicf("DModel must be divisible by NumHeads (got DModel=%d, NumHeads=%d)", b.DModel, b.NumHeads)
	}
	if b.Layers <= 0 {
		log.Panic("Layers must be > 0")
	}
	if b.SeqLen <= 0 {
		log.Panic("SeqLen must be > 0")
	}
	if b.NumClass <= 0 {
		log.Panic("NumClass must be > 0")
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
		b.driver.MemCopyH2D(b.context, b.dTokEmb, b.hostTokEmb)
		b.driver.MemCopyH2D(b.context, b.dPosEmb, b.hostPosEmb)
		b.driver.MemCopyH2D(b.context, b.dTypeEmb, b.hostTypeEmb)
	}

	for bi := start; bi < end; bi++ {
		if b.BatchUpdateInput && bi != start {
			b.fillInputForBatch(bi)
			b.driver.MemCopyH2D(b.context, b.dTokEmb, b.hostTokEmb)
			b.driver.MemCopyH2D(b.context, b.dPosEmb, b.hostPosEmb)
			b.driver.MemCopyH2D(b.context, b.dTypeEmb, b.hostTypeEmb)
		}
		b.exec()
	}
}

func (b *Benchmark) fillInputForBatch(batch int) {
	seed := b.BatchSeed + int64(batch)*1315423911
	rng := rand.New(rand.NewSource(seed))
	for i := range b.hostTokEmb {
		b.hostTokEmb[i] = (rng.Float32() - 0.5) * 0.02
	}
	for i := range b.hostPosEmb {
		b.hostPosEmb[i] = (rng.Float32() - 0.5) * 0.02
	}
	for i := range b.hostTypeEmb {
		b.hostTypeEmb[i] = (rng.Float32() - 0.5) * 0.02
	}
}

func (b *Benchmark) initMem() {
	rand.Seed(1)

	seq := b.SeqLen
	dm := b.DModel

	// Host inputs: embeddings already after lookup.
	b.hostTokEmb = make([]float32, seq*dm)
	b.hostPosEmb = make([]float32, seq*dm)
	b.hostTypeEmb = make([]float32, seq*dm)

	initVec := func(v []float32) {
		for i := range v {
			v[i] = (rand.Float32() - 0.5) * 0.02
		}
	}
	initVec(b.hostTokEmb)
	initVec(b.hostPosEmb)
	initVec(b.hostTypeEmb)

	// Embedding LayerNorm params.
	b.hostEmbGamma = make([]float32, dm)
	b.hostEmbBeta = make([]float32, dm)
	for i := 0; i < dm; i++ {
		b.hostEmbGamma[i] = 1.0
		b.hostEmbBeta[i] = 0.0
	}

	// Pooler + classifier weights.
	b.hostWPool = make([]float32, dm*dm)
	b.hostWCls = make([]float32, dm*b.NumClass)
	initVec(b.hostWPool)
	initVec(b.hostWCls)

	// Per-layer weights for CPU verify.
	b.layersHost = make([]layerHost, b.Layers)
	for l := 0; l < b.Layers; l++ {
		b.layersHost[l] = layerHost{
			Wq:     make([]float32, dm*dm),
			Wk:     make([]float32, dm*dm),
			Wv:     make([]float32, dm*dm),
			Wo:     make([]float32, dm*dm),
			W1:     make([]float32, dm*b.DFF),
			W2:     make([]float32, b.DFF*dm),
			Gamma1: make([]float32, dm),
			Beta1:  make([]float32, dm),
			Gamma2: make([]float32, dm),
			Beta2:  make([]float32, dm),
		}

		initVec(b.layersHost[l].Wq)
		initVec(b.layersHost[l].Wk)
		initVec(b.layersHost[l].Wv)
		initVec(b.layersHost[l].Wo)
		initVec(b.layersHost[l].W1)
		initVec(b.layersHost[l].W2)

		for i := 0; i < dm; i++ {
			b.layersHost[l].Gamma1[i] = 1.0
			b.layersHost[l].Beta1[i] = 0.0
			b.layersHost[l].Gamma2[i] = 1.0
			b.layersHost[l].Beta2[i] = 0.0
		}
	}

	// Device allocations.
	bytesX := uint64(seq * dm * 4)
	bytesScores := uint64(b.NumHeads * seq * seq * 4)
	bytesFF1 := uint64(seq * b.DFF * 4)

	// Embedding stage buffers.
	b.dTokEmb = b.alloc(bytesX, "bt.TokEmb")
	b.dPosEmb = b.alloc(bytesX, "bt.PosEmb")
	b.dTypeEmb = b.alloc(bytesX, "bt.TypeEmb")
	b.dEmbTmp = b.alloc(bytesX, "bt.EmbTmp")
	b.dEmbSum = b.alloc(bytesX, "bt.EmbSum")
	b.dEmbGamma = b.alloc(uint64(dm*4), "bt.EmbGamma")
	b.dEmbBeta = b.alloc(uint64(dm*4), "bt.EmbBeta")

	// Encoder buffers (ping-pong X).
	b.dX = b.alloc(bytesX, "bt.X")
	b.dX2 = b.alloc(bytesX, "bt.X2")

	b.dQ = b.alloc(bytesX, "bt.Q")
	b.dK = b.alloc(bytesX, "bt.K")
	b.dV = b.alloc(bytesX, "bt.V")

	b.dScores = b.alloc(bytesScores, "bt.Scores")
	b.dProbs = b.alloc(bytesScores, "bt.Probs")

	b.dContext = b.alloc(bytesX, "bt.Context")
	b.dOut = b.alloc(bytesX, "bt.Out")
	b.dY1 = b.alloc(bytesX, "bt.Y1")
	b.dY1n = b.alloc(bytesX, "bt.Y1n")
	b.dY2 = b.alloc(bytesX, "bt.Y2")

	b.dFF1Buf = b.alloc(bytesFF1, "bt.FF1")
	b.dFF2Buf = b.alloc(bytesX, "bt.FF2")

	// Pooler + classifier buffers.
	b.dPool = b.alloc(uint64(dm*4), "bt.Pool")
	b.dLogits = b.alloc(uint64(b.NumClass*4), "bt.Logits")
	b.dWPool = b.alloc(uint64(dm*dm*4), "bt.WPool")
	b.dWCls = b.alloc(uint64(dm*b.NumClass*4), "bt.WCls")

	// Per-layer device weights.
	b.layersDev = make([]layerDev, b.Layers)
	for l := 0; l < b.Layers; l++ {
		// Use per-layer tags; Step2 v3 resolves fmt.Sprintf tags statically.
		b.layersDev[l].dWq = b.alloc(uint64(dm*dm*4), fmt.Sprintf("bt.L%d.Wq", l))
		b.layersDev[l].dWk = b.alloc(uint64(dm*dm*4), fmt.Sprintf("bt.L%d.Wk", l))
		b.layersDev[l].dWv = b.alloc(uint64(dm*dm*4), fmt.Sprintf("bt.L%d.Wv", l))
		b.layersDev[l].dWo = b.alloc(uint64(dm*dm*4), fmt.Sprintf("bt.L%d.Wo", l))
		b.layersDev[l].dW1 = b.alloc(uint64(dm*b.DFF*4), fmt.Sprintf("bt.L%d.W1", l))
		b.layersDev[l].dW2 = b.alloc(uint64(b.DFF*dm*4), fmt.Sprintf("bt.L%d.W2", l))
		b.layersDev[l].dGamma1 = b.alloc(uint64(dm*4), fmt.Sprintf("bt.L%d.Gamma1", l))
		b.layersDev[l].dBeta1 = b.alloc(uint64(dm*4), fmt.Sprintf("bt.L%d.Beta1", l))
		b.layersDev[l].dGamma2 = b.alloc(uint64(dm*4), fmt.Sprintf("bt.L%d.Gamma2", l))
		b.layersDev[l].dBeta2 = b.alloc(uint64(dm*4), fmt.Sprintf("bt.L%d.Beta2", l))
	}

	// Copies: host -> device.
	b.driver.MemCopyH2D(b.context, b.dTokEmb, b.hostTokEmb)
	b.driver.MemCopyH2D(b.context, b.dPosEmb, b.hostPosEmb)
	b.driver.MemCopyH2D(b.context, b.dTypeEmb, b.hostTypeEmb)
	b.driver.MemCopyH2D(b.context, b.dEmbGamma, b.hostEmbGamma)
	b.driver.MemCopyH2D(b.context, b.dEmbBeta, b.hostEmbBeta)
	b.driver.MemCopyH2D(b.context, b.dWPool, b.hostWPool)
	b.driver.MemCopyH2D(b.context, b.dWCls, b.hostWCls)

	for l := 0; l < b.Layers; l++ {
		h := b.layersHost[l]
		d := b.layersDev[l]
		b.driver.MemCopyH2D(b.context, d.dWq, h.Wq)
		b.driver.MemCopyH2D(b.context, d.dWk, h.Wk)
		b.driver.MemCopyH2D(b.context, d.dWv, h.Wv)
		b.driver.MemCopyH2D(b.context, d.dWo, h.Wo)
		b.driver.MemCopyH2D(b.context, d.dW1, h.W1)
		b.driver.MemCopyH2D(b.context, d.dW2, h.W2)
		b.driver.MemCopyH2D(b.context, d.dGamma1, h.Gamma1)
		b.driver.MemCopyH2D(b.context, d.dBeta1, h.Beta1)
		b.driver.MemCopyH2D(b.context, d.dGamma2, h.Gamma2)
		b.driver.MemCopyH2D(b.context, d.dBeta2, h.Beta2)
	}

	b.hostLogits = make([]float32, b.NumClass)
}

func roundUp(x, multiple uint32) uint32 {
	if multiple == 0 {
		return x
	}
	r := x % multiple
	if r == 0 {
		return x
	}
	return x + (multiple - r)
}

func (b *Benchmark) launch1D(kernel *insts.HsaCo, total uint32, local uint16, args interface{}) {
	if total == 0 {
		return
	}
	localSize := [3]uint16{local, 1, 1}
	globalSize := [3]uint32{roundUp(total, uint32(local)), 1, 1}
	b.driver.LaunchKernel(b.context, kernel, globalSize, localSize, args)
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

func (b *Benchmark) exec() {
	seq := int32(b.SeqLen)
	dm := int32(b.DModel)
	heads := int32(b.NumHeads)
	dh := int32(b.DModel / b.NumHeads)
	dff := int32(b.DFF)
	eps := float32(1e-5)
	cls := int32(b.NumClass)

	// === 1) Embedding stage ===
	// TokEmb + PosEmb -> EmbTmp
	b.launch1D(b.kAdd1, uint32(b.SeqLen*b.DModel), 64, &AddArgs{
		A: b.dTokEmb, B: b.dPosEmb, Out: b.dEmbTmp, N: seq * dm,
	})
	// EmbTmp + TypeEmb -> EmbSum
	b.launch1D(b.kAdd2, uint32(b.SeqLen*b.DModel), 64, &AddArgs{
		A: b.dEmbTmp, B: b.dTypeEmb, Out: b.dEmbSum, N: seq * dm,
	})
	// LayerNorm over embeddings -> dX (encoder input)
	b.launch1D(b.kLN1, uint32(b.SeqLen), 64, &LayerNormArgs{
		In: b.dEmbSum, Gamma: b.dEmbGamma, Beta: b.dEmbBeta, Out: b.dX,
		SeqLen: seq, DModel: dm, Eps: eps,
	})

	// === 2) Encoder stack ===
	curX := b.dX
	nextX := b.dX2

	for l := 0; l < b.Layers; l++ {
		// Q/K/V projections
		b.launch2D(b.kQProj, uint32(b.DModel), uint32(b.SeqLen), 8, 8, &MatMulArgs{
			A: curX, B: b.layersDev[l].dWq, C: b.dQ, M: seq, N: dm, K: dm,
		})
		b.launch2D(b.kKProj, uint32(b.DModel), uint32(b.SeqLen), 8, 8, &MatMulArgs{
			A: curX, B: b.layersDev[l].dWk, C: b.dK, M: seq, N: dm, K: dm,
		})
		b.launch2D(b.kVProj, uint32(b.DModel), uint32(b.SeqLen), 8, 8, &MatMulArgs{
			A: curX, B: b.layersDev[l].dWv, C: b.dV, M: seq, N: dm, K: dm,
		})

		// Attention scores -> softmax -> context
		b.launch2D(b.kScores, uint32(b.SeqLen), uint32(b.NumHeads*b.SeqLen), 8, 8, &AttnScoreArgs{
			Q: b.dQ, K: b.dK, Scores: b.dScores, SeqLen: seq, DHead: dh, NumHeads: heads,
		})
		b.launch2D(b.kSoftmax, uint32(b.SeqLen), uint32(b.NumHeads), 8, 8, &SoftmaxArgs{
			In: b.dScores, Out: b.dProbs, SeqLen: seq, NumHeads: heads,
		})
		b.launch2D(b.kContext, uint32(b.DModel/b.NumHeads), uint32(b.NumHeads*b.SeqLen), 8, 8, &AttnCtxArgs{
			Probs: b.dProbs, V: b.dV, Context: b.dContext, SeqLen: seq, DHead: dh, NumHeads: heads,
		})

		// Output projection
		b.launch2D(b.kOProj, uint32(b.DModel), uint32(b.SeqLen), 8, 8, &MatMulArgs{
			A: b.dContext, B: b.layersDev[l].dWo, C: b.dOut, M: seq, N: dm, K: dm,
		})

		// Residual + LN (after attention)
		b.launch1D(b.kAdd1, uint32(b.SeqLen*b.DModel), 64, &AddArgs{
			A: curX, B: b.dOut, Out: b.dY1, N: seq * dm,
		})
		b.launch1D(b.kLN1, uint32(b.SeqLen), 64, &LayerNormArgs{
			In: b.dY1, Gamma: b.layersDev[l].dGamma1, Beta: b.layersDev[l].dBeta1, Out: b.dY1n,
			SeqLen: seq, DModel: dm, Eps: eps,
		})

		// FFN
		b.launch2D(b.kFF1, uint32(b.DFF), uint32(b.SeqLen), 8, 8, &MatMulArgs{
			A: b.dY1n, B: b.layersDev[l].dW1, C: b.dFF1Buf, M: seq, N: dff, K: dm,
		})
		b.launch1D(b.kReLU, uint32(b.SeqLen*b.DFF), 64, &ReLUArgs{
			X: b.dFF1Buf, N: seq * dff,
		})
		b.launch2D(b.kFF2, uint32(b.DModel), uint32(b.SeqLen), 8, 8, &MatMulArgs{
			A: b.dFF1Buf, B: b.layersDev[l].dW2, C: b.dFF2Buf, M: seq, N: dm, K: dff,
		})

		// Residual + LN (after FFN)
		b.launch1D(b.kAdd2, uint32(b.SeqLen*b.DModel), 64, &AddArgs{
			A: b.dY1, B: b.dFF2Buf, Out: b.dY2, N: seq * dm,
		})
		b.launch1D(b.kLN2, uint32(b.SeqLen), 64, &LayerNormArgs{
			In: b.dY2, Gamma: b.layersDev[l].dGamma2, Beta: b.layersDev[l].dBeta2, Out: nextX,
			SeqLen: seq, DModel: dm, Eps: eps,
		})

		curX, nextX = nextX, curX
	}

	// === 3) Pooler on [CLS] token (token index 0) ===
	// Use M=1 so we only process the first row of curX.
	b.launch2D(b.kOProj, uint32(b.DModel), 1, 8, 8, &MatMulArgs{
		A: curX, B: b.dWPool, C: b.dPool, M: 1, N: dm, K: dm,
	})
	b.launch1D(b.kReLU, uint32(b.DModel), 64, &ReLUArgs{
		X: b.dPool, N: dm,
	})

	// === 4) Classifier ===
	b.launch2D(b.kFF1, uint32(b.NumClass), 1, 8, 8, &MatMulArgs{
		A: b.dPool, B: b.dWCls, C: b.dLogits, M: 1, N: cls, K: dm,
	})

	b.driver.MemCopyD2H(b.context, b.hostLogits, b.dLogits)
}

func (b *Benchmark) Verify() {
	ref := b.cpuForwardLogits()

	const atol = 2e-3
	if len(ref) != len(b.hostLogits) {
		log.Panicf("logits length mismatch: ref=%d, got=%d", len(ref), len(b.hostLogits))
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

func (b *Benchmark) cpuMatMul(A []float32, B []float32, M, N, K int) []float32 {
	C := make([]float32, M*N)
	for i := 0; i < M; i++ {
		for j := 0; j < N; j++ {
			var sum float32
			ai := i * K
			for k := 0; k < K; k++ {
				sum += A[ai+k] * B[k*N+j]
			}
			C[i*N+j] = sum
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

func (b *Benchmark) cpuReLU(x []float32) []float32 {
	out := make([]float32, len(x))
	for i := range x {
		v := x[i]
		if v < 0 {
			v = 0
		}
		out[i] = v
	}
	return out
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

			// softmax
			var sumExp float32
			for j := 0; j < seq; j++ {
				e := float32(math.Exp(float64(probs[rowStart+j] - maxv)))
				probs[rowStart+j] = e
				sumExp += e
			}
			inv := 1.0 / sumExp
			for j := 0; j < seq; j++ {
				probs[rowStart+j] *= inv
			}
		}
	}

	// context
	for i := 0; i < seq; i++ {
		for h := 0; h < heads; h++ {
			for k := 0; k < dh; k++ {
				var sum float32
				pBase := (h*seq + i) * seq
				for j := 0; j < seq; j++ {
					p := probs[pBase+j]
					v := V[j*dm+h*dh+k]
					sum += p * v
				}
				context[i*dm+h*dh+k] = sum
			}
		}
	}

	return context
}

func (b *Benchmark) cpuForwardLogits() []float32 {
	seq := b.SeqLen
	dm := b.DModel
	heads := b.NumHeads
	dff := b.DFF

	// Embedding: Tok + Pos + Type
	X := make([]float32, seq*dm)
	for i := range X {
		X[i] = b.hostTokEmb[i] + b.hostPosEmb[i] + b.hostTypeEmb[i]
	}
	X = b.cpuLayerNorm(X, b.hostEmbGamma, b.hostEmbBeta, seq, dm)

	// Encoder layers
	for l := 0; l < b.Layers; l++ {
		h := b.layersHost[l]
		Q := b.cpuMatMul(X, h.Wq, seq, dm, dm)
		K := b.cpuMatMul(X, h.Wk, seq, dm, dm)
		V := b.cpuMatMul(X, h.Wv, seq, dm, dm)

		C := b.cpuAttention(Q, K, V, seq, dm, heads)
		O := b.cpuMatMul(C, h.Wo, seq, dm, dm)

		Y1 := b.cpuAdd(X, O)
		Y1n := b.cpuLayerNorm(Y1, h.Gamma1, h.Beta1, seq, dm)

		FF1 := b.cpuMatMul(Y1n, h.W1, seq, dff, dm)
		FF1 = b.cpuReLU(FF1)
		FF2 := b.cpuMatMul(FF1, h.W2, seq, dm, dff)

		Y2 := b.cpuAdd(Y1, FF2)
		X = b.cpuLayerNorm(Y2, h.Gamma2, h.Beta2, seq, dm)
	}

	// Pooler on [CLS] token (row 0).
	clsVec := make([]float32, dm)
	copy(clsVec, X[:dm])
	pool := b.cpuMatMul(clsVec, b.hostWPool, 1, dm, dm)
	pool = b.cpuReLU(pool)

	// Classifier logits.
	logits := b.cpuMatMul(pool, b.hostWCls, 1, b.NumClass, dm)
	return logits
}
