package runner

import (
	"fmt"
	"strings"

	"github.com/sarchlab/akita/v4/mem/vm/ptw"
	"github.com/sarchlab/akita/v4/sim"
	"github.com/sarchlab/akita/v4/simulation"
	"github.com/sarchlab/akita/v4/tracing"
	"github.com/sarchlab/mgpusim/v4/amd/driver"
	"github.com/sarchlab/mgpusim/v4/amd/timing/cu"
	"github.com/sarchlab/mgpusim/v4/amd/timing/rdma"
)

// injectMemAccessTracer attaches a single l2AccessTracer to all L2 caches.
// The tracer will maintain global counters that can be snapshotted per-kernel.
func (r *reporter) injectMemAccessTracer(s *simulation.Simulation) {
	if !(*reportAll || *memAccessReportFlag || *kernelMetricsReportFlag) {
		return
	}

	r.memAccessCounter = newMemAccessCounter()
	tracer := &l2AccessTracer{counter: r.memAccessCounter}

	components := s.Components()
	for _, comp := range components {
		nh, ok := comp.(tracing.NamedHookable)
		if !ok {
			continue
		}

		name := nh.Name()
		// In current chiplet builder, L2 caches are named like:
		//   GPU0.chiplet_00.L2_0
		// Older versions may use ".L2Cache".
		if strings.Contains(name, ".L2_") || strings.Contains(name, ".L2Cache") {
			tracing.CollectTrace(nh, tracer)
		}
	}
}

// injectKernelStatsTracer attaches a tracer to the Driver to record per-kernel
// latency and memory locality deltas.
func (r *reporter) injectKernelStatsTracer(s *simulation.Simulation) {
	if !(*reportAll || *kernelMetricsReportFlag) {
		return
	}

	if r.memAccessCounter == nil {
		// The kernel tracer is more useful when memory
		// access tracing is enabled,
		// but it can still record latency even without it.
		r.memAccessCounter = newMemAccessCounter()
	}

	// Also enable per-kernel DRAM request counting.
	dramCounter := newDramAccessCounter()
	dramTracer := &dramAccessTracer{counter: dramCounter}

	// Per-kernel RDMA/NoP/NoC traffic counters.
	rdmaCounter := newRDMATrafficCounter()

	// Per-kernel TLB event counters.
	tlbGroups := &tlbEventGroups{}

	// Per-kernel PTW/PWC stats are obtained by sampling PTW components.
	var ptwCounter *ptwStatsCounter
	ptwComps := make([]*ptw.Comp, 0, 8)
	components := s.Components()
	for _, comp := range components {
		if p, ok := comp.(*ptw.Comp); ok {
			ptwComps = append(ptwComps, p)
		}
	}
	if len(ptwComps) > 0 {
		ptwCounter = newPTWStatsCounter(ptwComps)
	}
	for _, comp := range components {
		nh, ok := comp.(tracing.NamedHookable)
		if !ok {
			continue
		}

		name := nh.Name()
		if strings.Contains(name, ".DRAM") {
			tracing.CollectTrace(nh, dramTracer)
		}

		// Attach port hooks to chiplet RDMA components to count NoP/NoC traffic.
		if rdmaEngine, ok := comp.(*rdma.Comp); ok {
			// Only count chiplet RDMA (e.g., GPU0.chiplet_00.RDMA), ignore the GPU-level RDMA.
			if strings.Contains(rdmaEngine.Name(), "chiplet_") || strings.Contains(rdmaEngine.Name(), "Chiplet[") || strings.Contains(rdmaEngine.Name(), ".Chiplet[") || strings.Contains(rdmaEngine.Name(), ".Chiplet") {
				hookSendNoP := &msgCounterHook{pos: sim.HookPosPortMsgSend, packets: &rdmaCounter.nopSendPackets, bytes: &rdmaCounter.nopSendBytes}
				hookRecvNoP := &msgCounterHook{pos: sim.HookPosPortMsgRecvd, packets: &rdmaCounter.nopRecvPackets, bytes: &rdmaCounter.nopRecvBytes}
				hookSendNoC := &msgCounterHook{pos: sim.HookPosPortMsgSend, packets: &rdmaCounter.nocSendPackets, bytes: &rdmaCounter.nocSendBytes}
				hookRecvNoC := &msgCounterHook{pos: sim.HookPosPortMsgRecvd, packets: &rdmaCounter.nocRecvPackets, bytes: &rdmaCounter.nocRecvBytes}
				hookSendReq := &msgCounterHook{pos: sim.HookPosPortMsgSend, packets: &rdmaCounter.rdmaReqSendPackets, bytes: &rdmaCounter.rdmaReqSendBytes}

				// Outside ports: NoP traffic.
				rdmaEngine.RDMARequestOutside.AcceptHook(hookSendNoP)
				rdmaEngine.RDMARequestOutside.AcceptHook(hookRecvNoP)
				rdmaEngine.RDMADataOutside.AcceptHook(hookSendNoP)
				rdmaEngine.RDMADataOutside.AcceptHook(hookRecvNoP)

				// Inside ports: NoC traffic at RDMA boundary.
				rdmaEngine.RDMARequestInside.AcceptHook(hookSendNoC)
				rdmaEngine.RDMARequestInside.AcceptHook(hookRecvNoC)
				rdmaEngine.RDMADataInside.AcceptHook(hookSendNoC)
				rdmaEngine.RDMADataInside.AcceptHook(hookRecvNoC)

				// RDMA request traffic: request messages sent to the outside.
				rdmaEngine.RDMARequestOutside.AcceptHook(hookSendReq)
			}
		}

		// Attach per-kernel TLB event tracers.
		if strings.Contains(name, "TLB") {
			var c *tlbEventCounter
			switch {
			case strings.Contains(name, "L2TLB"):
				c = &tlbGroups.L2
			case strings.Contains(name, "L1VTLB"):
				c = &tlbGroups.L1V
			case strings.Contains(name, "L1ITLB"):
				c = &tlbGroups.L1I
			case strings.Contains(name, "L1STLB"):
				c = &tlbGroups.L1S
			default:
				// Unknown TLB type; treat as L2 by default.
				c = &tlbGroups.L2
			}
			tracing.CollectTrace(nh, &tlbEventTracer{counter: c})
		}
	}

	// Attach kernel tracer to the Driver.
	for _, comp := range components {
		nh, ok := comp.(tracing.NamedHookable)
		if !ok {
			continue
		}
		if nh.Name() == "Driver" {
			r.kernelStatsTracer = newKernelStatsTracer(
				s.GetEngine(),
				r.memAccessCounter,
				dramCounter,
				r.cacheEventGroups,
				rdmaCounter,
				ptwCounter,
				tlbGroups,
			)
			tracing.CollectTrace(nh, r.kernelStatsTracer)
			return
		}
	}
}

func (r *reporter) reportMemAccessSummary() {
	if r.memAccessCounter == nil {
		return
	}

	s := r.memAccessCounter.Snapshot()

	where := "MemoryAccess"
	r.dataRecorder.InsertData(
		tableName,
		metric{Location: where, What: "l2_local_reads", Value: float64(s.localRead), Unit: "count"},
	)
	r.dataRecorder.InsertData(
		tableName,
		metric{Location: where, What: "l2_local_writes", Value: float64(s.localWrite), Unit: "count"},
	)
	r.dataRecorder.InsertData(
		tableName,
		metric{Location: where, What: "l2_remote_reads", Value: float64(s.remoteRead), Unit: "count"},
	)
	r.dataRecorder.InsertData(
		tableName,
		metric{Location: where, What: "l2_remote_writes", Value: float64(s.remoteWrite), Unit: "count"},
	)
	r.dataRecorder.InsertData(
		tableName,
		metric{Location: where, What: "l2_external_reads", Value: float64(s.externalRead), Unit: "count"},
	)
	r.dataRecorder.InsertData(
		tableName,
		metric{Location: where, What: "l2_external_writes", Value: float64(s.externalWrite), Unit: "count"},
	)

	r.dataRecorder.InsertData(
		tableName,
		metric{Location: where, What: "l2_local_bytes_est", Value: float64(s.localBytes), Unit: "byte"},
	)
	r.dataRecorder.InsertData(
		tableName,
		metric{Location: where, What: "l2_remote_bytes_est", Value: float64(s.remoteBytes), Unit: "byte"},
	)
	r.dataRecorder.InsertData(
		tableName,
		metric{Location: where, What: "l2_external_bytes_est", Value: float64(s.externalBytes), Unit: "byte"},
	)

	total := float64(s.localRead + s.localWrite + s.remoteRead + s.remoteWrite + s.externalRead + s.externalWrite)
	remote := float64(s.remoteRead + s.remoteWrite)
	ratio := 0.0
	if total > 0 {
		ratio = remote / total
	}

	r.dataRecorder.InsertData(
		tableName,
		metric{Location: where, What: "l2_remote_access_ratio", Value: ratio, Unit: "ratio"},
	)
}

func (r *reporter) dumpKernelMetricsCSVIfEnabled() {
	if !shouldDumpKernelMetricsCSV() {
		return
	}
	if r.kernelStatsTracer == nil {
		return
	}

	baseName := fmt.Sprintf("%s_kernels", *filenameFlag)
	if err := r.kernelStatsTracer.Dump(baseName); err != nil {
		fmt.Printf("failed to dump kernel metrics CSV: %v\n", err)
	}
}

func shouldDumpKernelMetricsCSV() bool {
	return *reportAll || *kernelMetricsReportFlag
}

func (r *reporter) dumpMetricsCSVIfEnabled() {
	if r.metricCSV == nil {
		return
	}

	if !shouldDumpMetricsCSV() {
		return
	}

	if err := r.metricCSV.Dump(*filenameFlag); err != nil {
		fmt.Printf("failed to dump metrics CSV: %v\n", err)
	}
}

func shouldDumpMetricsCSV() bool {
	return *reportAll ||
		*instCountReportFlag ||
		*cacheLatencyReportFlag ||
		*cacheHitRateReportFlag ||
		*tlbHitRateReportFlag ||
		*rdmaTransactionCountReportFlag ||
		*dramTransactionCountReportFlag ||
		*simdBusyTimeTracerFlag ||
		*reportCPIStackFlag ||
		*memAccessReportFlag ||
		*kernelMetricsReportFlag
}

func (r *reporter) injectKernelPageProfileTracer(s *simulation.Simulation) {
	if *profileJSONFlag == "" {
		return
	}

	driverComp := s.GetComponentByName("Driver")
	d, ok := driverComp.(*driver.Driver)
	if !ok {
		// In case the component is wrapped, try the interface cast.
		// If it still fails, skip profiling.
		return
	}

	tracer := newKernelPageProfileTracer(d)
	r.kernelPageProfileTracer = tracer

	if hookable, ok := driverComp.(tracing.NamedHookable); ok {
		tracing.CollectTrace(hookable, tracer)
	}

	// Collect L2 request traces.
	for _, comp := range s.Components() {
		if !strings.Contains(comp.Name(), "L2") {
			continue
		}
		if cache, ok := comp.(tracing.NamedHookable); ok {
			tracing.CollectTrace(cache, tracer)
		}
	}

	// Collect exact work-group dispatch/completion timing from the CU dispatch
	// port itself. The generic tracing Task API does not populate Task.StartTime
	// for task-start / task-end hooks, so we must read the current simulated
	// time from the owning CU at the port hook site.
	for _, comp := range s.Components() {
		cuComp, ok := comp.(*cu.ComputeUnit)
		if !ok || cuComp == nil || cuComp.ToACE == nil {
			continue
		}
		_, saGlobalID, _, ok := parseCUName(cuComp.Name())
		if !ok {
			continue
		}
		chipletID, _ := chipletAndLocalSA(saGlobalID)
		cuComp.ToACE.AcceptHook(newKernelWGTimingPortHook(tracer, chipletID, cuComp.Name()))
	}
}

func (r *reporter) dumpProfileJSONIfEnabled() {
	if *profileJSONFlag == "" {
		return
	}
	if r.kernelPageProfileTracer == nil {
		return
	}

	if err := r.kernelPageProfileTracer.DumpJSON(*profileJSONFlag); err != nil {
		fmt.Printf("Failed to dump profile json: %v\n", err)
		return
	}
	fmt.Printf("Profile JSON dumped to %s (%s)\n", *profileJSONFlag, r.kernelPageProfileTracer.SummaryString())
}
