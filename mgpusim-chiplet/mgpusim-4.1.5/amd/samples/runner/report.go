package runner

import (
	"sort"
	"strings"

	"github.com/sarchlab/akita/v4/datarecording"
	"github.com/sarchlab/akita/v4/sim"
	"github.com/sarchlab/akita/v4/simulation"
	"github.com/sarchlab/akita/v4/tracing"
	"github.com/sarchlab/mgpusim/v4/amd/timing/cu"
	"github.com/sarchlab/mgpusim/v4/amd/timing/rdma"
)

const (
	tableName = "mgpusim_metrics"
)

type metric struct {
	Location string
	What     string
	Value    float64
	Unit     string
}

type kernelTimeTracer struct {
	tracer *tracing.BusyTimeTracer
	comp   tracing.NamedHookable
}

type instCountTracer struct {
	tracer *instTracer
	cu     tracing.NamedHookable
}

type cacheLatencyTracer struct {
	tracer *tracing.AverageTimeTracer
	cache  tracing.NamedHookable
}

type cacheHitRateTracer struct {
	tracer *cacheEventTracer
	cache  tracing.NamedHookable
}

type tlbHitRateTracer struct {
	tracer *tracing.StepCountTracer
	tlb    tracing.NamedHookable
}

type dramTransactionCountTracer struct {
	tracer *dramTracer
	dram   tracing.NamedHookable
}

type rdmaTransactionCountTracer struct {
	outgoingTracer *tracing.AverageTimeTracer
	incomingTracer *tracing.AverageTimeTracer
	rdmaEngine     *rdma.Comp
}

type simdBusyTimeTracer struct {
	tracer *tracing.BusyTimeTracer
	simd   tracing.NamedHookable
}

type cuCPIStackTracer struct {
	cu     tracing.NamedHookable
	tracer *cu.CPIStackTracer
}

type reporter struct {
	dataRecorder datarecording.DataRecorder

	metricCSV               *csvCollector
	memAccessCounter        *memAccessCounter
	kernelStatsTracer       *kernelStatsTracer
	kernelPageProfileTracer *kernelPageProfileTracer
	cacheEventGroups        *cacheEventGroups
	wgMappingCollector      *wgMappingCollector

	kernelTimeTracer        *kernelTimeTracer
	perGPUKernelTimeTracers []*kernelTimeTracer
	instCountTracers        []*instCountTracer
	cacheLatencyTracers     []*cacheLatencyTracer
	cacheHitRateTracers     []*cacheHitRateTracer
	tlbHitRateTracers       []*tlbHitRateTracer
	dramTracers             []*dramTransactionCountTracer
	rdmaTransactionCounters []*rdmaTransactionCountTracer
	simdBusyTimeTracers     []*simdBusyTimeTracer
	cuCPITraces             []*cuCPIStackTracer

	ReportInstCount            bool
	ReportCacheLatency         bool
	ReportCacheHitRate         bool
	ReportTLBHitRate           bool
	ReportRDMATransactionCount bool
	ReportDRAMTransactionCount bool
	ReportSIMDBusyTime         bool
	ReportCPIStack             bool
}

func newReporter(s *simulation.Simulation) *reporter {
	csvCollector := newCSVCollector()
	dualRecorder := newDualDataRecorder(s.GetDataRecorder(), csvCollector, tableName)

	r := &reporter{
		dataRecorder: dualRecorder,
		metricCSV:    csvCollector,
	}

	r.injectTracers(s)

	r.dataRecorder.CreateTable(tableName, metric{})

	return r
}

func (r *reporter) injectTracers(s *simulation.Simulation) {
	r.injectKernelTimeTracer(s)
	r.injectInstCountTracer(s)
	r.injectCUCPIHook(s)
	r.injectCacheLatencyTracer(s)
	r.injectCacheHitRateTracer(s)
	r.injectTLBHitRateTracer(s)
	r.injectRDMAEngineTracer(s)
	r.injectDRAMTracer(s)
	r.injectSIMDBusyTimeTracer(s)
	r.injectMemAccessTracer(s)
	r.injectKernelStatsTracer(s)
	r.injectKernelPageProfileTracer(s)
	r.injectWGMappingTracer(s)
}

func (r *reporter) injectKernelTimeTracer(s *simulation.Simulation) {
	if *unifiedGPUFlag != "" {
		tracer := tracing.NewBusyTimeTracer(
			s.GetEngine(),
			func(task tracing.Task) bool {
				return task.What == "*driver.LaunchUnifiedMultiGPUKernelCommand"
			})
		tracing.CollectTrace(
			s.GetComponentByName("Driver").(tracing.NamedHookable),
			tracer)
		r.kernelTimeTracer = &kernelTimeTracer{
			tracer: tracer,
			comp:   s.GetComponentByName("Driver").(tracing.NamedHookable),
		}
	} else {
		tracer := tracing.NewBusyTimeTracer(
			s.GetEngine(),
			func(task tracing.Task) bool {
				return task.What == "*driver.LaunchKernelCommand"
			})
		tracing.CollectTrace(
			s.GetComponentByName("Driver").(tracing.NamedHookable),
			tracer)
		r.kernelTimeTracer = &kernelTimeTracer{
			tracer: tracer,
			comp:   s.GetComponentByName("Driver").(tracing.NamedHookable),
		}
	}

	for _, comp := range s.Components() {
		if strings.Contains(comp.Name(), "CommandProcessor") {
			tracer := tracing.NewBusyTimeTracer(
				s.GetEngine(),
				func(task tracing.Task) bool {
					return task.What == "*protocol.LaunchKernelReq"
				})
			tracing.CollectTrace(
				comp.(tracing.NamedHookable),
				tracer)
			r.perGPUKernelTimeTracers = append(
				r.perGPUKernelTimeTracers,
				&kernelTimeTracer{
					tracer: tracer,
					comp:   comp.(tracing.NamedHookable),
				})
		}
	}
}

func (r *reporter) injectInstCountTracer(s *simulation.Simulation) {
	if !*reportAll && !*instCountReportFlag {
		return
	}

	for _, comp := range s.Components() {
		if strings.Contains(comp.Name(), "CU") {
			tracer := newInstTracer()
			r.instCountTracers = append(r.instCountTracers,
				&instCountTracer{
					tracer: tracer,
					cu:     comp.(tracing.NamedHookable),
				})
			tracing.CollectTrace(comp.(tracing.NamedHookable), tracer)
		}
	}
}

func (r *reporter) injectCUCPIHook(s *simulation.Simulation) {
	if !*reportAll && !*reportCPIStackFlag {
		return
	}

	for _, comp := range s.Components() {
		if strings.Contains(comp.Name(), "CU") {
			tracer := cu.NewCPIStackInstHook(
				comp.(*cu.ComputeUnit), s.GetEngine())
			tracing.CollectTrace(comp.(tracing.NamedHookable), tracer)

			r.cuCPITraces = append(r.cuCPITraces,
				&cuCPIStackTracer{
					tracer: tracer,
					cu:     comp.(tracing.NamedHookable),
				})
		}
	}
}

func (r *reporter) injectCacheLatencyTracer(s *simulation.Simulation) {
	if !*reportAll && !*cacheLatencyReportFlag {
		return
	}

	for _, comp := range s.Components() {
		if strings.Contains(comp.Name(), "Cache") {
			tracer := tracing.NewAverageTimeTracer(
				s.GetEngine(),
				func(task tracing.Task) bool {
					return task.Kind == "req_in"
				})
			r.cacheLatencyTracers = append(r.cacheLatencyTracers,
				&cacheLatencyTracer{
					tracer: tracer,
					cache:  comp.(tracing.NamedHookable),
				})
			tracing.CollectTrace(comp.(tracing.NamedHookable), tracer)
		}
	}
}

func (r *reporter) injectCacheHitRateTracer(s *simulation.Simulation) {
	// We also need cache event counters for per-kernel metrics.
	if !(*reportAll || *cacheHitRateReportFlag || *kernelMetricsReportFlag) {
		return
	}

	if r.cacheEventGroups == nil {
		r.cacheEventGroups = newCacheEventGroups()
	}

	for _, comp := range s.Components() {
		nh, ok := comp.(tracing.NamedHookable)
		if !ok {
			continue
		}

		name := nh.Name()
		// Cache components normally contain "Cache". The chiplet builder may name
		// L2 slices as ".L2_<id>".
		if !strings.Contains(name, "Cache") &&
			!strings.Contains(name, ".L2_") &&
			!strings.Contains(name, ".L1V_") && !strings.Contains(name, ".L1V") &&
			!strings.Contains(name, ".L1I_") && !strings.Contains(name, ".L1I") &&
			!strings.Contains(name, ".L1S_") && !strings.Contains(name, ".L1S") {

			continue
		}

		// Grouping for per-kernel deltas / power modeling.
		var group *cacheEventCounter
		switch {
		case strings.Contains(name, "L1VCache") || strings.Contains(name, ".L1V_") || strings.Contains(name, ".L1V"):
			group = &r.cacheEventGroups.L1V
		case strings.Contains(name, "L1ICache") || strings.Contains(name, ".L1I_") || strings.Contains(name, ".L1I"):
			group = &r.cacheEventGroups.L1I
		case strings.Contains(name, "L1SCache") || strings.Contains(name, ".L1S_") || strings.Contains(name, ".L1S"):
			group = &r.cacheEventGroups.L1S
		case strings.Contains(name, "L2Cache") || strings.Contains(name, ".L2_"):
			group = &r.cacheEventGroups.L2
		default:
			group = nil
		}

		// Per-cache counter (for the global metrics.csv).
		perCache := &cacheEventCounter{}
		tracer := newCacheEventTracer(perCache, group)

		r.cacheHitRateTracers = append(r.cacheHitRateTracers,
			&cacheHitRateTracer{
				tracer: tracer,
				cache:  nh,
			})

		tracing.CollectTrace(nh, tracer)
	}
}

func (r *reporter) injectTLBHitRateTracer(s *simulation.Simulation) {
	if !*reportAll && !*tlbHitRateReportFlag {
		return
	}

	for _, comp := range s.Components() {
		if strings.Contains(comp.Name(), "TLB") {
			tracer := tracing.NewStepCountTracer(
				func(task tracing.Task) bool { return true })
			r.tlbHitRateTracers = append(r.tlbHitRateTracers,
				&tlbHitRateTracer{
					tracer: tracer,
					tlb:    comp.(tracing.NamedHookable),
				})
			tracing.CollectTrace(comp.(tracing.NamedHookable), tracer)
		}
	}
}

func (r *reporter) injectRDMAEngineTracer(s *simulation.Simulation) {
	if !*reportAll && !*rdmaTransactionCountReportFlag {
		return
	}

	for _, comp := range s.Components() {
		if strings.Contains(comp.Name(), "RDMA") {
			t := &rdmaTransactionCountTracer{}
			t.rdmaEngine = comp.(*rdma.Comp)
			t.incomingTracer = tracing.NewAverageTimeTracer(
				s.GetEngine(),
				func(task tracing.Task) bool {
					if task.Kind != "req_in" {
						return false
					}

					isFromOutside := strings.Contains(
						string(task.Detail.(sim.Msg).Meta().Src), "RDMA")
					if !isFromOutside {
						return false
					}

					return true
				})
			t.outgoingTracer = tracing.NewAverageTimeTracer(
				s.GetEngine(),
				func(task tracing.Task) bool {
					if task.Kind != "req_in" {
						return false
					}

					isFromOutside := strings.Contains(
						string(task.Detail.(sim.Msg).Meta().Src), "RDMA")
					if isFromOutside {
						return false
					}

					return true
				})

			tracing.CollectTrace(t.rdmaEngine, t.incomingTracer)
			tracing.CollectTrace(t.rdmaEngine, t.outgoingTracer)

			r.rdmaTransactionCounters = append(r.rdmaTransactionCounters, t)
		}
	}
}

func (r *reporter) injectDRAMTracer(s *simulation.Simulation) {
	if !*reportAll && !*dramTransactionCountReportFlag {
		return
	}

	for _, comp := range s.Components() {
		if strings.Contains(comp.Name(), "DRAM") {
			t := &dramTransactionCountTracer{}
			t.dram = comp.(tracing.NamedHookable)
			t.tracer = newDramTracer(s.GetEngine())

			tracing.CollectTrace(t.dram, t.tracer)

			r.dramTracers = append(r.dramTracers, t)
		}
	}
}

func (r *reporter) injectSIMDBusyTimeTracer(s *simulation.Simulation) {
	if !*reportAll && !*simdBusyTimeTracerFlag {
		return
	}

	for _, comp := range s.Components() {
		if strings.Contains(comp.Name(), "SIMD") {
			perSIMDBusyTimeTracer := tracing.NewBusyTimeTracer(
				s.GetEngine(),
				func(task tracing.Task) bool {
					return task.Kind == "pipeline"
				})
			r.simdBusyTimeTracers = append(r.simdBusyTimeTracers,
				&simdBusyTimeTracer{
					tracer: perSIMDBusyTimeTracer,
					simd:   comp.(tracing.NamedHookable),
				})
			tracing.CollectTrace(comp.(tracing.NamedHookable), perSIMDBusyTimeTracer)
		}
	}
}

func (r *reporter) report() {
	r.reportKernelTime()
	// Total simulation time is useful for background-energy accounting.
	r.reportSimulationTime()
	r.reportInstCount()
	r.reportCPIStack()
	r.reportSIMDBusyTime()
	r.reportCacheLatency()
	r.reportCacheHitRate()
	r.reportTLBHitRate()
	r.reportRDMATransactionCount()
	r.reportDRAMTransactionCount()
	r.reportMemAccessSummary()
	// Extra summaries for power modeling.
	r.reportRDMATrafficSummary()
	r.reportDRAMAccessSummary()
	r.reportPTWSummary()
	// Dump CSV files after all metrics have been inserted.
	r.dumpMetricsCSVIfEnabled()
	r.dumpKernelMetricsCSVIfEnabled()
	r.dumpWGMappingCSVIfEnabled()
	r.dumpProfileJSONIfEnabled()
}

func (r *reporter) reportKernelTime() {
	kernelTime := float64(r.kernelTimeTracer.tracer.BusyTime())
	r.dataRecorder.InsertData(
		tableName,
		metric{
			Location: r.kernelTimeTracer.comp.Name(),
			What:     "kernel_time",
			Value:    kernelTime,
			Unit:     "second",
		},
	)

	for _, t := range r.perGPUKernelTimeTracers {
		kernelTime := float64(t.tracer.BusyTime())
		r.dataRecorder.InsertData(
			tableName,
			metric{
				Location: t.comp.Name(),
				What:     "kernel_time",
				Value:    kernelTime,
				Unit:     "second",
			},
		)
	}
}

func (r *reporter) reportInstCount() {
	kernelTime := float64(r.kernelTimeTracer.tracer.BusyTime())
	for _, t := range r.instCountTracers {
		cuFreq := float64(t.cu.(*cu.ComputeUnit).Freq)
		numCycle := kernelTime * cuFreq

		// Avoid NaN/Inf when the instruction counter is zero.
		cuCPI := 0.0
		if t.tracer.count > 0 {
			cuCPI = numCycle / float64(t.tracer.count)
		}

		simdCPI := 0.0
		if t.tracer.simdCount > 0 {
			simdCPI = numCycle / float64(t.tracer.simdCount)
		}

		r.dataRecorder.InsertData(
			tableName,
			metric{
				Location: t.cu.Name(),
				What:     "cu_inst_count",
				Value:    float64(t.tracer.count),
				Unit:     "count",
			},
		)

		r.dataRecorder.InsertData(
			tableName,
			metric{
				Location: t.cu.Name(),
				What:     "cu_CPI",
				Value:    cuCPI,
				Unit:     "cycles/inst",
			},
		)

		r.dataRecorder.InsertData(
			tableName,
			metric{
				Location: t.cu.Name(),
				What:     "simd_inst_count",
				Value:    float64(t.tracer.simdCount),
				Unit:     "count",
			},
		)

		r.dataRecorder.InsertData(
			tableName,
			metric{
				Location: t.cu.Name(),
				What:     "simd_CPI",
				Value:    simdCPI,
				Unit:     "cycles/inst",
			},
		)
	}
}

func (r *reporter) reportCPIStack() {
	for _, t := range r.cuCPITraces {
		cu := t.cu
		hook := t.tracer

		r.reportCPIStackEntries(hook, cu, false)
		r.reportCPIStackEntries(hook, cu, true)
	}
}

func (r *reporter) reportCPIStackEntries(
	hook *cu.CPIStackTracer,
	cu tracing.NamedHookable,
	simdStack bool,
) {
	cpiStack := hook.GetCPIStack()
	if simdStack {
		cpiStack = hook.GetSIMDCPIStack()
	}

	keys := make([]string, 0, len(cpiStack))
	for k := range cpiStack {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	stackTypeName := "CPIStack"
	if simdStack {
		stackTypeName = "SIMDCPIStack"
	}

	for _, name := range keys {
		value := cpiStack[name]
		r.dataRecorder.InsertData(
			tableName,
			metric{
				Location: cu.Name(),
				What:     stackTypeName + "." + name,
				Value:    value,
				Unit:     "cycles/inst",
			},
		)
	}
}

func (r *reporter) reportSIMDBusyTime() {
	for _, t := range r.simdBusyTimeTracers {
		r.dataRecorder.InsertData(
			tableName,
			metric{
				Location: t.simd.Name(),
				What:     "busy_time",
				Value:    float64(t.tracer.BusyTime()),
				Unit:     "second",
			},
		)
	}
}

func (r *reporter) reportCacheLatency() {
	for _, tracer := range r.cacheLatencyTracers {
		if tracer.tracer.AverageTime() == 0 {
			continue
		}

		r.dataRecorder.InsertData(
			tableName,
			metric{
				Location: tracer.cache.Name(),
				What:     "req_average_latency",
				Value:    float64(tracer.tracer.AverageTime()),
				Unit:     "second",
			},
		)
	}
}

func (r *reporter) reportCacheHitRate() {
	for _, tracer := range r.cacheHitRateTracers {
		c := tracer.tracer.counter.Snapshot()

		readHit := c.readHit
		readMiss := c.readMiss
		readMSHRHit := c.readMSHRHit
		writeHit := c.writeHit
		writeMiss := c.writeMiss
		writeMSHRHit := c.writeMSHRHit

		totalTransaction := readHit + readMiss + readMSHRHit +
			writeHit + writeMiss + writeMSHRHit

		if totalTransaction == 0 {
			continue
		}

		r.dataRecorder.InsertData(tableName, metric{
			Location: tracer.cache.Name(),
			What:     "read-hit",
			Value:    float64(readHit),
			Unit:     "count",
		})
		r.dataRecorder.InsertData(tableName, metric{
			Location: tracer.cache.Name(),
			What:     "read-miss",
			Value:    float64(readMiss),
			Unit:     "count",
		})
		r.dataRecorder.InsertData(tableName, metric{
			Location: tracer.cache.Name(),
			What:     "read-mshr-hit",
			Value:    float64(readMSHRHit),
			Unit:     "count",
		})
		r.dataRecorder.InsertData(tableName, metric{
			Location: tracer.cache.Name(),
			What:     "write-hit",
			Value:    float64(writeHit),
			Unit:     "count",
		})
		r.dataRecorder.InsertData(tableName, metric{
			Location: tracer.cache.Name(),
			What:     "write-miss",
			Value:    float64(writeMiss),
			Unit:     "count",
		})
		r.dataRecorder.InsertData(tableName, metric{
			Location: tracer.cache.Name(),
			What:     "write-mshr-hit",
			Value:    float64(writeMSHRHit),
			Unit:     "count",
		})
	}
}

func (r *reporter) reportTLBHitRate() {
	for _, tracer := range r.tlbHitRateTracers {
		hit := tracer.tracer.GetStepCount("hit")
		miss := tracer.tracer.GetStepCount("miss")
		mshrHit := tracer.tracer.GetStepCount("mshr-hit")

		totalTransaction := hit + miss + mshrHit

		if totalTransaction == 0 {
			continue
		}

		r.dataRecorder.InsertData(
			tableName,
			metric{
				Location: tracer.tlb.Name(),
				What:     "hit",
				Value:    float64(hit),
				Unit:     "count",
			},
		)
		r.dataRecorder.InsertData(
			tableName,
			metric{
				Location: tracer.tlb.Name(),
				What:     "miss",
				Value:    float64(miss),
				Unit:     "count",
			},
		)
		r.dataRecorder.InsertData(
			tableName,
			metric{
				Location: tracer.tlb.Name(),
				What:     "mshr-hit",
				Value:    float64(mshrHit),
				Unit:     "count",
			},
		)
	}
}

func (r *reporter) reportRDMATransactionCount() {
	for _, t := range r.rdmaTransactionCounters {
		r.dataRecorder.InsertData(
			tableName,
			metric{
				Location: t.rdmaEngine.Name(),
				What:     "outgoing_trans_count",
				Value:    float64(t.outgoingTracer.TotalCount()),
				Unit:     "count",
			},
		)
		r.dataRecorder.InsertData(
			tableName,
			metric{
				Location: t.rdmaEngine.Name(),
				What:     "incoming_trans_count",
				Value:    float64(t.incomingTracer.TotalCount()),
				Unit:     "count",
			},
		)
	}
}

func (r *reporter) reportDRAMTransactionCount() {
	for _, t := range r.dramTracers {
		r.dataRecorder.InsertData(
			tableName,
			metric{
				Location: t.dram.Name(),
				What:     "read_trans_count",
				Value:    float64(t.tracer.readCount),
				Unit:     "count",
			},
		)
		r.dataRecorder.InsertData(
			tableName,
			metric{
				Location: t.dram.Name(),
				What:     "write_trans_count",
				Value:    float64(t.tracer.writeCount),
				Unit:     "count",
			},
		)
		r.dataRecorder.InsertData(
			tableName,
			metric{
				Location: t.dram.Name(),
				What:     "read_avg_latency",
				Value:    float64(t.tracer.readAvgLatency),
				Unit:     "second",
			},
		)
		r.dataRecorder.InsertData(
			tableName,
			metric{
				Location: t.dram.Name(),
				What:     "write_avg_latency",
				Value:    float64(t.tracer.writeAvgLatency),
				Unit:     "second",
			},
		)
		r.dataRecorder.InsertData(
			tableName,
			metric{
				Location: t.dram.Name(),
				What:     "read_size",
				Value:    float64(t.tracer.readSize),
				Unit:     "bytes",
			},
		)
		r.dataRecorder.InsertData(
			tableName,
			metric{
				Location: t.dram.Name(),
				What:     "write_size",
				Value:    float64(t.tracer.writeSize),
				Unit:     "bytes",
			},
		)
	}
}

func (r *reporter) reportSimulationTime() {
	// The engine time at the end of the run.
	// Note: this includes any non-kernel activities (e.g., memory init, verification).
	totalTime := float64(r.kernelStatsTracer.CurrentTime())
	r.dataRecorder.InsertData(
		tableName,
		metric{
			Location: "Simulation",
			What:     "total_time",
			Value:    totalTime,
			Unit:     "second",
		},
	)
}

func (r *reporter) reportRDMATrafficSummary() {
	snap := r.kernelStatsTracer.rdmaCounter.Snapshot()
	loc := "RDMATraffic"

	r.dataRecorder.InsertData(tableName, metric{Location: loc, What: "nop_send_packets", Value: float64(snap.nopSendPackets), Unit: "count"})
	r.dataRecorder.InsertData(tableName, metric{Location: loc, What: "nop_send_bytes", Value: float64(snap.nopSendBytes), Unit: "byte"})
	r.dataRecorder.InsertData(tableName, metric{Location: loc, What: "nop_recv_packets", Value: float64(snap.nopRecvPackets), Unit: "count"})
	r.dataRecorder.InsertData(tableName, metric{Location: loc, What: "nop_recv_bytes", Value: float64(snap.nopRecvBytes), Unit: "byte"})

	r.dataRecorder.InsertData(tableName, metric{Location: loc, What: "noc_send_packets", Value: float64(snap.nocSendPackets), Unit: "count"})
	r.dataRecorder.InsertData(tableName, metric{Location: loc, What: "noc_send_bytes", Value: float64(snap.nocSendBytes), Unit: "byte"})
	r.dataRecorder.InsertData(tableName, metric{Location: loc, What: "noc_recv_packets", Value: float64(snap.nocRecvPackets), Unit: "count"})
	r.dataRecorder.InsertData(tableName, metric{Location: loc, What: "noc_recv_bytes", Value: float64(snap.nocRecvBytes), Unit: "byte"})

	r.dataRecorder.InsertData(tableName, metric{Location: loc, What: "rdma_req_send_packets", Value: float64(snap.rdmaReqSendPackets), Unit: "count"})
	r.dataRecorder.InsertData(tableName, metric{Location: loc, What: "rdma_req_send_bytes", Value: float64(snap.rdmaReqSendBytes), Unit: "byte"})
}

func (r *reporter) reportDRAMAccessSummary() {
	snap := r.kernelStatsTracer.dramCounter.Snapshot()
	loc := "DRAMAccess"

	r.dataRecorder.InsertData(tableName, metric{Location: loc, What: "read_req_count", Value: float64(snap.readReq), Unit: "count"})
	r.dataRecorder.InsertData(tableName, metric{Location: loc, What: "read_req_bytes", Value: float64(snap.readBytes), Unit: "byte"})
	r.dataRecorder.InsertData(tableName, metric{Location: loc, What: "write_req_count", Value: float64(snap.writeReq), Unit: "count"})
	r.dataRecorder.InsertData(tableName, metric{Location: loc, What: "write_req_bytes", Value: float64(snap.writeBytes), Unit: "byte"})
}
