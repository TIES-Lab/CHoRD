package internal

// DeviceType marks the type of a device.
type DeviceType int

// Defines supported devices.
const (
	DeviceTypeInvalid DeviceType = iota
	DeviceTypeCPU
	DeviceTypeGPU
	DeviceTypeUnifiedGPU
)

// AllocatorType marks the type of memory allocator.
type AllocatorType int

// Defines supported allocation algorithms.
const (
	AllocatorTypeDefault AllocatorType = iota
	AllocatorTypeBuddy
	// AllocatorTypePartitioned allocates physical pages in an MGVM-style
	// page-granularity interleaved layout across chiplets. The chiplet owning a
	// page is determined by the (page-indexed) memory bank index.
	AllocatorTypePartitioned
)

// MemoryAllocatorType is a global flag variable for setting the allocator type.
// It is configured before building the platform.
var MemoryAllocatorType = AllocatorTypeDefault

// Chiplet memory layout configuration.
//
// These values are used by AllocatorTypePartitioned. They must be configured
// before the platform is built.
var (
	// NumChipletsPerGPU is the number of chiplets inside one GPU.
	NumChipletsPerGPU = 1
	// NumMemoryBanksPerChiplet is the number of memory banks per chiplet.
	// In MGVM, this is typically 16.
	NumMemoryBanksPerChiplet = 16
)

// DeviceProperties defines the properties of a device
type DeviceProperties struct {
	CUCount  int
	DRAMSize uint64
}

// Device is a CPU or GPU managed by the driver.
type Device struct {
	ID                 int
	Type               DeviceType
	UnifiedGPUIDs      []int
	ActualGPUs         []*Device
	nextActualGPUIndex int
	MemState           DeviceMemoryState
	Properties         DeviceProperties
}

// SetTotalMemSize sets total memory size
func (d *Device) SetTotalMemSize(size uint64) {
	d.MemState.setStorageSize(size)
}

func (d *Device) allocatePage() (pAddr uint64) {
	if d.Type == DeviceTypeUnifiedGPU {
		return d.allocateUnifiedGPUPage()
	}

	d.mustHaveSpaceLeft()
	pAddr = d.MemState.popNextAvailablePAddrs()

	return pAddr
}

func (d *Device) allocateMultiplePages(numPages int) (pAddrs []uint64) {
	if d.Type == DeviceTypeUnifiedGPU {
		return d.allocateMultipleUnifiedGPUPages(numPages)
	}

	d.mustHaveSpaceLeft()
	pAddrs = d.MemState.allocateMultiplePages(numPages)

	return pAddrs
}

// allocatePageOnChiplet allocates a page that belongs to a specific chiplet.
//
// For non-chiplet allocators, this method behaves the same as allocatePage.
func (d *Device) allocatePageOnChiplet(chiplet int) (pAddr uint64) {
	if d.Type == DeviceTypeUnifiedGPU {
		// Unified GPU does not model chiplets; map the "chiplet" id to an actual GPU.
		if len(d.ActualGPUs) == 0 {
			panic("unified GPU has no underlying devices")
		}
		dev := d.ActualGPUs[chiplet%len(d.ActualGPUs)]
		return dev.allocatePage()
	}

	d.mustHaveSpaceLeft()
	pAddr = d.MemState.allocatePageOnChiplet(chiplet)
	return pAddr
}

func (d *Device) mustHaveSpaceLeft() {
	if d.MemState.noAvailablePAddrs() {
		panic("out of memory")
	}
}

func (d *Device) allocateUnifiedGPUPage() (pAddr uint64) {
	var devSelected *Device

	devSelected = nil
	for i := 0; i < len(d.ActualGPUs); i++ {
		devIndex := (d.nextActualGPUIndex + i) % len(d.ActualGPUs)
		dev := d.ActualGPUs[devIndex]

		if dev.MemState.noAvailablePAddrs() {
			continue
		}

		devSelected = dev
	}

	if devSelected == nil {
		panic("out of memory")
	}

	pAddr = devSelected.allocatePage()
	d.nextActualGPUIndex = (d.nextActualGPUIndex + 1) % len(d.ActualGPUs)
	return pAddr
}

func (d *Device) allocateMultipleUnifiedGPUPages(numPages int) (pAddrs []uint64) {
	dev := d.ActualGPUs[d.nextActualGPUIndex]
	pAddrs = dev.allocateMultiplePages(numPages)
	d.nextActualGPUIndex = (d.nextActualGPUIndex + 1) % len(d.ActualGPUs)
	return pAddrs
}
