package resnet

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

type Conv2DArgs struct {
	In     driver.Ptr
	W      driver.Ptr
	B      driver.Ptr
	Out    driver.Ptr
	Cin    int32
	Hin    int32
	Win    int32
	Cout   int32
	K      int32
	Stride int32
	Pad    int32
	Hout   int32
	Wout   int32
}

type ReLUArgs struct {
	X driver.Ptr
	N int32
}
type AddReLUArgs struct {
	A   driver.Ptr
	B   driver.Ptr
	Out driver.Ptr
	N   int32
}
type GAPPartialArgs struct {
	In      driver.Ptr
	Partial driver.Ptr
	C       int32
	H       int32
	W       int32
	Tiles   int32
}
type GAPFinalizeArgs struct {
	Partial driver.Ptr
	Out     driver.Ptr
	C       int32
	Tiles   int32
	Denom   float32
}
type FCArgs struct {
	In     driver.Ptr
	W      driver.Ptr
	B      driver.Ptr
	Out    driver.Ptr
	InDim  int32
	OutDim int32
}

type blockHost struct{ W1, B1, W2, B2 []float32 }
type blockDev struct{ dW1, dB1, dW2, dB2 driver.Ptr }

type Benchmark struct {
	driver  *driver.Driver
	context *driver.Context
	gpus    []int

	kConv       *insts.HsaCo
	kReLU       *insts.HsaCo
	kAddReLU    *insts.HsaCo
	kGAPPartial *insts.HsaCo
	kGAPFinal   *insts.HsaCo
	kFC         *insts.HsaCo

	InH      int
	InW      int
	Cin      int
	C        int
	Blocks   int
	NumClass int
	K        int
	Pad      int
	GapTiles int

	BatchTotal       int
	BatchStart       int
	BatchCount       int
	BatchSeed        int64
	BatchUpdateInput bool

	hIn        []float32
	hW0        []float32
	hB0        []float32
	hWfc       []float32
	hBfc       []float32
	hOut       []float32
	blocksHost []blockHost

	dIn                            driver.Ptr
	dW0                            driver.Ptr
	dB0                            driver.Ptr
	dStem                          driver.Ptr
	dTmp1                          driver.Ptr
	dTmp2                          driver.Ptr
	dBlockOut                      driver.Ptr
	dGapPartial                    driver.Ptr
	dGAP                           driver.Ptr
	dWfc                           driver.Ptr
	dBfc                           driver.Ptr
	dOut                           driver.Ptr
	dBlkW1, dBlkB1, dBlkW2, dBlkB2 []driver.Ptr

	useUnifiedMemory bool
}

func NewBenchmark(driver *driver.Driver) *Benchmark {
	b := new(Benchmark)
	b.driver = driver
	b.context = driver.Init()
	b.InH, b.InW = 128, 128
	b.Cin = 3
	b.C = 16
	b.Blocks = 3
	b.NumClass = 10
	b.K = 3
	b.Pad = 1
	b.GapTiles = 64
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
	b.kConv = mustLoadKernel(hsacoBytes, "rn_conv2d")
	b.kReLU = mustLoadKernel(hsacoBytes, "rn_relu")
	b.kAddReLU = mustLoadKernel(hsacoBytes, "rn_add_relu")
	b.kGAPPartial = mustLoadKernel(hsacoBytes, "rn_gap_partial")
	b.kGAPFinal = mustLoadKernel(hsacoBytes, "rn_gap_finalize")
	b.kFC = mustLoadKernel(hsacoBytes, "rn_fc")
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

func (b *Benchmark) fillInputForBatch(batch int) {
	seed := b.BatchSeed + int64(batch)*1315423911
	rng := rand.New(rand.NewSource(seed))
	for i := range b.hIn {
		b.hIn[i] = (rng.Float32() - 0.5) * 0.02
	}
}

func (b *Benchmark) Run() {
	if b.Blocks <= 0 {
		log.Panic("Blocks must be > 0")
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
		b.driver.MemCopyH2D(b.context, b.dIn, b.hIn)
	}
	for bi := start; bi < end; bi++ {
		if b.BatchUpdateInput && bi != start {
			b.fillInputForBatch(bi)
			b.driver.MemCopyH2D(b.context, b.dIn, b.hIn)
		}
		b.exec()
	}
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

func (b *Benchmark) initMem() {
	// Use a local PRNG instead of math/rand's package-level generator.
	// Go 1.24 makes top-level rand.Seed a no-op by default, which can
	// otherwise make benchmark weights differ across runs.
	rng := rand.New(rand.NewSource(1))
	nIn := b.Cin * b.InH * b.InW
	b.hIn = make([]float32, nIn)
	initMatWithRNG(rng, b.hIn)
	b.hW0 = make([]float32, b.C*b.Cin*b.K*b.K)
	b.hB0 = make([]float32, b.C)
	b.hWfc = make([]float32, b.NumClass*b.C)
	b.hBfc = make([]float32, b.NumClass)
	b.hOut = make([]float32, b.NumClass)
	initMatWithRNG(rng, b.hW0)
	initMatWithRNG(rng, b.hB0)
	initMatWithRNG(rng, b.hWfc)
	initMatWithRNG(rng, b.hBfc)

	b.blocksHost = make([]blockHost, b.Blocks)
	b.dBlkW1 = make([]driver.Ptr, b.Blocks)
	b.dBlkB1 = make([]driver.Ptr, b.Blocks)
	b.dBlkW2 = make([]driver.Ptr, b.Blocks)
	b.dBlkB2 = make([]driver.Ptr, b.Blocks)
	for i := 0; i < b.Blocks; i++ {
		bh := blockHost{
			W1: make([]float32, b.C*b.C*b.K*b.K), B1: make([]float32, b.C),
			W2: make([]float32, b.C*b.C*b.K*b.K), B2: make([]float32, b.C),
		}
		initMatWithRNG(rng, bh.W1)
		initMatWithRNG(rng, bh.B1)
		initMatWithRNG(rng, bh.W2)
		initMatWithRNG(rng, bh.B2)
		b.blocksHost[i] = bh
	}

	b.dIn = b.alloc(uint64(nIn*4), "rn.Input")
	b.dW0 = b.alloc(uint64(b.C*b.Cin*b.K*b.K*4), "rn.Stem.W")
	b.dB0 = b.alloc(uint64(b.C*4), "rn.Stem.B")
	actBytes := uint64(b.C * b.InH * b.InW * 4)
	b.dStem = b.alloc(actBytes, "rn.Stem.Out")
	b.dTmp1 = b.alloc(actBytes, "rn.Blk.T1")
	b.dTmp2 = b.alloc(actBytes, "rn.Blk.T2")
	b.dBlockOut = b.alloc(actBytes, "rn.Blk.Out")
	b.dGapPartial = b.alloc(uint64(b.C*b.GapTiles*4), "rn.GAP.Partial")
	b.dGAP = b.alloc(uint64(b.C*4), "rn.GAP.Out")
	b.dWfc = b.alloc(uint64(b.NumClass*b.C*4), "rn.FC.W")
	b.dBfc = b.alloc(uint64(b.NumClass*4), "rn.FC.B")
	b.dOut = b.alloc(uint64(b.NumClass*4), "rn.FC.Out")

	blkWBytes := uint64(b.C * b.C * b.K * b.K * 4)
	blkBBytes := uint64(b.C * 4)
	for i := 0; i < b.Blocks; i++ {
		b.dBlkW1[i] = b.alloc(blkWBytes, fmt.Sprintf("rn.Blk%d.W1", i))
		b.dBlkB1[i] = b.alloc(blkBBytes, fmt.Sprintf("rn.Blk%d.B1", i))
		b.dBlkW2[i] = b.alloc(blkWBytes, fmt.Sprintf("rn.Blk%d.W2", i))
		b.dBlkB2[i] = b.alloc(blkBBytes, fmt.Sprintf("rn.Blk%d.B2", i))
		b.driver.MemCopyH2D(b.context, b.dBlkW1[i], b.blocksHost[i].W1)
		b.driver.MemCopyH2D(b.context, b.dBlkB1[i], b.blocksHost[i].B1)
		b.driver.MemCopyH2D(b.context, b.dBlkW2[i], b.blocksHost[i].W2)
		b.driver.MemCopyH2D(b.context, b.dBlkB2[i], b.blocksHost[i].B2)
	}

	b.driver.MemCopyH2D(b.context, b.dIn, b.hIn)
	b.driver.MemCopyH2D(b.context, b.dW0, b.hW0)
	b.driver.MemCopyH2D(b.context, b.dB0, b.hB0)
	b.driver.MemCopyH2D(b.context, b.dWfc, b.hWfc)
	b.driver.MemCopyH2D(b.context, b.dBfc, b.hBfc)
}

func (b *Benchmark) exec() {
	actN := uint32(b.C * b.InH * b.InW)
	b.launch2D(b.kConv, uint32(b.InW), uint32(b.C*b.InH), 8, 8, &Conv2DArgs{In: b.dIn, W: b.dW0, B: b.dB0, Out: b.dStem, Cin: int32(b.Cin), Hin: int32(b.InH), Win: int32(b.InW), Cout: int32(b.C), K: int32(b.K), Stride: 1, Pad: int32(b.Pad), Hout: int32(b.InH), Wout: int32(b.InW)})
	b.launch1D(b.kReLU, actN, 64, &ReLUArgs{X: b.dStem, N: int32(b.C * b.InH * b.InW)})

	cur := b.dStem
	for i := 0; i < b.Blocks; i++ {
		b.launch2D(b.kConv, uint32(b.InW), uint32(b.C*b.InH), 8, 8, &Conv2DArgs{In: cur, W: b.dBlkW1[i], B: b.dBlkB1[i], Out: b.dTmp1, Cin: int32(b.C), Hin: int32(b.InH), Win: int32(b.InW), Cout: int32(b.C), K: int32(b.K), Stride: 1, Pad: int32(b.Pad), Hout: int32(b.InH), Wout: int32(b.InW)})
		b.launch1D(b.kReLU, actN, 64, &ReLUArgs{X: b.dTmp1, N: int32(b.C * b.InH * b.InW)})
		b.launch2D(b.kConv, uint32(b.InW), uint32(b.C*b.InH), 8, 8, &Conv2DArgs{In: b.dTmp1, W: b.dBlkW2[i], B: b.dBlkB2[i], Out: b.dTmp2, Cin: int32(b.C), Hin: int32(b.InH), Win: int32(b.InW), Cout: int32(b.C), K: int32(b.K), Stride: 1, Pad: int32(b.Pad), Hout: int32(b.InH), Wout: int32(b.InW)})
		b.launch1D(b.kAddReLU, actN, 64, &AddReLUArgs{A: b.dTmp2, B: cur, Out: b.dBlockOut, N: int32(b.C * b.InH * b.InW)})
		cur, b.dBlockOut = b.dBlockOut, cur
	}

	// Two-stage GAP to avoid pathological single-workgroup kernels.
	b.launch2D(b.kGAPPartial, uint32(b.GapTiles), uint32(b.C), 8, 8, &GAPPartialArgs{In: cur, Partial: b.dGapPartial, C: int32(b.C), H: int32(b.InH), W: int32(b.InW), Tiles: int32(b.GapTiles)})
	b.launch1D(b.kGAPFinal, uint32(b.C), 64, &GAPFinalizeArgs{Partial: b.dGapPartial, Out: b.dGAP, C: int32(b.C), Tiles: int32(b.GapTiles), Denom: float32(b.InH * b.InW)})
	b.launch1D(b.kFC, uint32(b.NumClass), 64, &FCArgs{In: b.dGAP, W: b.dWfc, B: b.dBfc, Out: b.dOut, InDim: int32(b.C), OutDim: int32(b.NumClass)})
	b.driver.MemCopyD2H(b.context, b.hOut, b.dOut)
}

func (b *Benchmark) Verify() {
	ref := b.cpuForward()
	const atol = 2e-3
	if len(ref) != len(b.hOut) {
		log.Panicf("resnet output length mismatch: ref=%d, got=%d", len(ref), len(b.hOut))
	}
	for i := range ref {
		diff := math.Abs(float64(ref[i] - b.hOut[i]))
		if diff > atol {
			log.Panicf("Mismatch at logit %d, expected %.6f, got %.6f (diff=%.6f)",
				i, ref[i], b.hOut[i], diff)
		}
	}
	log.Printf("Passed!\n")
}

// ===== CPU reference (for optional -verify) =====

func (b *Benchmark) cpuConv2D(in, w, bias []float32, cin, hin, win, cout, kSize, stride, pad, hout, wout int) []float32 {
	out := make([]float32, cout*hout*wout)
	for co := 0; co < cout; co++ {
		for oh := 0; oh < hout; oh++ {
			for ow := 0; ow < wout; ow++ {
				sum := bias[co]
				for ci := 0; ci < cin; ci++ {
					for kh := 0; kh < kSize; kh++ {
						for kw := 0; kw < kSize; kw++ {
							ih := oh*stride + kh - pad
							iw := ow*stride + kw - pad
							if ih < 0 || ih >= hin || iw < 0 || iw >= win {
								continue
							}
							inIdx := (ci*hin+ih)*win + iw
							wIdx := ((co*cin+ci)*kSize+kh)*kSize + kw
							sum += in[inIdx] * w[wIdx]
						}
					}
				}
				out[(co*hout+oh)*wout+ow] = sum
			}
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

func (b *Benchmark) cpuAddReLU(a, c []float32) []float32 {
	out := make([]float32, len(a))
	for i := range a {
		v := a[i] + c[i]
		if v < 0 {
			v = 0
		}
		out[i] = v
	}
	return out
}

func (b *Benchmark) cpuGAP(in []float32) []float32 {
	partial := make([]float32, b.C*b.GapTiles)
	hw := b.InH * b.InW
	for c := 0; c < b.C; c++ {
		base := c * hw
		for t := 0; t < b.GapTiles; t++ {
			start := (t * hw) / b.GapTiles
			end := ((t + 1) * hw) / b.GapTiles
			var sum float32
			for i := start; i < end; i++ {
				sum += in[base+i]
			}
			partial[c*b.GapTiles+t] = sum
		}
	}

	out := make([]float32, b.C)
	denom := float32(hw)
	for c := 0; c < b.C; c++ {
		var sum float32
		base := c * b.GapTiles
		for t := 0; t < b.GapTiles; t++ {
			sum += partial[base+t]
		}
		out[c] = sum / denom
	}
	return out
}

func (b *Benchmark) cpuFC(in, w, bias []float32) []float32 {
	out := make([]float32, b.NumClass)
	for o := 0; o < b.NumClass; o++ {
		sum := bias[o]
		base := o * b.C
		for i := 0; i < b.C; i++ {
			sum += w[base+i] * in[i]
		}
		out[o] = sum
	}
	return out
}

func (b *Benchmark) cpuForward() []float32 {
	cur := b.cpuConv2D(b.hIn, b.hW0, b.hB0, b.Cin, b.InH, b.InW, b.C, b.K, 1, b.Pad, b.InH, b.InW)
	cur = b.cpuReLU(cur)

	for i := 0; i < b.Blocks; i++ {
		bh := b.blocksHost[i]
		tmp1 := b.cpuConv2D(cur, bh.W1, bh.B1, b.C, b.InH, b.InW, b.C, b.K, 1, b.Pad, b.InH, b.InW)
		tmp1 = b.cpuReLU(tmp1)
		tmp2 := b.cpuConv2D(tmp1, bh.W2, bh.B2, b.C, b.InH, b.InW, b.C, b.K, 1, b.Pad, b.InH, b.InW)
		cur = b.cpuAddReLU(tmp2, cur)
	}

	gap := b.cpuGAP(cur)
	return b.cpuFC(gap, b.hWfc, b.hBfc)
}
