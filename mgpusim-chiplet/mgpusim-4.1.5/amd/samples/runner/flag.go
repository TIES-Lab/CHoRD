package runner

import (
	"flag"
	"strconv"
	"strings"
)

var timingFlag = flag.Bool("timing", false, "Run detailed timing simulation.")

var platformTypeFlag = flag.String("platform-type", "r9nano",
	"Timing platform type. Options: r9nano, privatetlb (chiplet).")

// Kernel scheduling options (timing mode only)
var schedulingFlag = flag.String("scheduling", "round-robin",
	"Kernel scheduling policy. Options: round-robin, lasp, plan (use per-kernel hints from -plan).")
var schedPartitionFlag = flag.String("sched-partition", "Xdiv",
	"LASP partition scheme. Examples: Xdiv, Ydiv, Xblk4, Yblk4.")

// Memory allocator options (timing mode only)
var memAllocatorTypeFlag = flag.String("mem-allocator-type", "interleaved",
	"Memory allocator type. Options: default, buddy, partitioned/interleaved/lasp.")
var useLASPMemAllocFlag = flag.Bool("use-lasp-mem-alloc", false,
	"Enable LASP-aware memory allocation APIs in benchmarks (benchmarks must opt-in).")

// Plan JSON file (buffer placement + per-kernel scheduling)
var planJSONFlag = flag.String("plan-json", "",
	"Path to a plan JSON file that specifies buffer page placement and per-kernel scheduling.")

// Backward-compatible aliases (used by older scripts/benchmarks)
var planFlag = flag.String("plan", "", "Alias for -plan-json")

var chipletsPerGPUFlag = flag.Int("chiplets-per-gpu", 0,
	"Alias for -num-chiplets-per-gpu")
var saPerChipletFlag = flag.Int("sa-per-chiplet", 0,
	"Alias for -num-sa-per-chiplet")
var banksPerChipletFlag = flag.Int("banks-per-chiplet", 0,
	"Alias for -num-banks-per-chiplet")

// Chiplet platform parameters (only used when -platform-type=privatetlb)
var numChipletsPerGPUFlag = flag.Int("num-chiplets-per-gpu", 4,
	"Number of chiplets per GPU (privatetlb only).")
var numSAPerChipletFlag = flag.Int("num-sa-per-chiplet", 4,
	"Number of shader arrays per chiplet (privatetlb only).")
var numBanksPerChipletFlag = flag.Int("num-banks-per-chiplet", 4,
	"Number of memory banks per chiplet (privatetlb only).")

var log2MemoryBankInterleavingSizeFlag = flag.Uint64("log2-memory-bank-interleaving-size", 0,
	"Log2 of the memory bank interleaving size in bytes. 0 uses the platform default (r9nano: 7=128B, privatetlb: log2PageSize=4KB).")

// PTW/PWC parameters (privatetlb only)
//
// Defaults align with MGVM Table I:
//   - 16 page walkers per chiplet
//   - 32-entry fully associative page walk cache
//   - 10-cycle lookup latency
var ptwWalkersPerChipletFlag = flag.Int("ptw-walkers-per-chiplet", 16,
	"Number of parallel page table walkers (PTWs) per chiplet (privatetlb only). Default: 16 (MGVM).")
var pwcEntriesFlag = flag.Int("pwc-entries", 32,
	"Number of entries in the page walk cache (PWC) per chiplet (privatetlb only). Default: 32 (MGVM).")
var pwcLatencyFlag = flag.Int("pwc-latency", 10,
	"PWC lookup latency in cycles (privatetlb only). Default: 10 (MGVM).")

var maxInstCount = flag.Uint64("max-inst", 0,
	"Terminate the simulation after the given number of instructions is retired.")
var parallelFlag = flag.Bool("parallel", false,
	"Run the simulation in parallel.")
var isaDebug = flag.Bool("debug-isa", false, "Generate the ISA debugging file.")

var verifyFlag = flag.Bool("verify", false, "Verify the emulation result.")
var memTracing = flag.Bool("trace-mem", false, "Generate memory trace")
var instCountReportFlag = flag.Bool("report-inst-count", false,
	"Report the number of instructions executed in each compute unit.")
var cacheLatencyReportFlag = flag.Bool("report-cache-latency", false,
	"Report the average cache latency.")
var cacheHitRateReportFlag = flag.Bool("report-cache-hit-rate", false,
	"Report the cache hit rate of each cache.")
var tlbHitRateReportFlag = flag.Bool("report-tlb-hit-rate", false,
	"Report the TLB hit rate of each TLB.")
var rdmaTransactionCountReportFlag = flag.Bool("report-rdma-transaction-count",
	false, "Report the number of transactions going through the RDMA engines.")
var dramTransactionCountReportFlag = flag.Bool("report-dram-transaction-count",
	false, "Report the number of transactions accessing the DRAMs.")
var gpuFlag = flag.String("gpus", "",
	"The GPUs to use, use a format like 1,2,3,4. By default, GPU 1 is used.")
var unifiedGPUFlag = flag.String("unified-gpus", "",
	`Run multi-GPU benchmark in a unified mode.
Use a format like 1,2,3,4. Cannot coexist with -gpus.`)
var useUnifiedMemoryFlag = flag.Bool("use-unified-memory", false,
	"Run benchmark with Unified Memory or not")
var reportAll = flag.Bool("report-all", false, "Report all metrics to .csv file.")
var filenameFlag = flag.String("metric-file-name", "metrics",
	"Modify the name of the output csv file.")
var memAccessReportFlag = flag.Bool("report-mem-access", false,
	"Report local/remote L2 access counts and bytes (timing mode).")
var kernelMetricsReportFlag = flag.Bool("report-kernel-metrics", false,
	"Report per-kernel latency and local/remote memory access stats to <metric-file-name>_kernels.csv.")
var wgMappingReportFlag = flag.Bool("report-wg-mapping", false,
	"Dump work-group to (chiplet/SA/CU) mapping to <metric-file-name>_wgmap.csv (timing mode).")

var profileJSONFlag = flag.String("profile-json", "",
	"Write a kernel-by-buffer-page access profile to the given JSON file (timing mode). "+
		"The profile is collected at L2 (req_in) and attributed using req.Info.")

var profileGroupPagesFlag = flag.Int("profile-group-pages", 1,
	"Group consecutive pages in the profile by this factor (>=1). "+
		"The exported page_bytes will be multiplied accordingly.")
var profileWGTagPagesFlag = flag.Bool("profile-wg-tag-pages", false,
	"Include per-workgroup tag/page payloads in the profile JSON. Disabled by default to keep profile size manageable.")
var profilePacketAddrsFlag = flag.Bool("profile-packet-addrs", false,
	"Include packet address lists in the profile JSON. Disabled by default because step3 does not require them.")
var magicMemoryCopy = flag.Bool("magic-memory-copy", false,
	"Copy data from CPU directly to global memory")
var bufferLevelTraceDirFlag = flag.String("buffer-level-trace-dir", "",
	"The directory to dump the buffer level traces.")
var bufferLevelTracePeriodFlag = flag.Float64("buffer-level-trace-period", 0.0,
	"The period to dump the buffer level trace.")
var simdBusyTimeTracerFlag = flag.Bool("report-busy-time", false, "Report SIMD Unit's busy time")
var reportCPIStackFlag = flag.Bool("report-cpi-stack", false, "Report CPI stack")
var customPortForAkitaRTM = flag.Int("akitartm-port", 0,
	`Custom port to host AkitaRTM. A 4-digit or 5-digit port number is required. If 
this number is not given or a invalid number is given number, a random port 
will be used.`)
var disableAkitaRTM = flag.Bool("disable-rtm", false, "Disable the AkitaRTM monitoring portal")

var analyzerNameFlag = flag.String("analyzer-name", "",
	"The name of the analyzer to use.")

var analyzerPeriodFlag = flag.Float64("analyzer-period", 0.0,
	"The period to dump the analyzer results.")

var visTracing = flag.Bool("trace-vis", false,
	"Generate trace for visualization purposes.")
var visTracerDB = flag.String("trace-vis-db", "sqlite",
	"The database to store the visualization trace. Possible values are "+
		"sqlite, mysql, and csv.")
var visTracerDBFileName = flag.String("trace-vis-db-file", "",
	"The file name of the database to store the visualization trace. "+
		"Extension names are not required. "+
		"If not specified, a random file name will be used. "+
		"This flag does not work with Mysql db. When MySQL is used, "+
		"the database name is always randomly generated.")
var visTraceStartTime = flag.Float64("trace-vis-start", -1,
	"The starting time to collect visualization traces. A negative number "+
		"represents starting from the beginning.")
var visTraceEndTime = flag.Float64("trace-vis-end", -1,
	"The end time of collecting visualization traces. A negative number"+
		"means that the trace will be collected to the end of the simulation.")

// parseFlag applies the runner flag to runner object
func (r *Runner) parseFlag() *Runner {
	r.parseSimulationFlags()
	r.parseGPUFlag()

	return r
}

func (r *Runner) parseSimulationFlags() {
	if *parallelFlag {
		r.Parallel = true
	}

	if *verifyFlag {
		r.Verify = true
	}

	if *timingFlag {
		r.Timing = true
	}

	if *useUnifiedMemoryFlag {
		r.UseUnifiedMemory = true
	}
}

func (r *Runner) parseGPUFlag() {
	if *gpuFlag == "" && *unifiedGPUFlag == "" {
		r.GPUIDs = []int{1}
		return
	}

	if *gpuFlag != "" && *unifiedGPUFlag != "" {
		panic("cannot use -gpus and -unified-gpus together")
	}

	var gpuIDs []int
	if *gpuFlag != "" {
		gpuIDs = r.gpuIDStringToList(*gpuFlag)
	} else if *unifiedGPUFlag != "" {
		gpuIDs = r.gpuIDStringToList(*unifiedGPUFlag)
	}

	r.GPUIDs = gpuIDs
}

func (r *Runner) gpuIDStringToList(gpuIDsString string) []int {
	gpuIDs := make([]int, 0)
	gpuIDTokens := strings.Split(gpuIDsString, ",")

	for _, t := range gpuIDTokens {
		gpuID, err := strconv.Atoi(t)
		if err != nil {
			panic(err)
		}
		gpuIDs = append(gpuIDs, gpuID)
	}

	return gpuIDs
}
