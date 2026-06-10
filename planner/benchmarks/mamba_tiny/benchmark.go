package mamba_tiny

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
type RMSNormArgs struct {
	In         driver.Ptr
	Weight     driver.Ptr
	Out        driver.Ptr
	SeqLen     int32
	DModel     int32
	InvDModelQ int32
	Eps        float32
}
type Conv1DArgs struct {
	In     driver.Ptr
	Weight driver.Ptr
	Out    driver.Ptr
	SeqLen int32
	DModel int32
	K      int32
}
type ScanArgs struct {
	In     driver.Ptr
	A      driver.Ptr
	B      driver.Ptr
	C      driver.Ptr
	Delta  driver.Ptr
	Out    driver.Ptr
	SeqLen int32
	DModel int32
}
type GateArgs struct {
	X   driver.Ptr
	Z   driver.Ptr
	Out driver.Ptr
	N   int32
}
type AddArgs struct {
	A   driver.Ptr
	B   driver.Ptr
	Out driver.Ptr
	N   int32
}

type layerHost struct{ WinX, WinZ, Wout, ConvW, A, B, C, Delta, Norm []float32 }
type layerDev struct{ dWinX, dWinZ, dWout, dConvW, dA, dB, dC, dDelta, dNorm driver.Ptr }

type Benchmark struct {
	driver  *driver.Driver
	context *driver.Context
	gpus    []int

	kRMSNorm           *insts.HsaCo
	kInProjX, kInProjZ *insts.HsaCo
	kConv1D            *insts.HsaCo
	kScan              *insts.HsaCo
	kGate              *insts.HsaCo
	kOutProj           *insts.HsaCo
	kAdd               *insts.HsaCo

	SeqLen     int
	DModel     int
	Layers     int
	ConvK      int
	InvDModelQ int

	BatchTotal       int
	BatchStart       int
	BatchCount       int
	BatchSeed        int64
	BatchUpdateInput bool

	hostX      []float32
	hostOut    []float32
	layersHost []layerHost

	dX, dX2                                         driver.Ptr
	dNorm, dProjX, dProjZ, dConv, dScanOut, dTmpOut driver.Ptr
	dWinX, dWinZ, dWout, dConvW                     []driver.Ptr
	dA, dB, dC, dDelta, dLayerNorm                  []driver.Ptr
	useUnifiedMemory                                bool
}

func NewBenchmark(driver *driver.Driver) *Benchmark {
	b := new(Benchmark)
	b.driver = driver
	b.context = driver.Init()
	b.SeqLen = 256
	b.DModel = 128
	b.Layers = 4
	b.ConvK = 3
	b.refreshDerivedParams()
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
	b.kRMSNorm = mustLoadKernel(hsacoBytes, "mb_rmsnorm")
	b.kInProjX = mustLoadKernel(hsacoBytes, "mb_inproj_x")
	b.kInProjZ = mustLoadKernel(hsacoBytes, "mb_inproj_z")
	b.kConv1D = mustLoadKernel(hsacoBytes, "mb_conv1d")
	b.kScan = mustLoadKernel(hsacoBytes, "mb_selective_scan")
	b.kGate = mustLoadKernel(hsacoBytes, "mb_gate_mul")
	b.kOutProj = mustLoadKernel(hsacoBytes, "mb_outproj")
	b.kAdd = mustLoadKernel(hsacoBytes, "mb_add_resid")
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
func initPos(x []float32) {
	for i := range x {
		x[i] = 0.1 + rand.Float32()*0.02
	}
}
func initNorm(x []float32) {
	for i := range x {
		x[i] = 1.0
	}
}
func (b *Benchmark) fillInputForBatch(batch int) {
	seed := b.BatchSeed + int64(batch)*1315423911
	rng := rand.New(rand.NewSource(seed))
	for i := range b.hostX {
		b.hostX[i] = (rng.Float32() - 0.5) * 0.02
	}
}

func (b *Benchmark) refreshDerivedParams() {
	if b.DModel <= 0 {
		b.DModel = 128
	}
	q := (1000000 + b.DModel/2) / b.DModel
	if q <= 0 {
		q = 7813
	}
	b.InvDModelQ = q
}

func (b *Benchmark) Run() {
	b.refreshDerivedParams()
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
		h := layerHost{WinX: make([]float32, b.DModel*b.DModel), WinZ: make([]float32, b.DModel*b.DModel), Wout: make([]float32, b.DModel*b.DModel), ConvW: make([]float32, b.DModel*b.ConvK), A: make([]float32, b.DModel), B: make([]float32, b.DModel), C: make([]float32, b.DModel), Delta: make([]float32, b.DModel), Norm: make([]float32, b.DModel)}
		initMat(h.WinX)
		initMat(h.WinZ)
		initMat(h.Wout)
		initMat(h.ConvW)
		initPos(h.A)
		initPos(h.B)
		initPos(h.C)
		initPos(h.Delta)
		initNorm(h.Norm)
		b.layersHost[l] = h
	}
	b.dWinX = make([]driver.Ptr, b.Layers)
	b.dWinZ = make([]driver.Ptr, b.Layers)
	b.dWout = make([]driver.Ptr, b.Layers)
	b.dConvW = make([]driver.Ptr, b.Layers)
	b.dA = make([]driver.Ptr, b.Layers)
	b.dB = make([]driver.Ptr, b.Layers)
	b.dC = make([]driver.Ptr, b.Layers)
	b.dDelta = make([]driver.Ptr, b.Layers)
	b.dLayerNorm = make([]driver.Ptr, b.Layers)
	b.dX = b.alloc(uint64(b.SeqLen*b.DModel*4), "mb.X")
	b.dX2 = b.alloc(uint64(b.SeqLen*b.DModel*4), "mb.X2")
	b.dNorm = b.alloc(uint64(b.SeqLen*b.DModel*4), "mb.Norm")
	b.dProjX = b.alloc(uint64(b.SeqLen*b.DModel*4), "mb.ProjX")
	b.dProjZ = b.alloc(uint64(b.SeqLen*b.DModel*4), "mb.ProjZ")
	b.dConv = b.alloc(uint64(b.SeqLen*b.DModel*4), "mb.Conv")
	b.dScanOut = b.alloc(uint64(b.SeqLen*b.DModel*4), "mb.Scan")
	b.dTmpOut = b.alloc(uint64(b.SeqLen*b.DModel*4), "mb.TmpOut")
	b.driver.MemCopyH2D(b.context, b.dX, b.hostX)
	wMatBytes := uint64(b.DModel * b.DModel * 4)
	convBytes := uint64(b.DModel * b.ConvK * 4)
	vecBytes := uint64(b.DModel * 4)
	for l := 0; l < b.Layers; l++ {
		b.dWinX[l] = b.alloc(wMatBytes, fmt.Sprintf("mb.L%d.WinX", l))
		b.dWinZ[l] = b.alloc(wMatBytes, fmt.Sprintf("mb.L%d.WinZ", l))
		b.dWout[l] = b.alloc(wMatBytes, fmt.Sprintf("mb.L%d.Wout", l))
		b.dConvW[l] = b.alloc(convBytes, fmt.Sprintf("mb.L%d.ConvW", l))
		b.dA[l] = b.alloc(vecBytes, fmt.Sprintf("mb.L%d.A", l))
		b.dB[l] = b.alloc(vecBytes, fmt.Sprintf("mb.L%d.B", l))
		b.dC[l] = b.alloc(vecBytes, fmt.Sprintf("mb.L%d.C", l))
		b.dDelta[l] = b.alloc(vecBytes, fmt.Sprintf("mb.L%d.Delta", l))
		b.dLayerNorm[l] = b.alloc(vecBytes, fmt.Sprintf("mb.L%d.Norm", l))
		for ptr, arr := range map[driver.Ptr][]float32{b.dWinX[l]: b.layersHost[l].WinX, b.dWinZ[l]: b.layersHost[l].WinZ, b.dWout[l]: b.layersHost[l].Wout, b.dConvW[l]: b.layersHost[l].ConvW, b.dA[l]: b.layersHost[l].A, b.dB[l]: b.layersHost[l].B, b.dC[l]: b.layersHost[l].C, b.dDelta[l]: b.layersHost[l].Delta, b.dLayerNorm[l]: b.layersHost[l].Norm} {
			b.driver.MemCopyH2D(b.context, ptr, arr)
		}
	}
}

func (b *Benchmark) exec() {
	cur, other := b.dX, b.dX2
	for l := 0; l < b.Layers; l++ {
		b.launch1D(b.kRMSNorm, uint32(b.SeqLen), 64, &RMSNormArgs{In: cur, Weight: b.dLayerNorm[l], Out: b.dNorm, SeqLen: int32(b.SeqLen), DModel: int32(b.DModel), InvDModelQ: int32(b.InvDModelQ), Eps: 1e-5})
		b.launch2D(b.kInProjX, uint32(b.DModel), uint32(b.SeqLen), 8, 8, &MatMulArgs{A: b.dNorm, B: b.dWinX[l], C: b.dProjX, M: int32(b.SeqLen), N: int32(b.DModel), K: int32(b.DModel)})
		b.launch2D(b.kInProjZ, uint32(b.DModel), uint32(b.SeqLen), 8, 8, &MatMulArgs{A: b.dNorm, B: b.dWinZ[l], C: b.dProjZ, M: int32(b.SeqLen), N: int32(b.DModel), K: int32(b.DModel)})
		b.launch2D(b.kConv1D, uint32(b.DModel), uint32(b.SeqLen), 8, 8, &Conv1DArgs{In: b.dProjX, Weight: b.dConvW[l], Out: b.dConv, SeqLen: int32(b.SeqLen), DModel: int32(b.DModel), K: int32(b.ConvK)})
		b.launch1D(b.kScan, uint32(b.DModel), 64, &ScanArgs{In: b.dConv, A: b.dA[l], B: b.dB[l], C: b.dC[l], Delta: b.dDelta[l], Out: b.dScanOut, SeqLen: int32(b.SeqLen), DModel: int32(b.DModel)})
		b.launch1D(b.kGate, uint32(b.SeqLen*b.DModel), 64, &GateArgs{X: b.dScanOut, Z: b.dProjZ, Out: b.dTmpOut, N: int32(b.SeqLen * b.DModel)})
		b.launch2D(b.kOutProj, uint32(b.DModel), uint32(b.SeqLen), 8, 8, &MatMulArgs{A: b.dTmpOut, B: b.dWout[l], C: other, M: int32(b.SeqLen), N: int32(b.DModel), K: int32(b.DModel)})
		b.launch1D(b.kAdd, uint32(b.SeqLen*b.DModel), 64, &AddArgs{A: cur, B: other, Out: other, N: int32(b.SeqLen * b.DModel)})
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

func mbFastRsqrtCPU(x float32) float32 {
	if x <= 0 {
		return 0
	}
	xhalf := float32(0.5) * x
	i := math.Float32bits(x)
	i = uint32(0x5f3759df) - (i >> 1)
	y := math.Float32frombits(i)
	y = y * (float32(1.5) - xhalf*y*y)
	return y
}

func mbHardSigmoidCPU(x float32) float32 {
	y := float32(0.2)*x + float32(0.5)
	if y < 0 {
		y = 0
	}
	if y > 1 {
		y = 1
	}
	return y
}

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

func (b *Benchmark) cpuRMSNorm(in []float32, weight []float32, seqLen, dModel int) []float32 {
	out := make([]float32, len(in))
	const eps = 1e-5
	invCount := float32(b.InvDModelQ) * float32(0.000001)

	for row := 0; row < seqLen; row++ {
		base := row * dModel
		var mean2 float32
		for j := 0; j < dModel; j++ {
			v := in[base+j]
			mean2 += v * v
		}
		mean2 *= invCount

		inv := mbFastRsqrtCPU(mean2 + eps)
		for j := 0; j < dModel; j++ {
			out[base+j] = in[base+j] * inv * weight[j]
		}
	}
	return out
}

func (b *Benchmark) cpuConv1D(in []float32, weight []float32, seqLen, dModel, kSize int) []float32 {
	out := make([]float32, seqLen*dModel)
	for t := 0; t < seqLen; t++ {
		for d := 0; d < dModel; d++ {
			var sum float32
			for k := 0; k < kSize; k++ {
				tt := t - k
				if tt < 0 {
					continue
				}
				sum += in[tt*dModel+d] * weight[d*kSize+k]
			}
			out[t*dModel+d] = sum
		}
	}
	return out
}

func (b *Benchmark) cpuSelectiveScan(in, aVec, bVec, cVec, delta []float32, seqLen, dModel int) []float32 {
	out := make([]float32, seqLen*dModel)
	for d := 0; d < dModel; d++ {
		state := float32(0)
		a := aVec[d]
		bb := bVec[d]
		c := cVec[d]
		del := delta[d]
		for t := 0; t < seqLen; t++ {
			x := in[t*dModel+d]
			state = state + del*(-a*state+bb*x)
			out[t*dModel+d] = c * state
		}
	}
	return out
}

func (b *Benchmark) cpuGateMul(x, z []float32) []float32 {
	out := make([]float32, len(x))
	for i := range x {
		sig := mbHardSigmoidCPU(z[i])
		out[i] = x[i] * (z[i] * sig)
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

func (b *Benchmark) cpuForward() []float32 {
	b.refreshDerivedParams()

	seqLen, dModel := b.SeqLen, b.DModel
	cur := append([]float32(nil), b.hostX...)

	for l := 0; l < b.Layers; l++ {
		h := b.layersHost[l]

		norm := b.cpuRMSNorm(cur, h.Norm, seqLen, dModel)
		projX := b.cpuMatMul(norm, h.WinX, seqLen, dModel, dModel)
		projZ := b.cpuMatMul(norm, h.WinZ, seqLen, dModel, dModel)
		conv := b.cpuConv1D(projX, h.ConvW, seqLen, dModel, b.ConvK)
		scan := b.cpuSelectiveScan(conv, h.A, h.B, h.C, h.Delta, seqLen, dModel)
		gated := b.cpuGateMul(scan, projZ)
		out := b.cpuMatMul(gated, h.Wout, seqLen, dModel, dModel)
		cur = b.cpuAdd(cur, out)
	}

	return cur
}
