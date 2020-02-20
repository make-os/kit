package repo

import (
	"gitlab.com/makeos/mosdef/types/core"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/testutil"
	"gitlab.com/makeos/mosdef/util"
)

var _ = Describe("Merge", func() {
	var err error
	var cfg *config.AppConfig
	var path string
	var repo core.BareRepo
	var repoRoot string

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
		repoRoot = cfg.GetRepoRoot()
		repoName := util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		execGit(cfg.GetRepoRoot(), "init", repoName)
		repo, err = getRepoWithGitOpt(cfg.Node.GitBinPath, path)
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe("merge", func() {
		When("base branch does not exist", func() {
			BeforeEach(func() {
				err = merge(repo, "unknown_base", "branch2", repoRoot, false)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to find base branch: reference not found"))
			})
		})

		When("target branch format is not valid", func() {
			BeforeEach(func() {
				appendCommit(path, "file.txt", "Hello", "commit 1")
				err = merge(repo, "master", "branch2:stuff:stuff", repoRoot, false)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("invalid target format"))
			})
		})

		When("target branch does not exist in the base repo", func() {
			BeforeEach(func() {
				appendCommit(path, "file.txt", "Hello", "commit 1")
				err = merge(repo, "master", "branch_unknown", repoRoot, false)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to find target branch: reference not found"))
			})
		})

		When("target branch successfully merges into base and 'uncommitted' is false", func() {
			BeforeEach(func() {
				appendCommit(path, "file.txt", "Hello", "commit 1")
				createCheckoutBranch(path, "branch2")
				writeCommit(path, "file.txt", "Hello World!", "commit 2")
				err = merge(repo, "master", "branch2", repoRoot, false)
				Expect(err).To(BeNil())
				checkoutBranch(path, "master")
			})

			Specify("that base branch be updated with content of target", func() {
				content, _ := scriptFile(path, "file.txt").String()
				Expect(content).To(Equal("Hello World!"))
			})
		})

		When("target branch successfully merges into base and 'uncommitted' is true", func() {
			BeforeEach(func() {
				appendCommit(path, "file.txt", "Hello", "commit 1")
				createCheckoutBranch(path, "branch2")
				writeCommit(path, "file.txt", "Hello World!", "commit 2")
				err = merge(repo, "master", "branch2", repoRoot, true)
				Expect(err).To(BeNil())
				checkoutBranch(path, "master")
			})

			Specify("that base branch should not be updated with the content of target", func() {
				content, _ := scriptFile(path, "file.txt").String()
				Expect(content).To(Equal("Hello"))
			})
		})

		When("target branch merge into base results in a merge conflict and 'uncommitted' is false", func() {
			BeforeEach(func() {
				appendCommit(path, "file.txt", "Hello", "commit 1")

				createCheckoutBranch(path, "branch2")
				writeCommit(path, "file.txt", "Hello World!", "commit 1")

				checkoutBranch(path, "master")
				writeCommit(path, "file.txt", "Hello Great People", "commit 2")
				err = merge(repo, "master", "branch2", repoRoot, false)
			})

			Specify("that merge conflict occurred and base branch was unchanged", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("Merge conflict"))
				content, _ := scriptFile(path, "file.txt").String()
				Expect(content).To(Equal("Hello Great People"))
			})
		})

		When("target branch merge into base results in a merge conflict and 'uncommitted' is true", func() {
			BeforeEach(func() {
				appendCommit(path, "file.txt", "Hello", "commit 1")

				createCheckoutBranch(path, "branch2")
				writeCommit(path, "file.txt", "Hello World!", "commit 1")

				checkoutBranch(path, "master")
				writeCommit(path, "file.txt", "Hello Great People", "commit 2")
				err = merge(repo, "master", "branch2", repoRoot, true)
			})

			Specify("that merge conflict occurred and base branch was unchanged", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("Merge conflict"))
				content, _ := scriptFile(path, "file.txt").String()
				Expect(content).To(Equal("Hello Great People"))
			})
		})

		When("target branch repo is unknown", func() {
			BeforeEach(func() {
				appendCommit(path, "file.txt", "Hello", "commit 1")
				err = merge(repo, "master", "unknown_repo:branch2", repoRoot, false)
			})

			It("should return error about unknown target repo", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to find target branch's " +
					"repo: repository does not exist"))
			})
		})

		When("target repo exist but target branch does not exist", func() {
			var targetRepoPath string
			var targetRepoName = util.RandString(10)

			BeforeEach(func() {
				targetRepoPath = filepath.Join(cfg.GetRepoRoot(), targetRepoName)
				execGit(cfg.GetRepoRoot(), "init", targetRepoName)
				appendCommit(path, "file.txt", "Hello", "commit 1")
				err = merge(repo, "master", targetRepoName+":unknown_branch", repoRoot, false)
			})

			It("should return error about unknown target repo", func() {
				_ = targetRepoPath
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to find target branch: reference not found"))
			})
		})

		When("target repo and branch exist but share no common commit", func() {
			var targetRepoPath string
			var targetRepoName = util.RandString(10)

			BeforeEach(func() {
				appendCommit(path, "file.txt", "Hello", "commit 1")

				targetRepoPath = filepath.Join(cfg.GetRepoRoot(), targetRepoName)
				execGit(cfg.GetRepoRoot(), "init", targetRepoName)
				appendCommit(targetRepoPath, "file.txt", "Hello World!", "commit 1")

				err = merge(repo, "master", targetRepoName+":master", repoRoot, false)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("no common commits"))
			})
		})

		When("target repo and branch exist but 'uncommitted' is false", func() {
			var targetRepoPath string
			var targetRepoName = util.RandString(10)

			BeforeEach(func() {
				appendCommit(path, "file.txt", "Hello", "commit 1")

				targetRepoPath = filepath.Join(cfg.GetRepoRoot(), targetRepoName)
				execGit(cfg.GetRepoRoot(), "clone", path, targetRepoName)
				createCheckoutBranch(targetRepoPath, "branch2")
				writeCommit(targetRepoPath, "file.txt", "Hello World!", "commit 1")

				err = merge(repo, "master", targetRepoName+":branch2", repoRoot, false)
				Expect(err).To(BeNil())
			})

			Specify("that base branch be updated with content of target", func() {
				checkoutBranch(path, "master")
				content, _ := scriptFile(path, "file.txt").String()
				Expect(content).To(Equal("Hello World!"))
			})
		})

		When("target repo and branch exist but 'uncommitted' is true", func() {
			var targetRepoPath string
			var targetRepoName = util.RandString(10)

			BeforeEach(func() {
				appendCommit(path, "file.txt", "Hello", "commit 1")

				targetRepoPath = filepath.Join(cfg.GetRepoRoot(), targetRepoName)
				execGit(cfg.GetRepoRoot(), "clone", path, targetRepoName)
				createCheckoutBranch(targetRepoPath, "branch2")
				writeCommit(targetRepoPath, "file.txt", "Hello World!", "commit 1")

				err = merge(repo, "master", targetRepoName+":branch2", repoRoot, true)
				Expect(err).To(BeNil())
			})

			Specify("that base branch is not updated with content of target", func() {
				checkoutBranch(path, "master")
				content, _ := scriptFile(path, "file.txt").String()
				Expect(content).To(Equal("Hello"))
			})
		})
	})
})
