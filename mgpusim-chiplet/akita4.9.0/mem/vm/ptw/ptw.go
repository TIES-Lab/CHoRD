package ptw

import (
	"container/list"
	"encoding/binary"
	"fmt"
	"sync/atomic"

	"github.com/sarchlab/akita/v4/mem/mem"
	"github.com/sarchlab/akita/v4/mem/vm"
	"github.com/sarchlab/akita/v4/sim"
)

// PageTable defines the extra methods required by the page table walker.
// It is intended to be implemented by vm.RadixPageTable.
type PageTable interface {
	vm.PageTable

	GetLog2PageSize() uint64

	// Rearrange rewrites the VPN bits so the walker can consume indices
	// from low bits by shifting.
	Rearrange(vAddr uint64) uint64
	GetRoot(pid vm.PID) uint64
	AddOffset(ppn, rearrangedVAddr uint64) uint64
	NextLevel(rearrangedVAddr uint64) uint64
	MoveToLevel(rearrangedVAddr uint64, level int) uint64
	FindPTEValue(pid vm.PID, vAddr uint64, level int) (uint64, bool)
}

// Comp is a page-table walker with an in-PTW Page Walk Cache (PWC).
//
// It accepts vm.TranslationReq from the Top port and issues mem.ReadReq
// for page table entries via the Mem port. The returned data is not used
// for correctness in this checkpoint; the access traffic and dependencies
// are modeled by waiting for responses.
type Comp struct {
	*sim.TickingComponent

	topPort sim.Port
	memPort sim.Port

	// Optional chiplet layout information used for attributing PTE reads as
	// local vs remote. These values are set by the platform builder.
	chipletID            int
	memAddrOffset        uint64
	numChiplets          int
	numBanksPerChiplet   int
	bankInterleavingSize uint64

	// Statistics counters (monotonic).
	translationReqCount uint64
	walkCompletedCount  uint64
	walkFailedCount     uint64
	pwcLookupCount      uint64
	pwcHitCount         uint64
	pwcMissCount        uint64
	pwcHitLvl0          uint64
	pwcHitLvl1          uint64
	pwcHitLvl2          uint64
	pteReadCount         uint64
	pteReadLocalCount    uint64
	pteReadRemoteCount   uint64
	walkLatencySumPS     uint64
	walkLatencyCount     uint64

	pageTable  PageTable
	addrMapper mem.AddressToPortMapper

	numWalkers       int
	pwcLookupLatency int

	// Page table geometry (align with MGVM privateTLB baseline)
	numLevels    int
	bitsPerLevel uint64

	pwc *pwc

	transactions  []*transaction
	memReqToTrans map[string]*transaction
}

// NewComp creates a PTW component with default geometry.
func NewComp(name string, engine sim.Engine, freq sim.Freq) *Comp {
	c := &Comp{}

	c.numLevels = 4
	c.bitsPerLevel = 9
	c.numWalkers = 16
	c.pwcLookupLatency = 10

	c.memReqToTrans = make(map[string]*transaction)

	c.TickingComponent = sim.NewTickingComponent(name, engine, freq, c)
	c.topPort = sim.NewPort(c, 64, 64, name+".TopPort")
	c.memPort = sim.NewPort(c, 64, 64, name+".MemPort")
	c.AddPort("Top", c.topPort)
	c.AddPort("Mem", c.memPort)
	return c
}

const psPerSecond = 1e12

// Stats is a snapshot of PTW/PWC statistics.
//
// All fields are monotonic counters. Latency is accumulated in picoseconds.
type Stats struct {
	TranslationReqCount uint64
	WalkCompletedCount  uint64
	WalkFailedCount     uint64
	PWCLookupCount      uint64
	PWCHitCount         uint64
	PWCMissCount        uint64
	PWCHitLevel0        uint64
	PWCHitLevel1        uint64
	PWCHitLevel2        uint64
	PTEReadCount        uint64
	PTEReadLocalCount   uint64
	PTEReadRemoteCount  uint64
	WalkLatencySumPS    uint64
	WalkLatencyCount    uint64
}

// Stats returns a consistent snapshot of the PTW statistics.
func (c *Comp) Stats() Stats {
	return Stats{
		TranslationReqCount: atomic.LoadUint64(&c.translationReqCount),
		WalkCompletedCount:  atomic.LoadUint64(&c.walkCompletedCount),
		WalkFailedCount:     atomic.LoadUint64(&c.walkFailedCount),
		PWCLookupCount:      atomic.LoadUint64(&c.pwcLookupCount),
		PWCHitCount:         atomic.LoadUint64(&c.pwcHitCount),
		PWCMissCount:        atomic.LoadUint64(&c.pwcMissCount),
		PWCHitLevel0:        atomic.LoadUint64(&c.pwcHitLvl0),
		PWCHitLevel1:        atomic.LoadUint64(&c.pwcHitLvl1),
		PWCHitLevel2:        atomic.LoadUint64(&c.pwcHitLvl2),
		PTEReadCount:        atomic.LoadUint64(&c.pteReadCount),
		PTEReadLocalCount:   atomic.LoadUint64(&c.pteReadLocalCount),
		PTEReadRemoteCount:  atomic.LoadUint64(&c.pteReadRemoteCount),
		WalkLatencySumPS:    atomic.LoadUint64(&c.walkLatencySumPS),
		WalkLatencyCount:    atomic.LoadUint64(&c.walkLatencyCount),
	}
}

type transactionState int

const (
	transStatePWCWait transactionState = iota
	transStateReadyToSendMem
	transStateWaitingMem
	transStateReadyToSendRsp
	transStateDone
)

type transaction struct {
	req *vm.TranslationReq
	startTime sim.VTimeInSec

	state transactionState

	startRearrangedVAddr uint64
	curRearrangedVAddr   uint64
	level                int
	ppn                  uint64

	pwcLeft int

	memReqID string
	memReqAddr uint64
	rsp      *vm.TranslationRsp
}

func (c *Comp) Tick() bool {
	madeProgress := false

	// 1) Handle memory responses first to unblock walks.
	if c.handleMemResponses() {
		madeProgress = true
	}

	// 2) Try to accept new translation requests.
	if c.acceptNewTranslationReq() {
		madeProgress = true
	}

	// 3) Advance outstanding transactions.
	if c.advanceTransactions() {
		madeProgress = true
	}

	c.cleanupDoneTransactions()
	return madeProgress
}

func (c *Comp) acceptNewTranslationReq() bool {
	msg := c.topPort.PeekIncoming()
	if msg == nil {
		return false
	}

	if len(c.transactions) >= c.numWalkers {
		return false
	}

	req, ok := msg.(*vm.TranslationReq)
	if !ok {
		panic("PTW can only handle *vm.TranslationReq")
	}

	c.topPort.RetrieveIncoming()
	atomic.AddUint64(&c.translationReqCount, 1)

	t := &transaction{
		req: req,
		startTime: c.CurrentTime(),

		state: transStatePWCWait,

		startRearrangedVAddr: c.pageTable.Rearrange(req.VAddr),
		pwcLeft:              c.pwcLookupLatency,
	}
	c.transactions = append(c.transactions, t)
	return true
}

func (c *Comp) handleMemResponses() bool {
	madeProgress := false

	for {
		msg := c.memPort.PeekIncoming()
		if msg == nil {
			break
		}

		rsp, ok := msg.(*mem.DataReadyRsp)
		if !ok {
			panic(fmt.Sprintf("PTW received unsupported message %T on Mem", msg))
		}

		t, ok := c.memReqToTrans[rsp.RespondTo]
		if !ok {
			// The transaction might have been aborted; just consume the msg.
			c.memPort.RetrieveIncoming()
			madeProgress = true
			continue
		}

		delete(c.memReqToTrans, rsp.RespondTo)
		t.memReqID = ""

		c.memPort.RetrieveIncoming()
		c.handleMemResponseForTransaction(t, rsp)
		madeProgress = true
	}

	return madeProgress
}


func (c *Comp) handleMemResponseForTransaction(t *transaction, rsp *mem.DataReadyRsp) {
	// Parse the PTE value from the memory response. We expect the page table to
	// have materialized PTE bytes into the backing storage.
	var nextPPN uint64
	var ok bool
	if len(rsp.Data) >= 8 {
		nextPPN = binary.LittleEndian.Uint64(rsp.Data[:8])
		ok = nextPPN != 0
	}
	if !ok {
		// Fallback: obtain the next pointer/value from the page table structure.
		nextPPN, ok = c.pageTable.FindPTEValue(t.req.PID, t.req.VAddr, t.level)
	}
	if !ok {
		// Missing mapping.
		t.rsp = vm.TranslationRspBuilder{}.
			WithSrc(c.topPort.AsRemote()).
			WithDst(t.req.Src).
			WithRspTo(t.req.ID).
			WithPage(vm.Page{}).
			Build()
		t.state = transStateReadyToSendRsp
		return
	}

	if t.level < c.numLevels-1 {
		// Cache the intermediate level in PWC (levels 0..2).
		c.pwc.insert(t.req.PID, t.req.VAddr, t.level, nextPPN)
		t.ppn = nextPPN
		t.level++
		t.state = transStateReadyToSendMem
		return
	}

	// Leaf level complete. Return the page mapping.
	page, found := c.pageTable.Find(t.req.PID, t.req.VAddr)
	if !found {
		page = vm.Page{}
	}

	t.rsp = vm.TranslationRspBuilder{}.
		WithSrc(c.topPort.AsRemote()).
		WithDst(t.req.Src).
		WithRspTo(t.req.ID).
		WithPage(page).
		Build()
	t.state = transStateReadyToSendRsp
}

func (c *Comp) advanceTransactions() bool {
	madeProgress := false

	for _, t := range c.transactions {
		switch t.state {
		case transStatePWCWait:
			if c.advancePWCLookup(t) {
				madeProgress = true
			}
		case transStateReadyToSendMem:
			if c.sendMemReq(t) {
				madeProgress = true
			}
		case transStateWaitingMem:
			// wait
		case transStateReadyToSendRsp:
			if c.sendRsp(t) {
				madeProgress = true
			}
		case transStateDone:
			// will be removed
		default:
			panic("PTW transaction in unknown state")
		}
	}

	return madeProgress
}

func (c *Comp) advancePWCLookup(t *transaction) bool {
	if t.pwcLeft > 0 {
		t.pwcLeft--
		return true
	}

	// Perform lookup.
	atomic.AddUint64(&c.pwcLookupCount, 1)
	if ppn, level, ok := c.pwc.lookup(t.req.PID, t.req.VAddr); ok {
		atomic.AddUint64(&c.pwcHitCount, 1)
		switch level {
		case 0:
			atomic.AddUint64(&c.pwcHitLvl0, 1)
		case 1:
			atomic.AddUint64(&c.pwcHitLvl1, 1)
		case 2:
			atomic.AddUint64(&c.pwcHitLvl2, 1)
		}
		t.level = level + 1
		t.ppn = ppn
		t.curRearrangedVAddr = c.pageTable.MoveToLevel(t.startRearrangedVAddr, t.level)
		t.state = transStateReadyToSendMem
		return true
	}
	atomic.AddUint64(&c.pwcMissCount, 1)

	// Miss: start from root.
	root := c.pageTable.GetRoot(t.req.PID)
	if root == 0 {
		// No page table yet for this PID.
		t.rsp = vm.TranslationRspBuilder{}.
			WithSrc(c.topPort.AsRemote()).
			WithDst(t.req.Src).
			WithRspTo(t.req.ID).
			WithPage(vm.Page{}).
			Build()
		t.state = transStateReadyToSendRsp
		return true
	}

	t.level = 0
	t.ppn = root
	t.curRearrangedVAddr = t.startRearrangedVAddr
	t.state = transStateReadyToSendMem
	return true
}

func (c *Comp) sendMemReq(t *transaction) bool {
	if t.level >= c.numLevels {
		// Should not happen. Fail safely.
		t.rsp = vm.TranslationRspBuilder{}.
			WithSrc(c.topPort.AsRemote()).
			WithDst(t.req.Src).
			WithRspTo(t.req.ID).
			WithPage(vm.Page{}).
			Build()
		t.state = transStateReadyToSendRsp
		return true
	}

	pteAddr := c.pageTable.AddOffset(t.ppn, t.curRearrangedVAddr)
	dst := c.addrMapper.Find(pteAddr)
	if dst == "" {
		panic(fmt.Sprintf("PTW cannot find destination for address 0x%x", pteAddr))
	}

	read := mem.ReadReqBuilder{}.
		WithSrc(c.memPort.AsRemote()).
		WithDst(dst).
		WithPID(t.req.PID).
		WithAddress(pteAddr).
		WithByteSize(8).
		Build()

	if err := c.memPort.Send(read); err != nil {
		return false
	}
	atomic.AddUint64(&c.pteReadCount, 1)
	if c.numChiplets > 0 && c.numBanksPerChiplet > 0 && c.bankInterleavingSize > 0 {
		if dstChiplet, ok := c.chipletOfPAddr(pteAddr); ok {
			if dstChiplet == c.chipletID {
				atomic.AddUint64(&c.pteReadLocalCount, 1)
			} else {
				atomic.AddUint64(&c.pteReadRemoteCount, 1)
			}
		}
	}

	t.memReqID = read.ID
	t.memReqAddr = pteAddr
	c.memReqToTrans[read.ID] = t
	t.state = transStateWaitingMem

	// Prepare for next level.
	t.curRearrangedVAddr = c.pageTable.NextLevel(t.curRearrangedVAddr)
	return true
}

func (c *Comp) sendRsp(t *transaction) bool {
	if t.rsp == nil {
		panic("PTW transaction ready to send response but rsp is nil")
	}

	if err := c.topPort.Send(t.rsp); err != nil {
		return false
	}

	atomic.AddUint64(&c.walkCompletedCount, 1)
	if !t.rsp.Page.Valid {
		atomic.AddUint64(&c.walkFailedCount, 1)
	}
	// Include all queuing/serialization delays until the response is actually sent.
	latency := c.CurrentTime() - t.startTime
	if latency < 0 {
		latency = 0
	}
	latPS := uint64(float64(latency)*psPerSecond + 0.5)
	atomic.AddUint64(&c.walkLatencySumPS, latPS)
	atomic.AddUint64(&c.walkLatencyCount, 1)

	t.state = transStateDone
	return true
}

func (c *Comp) cleanupDoneTransactions() {
	if len(c.transactions) == 0 {
		return
	}

	// Compact in-place.
	kept := c.transactions[:0]
	for _, t := range c.transactions {
		if t.state != transStateDone {
			kept = append(kept, t)
		}
	}
	c.transactions = kept
}

func (c *Comp) chipletOfPAddr(addr uint64) (int, bool) {
	if c.numChiplets <= 0 || c.numBanksPerChiplet <= 0 || c.bankInterleavingSize == 0 {
		return 0, false
	}
	if addr < c.memAddrOffset {
		return 0, false
	}
	rel := addr - c.memAddrOffset
	numBanks := uint64(c.numChiplets * c.numBanksPerChiplet)
	if numBanks == 0 {
		return 0, false
	}
	bank := (rel / c.bankInterleavingSize) % numBanks
	chiplet := int(bank / uint64(c.numBanksPerChiplet))
	return chiplet, true
}

// --------------------
// PWC (fully-associative LRU)
// --------------------

type pwcKey struct {
	pid   vm.PID
	level int
	tag   uint64
}

type pwcEntry struct {
	key pwcKey
	ppn uint64
}

type pwc struct {
	capacity int

	pageSizeMask uint64
	masks        [3]uint64

	lru   *list.List
	table map[pwcKey]*list.Element
}

func newPWC(capacity int, log2PageSize uint64, bitsPerLevel uint64, numLevels int) *pwc {
	p := &pwc{}
	p.capacity = capacity
	p.lru = list.New()
	p.table = make(map[pwcKey]*list.Element)

	if capacity <= 0 {
		return p
	}

	pageSizeMask := uint64((1 << log2PageSize) - 1)
	p.pageSizeMask = pageSizeMask

	// Replicate MGVM directory masks: cache levels 0..(numLevels-2).
	// With numLevels=4, levels 0..2 are cacheable.
	curMask := ^pageSizeMask
	curMask = curMask << bitsPerLevel
	for i := 2; i >= 0; i-- {
		p.masks[i] = curMask
		curMask = curMask << bitsPerLevel
	}
	return p
}

func (p *pwc) lookup(pid vm.PID, vAddr uint64) (ppn uint64, level int, ok bool) {
	if p.capacity <= 0 {
		return 0, 0, false
	}

	aligned := vAddr &^ p.pageSizeMask
	// Longest prefix match: try deeper levels first.
	for l := 2; l >= 0; l-- {
		tag := aligned & p.masks[l]
		key := pwcKey{pid: pid, level: l, tag: tag}
		if el, found := p.table[key]; found {
			p.lru.MoveToFront(el)
			ent := el.Value.(pwcEntry)
			return ent.ppn, ent.key.level, true
		}
	}
	return 0, 0, false
}

func (p *pwc) insert(pid vm.PID, vAddr uint64, level int, ppn uint64) {
	if p.capacity <= 0 {
		return
	}
	if level < 0 || level >= 3 {
		return
	}

	aligned := vAddr &^ p.pageSizeMask
	tag := aligned & p.masks[level]
	key := pwcKey{pid: pid, level: level, tag: tag}

	if el, found := p.table[key]; found {
		ent := el.Value.(pwcEntry)
		ent.ppn = ppn
		el.Value = ent
		p.lru.MoveToFront(el)
		return
	}

	// Evict.
	if p.lru.Len() >= p.capacity {
		victim := p.lru.Back()
		if victim != nil {
			ent := victim.Value.(pwcEntry)
			delete(p.table, ent.key)
			p.lru.Remove(victim)
		}
	}

	el := p.lru.PushFront(pwcEntry{key: key, ppn: ppn})
	p.table[key] = el
}
