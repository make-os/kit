package plumbing_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/mocks"
	plumbing2 "github.com/make-os/kit/remote/plumbing"
	rr "github.com/make-os/kit/remote/repo"
	testutil2 "github.com/make-os/kit/remote/testutil"
	"github.com/make-os/kit/remote/types"
	"github.com/make-os/kit/testutil"
	types2 "github.com/make-os/kit/types"
	"github.com/make-os/kit/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

var _ = Describe("Traverse", func() {
	var err error
	var cfg *config.AppConfig
	var path string
	var testRepo types.LocalRepo
	var ctrl *gomock.Controller
	var mockRepo *mocks.MockLocalRepo

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"

		repoName := util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		testutil2.ExecGit(cfg.GetRepoRoot(), "init", repoName)
		testRepo, err = rr.GetWithLiteGit(cfg.Node.GitBinPath, path)
		Expect(err).To(BeNil())

		mockRepo = mocks.NewMockLocalRepo(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".GetTreeEntries", func() {
		var entries []string
		When("no directory exist in tree", func() {
			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
				ci, _ := testRepo.CommitObjects()
				commit, _ := ci.Next()
				entries, err = plumbing2.GetTreeEntries(testRepo, commit.TreeHash.String())
				Expect(err).To(BeNil())
			})

			It("should have 1 entry", func() {
				Expect(entries).To(HaveLen(1))
			})
		})

		When("one directory with one file exist in tree", func() {
			BeforeEach(func() {
				testutil2.AppendDirAndCommitFile(path, "my_dir", "file_x.txt", "some data", "commit 2")
				ci, _ := testRepo.CommitObjects()
				commit, _ := ci.Next()
				entries, err = plumbing2.GetTreeEntries(testRepo, commit.TreeHash.String())
				Expect(err).To(BeNil())
			})

			It("should have 2 entries (one tree and one blob)", func() {
				Expect(entries).To(HaveLen(2))
			})
		})
	})

	Describe(".WalkCommitHistory", func() {
		When("there is a single commit", func() {
			var history []string

			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
				ci, _ := testRepo.CommitObjects()
				var commits []*object.Commit
				ci.ForEach(func(c *object.Commit) error {
					commits = append(commits, c)
					return nil
				})
				Expect(commits).To(HaveLen(1))
				history, err = plumbing2.WalkCommitHistory(testRepo, commits[0], "")
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
				ci, _ := testRepo.CommitObjects()
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

				history, err = plumbing2.WalkCommitHistory(testRepo, commits[1], "")
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
				ci, _ := testRepo.CommitObjects()
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
				history, err = plumbing2.WalkCommitHistory(testRepo, commits[1], commits[1].Hash.String())
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
				ci, _ := testRepo.CommitObjects()
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

				history, err = plumbing2.WalkCommitHistory(testRepo, commits[1], commits[0].Hash.String())
				Expect(err).To(BeNil())
			})

			It("should have 3 history hashes", func() {
				Expect(history).To(HaveLen(3))
			})
		})
	})

	Describe(".WalkCommitHistoryWithIteratee", func() {
		It("should return error when unable to get stop object", func() {
			args := &plumbing2.WalkCommitHistoryArgs{StopHash: "stop_hash"}
			mockRepo.EXPECT().GetObject(args.StopHash).Return(nil, fmt.Errorf("error"))
			err := plumbing2.WalkCommitHistoryWithIteratee(mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		It("should return immediately when stop hash and commit hash match", func() {
			args := &plumbing2.WalkCommitHistoryArgs{
				StopHash: "2b2a31123b57e6f8d7e6b88c9f3f5ca4d0bb2475",
				Commit:   &object.Commit{Hash: plumbing.NewHash("2b2a31123b57e6f8d7e6b88c9f3f5ca4d0bb2475")},
			}
			mockRepo.EXPECT().GetObject(args.StopHash).Return(nil, nil)
			err := plumbing2.WalkCommitHistoryWithIteratee(mockRepo, args)
			Expect(err).To(BeNil())
		})

		It("should return immediately when the commit is an ancestor of the stop object", func() {
			testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
			commit1Hash := testutil2.GetRecentCommitHash(path, "master")
			testutil2.AppendCommit(path, "file.txt", "some text update", "commit msg")
			commit2 := testutil2.GetRecentCommitHash(path, "master")
			args := &plumbing2.WalkCommitHistoryArgs{
				StopHash: commit2,
				Commit:   &object.Commit{Hash: plumbing.NewHash(commit1Hash)},
			}
			err := plumbing2.WalkCommitHistoryWithIteratee(testRepo, args)
			Expect(err).To(BeNil())
		})

		When("no stop hash is set", func() {
			It("should set a noop callback if one is not provided", func() {
				testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
				testutil2.AppendCommit(path, "file.txt", "some text update", "commit msg")
				commit2Hash := testutil2.GetRecentCommitHash(path, "master")
				commit2, _ := testRepo.CommitObject(plumbing.NewHash(commit2Hash))
				args := &plumbing2.WalkCommitHistoryArgs{
					Commit: commit2,
				}
				err := plumbing2.WalkCommitHistoryWithIteratee(testRepo, args)
				Expect(err).To(BeNil())
				Expect(args.Res).ToNot(BeNil())
			})

			It("should pass objects hash to provided callback", func() {
				var found []string
				testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
				testutil2.AppendCommit(path, "file.txt", "some text update", "commit msg")
				commit2Hash := testutil2.GetRecentCommitHash(path, "master")
				commit2, _ := testRepo.CommitObject(plumbing.NewHash(commit2Hash))
				args := &plumbing2.WalkCommitHistoryArgs{
					Commit: commit2,
					Res: func(objHash string) error {
						found = append(found, objHash)
						return nil
					},
				}
				err := plumbing2.WalkCommitHistoryWithIteratee(testRepo, args)
				Expect(err).To(BeNil())
				Expect(found).To(HaveLen(6))
			})

			It("should stop walking and return error if result callback returns non-ErrExit error", func() {
				var found []string
				testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
				testutil2.AppendCommit(path, "file.txt", "some text update", "commit msg")
				commit2Hash := testutil2.GetRecentCommitHash(path, "master")
				commit2, _ := testRepo.CommitObject(plumbing.NewHash(commit2Hash))
				args := &plumbing2.WalkCommitHistoryArgs{
					Commit: commit2,
					Res: func(objHash string) error {
						found = append(found, objHash)
						return fmt.Errorf("error")
					},
				}
				err := plumbing2.WalkCommitHistoryWithIteratee(testRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("error"))
				Expect(found).To(HaveLen(1))
			})

			It("should stop walking and return nil if result callback returns ErrExit error", func() {
				var found []string
				testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
				testutil2.AppendCommit(path, "file.txt", "some text update", "commit msg")
				commit2Hash := testutil2.GetRecentCommitHash(path, "master")
				commit2, _ := testRepo.CommitObject(plumbing.NewHash(commit2Hash))
				args := &plumbing2.WalkCommitHistoryArgs{
					Commit: commit2,
					Res: func(objHash string) error {
						found = append(found, objHash)
						return types2.ErrExit
					},
				}
				err := plumbing2.WalkCommitHistoryWithIteratee(testRepo, args)
				Expect(err).To(BeNil())
				Expect(found).To(HaveLen(1))
			})
		})

		When(`there are 2 commits: 
[c1]-[c2]
- stop hash is [c1].
- start commit is [c2]`, func() {
			It("should return 3 objects (c1, c1's tree and tree entries)", func() {
				var found []string
				testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
				commit1Hash := testutil2.GetRecentCommitHash(path, "master")
				testutil2.AppendCommit(path, "file.txt", "some text update", "commit msg")
				commit2Hash := testutil2.GetRecentCommitHash(path, "master")
				commit2, _ := testRepo.CommitObject(plumbing.NewHash(commit2Hash))
				args := &plumbing2.WalkCommitHistoryArgs{
					Commit:   commit2,
					StopHash: commit1Hash,
					Res: func(objHash string) error {
						found = append(found, objHash)
						return nil
					},
				}
				err := plumbing2.WalkCommitHistoryWithIteratee(testRepo, args)
				Expect(err).To(BeNil())
				Expect(found).To(HaveLen(3))
				Expect(found).To(ContainElement(commit2.Hash.String()))
				Expect(found).To(ContainElement(commit2.TreeHash.String()))
				tree, _ := commit2.Tree()
				Expect(found).To(ContainElement(tree.Entries[0].Hash.String()))
			})
		})

		When(`there are 2 commits: 
[c1]-[c2]
- stop hash is [c2's tree].
- start commit is [c2]`, func() {
			It("should return 1 object (c1)", func() {
				var found []string
				testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
				testutil2.AppendCommit(path, "file.txt", "some text update", "commit msg")
				commit2Hash := testutil2.GetRecentCommitHash(path, "master")
				commit2, _ := testRepo.CommitObject(plumbing.NewHash(commit2Hash))
				args := &plumbing2.WalkCommitHistoryArgs{
					Commit:   commit2,
					StopHash: commit2.TreeHash.String(),
					Res: func(objHash string) error {
						found = append(found, objHash)
						return nil
					},
				}
				err := plumbing2.WalkCommitHistoryWithIteratee(testRepo, args)
				Expect(err).To(BeNil())
				Expect(found).To(HaveLen(1))
				Expect(found).To(ContainElement(commit2.Hash.String()))
			})
		})

		When(`there are 2 commits: 
[c1]-[c2]
- stop hash is [c2's tree entry].
- start commit is [c2]`, func() {
			It("should return 2 objects (c1, c1's tree)", func() {
				var found []string
				testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
				testutil2.AppendCommit(path, "file.txt", "some text update", "commit msg")
				commit2Hash := testutil2.GetRecentCommitHash(path, "master")
				commit2, _ := testRepo.CommitObject(plumbing.NewHash(commit2Hash))
				commit2Tree, _ := commit2.Tree()
				args := &plumbing2.WalkCommitHistoryArgs{
					Commit:   commit2,
					StopHash: commit2Tree.Entries[0].Hash.String(),
					Res: func(objHash string) error {
						found = append(found, objHash)
						return nil
					},
				}
				err := plumbing2.WalkCommitHistoryWithIteratee(testRepo, args)
				Expect(err).To(BeNil())
				Expect(found).To(HaveLen(2))
				Expect(found).To(ContainElement(commit2.Hash.String()))
				Expect(found).To(ContainElement(commit2Tree.Hash.String()))
			})
		})

		When(`there are 4 commits and 2 branches: 
[c1]-[c2]---[c3]
  \--[c4]----/
- [c3] is a merge commit with [c2] and [c4] as parents.
- stop hash is [c2].
- start commit is [c3]`, func() {
			var c2Hash, c3Hash, c4Hash string
			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
				testutil2.CreateCheckoutBranch(path, "dev")
				testutil2.AppendCommit(path, "file.txt", "log some good text", "commit msg")
				c4Hash = testutil2.GetRecentCommitHash(path, "dev")
				testutil2.CheckoutBranch(path, "master")
				testutil2.AppendCommit(path, "file.txt", "intro to \n****some nice text", "commit msg")
				c2Hash = testutil2.GetRecentCommitHash(path, "master")
				testutil2.ForceMergeOurs(path, "dev")
				c3Hash = testutil2.GetRecentCommitHash(path, "master")
			})

			It("should return 6 objects (c3, c3's tree, tree entries) and (c4, c4's tree, tree entries)", func() {
				var found []string
				commit3, _ := testRepo.CommitObject(plumbing.NewHash(c3Hash))
				args := &plumbing2.WalkCommitHistoryArgs{
					Commit:   commit3,
					StopHash: c2Hash,
					Res: func(objHash string) error {
						found = append(found, objHash)
						return nil
					},
				}
				err := plumbing2.WalkCommitHistoryWithIteratee(testRepo, args)
				Expect(err).To(BeNil())
				Expect(found).To(HaveLen(6))

				Expect(found).To(ContainElement(commit3.Hash.String()))
				Expect(found).To(ContainElement(commit3.TreeHash.String()))
				tree, _ := commit3.Tree()
				for _, entry := range tree.Entries {
					Expect(found).To(ContainElement(entry.Hash.String()))
				}

				commit4, _ := testRepo.CommitObject(plumbing.NewHash(c4Hash))
				Expect(found).To(ContainElement(commit4.Hash.String()))
				Expect(found).To(ContainElement(commit4.TreeHash.String()))
				tree, _ = commit3.Tree()
				for _, entry := range tree.Entries {
					Expect(found).To(ContainElement(entry.Hash.String()))
				}
			})
		})
	})

	Describe(".WalkBack", func() {
		startHash := "e070e3147d617e026e6ac08f1aac9ca3d0ae561a"
		It("should return error when unable to get start object", func() {
			resCB := func(obj string) error { return nil }
			mockRepo.EXPECT().GetObject(startHash).Return(nil, fmt.Errorf("error"))
			err := plumbing2.WalkBack(mockRepo, startHash, "", resCB)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to get start object: error"))
		})

		When("start object is not a commit or a tag", func() {
			It("should immediately return start object (and its packable object) when start object is a blob", func() {
				testutil2.AppendCommit(path, "file.txt", "log some good text", "commit msg")
				hash := testutil2.GetRecentCommitHash(path, "master")
				commit, _ := testRepo.CommitObject(plumbing.NewHash(hash))
				tree, _ := commit.Tree()
				blobHash := tree.Entries[0].Hash.String()

				found := []string{}
				resCB := func(obj string) error {
					found = append(found, obj)
					return nil
				}
				err := plumbing2.WalkBack(testRepo, blobHash, "", resCB)
				Expect(err).To(BeNil())
				Expect(found).To(HaveLen(1))
				Expect(found).To(ContainElement(blobHash))
			})

			It("should immediately return start object (and its packable object) when start object is a tree", func() {
				testutil2.AppendCommit(path, "file.txt", "log some good text", "commit msg")
				hash := testutil2.GetRecentCommitHash(path, "master")
				commit, _ := testRepo.CommitObject(plumbing.NewHash(hash))
				tree, _ := commit.Tree()
				treeHash := commit.TreeHash.String()

				found := []string{}
				resCB := func(obj string) error {
					found = append(found, obj)
					return nil
				}
				err := plumbing2.WalkBack(testRepo, treeHash, "", resCB)
				Expect(err).To(BeNil())
				Expect(found).To(HaveLen(2))
				Expect(found).To(ContainElement(treeHash))
				Expect(found).To(ContainElement(tree.Entries[0].Hash.String()))
			})
		})

		When("start object is a tag not pointed to a commit or tag but a blob", func() {
			It("should immediately return the tag and its target (blob)", func() {
				testutil2.AppendCommit(path, "file.txt", "log some good text", "commit msg")
				hash := testutil2.GetRecentCommitHash(path, "master")
				commit, _ := testRepo.CommitObject(plumbing.NewHash(hash))
				tree, _ := commit.Tree()
				blobHash := tree.Entries[0].Hash.String()
				testutil2.CreateTagPointedToTag(path, "tag msg", "tag1", blobHash)
				tag, _ := testRepo.Tag("tag1")
				tagHash := tag.Hash().String()

				found := []string{}
				resCB := func(obj string) error {
					found = append(found, obj)
					return nil
				}
				err := plumbing2.WalkBack(testRepo, tagHash, "", resCB)
				Expect(err).To(BeNil())
				Expect(found).To(HaveLen(2))
				Expect(found).To(ContainElement(tagHash))
				Expect(found).To(ContainElement(blobHash))
			})
		})

		When("start object is a tag pointed to a tag that points to a blob", func() {
			It("should immediately return the tag and its target (blob)", func() {
				testutil2.AppendCommit(path, "file.txt", "log some good text", "commit msg")
				hash := testutil2.GetRecentCommitHash(path, "master")
				commit, _ := testRepo.CommitObject(plumbing.NewHash(hash))
				tree, _ := commit.Tree()
				blobHash := tree.Entries[0].Hash.String()
				testutil2.CreateTagPointedToTag(path, "tag msg", "tag1", blobHash)
				tag, _ := testRepo.Tag("tag1")
				tagHash := tag.Hash().String()
				testutil2.CreateTagPointedToTag(path, "tag msg2", "tag2", tag.Hash().String())
				tag2, _ := testRepo.Tag("tag2")
				tag2Hash := tag2.Hash().String()

				found := []string{}
				resCB := func(obj string) error {
					found = append(found, obj)
					return nil
				}
				err := plumbing2.WalkBack(testRepo, tag2Hash, "", resCB)
				Expect(err).To(BeNil())
				Expect(found).To(HaveLen(3))
				Expect(found).To(ContainElement(tagHash))
				Expect(found).To(ContainElement(tag2Hash))
				Expect(found).To(ContainElement(blobHash))
			})
		})

		When("start object is a commit", func() {
			It("should immediately return the commit and its ancestors and their related objects", func() {
				testutil2.AppendCommit(path, "file.txt", "log some good text", "commit msg")
				commit1Hash := testutil2.GetRecentCommitHash(path, "master")
				testutil2.AppendCommit(path, "file.txt", "log some good text", "commit msg")
				commit2 := testutil2.GetRecentCommitHash(path, "master")

				found := []string{}
				resCB := func(obj string) error {
					found = append(found, obj)
					return nil
				}
				err := plumbing2.WalkBack(testRepo, commit2, "", resCB)
				Expect(err).To(BeNil())
				Expect(found).To(HaveLen(6))
				Expect(found).To(ContainElement(commit1Hash))
				Expect(found).To(ContainElement(commit2))
			})
		})

		When("end object is not a commit or tag", func() {
			It("should immediately return the commit and its ancestors and their related objects", func() {
				testutil2.AppendCommit(path, "file.txt", "log some good text", "commit msg")
				commit1Hash := testutil2.GetRecentCommitHash(path, "master")
				testutil2.AppendCommit(path, "file.txt", "log some good text", "commit msg")
				commit2 := testutil2.GetRecentCommitHash(path, "master")

				commit, _ := testRepo.CommitObject(plumbing.NewHash(commit1Hash))
				treeHash := commit.TreeHash.String()

				err := plumbing2.WalkBack(testRepo, commit2, treeHash, nil)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("end object must be a tag or a commit"))
			})
		})

		When("end object is a tag pointing to a non-commit/tag object", func() {
			It("should immediately return the commit and its ancestors and their related objects", func() {
				testutil2.AppendCommit(path, "file.txt", "log some good text", "commit msg")
				commit1Hash := testutil2.GetRecentCommitHash(path, "master")
				testutil2.AppendCommit(path, "file.txt", "log some good text", "commit msg")
				commit2 := testutil2.GetRecentCommitHash(path, "master")

				commit, _ := testRepo.CommitObject(plumbing.NewHash(commit1Hash))
				treeHash := commit.TreeHash.String()
				testutil2.CreateTagPointedToTag(path, "tag msg", "tag1", treeHash)
				tag, _ := testRepo.Tag("tag1")
				tagHash := tag.Hash().String()

				err := plumbing2.WalkBack(testRepo, commit2, tagHash, nil)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("end object must be a tag or a commit"))
			})
		})

		When("end object is a tag pointing to a commit object", func() {
			It("should immediately return the commit and its ancestors and their related objects", func() {
				testutil2.AppendCommit(path, "file.txt", "log some good text", "commit msg")
				commit1Hash := testutil2.GetRecentCommitHash(path, "master")
				testutil2.AppendCommit(path, "file.txt", "log some good text", "commit msg")
				commit2Hash := testutil2.GetRecentCommitHash(path, "master")

				testutil2.CreateTagPointedToTag(path, "tag msg", "tag1", commit1Hash)
				tag, _ := testRepo.Tag("tag1")
				tagHash := tag.Hash().String()

				found := []string{}
				resCB := func(obj string) error {
					found = append(found, obj)
					return nil
				}
				err := plumbing2.WalkBack(testRepo, commit2Hash, tagHash, resCB)
				Expect(err).To(BeNil())
				Expect(found).To(HaveLen(3))
				Expect(found).To(ContainElement(commit2Hash))

				commit2, _ := testRepo.CommitObject(plumbing.NewHash(commit2Hash))
				Expect(found).To(ContainElement(commit2.TreeHash.String()))
				tree, _ := commit2.Tree()
				Expect(found).To(ContainElement(tree.Entries[0].Hash.String()))
			})
		})

		When("end hash is zero hash", func() {
			It("should consider end hash unset and return all commits and related objects", func() {
				testutil2.AppendCommit(path, "file.txt", "log some good text", "commit msg")
				testutil2.AppendCommit(path, "file.txt", "log some good text", "commit msg")
				commit2Hash := testutil2.GetRecentCommitHash(path, "master")
				found := []string{}
				resCB := func(obj string) error {
					found = append(found, obj)
					return nil
				}
				err := plumbing2.WalkBack(testRepo, commit2Hash, plumbing.ZeroHash.String(), resCB)
				Expect(err).To(BeNil())
				Expect(found).To(HaveLen(6))
			})
		})

		When("result callback is unset", func() {
			It("should not cause an error", func() {
				testutil2.AppendCommit(path, "file.txt", "log some good text", "commit msg")
				testutil2.AppendCommit(path, "file.txt", "log some good text", "commit msg")
				commit2Hash := testutil2.GetRecentCommitHash(path, "master")
				err := plumbing2.WalkBack(testRepo, commit2Hash, plumbing.ZeroHash.String(), nil)
				Expect(err).To(BeNil())
			})
		})
	})
})
