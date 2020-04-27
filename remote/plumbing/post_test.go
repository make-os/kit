package plumbing_test

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/remote/plumbing"
	repo2 "gitlab.com/makeos/mosdef/remote/repo"
	testutil2 "gitlab.com/makeos/mosdef/remote/testutil"
	"gitlab.com/makeos/mosdef/testutil"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/util"
	plumbing2 "gopkg.in/src-d/go-git.v4/plumbing"
)

var _ = Describe("Post", func() {
	var err error
	var cfg *config.AppConfig
	var repo core.BareRepo
	var repoName, path string

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"

		repoName = util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		testutil2.ExecGit(cfg.GetRepoRoot(), "init", repoName)

		repo, err = repo2.GetRepoWithLiteGit(cfg.Node.GitBinPath, path)
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".GetPosts", func() {
		It("should return empty slice when no post reference is found by filter", func() {
			post, err := plumbing.GetPosts(repo, func(ref *plumbing2.Reference) bool { return false })
			Expect(err).To(BeNil())
			Expect(post).To(BeEmpty())
		})

		It("should return 1 post when a post reference is found by filter", func() {
			testutil2.AppendCommit(path, "file.txt", "some text 1", "commit 1")
			testutil2.CreateCheckoutOrphanBranch(path, "issues/1")
			testutil2.AppendCommit(path, "body", "some text 1", "commit 1")
			posts, err := plumbing.GetPosts(repo, func(ref *plumbing2.Reference) bool {
				return strings.Contains(ref.Name().String(), "issues")
			})
			Expect(err).To(BeNil())
			Expect(posts).To(HaveLen(1))
		})

		It("should return err when a post reference does not include body file", func() {
			testutil2.AppendCommit(path, "file.txt", "some text 1", "commit 1")
			testutil2.CreateCheckoutOrphanBranch(path, "issues/1")
			testutil2.AppendCommit(path, "some_file", "some text 1", "commit 1")
			_, err := plumbing.GetPosts(repo, func(ref *plumbing2.Reference) bool {
				return strings.Contains(ref.Name().String(), "issues")
			})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("body file is missing in refs/heads/issues/1"))
		})

		It("should return err when a post reference does not include body file", func() {
			testutil2.AppendCommit(path, "file.txt", "some text 1", "commit 1")
			testutil2.CreateCheckoutOrphanBranch(path, "issues/1")
			testutil2.AppendCommit(path, "body", "---\nbad body: {{}123\n---", "commit 2")
			_, err := plumbing.GetPosts(repo, func(ref *plumbing2.Reference) bool {
				return strings.Contains(ref.Name().String(), "issues")
			})
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("root commit of refs/heads/issues/1 has bad body file"))
		})
	})

	Describe(".GetCommentPreview", func() {
		It("should return sentence", func() {
			prev := plumbing.GetCommentPreview(&plumbing.Comment{Content: "This is a simulation. We are in a simulation."})
			Expect(strings.TrimSpace(prev)).To(Equal("This is a simulation..."))

			prev = plumbing.GetCommentPreview(&plumbing.Comment{Content: "This is a simulation."})
			Expect(strings.TrimSpace(prev)).To(Equal("This is a simulation."))
		})
	})
})
