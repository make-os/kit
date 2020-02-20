package repo

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/testutil"
)

var _ = Describe("Common", func() {
	var err error
	var cfg *config.AppConfig

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
	})

	AfterEach(func() {
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".MakeRepoObjectDHTKey", func() {
		It("should return a string in the format <repo name>/<object hash>", func() {
			key := MakeRepoObjectDHTKey("facebook", "hash")
			Expect(key).To(Equal("facebook/hash"))
		})
	})

	Describe(".ParseRepoObjectDHTKey", func() {
		It("should return error if key not formatted as <repo name>/<object hash", func() {
			_, _, err := ParseRepoObjectDHTKey("invalid")
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("invalid repo object dht key"))
		})

		It("should return repo name and object hash if formatted as <repo name>/<object hash", func() {
			rn, on, err := ParseRepoObjectDHTKey("facebook/hash")
			Expect(err).To(BeNil())
			Expect(rn).To(Equal("facebook"))
			Expect(on).To(Equal("hash"))
		})
	})
})
