// Package lenet implements a tiny LeNet-style CNN forward-pass benchmark for MGPUSim.
//
// Model (forward only, batch=1):
//
//	Input: [Cin, H, W]  (default Cin=1, H=W=28)
//	Conv1 (K=5) -> ReLU -> MaxPool2x2
//	Conv2 (K=5) -> ReLU -> MaxPool2x2
//	FC1 -> ReLU
//	FC2 (logits)
//
// Notes:
//   - Each layer is a separate kernel to facilitate your kernel/page planning.
//   - Kernels are intentionally simple (no tiling) for correctness and locality visibility.
//   - Build:
//     cd amd/benchmarks/cnn/lenet && make        (generates kernels.hsaco)
//     cd amd/samples/lenet && go build
package lenet

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

type MaxPool2DArgs struct {
	In     driver.Ptr
	Out    driver.Ptr
	C      int32
	Hin    int32
	Win    int32
	Pool   int32
	Stride int32
	Hout   int32
	Wout   int32
}

type FCArgs struct {
	In     driver.Ptr
	W      driver.Ptr
	B      driver.Ptr
	Out    driver.Ptr
	InDim  int32
	OutDim int32
}

type Benchmark struct {
	driver  *driver.Driver
	context *driver.Context
	gpus    []int

	// kernels
	kConv *insts.HsaCo
	kReLU *insts.HsaCo
	kPool *insts.HsaCo
	kFC   *insts.HsaCo

	// model sizes (tunable)
	InH int
	InW int
	Cin int

	Conv1Out int
	Conv2Out int
	FC1Out   int
	NumClass int

	// Batch inference controls (default: batch=1)
	//
	// This enables the "offline two-stage" flow:
	//   pass1: run first X% batches with profiling -> dump profile
	//   pass2: replan using profile -> run remaining batches
	//
	// Batches are independent (inference), so we do NOT need simulator checkpointing.
	BatchTotal       int
	BatchStart       int
	BatchCount       int
	BatchSeed        int64
	BatchUpdateInput bool

	// fixed layer params
	K1 int
	K2 int

	// derived sizes
	H1   int
	W1   int
	H1p  int
	W1p  int
	H2   int
	W2   int
	H2p  int
	W2p  int
	Flat int

	// host buffers
	hIn   []float32
	hW1   []float32
	hB1   []float32
	hW2   []float32
	hB2   []float32
	hWfc1 []float32
	hBfc1 []float32
	hWfc2 []float32
	hBfc2 []float32
	hOut  []float32

	// device buffers
	dIn    driver.Ptr
	dW1    driver.Ptr
	dB1    driver.Ptr
	dConv1 driver.Ptr
	dPool1 driver.Ptr

	dW2    driver.Ptr
	dB2    driver.Ptr
	dConv2 driver.Ptr
	dPool2 driver.Ptr

	dWfc1 driver.Ptr
	dBfc1 driver.Ptr
	dFC1  driver.Ptr

	dWfc2 driver.Ptr
	dBfc2 driver.Ptr
	dOut  driver.Ptr

	useUnifiedMemory bool
}

func NewBenchmark(driver *driver.Driver) *Benchmark {
	b := new(Benchmark)
	b.driver = driver
	b.context = driver.Init()

	// defaults: small and sim-friendly
	b.InH, b.InW = 28, 28
	b.Cin = 1
	b.Conv1Out = 4
	b.Conv2Out = 8
	b.FC1Out = 32
	b.NumClass = 10
	b.K1, b.K2 = 5, 5

	// batch defaults
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
	b.kConv = mustLoadKernel(hsacoBytes, "ln_conv2d")
	b.kReLU = mustLoadKernel(hsacoBytes, "ln_relu")
	b.kPool = mustLoadKernel(hsacoBytes, "ln_maxpool2d")
	b.kFC = mustLoadKernel(hsacoBytes, "ln_fc")
}

func (b *Benchmark) alloc(bytes uint64, tag string) driver.Ptr {
	if b.useUnifiedMemory {
		return b.driver.AllocateUnifiedMemory(b.context, bytes)
	}
	return b.driver.AllocateMemoryWithTag(b.context, bytes, tag)
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

func (b *Benchmark) deriveShapes() {
	// Conv1: no padding, stride=1
	b.H1 = b.InH - b.K1 + 1
	b.W1 = b.InW - b.K1 + 1
	// Pool1: 2x2 stride2
	b.H1p = b.H1 / 2
	b.W1p = b.W1 / 2
	// Conv2: no padding
	b.H2 = b.H1p - b.K2 + 1
	b.W2 = b.W1p - b.K2 + 1
	// Pool2
	b.H2p = b.H2 / 2
	b.W2p = b.W2 / 2
	b.Flat = b.Conv2Out * b.H2p * b.W2p

	if b.H2p <= 0 || b.W2p <= 0 {
		log.Panicf("Invalid shapes: got H2p=%d W2p=%d. Reduce K or increase input size.", b.H2p, b.W2p)
	}
}

func (b *Benchmark) Run() {
	for _, gpu := range b.gpus {
		b.driver.SelectGPU(b.context, gpu)
		b.driver.CreateCommandQueue(b.context) // keep API consistent with other benches
	}
	b.deriveShapes()
	b.initMem()

	// Batch loop
	start := b.BatchStart
	if start < 0 {
		start = 0
	}
	total := b.BatchTotal
	if total <= 0 {
		// Treat invalid total as "just run one batch".
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

func (b *Benchmark) fillInputForBatch(batch int) {
	// Deterministic per-batch input generation.
	// NOTE: Input values do not affect access pattern, but this helps realism.
	seed := b.BatchSeed + int64(batch)*1315423911
	rng := rand.New(rand.NewSource(seed))
	for i := range b.hIn {
		b.hIn[i] = (rng.Float32() - 0.5) * 0.02
	}
}

func (b *Benchmark) initMem() {
	rand.Seed(1)

	// For mgvm-host-analyzer friendliness: re-compute derived shapes locally using
	// only config params + constants, so buffer sizes and launch sizes are statically evaluable.
	k1 := 5
	k2 := 5
	h1 := b.InH - k1 + 1
	w1 := b.InW - k1 + 1
	h1p := h1 / 2
	w1p := w1 / 2
	h2 := h1p - k2 + 1
	w2 := w1p - k2 + 1
	h2p := h2 / 2
	w2p := w2 / 2
	flat := b.Conv2Out * h2p * w2p

	// Keep derived fields in sync (helps both runtime and analyzers).
	b.H1, b.W1, b.H1p, b.W1p = h1, w1, h1p, w1p
	b.H2, b.W2, b.H2p, b.W2p = h2, w2, h2p, w2p
	b.Flat = flat

	// host tensors
	b.hIn = make([]float32, b.Cin*b.InH*b.InW)
	for i := range b.hIn {
		b.hIn[i] = (rand.Float32() - 0.5) * 0.02
	}

	// weights
	w1Size := b.Conv1Out * b.Cin * k1 * k1
	b.hW1 = make([]float32, w1Size)
	b.hB1 = make([]float32, b.Conv1Out)

	w2Size := b.Conv2Out * b.Conv1Out * k2 * k2
	b.hW2 = make([]float32, w2Size)
	b.hB2 = make([]float32, b.Conv2Out)

	b.hWfc1 = make([]float32, b.FC1Out*flat)
	b.hBfc1 = make([]float32, b.FC1Out)

	b.hWfc2 = make([]float32, b.NumClass*b.FC1Out)
	b.hBfc2 = make([]float32, b.NumClass)

	initMat := func(x []float32) {
		for i := range x {
			x[i] = (rand.Float32() - 0.5) * 0.02
		}
	}
	initMat(b.hW1)
	initMat(b.hB1)
	initMat(b.hW2)
	initMat(b.hB2)
	initMat(b.hWfc1)
	initMat(b.hBfc1)
	initMat(b.hWfc2)
	initMat(b.hBfc2)

	b.hOut = make([]float32, b.NumClass)

	// device alloc
	b.dIn = b.alloc(uint64(b.Cin*b.InH*b.InW*4), "ln.Input")
	b.dW1 = b.alloc(uint64(w1Size*4), "ln.Conv1.W")
	b.dB1 = b.alloc(uint64(b.Conv1Out*4), "ln.Conv1.B")
	b.dConv1 = b.alloc(uint64(b.Conv1Out*h1*w1*4), "ln.Conv1.Out")
	b.dPool1 = b.alloc(uint64(b.Conv1Out*h1p*w1p*4), "ln.Pool1.Out")

	b.dW2 = b.alloc(uint64(w2Size*4), "ln.Conv2.W")
	b.dB2 = b.alloc(uint64(b.Conv2Out*4), "ln.Conv2.B")
	b.dConv2 = b.alloc(uint64(b.Conv2Out*h2*w2*4), "ln.Conv2.Out")
	b.dPool2 = b.alloc(uint64(b.Conv2Out*h2p*w2p*4), "ln.Pool2.Out")

	b.dWfc1 = b.alloc(uint64(b.FC1Out*flat*4), "ln.FC1.W")
	b.dBfc1 = b.alloc(uint64(b.FC1Out*4), "ln.FC1.B")
	b.dFC1 = b.alloc(uint64(b.FC1Out*4), "ln.FC1.Out")

	b.dWfc2 = b.alloc(uint64(b.NumClass*b.FC1Out*4), "ln.FC2.W")
	b.dBfc2 = b.alloc(uint64(b.NumClass*4), "ln.FC2.B")
	b.dOut = b.alloc(uint64(b.NumClass*4), "ln.FC2.Out")
	// copies
	b.driver.MemCopyH2D(b.context, b.dIn, b.hIn)
	b.driver.MemCopyH2D(b.context, b.dW1, b.hW1)
	b.driver.MemCopyH2D(b.context, b.dB1, b.hB1)
	b.driver.MemCopyH2D(b.context, b.dW2, b.hW2)
	b.driver.MemCopyH2D(b.context, b.dB2, b.hB2)
	b.driver.MemCopyH2D(b.context, b.dWfc1, b.hWfc1)
	b.driver.MemCopyH2D(b.context, b.dBfc1, b.hBfc1)
	b.driver.MemCopyH2D(b.context, b.dWfc2, b.hWfc2)
	b.driver.MemCopyH2D(b.context, b.dBfc2, b.hBfc2)
}

func (b *Benchmark) exec() {
	// Re-compute derived shapes locally (mgvm-host-analyzer needs these as local constants).
	k1 := 5
	k2 := 5
	h1 := b.InH - k1 + 1
	w1 := b.InW - k1 + 1
	h1p := h1 / 2
	w1p := w1 / 2
	h2 := h1p - k2 + 1
	w2 := w1p - k2 + 1
	h2p := h2 / 2
	w2p := w2 / 2
	flat := b.Conv2Out * h2p * w2p

	// Conv1
	b.launch2D(b.kConv, uint32(w1), uint32(b.Conv1Out*h1), 8, 8, &Conv2DArgs{
		In: b.dIn, W: b.dW1, B: b.dB1, Out: b.dConv1,
		Cin: int32(b.Cin), Hin: int32(b.InH), Win: int32(b.InW),
		Cout: int32(b.Conv1Out), K: int32(k1), Stride: 1, Pad: 0,
		Hout: int32(h1), Wout: int32(w1),
	})
	// ReLU
	b.launch1D(b.kReLU, uint32(b.Conv1Out*h1*w1), 64, &ReLUArgs{
		X: b.dConv1, N: int32(b.Conv1Out * h1 * w1),
	})
	// Pool1
	b.launch2D(b.kPool, uint32(w1p), uint32(b.Conv1Out*h1p), 8, 8, &MaxPool2DArgs{
		In: b.dConv1, Out: b.dPool1, C: int32(b.Conv1Out),
		Hin: int32(h1), Win: int32(w1), Pool: 2, Stride: 2,
		Hout: int32(h1p), Wout: int32(w1p),
	})

	// Conv2
	b.launch2D(b.kConv, uint32(w2), uint32(b.Conv2Out*h2), 8, 8, &Conv2DArgs{
		In: b.dPool1, W: b.dW2, B: b.dB2, Out: b.dConv2,
		Cin: int32(b.Conv1Out), Hin: int32(h1p), Win: int32(w1p),
		Cout: int32(b.Conv2Out), K: int32(k2), Stride: 1, Pad: 0,
		Hout: int32(h2), Wout: int32(w2),
	})
	// ReLU
	b.launch1D(b.kReLU, uint32(b.Conv2Out*h2*w2), 64, &ReLUArgs{
		X: b.dConv2, N: int32(b.Conv2Out * h2 * w2),
	})
	// Pool2
	b.launch2D(b.kPool, uint32(w2p), uint32(b.Conv2Out*h2p), 8, 8, &MaxPool2DArgs{
		In: b.dConv2, Out: b.dPool2, C: int32(b.Conv2Out),
		Hin: int32(h2), Win: int32(w2), Pool: 2, Stride: 2,
		Hout: int32(h2p), Wout: int32(w2p),
	})

	// FC1
	b.launch1D(b.kFC, uint32(b.FC1Out), 64, &FCArgs{
		In: b.dPool2, W: b.dWfc1, B: b.dBfc1, Out: b.dFC1,
		InDim: int32(flat), OutDim: int32(b.FC1Out),
	})
	// ReLU
	b.launch1D(b.kReLU, uint32(b.FC1Out), 64, &ReLUArgs{
		X: b.dFC1, N: int32(b.FC1Out),
	})
	// FC2
	b.launch1D(b.kFC, uint32(b.NumClass), 64, &FCArgs{
		In: b.dFC1, W: b.dWfc2, B: b.dBfc2, Out: b.dOut,
		InDim: int32(b.FC1Out), OutDim: int32(b.NumClass),
	})

	// output
	b.driver.MemCopyD2H(b.context, b.hOut, b.dOut)
}
func (b *Benchmark) Verify() {
	ref := b.cpuForward()

	const atol = 1e-3
	for i := range ref {
		diff := math.Abs(float64(ref[i] - b.hOut[i]))
		if diff > atol {
			log.Panicf("Mismatch at %d, expected %.6f, got %.6f (diff=%.6f)",
				i, ref[i], b.hOut[i], diff)
		}
	}
	log.Printf("Passed!\n")
}

// ===== CPU reference =====

func relu(x float32) float32 {
	if x < 0 {
		return 0
	}
	return x
}

func (b *Benchmark) cpuConv2D(in []float32, w []float32, bias []float32,
	cin, hin, win, cout, k, stride, pad, hout, wout int) []float32 {

	out := make([]float32, cout*hout*wout)
	for co := 0; co < cout; co++ {
		for oh := 0; oh < hout; oh++ {
			for ow := 0; ow < wout; ow++ {
				sum := bias[co]
				for ci := 0; ci < cin; ci++ {
					for kh := 0; kh < k; kh++ {
						for kw := 0; kw < k; kw++ {
							ih := oh*stride + kh - pad
							iw := ow*stride + kw - pad
							if ih < 0 || ih >= hin || iw < 0 || iw >= win {
								continue
							}
							inIdx := (ci*hin+ih)*win + iw
							wIdx := ((co*cin+ci)*k+kh)*k + kw
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

func (b *Benchmark) cpuMaxPool2D(in []float32, c, hin, win, pool, stride, hout, wout int) []float32 {
	out := make([]float32, c*hout*wout)
	for ch := 0; ch < c; ch++ {
		for oh := 0; oh < hout; oh++ {
			for ow := 0; ow < wout; ow++ {
				maxv := float32(-1e30)
				for ph := 0; ph < pool; ph++ {
					for pw := 0; pw < pool; pw++ {
						ih := oh*stride + ph
						iw := ow*stride + pw
						if ih >= hin || iw >= win {
							continue
						}
						v := in[(ch*hin+ih)*win+iw]
						if v > maxv {
							maxv = v
						}
					}
				}
				out[(ch*hout+oh)*wout+ow] = maxv
			}
		}
	}
	return out
}

func (b *Benchmark) cpuFC(in []float32, w []float32, bias []float32, outDim, inDim int) []float32 {
	out := make([]float32, outDim)
	for o := 0; o < outDim; o++ {
		sum := bias[o]
		base := o * inDim
		for i := 0; i < inDim; i++ {
			sum += w[base+i] * in[i]
		}
		out[o] = sum
	}
	return out
}

func (b *Benchmark) cpuForward() []float32 {
	// conv1
	x1 := b.cpuConv2D(b.hIn, b.hW1, b.hB1, b.Cin, b.InH, b.InW, b.Conv1Out, b.K1, 1, 0, b.H1, b.W1)
	for i := range x1 {
		x1[i] = relu(x1[i])
	}
	p1 := b.cpuMaxPool2D(x1, b.Conv1Out, b.H1, b.W1, 2, 2, b.H1p, b.W1p)

	// conv2
	x2 := b.cpuConv2D(p1, b.hW2, b.hB2, b.Conv1Out, b.H1p, b.W1p, b.Conv2Out, b.K2, 1, 0, b.H2, b.W2)
	for i := range x2 {
		x2[i] = relu(x2[i])
	}
	p2 := b.cpuMaxPool2D(x2, b.Conv2Out, b.H2, b.W2, 2, 2, b.H2p, b.W2p)

	// fc1
	fc1 := b.cpuFC(p2, b.hWfc1, b.hBfc1, b.FC1Out, b.Flat)
	for i := range fc1 {
		fc1[i] = relu(fc1[i])
	}
	// fc2
	fc2 := b.cpuFC(fc1, b.hWfc2, b.hBfc2, b.NumClass, b.FC1Out)

	return fc2
}

func (b *Benchmark) String() string {
	return fmt.Sprintf("lenet(In=%dx%dx%d C1=%d C2=%d FC1=%d Class=%d)",
		b.Cin, b.InH, b.InW, b.Conv1Out, b.Conv2Out, b.FC1Out, b.NumClass)
}
