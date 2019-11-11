package repo

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/bitfield/script"
	"github.com/phayes/freeport"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/testutil"
	"github.com/makeos/mosdef/util"
)

var gitPath = "/usr/bin/git"

func execGit(workDir string, arg ...string) []byte {
	cmd := exec.Command(gitPath, arg...)
	cmd.Dir = workDir
	bz, err := cmd.Output()
	if err != nil {
		panic(err)
	}
	return bz
}

func appendToFile(path, file string, data string) {
	script.Echo(data).AppendFile(filepath.Join(path, file))
}

func execGitCommit(path, msg string) []byte {
	execGit(path, "add", ".")
	return execGit(path, "commit", "-m", msg)
}

func appendCommit(path, file, fileData, commitMsg string) {
	appendToFile(path, file, fileData)
	execGitCommit(path, commitMsg)
}

func createAnnotatedTag(path, file, fileData, commitMsg, tagName string) {
	appendToFile(path, file, fileData)
	execGitCommit(path, commitMsg)
	execGit(path, "tag", "-a", tagName, "-m", `""`, "-f")
}

func createLightWeightTag(path, file, fileData, commitMsg, tagName string) {
	appendToFile(path, file, fileData)
	execGitCommit(path, commitMsg)
	execGit(path, "tag", tagName, "-f")
}

func deleteTag(path, name string) {
	execGit(path, "tag", "-d", name)
}

func scriptFile(path, file string) *script.Pipe {
	return script.File(filepath.Join(path, file))
}

func createCheckoutBranch(path, branch string) {
	execGit(path, "checkout", "-b", branch)
}

func execAnyCmd(workDir, name string, arg ...string) []byte {
	cmd := exec.Command(name, arg...)
	cmd.Dir = workDir
	bz, err := cmd.Output()
	if err != nil {
		panic(err)
	}
	return bz
}

var _ = Describe("Revert", func() {
	var err error
	var cfg *config.EngineConfig
	var repoMgr *Manager

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
		port, _ := freeport.GetFreePort()
		repoMgr = NewManager(cfg, fmt.Sprintf(":%d", port))
	})

	AfterEach(func() {
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".Revert (head references)", func() {
		var path string
		var prevState *State

		BeforeEach(func() {
			repoName := util.RandString(5)
			path = filepath.Join(cfg.GetRepoRoot(), repoName)
			execGit(cfg.GetRepoRoot(), "init", repoName)
		})

		When("a repo has 1 ref and 4 commits; revert the 4th commit", func() {

			BeforeEach(func() {
				// update file.txt in 3 commits
				appendCommit(path, "file.txt", "line 1\n", "commit 1")
				appendCommit(path, "file.txt", "line 2\n", "commit 2")
				appendCommit(path, "file.txt", "line 3\n", "commit 3")

				prevState, _ = repoMgr.GetRepoState(path)

				// update the file.txt in a new commit
				appendCommit(path, "file.txt", "line 4\n", "commit 4")
				lastLine, _ := scriptFile(path, "file.txt").Last(1).String()
				Expect(lastLine).To(Equal("line 4\n"))
			})

			Specify("that current state equal previous state", func() {
				err := repoMgr.Revert(path, prevState)
				Expect(err).To(BeNil())
				curState, _ := repoMgr.GetRepoState(path)
				Expect(curState).To(Equal(prevState))
			})
		})

		When("a repo has 1 ref and 4 commits; revert the 3th & 4th commits", func() {

			BeforeEach(func() {
				appendCommit(path, "file.txt", "line 1\n", "commit 1")
				appendCommit(path, "file.txt", "line 2\n", "commit 2")
				prevState, _ = repoMgr.GetRepoState(path)
				appendCommit(path, "file.txt", "line 3\n", "commit 3")

				// update the file.txt in a new commit
				appendCommit(path, "file.txt", "line 4\n", "commit 4")
				lastLine, _ := scriptFile(path, "file.txt").Last(1).String()
				Expect(lastLine).To(Equal("line 4\n"))
			})

			Specify("that current state equal previous state", func() {
				err := repoMgr.Revert(path, prevState)
				Expect(err).To(BeNil())
				curState, _ := repoMgr.GetRepoState(path)
				Expect(curState).To(Equal(prevState))
			})
		})

		When("a repo has 1 ref and 2 commits; prev state has matching ref but with unknown commit hash", func() {

			BeforeEach(func() {
				appendCommit(path, "file.txt", "line 1\n", "commit 1")
				appendCommit(path, "file.txt", "line 2\n", "commit 2")
				prevState = &State{
					Refs: NewObjCol(map[string]*Obj{
						"refs/heads/master": {
							Type: "ref",
							Name: "refs/heads/master",
							Data: "6ac8e9cf08409c169a12b526b20488d549106d69",
						},
					}),
				}
			})

			It("should return err='exec failed: hard reset failed'", func() {
				err := repoMgr.Revert(path, prevState)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("exec failed: reference update failed"))
			})
		})

		// This test case test how to handle reverting a new repo state update that
		// adds a new reference that isn't present on an older state
		When("a older state has 1 ref with 1 commit and new state has 2 ref (1 new ref) with 1 commit", func() {

			BeforeEach(func() {
				appendCommit(path, "file.txt", "line 1", "commit 1")
				prevState, _ = repoMgr.GetRepoState(path)
				createCheckoutBranch(path, "branch2")
				appendCommit(path, "file.txt", "line 2", "commit 2")
			})

			It("should return err=nil and only 1 reference should exist and current state equal previous state", func() {
				err := repoMgr.Revert(path, prevState)
				Expect(err).To(BeNil())
				numRefs, _ := script.ExecInDir("git show-ref --heads", path).CountLines()
				Expect(numRefs).To(Equal(1))
				curState, _ := repoMgr.GetRepoState(path)
				Expect(curState).To(Equal(prevState))
			})
		})

		// This test case tests how to handle reverting a new repo state update that
		// does not include references that are present on an older state
		When("a older state has 2 refs (master, branch2) with 1 commit in each | new state has 1 ref (master) with 1 commit", func() {

			BeforeEach(func() {
				appendCommit(path, "file.txt", "line 1", "commit 1 of master branch")
				createCheckoutBranch(path, "branch2")
				appendCommit(path, "file.txt", "line 1", "commit 1 of branch 2")
				prevState, _ = repoMgr.GetRepoState(path)
				execGit(path, "update-ref", "-d", "refs/heads/branch2")
			})

			It("should return err=nil and only 2 references should exist and current state equal previous state", func() {
				err := repoMgr.Revert(path, prevState)
				Expect(err).To(BeNil())
				numRefs, _ := script.ExecInDir("git show-ref --heads", path).CountLines()
				Expect(numRefs).To(Equal(2))
				curState, _ := repoMgr.GetRepoState(path)
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
				prevState, _ = repoMgr.GetRepoState(path)
				execGit(path, "update-ref", "-d", "refs/heads/branch2")
				execGit(path, "update-ref", "-d", "refs/heads/branch3")
				execGit(path, "update-ref", "-d", "refs/heads/branch4")
			})

			It("should return err=nil and only 2 references should exist and current state equal previous state", func() {
				err := repoMgr.Revert(path, prevState)
				Expect(err).To(BeNil())
				numRefs, _ := script.ExecInDir("git show-ref --heads", path).CountLines()
				Expect(numRefs).To(Equal(4))
				curState, _ := repoMgr.GetRepoState(path)
				Expect(curState).To(Equal(prevState))
			})
		})
	})

	Describe(".Revert (annotated tags)", func() {
		var path string
		var prevState *State

		BeforeEach(func() {
			repoName := util.RandString(5)
			path = filepath.Join(cfg.GetRepoRoot(), repoName)
			execGit(cfg.GetRepoRoot(), "init", repoName)
		})

		When("repo old state has 0 tags; new state has 1 tag", func() {

			BeforeEach(func() {
				prevState, _ = repoMgr.GetRepoState(path)
				Expect(prevState.IsEmpty()).To(BeTrue())
				createAnnotatedTag(path, "file.txt", "v1 file", "v1 commit", "v1")
			})

			It("should remove the new tag and old state should equal current state", func() {
				err := repoMgr.Revert(path, prevState)
				Expect(err).To(BeNil())
				curState, _ := repoMgr.GetRepoState(path)
				Expect(curState).To(Equal(prevState))
			})
		})

		When("repo old state has 1 tags; new state has 3 tag", func() {

			BeforeEach(func() {
				createAnnotatedTag(path, "file.txt", "first file", "first commit", "v1")
				prevState, _ = repoMgr.GetRepoState(path)
				createAnnotatedTag(path, "file.txt", "first file", "commit 2", "v2")
				createAnnotatedTag(path, "file.txt", "first file", "commit 3", "v3")
				createAnnotatedTag(path, "file.txt", "first file", "commit 4", "v4")
			})

			It("should remove the new tags and old state should equal current state", func() {
				err := repoMgr.Revert(path, prevState)
				Expect(err).To(BeNil())
				curState, _ := repoMgr.GetRepoState(path)
				Expect(curState).To(Equal(prevState))
			})
		})

		When("repo old state has 1 annotated tag (v1); new state has same 1 annotated tag (v1) but with different value", func() {

			BeforeEach(func() {
				createAnnotatedTag(path, "file.txt", "first file", "first commit", "v1")
				prevState, _ = repoMgr.GetRepoState(path)
				createAnnotatedTag(path, "file.txt", "updated file", "second commit", "v1")
			})

			It("should update the reference value of the tag to the old value and old state should equal current state", func() {
				err := repoMgr.Revert(path, prevState)
				Expect(err).To(BeNil())
				curState, _ := repoMgr.GetRepoState(path)
				Expect(curState).To(Equal(prevState))
			})
		})

		When("repo old state has 1 lightweight tag (v1); new state has same 1 lightweight tag (v1) but with different value", func() {

			BeforeEach(func() {
				createLightWeightTag(path, "file.txt", "first file", "first commit", "v1")
				prevState, _ = repoMgr.GetRepoState(path)
				createLightWeightTag(path, "file.txt", "updated file", "second commit", "v1")
			})

			It("should update the reference value of the tag to the old value and old state should equal current state", func() {
				err := repoMgr.Revert(path, prevState)
				Expect(err).To(BeNil())
				curState, _ := repoMgr.GetRepoState(path)
				Expect(curState).To(Equal(prevState))
			})
		})

		When("repo old state has 2 annotated tag (v1,v2); new state has same 2 annotated tag (v1,v2) but with different value", func() {

			BeforeEach(func() {
				createAnnotatedTag(path, "file.txt", "file1", "first commit", "v1")
				createAnnotatedTag(path, "file.txt", "file2", "second commit", "v2")
				prevState, _ = repoMgr.GetRepoState(path)
				createAnnotatedTag(path, "file.txt", "file3", "third commit", "v1")
				createAnnotatedTag(path, "file.txt", "file4", "fourth commit", "v2")
			})

			It("should update the reference value of the tags to their old value and old state should equal current state", func() {
				err := repoMgr.Revert(path, prevState)
				Expect(err).To(BeNil())
				curState, _ := repoMgr.GetRepoState(path)
				Expect(curState).To(Equal(prevState))
			})
		})

		When("repo old state has 1 annotated tag (v1); new state has 0 annotated tag (v1) but", func() {

			BeforeEach(func() {
				createAnnotatedTag(path, "file.txt", "first file", "first commit", "v1")
				prevState, _ = repoMgr.GetRepoState(path)
				deleteTag(path, "v1")
			})

			It("should reset the tag value to the old tag value and old state should equal current state", func() {
				err := repoMgr.Revert(path, prevState)
				Expect(err).To(BeNil())
				curState, _ := repoMgr.GetRepoState(path)
				Expect(curState).To(Equal(prevState))
			})
		})
	})
})
