package plumbing_test

import (
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/config"
	plumbing2 "gitlab.com/makeos/mosdef/remote/plumbing"
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

	Describe(".MakeRepoObjectDHTKey", func() {
		It("should return a string in the format <repo name>/<object hash>", func() {
			key := plumbing2.MakeRepoObjectDHTKey("facebook", "hash")
			Expect(key).To(Equal("facebook/hash"))
		})
	})

	Describe(".ParseRepoObjectDHTKey", func() {
		It("should return error if key not formatted as <repo name>/<object hash", func() {
			_, _, err := plumbing2.ParseRepoObjectDHTKey("invalid")
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("invalid repo object dht key"))
		})

		It("should return repo name and object hash if formatted as <repo name>/<object hash", func() {
			rn, on, err := plumbing2.ParseRepoObjectDHTKey("facebook/hash")
			Expect(err).To(BeNil())
			Expect(rn).To(Equal("facebook"))
			Expect(on).To(Equal("hash"))
		})
	})
})
