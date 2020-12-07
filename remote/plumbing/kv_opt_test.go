package plumbing_test

import (
	"os"

	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/remote/plumbing"
	"github.com/make-os/kit/remote/types"
	"github.com/make-os/kit/testutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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

	Describe(".MatchOpt", func() {
		It("should create MatchOpt with expected key and value", func() {
			opt := plumbing.MatchOpt("data")
			Expect(opt.Key).To(Equal("match"))
			Expect(opt.Value).To(Equal("data"))
		})
	})

	Describe(".ChangesOpt", func() {
		It("should create ChangesOpt with expected key and value", func() {
			cs := &types.Changes{References: &types.ChangeResult{}}
			opt := plumbing.ChangesOpt(cs)
			Expect(opt.Key).To(Equal("changes"))
			Expect(opt.Value).To(Equal(cs))
		})
	})
})
