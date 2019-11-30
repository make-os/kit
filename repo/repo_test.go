package repo

import (
	"os"
	"path/filepath"
	"sort"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/testutil"
	"github.com/makeos/mosdef/util"
)

var _ = Describe("Gitops", func() {
	var err error
	var cfg *config.EngineConfig
	var path string
	var repo *Repo

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
	})

	BeforeEach(func() {
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

	Describe(".getObjectsSize", func() {
		var objs []string
		var expectedSize = int64(0)
		BeforeEach(func() {
			appendCommit(path, "file.txt", "some text", "commit msg")
			it, _ := repo.Objects()
			it.ForEach(func(obj object.Object) error {
				objs = append(objs, obj.ID().String())
				encoded := &plumbing.MemoryObject{}
				obj.Encode(encoded)
				expectedSize += encoded.Size()
				return nil
			})
		})

		It("should return expected size", func() {
			size := getObjectsSize(repo, objs)
			Expect(size).To(Equal(uint64(expectedSize)))
		})
	})

	Describe(".getTreeEntries", func() {
		var entries []string
		When("no directory exist in tree", func() {
			BeforeEach(func() {
				appendCommit(path, "file.txt", "some text", "commit msg")
				ci, _ := repo.CommitObjects()
				commit, _ := ci.Next()
				entries, err = getTreeEntries(repo, commit.TreeHash.String())
				Expect(err).To(BeNil())
			})

			It("should have 1 entry", func() {
				Expect(entries).To(HaveLen(1))
			})
		})

		When("one directory with one file exist in tree", func() {
			BeforeEach(func() {
				appendDirAndCommitFile(path, "my_dir", "file_x.txt", "some data", "commit 2")
				ci, _ := repo.CommitObjects()
				commit, _ := ci.Next()
				entries, err = getTreeEntries(repo, commit.TreeHash.String())
				Expect(err).To(BeNil())
			})

			It("should have 2 entries (one tree and one blob)", func() {
				Expect(entries).To(HaveLen(2))
			})
		})
	})

	Describe(".getCommitHistory", func() {
		When("there is a single commit", func() {
			var history []string

			BeforeEach(func() {
				appendCommit(path, "file.txt", "some text", "commit msg")
				ci, _ := repo.CommitObjects()
				commits := []*object.Commit{}
				ci.ForEach(func(c *object.Commit) error {
					commits = append(commits, c)
					return nil
				})
				Expect(commits).To(HaveLen(1))
				history, err = getCommitHistory(repo, commits[0], "")
				Expect(err).To(BeNil())
			})

			It("should have 3 history hashes (1 commit, 1 tree, 1 blob)", func() {
				Expect(history).To(HaveLen(3))
			})
		})

		When("there are two commits", func() {
			var history []string

			BeforeEach(func() {
				appendCommit(path, "file.txt", "some text", "commit msg")
				appendCommit(path, "file2.txt", "some text 2", "commit msg 2")
				ci, _ := repo.CommitObjects()
				commits := []*object.Commit{}

				// order is not guaranteed
				ci.ForEach(func(c *object.Commit) error {
					commits = append(commits, c)
					return nil
				})
				Expect(commits).To(HaveLen(2))

				sort.Slice(commits, func(i, j int) bool {
					return commits[i].NumParents() < commits[j].NumParents()
				})

				history, err = getCommitHistory(repo, commits[1], "")
				Expect(err).To(BeNil())
			})

			It("should have 6 history hashes (2 commits, 2 trees, 2 blobs)", func() {
				Expect(history).To(HaveLen(6))
			})
		})

		When("there are two commits and the stop commit is the target commit", func() {
			var history []string

			BeforeEach(func() {
				appendCommit(path, "file.txt", "some text", "commit msg")
				appendCommit(path, "file2.txt", "some text 2", "commit msg 2")
				ci, _ := repo.CommitObjects()
				commits := []*object.Commit{}

				// order is not guaranteed
				ci.ForEach(func(c *object.Commit) error {
					commits = append(commits, c)
					return nil
				})

				sort.Slice(commits, func(i, j int) bool {
					return commits[i].NumParents() < commits[j].NumParents()
				})

				Expect(commits).To(HaveLen(2))
				history, err = getCommitHistory(repo, commits[1], commits[1].Hash.String())
				Expect(err).To(BeNil())
			})

			It("should have 0 history hashes", func() {
				Expect(history).To(HaveLen(0))
			})
		})

		When("there are two commits and the stop commit is the first commit", func() {
			var history []string

			BeforeEach(func() {
				appendCommit(path, "file.txt", "some text", "commit msg")
				appendCommit(path, "file.txt", "some text 2", "commit msg 2")
				ci, _ := repo.CommitObjects()
				commits := []*object.Commit{}

				// order is not guaranteed
				ci.ForEach(func(c *object.Commit) error {
					commits = append(commits, c)
					return nil
				})
				Expect(commits).To(HaveLen(2))

				sort.Slice(commits, func(i, j int) bool {
					return commits[i].NumParents() < commits[j].NumParents()
				})

				history, err = getCommitHistory(repo, commits[1], commits[0].Hash.String())
				Expect(err).To(BeNil())
			})

			It("should have 3 history hashes", func() {
				Expect(history).To(HaveLen(3))
			})
		})
	})
})
