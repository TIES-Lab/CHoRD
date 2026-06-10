// Package timingconfig contains the configuration for the timing simulation.
package timingconfig

import (
	"fmt"

	"github.com/sarchlab/akita/v4/mem/mem"
	"github.com/sarchlab/akita/v4/mem/vm"
	"github.com/sarchlab/akita/v4/mem/vm/mmu"
	"github.com/sarchlab/akita/v4/noc/networking/pcie"
	"github.com/sarchlab/akita/v4/sim"
	"github.com/sarchlab/akita/v4/simulation"
	"github.com/sarchlab/mgpusim/v4/amd/driver"
	"github.com/sarchlab/mgpusim/v4/amd/samples/runner/timingconfig/privatetlb"
	"github.com/sarchlab/mgpusim/v4/amd/samples/runner/timingconfig/r9nano"
	"github.com/sarchlab/mgpusim/v4/amd/timing/cp"
)

// Builder builds a hardware platform for timing simulation.
type Builder struct {
	simulation *simulation.Simulation

	numGPUs      int
	platformType string
	// Chiplet configuration (only used when platformType == \"privatetlb\")
	numChipletsPerGPU       int
	numSAPerChiplet         int
	numMemoryBankPerChiplet int

	numCUPerSA                     int
	numSAPerGPU                    int
	cpuMemSize                     uint64
	gpuMemSize                     uint64
	log2PageSize                   uint64
	log2MemoryBankInterleavingSize uint64
	useMagicMemoryCopy             bool

	// Kernel scheduling configuration (applied at the command processor).
	schedulingAlgorithm string
	schedPartition      string

	// Memory allocator configuration (driver-level).
	memAllocatorType string

	// PTW/PWC configuration (privatetlb only).
	ptwWalkersPerChiplet int
	pwcEntries           int
	pwcLookupLatency     int

	platform          *sim.Domain
	globalStorage     *mem.Storage
	mmu               *mmu.Comp
	pageTable         vm.PageTable
	rdmaAddressMapper *mem.BankedAddressPortMapper
}

// MakeBuilder creates a new Builder with default parameters.
func MakeBuilder() Builder {
	return Builder{
		numGPUs:                        1,
		numCUPerSA:                     4,
		numSAPerGPU:                    16,
		cpuMemSize:                     4 * mem.GB,
		gpuMemSize:                     4 * mem.GB,
		log2PageSize:                   12,
		log2MemoryBankInterleavingSize: 0,
		useMagicMemoryCopy:             false,
		platformType:                   "r9nano",
		numChipletsPerGPU:              4,
		numSAPerChiplet:                4,
		numMemoryBankPerChiplet:        4,

		schedulingAlgorithm: "round-robin",
		schedPartition:      "Xdiv",
		memAllocatorType:    "interleaved",

		ptwWalkersPerChiplet: 16,
		pwcEntries:           32,
		pwcLookupLatency:     10,
	}
}

// WithSimulation sets the simulation to use.
func (b Builder) WithSimulation(sim *simulation.Simulation) Builder {
	b.simulation = sim
	return b
}

// WithNumGPUs sets the number of GPUs to simulate.
func (b Builder) WithNumGPUs(numGPUs int) Builder {
	b.numGPUs = numGPUs
	return b
}

// WithPlatformType sets the timing platform type. Supported values:
//   - "r9nano": the original monolithic R9Nano-like GPU
//   - "privatetlb": a chiplet-based GPU that follows MGVM's private-TLB baseline
func (b Builder) WithPlatformType(platformType string) Builder {
	b.platformType = platformType
	return b
}

// WithSchedulingAlgorithm sets the default kernel scheduling algorithm.
// Supported values: "round-robin", "lasp".
func (b Builder) WithSchedulingAlgorithm(alg string) Builder {
	b.schedulingAlgorithm = alg
	return b
}

// WithSchedPartition sets the default LASP partitioning scheme.
// Examples: "Xdiv", "Ydiv", "Xblk4", "Yblk4".
func (b Builder) WithSchedPartition(partition string) Builder {
	b.schedPartition = partition
	return b
}

// WithMemAllocatorType configures the driver-side memory allocator.
// For chiplet experiments, "interleaved" or "lasp"/"partitioned" enable
// per-page placement on chiplets.
func (b Builder) WithMemAllocatorType(t string) Builder {
	b.memAllocatorType = t
	return b
}

// WithPTWWalkersPerChiplet sets the number of parallel PTWs per chiplet.
//
// This is only used by the privatetlb platform.
func (b Builder) WithPTWWalkersPerChiplet(n int) Builder {
	b.ptwWalkersPerChiplet = n
	return b
}

// WithPWCEntries sets the number of entries in the per-chiplet PWC.
//
// This is only used by the privatetlb platform.
func (b Builder) WithPWCEntries(n int) Builder {
	b.pwcEntries = n
	return b
}

// WithPWCLookupLatency sets the PWC lookup latency in cycles.
//
// This is only used by the privatetlb platform.
func (b Builder) WithPWCLookupLatency(n int) Builder {
	b.pwcLookupLatency = n
	return b
}

// WithNumChipletsPerGPU sets the number of chiplets per GPU (privatetlb only).
func (b Builder) WithNumChipletsPerGPU(n int) Builder {
	b.numChipletsPerGPU = n
	return b
}

// WithNumShaderArrayPerChiplet sets the number of shader arrays per chiplet (privatetlb only).
func (b Builder) WithNumShaderArrayPerChiplet(n int) Builder {
	b.numSAPerChiplet = n
	return b
}

// WithNumMemoryBankPerChiplet sets the number of memory banks per chiplet (privatetlb only).
func (b Builder) WithNumMemoryBankPerChiplet(n int) Builder {
	b.numMemoryBankPerChiplet = n
	return b
}

// WithLog2MemoryBankInterleavingSize sets the log2 of the memory bank interleaving size in bytes.
//
// When set to 0, the platform default is used (r9nano: 7=128B, privatetlb: log2PageSize=4KB).
func (b Builder) WithLog2MemoryBankInterleavingSize(log2 uint64) Builder {
	b.log2MemoryBankInterleavingSize = log2
	return b
}

// WithMagicMemoryCopy sets whether to use the magic memory copy middleware.
func (b Builder) WithMagicMemoryCopy() Builder {
	b.useMagicMemoryCopy = true
	return b
}

// Build builds the hardware platform.
func (b Builder) Build() *sim.Domain {
	b.cpuGPUMemSizeMustEqual()

	// Configure driver-side memory allocator before building the driver.
	//
	// We keep this configuration global (driver/internal) to minimize the amount
	// of plumbing needed for the chiplet experiments.
	if b.platformType == "privatetlb" {
		driver.SetChipletMemoryConfig(b.numChipletsPerGPU, b.numMemoryBankPerChiplet)
	} else {
		// Monolithic baseline.
		driver.SetChipletMemoryConfig(1, 16)
	}
	driver.SetMemoryAllocatorType(b.memAllocatorType)

	b.platform = &sim.Domain{}

	b.globalStorage = mem.NewStorage(
		uint64(b.numGPUs)*b.gpuMemSize + b.cpuMemSize)

	mmuComp, pageTable := b.createMMU()
	b.mmu = mmuComp
	b.pageTable = pageTable
	gpuDriver := b.buildGPUDriver(pageTable)

	pcieConnector, rootComplexID :=
		b.createConnection(gpuDriver, mmuComp)

	mmuComp.MigrationServiceProvider = gpuDriver.GetPortByName("MMU").AsRemote()

	b.createRDMAAddressMapper()

	b.createRDMAAddrTable()
	pmcAddressTable := b.createPMCPageTable()

	b.createGPUs(
		rootComplexID, pcieConnector,
		gpuDriver,
		pmcAddressTable)

	pcieConnector.EstablishRoute()

	return b.platform
}

func (b *Builder) cpuGPUMemSizeMustEqual() {
	if b.cpuMemSize != b.gpuMemSize {
		panic("currently only support cpuMemSize == gpuMemSize")
	}
}

func (b *Builder) createMMU() (*mmu.Comp, vm.PageTable) {
	pageTable := vm.NewRadixPageTable(b.log2PageSize).WithStorage(b.globalStorage)
	mmuBuilder := mmu.MakeBuilder().
		WithEngine(b.simulation.GetEngine()).
		WithFreq(1 * sim.GHz).
		WithPageWalkingLatency(100).
		WithLog2PageSize(b.log2PageSize).
		WithPageTable(pageTable)

	mmuComponent := mmuBuilder.Build("MMU")

	b.simulation.RegisterComponent(mmuComponent)

	return mmuComponent, pageTable
}

func (b *Builder) buildGPUDriver(
	pageTable vm.PageTable,
) *driver.Driver {
	gpuDriverBuilder := driver.MakeBuilder()

	if b.useMagicMemoryCopy {
		gpuDriverBuilder = gpuDriverBuilder.WithMagicMemoryCopyMiddleware()
	}

	gpuDriver := gpuDriverBuilder.
		WithEngine(b.simulation.GetEngine()).
		WithPageTable(pageTable).
		WithLog2PageSize(b.log2PageSize).
		WithGlobalStorage(b.globalStorage).
		WithD2HCycles(8500).
		WithH2DCycles(14500).
		Build("Driver")

	b.simulation.RegisterComponent(gpuDriver)

	return gpuDriver
}

func (b *Builder) createR9NanoGPUBuilder(
	gpuDriver *driver.Driver,
	mmuComponent *mmu.Comp,
) r9nano.Builder {
	log2Interleaving := b.log2MemoryBankInterleavingSize
	if log2Interleaving == 0 {
		log2Interleaving = 7
	}

	gpuBuilder := r9nano.MakeBuilder().
		WithFreq(1 * sim.GHz).
		WithSimulation(b.simulation).
		WithMMU(mmuComponent).
		WithSchedulingAlgorithm(b.schedulingAlgorithm).
		WithSchedPartition(b.schedPartition).
		WithNumCUPerShaderArray(b.numCUPerSA).
		WithNumShaderArray(b.numSAPerGPU).
		WithNumMemoryBank(16).
		WithLog2MemoryBankInterleavingSize(log2Interleaving).
		WithLog2PageSize(b.log2PageSize).
		WithGlobalStorage(b.globalStorage)

	return gpuBuilder
}

func (b *Builder) createPrivateTLBGPUBuilder(
	gpuDriver *driver.Driver,
	mmuComponent *mmu.Comp,
) privatetlb.Builder {
	totalBanks := b.numChipletsPerGPU * b.numMemoryBankPerChiplet
	totalSAs := b.numChipletsPerGPU * b.numSAPerChiplet

	log2Interleaving := b.log2MemoryBankInterleavingSize
	if log2Interleaving == 0 {
		// MGVM privatetlb uses page-granularity interleaving so that per-page placement
		// (LASP/plan) has a visible locality impact.
		log2Interleaving = b.log2PageSize
	}

	gpuBuilder := privatetlb.MakeBuilder().
		WithFreq(1 * sim.GHz).
		WithSimulation(b.simulation).
		WithMMU(mmuComponent).
		WithPageTable(b.pageTable).
		WithSchedulingAlgorithm(b.schedulingAlgorithm).
		WithSchedPartition(b.schedPartition).
		WithPTWWalkersPerChiplet(b.ptwWalkersPerChiplet).
		WithPWCEntries(b.pwcEntries).
		WithPWCLookupLatency(b.pwcLookupLatency).
		WithNumChipletsPerGPU(b.numChipletsPerGPU).
		WithNumCUPerShaderArray(b.numCUPerSA).
		WithNumShaderArrayPerChiplet(b.numSAPerChiplet).
		WithNumMemoryBankPerChiplet(b.numMemoryBankPerChiplet).
		WithNumShaderArray(totalSAs).
		WithNumMemoryBank(totalBanks).
		WithLog2MemoryBankInterleavingSize(log2Interleaving).
		WithLog2PageSize(b.log2PageSize).
		WithGlobalStorage(b.globalStorage)

	return gpuBuilder
}

func (b *Builder) createGPUs(
	rootComplexID int,
	pcieConnector *pcie.Connector,
	gpuDriver *driver.Driver,
	pmcAddressTable *mem.BankedAddressPortMapper,
) {
	lastSwitchID := rootComplexID
	for i := 1; i < b.numGPUs+1; i++ {
		if i%2 == 1 {
			lastSwitchID = pcieConnector.AddSwitch(rootComplexID)
		}

		b.createGPU(i, gpuDriver, pmcAddressTable, pcieConnector, lastSwitchID)
	}
}

func (b *Builder) createPMCPageTable() *mem.BankedAddressPortMapper {
	pmcAddressTable := new(mem.BankedAddressPortMapper)
	pmcAddressTable.BankSize = 4 * mem.GB
	pmcAddressTable.LowModules = append(pmcAddressTable.LowModules, "")
	return pmcAddressTable
}

func (b *Builder) createRDMAAddrTable() *mem.BankedAddressPortMapper {
	rdmaAddressTable := new(mem.BankedAddressPortMapper)
	rdmaAddressTable.BankSize = 4 * mem.GB
	rdmaAddressTable.LowModules = append(rdmaAddressTable.LowModules, "")
	return rdmaAddressTable
}

func (b *Builder) createConnection(
	gpuDriver *driver.Driver,
	mmuComponent *mmu.Comp,
) (*pcie.Connector, int) {
	// connection := sim.NewDirectConnection(engine)
	// connection := noc.NewFixedBandwidthConnection(32, engine, 1*sim.GHz)
	// connection.SrcBufferCapacity = 40960000
	pcieConnector := pcie.NewConnector().
		WithEngine(b.simulation.GetEngine()).
		WithVersion(4, 16).
		WithSwitchLatency(140)

	pcieConnector.CreateNetwork("PCIe")
	rootComplexID := pcieConnector.AddRootComplex(
		[]sim.Port{
			gpuDriver.GetPortByName("GPU"),
			gpuDriver.GetPortByName("MMU"),
			mmuComponent.GetPortByName("Migration"),
			mmuComponent.GetPortByName("Top"),
		})

	return pcieConnector, rootComplexID
}

func (b *Builder) createRDMAAddressMapper() {
	b.rdmaAddressMapper = new(mem.BankedAddressPortMapper)
	b.rdmaAddressMapper.BankSize = b.gpuMemSize
	b.rdmaAddressMapper.LowModules = append(b.rdmaAddressMapper.LowModules,
		sim.RemotePort("CPU"))
}

func (b *Builder) createGPU(
	index int,
	gpuDriver *driver.Driver,
	pmcAddressTable *mem.BankedAddressPortMapper,
	pcieConnector *pcie.Connector,
	pcieSwitchID int,
) *sim.Domain {
	name := fmt.Sprintf("GPU[%d]", index)

	// Align the GPU memory base address with the driver's memory allocator.
	memAddrOffset := gpuDriver.NextDeviceInitialAddress()

	var gpu *sim.Domain
	var props driver.DeviceProperties

	switch b.platformType {
	case "", "r9nano":
		gpuBuilder := b.createR9NanoGPUBuilder(gpuDriver, b.mmu).
			WithGPUID(uint64(index)).
			WithMemAddrOffset(memAddrOffset).
			WithRDMAAddressMapper(b.rdmaAddressMapper)
		gpu = gpuBuilder.Build(name)
		props = driver.DeviceProperties{
			CUCount:  b.numCUPerSA * b.numSAPerGPU,
			DRAMSize: 4 * mem.GB,
		}
	case "privatetlb":
		gpuBuilder := b.createPrivateTLBGPUBuilder(gpuDriver, b.mmu).
			WithGPUID(uint64(index)).
			WithMemAddrOffset(memAddrOffset).
			WithRDMAAddressMapper(b.rdmaAddressMapper)
		gpu = gpuBuilder.Build(name)
		props = driver.DeviceProperties{
			CUCount:  b.numCUPerSA * b.numSAPerChiplet * b.numChipletsPerGPU,
			DRAMSize: 4 * mem.GB,
		}
	default:
		panic(fmt.Sprintf("unknown platform type %q", b.platformType))
	}

	gpuDriver.RegisterGPU(gpu.GetPortByName("CommandProcessor"), props)

	// Some CP control paths (e.g., TLB shootdown and RDMA drain) need a valid
	// handle to the driver's "GPU" port to send responses back.
	if cpPort := gpu.GetPortByName("CommandProcessor"); cpPort != nil {
		if cpComp, ok := cpPort.Component().(*cp.CommandProcessor); ok {
			cpComp.Driver = gpuDriver.GetPortByName("GPU")
		}
	}

	b.configRDMAEngine(gpu)
	// b.configPMC(gpu, gpuDriver, pmcAddressTable)

	pcieConnector.PlugInDevice(pcieSwitchID, gpu.Ports())

	return gpu
}

func (b *Builder) configRDMAEngine(
	gpu *sim.Domain,
) {
	b.rdmaAddressMapper.LowModules = append(
		b.rdmaAddressMapper.LowModules,
		gpu.GetPortByName("RDMAData").AsRemote())
}

// func (b *Builder) configPMC(
// 	gpu *GPU,
// 	gpuDriver *driver.Driver,
// 	addrTable *mem.BankedAddressPortMapper,
// ) {
// 	gpu.PMC.RemotePMCAddressTable = addrTable
// 	addrTable.LowModules = append(
// 		addrTable.LowModules,
// 		gpu.PMC.GetPortByName("Remote").AsRemote())
// 	gpuDriver.RemotePMCPorts = append(
// 		gpuDriver.RemotePMCPorts, gpu.PMC.GetPortByName("Remote"))
// }
