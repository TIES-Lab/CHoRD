package privatetlb

import (
	"fmt"

	"github.com/sarchlab/akita/v4/mem/cache/writeback"
	"github.com/sarchlab/akita/v4/mem/idealmemcontroller"
	"github.com/sarchlab/akita/v4/mem/mem"
	"github.com/sarchlab/akita/v4/mem/vm"
	"github.com/sarchlab/akita/v4/mem/vm/mmu"
	"github.com/sarchlab/akita/v4/mem/vm/ptw"
	"github.com/sarchlab/akita/v4/mem/vm/tlb"
	"github.com/sarchlab/akita/v4/noc/networking/networkconnector"
	"github.com/sarchlab/akita/v4/sim"
	"github.com/sarchlab/akita/v4/sim/directconnection"
	"github.com/sarchlab/akita/v4/simulation"
	"github.com/sarchlab/mgpusim/v4/amd/samples/runner/timingconfig/shaderarray"
	"github.com/sarchlab/mgpusim/v4/amd/timing/cp"
	"github.com/sarchlab/mgpusim/v4/amd/timing/pagemigrationcontroller"
	"github.com/sarchlab/mgpusim/v4/amd/timing/rdma"
)

// Builder builds an AMD GPU timing model with an MGVM-like chiplet layout.
//
// This platform is intended to be the *baseline* for chiplet experiments.
// It provides:
//   - multiple chiplets per GPU (configurable)
//   - memory banks distributed across chiplets (MGVM-style interleaving by bank)
//   - an intra-GPU inter-chiplet network
//   - one L2 TLB per chiplet ("privatetlb" configuration)
//
// NOTE: Advanced MGVM features (shared TLB, remote translation, dHSL, PTE
// placement, etc.) are intentionally NOT implemented here.
type Builder struct {
	simulation *simulation.Simulation
	name       string
	freq       sim.Freq

	gpuID         uint64
	memAddrOffset uint64

	globalStorage *mem.Storage
	mmu           *mmu.Comp
	pageTable     vm.PageTable

	// Config
	log2PageSize                   uint64
	log2CacheLineSize              uint64
	log2MemoryBankInterleavingSize uint64
	l2CacheSize                    uint64

	numChipletsPerGPU        int
	numShaderArrayPerChiplet int
	numMemoryBankPerChiplet  int
	numCUPerShaderArray      int

	// Kernel scheduling configuration.
	schedulingAlgorithm string
	schedPartition      string

	// PTW/PWC configuration (MGVM Table I defaults).
	ptwWalkersPerChiplet int
	pwcEntries           int
	pwcLookupLatency     int

	// Compatibility knobs (total counts). TimingConfig builder sets these.
	numShaderArray int
	numMemoryBank  int

	// External address mapper used by the top-level TimingConfig builder to
	// connect GPUs over PCIe.
	rdmaAddressMapper mem.AddressToPortMapper

	// Built objects
	gpu          *sim.Domain
	sas          []*sim.Domain
	drams        []*idealmemcontroller.Comp
	l2Caches     []*writeback.Comp
	l2TLBs       []*tlb.Comp // one per chiplet
	ptws         []*ptw.Comp // one per chiplet
	l2TLBToPTW   *directconnection.Comp
	rdmaEngine   *rdma.Comp   // GPU-level RDMA (to CPU/other GPUs)
	chipletRDMAs []*rdma.Comp // one per chiplet (for intra-GPU remote bank access)

	cp        *cp.CommandProcessor
	dmaEngine *cp.DMAEngine
	pmc       *pagemigrationcontroller.PageMigrationController

	// Connections
	internalConn  *directconnection.Comp
	l1ToL2Conn    *directconnection.Comp
	l2ToDRAMConn  *directconnection.Comp
	l1ToL2TLBConn *directconnection.Comp

	// Address mappers
	globalL2Mapper      *StripedAddressPortMapper
	chipletL1Mappers    []*StripedLocalVRemoteAddressPortMapper
	chipletL1TLBMappers []*mem.SinglePortMapper
}

// MakeBuilder creates a privatetlb Builder with reasonable defaults.
func MakeBuilder() Builder {
	return Builder{
		freq: 1 * sim.GHz,

		log2PageSize:                   12,
		log2CacheLineSize:              6,
		log2MemoryBankInterleavingSize: 12,
		// IMPORTANT (MGVM alignment): L2 capacity should be sized *per memory bank*.
		// MGVM's MCMGPU builder uses 256KB per bank (see buildMemBanks in MGVM
		// gpubuilder/mcmgpu.go). If we instead fix a small total L2 size and divide
		// it across many banks, each bank becomes unrealistically tiny (e.g., 32KB
		// when using 64 banks), which can cause severe thrashing and even negative
		// scaling when increasing the number of ShaderArrays.
		l2CacheSize: 256 * mem.KB,

		numChipletsPerGPU:        4,
		numShaderArrayPerChiplet: 4,
		numMemoryBankPerChiplet:  4,
		numCUPerShaderArray:      4,

		schedulingAlgorithm: "round-robin",
		schedPartition:      "Xdiv",

		ptwWalkersPerChiplet: 16,
		pwcEntries:           32,
		pwcLookupLatency:     10,
	}
}

func (b Builder) WithSimulation(simulation *simulation.Simulation) Builder {
	b.simulation = simulation
	return b
}

func (b Builder) WithMMU(mmuComp *mmu.Comp) Builder {
	b.mmu = mmuComp
	return b
}

func (b Builder) WithPageTable(pageTable vm.PageTable) Builder {
	b.pageTable = pageTable
	return b
}

func (b Builder) WithFreq(freq sim.Freq) Builder {
	b.freq = freq
	return b
}

func (b Builder) WithGPUID(gpuID uint64) Builder {
	b.gpuID = gpuID
	return b
}

func (b Builder) WithMemAddrOffset(offset uint64) Builder {
	b.memAddrOffset = offset
	return b
}

func (b Builder) WithGlobalStorage(storage *mem.Storage) Builder {
	b.globalStorage = storage
	return b
}

func (b Builder) WithRDMAAddressMapper(mapper mem.AddressToPortMapper) Builder {
	b.rdmaAddressMapper = mapper
	return b
}

func (b Builder) WithLog2PageSize(log2PageSize uint64) Builder {
	b.log2PageSize = log2PageSize
	return b
}

func (b Builder) WithLog2MemoryBankInterleavingSize(log2 uint64) Builder {
	b.log2MemoryBankInterleavingSize = log2
	return b
}

func (b Builder) WithNumChipletsPerGPU(n int) Builder {
	b.numChipletsPerGPU = n
	return b
}

func (b Builder) WithNumShaderArrayPerChiplet(n int) Builder {
	b.numShaderArrayPerChiplet = n
	return b
}

func (b Builder) WithNumMemoryBankPerChiplet(n int) Builder {
	b.numMemoryBankPerChiplet = n
	return b
}

func (b Builder) WithNumCUPerShaderArray(n int) Builder {
	b.numCUPerShaderArray = n
	return b
}

// WithSchedulingAlgorithm sets the default kernel scheduling algorithm.
func (b Builder) WithSchedulingAlgorithm(alg string) Builder {
	b.schedulingAlgorithm = alg
	return b
}

// WithSchedPartition sets the LASP partitioning mode (e.g., Xdiv, Ydiv, Xblk4).
func (b Builder) WithSchedPartition(partition string) Builder {
	b.schedPartition = partition
	return b
}

// WithPTWWalkersPerChiplet sets the number of parallel PTWs per chiplet.
//
// Default: 16 (MGVM Table I).
func (b Builder) WithPTWWalkersPerChiplet(n int) Builder {
	b.ptwWalkersPerChiplet = n
	return b
}

// WithPWCEntries sets the number of entries in the per-chiplet PWC.
//
// Default: 32 (MGVM Table I).
func (b Builder) WithPWCEntries(n int) Builder {
	b.pwcEntries = n
	return b
}

// WithPWCLookupLatency sets the PWC lookup latency in cycles.
//
// Default: 10 (MGVM Table I).
func (b Builder) WithPWCLookupLatency(n int) Builder {
	b.pwcLookupLatency = n
	return b
}

func (b Builder) WithNumShaderArray(n int) Builder {
	b.numShaderArray = n
	return b
}

func (b Builder) WithNumMemoryBank(n int) Builder {
	b.numMemoryBank = n
	return b
}

// Build creates a GPU domain.
func (b Builder) Build(name string) *sim.Domain {
	b.name = name

	if b.simulation == nil {
		panic("privatetlb.Builder requires Simulation")
	}
	if b.globalStorage == nil {
		panic("privatetlb.Builder requires GlobalStorage")
	}
	if b.mmu == nil {
		panic("privatetlb.Builder requires MMU")
	}
	if b.pageTable == nil {
		panic("privatetlb.Builder requires PageTable")
	}

	b.gpu = sim.NewDomain(name)

	// Allocate per-chiplet placeholders that shader arrays and L2TLBs will use.
	b.chipletL1TLBMappers = make([]*mem.SinglePortMapper, b.numChipletsPerGPU)
	for i := range b.chipletL1TLBMappers {
		b.chipletL1TLBMappers[i] = &mem.SinglePortMapper{}
	}

	b.buildDRAMControllers()
	b.buildL2Caches()
	b.buildGlobalL2Mapper()

	b.buildRDMAEngine()
	b.buildChipletRDMAsAndL1Mappers()
	b.buildInterChipletNetwork()

	b.buildShaderArrays()
	b.buildPTWs()
	b.buildL2TLBs()

	b.buildCP()

	b.connectCP()
	b.connectL1ToL2()
	b.connectL2AndDRAM()
	b.connectL1TLBToL2TLB()
	b.connectL2TLBToPTW()
	b.populateExternalPorts()

	return b.gpu
}

func (b *Builder) totalShaderArrays() int {
	if b.numShaderArray != 0 {
		return b.numShaderArray
	}
	return b.numChipletsPerGPU * b.numShaderArrayPerChiplet
}

func (b *Builder) totalMemoryBanks() int {
	if b.numMemoryBank != 0 {
		return b.numMemoryBank
	}
	return b.numChipletsPerGPU * b.numMemoryBankPerChiplet
}

func (b *Builder) buildDRAMControllers() {
	numBanks := b.totalMemoryBanks()
	b.drams = make([]*idealmemcontroller.Comp, 0, numBanks)

	builder := idealmemcontroller.MakeBuilder().
		WithEngine(b.simulation.GetEngine()).
		WithFreq(b.freq).
		WithLatency(100).
		WithStorage(b.globalStorage)

	for i := 0; i < numBanks; i++ {
		dram := builder.Build(fmt.Sprintf("%s.DRAM[%d]", b.name, i))
		b.simulation.RegisterComponent(dram)
		b.drams = append(b.drams, dram)
	}
}

func (b *Builder) buildL2Caches() {
	numBanks := b.totalMemoryBanks()

	// NOTE: b.l2CacheSize is interpreted as *per-bank* capacity.
	// See the comment in MakeBuilder for why this matters for scaling.
	perBankSize := b.l2CacheSize

	l2Builder := writeback.MakeBuilder().
		WithEngine(b.simulation.GetEngine()).
		WithFreq(b.freq).
		WithLog2BlockSize(b.log2CacheLineSize).
		WithWayAssociativity(16).
		WithByteSize(perBankSize).
		WithNumMSHREntry(64).
		// Align with MGVM's baseline: allow a small, realistic number of requests
		// per cycle through each L2 slice. (MGVM uses 2 req/cycle.)
		WithNumReqPerCycle(2)

	b.l2Caches = make([]*writeback.Comp, 0, numBanks)
	for i := 0; i < numBanks; i++ {
		cacheName := fmt.Sprintf("%s.L2Cache[%d]", b.name, i)

		l2 := l2Builder.
			WithInterleaving(
				1<<(b.log2MemoryBankInterleavingSize-b.log2CacheLineSize),
				numBanks,
				i,
			).
			WithInterleavingOffset(b.memAddrOffset).
			WithAddressMapperType("single").
			WithRemotePorts(b.drams[i].GetPortByName("Top").AsRemote()).
			Build(cacheName)

		b.simulation.RegisterComponent(l2)
		b.l2Caches = append(b.l2Caches, l2)
	}
}

func (b *Builder) buildPTWs() {
	// Defaults align with MGVM Table I:
	//   - 16 parallel walkers per chiplet
	//   - 32-entry fully associative Page Walk Cache (PWC)
	//   - 10-cycle PWC lookup
	//
	// These knobs are configurable via runner flags.
	numWalkers := b.ptwWalkersPerChiplet
	if numWalkers <= 0 {
		numWalkers = 1
	}

	pwcEntries := b.pwcEntries
	if pwcEntries < 0 {
		pwcEntries = 0
	}

	pwcLookupLatency := b.pwcLookupLatency
	if pwcLookupLatency < 0 {
		pwcLookupLatency = 0
	}

	b.ptws = make([]*ptw.Comp, b.numChipletsPerGPU)

	ptwBuilder := ptw.MakeBuilder().
		WithEngine(b.simulation.GetEngine()).
		WithFreq(b.freq).
		WithPageTable(b.pageTable).
		WithNumWalkers(numWalkers).
		WithPWCEntries(pwcEntries).
		WithPWCLookupLatency(pwcLookupLatency)
	striping := uint64(1 << b.log2MemoryBankInterleavingSize)

	for c := 0; c < b.numChipletsPerGPU; c++ {
		w := ptwBuilder.
			WithAddressMapper(b.chipletL1Mappers[c]).
			WithChipletLayout(c, b.memAddrOffset, b.numChipletsPerGPU, b.numMemoryBankPerChiplet, striping).
			Build(fmt.Sprintf("%s.PTW[%d]", b.name, c))
		b.ptws[c] = w
		b.simulation.RegisterComponent(w)
	}
}

func (b *Builder) buildGlobalL2Mapper() {
	striping := uint64(1 << b.log2MemoryBankInterleavingSize)
	numBanks := uint64(b.totalMemoryBanks())

	ports := make([]sim.RemotePort, 0, int(numBanks))
	for _, l2 := range b.l2Caches {
		ports = append(ports, l2.GetPortByName("Top").AsRemote())
	}

	b.globalL2Mapper = &StripedAddressPortMapper{
		MemAddrOffset:             b.memAddrOffset,
		NumBanks:                  numBanks,
		Striping:                  striping,
		LowModules:                ports,
		UseAddressSpaceLimitation: true,
		LowAddress:                b.memAddrOffset,
		HighAddress:               b.memAddrOffset + 4*mem.GB,
		ModuleForOtherAddresses:   sim.RemotePort(""),
	}
}

func (b *Builder) buildRDMAEngine() {
	b.rdmaEngine = rdma.MakeBuilder().
		WithEngine(b.simulation.GetEngine()).
		WithFreq(b.freq).
		WithLocalModules(b.globalL2Mapper).
		WithRemoteModules(b.rdmaAddressMapper).
		Build(fmt.Sprintf("%s.RDMA", b.name))

	b.simulation.RegisterComponent(b.rdmaEngine)
}

func (b *Builder) buildChipletRDMAsAndL1Mappers() {
	numChiplets := b.numChipletsPerGPU
	striping := uint64(1 << b.log2MemoryBankInterleavingSize)
	totalBanks := uint64(b.totalMemoryBanks())

	b.chipletRDMAs = make([]*rdma.Comp, numChiplets)
	b.chipletL1Mappers = make([]*StripedLocalVRemoteAddressPortMapper, numChiplets)

	// 1) Build per-chiplet RDMAs with local-bank mappers.
	for c := 0; c < numChiplets; c++ {
		bankStart := c * b.numMemoryBankPerChiplet
		localPorts := make([]sim.RemotePort, 0, b.numMemoryBankPerChiplet)
		for i := 0; i < b.numMemoryBankPerChiplet; i++ {
			l2 := b.l2Caches[bankStart+i]
			localPorts = append(localPorts, l2.GetPortByName("Top").AsRemote())
		}

		localMapper := &StripedLocalBankAddressPortMapper{
			MemAddrOffset:  b.memAddrOffset,
			NumBanks:       totalBanks,
			Striping:       striping,
			LocalBankStart: uint64(bankStart),
			LocalModules:   localPorts,
		}

		r := rdma.MakeBuilder().
			WithEngine(b.simulation.GetEngine()).
			WithFreq(b.freq).
			WithLocalModules(localMapper).
			Build(fmt.Sprintf("%s.Chiplet[%d].RDMA", b.name, c))

		b.simulation.RegisterComponent(r)
		b.chipletRDMAs[c] = r
	}

	// 2) Create a single remote-table that maps an address to a *destination
	// chiplet* based on bank index. This is MGVM's interleaved layout.
	chipletRemotePorts := make([]sim.RemotePort, 0, numChiplets)
	for c := 0; c < numChiplets; c++ {
		chipletRemotePorts = append(chipletRemotePorts, b.chipletRDMAs[c].RDMADataOutside.AsRemote())
	}

	remoteMapper := &StripedChipletAddressPortMapper{
		MemAddrOffset:             b.memAddrOffset,
		NumChiplets:               uint64(numChiplets),
		StripingPerChiplet:        striping * uint64(b.numMemoryBankPerChiplet),
		LowModules:                chipletRemotePorts,
		UseAddressSpaceLimitation: true,
		LowAddress:                b.memAddrOffset,
		HighAddress:               b.memAddrOffset + 4*mem.GB,
		ModuleForOtherAddresses:   sim.RemotePort(""),
	}

	for c := 0; c < numChiplets; c++ {
		b.chipletRDMAs[c].RemoteRDMAAddressTable = remoteMapper
	}

	// 3) Build per-chiplet L1 address mappers.
	for c := 0; c < numChiplets; c++ {
		bankStart := c * b.numMemoryBankPerChiplet
		localPorts := make([]sim.RemotePort, 0, b.numMemoryBankPerChiplet)
		for i := 0; i < b.numMemoryBankPerChiplet; i++ {
			l2 := b.l2Caches[bankStart+i]
			localPorts = append(localPorts, l2.GetPortByName("Top").AsRemote())
		}

		b.chipletL1Mappers[c] = &StripedLocalVRemoteAddressPortMapper{
			MemAddrOffset:         b.memAddrOffset,
			NumBanks:              totalBanks,
			Striping:              striping,
			LocalBankStart:        uint64(bankStart),
			LocalModules:          localPorts,
			ModuleForRemoteBanks:  b.chipletRDMAs[c].RDMARequestInside.AsRemote(),
			ModuleForOutsideRange: b.rdmaEngine.RDMARequestInside.AsRemote(),

			UseAddressSpaceLimitation: true,
			LowAddress:                b.memAddrOffset,
			HighAddress:               b.memAddrOffset + 4*mem.GB,
		}
	}
}

func (b *Builder) buildInterChipletNetwork() {
	// Inter-chiplet network (ICN) for connecting each chiplet RDMA (request + data).
	//
	// MGVM's chipnetwork uses a relatively high per-cycle injection capability (NumReqPerCycle=12)
	// and a larger flit size (76B). If we keep the default (1 channel, tiny buffers),
	// increasing SAs/CUs can easily saturate the ICN and cause *negative scaling*.
	//
	// Here we approximate MGVM's ICN bandwidth by:
	//   * flit size = 76 bytes
	//   * 12 input/output channels per switch port and per endpoint
	//   * larger endpoint/switch buffers
	// and keep the full-mesh per-chiplet switch topology.
	conn := networkconnector.MakeConnector().
		WithEngine(b.simulation.GetEngine()).
		WithMonitor(b.simulation.GetMonitor()).
		WithVisTracer(b.simulation.GetVisTracer()).
		WithDefaultFreq(b.freq).
		WithFlitSize(76)

	conn.NewNetwork(fmt.Sprintf("%s.ICN", b.name))

	// Create one switch per chiplet.
	swIDs := make([]int, len(b.chipletRDMAs))
	for i := range b.chipletRDMAs {
		swIDs[i] = conn.AddSwitchWithName(fmt.Sprintf("ChipletSwitch[%d]", i))
	}

	const (
		bufSize       = 128
		numChannels   = 12
		switchLatency = 360
	)

	linkParam := networkconnector.DeviceToSwitchLinkParameter{
		DeviceEndParam: networkconnector.LinkEndDeviceParameter{
			IncomingBufSize:  bufSize,
			OutgoingBufSize:  bufSize,
			NumInputChannel:  numChannels,
			NumOutputChannel: numChannels,
		},
		SwitchEndParam: networkconnector.LinkEndSwitchParameter{
			IncomingBufSize:  bufSize,
			OutgoingBufSize:  bufSize,
			Latency:          switchLatency,
			NumInputChannel:  numChannels,
			NumOutputChannel: numChannels,
		},
		LinkParam: networkconnector.LinkParameter{
			IsIdeal:   true,
			Frequency: b.freq,
		},
	}

	// Connect each chiplet RDMA to its own switch.
	for i, r := range b.chipletRDMAs {
		conn.ConnectDevice(swIDs[i], []sim.Port{r.RDMARequestOutside, r.RDMADataOutside}, linkParam)
	}

	// Fully connect all chiplet switches.
	swLinkParam := networkconnector.SwitchToSwitchLinkParameter{
		LeftEndParam: networkconnector.LinkEndSwitchParameter{
			IncomingBufSize:  bufSize,
			OutgoingBufSize:  bufSize,
			Latency:          switchLatency,
			NumInputChannel:  numChannels,
			NumOutputChannel: numChannels,
		},
		RightEndParam: networkconnector.LinkEndSwitchParameter{
			IncomingBufSize:  bufSize,
			OutgoingBufSize:  bufSize,
			Latency:          switchLatency,
			NumInputChannel:  numChannels,
			NumOutputChannel: numChannels,
		},
		LinkParam: networkconnector.LinkParameter{
			IsIdeal:   true,
			Frequency: b.freq,
		},
	}

	for i := 0; i < len(swIDs); i++ {
		for j := i + 1; j < len(swIDs); j++ {
			conn.ConnectSwitches(swIDs[i], swIDs[j], swLinkParam)
		}
	}

	conn.EstablishRoute()
}

func (b *Builder) buildShaderArrays() {
	saBuilderBase := shaderarray.MakeBuilder().
		WithSimulation(b.simulation).
		WithFreq(b.freq).
		WithGPUID(b.gpuID).
		WithNumCUs(b.numCUPerShaderArray).
		WithLog2CacheLineSize(b.log2CacheLineSize).
		WithLog2PageSize(b.log2PageSize)

	b.sas = make([]*sim.Domain, 0, b.totalShaderArrays())
	globalSAID := 0
	for c := 0; c < b.numChipletsPerGPU; c++ {
		saBuilder := saBuilderBase.
			WithL1AddressMapper(b.chipletL1Mappers[c]).
			WithL1TLBAddressMapper(b.chipletL1TLBMappers[c])

		for i := 0; i < b.numShaderArrayPerChiplet; i++ {
			sa := saBuilder.Build(fmt.Sprintf("%s.SA[%d]", b.name, globalSAID))
			b.sas = append(b.sas, sa)
			globalSAID++
		}
	}
}

func (b *Builder) buildL2TLBs() {
	// MGVM private-TLB config: one L2 TLB per chiplet.
	// We use 512-entry, 8-way as a reasonable default.
	builderBase := tlb.MakeBuilder().
		WithEngine(b.simulation.GetEngine()).
		WithFreq(b.freq).
		WithNumWays(8).
		WithNumSets(64).
		WithNumMSHREntry(64).
		WithNumReqPerCycle(16).
		WithPageSize(1 << b.log2PageSize).
		WithLatency(10)

	b.l2TLBs = make([]*tlb.Comp, b.numChipletsPerGPU)
	for c := 0; c < b.numChipletsPerGPU; c++ {
		ptwPort := b.ptws[c].GetPortByName("Top").AsRemote()
		builder := builderBase.
			WithLowModule(ptwPort).
			WithTranslationProviderMapper(&mem.SinglePortMapper{Port: ptwPort})
		l2 := builder.Build(fmt.Sprintf("%s.L2TLB[%d]", b.name, c))
		b.simulation.RegisterComponent(l2)
		b.l2TLBs[c] = l2
		// Update the chiplet's L1->L2 TLB mapper.
		b.chipletL1TLBMappers[c].Port = l2.GetPortByName("Top").AsRemote()
	}
}

func (b *Builder) buildCP() {
	b.cp = cp.MakeBuilder().
		WithEngine(b.simulation.GetEngine()).
		WithVisTracer(b.simulation.GetVisTracer()).
		WithFreq(b.freq).
		WithSchedulingAlgorithm(b.schedulingAlgorithm).
		WithSchedPartition(b.schedPartition).
		WithNumChiplets(b.numChipletsPerGPU).
		WithMonitor(b.simulation.GetMonitor()).
		Build(b.name + ".CommandProcessor")
	b.simulation.RegisterComponent(b.cp)

	b.dmaEngine = cp.NewDMAEngine(
		fmt.Sprintf("%s.DMA", b.name),
		b.simulation.GetEngine(),
		nil,
	)
	b.simulation.RegisterComponent(b.dmaEngine)

	// PMC mem controller finder will be set in connectL2AndDRAM.
	b.pmc = pagemigrationcontroller.NewPageMigrationController(
		fmt.Sprintf("%s.PMC", b.name),
		b.simulation.GetEngine(),
		&mem.SinglePortMapper{Port: sim.RemotePort("")},
		nil,
	)
	b.simulation.RegisterComponent(b.pmc)
}

func (b *Builder) connectCP() {
	b.internalConn = directconnection.MakeBuilder().
		WithEngine(b.simulation.GetEngine()).
		WithFreq(b.freq).
		Build(b.name + ".InternalConn")
	b.simulation.RegisterComponent(b.internalConn)

	b.internalConn.PlugIn(b.cp.ToDMA)
	b.internalConn.PlugIn(b.cp.ToCaches)
	b.internalConn.PlugIn(b.cp.ToCUs)
	b.internalConn.PlugIn(b.cp.ToTLBs)
	b.internalConn.PlugIn(b.cp.ToAddressTranslators)
	b.internalConn.PlugIn(b.cp.ToRDMA)
	b.internalConn.PlugIn(b.cp.ToPMC)

	b.cp.RDMA = b.rdmaEngine.CtrlPort
	b.internalConn.PlugIn(b.cp.RDMA)

	b.cp.DMAEngine = b.dmaEngine.ToCP
	b.internalConn.PlugIn(b.dmaEngine.ToCP)

	b.cp.PMC = b.pmc.GetPortByName("Control")
	b.internalConn.PlugIn(b.cp.PMC)

	b.connectCPWithCUs()
	b.connectCPWithAddressTranslators()
	b.connectCPWithTLBs()
	b.connectCPWithCaches()
}

func (b *Builder) connectL1ToL2() {
	b.l1ToL2Conn = directconnection.MakeBuilder().
		WithEngine(b.simulation.GetEngine()).
		WithFreq(b.freq).
		Build(b.name + ".L1ToL2")
	b.simulation.RegisterComponent(b.l1ToL2Conn)

	// GPU-level RDMA (to CPU / other GPUs)
	b.rdmaEngine.SetLocalModuleFinder(b.globalL2Mapper)
	b.l1ToL2Conn.PlugIn(b.rdmaEngine.RDMARequestInside)
	b.l1ToL2Conn.PlugIn(b.rdmaEngine.RDMADataInside)

	// Chiplet-level RDMAs (intra-GPU remote banks)
	for _, r := range b.chipletRDMAs {
		b.l1ToL2Conn.PlugIn(r.RDMARequestInside)
		b.l1ToL2Conn.PlugIn(r.RDMADataInside)
	}

	// PTWs issue memory reads for PTEs and should share the same path to L2/RDMA.
	for _, w := range b.ptws {
		b.l1ToL2Conn.PlugIn(w.GetPortByName("Mem"))
	}

	for _, l2 := range b.l2Caches {
		b.l1ToL2Conn.PlugIn(l2.GetPortByName("Top"))
	}

	for _, sa := range b.sas {
		for i := 0; i < b.numCUPerShaderArray; i++ {
			b.l1ToL2Conn.PlugIn(sa.GetPortByName(fmt.Sprintf("L1VCacheBottom[%d]", i)))
		}
		b.l1ToL2Conn.PlugIn(sa.GetPortByName("L1SCacheBottom"))
		b.l1ToL2Conn.PlugIn(sa.GetPortByName("L1ICacheBottom"))
	}
}

func (b *Builder) connectL2AndDRAM() {
	b.l2ToDRAMConn = directconnection.MakeBuilder().
		WithEngine(b.simulation.GetEngine()).
		WithFreq(b.freq).
		Build(b.name + ".L2ToDRAM")
	b.simulation.RegisterComponent(b.l2ToDRAMConn)

	striping := uint64(1 << b.log2MemoryBankInterleavingSize)
	numBanks := uint64(b.totalMemoryBanks())

	dramPorts := make([]sim.RemotePort, 0, int(numBanks))
	for i, l2 := range b.l2Caches {
		b.l2ToDRAMConn.PlugIn(l2.GetPortByName("Bottom"))
		// Ensure each L2 bank only talks to its own DRAM bank.
		l2.SetAddressToPortMapper(&mem.SinglePortMapper{Port: b.drams[i].GetPortByName("Top").AsRemote()})
	}

	for _, dram := range b.drams {
		b.l2ToDRAMConn.PlugIn(dram.GetPortByName("Top"))
		dramPorts = append(dramPorts, dram.GetPortByName("Top").AsRemote())
	}

	// A finder for DMA/PMC that is consistent with the bank-interleaving scheme.
	memCtrlFinder := &StripedAddressPortMapper{
		MemAddrOffset:             b.memAddrOffset,
		NumBanks:                  numBanks,
		Striping:                  striping,
		LowModules:                dramPorts,
		UseAddressSpaceLimitation: true,
		LowAddress:                b.memAddrOffset,
		HighAddress:               b.memAddrOffset + 4*mem.GB,
		ModuleForOtherAddresses:   sim.RemotePort(""),
	}

	b.dmaEngine.SetLocalDataSource(memCtrlFinder)
	b.l2ToDRAMConn.PlugIn(b.dmaEngine.ToMem)

	b.pmc.MemCtrlFinder = memCtrlFinder
	b.l2ToDRAMConn.PlugIn(b.pmc.GetPortByName("LocalMem"))
}

func (b *Builder) connectL1TLBToL2TLB() {
	b.l1ToL2TLBConn = directconnection.MakeBuilder().
		WithEngine(b.simulation.GetEngine()).
		WithFreq(b.freq).
		Build(b.name + ".L1TLBToL2TLB")
	b.simulation.RegisterComponent(b.l1ToL2TLBConn)

	for _, l2 := range b.l2TLBs {
		b.l1ToL2TLBConn.PlugIn(l2.GetPortByName("Top"))
	}

	for _, sa := range b.sas {
		for i := 0; i < b.numCUPerShaderArray; i++ {
			b.l1ToL2TLBConn.PlugIn(sa.GetPortByName(fmt.Sprintf("L1VTLBBottom[%d]", i)))
		}
		b.l1ToL2TLBConn.PlugIn(sa.GetPortByName("L1STLBBottom"))
		b.l1ToL2TLBConn.PlugIn(sa.GetPortByName("L1ITLBBottom"))
	}
}

func (b *Builder) connectL2TLBToPTW() {
	b.l2TLBToPTW = directconnection.MakeBuilder().
		WithEngine(b.simulation.GetEngine()).
		WithFreq(b.freq).
		Build(fmt.Sprintf("%s.L2TLBToPTW", b.name))

	b.simulation.RegisterComponent(b.l2TLBToPTW)

	for c := 0; c < b.numChipletsPerGPU; c++ {
		b.l2TLBToPTW.PlugIn(b.l2TLBs[c].GetPortByName("Bottom"))
		b.l2TLBToPTW.PlugIn(b.ptws[c].GetPortByName("Top"))
	}
}

func (b *Builder) populateExternalPorts() {
	// Command processor (driver <-> GPU)
	b.gpu.AddPort("CommandProcessor", b.cp.ToDriver)

	// GPU-level RDMA (CPU/other GPU)
	b.gpu.AddPort("RDMARequest", b.rdmaEngine.RDMARequestOutside)
	b.gpu.AddPort("RDMAData", b.rdmaEngine.RDMADataOutside)

	// Page migration controller (remote PMC network)
	b.gpu.AddPort("PageMigrationController", b.pmc.GetPortByName("Remote"))

	// NOTE: In the MGVM-style private-TLB configuration, translation requests
	// are serviced on-chip by the page-table walkers (PTW). Therefore, L2TLB
	// Bottom ports are not exposed externally.
}

// --- CP helper code (copied from r9nano timing config) ---

type cuInterfaceForCP struct {
	ctrlPort        sim.RemotePort
	dispatchingPort sim.RemotePort
	wfPoolSizes     []int
	vRegCounts      []int
	sRegCount       int
	ldsBytes        int
}

func (cu cuInterfaceForCP) ControlPort() sim.RemotePort {
	return cu.ctrlPort
}

func (cu cuInterfaceForCP) DispatchingPort() sim.RemotePort {
	return cu.dispatchingPort
}

func (cu cuInterfaceForCP) WfPoolSizes() []int {
	return cu.wfPoolSizes
}

func (cu cuInterfaceForCP) VRegCounts() []int {
	return cu.vRegCounts
}

func (cu cuInterfaceForCP) SRegCount() int {
	return cu.sRegCount
}

func (cu cuInterfaceForCP) LDSBytes() int {
	return cu.ldsBytes
}

func (b *Builder) connectCPWithCUs() {
	for _, sa := range b.sas {
		for i := 0; i < b.numCUPerShaderArray; i++ {
			cuDispatchingPort := sa.GetPortByName(fmt.Sprintf("CU[%d]", i))
			cuCtrlPort := sa.GetPortByName(fmt.Sprintf("CUCtrl[%d]", i))

			cu := cuInterfaceForCP{
				ctrlPort:        cuCtrlPort.AsRemote(),
				dispatchingPort: cuDispatchingPort.AsRemote(),
				wfPoolSizes:     []int{10, 10, 10, 10},
				vRegCounts:      []int{16384, 16384, 16384, 16384},
				sRegCount:       3200,
				ldsBytes:        64 * 1024,
			}

			b.cp.RegisterCU(cu)

			b.internalConn.PlugIn(cuDispatchingPort)
			b.internalConn.PlugIn(cuCtrlPort)
		}
	}
}

func (b *Builder) connectCPWithAddressTranslators() {
	for _, sa := range b.sas {
		for i := 0; i < b.numCUPerShaderArray; i++ {
			port := sa.GetPortByName(fmt.Sprintf("L1VAddrTransCtrl[%d]", i))
			b.cp.AddressTranslators = append(b.cp.AddressTranslators, port)
			b.internalConn.PlugIn(port)
		}

		port := sa.GetPortByName("L1SAddrTransCtrl")
		b.cp.AddressTranslators = append(b.cp.AddressTranslators, port)
		b.internalConn.PlugIn(port)

		port = sa.GetPortByName("L1IAddrTransCtrl")
		b.cp.AddressTranslators = append(b.cp.AddressTranslators, port)
		b.internalConn.PlugIn(port)
	}
}

func (b *Builder) connectCPWithTLBs() {
	for _, sa := range b.sas {
		for i := 0; i < b.numCUPerShaderArray; i++ {
			port := sa.GetPortByName(fmt.Sprintf("L1VTLBCtrl[%d]", i))
			b.cp.TLBs = append(b.cp.TLBs, port)
			b.internalConn.PlugIn(port)
		}

		port := sa.GetPortByName("L1STLBCtrl")
		b.cp.TLBs = append(b.cp.TLBs, port)
		b.internalConn.PlugIn(port)

		port = sa.GetPortByName("L1ITLBCtrl")
		b.cp.TLBs = append(b.cp.TLBs, port)
		b.internalConn.PlugIn(port)
	}

	for _, l2 := range b.l2TLBs {
		port := l2.GetPortByName("Control")
		b.cp.TLBs = append(b.cp.TLBs, port)
		b.internalConn.PlugIn(port)
	}
}

func (b *Builder) connectCPWithCaches() {
	for _, sa := range b.sas {
		for i := 0; i < b.numCUPerShaderArray; i++ {
			port := sa.GetPortByName(fmt.Sprintf("L1VCacheCtrl[%d]", i))
			b.cp.L1VCaches = append(b.cp.L1VCaches, port)
			b.internalConn.PlugIn(port)
		}

		port := sa.GetPortByName("L1ICacheCtrl")
		b.cp.L1ICaches = append(b.cp.L1ICaches, port)
		b.internalConn.PlugIn(port)

		port = sa.GetPortByName("L1SCacheCtrl")
		b.cp.L1SCaches = append(b.cp.L1SCaches, port)
		b.internalConn.PlugIn(port)
	}

	for _, l2 := range b.l2Caches {
		port := l2.GetPortByName("Control")
		b.cp.L2Caches = append(b.cp.L2Caches, port)
		b.internalConn.PlugIn(port)
	}
}
