package performer

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
type PhiArgs struct {
	In  driver.Ptr
	Out driver.Ptr
	N   int32
}
type KVReduceArgs struct {
	PhiK     driver.Ptr
	V        driver.Ptr
	KV       driver.Ptr
	SeqLen   int32
	DHead    int32
	NumHeads int32
}
type ZReduceArgs struct {
	PhiK     driver.Ptr
	Z        driver.Ptr
	SeqLen   int32
	DHead    int32
	NumHeads int32
}
type LinearCtxArgs struct {
	PhiQ     driver.Ptr
	KV       driver.Ptr
	Z        driver.Ptr
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

type layerHost struct{ Wq, Wk, Wv, Wo, W1, W2, G1, B1, G2, B2 []float32 }
type layerDev struct{ dWq, dWk, dWv, dWo, dW1, dW2, dG1, dB1, dG2, dB2 driver.Ptr }

type Benchmark struct {
	driver  *driver.Driver
	context *driver.Context
	gpus    []int

	kQProj, kKProj, kVProj          *insts.HsaCo
	kPhiQ, kPhiK                    *insts.HsaCo
	kKVReduce, kZReduce, kLinearCtx *insts.HsaCo
	kOProj                          *insts.HsaCo
	kAdd1, kAdd2                    *insts.HsaCo
	kLN1, kLN2                      *insts.HsaCo
	kFF1, kReLU, kFF2               *insts.HsaCo

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

	dX, dX2            driver.Ptr
	dQ, dK, dV         driver.Ptr
	dPhiQ, dPhiK       driver.Ptr
	dKV, dZ            driver.Ptr
	dContext, dTmpOut  driver.Ptr
	dFF1               driver.Ptr
	dWq, dWk, dWv, dWo []driver.Ptr
	dW1, dW2           []driver.Ptr
	dG1, dB1, dG2, dB2 []driver.Ptr
	useUnifiedMemory   bool
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
	b.kQProj = mustLoadKernel(hsacoBytes, "pf_q_proj")
	b.kKProj = mustLoadKernel(hsacoBytes, "pf_k_proj")
	b.kVProj = mustLoadKernel(hsacoBytes, "pf_v_proj")
	b.kPhiQ = mustLoadKernel(hsacoBytes, "pf_phi_q")
	b.kPhiK = mustLoadKernel(hsacoBytes, "pf_phi_k")
	b.kKVReduce = mustLoadKernel(hsacoBytes, "pf_kv_reduce")
	b.kZReduce = mustLoadKernel(hsacoBytes, "pf_z_reduce")
	b.kLinearCtx = mustLoadKernel(hsacoBytes, "pf_linear_context")
	b.kOProj = mustLoadKernel(hsacoBytes, "pf_o_proj")
	b.kAdd1 = mustLoadKernel(hsacoBytes, "pf_add_resid1")
	b.kAdd2 = mustLoadKernel(hsacoBytes, "pf_add_resid2")
	b.kLN1 = mustLoadKernel(hsacoBytes, "pf_layernorm1")
	b.kLN2 = mustLoadKernel(hsacoBytes, "pf_layernorm2")
	b.kFF1 = mustLoadKernel(hsacoBytes, "pf_ff1")
	b.kReLU = mustLoadKernel(hsacoBytes, "pf_relu")
	b.kFF2 = mustLoadKernel(hsacoBytes, "pf_ff2")
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
		log.Panic("DModel must divide NumHeads")
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
	for l := 0; l < b.Layers; l++ {
		h := layerHost{Wq: make([]float32, b.DModel*b.DModel), Wk: make([]float32, b.DModel*b.DModel), Wv: make([]float32, b.DModel*b.DModel), Wo: make([]float32, b.DModel*b.DModel), W1: make([]float32, b.DModel*b.DFF), W2: make([]float32, b.DFF*b.DModel), G1: make([]float32, b.DModel), B1: make([]float32, b.DModel), G2: make([]float32, b.DModel), B2: make([]float32, b.DModel)}
		for _, arr := range [][]float32{h.Wq, h.Wk, h.Wv, h.Wo, h.W1, h.W2} {
			initMat(arr)
		}
		initGamma(h.G1, h.B1)
		initGamma(h.G2, h.B2)
		b.layersHost[l] = h
	}
	b.dWq = make([]driver.Ptr, b.Layers)
	b.dWk = make([]driver.Ptr, b.Layers)
	b.dWv = make([]driver.Ptr, b.Layers)
	b.dWo = make([]driver.Ptr, b.Layers)
	b.dW1 = make([]driver.Ptr, b.Layers)
	b.dW2 = make([]driver.Ptr, b.Layers)
	b.dG1 = make([]driver.Ptr, b.Layers)
	b.dB1 = make([]driver.Ptr, b.Layers)
	b.dG2 = make([]driver.Ptr, b.Layers)
	b.dB2 = make([]driver.Ptr, b.Layers)
	dh := b.DModel / b.NumHeads
	b.dX = b.alloc(uint64(b.SeqLen*b.DModel*4), "pf.X")
	b.dX2 = b.alloc(uint64(b.SeqLen*b.DModel*4), "pf.X2")
	b.dQ = b.alloc(uint64(b.SeqLen*b.DModel*4), "pf.Q")
	b.dK = b.alloc(uint64(b.SeqLen*b.DModel*4), "pf.K")
	b.dV = b.alloc(uint64(b.SeqLen*b.DModel*4), "pf.V")
	b.dPhiQ = b.alloc(uint64(b.SeqLen*b.DModel*4), "pf.PhiQ")
	b.dPhiK = b.alloc(uint64(b.SeqLen*b.DModel*4), "pf.PhiK")
	b.dKV = b.alloc(uint64(b.NumHeads*dh*dh*4), "pf.KV")
	b.dZ = b.alloc(uint64(b.NumHeads*dh*4), "pf.Z")
	b.dContext = b.alloc(uint64(b.SeqLen*b.DModel*4), "pf.Context")
	b.dTmpOut = b.alloc(uint64(b.SeqLen*b.DModel*4), "pf.TmpOut")
	b.dFF1 = b.alloc(uint64(b.SeqLen*b.DFF*4), "pf.FF1")
	b.driver.MemCopyH2D(b.context, b.dX, b.hostX)
	wMatBytes := uint64(b.DModel * b.DModel * 4)
	w1Bytes := uint64(b.DModel * b.DFF * 4)
	w2Bytes := uint64(b.DFF * b.DModel * 4)
	normBytes := uint64(b.DModel * 4)
	for l := 0; l < b.Layers; l++ {
		b.dWq[l] = b.alloc(wMatBytes, fmt.Sprintf("pf.L%d.Wq", l))
		b.dWk[l] = b.alloc(wMatBytes, fmt.Sprintf("pf.L%d.Wk", l))
		b.dWv[l] = b.alloc(wMatBytes, fmt.Sprintf("pf.L%d.Wv", l))
		b.dWo[l] = b.alloc(wMatBytes, fmt.Sprintf("pf.L%d.Wo", l))
		b.dW1[l] = b.alloc(w1Bytes, fmt.Sprintf("pf.L%d.W1", l))
		b.dW2[l] = b.alloc(w2Bytes, fmt.Sprintf("pf.L%d.W2", l))
		b.dG1[l] = b.alloc(normBytes, fmt.Sprintf("pf.L%d.G1", l))
		b.dB1[l] = b.alloc(normBytes, fmt.Sprintf("pf.L%d.B1", l))
		b.dG2[l] = b.alloc(normBytes, fmt.Sprintf("pf.L%d.G2", l))
		b.dB2[l] = b.alloc(normBytes, fmt.Sprintf("pf.L%d.B2", l))
		for ptr, arr := range map[driver.Ptr][]float32{b.dWq[l]: b.layersHost[l].Wq, b.dWk[l]: b.layersHost[l].Wk, b.dWv[l]: b.layersHost[l].Wv, b.dWo[l]: b.layersHost[l].Wo, b.dW1[l]: b.layersHost[l].W1, b.dW2[l]: b.layersHost[l].W2, b.dG1[l]: b.layersHost[l].G1, b.dB1[l]: b.layersHost[l].B1, b.dG2[l]: b.layersHost[l].G2, b.dB2[l]: b.layersHost[l].B2} {
			b.driver.MemCopyH2D(b.context, ptr, arr)
		}
	}
}

func (b *Benchmark) exec() {
	dh := b.DModel / b.NumHeads
	cur, other := b.dX, b.dX2
	for l := 0; l < b.Layers; l++ {
		b.launch1D(b.kLN1, uint32(b.SeqLen), 64, &LayerNormArgs{In: cur, Gamma: b.dG1[l], Beta: b.dB1[l], Out: other, SeqLen: int32(b.SeqLen), DModel: int32(b.DModel), Eps: 1e-5})
		b.launch1D(b.kQProj, uint32(b.SeqLen*b.DModel), 64, &MatMulArgs{A: other, B: b.dWq[l], C: b.dQ, M: int32(b.SeqLen), N: int32(b.DModel), K: int32(b.DModel)})
		b.launch1D(b.kKProj, uint32(b.SeqLen*b.DModel), 64, &MatMulArgs{A: other, B: b.dWk[l], C: b.dK, M: int32(b.SeqLen), N: int32(b.DModel), K: int32(b.DModel)})
		b.launch1D(b.kVProj, uint32(b.SeqLen*b.DModel), 64, &MatMulArgs{A: other, B: b.dWv[l], C: b.dV, M: int32(b.SeqLen), N: int32(b.DModel), K: int32(b.DModel)})
		b.launch1D(b.kPhiQ, uint32(b.SeqLen*b.DModel), 64, &PhiArgs{In: b.dQ, Out: b.dPhiQ, N: int32(b.SeqLen * b.DModel)})
		b.launch1D(b.kPhiK, uint32(b.SeqLen*b.DModel), 64, &PhiArgs{In: b.dK, Out: b.dPhiK, N: int32(b.SeqLen * b.DModel)})
		b.launch1D(b.kKVReduce, uint32(b.NumHeads*dh*dh), 64, &KVReduceArgs{PhiK: b.dPhiK, V: b.dV, KV: b.dKV, SeqLen: int32(b.SeqLen), DHead: int32(dh), NumHeads: int32(b.NumHeads)})
		b.launch1D(b.kZReduce, uint32(b.NumHeads*dh), 64, &ZReduceArgs{PhiK: b.dPhiK, Z: b.dZ, SeqLen: int32(b.SeqLen), DHead: int32(dh), NumHeads: int32(b.NumHeads)})
		b.launch1D(b.kLinearCtx, uint32(b.SeqLen*b.DModel), 64, &LinearCtxArgs{PhiQ: b.dPhiQ, KV: b.dKV, Z: b.dZ, Context: b.dContext, SeqLen: int32(b.SeqLen), DHead: int32(dh), NumHeads: int32(b.NumHeads)})
		b.launch1D(b.kOProj, uint32(b.SeqLen*b.DModel), 64, &MatMulArgs{A: b.dContext, B: b.dWo[l], C: b.dTmpOut, M: int32(b.SeqLen), N: int32(b.DModel), K: int32(b.DModel)})
		b.launch1D(b.kAdd1, uint32(b.SeqLen*b.DModel), 64, &AddArgs{A: cur, B: b.dTmpOut, Out: other, N: int32(b.SeqLen * b.DModel)})
		cur, other = other, cur
		b.launch1D(b.kLN2, uint32(b.SeqLen), 64, &LayerNormArgs{In: cur, Gamma: b.dG2[l], Beta: b.dB2[l], Out: other, SeqLen: int32(b.SeqLen), DModel: int32(b.DModel), Eps: 1e-5})
		b.launch1D(b.kFF1, uint32(b.SeqLen*b.DFF), 64, &MatMulArgs{A: other, B: b.dW1[l], C: b.dFF1, M: int32(b.SeqLen), N: int32(b.DFF), K: int32(b.DModel)})
		b.launch1D(b.kReLU, uint32(b.SeqLen*b.DFF), 64, &ReLUArgs{X: b.dFF1, N: int32(b.SeqLen * b.DFF)})
		b.launch1D(b.kFF2, uint32(b.SeqLen*b.DModel), 64, &MatMulArgs{A: b.dFF1, B: b.dW2[l], C: b.dTmpOut, M: int32(b.SeqLen), N: int32(b.DModel), K: int32(b.DFF)})
		b.launch1D(b.kAdd2, uint32(b.SeqLen*b.DModel), 64, &AddArgs{A: cur, B: b.dTmpOut, Out: other, N: int32(b.SeqLen * b.DModel)})
		cur, other = other, cur
	}
	b.driver.MemCopyD2H(b.context, b.hostOut, cur)
}

func (b *Benchmark) Verify() {
	ref := b.cpuForward()

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

// ===== CPU reference (for -verify) =====

func (b *Benchmark) cpuMatMul(A []float32, B []float32, M, N, K int) []float32 {
	C := make([]float32, M*N)
	for row := 0; row < M; row++ {
		baseA := row * K
		baseC := row * N
		for col := 0; col < N; col++ {
			var sum float32
			for k := 0; k < K; k++ {
				sum += A[baseA+k] * B[k*N+col]
			}
			C[baseC+col] = sum
		}
	}
	return C
}

func cpuOpenCLExpF32(x float32) float32 {
	xHi := math.Float32frombits(math.Float32bits(x) & 0xfffff000)
	xLo := x - xHi

	cLo := math.Float32frombits(0x39a3b295)
	cHi := math.Float32frombits(0x3fb8a000)

	ySmall := cLo * xLo
	ySmall += cHi * xLo
	yBig := cHi * xHi
	ySmall += cLo * xHi

	y := float32(math.Exp2(float64(yBig))) * float32(math.Exp2(float64(ySmall)))

	if !(x > math.Float32frombits(0xc2aeac50)) {
		y = 0
	}
	if !(x < math.Float32frombits(0x42b17218)) {
		y = float32(math.Inf(1))
	}
	return y
}

func (b *Benchmark) cpuPhi(in []float32) []float32 {
	out := make([]float32, len(in))
	for i, x := range in {
		if x > 0 {
			out[i] = x
		} else {
			out[i] = cpuOpenCLExpF32(x)
		}
	}
	return out
}

func (b *Benchmark) cpuKVReduce(phiK, V []float32, seqLen, dHead, numHeads int) []float32 {
	kv := make([]float32, numHeads*dHead*dHead)
	dModel := numHeads * dHead
	for h := 0; h < numHeads; h++ {
		for f := 0; f < dHead; f++ {
			for d := 0; d < dHead; d++ {
				var sum float32
				for j := 0; j < seqLen; j++ {
					pk := phiK[j*dModel+h*dHead+f]
					vv := V[j*dModel+h*dHead+d]
					sum += pk * vv
				}
				kv[h*dHead*dHead+f*dHead+d] = sum
			}
		}
	}
	return kv
}

func (b *Benchmark) cpuZReduce(phiK []float32, seqLen, dHead, numHeads int) []float32 {
	z := make([]float32, numHeads*dHead)
	dModel := numHeads * dHead
	for h := 0; h < numHeads; h++ {
		for f := 0; f < dHead; f++ {
			var sum float32
			for j := 0; j < seqLen; j++ {
				sum += phiK[j*dModel+h*dHead+f]
			}
			z[h*dHead+f] = sum
		}
	}
	return z
}

func (b *Benchmark) cpuLinearContext(phiQ, kv, z []float32, seqLen, dHead, numHeads int) []float32 {
	dModel := numHeads * dHead
	ctx := make([]float32, seqLen*dModel)
	for i := 0; i < seqLen; i++ {
		for h := 0; h < numHeads; h++ {
			kvBase := h * dHead * dHead
			zBase := h * dHead
			for d := 0; d < dHead; d++ {
				var num float32
				den := float32(1e-6)
				for f := 0; f < dHead; f++ {
					pq := phiQ[i*dModel+h*dHead+f]
					num += pq * kv[kvBase+f*dHead+d]
					den += pq * z[zBase+f]
				}
				ctx[i*dModel+h*dHead+d] = num / den
			}
		}
	}
	return ctx
}

func (b *Benchmark) cpuAdd(a, c []float32) []float32 {
	out := make([]float32, len(a))
	for i := range a {
		out[i] = a[i] + c[i]
	}
	return out
}

func (b *Benchmark) cpuLayerNorm(x []float32, gamma []float32, beta []float32, seqLen, dModel int) []float32 {
	out := make([]float32, len(x))
	const eps = 1e-5
	for row := 0; row < seqLen; row++ {
		base := row * dModel
		var mean float32
		for j := 0; j < dModel; j++ {
			mean += x[base+j]
		}
		mean /= float32(dModel)

		var variance float32
		for j := 0; j < dModel; j++ {
			d := x[base+j] - mean
			variance += d * d
		}
		variance /= float32(dModel)

		inv := float32(1.0 / math.Sqrt(float64(variance+eps)))
		for j := 0; j < dModel; j++ {
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

func (b *Benchmark) cpuForward() []float32 {
	if b.DModel%b.NumHeads != 0 {
		log.Panic("DModel must divide NumHeads")
	}

	seqLen, dModel, dFF := b.SeqLen, b.DModel, b.DFF
	dHead := dModel / b.NumHeads
	cur := append([]float32(nil), b.hostX...)

	for l := 0; l < b.Layers; l++ {
		h := b.layersHost[l]

		norm1 := b.cpuLayerNorm(cur, h.G1, h.B1, seqLen, dModel)
		q := b.cpuMatMul(norm1, h.Wq, seqLen, dModel, dModel)
		k := b.cpuMatMul(norm1, h.Wk, seqLen, dModel, dModel)
		v := b.cpuMatMul(norm1, h.Wv, seqLen, dModel, dModel)

		phiQ := b.cpuPhi(q)
		phiK := b.cpuPhi(k)
		kv := b.cpuKVReduce(phiK, v, seqLen, dHead, b.NumHeads)
		z := b.cpuZReduce(phiK, seqLen, dHead, b.NumHeads)
		ctx := b.cpuLinearContext(phiQ, kv, z, seqLen, dHead, b.NumHeads)

		oProj := b.cpuMatMul(ctx, h.Wo, seqLen, dModel, dModel)
		cur = b.cpuAdd(cur, oProj)

		norm2 := b.cpuLayerNorm(cur, h.G2, h.B2, seqLen, dModel)
		ff1 := b.cpuMatMul(norm2, h.W1, seqLen, dFF, dModel)
		ff1 = b.cpuReLU(ff1)
		ff2 := b.cpuMatMul(ff1, h.W2, seqLen, dModel, dFF)
		cur = b.cpuAdd(cur, ff2)
	}

	return cur
}
