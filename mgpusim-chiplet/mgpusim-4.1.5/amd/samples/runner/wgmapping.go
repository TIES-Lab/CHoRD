package runner

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/sarchlab/akita/v4/sim"
	"github.com/sarchlab/akita/v4/simulation"
	"github.com/sarchlab/akita/v4/tracing"
	"github.com/sarchlab/mgpusim/v4/amd/protocol"
	"github.com/sarchlab/mgpusim/v4/amd/timing/cu"
)

type wgMappingRecord struct {
	KernelName      string
	WGUID           string
	PacketAddr      uint64
	LaunchAbsID     int
	LaunchRelID     int
	ProgramLaunchID int
	WGWindowStart   int
	WGWindowCount   int
	WGIDX           int
	WGIDY           int
	WGIDZ           int
	SubWGFactor     int
	SubWGIndex      int
	LocalXOffset    int
	LocalYOffset    int
	SubSizeX        int
	SubSizeY        int
	SubSizeZ        int
	DispatchTime    sim.VTimeInSec
	CompletionTime  sim.VTimeInSec
	HasCompletion   bool

	GPUID    int
	Chiplet  int
	SAGlobal int
	SA       int
	CU       int
	CUName   string
}

type wgMappingCollector struct {
	mu          sync.Mutex
	records     []wgMappingRecord
	inflightReq map[string]int
}

func newWGMappingCollector() *wgMappingCollector {
	return &wgMappingCollector{records: make([]wgMappingRecord, 0, 4096), inflightReq: make(map[string]int)}
}

func (c *wgMappingCollector) AddDispatch(reqID string, r wgMappingRecord) {
	c.mu.Lock()
	defer c.mu.Unlock()
	idx := len(c.records)
	c.records = append(c.records, r)
	if reqID != "" {
		c.inflightReq[reqID] = idx
	}
}

func (c *wgMappingCollector) MarkCompletion(reqID string, completion sim.VTimeInSec) {
	if reqID == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	idx, ok := c.inflightReq[reqID]
	if !ok || idx < 0 || idx >= len(c.records) {
		return
	}
	c.records[idx].CompletionTime = completion
	c.records[idx].HasCompletion = true
	delete(c.inflightReq, reqID)
}

func (c *wgMappingCollector) DumpCSV(baseName string) error {
	c.mu.Lock()
	recs := make([]wgMappingRecord, len(c.records))
	copy(recs, c.records)
	c.mu.Unlock()

	sort.Slice(recs, func(i, j int) bool {
		if recs[i].KernelName != recs[j].KernelName {
			return recs[i].KernelName < recs[j].KernelName
		}
		if recs[i].DispatchTime != recs[j].DispatchTime {
			return recs[i].DispatchTime < recs[j].DispatchTime
		}
		if recs[i].WGIDZ != recs[j].WGIDZ {
			return recs[i].WGIDZ < recs[j].WGIDZ
		}
		if recs[i].WGIDY != recs[j].WGIDY {
			return recs[i].WGIDY < recs[j].WGIDY
		}
		if recs[i].WGIDX != recs[j].WGIDX {
			return recs[i].WGIDX < recs[j].WGIDX
		}
		return recs[i].SubWGIndex < recs[j].SubWGIndex
	})

	f, err := os.Create(baseName + ".csv")
	if err != nil {
		return err
	}
	defer func() {
		_ = f.Close()
	}()

	w := csv.NewWriter(f)
	defer w.Flush()

	_ = w.Write([]string{
		"kernel",
		"wg_uid",
		"packet_addr",
		"launch_abs_id",
		"launch_rel_id",
		"program_launch_id",
		"wg_window_start",
		"wg_window_count",
		"wg_x",
		"wg_y",
		"wg_z",
		"sub_wg_factor",
		"sub_wg_index",
		"local_x_offset",
		"local_y_offset",
		"sub_wg_size_x",
		"sub_wg_size_y",
		"sub_wg_size_z",
		"dispatch_time",
		"completion_time",
		"duration_sec",
		"gpu",
		"chiplet",
		"sa_global",
		"sa",
		"cu",
		"cu_name",
	})

	for _, r := range recs {
		duration := 0.0
		completion := ""
		if r.HasCompletion {
			completion = fmt.Sprintf("%.9f", float64(r.CompletionTime))
			if r.CompletionTime > r.DispatchTime {
				duration = float64(r.CompletionTime - r.DispatchTime)
			}
		}
		_ = w.Write([]string{
			r.KernelName,
			r.WGUID,
			strconv.FormatUint(r.PacketAddr, 10),
			strconv.Itoa(r.LaunchAbsID),
			strconv.Itoa(r.LaunchRelID),
			strconv.Itoa(r.ProgramLaunchID),
			strconv.Itoa(r.WGWindowStart),
			strconv.Itoa(r.WGWindowCount),
			strconv.Itoa(r.WGIDX),
			strconv.Itoa(r.WGIDY),
			strconv.Itoa(r.WGIDZ),
			strconv.Itoa(r.SubWGFactor),
			strconv.Itoa(r.SubWGIndex),
			strconv.Itoa(r.LocalXOffset),
			strconv.Itoa(r.LocalYOffset),
			strconv.Itoa(r.SubSizeX),
			strconv.Itoa(r.SubSizeY),
			strconv.Itoa(r.SubSizeZ),
			fmt.Sprintf("%.9f", float64(r.DispatchTime)),
			completion,
			fmt.Sprintf("%.9f", duration),
			strconv.Itoa(r.GPUID),
			strconv.Itoa(r.Chiplet),
			strconv.Itoa(r.SAGlobal),
			strconv.Itoa(r.SA),
			strconv.Itoa(r.CU),
			r.CUName,
		})
	}

	return nil
}

type wgMappingPortHook struct {
	collector *wgMappingCollector
	gpuID     int
	chipletID int
	saGlobal  int
	saLocal   int
	cuID      int
	cuName    string
}

func newWGMappingPortHook(
	collector *wgMappingCollector,
	gpuID, chipletID, saGlobalID, saLocalID, cuID int,
	cuName string,
) *wgMappingPortHook {
	return &wgMappingPortHook{
		collector: collector,
		gpuID:     gpuID,
		chipletID: chipletID,
		saGlobal:  saGlobalID,
		saLocal:   saLocalID,
		cuID:      cuID,
		cuName:    cuName,
	}
}

func (h *wgMappingPortHook) Func(ctx sim.HookCtx) {
	if h == nil || h.collector == nil {
		return
	}
	now, ok := currentTimeFromPortHook(ctx)
	if !ok {
		return
	}
	switch ctx.Pos {
	case sim.HookPosPortMsgRetrieveIncoming:
		req, ok := ctx.Item.(*protocol.MapWGReq)
		if !ok || req == nil {
			if msg, ok := ctx.Item.(sim.Msg); ok {
				req, _ = msg.(*protocol.MapWGReq)
			}
		}
		if req == nil || req.WorkGroup == nil {
			return
		}
		kernelName := "unknown"
		if req.WorkGroup.CodeObject != nil && req.WorkGroup.CodeObject.Symbol != nil {
			kernelName = req.WorkGroup.CodeObject.Symbol.Name
		}
		h.collector.AddDispatch(req.ID, wgMappingRecord{
			KernelName:      kernelName,
			WGUID:           req.WorkGroup.UID,
			PacketAddr:      packetAddrFromWG(req),
			LaunchAbsID:     req.WorkGroup.LaunchAbsID,
			LaunchRelID:     req.WorkGroup.LaunchRelID,
			ProgramLaunchID: req.WorkGroup.ProgramLaunchID,
			WGWindowStart:   req.WorkGroup.WGWindowStart,
			WGWindowCount:   req.WorkGroup.WGWindowCount,
			WGIDX:           req.WorkGroup.IDX,
			WGIDY:           req.WorkGroup.IDY,
			WGIDZ:           req.WorkGroup.IDZ,
			SubWGFactor:     req.WorkGroup.SubWGFactor,
			SubWGIndex:      req.WorkGroup.SubWGIndex,
			LocalXOffset:    req.WorkGroup.LocalXOffset,
			LocalYOffset:    req.WorkGroup.LocalYOffset,
			SubSizeX:        req.WorkGroup.CurrSizeX,
			SubSizeY:        req.WorkGroup.CurrSizeY,
			SubSizeZ:        req.WorkGroup.CurrSizeZ,
			DispatchTime:    now,
			GPUID:           h.gpuID,
			Chiplet:         h.chipletID,
			SAGlobal:        h.saGlobal,
			SA:              h.saLocal,
			CU:              h.cuID,
			CUName:          h.cuName,
		})
	case sim.HookPosPortMsgSend:
		rsp, ok := ctx.Item.(*protocol.WGCompletionMsg)
		if !ok || rsp == nil {
			if msg, ok := ctx.Item.(sim.Msg); ok {
				rsp, _ = msg.(*protocol.WGCompletionMsg)
			}
		}
		if rsp == nil {
			return
		}
		for _, reqID := range rsp.RspTo {
			h.collector.MarkCompletion(reqID, now)
		}
	}
}

type wgMappingTracer struct {
	collector *wgMappingCollector

	gpuID      int
	chipletID  int
	saGlobalID int
	saLocalID  int
	cuID       int
	cuName     string
}

func newWGMappingTracer(
	collector *wgMappingCollector,
	gpuID, chipletID, saGlobalID, saLocalID, cuID int,
	cuName string,
) *wgMappingTracer {
	return &wgMappingTracer{
		collector:  collector,
		gpuID:      gpuID,
		chipletID:  chipletID,
		saGlobalID: saGlobalID,
		saLocalID:  saLocalID,
		cuID:       cuID,
		cuName:     cuName,
	}
}

func packetAddrFromWG(req *protocol.MapWGReq) uint64 {
	if req == nil || req.WorkGroup == nil {
		return 0
	}
	if req.WorkGroup.PacketAddress != 0 {
		return req.WorkGroup.PacketAddress
	}
	for _, wf := range req.WorkGroup.Wavefronts {
		if wf != nil && wf.PacketAddress != 0 {
			return wf.PacketAddress
		}
	}
	return 0
}

func (t *wgMappingTracer) StartTask(task tracing.Task) {
	switch task.Kind {
	case "req_in":
		req := extractMapWGReq(task.Detail)
		if req == nil || req.WorkGroup == nil {
			return
		}

		kernelName := "unknown"
		if req.WorkGroup.CodeObject != nil && req.WorkGroup.CodeObject.Symbol != nil {
			kernelName = req.WorkGroup.CodeObject.Symbol.Name
		}

		t.collector.AddDispatch(req.ID, wgMappingRecord{
			KernelName:      kernelName,
			WGUID:           req.WorkGroup.UID,
			PacketAddr:      packetAddrFromWG(req),
			LaunchAbsID:     req.WorkGroup.LaunchAbsID,
			LaunchRelID:     req.WorkGroup.LaunchRelID,
			ProgramLaunchID: req.WorkGroup.ProgramLaunchID,
			WGWindowStart:   req.WorkGroup.WGWindowStart,
			WGWindowCount:   req.WorkGroup.WGWindowCount,
			WGIDX:           req.WorkGroup.IDX,
			WGIDY:           req.WorkGroup.IDY,
			WGIDZ:           req.WorkGroup.IDZ,
			SubWGFactor:     req.WorkGroup.SubWGFactor,
			SubWGIndex:      req.WorkGroup.SubWGIndex,
			LocalXOffset:    req.WorkGroup.LocalXOffset,
			LocalYOffset:    req.WorkGroup.LocalYOffset,
			SubSizeX:        req.WorkGroup.CurrSizeX,
			SubSizeY:        req.WorkGroup.CurrSizeY,
			SubSizeZ:        req.WorkGroup.CurrSizeZ,
			DispatchTime:    task.StartTime,
			GPUID:           t.gpuID,
			Chiplet:         t.chipletID,
			SAGlobal:        t.saGlobalID,
			SA:              t.saLocalID,
			CU:              t.cuID,
			CUName:          t.cuName,
		})
	case "req_out":
		msg, ok := task.Detail.(sim.Msg)
		if !ok {
			return
		}
		rsp, ok := msg.(*protocol.WGCompletionMsg)
		if !ok {
			return
		}
		for _, reqID := range rsp.RspTo {
			t.collector.MarkCompletion(reqID, task.StartTime)
		}
	}
}

func (t *wgMappingTracer) StepTask(task tracing.Task) {}

func (t *wgMappingTracer) AddMilestone(m tracing.Milestone) {}

func (t *wgMappingTracer) EndTask(task tracing.Task) {}

func extractMapWGReq(detail interface{}) *protocol.MapWGReq {
	if detail == nil {
		return nil
	}

	// The Akita tracing system usually stores messages as sim.Msg.
	if msg, ok := detail.(sim.Msg); ok {
		if req, ok := msg.(*protocol.MapWGReq); ok {
			return req
		}
		return nil
	}

	// Be defensive for cases where the tracer passes the message directly.
	if req, ok := detail.(*protocol.MapWGReq); ok {
		return req
	}

	return nil
}

func parseIntInBracketedToken(name, token string) (int, bool) {
	idx := strings.Index(name, token)
	if idx < 0 {
		return 0, false
	}
	idx += len(token)
	end := strings.Index(name[idx:], "]")
	if end < 0 {
		return 0, false
	}
	valStr := name[idx : idx+end]
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return 0, false
	}
	return val, true
}

func parseCUName(name string) (gpuID, saGlobalID, cuID int, ok bool) {
	gpuID, ok = parseIntInBracketedToken(name, "GPU[")
	if !ok {
		return 0, 0, 0, false
	}
	saGlobalID, ok = parseIntInBracketedToken(name, "SA[")
	if !ok {
		return 0, 0, 0, false
	}
	cuID, ok = parseIntInBracketedToken(name, "CU[")
	if !ok {
		return 0, 0, 0, false
	}

	return gpuID, saGlobalID, cuID, true
}

func chipletAndLocalSA(saGlobalID int) (chipletID, saLocalID int) {
	// Only treat SA grouping as chiplets in the privatetlb platform.
	if strings.ToLower(strings.TrimSpace(*platformTypeFlag)) != "privatetlb" {
		return 0, saGlobalID
	}

	numSAPerChiplet := *numSAPerChipletFlag
	if numSAPerChiplet <= 0 {
		return 0, saGlobalID
	}

	return saGlobalID / numSAPerChiplet, saGlobalID % numSAPerChiplet
}

func (r *reporter) injectWGMappingTracer(s *simulation.Simulation) {
	if !*wgMappingReportFlag {
		return
	}

	r.wgMappingCollector = newWGMappingCollector()

	for _, comp := range s.Components() {
		cuComp, ok := comp.(*cu.ComputeUnit)
		if !ok || cuComp == nil || cuComp.ToACE == nil {
			continue
		}

		gpuID, saGlobalID, cuID, ok := parseCUName(cuComp.Name())
		if !ok {
			continue
		}

		chipletID, saLocalID := chipletAndLocalSA(saGlobalID)
		cuComp.ToACE.AcceptHook(
			newWGMappingPortHook(
				r.wgMappingCollector,
				gpuID,
				chipletID,
				saGlobalID,
				saLocalID,
				cuID,
				cuComp.Name(),
			),
		)
	}
}

func (r *reporter) dumpWGMappingCSVIfEnabled() {
	if !*wgMappingReportFlag {
		return
	}
	if r.wgMappingCollector == nil {
		return
	}

	baseName := fmt.Sprintf("%s_wgmap", *filenameFlag)
	if err := r.wgMappingCollector.DumpCSV(baseName); err != nil {
		log.Printf("failed to dump wg mapping CSV: %v", err)
	}
}
