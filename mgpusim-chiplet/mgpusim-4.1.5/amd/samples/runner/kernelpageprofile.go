package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"

	"github.com/sarchlab/akita/v4/mem/mem"
	"github.com/sarchlab/akita/v4/mem/vm"
	"github.com/sarchlab/akita/v4/sim"
	"github.com/sarchlab/akita/v4/tracing"
	"github.com/sarchlab/mgpusim/v4/amd/driver"
	"github.com/sarchlab/mgpusim/v4/amd/profiler"
	"github.com/sarchlab/mgpusim/v4/amd/protocol"
)

type bufferRange struct {
	start uint64
	end   uint64
	tag   string
}

type pageRW struct {
	r []uint64
	w []uint64
}

type kernelRuntimeProfileState struct {
	firstDispatchSet bool
	firstDispatch    sim.VTimeInSec
	lastCompletion   sim.VTimeInSec

	firstDispatchByChiplet  []sim.VTimeInSec
	firstDispatchSeen       []bool
	lastCompletionByChiplet []sim.VTimeInSec

	activeWGByCU           map[string]int
	activeCUs              int
	activeCUsByChiplet     []int
	peakActiveCUs          int
	peakActiveCUsByChiplet []int
	usedCUs                map[string]struct{}
	usedCUsByChiplet       []map[string]struct{}
	wgCountByChiplet       []int
}

func newKernelRuntimeProfileState(chiplets int) *kernelRuntimeProfileState {
	if chiplets <= 0 {
		chiplets = 1
	}
	usedByChiplet := make([]map[string]struct{}, chiplets)
	for i := 0; i < chiplets; i++ {
		usedByChiplet[i] = make(map[string]struct{})
	}
	return &kernelRuntimeProfileState{
		firstDispatchByChiplet:  make([]sim.VTimeInSec, chiplets),
		firstDispatchSeen:       make([]bool, chiplets),
		lastCompletionByChiplet: make([]sim.VTimeInSec, chiplets),
		activeWGByCU:            make(map[string]int),
		activeCUsByChiplet:      make([]int, chiplets),
		peakActiveCUsByChiplet:  make([]int, chiplets),
		usedCUs:                 make(map[string]struct{}),
		usedCUsByChiplet:        usedByChiplet,
		wgCountByChiplet:        make([]int, chiplets),
	}
}

func (r *kernelRuntimeProfileState) recordDispatch(
	time sim.VTimeInSec,
	chiplet int,
	cuKey string,
) {
	if r == nil {
		return
	}
	if chiplet < 0 || chiplet >= len(r.activeCUsByChiplet) {
		return
	}

	if !r.firstDispatchSet {
		r.firstDispatchSet = true
		r.firstDispatch = time
	}
	if !r.firstDispatchSeen[chiplet] {
		r.firstDispatchSeen[chiplet] = true
		r.firstDispatchByChiplet[chiplet] = time
	}

	r.wgCountByChiplet[chiplet]++
	r.usedCUs[cuKey] = struct{}{}
	r.usedCUsByChiplet[chiplet][cuKey] = struct{}{}

	if r.activeWGByCU[cuKey] == 0 {
		r.activeCUs++
		r.activeCUsByChiplet[chiplet]++
		if r.activeCUs > r.peakActiveCUs {
			r.peakActiveCUs = r.activeCUs
		}
		if r.activeCUsByChiplet[chiplet] > r.peakActiveCUsByChiplet[chiplet] {
			r.peakActiveCUsByChiplet[chiplet] = r.activeCUsByChiplet[chiplet]
		}
	}
	r.activeWGByCU[cuKey]++
}

func (r *kernelRuntimeProfileState) recordCompletion(
	time sim.VTimeInSec,
	chiplet int,
	cuKey string,
) {
	if r == nil {
		return
	}
	if chiplet < 0 || chiplet >= len(r.activeCUsByChiplet) {
		return
	}

	if !r.firstDispatchSet {
		return
	}

	r.lastCompletion = time
	r.lastCompletionByChiplet[chiplet] = time

	count, ok := r.activeWGByCU[cuKey]
	if !ok || count <= 0 {
		return
	}
	count--
	if count == 0 {
		delete(r.activeWGByCU, cuKey)
		r.activeCUs--
		if r.activeCUs < 0 {
			r.activeCUs = 0
		}
		r.activeCUsByChiplet[chiplet]--
		if r.activeCUsByChiplet[chiplet] < 0 {
			r.activeCUsByChiplet[chiplet] = 0
		}
		return
	}
	r.activeWGByCU[cuKey] = count
}

type KernelRuntimeJSON struct {
	FirstDispatchTimeSec           float64   `json:"first_dispatch_time_sec,omitempty"`
	LastCompletionTimeSec          float64   `json:"last_completion_time_sec,omitempty"`
	ActiveTimeSec                  float64   `json:"active_time_sec,omitempty"`
	PeakActiveCUs                  int       `json:"peak_active_cus,omitempty"`
	PeakActiveCUsByChiplet         []int     `json:"peak_active_cus_by_chiplet,omitempty"`
	UsedCUs                        int       `json:"used_cus,omitempty"`
	UsedCUsByChiplet               []int     `json:"used_cus_by_chiplet,omitempty"`
	WGCountByChiplet               []int     `json:"wg_count_by_chiplet,omitempty"`
	FirstDispatchTimeByChipletSec  []float64 `json:"first_dispatch_time_by_chiplet_sec,omitempty"`
	LastCompletionTimeByChipletSec []float64 `json:"last_completion_time_by_chiplet_sec,omitempty"`
	ActiveTimeByChipletSec         []float64 `json:"active_time_by_chiplet_sec,omitempty"`
}

func (r *kernelRuntimeProfileState) toJSON() *KernelRuntimeJSON {
	if r == nil || !r.firstDispatchSet {
		return nil
	}

	usedByChiplet := make([]int, len(r.usedCUsByChiplet))
	firstByChiplet := make([]float64, len(r.firstDispatchByChiplet))
	lastByChiplet := make([]float64, len(r.lastCompletionByChiplet))
	activeByChiplet := make([]float64, len(r.lastCompletionByChiplet))
	for i := range r.usedCUsByChiplet {
		usedByChiplet[i] = len(r.usedCUsByChiplet[i])
		if r.firstDispatchSeen[i] {
			firstByChiplet[i] = float64(r.firstDispatchByChiplet[i])
			lastByChiplet[i] = float64(r.lastCompletionByChiplet[i])
			if r.lastCompletionByChiplet[i] > r.firstDispatchByChiplet[i] {
				activeByChiplet[i] = float64(r.lastCompletionByChiplet[i] - r.firstDispatchByChiplet[i])
			}
		}
	}

	active := 0.0
	if r.lastCompletion > r.firstDispatch {
		active = float64(r.lastCompletion - r.firstDispatch)
	}

	return &KernelRuntimeJSON{
		FirstDispatchTimeSec:           float64(r.firstDispatch),
		LastCompletionTimeSec:          float64(r.lastCompletion),
		ActiveTimeSec:                  active,
		PeakActiveCUs:                  r.peakActiveCUs,
		PeakActiveCUsByChiplet:         append([]int(nil), r.peakActiveCUsByChiplet...),
		UsedCUs:                        len(r.usedCUs),
		UsedCUsByChiplet:               usedByChiplet,
		WGCountByChiplet:               append([]int(nil), r.wgCountByChiplet...),
		FirstDispatchTimeByChipletSec:  firstByChiplet,
		LastCompletionTimeByChipletSec: lastByChiplet,
		ActiveTimeByChipletSec:         activeByChiplet,
	}
}

type wgProfileState struct {
	WGX int
	WGY int
	WGZ int

	PacketAddr uint64
	Chiplet    int

	DispatchTime   sim.VTimeInSec
	CompletionTime sim.VTimeInSec
	HasDispatch    bool
	HasCompletion  bool

	Tags map[string]map[uint32]*pageRW
}

func wgProfileKey(x, y, z int) string {
	return fmt.Sprintf("%d,%d,%d", x, y, z)
}

type kernelPageProfileRecord struct {
	Name  string `json:"name"`
	CmdID string `json:"cmd_id"`

	// Launch dimensions (optional; best-effort).
	GridSize [3]uint32 `json:"grid_size"`
	WGSize   [3]uint16 `json:"wg_size"`

	// Tag -> page -> per-chiplet bytes.
	Tags map[string]map[uint32]*pageRW `json:"-"`

	// Per-workgroup tag/page profile (best-effort, sparse).
	WGProfiles map[string]*wgProfileState `json:"-"`

	packetAddrs []uint64
	Runtime     *kernelRuntimeProfileState `json:"-"`
}

// ProfileJSON is the exported format consumed by the planner.
// Keep it stable and versioned.
type ProfileJSON struct {
	Version int `json:"version"`

	// Hardware / addressing.
	Chiplets     int    `json:"chiplets"`
	SAPerChiplet int    `json:"sa_per_chiplet"`
	PageBytes    uint64 `json:"page_bytes"`

	// Byte model for each recorded access.
	// We keep it consistent with the existing L2 estimator used by the ILP
	// objective.
	ByteModel string `json:"byte_model"`

	// Buffer table to help external debugging.
	Buffers []driver.BufferInfo `json:"buffers"`

	// Per-kernel (per command) profiles.
	Kernels []KernelProfileJSON `json:"kernels"`
}

type KernelProfileJSON struct {
	Name  string `json:"name"`
	CmdID string `json:"cmd_id"`

	GridSize    [3]uint32 `json:"grid_size"`
	WGSize      [3]uint16 `json:"wg_size"`
	PacketAddrs []uint64  `json:"packet_addrs,omitempty"`

	Runtime    *KernelRuntimeJSON        `json:"runtime,omitempty"`
	Tags       map[string]TagProfileJSON `json:"tags"`
	Workgroups []WGProfileJSON           `json:"workgroups,omitempty"`
}

type TagProfileJSON struct {
	// Sparse list (only pages with non-zero bytes).
	Pages []PageProfileJSON `json:"pages"`
}

type PageProfileJSON struct {
	Page uint32   `json:"page"`
	R    []uint64 `json:"r"`
	W    []uint64 `json:"w"`
}

type WGProfileJSON struct {
	WG             [3]int                    `json:"wg"`
	PacketAddr     uint64                    `json:"packet_addr,omitempty"`
	Chiplet        int                       `json:"chiplet"`
	DispatchTime   float64                   `json:"dispatch_time_sec,omitempty"`
	CompletionTime float64                   `json:"completion_time_sec,omitempty"`
	ActiveTime     float64                   `json:"active_time_sec,omitempty"`
	Tags           map[string]TagProfileJSON `json:"tags,omitempty"`
}

type wgInflightInfo struct {
	rec     *kernelPageProfileRecord
	chiplet int
	cuKey   string
	wgKey   string
}

type currentTimeProvider interface {
	CurrentTime() sim.VTimeInSec
}

func currentTimeFromPortHook(ctx sim.HookCtx) (sim.VTimeInSec, bool) {
	p, ok := ctx.Domain.(sim.Port)
	if !ok || p == nil {
		return 0, false
	}
	comp := p.Component()
	if comp == nil {
		return 0, false
	}
	tp, ok := comp.(currentTimeProvider)
	if !ok {
		return 0, false
	}
	return tp.CurrentTime(), true
}

type kernelWGTimingPortHook struct {
	tracer  *kernelPageProfileTracer
	chiplet int
	cuKey   string
}

func newKernelWGTimingPortHook(tracer *kernelPageProfileTracer, chiplet int, cuKey string) *kernelWGTimingPortHook {
	return &kernelWGTimingPortHook{tracer: tracer, chiplet: chiplet, cuKey: cuKey}
}

func (h *kernelWGTimingPortHook) Func(ctx sim.HookCtx) {
	if h == nil || h.tracer == nil {
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
		if req != nil {
			h.tracer.onWGDispatchAtTime(now, req, h.chiplet, h.cuKey)
		}
	case sim.HookPosPortMsgSend:
		rsp, ok := ctx.Item.(*protocol.WGCompletionMsg)
		if !ok || rsp == nil {
			if msg, ok := ctx.Item.(sim.Msg); ok {
				rsp, _ = msg.(*protocol.WGCompletionMsg)
			}
		}
		if rsp != nil {
			h.tracer.onWGCompletionAtTime(now, rsp, h.chiplet, h.cuKey)
		}
	}
}

// kernelPageProfileTracer records per-kernel, per-tag, per-page (group)
// read/write bytes issued by each chiplet.
//
// It observes L2 cache requests (req_in) and attributes them using
// profiler.MemReqInfo carried in req.Info.
//
// It also records exact per-launch runtime resource usage from CU-side
// work-group dispatch/completion events, so the planner can use profile-derived
// CU demand instead of static estimates.
type kernelPageProfileTracer struct {
	driver *driver.Driver

	chiplets     int
	saPerChiplet int
	groupPages   int
	pageBytes    uint64
	log2PageSize uint64

	mu sync.Mutex

	bufferRanges map[vm.PID][]bufferRange
	buffersSnap  []driver.BufferInfo
	// needRefreshBuffers indicates that the buffer table snapshot must be
	// refreshed before attributing L2 requests to buffer tags.
	//
	// IMPORTANT: We must not call driver.ListBuffers() inside the Driver's
	// HookPosTaskStart callback for LaunchKernelCommand, because the Driver
	// emits the task while holding d.contextMutex. ListBuffers() also locks
	// d.contextMutex and would deadlock.
	//
	// Therefore we refresh lazily on the first observed L2 request of the
	// kernel, when the Driver is no longer holding its internal locks.
	needRefreshBuffers bool

	activeKernel   *kernelPageProfileRecord
	kernels        []*kernelPageProfileRecord
	packetToKernel map[uint64]*kernelPageProfileRecord
	mapReqToWG     map[string]wgInflightInfo
}

func newKernelPageProfileTracer(d *driver.Driver) *kernelPageProfileTracer {
	pageBytes := uint64(1) << d.Log2PageSize
	return &kernelPageProfileTracer{
		driver:         d,
		chiplets:       *numChipletsPerGPUFlag,
		saPerChiplet:   *numSAPerChipletFlag,
		groupPages:     maxInt(1, *profileGroupPagesFlag),
		pageBytes:      pageBytes,
		log2PageSize:   d.Log2PageSize,
		bufferRanges:   make(map[vm.PID][]bufferRange),
		packetToKernel: make(map[uint64]*kernelPageProfileRecord),
		mapReqToWG:     make(map[string]wgInflightInfo),
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (t *kernelPageProfileTracer) StartTask(task tracing.Task) {
	if isKernelCommandTask(task) {
		t.onKernelStart(task)
		return
	}

	if task.Kind != "req_in" {
		return
	}

	msg, ok := task.Detail.(sim.Msg)
	if !ok {
		return
	}

	t.onL2Request(task, msg)
}

func (t *kernelPageProfileTracer) onL2Request(task tracing.Task, msg sim.Msg) {
	var (
		infoAny interface{}
		isRead  bool
	)
	switch m := msg.(type) {
	case *mem.ReadReq:
		infoAny = m.Info
		isRead = true
	case *mem.WriteReq:
		infoAny = m.Info
		isRead = false
	default:
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Refresh buffer snapshot lazily at the first L2 request after a kernel has
	// started. See the comment on needRefreshBuffers for the deadlock rationale.
	if t.needRefreshBuffers {
		t.buffersSnap = t.driver.ListBuffers()
		t.rebuildBufferRangesLocked()
		t.needRefreshBuffers = false
	}

	info, ok := infoAny.(*profiler.MemReqInfo)
	if !ok || info == nil {
		return
	}

	rec := t.activeKernel
	if info.PacketAddr != 0 {
		rec = t.packetToKernel[info.PacketAddr]
	}
	if rec == nil {
		return
	}

	wgKey := wgProfileKey(info.WGX, info.WGY, info.WGZ)
	wgRec := rec.WGProfiles[wgKey]
	if wgRec == nil {
		if rec.WGProfiles == nil {
			rec.WGProfiles = make(map[string]*wgProfileState)
		}
		wgRec = &wgProfileState{WGX: info.WGX, WGY: info.WGY, WGZ: info.WGZ, PacketAddr: info.PacketAddr, Tags: make(map[string]map[uint32]*pageRW)}
		rec.WGProfiles[wgKey] = wgRec
	}

	chiplet := 0
	if t.saPerChiplet > 0 {
		chiplet = info.RequesterSA / t.saPerChiplet
	}
	if chiplet < 0 || chiplet >= t.chiplets {
		return
	}
	if wgRec != nil {
		wgRec.Chiplet = chiplet
	}

	pid := info.PID
	vaddr := info.VAddr
	if vaddr == 0 {
		return
	}

	tag, base, ok := t.lookupTag(pid, vaddr)
	if !ok {
		return
	}

	page := uint32((vaddr - base) >> t.log2PageSize)
	page = page / uint32(t.groupPages)

	bytes := estWriteTotalBytes
	if isRead {
		bytes = estReadTotalBytes
	}

	if rec.Tags == nil {
		rec.Tags = make(map[string]map[uint32]*pageRW)
	}
	pages, ok := rec.Tags[tag]
	if !ok {
		pages = make(map[uint32]*pageRW)
		rec.Tags[tag] = pages
	}
	rw, ok := pages[page]
	if !ok {
		rw = &pageRW{
			r: make([]uint64, t.chiplets),
			w: make([]uint64, t.chiplets),
		}
		pages[page] = rw
	}
	if isRead {
		rw.r[chiplet] += bytes
	} else {
		rw.w[chiplet] += bytes
	}

	if wgRec != nil {
		if wgRec.Tags == nil {
			wgRec.Tags = make(map[string]map[uint32]*pageRW)
		}
		wgPages, ok := wgRec.Tags[tag]
		if !ok {
			wgPages = make(map[uint32]*pageRW)
			wgRec.Tags[tag] = wgPages
		}
		wgRW, ok := wgPages[page]
		if !ok {
			wgRW = &pageRW{r: make([]uint64, t.chiplets), w: make([]uint64, t.chiplets)}
			wgPages[page] = wgRW
		}
		if isRead {
			wgRW.r[chiplet] += bytes
		} else {
			wgRW.w[chiplet] += bytes
		}
	}
}

func packetAddrsFromTask(task tracing.Task) []uint64 {
	if task.Detail == nil {
		return nil
	}
	out := make([]uint64, 0, 4)
	switch cmd := task.Detail.(type) {
	case *driver.LaunchKernelCommand:
		if cmd.DPacket != 0 {
			out = append(out, uint64(cmd.DPacket))
		}
	case *driver.LaunchUnifiedMultiGPUKernelCommand:
		for _, p := range cmd.DPacketArray {
			if p != 0 {
				out = append(out, uint64(p))
			}
		}
	}
	return out
}

func packetAddrFromMapWGReq(req *protocol.MapWGReq) uint64 {
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

func cuIdentityFromTask(task tracing.Task) (chiplet int, cuKey string, ok bool) {
	gpuID, saGlobalID, cuID, ok := parseCUName(task.Location)
	if !ok {
		return 0, "", false
	}
	chipletID, _ := chipletAndLocalSA(saGlobalID)
	cuKey = fmt.Sprintf("gpu%d.chiplet%d.cu%d", gpuID, chipletID, cuID)
	return chipletID, cuKey, true
}

func (t *kernelPageProfileTracer) onWGDispatchAtTime(now sim.VTimeInSec, req *protocol.MapWGReq, chiplet int, cuKey string) {
	if req == nil || req.WorkGroup == nil {
		return
	}
	packetAddr := packetAddrFromMapWGReq(req)
	if packetAddr == 0 {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	rec := t.packetToKernel[packetAddr]
	if rec == nil {
		return
	}
	if rec.Runtime == nil {
		rec.Runtime = newKernelRuntimeProfileState(t.chiplets)
	}
	rec.Runtime.recordDispatch(now, chiplet, cuKey)
	wgKey := wgProfileKey(req.WorkGroup.IDX, req.WorkGroup.IDY, req.WorkGroup.IDZ)
	wgRec := rec.WGProfiles[wgKey]
	if wgRec == nil {
		if rec.WGProfiles == nil {
			rec.WGProfiles = make(map[string]*wgProfileState)
		}
		wgRec = &wgProfileState{WGX: req.WorkGroup.IDX, WGY: req.WorkGroup.IDY, WGZ: req.WorkGroup.IDZ, PacketAddr: packetAddr, Tags: make(map[string]map[uint32]*pageRW)}
		rec.WGProfiles[wgKey] = wgRec
	}
	wgRec.PacketAddr = packetAddr
	wgRec.Chiplet = chiplet
	wgRec.DispatchTime = now
	wgRec.HasDispatch = true
	t.mapReqToWG[req.ID] = wgInflightInfo{
		rec:     rec,
		chiplet: chiplet,
		cuKey:   cuKey,
		wgKey:   wgKey,
	}
}

func (t *kernelPageProfileTracer) onWGCompletionAtTime(now sim.VTimeInSec, rsp *protocol.WGCompletionMsg, chiplet int, cuKey string) {
	if rsp == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	for _, rspTo := range rsp.RspTo {
		info, ok := t.mapReqToWG[rspTo]
		if !ok {
			continue
		}
		if info.rec != nil && info.rec.Runtime != nil {
			info.rec.Runtime.recordCompletion(now, info.chiplet, info.cuKey)
		}
		if info.rec != nil && info.wgKey != "" {
			if wgRec := info.rec.WGProfiles[info.wgKey]; wgRec != nil {
				wgRec.CompletionTime = now
				wgRec.HasCompletion = true
				if chiplet >= 0 {
					wgRec.Chiplet = info.chiplet
				}
			}
		}
		delete(t.mapReqToWG, rspTo)
	}
}

func (t *kernelPageProfileTracer) onKernelStart(task tracing.Task) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// IMPORTANT: Do not call driver.ListBuffers() here.
	// The Driver emits the LaunchKernelCommand task while holding its
	// contextMutex (see Driver.processNewCommand). Calling ListBuffers() here
	// would attempt to lock the same mutex again and deadlock.
	//
	// We refresh the buffer table lazily when we observe the first L2 request
	// of this kernel (StartTask kind=req_in).
	t.needRefreshBuffers = true

	name := kernelNameFromTask(task)
	grid, wg := kernelLaunchDimsFromTask(task)
	packetAddrs := packetAddrsFromTask(task)

	rec := &kernelPageProfileRecord{
		Name:        name,
		CmdID:       task.ID,
		GridSize:    [3]uint32{grid[0], grid[1], grid[2]},
		WGSize:      [3]uint16{wg[0], wg[1], wg[2]},
		Tags:        make(map[string]map[uint32]*pageRW),
		WGProfiles:  make(map[string]*wgProfileState),
		packetAddrs: append([]uint64(nil), packetAddrs...),
		Runtime:     newKernelRuntimeProfileState(t.chiplets),
	}
	t.kernels = append(t.kernels, rec)
	t.activeKernel = rec
	for _, pa := range packetAddrs {
		if pa != 0 {
			t.packetToKernel[pa] = rec
		}
	}
}

func (t *kernelPageProfileTracer) EndTask(task tracing.Task) {
	if !isKernelCommandTask(task) {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.activeKernel != nil && t.activeKernel.CmdID == task.ID {
		t.activeKernel = nil
	}
}

func (t *kernelPageProfileTracer) StepTask(task tracing.Task) {}

func (t *kernelPageProfileTracer) AddMilestone(m tracing.Milestone) {}

func (t *kernelPageProfileTracer) rebuildBufferRangesLocked() {
	m := make(map[vm.PID][]bufferRange)
	for _, b := range t.buffersSnap {
		if b.Tag == "" {
			continue
		}
		m[b.PID] = append(m[b.PID], bufferRange{
			start: b.VAddr,
			end:   b.VAddr + b.Size,
			tag:   b.Tag,
		})
	}
	for pid := range m {
		ranges := m[pid]
		sort.Slice(ranges, func(i, j int) bool { return ranges[i].start < ranges[j].start })
		m[pid] = ranges
	}
	t.bufferRanges = m
}

func (t *kernelPageProfileTracer) lookupTag(pid vm.PID, vaddr uint64) (tag string, base uint64, ok bool) {
	ranges := t.bufferRanges[pid]
	if len(ranges) == 0 {
		return "", 0, false
	}
	// Find the last range with start <= vaddr.
	i := sort.Search(len(ranges), func(i int) bool { return ranges[i].start > vaddr }) - 1
	if i >= 0 && vaddr < ranges[i].end {
		return ranges[i].tag, ranges[i].start, true
	}
	return "", 0, false
}

func (t *kernelPageProfileTracer) DumpJSON(filename string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if filename == "" {
		return nil
	}

	out := ProfileJSON{
		Version:      3,
		Chiplets:     t.chiplets,
		SAPerChiplet: t.saPerChiplet,
		PageBytes:    t.pageBytes * uint64(t.groupPages),
		ByteModel:    "l2_req_est_80_32",
		Buffers:      t.buffersSnap,
		Kernels:      make([]KernelProfileJSON, 0, len(t.kernels)),
	}

	for _, k := range t.kernels {
		kj := KernelProfileJSON{
			Name:     k.Name,
			CmdID:    k.CmdID,
			GridSize: k.GridSize,
			WGSize:   k.WGSize,
			Runtime:  k.Runtime.toJSON(),
			Tags:     make(map[string]TagProfileJSON),
		}
		if *profilePacketAddrsFlag {
			kj.PacketAddrs = append([]uint64(nil), k.packetAddrs...)
		}

		// Stable order for tags.
		tagNames := make([]string, 0, len(k.Tags))
		for tag := range k.Tags {
			tagNames = append(tagNames, tag)
		}
		sort.Strings(tagNames)

		for _, tag := range tagNames {
			pages := k.Tags[tag]
			pageIDs := make([]int, 0, len(pages))
			for p := range pages {
				pageIDs = append(pageIDs, int(p))
			}
			sort.Ints(pageIDs)

			pj := TagProfileJSON{Pages: make([]PageProfileJSON, 0, len(pageIDs))}
			for _, p := range pageIDs {
				rw := pages[uint32(p)]
				empty := true
				for i := 0; i < t.chiplets; i++ {
					if rw.r[i] != 0 || rw.w[i] != 0 {
						empty = false
						break
					}
				}
				if empty {
					continue
				}
				pj.Pages = append(pj.Pages, PageProfileJSON{
					Page: uint32(p),
					R:    append([]uint64(nil), rw.r...),
					W:    append([]uint64(nil), rw.w...),
				})
			}
			kj.Tags[tag] = pj
		}

		wgKeys := make([]string, 0, len(k.WGProfiles))
		for wk := range k.WGProfiles {
			wgKeys = append(wgKeys, wk)
		}
		sort.SliceStable(wgKeys, func(i, j int) bool {
			a := k.WGProfiles[wgKeys[i]]
			b := k.WGProfiles[wgKeys[j]]
			if a == nil || b == nil {
				return wgKeys[i] < wgKeys[j]
			}
			if a.WGZ != b.WGZ {
				return a.WGZ < b.WGZ
			}
			if a.WGY != b.WGY {
				return a.WGY < b.WGY
			}
			return a.WGX < b.WGX
		})
		if len(wgKeys) > 0 {
			kj.Workgroups = make([]WGProfileJSON, 0, len(wgKeys))
		}
		for _, wk := range wgKeys {
			wgRec := k.WGProfiles[wk]
			if wgRec == nil {
				continue
			}
			wgj := WGProfileJSON{
				WG:      [3]int{wgRec.WGX, wgRec.WGY, wgRec.WGZ},
				Chiplet: wgRec.Chiplet,
			}
			if *profilePacketAddrsFlag {
				wgj.PacketAddr = wgRec.PacketAddr
			}
			if wgRec.HasDispatch {
				wgj.DispatchTime = float64(wgRec.DispatchTime)
			}
			if wgRec.HasCompletion {
				wgj.CompletionTime = float64(wgRec.CompletionTime)
			}
			if wgRec.HasDispatch && wgRec.HasCompletion && wgRec.CompletionTime > wgRec.DispatchTime {
				wgj.ActiveTime = float64(wgRec.CompletionTime - wgRec.DispatchTime)
			}
			if *profileWGTagPagesFlag {
				wgj.Tags = make(map[string]TagProfileJSON)
				tgNames := make([]string, 0, len(wgRec.Tags))
				for tag := range wgRec.Tags {
					tgNames = append(tgNames, tag)
				}
				sort.Strings(tgNames)
				for _, tag := range tgNames {
					pages := wgRec.Tags[tag]
					pageIDs := make([]int, 0, len(pages))
					for p := range pages {
						pageIDs = append(pageIDs, int(p))
					}
					sort.Ints(pageIDs)
					pj := TagProfileJSON{Pages: make([]PageProfileJSON, 0, len(pageIDs))}
					for _, p := range pageIDs {
						rw := pages[uint32(p)]
						empty := true
						for i := 0; i < t.chiplets; i++ {
							if rw.r[i] != 0 || rw.w[i] != 0 {
								empty = false
								break
							}
						}
						if empty {
							continue
						}
						pj.Pages = append(pj.Pages, PageProfileJSON{Page: uint32(p), R: append([]uint64(nil), rw.r...), W: append([]uint64(nil), rw.w...)})
					}
					if len(pj.Pages) > 0 {
						wgj.Tags[tag] = pj
					}
				}
				if len(wgj.Tags) == 0 {
					wgj.Tags = nil
				}
			}
			kj.Workgroups = append(kj.Workgroups, wgj)
		}

		out.Kernels = append(out.Kernels, kj)
	}

	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return err
	}
	return nil
}

func (t *kernelPageProfileTracer) SummaryString() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	kernels := len(t.kernels)
	tags := 0
	pages := 0
	runtime := 0
	wgs := 0
	for _, k := range t.kernels {
		tags += len(k.Tags)
		wgs += len(k.WGProfiles)
		if k.Runtime != nil && k.Runtime.firstDispatchSet {
			runtime++
		}
		for _, pm := range k.Tags {
			pages += len(pm)
		}
	}

	return fmt.Sprintf("profile(kernels=%d tags=%d pages=%d runtime=%d wgs=%d group_pages=%d)",
		kernels, tags, pages, runtime, wgs, t.groupPages)
}
