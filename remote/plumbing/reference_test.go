package plumbing

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

	Describe("isIssueBranch", func() {
		It("should return false if not an issue branch name", func() {
			Expect(IsIssueBranch("refs/heads/abc")).To(BeFalse())
		})

		It("should return false if not an issue branch name", func() {
			Expect(IsIssueBranch(fmt.Sprintf("refs/heads/issues/something-bad"))).To(BeTrue())
		})
	})

	Describe(".IsTag", func() {
		Specify("that it returns true for valid tag reference or false for invalids", func() {
			Expect(IsTag("refs/heads/branch1")).To(BeFalse())
			Expect(IsTag("refs/notes/note1")).To(BeFalse())
			Expect(IsTag("refs/tags/tag1")).To(BeTrue())
		})
	})

	Describe(".IsNote()", func() {
		Specify("that it returns true for valid note reference or false for invalids", func() {
			Expect(IsNote("refs/heads/branch1")).To(BeFalse())
			Expect(IsNote("refs/tags/tag1")).To(BeFalse())
			Expect(IsNote("refs/notes/note1")).To(BeTrue())
		})
	})

	Describe(".IsBranch", func() {
		Specify("that it returns true for valid branch reference or false for invalids", func() {
			Expect(IsBranch("refs/heads/branch1")).To(BeTrue())
			Expect(IsBranch("refs/heads/branch_1")).To(BeTrue())
			Expect(IsBranch("refs/heads/branch-1")).To(BeTrue())
			Expect(IsBranch("refs/heads/branches/mine")).To(BeTrue())
			Expect(IsBranch("refs/tags/tag1")).To(BeFalse())
			Expect(IsBranch("refs/notes/note1")).To(BeFalse())
		})
	})

	Describe(".IsReference", func() {
		It("should return false if reference is not valid", func() {
			Expect(IsReference("refs/something/something")).To(BeFalse())
			Expect(IsReference("refs/heads/issues/something-bad/")).To(BeFalse())
			Expect(IsReference("refs/heads/issues/something-bad//")).To(BeFalse())
		})
		It("should return true if reference is valid", func() {
			Expect(IsReference("refs/heads/branch-name")).To(BeTrue())
			Expect(IsReference("refs/heads/issues/some_thing-bad/happened")).To(BeTrue())
			Expect(IsReference("refs/heads")).To(BeTrue())
			Expect(IsReference("refs/tags")).To(BeTrue())
			Expect(IsReference("refs/notes")).To(BeTrue())
		})
	})
})
