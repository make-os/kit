package repo

import (
	"fmt"
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/config"
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

	Describe("isIssueBranch", func() {
		It("should return false if not an issue branch name", func() {
			Expect(isIssueBranch("refs/heads/abc")).To(BeFalse())
		})

		It("should return false if not an issue branch name", func() {
			Expect(isIssueBranch(fmt.Sprintf("refs/heads/issues/something-bad"))).To(BeTrue())
		})
	})

	Describe(".isTag", func() {
		Specify("that it returns true for valid tag reference or false for invalids", func() {
			Expect(isTag("refs/heads/branch1")).To(BeFalse())
			Expect(isTag("refs/notes/note1")).To(BeFalse())
			Expect(isTag("refs/tags/tag1")).To(BeTrue())
		})
	})

	Describe(".isNote()", func() {
		Specify("that it returns true for valid note reference or false for invalids", func() {
			Expect(isNote("refs/heads/branch1")).To(BeFalse())
			Expect(isNote("refs/tags/tag1")).To(BeFalse())
			Expect(isNote("refs/notes/note1")).To(BeTrue())
		})
	})

	Describe(".isBranch", func() {
		Specify("that it returns true for valid branch reference or false for invalids", func() {
			Expect(isBranch("refs/heads/branch1")).To(BeTrue())
			Expect(isBranch("refs/heads/branch_1")).To(BeTrue())
			Expect(isBranch("refs/heads/branch-1")).To(BeTrue())
			Expect(isBranch("refs/heads/branches/mine")).To(BeTrue())
			Expect(isBranch("refs/tags/tag1")).To(BeFalse())
			Expect(isBranch("refs/notes/note1")).To(BeFalse())
		})
	})

	Describe(".isReference", func() {
		It("should return false if reference is not valid", func() {
			Expect(isReference("refs/something/something")).To(BeFalse())
			Expect(isReference("refs/heads/issues/something-bad/")).To(BeFalse())
			Expect(isReference("refs/heads/issues/something-bad//")).To(BeFalse())
		})
		It("should return true if reference is valid", func() {
			Expect(isReference("refs/heads/branch-name")).To(BeTrue())
			Expect(isReference("refs/heads/issues/some_thing-bad/happened")).To(BeTrue())
			Expect(isReference("refs/heads")).To(BeTrue())
			Expect(isReference("refs/tags")).To(BeTrue())
			Expect(isReference("refs/notes")).To(BeTrue())
		})
	})
})
