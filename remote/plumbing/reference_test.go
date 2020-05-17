package plumbing_test

import (
	"fmt"
	"os"

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

	Describe("isIssueReference", func() {
		It("should return false if not an issue branch name", func() {
			Expect(plumbing.IsIssueReference("refs/heads/abc")).To(BeFalse())
			Expect(plumbing.IsIssueReference(fmt.Sprintf("refs/heads/%s/0001", plumbing.IssueBranchPrefix))).To(BeFalse())
		})

		It("should return true if it is an issue branch name", func() {
			Expect(plumbing.IsIssueReference(fmt.Sprintf("refs/heads/%s/1", plumbing.IssueBranchPrefix))).To(BeTrue())
		})
	})

	Describe(".IsIssueReferencePath", func() {
		It("should return true if string has issue reference path", func() {
			Expect(plumbing.IsIssueReferencePath(fmt.Sprintf("refs/heads/%s/", plumbing.IssueBranchPrefix))).To(BeTrue())
			Expect(plumbing.IsIssueReferencePath(fmt.Sprintf("refs/heads/%s", plumbing.IssueBranchPrefix))).To(BeTrue())
			Expect(plumbing.IsIssueReferencePath("refs/heads/stuffs")).To(BeFalse())
		})
	})

	Describe(".MakeIssueReference", func() {
		It("should create a valid issue reference", func() {
			ref := plumbing.MakeIssueReference(1)
			Expect(plumbing.IsIssueReference(ref)).To(BeTrue())
			ref = plumbing.MakeIssueReference("1")
			Expect(plumbing.IsIssueReference(ref)).To(BeTrue())
		})
	})

	Describe(".IsTag", func() {
		Specify("that it returns true for valid tag reference or false for invalids", func() {
			Expect(plumbing.IsTag("refs/heads/branch1")).To(BeFalse())
			Expect(plumbing.IsTag("refs/notes/note1")).To(BeFalse())
			Expect(plumbing.IsTag("refs/tags/tag1")).To(BeTrue())
		})
	})

	Describe(".IsNote()", func() {
		Specify("that it returns true for valid note reference or false for invalids", func() {
			Expect(plumbing.IsNote("refs/heads/branch1")).To(BeFalse())
			Expect(plumbing.IsNote("refs/tags/tag1")).To(BeFalse())
			Expect(plumbing.IsNote("refs/notes/note1")).To(BeTrue())
		})
	})

	Describe(".IsBranch", func() {
		Specify("that it returns true for valid branch reference or false for invalids", func() {
			Expect(plumbing.IsBranch("refs/heads/branch1")).To(BeTrue())
			Expect(plumbing.IsBranch("refs/heads/branch_1")).To(BeTrue())
			Expect(plumbing.IsBranch("refs/heads/branch-1")).To(BeTrue())
			Expect(plumbing.IsBranch("refs/heads/branches/mine")).To(BeTrue())
			Expect(plumbing.IsBranch("refs/tags/tag1")).To(BeFalse())
			Expect(plumbing.IsBranch("refs/notes/note1")).To(BeFalse())
		})
	})

	Describe(".IsReference", func() {
		It("should return false if reference is not valid", func() {
			Expect(plumbing.IsReference("refs/something/something")).To(BeFalse())
			Expect(plumbing.IsReference("refs/heads/issues/something-bad/")).To(BeFalse())
			Expect(plumbing.IsReference("refs/heads/issues/something-bad//")).To(BeFalse())
		})
		It("should return true if reference is valid", func() {
			Expect(plumbing.IsReference("refs/heads/branch-name")).To(BeTrue())
			Expect(plumbing.IsReference("refs/heads/issues/some_thing-bad/happened")).To(BeTrue())
			Expect(plumbing.IsReference("refs/heads")).To(BeTrue())
			Expect(plumbing.IsReference("refs/tags")).To(BeTrue())
			Expect(plumbing.IsReference("refs/notes")).To(BeTrue())
		})
	})

	Describe(".MakeIssueReferencePath", func() {
		It("should return refs/heads/<issues_branch_prefix>", func() {
			Expect(plumbing.MakeIssueReferencePath()).To(Equal(fmt.Sprintf("refs/heads/" + plumbing.IssueBranchPrefix)))
		})
	})
})
