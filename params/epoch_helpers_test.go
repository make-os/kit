package params_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/make-os/kit/params"
)

var _ = Describe("EpochHelpers", func() {
	Describe(".GetEpochOfHeight", func() {
		It("should get expected epoch", func() {
			NumBlocksPerEpoch = 5
			Expect(GetEpochOfHeight(1)).To(Equal(int64(1)))
			Expect(GetEpochOfHeight(4)).To(Equal(int64(1)))
			Expect(GetEpochOfHeight(5)).To(Equal(int64(1)))
			Expect(GetEpochOfHeight(6)).To(Equal(int64(2)))
			Expect(GetEpochOfHeight(9)).To(Equal(int64(2)))
			Expect(GetEpochOfHeight(10)).To(Equal(int64(2)))
			Expect(GetEpochOfHeight(11)).To(Equal(int64(3)))
		})
	})

	Describe(".GetStartOfEpochOfHeight", func() {
		It("should get expected height", func() {
			NumBlocksPerEpoch = 5
			Expect(GetStartOfEpochOfHeight(1)).To(Equal(int64(1)))
			Expect(GetStartOfEpochOfHeight(4)).To(Equal(int64(1)))
			Expect(GetStartOfEpochOfHeight(5)).To(Equal(int64(1)))
			Expect(GetStartOfEpochOfHeight(6)).To(Equal(int64(6)))
			Expect(GetStartOfEpochOfHeight(9)).To(Equal(int64(6)))
			Expect(GetStartOfEpochOfHeight(10)).To(Equal(int64(6)))
			Expect(GetStartOfEpochOfHeight(11)).To(Equal(int64(11)))
		})
	})

	Describe(".GetEndOfEpochOfHeight", func() {
		It("should get expected height", func() {
			NumBlocksPerEpoch = 5
			Expect(GetEndOfEpochOfHeight(1)).To(Equal(int64(5)))
			Expect(GetEndOfEpochOfHeight(4)).To(Equal(int64(5)))
			Expect(GetEndOfEpochOfHeight(5)).To(Equal(int64(5)))
			Expect(GetEndOfEpochOfHeight(6)).To(Equal(int64(10)))
			Expect(GetEndOfEpochOfHeight(9)).To(Equal(int64(10)))
			Expect(GetEndOfEpochOfHeight(10)).To(Equal(int64(10)))
			Expect(GetEndOfEpochOfHeight(11)).To(Equal(int64(15)))
		})
	})

	Describe(".GetSeedHeightInEpochOfHeight", func() {
		It("should get expected height", func() {
			NumBlocksPerEpoch = 5
			Expect(GetSeedHeightInEpochOfHeight(1)).To(Equal(int64(3)))
			Expect(GetSeedHeightInEpochOfHeight(4)).To(Equal(int64(3)))
			Expect(GetSeedHeightInEpochOfHeight(5)).To(Equal(int64(3)))
			Expect(GetSeedHeightInEpochOfHeight(6)).To(Equal(int64(8)))
			Expect(GetSeedHeightInEpochOfHeight(9)).To(Equal(int64(8)))
			Expect(GetSeedHeightInEpochOfHeight(10)).To(Equal(int64(8)))
			Expect(GetSeedHeightInEpochOfHeight(11)).To(Equal(int64(13)))
		})
	})

	Describe(".GetEndOfParentEpochOfHeight", func() {
		It("should get expected height", func() {
			NumBlocksPerEpoch = 5
			Expect(GetEndOfParentEpochOfHeight(1)).To(Equal(int64(0)))
			Expect(GetEndOfParentEpochOfHeight(4)).To(Equal(int64(0)))
			Expect(GetEndOfParentEpochOfHeight(5)).To(Equal(int64(0)))
			Expect(GetEndOfParentEpochOfHeight(6)).To(Equal(int64(5)))
			Expect(GetEndOfParentEpochOfHeight(9)).To(Equal(int64(5)))
			Expect(GetEndOfParentEpochOfHeight(10)).To(Equal(int64(5)))
			Expect(GetEndOfParentEpochOfHeight(11)).To(Equal(int64(10)))
		})
	})

	Describe(".IsStartOfEndOfEpochOfHeight", func() {
		It("should get expected height", func() {
			NumBlocksPerEpoch = 5
			Expect(IsStartOfEndOfEpochOfHeight(1)).To(BeFalse())
			Expect(IsStartOfEndOfEpochOfHeight(3)).To(BeTrue())
			Expect(IsStartOfEndOfEpochOfHeight(6)).To(BeFalse())
			Expect(IsStartOfEndOfEpochOfHeight(8)).To(BeTrue())
		})
	})

	Describe(".IsBeforeEndOfEpoch", func() {
		It("should get expected height", func() {
			NumBlocksPerEpoch = 5
			Expect(IsBeforeEndOfEpoch(3)).To(BeFalse())
			Expect(IsBeforeEndOfEpoch(4)).To(BeTrue())
			Expect(IsBeforeEndOfEpoch(8)).To(BeFalse())
			Expect(IsBeforeEndOfEpoch(9)).To(BeTrue())
		})
	})

	Describe(".IsEndOfEpoch", func() {
		It("should get expected height", func() {
			NumBlocksPerEpoch = 5
			Expect(IsEndOfEpoch(4)).To(BeFalse())
			Expect(IsEndOfEpoch(5)).To(BeTrue())
			Expect(IsEndOfEpoch(9)).To(BeFalse())
			Expect(IsEndOfEpoch(10)).To(BeTrue())
		})
	})
})
