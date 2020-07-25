package plumbing_test

import (
	"os"
	"strings"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/themakeos/lobe/config"
	"github.com/themakeos/lobe/remote/plumbing"
	"github.com/themakeos/lobe/testutil"
)

var _ = Describe("Common", func() {
	var err error
	var cfg *config.AppConfig
	var ctrl *gomock.Controller

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".MakeCommitHash", func() {
		It("should make a commit hash of the given string", func() {
			Expect(plumbing.MakeCommitHash("data")).To(HaveLen(20))
		})
	})

	Describe(".IsZeroHash", func() {
		It("should return true if hash is a zero hash", func() {
			Expect(plumbing.IsZeroHash(strings.Repeat("0", 40))).To(BeTrue())
			Expect(plumbing.IsZeroHash(strings.Repeat("a", 40))).To(BeFalse())
		})
	})

	Describe(".HashToBytes", func() {
		It("should panic if input is not hex encoded", func() {
			Expect(func() { plumbing.HashToBytes("invalid") }).To(Panic())
		})

		It("should return 20 bytes value", func() {
			bz := plumbing.HashToBytes("f1b7adc21d97cfe61d0594c7f58af61d4631d02a")
			Expect(bz).To(HaveLen(20))
		})
	})

	Describe(".BytesToHex", func() {
		It("should return expected hash", func() {
			hash := "f1b7adc21d97cfe61d0594c7f58af61d4631d02a"
			bz := plumbing.HashToBytes(hash)
			Expect(plumbing.BytesToHex(bz)).To(Equal(hash))
		})
	})

	Describe(".BytesToHash", func() {
		It("should convert to plumbing.Hash", func() {
			hash := "f1b7adc21d97cfe61d0594c7f58af61d4631d02a"
			bz := plumbing.HashToBytes(hash)
			res := plumbing.BytesToHash(bz)
			Expect(res.String()).To(Equal(hash))
		})
	})
})
