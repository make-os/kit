package repo_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gitlab.com/makeos/mosdef/crypto"
	plumbing2 "gitlab.com/makeos/mosdef/remote/plumbing"
	repo2 "gitlab.com/makeos/mosdef/remote/repo"
	"gitlab.com/makeos/mosdef/types/core"
	state2 "gitlab.com/makeos/mosdef/types/state"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"

	"gitlab.com/makeos/mosdef/config"
	testutil2 "gitlab.com/makeos/mosdef/remote/testutil"
	"gitlab.com/makeos/mosdef/testutil"
	"gitlab.com/makeos/mosdef/util"
)

var _ = Describe("RepoContext", func() {
	var err error
	var cfg *config.AppConfig
	var path, dotGitPath string
	var repo core.LocalRepo
	var key *crypto.Key

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
		key = crypto.NewKeyFromIntSeed(1)
	})

	BeforeEach(func() {
		repoName := util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		dotGitPath = filepath.Join(path, ".git")
		testutil2.ExecGit(cfg.GetRepoRoot(), "init", repoName)
		repo, err = repo2.GetWithLiteGit(cfg.Node.GitBinPath, path)
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

	Describe(".GetReferences", func() {
		It("should return all references", func() {
			testutil2.AppendCommit(path, "body", "content", "m1")
			testutil2.CreateCheckoutBranch(path, "issues/1")
			testutil2.AppendCommit(path, "body", "content", "m2")
			refs, err := repo.GetReferences()
			Expect(err).To(BeNil())
			Expect(refs).To(HaveLen(3))
			Expect(refs[0].String()).To(Equal("refs/heads/issues/1"))
			Expect(refs[1].String()).To(Equal("refs/heads/master"))
			Expect(refs[2].String()).To(Equal("HEAD"))
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

	Describe(".IsContributor", func() {
		It("should return true when push key is a repo contributor", func() {
			repo.(*repo2.Repo).State = &state2.Repository{Contributors: map[string]*state2.RepoContributor{key.PushAddr().String(): {}}}
			Expect(repo.IsContributor(key.PushAddr().String())).To(BeTrue())
		})

		It("should return true when push key is a namespace contributor", func() {
			repo.(*repo2.Repo).Namespace = &state2.Namespace{Contributors: map[string]*state2.BaseContributor{key.PushAddr().String(): {}}}
			Expect(repo.IsContributor(key.PushAddr().String())).To(BeTrue())
		})

		It("should return false when push key is a namespace or repo contributor", func() {
			Expect(repo.IsContributor(key.PushAddr().String())).To(BeFalse())
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

	Describe(".GetFreeIssueNum", func() {
		It("should return start point if free", func() {
			n, err := repo.GetFreeIssueNum(1)
			Expect(err).To(BeNil())
			Expect(n).To(Equal(1))
		})

		It("should return next monotonic number if start number is not free", func() {
			start := 1
			testutil2.CreateCheckoutBranch(path, fmt.Sprintf("%s/%d", plumbing2.IssueBranchPrefix, start))
			testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
			n, err := repo.GetFreeIssueNum(start)
			Expect(err).To(BeNil())
			Expect(n).To(Equal(2))
		})
	})

	Describe(".GetAncestors", func() {
		var recentHash, c1, c2 string

		BeforeEach(func() {
			testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
			c1 = testutil2.GetRecentCommitHash(path, "refs/heads/master")
			testutil2.AppendCommit(path, "file.txt", "some text 2", "commit msg 2")
			c2 = testutil2.GetRecentCommitHash(path, "refs/heads/master")
			testutil2.AppendCommit(path, "file.txt", "some text 3", "commit msg 2")
			recentHash = testutil2.GetRecentCommitHash(path, "refs/heads/master")
		})

		It("should return two hashes (c2, c1)", func() {
			commit, err := repo.CommitObject(plumbing.NewHash(recentHash))
			Expect(err).To(BeNil())
			ancestors, err := repo.GetAncestors(commit, "", false)
			Expect(err).To(BeNil())
			Expect(ancestors).To(HaveLen(2))
			Expect(ancestors[0].Hash.String()).To(Equal(c2))
			Expect(ancestors[0].Hash.String()).To(Equal(c2))
			Expect(ancestors[1].Hash.String()).To(Equal(c1))
			Expect(ancestors[1].Hash.String()).To(Equal(c1))
		})

		It("should return two hashes (c1, c2) when reverse=true", func() {
			commit, err := repo.CommitObject(plumbing.NewHash(recentHash))
			Expect(err).To(BeNil())
			ancestors, err := repo.GetAncestors(commit, "", true)
			Expect(err).To(BeNil())
			Expect(ancestors).To(HaveLen(2))
			Expect(ancestors[0].Hash.String()).To(Equal(c1))
			Expect(ancestors[0].Hash.String()).To(Equal(c1))
			Expect(ancestors[1].Hash.String()).To(Equal(c2))
			Expect(ancestors[1].Hash.String()).To(Equal(c2))
		})

		It("should return 1 hashes (c2) when stopHash=c1", func() {
			commit, err := repo.CommitObject(plumbing.NewHash(recentHash))
			Expect(err).To(BeNil())
			ancestors, err := repo.GetAncestors(commit, c1, false)
			Expect(err).To(BeNil())
			Expect(ancestors).To(HaveLen(1))
			Expect(ancestors[0].Hash.String()).To(Equal(c2))
			Expect(ancestors[0].Hash.String()).To(Equal(c2))
		})
	})
})
