package epoch_test

import (
	"testing"

	"github.com/make-os/kit/util/epoch"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/make-os/kit/params"
)

func TestParams(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Params Suite")
}

var _ = Describe("EpochHelpers", func() {
	NumBlocksPerEpoch = 5

	Describe(".GetEpochAt", func() {
		It("should get expected epoch", func() {
			Expect(epoch.GetEpochAt(1)).To(Equal(int64(1)))
			Expect(epoch.GetEpochAt(4)).To(Equal(int64(1)))
			Expect(epoch.GetEpochAt(5)).To(Equal(int64(1)))
			Expect(epoch.GetEpochAt(6)).To(Equal(int64(2)))
			Expect(epoch.GetEpochAt(9)).To(Equal(int64(2)))
			Expect(epoch.GetEpochAt(10)).To(Equal(int64(2)))
			Expect(epoch.GetEpochAt(11)).To(Equal(int64(3)))
		})
	})

	Describe(".GetFirstInEpochOfHeight", func() {
		It("should get expected height", func() {
			Expect(epoch.GetFirstInEpochOfHeight(1)).To(Equal(int64(1)))
			Expect(epoch.GetFirstInEpochOfHeight(4)).To(Equal(int64(1)))
			Expect(epoch.GetFirstInEpochOfHeight(5)).To(Equal(int64(1)))
			Expect(epoch.GetFirstInEpochOfHeight(6)).To(Equal(int64(6)))
			Expect(epoch.GetFirstInEpochOfHeight(9)).To(Equal(int64(6)))
			Expect(epoch.GetFirstInEpochOfHeight(10)).To(Equal(int64(6)))
			Expect(epoch.GetFirstInEpochOfHeight(11)).To(Equal(int64(11)))
		})
	})

	Describe(".GetLastHeightInEpochOfHeight", func() {
		It("should get expected height", func() {
			Expect(epoch.GetLastHeightInEpochOfHeight(1)).To(Equal(int64(5)))
			Expect(epoch.GetLastHeightInEpochOfHeight(4)).To(Equal(int64(5)))
			Expect(epoch.GetLastHeightInEpochOfHeight(5)).To(Equal(int64(5)))
			Expect(epoch.GetLastHeightInEpochOfHeight(6)).To(Equal(int64(10)))
			Expect(epoch.GetLastHeightInEpochOfHeight(9)).To(Equal(int64(10)))
			Expect(epoch.GetLastHeightInEpochOfHeight(10)).To(Equal(int64(10)))
			Expect(epoch.GetLastHeightInEpochOfHeight(11)).To(Equal(int64(15)))
		})
	})

	Describe(".GetLastHeightInEpochOfHeight", func() {
		It("should panic if epoch=0", func() {
			Expect(func() { epoch.GetFirstInEpoch(0) }).To(Panic())
		})

		It("should get expected height", func() {
			Expect(epoch.GetFirstInEpoch(1)).To(Equal(int64(1)))
			Expect(epoch.GetFirstInEpoch(3)).To(Equal(int64(11)))
			Expect(epoch.GetFirstInEpoch(2)).To(Equal(int64(6)))
			Expect(epoch.GetFirstInEpoch(30)).To(Equal(int64(146)))
		})
	})

	Describe(".GetSeedInEpochOfHeight", func() {
		It("should get expected height", func() {
			Expect(epoch.GetSeedInEpochOfHeight(1)).To(Equal(int64(3)))
			Expect(epoch.GetSeedInEpochOfHeight(4)).To(Equal(int64(3)))
			Expect(epoch.GetSeedInEpochOfHeight(5)).To(Equal(int64(3)))
			Expect(epoch.GetSeedInEpochOfHeight(6)).To(Equal(int64(8)))
			Expect(epoch.GetSeedInEpochOfHeight(9)).To(Equal(int64(8)))
			Expect(epoch.GetSeedInEpochOfHeight(10)).To(Equal(int64(8)))
			Expect(epoch.GetSeedInEpochOfHeight(11)).To(Equal(int64(13)))
		})
	})

	Describe(".GetLastInParentOfEpochOfHeight", func() {
		It("should get expected height", func() {
			Expect(epoch.GetLastInParentOfEpochOfHeight(1)).To(Equal(int64(0)))
			Expect(epoch.GetLastInParentOfEpochOfHeight(4)).To(Equal(int64(0)))
			Expect(epoch.GetLastInParentOfEpochOfHeight(5)).To(Equal(int64(0)))
			Expect(epoch.GetLastInParentOfEpochOfHeight(6)).To(Equal(int64(5)))
			Expect(epoch.GetLastInParentOfEpochOfHeight(9)).To(Equal(int64(5)))
			Expect(epoch.GetLastInParentOfEpochOfHeight(10)).To(Equal(int64(5)))
			Expect(epoch.GetLastInParentOfEpochOfHeight(11)).To(Equal(int64(10)))
		})
	})

	Describe(".IsThirdToLastInEpochOfHeight", func() {
		It("should get expected height", func() {
			Expect(epoch.IsThirdToLastInEpochOfHeight(1)).To(BeFalse())
			Expect(epoch.IsThirdToLastInEpochOfHeight(3)).To(BeTrue())
			Expect(epoch.IsThirdToLastInEpochOfHeight(6)).To(BeFalse())
			Expect(epoch.IsThirdToLastInEpochOfHeight(8)).To(BeTrue())
		})
	})

	Describe(".IsBeforeEndOfEpochOfHeight", func() {
		It("should get expected height", func() {
			Expect(epoch.IsBeforeEndOfEpochOfHeight(3)).To(BeFalse())
			Expect(epoch.IsBeforeEndOfEpochOfHeight(4)).To(BeTrue())
			Expect(epoch.IsBeforeEndOfEpochOfHeight(8)).To(BeFalse())
			Expect(epoch.IsBeforeEndOfEpochOfHeight(9)).To(BeTrue())
		})
	})

	Describe(".IsEndOfEpochOfHeight", func() {
		It("should get expected height", func() {
			Expect(epoch.IsEndOfEpochOfHeight(4)).To(BeFalse())
			Expect(epoch.IsEndOfEpochOfHeight(5)).To(BeTrue())
			Expect(epoch.IsEndOfEpochOfHeight(9)).To(BeFalse())
			Expect(epoch.IsEndOfEpochOfHeight(10)).To(BeTrue())
		})
	})
})
