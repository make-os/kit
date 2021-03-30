package plumbing_test

import (
	"os"
	"path/filepath"

	"github.com/bitfield/script"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/remote/plumbing"
	r "github.com/make-os/kit/remote/repo"
	testutil2 "github.com/make-os/kit/remote/testutil"
	"github.com/make-os/kit/remote/types"
	"github.com/make-os/kit/testutil"
	"github.com/make-os/kit/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("plumbing.Revert", func() {
	var err error
	var cfg *config.AppConfig
	var repo types.LocalRepo
	var repoName, path string

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())

		repoName = util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		testutil2.ExecGit(cfg.GetRepoRoot(), "init", repoName)

		repo, err = r.GetWithGitModule(cfg.Node.GitBinPath, path)
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".Revert (head references)", func() {
		var prevState types.RepoRefsState

		When("a repo has 1 ref and 4 commits; plumbing.Revert the 4th commit", func() {

			BeforeEach(func() {
				// update file.txt in 3 commits
				testutil2.AppendCommit(path, "file.txt", "line 1\n", "commit 1")
				testutil2.AppendCommit(path, "file.txt", "line 2\n", "commit 2")
				testutil2.AppendCommit(path, "file.txt", "line 3\n", "commit 3")

				prevState = plumbing.GetRepoState(repo)

				// update the file.txt in a new commit
				testutil2.AppendCommit(path, "file.txt", "line 4\n", "commit 4")
				lastLine, _ := testutil2.ScriptFile(path, "file.txt").Last(1).String()
				Expect(lastLine).To(Equal("line 4\n"))
			})

			Specify("that current state equal previous state", func() {
				_, err := plumbing.Revert(repo, prevState)
				Expect(err).To(BeNil())
				curState := plumbing.GetRepoState(repo)
				Expect(curState.GetReferences()).To(Equal(prevState.GetReferences()))
			})
		})

		When("a repo has 1 ref and 4 commits; plumbing.Revert the 3th & 4th commits", func() {

			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "line 1\n", "commit 1")
				testutil2.AppendCommit(path, "file.txt", "line 2\n", "commit 2")
				prevState = plumbing.GetRepoState(repo)
				testutil2.AppendCommit(path, "file.txt", "line 3\n", "commit 3")

				// update the file.txt in a new commit
				testutil2.AppendCommit(path, "file.txt", "line 4\n", "commit 4")
				lastLine, _ := testutil2.ScriptFile(path, "file.txt").Last(1).String()
				Expect(lastLine).To(Equal("line 4\n"))
			})

			Specify("that current state equal previous state", func() {
				_, err := plumbing.Revert(repo, prevState)
				Expect(err).To(BeNil())
				curState := plumbing.GetRepoState(repo)
				Expect(curState.GetReferences()).To(Equal(prevState.GetReferences()))
			})
		})

		When("a repo has 1 ref and 2 commits; prev state has matching ref but with unknown commit hash", func() {

			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "line 1\n", "commit 1")
				testutil2.AppendCommit(path, "file.txt", "line 2\n", "commit 2")
				prevState = &plumbing.State{
					References: plumbing.NewObjCol(map[string]types.Item{
						"refs/heads/master": &plumbing.Obj{
							Type: "ref",
							Name: "refs/heads/master",
							Data: "6ac8e9cf08409c169a12b526b20488d549106d69",
						},
					}),
				}
			})

			It("should return err='exec failed: hard reset failed'", func() {
				_, err := plumbing.Revert(repo, prevState)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("exec failed: reference update failed"))
			})
		})

		// This test case test how to handle reverting a new repo state update that
		// adds a new reference that isn't present on an older state
		When("a older state has 1 ref with 1 commit and new state has 2 ref (1 new ref) with 1 commit", func() {

			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "line 1", "commit 1")
				prevState = plumbing.GetRepoState(repo)
				testutil2.CreateCheckoutBranch(path, "branch2")
				testutil2.AppendCommit(path, "file.txt", "line 2", "commit 2")
			})

			It("should return err=nil and only 1 reference should exist and current state equal previous state", func() {
				_, err := plumbing.Revert(repo, prevState)
				Expect(err).To(BeNil())
				numRefs, _ := script.ExecInDir("git show-ref --heads", path).CountLines()
				Expect(numRefs).To(Equal(1))
				curState := plumbing.GetRepoState(repo)
				Expect(curState.GetReferences()).To(Equal(prevState.GetReferences()))
			})
		})

		// This test case tests how to handle reverting a new repo state update that
		// does not include references that are present on an older state
		When("a older state has 2 refs (master, branch2) with 1 commit in each | new state has 1 ref (master) with 1 commit", func() {

			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "line 1", "commit 1 of master branch")
				testutil2.CreateCheckoutBranch(path, "branch2")
				testutil2.AppendCommit(path, "file.txt", "line 1", "commit 1 of branch 2")
				prevState = plumbing.GetRepoState(repo)
				testutil2.ExecGit(path, "update-ref", "-d", "refs/heads/branch2")
			})

			It("should return err=nil and only 2 references should exist and current state equal previous state", func() {
				_, err := plumbing.Revert(repo, prevState)
				Expect(err).To(BeNil())
				numRefs, _ := script.ExecInDir("git show-ref --heads", path).CountLines()
				Expect(numRefs).To(Equal(2))
				curState := plumbing.GetRepoState(repo)
				Expect(curState).To(Equal(prevState))
			})
		})

		// This test case tests how to handle reverting a new repo state update that
		// does not include references that are present on an older state
		When("a older state has 4 refs (master, branch2, branch3, branch4) with 1 commit in each | new state has 1 ref (master) with 1 commit", func() {

			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "line 1", "commit 1 of master branch")
				testutil2.CreateCheckoutBranch(path, "branch2")
				testutil2.AppendCommit(path, "file.txt", "line 1", "commit 1 of branch 2")
				testutil2.CreateCheckoutBranch(path, "branch3")
				testutil2.AppendCommit(path, "file.txt", "line 1", "commit 1 of branch 3")
				testutil2.CreateCheckoutBranch(path, "branch4")
				testutil2.AppendCommit(path, "file.txt", "line 1", "commit 1 of branch 4")
				prevState = plumbing.GetRepoState(repo)
				testutil2.ExecGit(path, "update-ref", "-d", "refs/heads/branch2")
				testutil2.ExecGit(path, "update-ref", "-d", "refs/heads/branch3")
				testutil2.ExecGit(path, "update-ref", "-d", "refs/heads/branch4")
			})

			It("should return err=nil and only 2 references should exist and current state equal previous state", func() {
				_, err := plumbing.Revert(repo, prevState)
				Expect(err).To(BeNil())
				numRefs, _ := script.ExecInDir("git show-ref --heads", path).CountLines()
				Expect(numRefs).To(Equal(4))
				curState := plumbing.GetRepoState(repo)
				Expect(curState).To(Equal(prevState))
			})
		})
	})

	Describe(".Revert (annotated tags)", func() {
		var prevState types.RepoRefsState

		When("repo old state has 0 tags; new state has 1 tag", func() {
			BeforeEach(func() {
				prevState = plumbing.GetRepoState(repo)
				Expect(prevState.IsEmpty()).To(BeTrue())
				testutil2.CreateCommitAndAnnotatedTag(path, "file.txt", "v1 file", "v1 commit", "v1")
			})

			It("should remove the new tag and old state should equal current state", func() {
				_, err := plumbing.Revert(repo, prevState)
				Expect(err).To(BeNil())
				curState := plumbing.GetRepoState(repo)
				Expect(curState.GetReferences()).To(Equal(prevState.GetReferences()))
			})
		})

		When("repo old state has 1 tags; new state has 3 tag", func() {

			BeforeEach(func() {
				testutil2.CreateCommitAndAnnotatedTag(path, "file.txt", "first file", "first commit", "v1")
				prevState = plumbing.GetRepoState(repo)
				testutil2.CreateCommitAndAnnotatedTag(path, "file.txt", "first file", "commit 2", "v2")
				testutil2.CreateCommitAndAnnotatedTag(path, "file.txt", "first file", "commit 3", "v3")
				testutil2.CreateCommitAndAnnotatedTag(path, "file.txt", "first file", "commit 4", "v4")
			})

			It("should remove the new tags and old state should equal current state", func() {
				_, err := plumbing.Revert(repo, prevState)
				Expect(err).To(BeNil())
				curState := plumbing.GetRepoState(repo)
				Expect(curState.GetReferences()).To(Equal(prevState.GetReferences()))
			})
		})

		When("repo old state has 1 annotated tag (v1); new state has same 1 annotated tag (v1) but with different value", func() {

			BeforeEach(func() {
				testutil2.CreateCommitAndAnnotatedTag(path, "file.txt", "first file", "first commit", "v1")
				prevState = plumbing.GetRepoState(repo)
				testutil2.CreateCommitAndAnnotatedTag(path, "file.txt", "updated file", "second commit", "v1")
			})

			It("should update the reference value of the tag to the old value and old state should equal current state", func() {
				_, err := plumbing.Revert(repo, prevState)
				Expect(err).To(BeNil())
				curState := plumbing.GetRepoState(repo)
				Expect(curState.GetReferences()).To(Equal(prevState.GetReferences()))
			})
		})

		When("repo old state has 1 lightweight tag (v1); new state has same 1 lightweight tag (v1) but with different value", func() {

			BeforeEach(func() {
				testutil2.CreateCommitAndLightWeightTag(path, "file.txt", "first file", "first commit", "v1")
				prevState = plumbing.GetRepoState(repo)
				testutil2.CreateCommitAndLightWeightTag(path, "file.txt", "updated file", "second commit", "v1")
			})

			It("should update the reference value of the tag to the old value and old state should equal current state", func() {
				_, err := plumbing.Revert(repo, prevState)
				Expect(err).To(BeNil())
				curState := plumbing.GetRepoState(repo)
				Expect(curState.GetReferences()).To(Equal(prevState.GetReferences()))
			})
		})

		When("repo old state has 2 annotated tag (v1,v2); new state has same 2 annotated tag (v1,v2) but with different value", func() {

			BeforeEach(func() {
				testutil2.CreateCommitAndAnnotatedTag(path, "file.txt", "file1", "first commit", "v1")
				testutil2.CreateCommitAndAnnotatedTag(path, "file.txt", "file2", "second commit", "v2")
				prevState = plumbing.GetRepoState(repo)
				testutil2.CreateCommitAndAnnotatedTag(path, "file.txt", "file3", "third commit", "v1")
				testutil2.CreateCommitAndAnnotatedTag(path, "file.txt", "file4", "fourth commit", "v2")
			})

			It("should update the reference value of the tags to their old value and old state should equal current state", func() {
				_, err := plumbing.Revert(repo, prevState)
				Expect(err).To(BeNil())
				curState := plumbing.GetRepoState(repo)
				Expect(curState.GetReferences()).To(Equal(prevState.GetReferences()))
			})
		})

		When("repo old state has 1 annotated tag (v1); new state has 0 annotated tag (v1)", func() {

			BeforeEach(func() {
				testutil2.CreateCommitAndAnnotatedTag(path, "file.txt", "first file", "first commit", "v1")
				prevState = plumbing.GetRepoState(repo)
				testutil2.DeleteTag(path, "v1")
			})

			It("should reset the tag value to the old tag value and old state should equal current state", func() {
				_, err := plumbing.Revert(repo, prevState)
				Expect(err).To(BeNil())
				curState := plumbing.GetRepoState(repo)
				Expect(curState.GetReferences()).To(Equal(prevState.GetReferences()))
			})
		})
	})

	Describe(".Revert (notes)", func() {
		var prevState types.RepoRefsState

		When("repo old state has 0 notes; new state has 1 note", func() {

			BeforeEach(func() {
				prevState = plumbing.GetRepoState(repo)
				Expect(prevState.IsEmpty()).To(BeTrue())
				testutil2.CreateCommitAndNote(path, "file.txt", "v1 file", "v1 commit", "note1")
			})

			It("should remove the new note reference and old state should equal current state", func() {
				_, err := plumbing.Revert(repo, prevState)
				Expect(err).To(BeNil())
				curState := plumbing.GetRepoState(repo)
				Expect(curState.GetReferences()).To(Equal(prevState.GetReferences()))
			})
		})

		When("repo old state has 1 note; new state has 1 note but with updated content", func() {

			BeforeEach(func() {
				testutil2.CreateCommitAndNote(path, "file.txt", "v1 file", "v1 commit", "note1")
				prevState = plumbing.GetRepoState(repo)
				testutil2.CreateCommitAndNote(path, "file.txt", "v1 file", "v2 commit", "note1")
			})

			It("should reset the note reference to the previous value and old state should equal current state", func() {
				_, err := plumbing.Revert(repo, prevState)
				Expect(err).To(BeNil())
				curState := plumbing.GetRepoState(repo)
				Expect(curState.GetReferences()).To(Equal(prevState.GetReferences()))
			})
		})

		When("repo old state has 1 note; new state has 0 note", func() {

			BeforeEach(func() {
				testutil2.CreateCommitAndNote(path, "file.txt", "v1 file", "v1 commit", "note1")
				prevState = plumbing.GetRepoState(repo)
				testutil2.DeleteRef(path, "refs/notes/note1")
			})

			It("should reset the note reference to the initial value and old state should equal current state", func() {
				_, err := plumbing.Revert(repo, prevState)
				Expect(err).To(BeNil())
				curState := plumbing.GetRepoState(repo)
				Expect(curState.GetReferences()).To(Equal(prevState.GetReferences()))
			})
		})
	})

	Describe(".GetBranchRevertActions", func() {
		When("change type is unknown", func() {
			It("should return err=unknown change type", func() {
				changeItem := &types.ItemChange{
					Action: 100,
					Item:   &plumbing.Obj{Name: "refs/heads/branch"},
				}
				_, err := plumbing.GetBranchRevertActions(changeItem, nil)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("unknown change type"))
			})
		})
	})

	Describe(".GetTagRevertActions", func() {
		When("change type is unknown", func() {
			It("should return err=unknown change type", func() {
				changeItem := &types.ItemChange{
					Action: 100,
					Item:   &plumbing.Obj{Name: "refs/tags/tagname"},
				}
				_, err := plumbing.GetTagRevertActions(changeItem, nil)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("unknown change type"))
			})
		})
	})

	Describe(".GetNoteRevertActions", func() {
		When("change type is unknown", func() {
			It("should return err=unknown change type", func() {
				changeItem := &types.ItemChange{
					Action: 100,
					Item:   &plumbing.Obj{Name: "refs/notes/notename"},
				}
				_, err := plumbing.GetNoteRevertActions(changeItem, nil)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("unknown change type"))
			})
		})
	})
})
