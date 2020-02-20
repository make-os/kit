package repo

import (
	"fmt"
	"gitlab.com/makeos/mosdef/repo/types/core"
	"os"
	"path/filepath"

	"github.com/bitfield/script"
	"github.com/golang/mock/gomock"
	"github.com/phayes/freeport"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/testutil"
	"gitlab.com/makeos/mosdef/types/mocks"
	"gitlab.com/makeos/mosdef/util"
)

var gitPath = "/usr/bin/git"

var _ = Describe("Revert", func() {
	var err error
	var cfg *config.AppConfig
	var repoMgr *Manager
	var repo core.BareRepo
	var path string
	var ctrl *gomock.Controller
	var mockLogic *testutil.MockObjects
	var mockDHT *mocks.MockDHT
	var mockMempool *mocks.MockMempool
	var mockBlockGetter *mocks.MockBlockGetter

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"

		repoName := util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		execGit(cfg.GetRepoRoot(), "init", repoName)

		ctrl = gomock.NewController(GinkgoT())
		mockLogic = testutil.MockLogic(ctrl)
		port, _ := freeport.GetFreePort()
		mockDHT = mocks.NewMockDHT(ctrl)
		mockMempool = mocks.NewMockMempool(ctrl)
		mockBlockGetter = mocks.NewMockBlockGetter(ctrl)

		repoMgr = NewManager(cfg, fmt.Sprintf(":%d", port), mockLogic.Logic, mockDHT,
			mockMempool, mockBlockGetter)
		repo, err = getRepoWithGitOpt(cfg.Node.GitBinPath, path)
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".revert (head references)", func() {
		var prevState core.BareRepoState

		When("a repo has 1 ref and 4 commits; revert the 4th commit", func() {

			BeforeEach(func() {
				// update file.txt in 3 commits
				appendCommit(path, "file.txt", "line 1\n", "commit 1")
				appendCommit(path, "file.txt", "line 2\n", "commit 2")
				appendCommit(path, "file.txt", "line 3\n", "commit 3")

				prevState, _ = repoMgr.GetRepoState(repo)

				// update the file.txt in a new commit
				appendCommit(path, "file.txt", "line 4\n", "commit 4")
				lastLine, _ := scriptFile(path, "file.txt").Last(1).String()
				Expect(lastLine).To(Equal("line 4\n"))
			})

			Specify("that current state equal previous state", func() {
				_, err := revert(repo, prevState)
				Expect(err).To(BeNil())
				curState, _ := repoMgr.GetRepoState(repo)
				Expect(curState.GetReferences()).To(Equal(prevState.GetReferences()))
			})
		})

		When("a repo has 1 ref and 4 commits; revert the 3th & 4th commits", func() {

			BeforeEach(func() {
				appendCommit(path, "file.txt", "line 1\n", "commit 1")
				appendCommit(path, "file.txt", "line 2\n", "commit 2")
				prevState, _ = repoMgr.GetRepoState(repo)
				appendCommit(path, "file.txt", "line 3\n", "commit 3")

				// update the file.txt in a new commit
				appendCommit(path, "file.txt", "line 4\n", "commit 4")
				lastLine, _ := scriptFile(path, "file.txt").Last(1).String()
				Expect(lastLine).To(Equal("line 4\n"))
			})

			Specify("that current state equal previous state", func() {
				_, err := revert(repo, prevState)
				Expect(err).To(BeNil())
				curState, _ := repoMgr.GetRepoState(repo)
				Expect(curState.GetReferences()).To(Equal(prevState.GetReferences()))
			})
		})

		When("a repo has 1 ref and 2 commits; prev state has matching ref but with unknown commit hash", func() {

			BeforeEach(func() {
				appendCommit(path, "file.txt", "line 1\n", "commit 1")
				appendCommit(path, "file.txt", "line 2\n", "commit 2")
				prevState = &State{
					References: NewObjCol(map[string]core.Item{
						"refs/heads/master": &Obj{
							Type: "ref",
							Name: "refs/heads/master",
							Data: "6ac8e9cf08409c169a12b526b20488d549106d69",
						},
					}),
				}
			})

			It("should return err='exec failed: hard reset failed'", func() {
				_, err := revert(repo, prevState)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("exec failed: reference update failed"))
			})
		})

		// This test case test how to handle reverting a new repo state update that
		// adds a new reference that isn't present on an older state
		When("a older state has 1 ref with 1 commit and new state has 2 ref (1 new ref) with 1 commit", func() {

			BeforeEach(func() {
				appendCommit(path, "file.txt", "line 1", "commit 1")
				prevState, _ = repoMgr.GetRepoState(repo)
				createCheckoutBranch(path, "branch2")
				appendCommit(path, "file.txt", "line 2", "commit 2")
			})

			It("should return err=nil and only 1 reference should exist and current state equal previous state", func() {
				_, err := revert(repo, prevState)
				Expect(err).To(BeNil())
				numRefs, _ := script.ExecInDir("git show-ref --heads", path).CountLines()
				Expect(numRefs).To(Equal(1))
				curState, _ := repoMgr.GetRepoState(repo)
				Expect(curState.GetReferences()).To(Equal(prevState.GetReferences()))
			})
		})

		// This test case tests how to handle reverting a new repo state update that
		// does not include references that are present on an older state
		When("a older state has 2 refs (master, branch2) with 1 commit in each | new state has 1 ref (master) with 1 commit", func() {

			BeforeEach(func() {
				appendCommit(path, "file.txt", "line 1", "commit 1 of master branch")
				createCheckoutBranch(path, "branch2")
				appendCommit(path, "file.txt", "line 1", "commit 1 of branch 2")
				prevState, _ = repoMgr.GetRepoState(repo)
				execGit(path, "update-ref", "-d", "refs/heads/branch2")
			})

			It("should return err=nil and only 2 references should exist and current state equal previous state", func() {
				_, err := revert(repo, prevState)
				Expect(err).To(BeNil())
				numRefs, _ := script.ExecInDir("git show-ref --heads", path).CountLines()
				Expect(numRefs).To(Equal(2))
				curState, _ := repoMgr.GetRepoState(repo)
				Expect(curState).To(Equal(prevState))
			})
		})

		// This test case tests how to handle reverting a new repo state update that
		// does not include references that are present on an older state
		When("a older state has 4 refs (master, branch2, branch3, branch4) with 1 commit in each | new state has 1 ref (master) with 1 commit", func() {

			BeforeEach(func() {
				appendCommit(path, "file.txt", "line 1", "commit 1 of master branch")
				createCheckoutBranch(path, "branch2")
				appendCommit(path, "file.txt", "line 1", "commit 1 of branch 2")
				createCheckoutBranch(path, "branch3")
				appendCommit(path, "file.txt", "line 1", "commit 1 of branch 3")
				createCheckoutBranch(path, "branch4")
				appendCommit(path, "file.txt", "line 1", "commit 1 of branch 4")
				prevState, _ = repoMgr.GetRepoState(repo)
				execGit(path, "update-ref", "-d", "refs/heads/branch2")
				execGit(path, "update-ref", "-d", "refs/heads/branch3")
				execGit(path, "update-ref", "-d", "refs/heads/branch4")
			})

			It("should return err=nil and only 2 references should exist and current state equal previous state", func() {
				_, err := revert(repo, prevState)
				Expect(err).To(BeNil())
				numRefs, _ := script.ExecInDir("git show-ref --heads", path).CountLines()
				Expect(numRefs).To(Equal(4))
				curState, _ := repoMgr.GetRepoState(repo)
				Expect(curState).To(Equal(prevState))
			})
		})
	})

	Describe(".revert (annotated tags)", func() {
		var path string
		var prevState core.BareRepoState

		BeforeEach(func() {
			repoName := util.RandString(5)
			path = filepath.Join(cfg.GetRepoRoot(), repoName)
			execGit(cfg.GetRepoRoot(), "init", repoName)
		})

		When("repo old state has 0 tags; new state has 1 tag", func() {

			BeforeEach(func() {
				prevState, _ = repoMgr.GetRepoState(repo)
				Expect(prevState.IsEmpty()).To(BeTrue())
				createCommitAndAnnotatedTag(path, "file.txt", "v1 file", "v1 commit", "v1")
			})

			It("should remove the new tag and old state should equal current state", func() {
				_, err := revert(repo, prevState)
				Expect(err).To(BeNil())
				curState, _ := repoMgr.GetRepoState(repo)
				Expect(curState.GetReferences()).To(Equal(prevState.GetReferences()))
			})
		})

		When("repo old state has 1 tags; new state has 3 tag", func() {

			BeforeEach(func() {
				createCommitAndAnnotatedTag(path, "file.txt", "first file", "first commit", "v1")
				prevState, _ = repoMgr.GetRepoState(repo)
				createCommitAndAnnotatedTag(path, "file.txt", "first file", "commit 2", "v2")
				createCommitAndAnnotatedTag(path, "file.txt", "first file", "commit 3", "v3")
				createCommitAndAnnotatedTag(path, "file.txt", "first file", "commit 4", "v4")
			})

			It("should remove the new tags and old state should equal current state", func() {
				_, err := revert(repo, prevState)
				Expect(err).To(BeNil())
				curState, _ := repoMgr.GetRepoState(repo)
				Expect(curState.GetReferences()).To(Equal(prevState.GetReferences()))
			})
		})

		When("repo old state has 1 annotated tag (v1); new state has same 1 annotated tag (v1) but with different value", func() {

			BeforeEach(func() {
				createCommitAndAnnotatedTag(path, "file.txt", "first file", "first commit", "v1")
				prevState, _ = repoMgr.GetRepoState(repo)
				createCommitAndAnnotatedTag(path, "file.txt", "updated file", "second commit", "v1")
			})

			It("should update the reference value of the tag to the old value and old state should equal current state", func() {
				_, err := revert(repo, prevState)
				Expect(err).To(BeNil())
				curState, _ := repoMgr.GetRepoState(repo)
				Expect(curState.GetReferences()).To(Equal(prevState.GetReferences()))
			})
		})

		When("repo old state has 1 lightweight tag (v1); new state has same 1 lightweight tag (v1) but with different value", func() {

			BeforeEach(func() {
				createCommitAndLightWeightTag(path, "file.txt", "first file", "first commit", "v1")
				prevState, _ = repoMgr.GetRepoState(repo)
				createCommitAndLightWeightTag(path, "file.txt", "updated file", "second commit", "v1")
			})

			It("should update the reference value of the tag to the old value and old state should equal current state", func() {
				_, err := revert(repo, prevState)
				Expect(err).To(BeNil())
				curState, _ := repoMgr.GetRepoState(repo)
				Expect(curState.GetReferences()).To(Equal(prevState.GetReferences()))
			})
		})

		When("repo old state has 2 annotated tag (v1,v2); new state has same 2 annotated tag (v1,v2) but with different value", func() {

			BeforeEach(func() {
				createCommitAndAnnotatedTag(path, "file.txt", "file1", "first commit", "v1")
				createCommitAndAnnotatedTag(path, "file.txt", "file2", "second commit", "v2")
				prevState, _ = repoMgr.GetRepoState(repo)
				createCommitAndAnnotatedTag(path, "file.txt", "file3", "third commit", "v1")
				createCommitAndAnnotatedTag(path, "file.txt", "file4", "fourth commit", "v2")
			})

			It("should update the reference value of the tags to their old value and old state should equal current state", func() {
				_, err := revert(repo, prevState)
				Expect(err).To(BeNil())
				curState, _ := repoMgr.GetRepoState(repo)
				Expect(curState.GetReferences()).To(Equal(prevState.GetReferences()))
			})
		})

		When("repo old state has 1 annotated tag (v1); new state has 0 annotated tag (v1)", func() {

			BeforeEach(func() {
				createCommitAndAnnotatedTag(path, "file.txt", "first file", "first commit", "v1")
				prevState, _ = repoMgr.GetRepoState(repo)
				deleteTag(path, "v1")
			})

			It("should reset the tag value to the old tag value and old state should equal current state", func() {
				_, err := revert(repo, prevState)
				Expect(err).To(BeNil())
				curState, _ := repoMgr.GetRepoState(repo)
				Expect(curState.GetReferences()).To(Equal(prevState.GetReferences()))
			})
		})
	})

	Describe(".revert (notes)", func() {
		var path string
		var prevState core.BareRepoState

		BeforeEach(func() {
			repoName := util.RandString(5)
			path = filepath.Join(cfg.GetRepoRoot(), repoName)
			execGit(cfg.GetRepoRoot(), "init", repoName)
		})

		When("repo old state has 0 notes; new state has 1 note", func() {

			BeforeEach(func() {
				prevState, _ = repoMgr.GetRepoState(repo)
				Expect(prevState.IsEmpty()).To(BeTrue())
				createCommitAndNote(path, "file.txt", "v1 file", "v1 commit", "note1")
			})

			It("should remove the new note reference and old state should equal current state", func() {
				_, err := revert(repo, prevState)
				Expect(err).To(BeNil())
				curState, _ := repoMgr.GetRepoState(repo)
				Expect(curState.GetReferences()).To(Equal(prevState.GetReferences()))
			})
		})

		When("repo old state has 1 note; new state has 1 note but with updated content", func() {

			BeforeEach(func() {
				createCommitAndNote(path, "file.txt", "v1 file", "v1 commit", "note1")
				prevState, _ = repoMgr.GetRepoState(repo)
				createCommitAndNote(path, "file.txt", "v1 file", "v2 commit", "note1")
			})

			It("should reset the note reference to the previous value and old state should equal current state", func() {
				_, err := revert(repo, prevState)
				Expect(err).To(BeNil())
				curState, _ := repoMgr.GetRepoState(repo)
				Expect(curState.GetReferences()).To(Equal(prevState.GetReferences()))
			})
		})

		When("repo old state has 1 note; new state has 0 note", func() {

			BeforeEach(func() {
				createCommitAndNote(path, "file.txt", "v1 file", "v1 commit", "note1")
				prevState, _ = repoMgr.GetRepoState(repo)
				deleteNote(path, "refs/notes/note1")
			})

			It("should reset the note reference to the initial value and old state should equal current state", func() {
				_, err := revert(repo, prevState)
				Expect(err).To(BeNil())
				curState, _ := repoMgr.GetRepoState(repo)
				Expect(curState.GetReferences()).To(Equal(prevState.GetReferences()))
			})
		})
	})

	Describe(".getBranchRevertActions", func() {
		When("change type is unknown", func() {
			It("should return err=unknown change type", func() {
				changeItem := &core.ItemChange{
					Action: 100,
					Item:   &Obj{Name: "refs/heads/branch"},
				}
				_, err := getBranchRevertActions(changeItem, nil)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("unknown change type"))
			})
		})
	})

	Describe(".getTagRevertActions", func() {
		When("change type is unknown", func() {
			It("should return err=unknown change type", func() {
				changeItem := &core.ItemChange{
					Action: 100,
					Item:   &Obj{Name: "refs/tags/tagname"},
				}
				_, err := getTagRevertActions(changeItem, nil)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("unknown change type"))
			})
		})
	})

	Describe(".getNoteRevertActions", func() {
		When("change type is unknown", func() {
			It("should return err=unknown change type", func() {
				changeItem := &core.ItemChange{
					Action: 100,
					Item:   &Obj{Name: "refs/notes/notename"},
				}
				_, err := getNoteRevertActions(changeItem, nil)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("unknown change type"))
			})
		})
	})
})
