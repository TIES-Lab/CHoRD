package kernels

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sarchlab/mgpusim/v4/amd/insts"
)

var _ = Describe("GridBuilder", func() {

	var (
		builder *gridBuilderImpl
	)

	BeforeEach(func() {
		builder = &gridBuilderImpl{}
	})

	It("should sanitize 1D split factors to keep at least 4 threads", func() {
		packet := new(HsaKernelDispatchPacket)
		packet.WorkgroupSizeX = 64
		packet.WorkgroupSizeY = 1
		packet.WorkgroupSizeZ = 1
		packet.GridSizeX = 64
		packet.GridSizeY = 1
		packet.GridSizeZ = 1

		shape, ok := chooseSubWGShape(packet, 32)
		Expect(ok).To(BeTrue())
		Expect(shape.factor).To(Equal(16))
		Expect(shape.subX).To(Equal(4))
		Expect(shape.subY).To(Equal(1))
		Expect(SanitizeSubWGFactor(packet, 32)).To(Equal(16))
	})

	It("should keep 2D split shapes balanced and above the 2x2 floor", func() {
		packet := new(HsaKernelDispatchPacket)
		packet.WorkgroupSizeX = 8
		packet.WorkgroupSizeY = 8
		packet.WorkgroupSizeZ = 1
		packet.GridSizeX = 8
		packet.GridSizeY = 8
		packet.GridSizeZ = 1

		shape, ok := chooseSubWGShape(packet, 32)
		Expect(ok).To(BeTrue())
		Expect(shape.factor).To(Equal(16))
		Expect(shape.subX).To(Equal(2))
		Expect(shape.subY).To(Equal(2))
		Expect(SanitizeSubWGFactor(packet, 32)).To(Equal(16))
	})

	It("should build balanced 2D sub-workgroups", func() {
		codeObject := new(insts.HsaCo)
		packet := new(HsaKernelDispatchPacket)
		packet.WorkgroupSizeX = 8
		packet.WorkgroupSizeY = 8
		packet.WorkgroupSizeZ = 1
		packet.GridSizeX = 8
		packet.GridSizeY = 8
		packet.GridSizeZ = 1
		builder.SetKernel(KernelLaunchInfo{
			CodeObject:  codeObject,
			Packet:      packet,
			PacketAddr:  0,
			SubWGFactor: 4,
		})

		Expect(builder.NumWG()).To(Equal(4))

		wg0 := builder.NextWG()
		wg1 := builder.NextWG()
		wg2 := builder.NextWG()
		wg3 := builder.NextWG()
		wg4 := builder.NextWG()

		for _, wg := range []*WorkGroup{wg0, wg1, wg2, wg3} {
			Expect(wg).NotTo(BeNil())
			Expect(wg.CurrSizeX).To(Equal(4))
			Expect(wg.CurrSizeY).To(Equal(4))
			Expect(wg.WorkItems).To(HaveLen(16))
		}
		Expect(wg0.LocalXOffset).To(Equal(0))
		Expect(wg0.LocalYOffset).To(Equal(0))
		Expect(wg1.LocalXOffset).To(Equal(4))
		Expect(wg1.LocalYOffset).To(Equal(0))
		Expect(wg2.LocalXOffset).To(Equal(0))
		Expect(wg2.LocalYOffset).To(Equal(4))
		Expect(wg3.LocalXOffset).To(Equal(4))
		Expect(wg3.LocalYOffset).To(Equal(4))
		Expect(wg4).To(BeNil())
	})

	It("should build partial wavefront", func() {
		codeObject := new(insts.HsaCo)
		packet := new(HsaKernelDispatchPacket)
		packet.WorkgroupSizeX = 64
		packet.WorkgroupSizeY = 1
		packet.WorkgroupSizeZ = 1
		packet.GridSizeX = 32
		packet.GridSizeY = 1
		packet.GridSizeZ = 1
		builder.SetKernel(KernelLaunchInfo{
			CodeObject: codeObject,
			Packet:     packet,
			PacketAddr: 0,
		})

		wg := builder.NextWG()

		Expect(builder.NumWG()).To(Equal(1))
		Expect(wg.SizeX).To(Equal(64))
		Expect(wg.CurrSizeX).To(Equal(32))
		Expect(wg.Wavefronts).To(HaveLen(1))
		Expect(wg.WorkItems).To(HaveLen(32))
		Expect(wg.Wavefronts[0].InitExecMask).
			To(Equal(uint64(0x00000000ffffffff)))
	})

	It("should build partial 2d wavefront", func() {
		codeObject := new(insts.HsaCo)
		packet := new(HsaKernelDispatchPacket)
		packet.WorkgroupSizeX = 8
		packet.WorkgroupSizeY = 8
		packet.WorkgroupSizeZ = 1
		packet.GridSizeX = 12
		packet.GridSizeY = 4
		packet.GridSizeZ = 1
		builder.SetKernel(KernelLaunchInfo{
			CodeObject: codeObject,
			Packet:     packet,
			PacketAddr: 0,
		})

		Expect(builder.NumWG()).To(Equal(2))

		wg1 := builder.NextWG()
		Expect(wg1.SizeX).To(Equal(8))
		Expect(wg1.SizeY).To(Equal(8))
		Expect(wg1.CurrSizeX).To(Equal(8))
		Expect(wg1.CurrSizeY).To(Equal(4))
		Expect(wg1.Wavefronts).To(HaveLen(1))
		Expect(wg1.WorkItems).To(HaveLen(32))
		Expect(wg1.Wavefronts[0].InitExecMask).
			To(Equal(uint64(0x00000000ffffffff)))

		wg2 := builder.NextWG()
		Expect(wg2.SizeX).To(Equal(8))
		Expect(wg2.SizeY).To(Equal(8))
		Expect(wg2.CurrSizeX).To(Equal(4))
		Expect(wg2.CurrSizeY).To(Equal(4))
		Expect(wg2.Wavefronts).To(HaveLen(1))
		Expect(wg2.WorkItems).To(HaveLen(16))
		Expect(wg2.Wavefronts[0].InitExecMask).
			To(Equal(uint64(0x000000000f0f0f0f)))

	})

	It("should build 1D grid workgroup", func() {
		codeObject := new(insts.HsaCo)
		packet := new(HsaKernelDispatchPacket)
		packet.WorkgroupSizeX = 256
		packet.WorkgroupSizeY = 1
		packet.WorkgroupSizeZ = 1
		packet.GridSizeX = 1025
		packet.GridSizeY = 1
		packet.GridSizeZ = 1
		builder.SetKernel(KernelLaunchInfo{
			CodeObject: codeObject,
			Packet:     packet,
			PacketAddr: 0,
		})

		wg1 := builder.NextWG()
		wg2 := builder.NextWG()
		wg3 := builder.NextWG()
		wg4 := builder.NextWG()
		wg5 := builder.NextWG()
		wg6 := builder.NextWG()

		Expect(builder.NumWG()).To(Equal(5))
		Expect(wg1.SizeX).To(Equal(256))
		Expect(wg1.SizeY).To(Equal(1))
		Expect(wg1.SizeZ).To(Equal(1))
		Expect(wg1.IDX).To(Equal(0))
		Expect(wg1.IDY).To(Equal(0))
		Expect(wg1.IDZ).To(Equal(0))
		Expect(wg1.Wavefronts).To(HaveLen(4))
		Expect(wg1.WorkItems).To(HaveLen(256))
		Expect(wg2.SizeX).To(Equal(256))
		Expect(wg2.SizeY).To(Equal(1))
		Expect(wg2.SizeZ).To(Equal(1))
		Expect(wg2.IDX).To(Equal(1))
		Expect(wg2.IDY).To(Equal(0))
		Expect(wg2.IDZ).To(Equal(0))
		Expect(wg3.IDX).To(Equal(2))
		Expect(wg4.IDX).To(Equal(3))
		Expect(wg5.IDX).To(Equal(4))
		Expect(wg5.CurrSizeX).To(Equal(1))
		Expect(wg6).To(BeNil())
	})

	It("should build 2D grid", func() {
		codeObject := new(insts.HsaCo)
		packet := new(HsaKernelDispatchPacket)
		packet.WorkgroupSizeX = 16
		packet.WorkgroupSizeY = 16
		packet.WorkgroupSizeZ = 1
		packet.GridSizeX = 33
		packet.GridSizeY = 17
		packet.GridSizeZ = 1
		builder.SetKernel(KernelLaunchInfo{
			CodeObject: codeObject,
			Packet:     packet,
			PacketAddr: 0,
		})

		wg1 := builder.NextWG()
		wg2 := builder.NextWG()
		wg3 := builder.NextWG()
		wg4 := builder.NextWG()
		wg5 := builder.NextWG()
		wg6 := builder.NextWG()
		wg7 := builder.NextWG()

		Expect(builder.NumWG()).To(Equal(6))

		Expect(wg1.SizeX).To(Equal(16))
		Expect(wg1.SizeY).To(Equal(16))
		Expect(wg1.SizeZ).To(Equal(1))
		Expect(wg1.IDX).To(Equal(0))
		Expect(wg1.IDY).To(Equal(0))
		Expect(wg1.IDZ).To(Equal(0))
		Expect(wg1.Wavefronts).To(HaveLen(4))
		Expect(wg1.WorkItems).To(HaveLen(256))

		Expect(wg2.SizeX).To(Equal(16))
		Expect(wg2.SizeY).To(Equal(16))
		Expect(wg2.SizeZ).To(Equal(1))
		Expect(wg2.IDX).To(Equal(1))
		Expect(wg2.IDY).To(Equal(0))
		Expect(wg2.IDZ).To(Equal(0))

		Expect(wg3.IDX).To(Equal(2))
		Expect(wg3.IDY).To(Equal(0))
		Expect(wg3.CurrSizeX).To(Equal(1))
		Expect(wg3.CurrSizeY).To(Equal(16))
		Expect(wg3.Wavefronts).To(HaveLen(4))

		Expect(wg4.IDX).To(Equal(0))
		Expect(wg4.IDY).To(Equal(1))
		Expect(wg4.CurrSizeX).To(Equal(16))
		Expect(wg4.CurrSizeY).To(Equal(1))
		Expect(wg4.Wavefronts).To(HaveLen(1))

		Expect(wg5.IDX).To(Equal(1))
		Expect(wg5.IDY).To(Equal(1))
		Expect(wg5.CurrSizeX).To(Equal(16))
		Expect(wg5.CurrSizeY).To(Equal(1))
		Expect(wg5.Wavefronts).To(HaveLen(1))

		Expect(wg6.IDX).To(Equal(2))
		Expect(wg6.IDY).To(Equal(1))
		Expect(wg6.CurrSizeX).To(Equal(1))
		Expect(wg6.CurrSizeY).To(Equal(1))
		Expect(wg6.Wavefronts).To(HaveLen(1))

		Expect(wg7).To(BeNil())
	})
})
