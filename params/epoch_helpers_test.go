package params_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/make-os/kit/params"
)

var _ = Describe("EpochHelpers", func() {
	NumBlocksPerEpoch = 5

	Describe(".GetEpochOfHeight", func() {
		It("should get expected epoch", func() {
			Expect(GetEpochOfHeight(1)).To(Equal(int64(1)))
			Expect(GetEpochOfHeight(4)).To(Equal(int64(1)))
			Expect(GetEpochOfHeight(5)).To(Equal(int64(1)))
			Expect(GetEpochOfHeight(6)).To(Equal(int64(2)))
			Expect(GetEpochOfHeight(9)).To(Equal(int64(2)))
			Expect(GetEpochOfHeight(10)).To(Equal(int64(2)))
			Expect(GetEpochOfHeight(11)).To(Equal(int64(3)))
		})
	})

	Describe(".GetFirstInEpochOfHeight", func() {
		It("should get expected height", func() {
			Expect(GetFirstInEpochOfHeight(1)).To(Equal(int64(1)))
			Expect(GetFirstInEpochOfHeight(4)).To(Equal(int64(1)))
			Expect(GetFirstInEpochOfHeight(5)).To(Equal(int64(1)))
			Expect(GetFirstInEpochOfHeight(6)).To(Equal(int64(6)))
			Expect(GetFirstInEpochOfHeight(9)).To(Equal(int64(6)))
			Expect(GetFirstInEpochOfHeight(10)).To(Equal(int64(6)))
			Expect(GetFirstInEpochOfHeight(11)).To(Equal(int64(11)))
		})
	})

	Describe(".IsLastInEpochOfHeight", func() {
		It("should get expected height", func() {
			Expect(IsLastInEpochOfHeight(1)).To(Equal(int64(5)))
			Expect(IsLastInEpochOfHeight(4)).To(Equal(int64(5)))
			Expect(IsLastInEpochOfHeight(5)).To(Equal(int64(5)))
			Expect(IsLastInEpochOfHeight(6)).To(Equal(int64(10)))
			Expect(IsLastInEpochOfHeight(9)).To(Equal(int64(10)))
			Expect(IsLastInEpochOfHeight(10)).To(Equal(int64(10)))
			Expect(IsLastInEpochOfHeight(11)).To(Equal(int64(15)))
		})
	})

	Describe(".IsLastInEpochOfHeight", func() {
		It("should panic if epoch=0", func() {
			Expect(func() { GetFirstInEpoch(0) }).To(Panic())
		})

		It("should get expected height", func() {
			Expect(GetFirstInEpoch(1)).To(Equal(int64(1)))
			Expect(GetFirstInEpoch(3)).To(Equal(int64(11)))
			Expect(GetFirstInEpoch(2)).To(Equal(int64(6)))
			Expect(GetFirstInEpoch(30)).To(Equal(int64(146)))
		})
	})

	Describe(".GetSeedInEpochOfHeight", func() {
		It("should get expected height", func() {
			Expect(GetSeedInEpochOfHeight(1)).To(Equal(int64(3)))
			Expect(GetSeedInEpochOfHeight(4)).To(Equal(int64(3)))
			Expect(GetSeedInEpochOfHeight(5)).To(Equal(int64(3)))
			Expect(GetSeedInEpochOfHeight(6)).To(Equal(int64(8)))
			Expect(GetSeedInEpochOfHeight(9)).To(Equal(int64(8)))
			Expect(GetSeedInEpochOfHeight(10)).To(Equal(int64(8)))
			Expect(GetSeedInEpochOfHeight(11)).To(Equal(int64(13)))
		})
	})

	Describe(".GetLastInParentOfEpochOfHeight", func() {
		It("should get expected height", func() {
			Expect(GetLastInParentOfEpochOfHeight(1)).To(Equal(int64(0)))
			Expect(GetLastInParentOfEpochOfHeight(4)).To(Equal(int64(0)))
			Expect(GetLastInParentOfEpochOfHeight(5)).To(Equal(int64(0)))
			Expect(GetLastInParentOfEpochOfHeight(6)).To(Equal(int64(5)))
			Expect(GetLastInParentOfEpochOfHeight(9)).To(Equal(int64(5)))
			Expect(GetLastInParentOfEpochOfHeight(10)).To(Equal(int64(5)))
			Expect(GetLastInParentOfEpochOfHeight(11)).To(Equal(int64(10)))
		})
	})

	Describe(".IsThirdToLastInEpochOfHeight", func() {
		It("should get expected height", func() {
			Expect(IsThirdToLastInEpochOfHeight(1)).To(BeFalse())
			Expect(IsThirdToLastInEpochOfHeight(3)).To(BeTrue())
			Expect(IsThirdToLastInEpochOfHeight(6)).To(BeFalse())
			Expect(IsThirdToLastInEpochOfHeight(8)).To(BeTrue())
		})
	})

	Describe(".IsBeforeEndOfEpochOfHeight", func() {
		It("should get expected height", func() {
			Expect(IsBeforeEndOfEpochOfHeight(3)).To(BeFalse())
			Expect(IsBeforeEndOfEpochOfHeight(4)).To(BeTrue())
			Expect(IsBeforeEndOfEpochOfHeight(8)).To(BeFalse())
			Expect(IsBeforeEndOfEpochOfHeight(9)).To(BeTrue())
		})
	})

	Describe(".IsEndOfEpochOfHeight", func() {
		It("should get expected height", func() {
			Expect(IsEndOfEpochOfHeight(4)).To(BeFalse())
			Expect(IsEndOfEpochOfHeight(5)).To(BeTrue())
			Expect(IsEndOfEpochOfHeight(9)).To(BeFalse())
			Expect(IsEndOfEpochOfHeight(10)).To(BeTrue())
		})
	})
})
