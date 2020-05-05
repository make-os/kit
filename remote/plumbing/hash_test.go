package plumbing_test

import (
	"os"
	"strings"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/remote/plumbing"
	"gitlab.com/makeos/mosdef/testutil"
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
			Expect(plumbing.MakeCommitHash("data").String()).To(HaveLen(40))
		})
	})

	Describe(".IsZeroHash", func() {
		It("should return true if hash is a zero hash", func() {
			Expect(plumbing.IsZeroHash(strings.Repeat("0", 40))).To(BeTrue())
			Expect(plumbing.IsZeroHash(strings.Repeat("a", 40))).To(BeFalse())
		})
	})
})
