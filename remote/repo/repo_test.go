package repo_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	repo2 "gitlab.com/makeos/mosdef/remote/repo"
	"gitlab.com/makeos/mosdef/types/core"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"

	"gitlab.com/makeos/mosdef/config"
	testutil2 "gitlab.com/makeos/mosdef/remote/testutil"
	"gitlab.com/makeos/mosdef/testutil"
	"gitlab.com/makeos/mosdef/util"
)

var _ = Describe("Repo", func() {
	var err error
	var cfg *config.AppConfig
	var path, dotGitPath string
	var repo core.BareRepo

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
	})

	BeforeEach(func() {
		repoName := util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		dotGitPath = filepath.Join(path, ".git")
		testutil2.ExecGit(cfg.GetRepoRoot(), "init", repoName)
		repo, err = repo2.GetRepoWithLiteGit(cfg.Node.GitBinPath, path)
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".ObjectExist", func() {
		It("should return true when object exist", func() {
			hash := testutil2.CreateBlob(path, "hello world")
			Expect(repo.ObjectExist(hash)).To(BeTrue())
		})

		It("should return false when object does not exist", func() {
			hash := strings.Repeat("0", 40)
			Expect(repo.ObjectExist(hash)).To(BeFalse())
		})
	})

	Describe(".Prune", func() {
		It("should remove unreachable objects", func() {
			hash := testutil2.CreateBlob(path, "hello world")
			Expect(repo.ObjectExist(hash)).To(BeTrue())
			err := repo.Prune(time.Time{})
			Expect(err).To(BeNil())
			Expect(repo.ObjectExist(hash)).To(BeFalse())
		})
	})

	Describe(".GetObject", func() {
		It("should return object if it exists", func() {
			hash := testutil2.CreateBlob(path, "hello world")
			obj, err := repo.GetObject(hash)
			Expect(err).To(BeNil())
			Expect(obj).NotTo(BeNil())
			Expect(obj.ID().String()).To(Equal(hash))
		})

		It("should return error if object does not exists", func() {
			obj, err := repo.GetObject("e69de29bb2d1d6434b8b29ae775ad8c2e48c5391")
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(plumbing.ErrObjectNotFound))
			Expect(obj).To(BeNil())
		})
	})

	Describe(".NumIssueBranches", func() {
		It("should return 1 when only one issue branch exists", func() {
			testutil2.CreateCheckoutBranch(path, "issues/1")
			testutil2.AppendCommit(path, "body", "content", "added body")
			n, err := repo.NumIssueBranches()
			Expect(err).To(BeNil())
			Expect(n).To(Equal(1))
		})

		It("should return 2 when only one issue branch exists", func() {
			testutil2.CreateCheckoutBranch(path, "issues/1")
			testutil2.AppendCommit(path, "body", "content", "added body")
			testutil2.CreateCheckoutBranch(path, "issues/2")
			testutil2.AppendCommit(path, "body", "content", "added body")
			n, err := repo.NumIssueBranches()
			Expect(err).To(BeNil())
			Expect(n).To(Equal(2))
		})

		It("should return 0 when no issue branch exists", func() {
			n, err := repo.NumIssueBranches()
			Expect(err).To(BeNil())
			Expect(n).To(Equal(0))
		})
	})

	Describe(".GetEncodedObject", func() {
		It("should return object if it exists", func() {
			hash := testutil2.CreateBlob(path, "hello world")
			obj, err := repo.GetEncodedObject(hash)
			Expect(err).To(BeNil())
			Expect(obj).NotTo(BeNil())
			Expect(obj.Hash().String()).To(Equal(hash))
		})

		It("should return error if object does not exists", func() {
			obj, err := repo.GetEncodedObject("e69de29bb2d1d6434b8b29ae775ad8c2e48c5391")
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(plumbing.ErrObjectNotFound))
			Expect(obj).To(BeNil())
		})
	})

	Describe(".GetObjectSize", func() {
		It("should return size of content", func() {
			testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
			hash, err := repo.GetRecentCommitHash()
			Expect(err).To(BeNil())
			size, err := repo.GetObjectSize(hash)
			Expect(err).To(BeNil())
			Expect(size).ToNot(Equal(int64(0)))
		})
	})

	Describe(".GetObjectDiskSize", func() {
		BeforeEach(func() {
			repo.SetPath(dotGitPath)
		})

		It("should return size of content", func() {
			testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
			hash, err := repo.GetRecentCommitHash()
			Expect(err).To(BeNil())
			size, err := repo.GetObjectDiskSize(hash)
			Expect(err).To(BeNil())
			Expect(size).ToNot(Equal(int64(0)))
		})
	})

	Describe(".WriteObjectToFile", func() {
		hash := "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"

		BeforeEach(func() {
			repo.SetPath(dotGitPath)
			Expect(repo.ObjectExist(hash)).To(BeFalse())
		})

		It("should successfully write object", func() {
			content := []byte("hello world")
			err := repo.WriteObjectToFile(hash, content)
			Expect(err).To(BeNil())

			objPath := filepath.Join(dotGitPath, "objects", hash[:2], hash[2:])
			fi, err := os.Stat(objPath)
			Expect(err).To(BeNil())
			Expect(fi.Size()).To(Equal(int64(len(content))))
		})
	})

	Describe(".GetObjectsSize", func() {
		var objs []string
		var expectedSize = int64(0)
		BeforeEach(func() {
			testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
			it, _ := repo.Objects()
			it.ForEach(func(obj object.Object) error {
				size, _ := repo.GetObjectDiskSize(obj.ID().String())
				expectedSize += size
				return nil
			})
		})

		It("should return expected size", func() {
			size, err := repo2.GetObjectsSize(repo, objs)
			Expect(err).To(BeNil())
			Expect(size).To(Equal(uint64(expectedSize)))
		})
	})

	Describe(".GetTreeEntries", func() {
		var entries []string
		When("no directory exist in tree", func() {
			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
				ci, _ := repo.CommitObjects()
				commit, _ := ci.Next()
				entries, err = repo2.GetTreeEntries(repo, commit.TreeHash.String())
				Expect(err).To(BeNil())
			})

			It("should have 1 entry", func() {
				Expect(entries).To(HaveLen(1))
			})
		})

		When("one directory with one file exist in tree", func() {
			BeforeEach(func() {
				testutil2.AppendDirAndCommitFile(path, "my_dir", "file_x.txt", "some data", "commit 2")
				ci, _ := repo.CommitObjects()
				commit, _ := ci.Next()
				entries, err = repo2.GetTreeEntries(repo, commit.TreeHash.String())
				Expect(err).To(BeNil())
			})

			It("should have 2 entries (one tree and one blob)", func() {
				Expect(entries).To(HaveLen(2))
			})
		})
	})

	Describe(".GetCommitHistory", func() {
		When("there is a single commit", func() {
			var history []string

			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
				ci, _ := repo.CommitObjects()
				var commits []*object.Commit
				ci.ForEach(func(c *object.Commit) error {
					commits = append(commits, c)
					return nil
				})
				Expect(commits).To(HaveLen(1))
				history, err = repo2.GetCommitHistory(repo, commits[0], "")
				Expect(err).To(BeNil())
			})

			It("should have 3 history hashes (1 commit, 1 tree, 1 blob)", func() {
				Expect(history).To(HaveLen(3))
			})
		})

		When("there are two commits", func() {
			var history []string

			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
				testutil2.AppendCommit(path, "file2.txt", "some text 2", "commit msg 2")
				ci, _ := repo.CommitObjects()
				var commits []*object.Commit

				// order is not guaranteed
				ci.ForEach(func(c *object.Commit) error {
					commits = append(commits, c)
					return nil
				})
				Expect(commits).To(HaveLen(2))

				sort.Slice(commits, func(i, j int) bool {
					return commits[i].NumParents() < commits[j].NumParents()
				})

				history, err = repo2.GetCommitHistory(repo, commits[1], "")
				Expect(err).To(BeNil())
			})

			It("should have 6 history hashes (2 commits, 2 trees, 2 blobs)", func() {
				Expect(history).To(HaveLen(6))
			})
		})

		When("there are two commits and the stop commit is the target commit", func() {
			var history []string

			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
				testutil2.AppendCommit(path, "file2.txt", "some text 2", "commit msg 2")
				ci, _ := repo.CommitObjects()
				var commits []*object.Commit

				// order is not guaranteed
				ci.ForEach(func(c *object.Commit) error {
					commits = append(commits, c)
					return nil
				})

				sort.Slice(commits, func(i, j int) bool {
					return commits[i].NumParents() < commits[j].NumParents()
				})

				Expect(commits).To(HaveLen(2))
				history, err = repo2.GetCommitHistory(repo, commits[1], commits[1].Hash.String())
				Expect(err).To(BeNil())
			})

			It("should have 0 history hashes", func() {
				Expect(history).To(HaveLen(0))
			})
		})

		When("there are two commits and the stop commit is the first commit", func() {
			var history []string

			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
				testutil2.AppendCommit(path, "file.txt", "some text 2", "commit msg 2")
				ci, _ := repo.CommitObjects()
				var commits []*object.Commit

				// order is not guaranteed
				ci.ForEach(func(c *object.Commit) error {
					commits = append(commits, c)
					return nil
				})
				Expect(commits).To(HaveLen(2))

				sort.Slice(commits, func(i, j int) bool {
					return commits[i].NumParents() < commits[j].NumParents()
				})

				history, err = repo2.GetCommitHistory(repo, commits[1], commits[0].Hash.String())
				Expect(err).To(BeNil())
			})

			It("should have 3 history hashes", func() {
				Expect(history).To(HaveLen(3))
			})
		})
	})
})
