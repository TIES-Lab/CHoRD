package driver

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Defaultmemorycopymiddleware", func() {
	It("should treat touching ranges as non-overlapping", func() {
		Expect(memRangeOverlap(0, 4096, 4096, 8192)).To(BeFalse())
		Expect(memRangeOverlap(4096, 8192, 0, 4096)).To(BeFalse())
	})

	It("should detect real overlap for half-open intervals", func() {
		Expect(memRangeOverlap(0, 4096, 2048, 6144)).To(BeTrue())
		Expect(memRangeOverlap(2048, 6144, 0, 4096)).To(BeTrue())
	})

	It("should remove all freed buffers without panicking", func() {
		ctx := &Context{
			buffers: []*buffer{
				{vAddr: 1, size: 16},
				{vAddr: 2, size: 16, freed: true},
				{vAddr: 3, size: 16, freed: true},
				{vAddr: 4, size: 16},
			},
		}

		Expect(func() { ctx.removeFreedBuffers() }).NotTo(Panic())
		Expect(ctx.buffers).To(HaveLen(2))
		Expect(ctx.buffers[0].vAddr).To(Equal(Ptr(1)))
		Expect(ctx.buffers[1].vAddr).To(Equal(Ptr(4)))
	})
})
