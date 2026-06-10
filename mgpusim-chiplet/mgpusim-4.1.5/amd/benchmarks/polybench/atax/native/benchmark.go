// Package atax implements the ATAX benchmark from Polybench.
package atax

import (
	"log"
	"math"
	"math/rand"

	// embed hsaco files
	_ "embed"

	"github.com/sarchlab/mgpusim/v4/amd/driver"
	"github.com/sarchlab/mgpusim/v4/amd/insts"
	"github.com/sarchlab/mgpusim/v4/amd/kernels"
)

// Kernel1Args list first set of kernel arguments
type Kernel1Args struct {
	A   driver.Ptr
	X   driver.Ptr
	Tmp driver.Ptr
	NX  int32
	NY  int32
}

// Kernel2Args list second set of kernel arguments
type Kernel2Args struct {
	A      driver.Ptr
	Y      driver.Ptr
	Tmp    driver.Ptr
	NX, NY int32
}

// Benchmark defines a benchmark
type Benchmark struct {
	driver           *driver.Driver
	context          *driver.Context
	gpus             []int
	queues           []*driver.CommandQueue
	kernel1, kernel2 *insts.HsaCo

	NX, NY int

	// LocalSizeX controls the work-group size (x dimension) used when launching the kernels.
	// It only affects performance (parallelism), not numerical correctness.
	// If not set, it defaults to 64 (matching the MGVM artifact).
	LocalSizeX int

	a, x, y, yOutput, tmp []float32
	dA, dX, dY, dTmp      driver.Ptr
	cpuY                  []float32

	useUnifiedMemory   bool
	useLASPMemoryAlloc bool
}

// NewBenchmark makes a new benchmark
func NewBenchmark(driver *driver.Driver) *Benchmark {
	b := new(Benchmark)
	b.driver = driver
	b.context = driver.Init()
	b.loadProgram()
	b.LocalSizeX = 64
	return b
}

// SelectGPU selects GPU
func (b *Benchmark) SelectGPU(gpus []int) {
	b.gpus = gpus
}

// SetUnifiedMemory uses Unified Memory
func (b *Benchmark) SetUnifiedMemory() {
	b.useUnifiedMemory = true
}

// EnableLASPMemoryAlloc requests MGVM/LASP-style explicit page placement.
//
// This is wired to the runner flag `-use-lasp-mem-alloc` for MGVM compatibility.
func (b *Benchmark) EnableLASPMemoryAlloc() {
	b.useLASPMemoryAlloc = true
}

//go:embed kernels.hsaco
var hsacoBytes []byte

func (b *Benchmark) loadProgram() {
	b.kernel1 = kernels.LoadProgramFromMemory(
		hsacoBytes, "atax_kernel1")
	if b.kernel1 == nil {
		log.Panic("Failed to load kernel binary")
	}

	b.kernel2 = kernels.LoadProgramFromMemory(
		hsacoBytes, "atax_kernel2")
	if b.kernel2 == nil {
		log.Panic("Failed to load kernel binary")
	}
}

// Run runs
func (b *Benchmark) Run() {
	for _, gpu := range b.gpus {
		b.driver.SelectGPU(b.context, gpu)
		b.queues = append(b.queues, b.driver.CreateCommandQueue(b.context))
	}

	b.initMem()
	b.exec()
}

func (b *Benchmark) initMem() {
	rand.Seed(1)
	b.a = make([]float32, b.NX*b.NY)
	b.x = make([]float32, b.NY)
	b.y = make([]float32, b.NY)
	b.yOutput = make([]float32, b.NY)
	b.tmp = make([]float32, b.NX)

	for j := 0; j < b.NY; j++ {
		b.x[j] = float32(j) * math.Pi
	}

	for i := 0; i < b.NX; i++ {
		for j := 0; j < b.NY; j++ {
			b.a[i*b.NY+j] = float32(i) * float32(j) / float32(b.NX)
		}
	}

	szA := uint64(b.NY * b.NX * 4)
	szX := uint64(b.NY * 4)
	szY := uint64(b.NY * 4)
	szTmp := uint64(b.NX * 4)

	if b.useUnifiedMemory {
		b.dA = b.driver.AllocateUnifiedMemory(b.context, szA)
		b.dX = b.driver.AllocateUnifiedMemory(b.context, szX)
		b.dY = b.driver.AllocateUnifiedMemory(b.context, szY)
		b.dTmp = b.driver.AllocateUnifiedMemory(b.context, szTmp)
		return
	}

	if b.useLASPMemoryAlloc {
		// Use the configured number of chiplets by omitting the numeric suffix.
		partition := "div"
		b.dA = b.driver.AllocateMemoryLASP(b.context, szA, partition)
		b.dX = b.driver.AllocateMemoryLASP(b.context, szX, partition)
		b.dY = b.driver.AllocateMemoryLASP(b.context, szY, partition)
		b.dTmp = b.driver.AllocateMemoryLASP(b.context, szTmp, partition)
		return
	}

	// Default: allocate with tags so that a plan JSON can control per-page chiplet placement.
	b.dA = b.driver.AllocateMemoryWithTag(b.context, szA, "A")
	b.dX = b.driver.AllocateMemoryWithTag(b.context, szX, "X")
	b.dY = b.driver.AllocateMemoryWithTag(b.context, szY, "Y")
	b.dTmp = b.driver.AllocateMemoryWithTag(b.context, szTmp, "Tmp")
}

func (b *Benchmark) exec() {
	b.driver.MemCopyH2D(b.context, b.dA, b.a)
	b.driver.MemCopyH2D(b.context, b.dX, b.x)

	localSizeX := b.LocalSizeX
	if localSizeX <= 0 {
		localSizeX = 64
	}
	if localSizeX > 65535 {
		log.Panicf("invalid localSizeX %d, must fit in uint16", localSizeX)
	}

	localSize := [3]uint16{uint16(localSizeX), 1, 1}
	globalSizeX := uint32(((b.NX-1)/localSizeX + 1) * localSizeX)
	globalSize := [3]uint32{globalSizeX, 1, 1}

	kernel1Arg := Kernel1Args{
		A:   b.dA,
		X:   b.dX,
		Tmp: b.dTmp,
		NX:  int32(b.NX),
		NY:  int32(b.NY),
	}
	b.driver.LaunchKernel(b.context, b.kernel1,
		globalSize, localSize, &kernel1Arg)

	globalSizeX = uint32(((b.NY-1)/localSizeX + 1) * localSizeX)
	globalSize = [3]uint32{globalSizeX, 1, 1}

	kernel2Arg := Kernel2Args{
		A:   b.dA,
		Y:   b.dY,
		Tmp: b.dTmp,
		NX:  int32(b.NX),
		NY:  int32(b.NY),
	}
	b.driver.LaunchKernel(b.context, b.kernel2,
		globalSize, localSize, &kernel2Arg)

	b.driver.MemCopyD2H(b.context, b.yOutput, b.dY)
}

// Verify verifies
func (b *Benchmark) Verify() {
	b.cpuAtax()

	for i := 0; i < b.NY; i++ {
		if b.cpuY[i] != b.yOutput[i] {
			log.Panicf("Mismatch at %d, expected %f, but get %f",
				i, b.cpuY[i], b.yOutput[i])
		}
	}

	log.Printf("Passed!\n")
}

func (b *Benchmark) cpuAtax() {
	b.cpuY = make([]float32, b.NY)
	tmp := make([]float32, b.NX)

	for i := 0; i < b.NY; i++ {
		b.cpuY[i] = 0
	}

	for i := 0; i < b.NX; i++ {
		tmp[i] = 0

		for j := 0; j < b.NY; j++ {
			tmp[i] += b.a[i*b.NY+j] * b.x[j]
		}

		for j := 0; j < b.NY; j++ {
			b.cpuY[j] += b.a[i*b.NY+j] * tmp[i]
		}
	}
}
