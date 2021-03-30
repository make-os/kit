package repo_test

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/bitfield/script"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/remote/plumbing"
	"github.com/make-os/kit/remote/repo"
	testutil2 "github.com/make-os/kit/remote/testutil"
	"github.com/make-os/kit/testutil"
	"github.com/make-os/kit/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("GitModule", func() {
	var err error
	var cfg *config.AppConfig
	var path string
	var r *repo.GitModule

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
	})

	BeforeEach(func() {
		repoName := util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		testutil2.ExecGit(cfg.GetRepoRoot(), "init", repoName)
		r = repo.NewGitModule(cfg.Node.GitBinPath, path)
	})

	AfterEach(func() {
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".RefDelete", func() {
		When("ref exist", func() {
			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
				nRef, _ := script.ExecInDir("git show-ref", path).CountLines()
				Expect(nRef).To(Equal(1))
				err := r.RefDelete("refs/heads/master")
				Expect(err).To(BeNil())
			})

			It("should return remove ref", func() {
				nRef, _ := script.ExecInDir("git show-ref", path).CountLines()
				Expect(nRef).To(Equal(0))
			})
		})

		When("ref does not exist", func() {
			BeforeEach(func() {
				nRef, _ := script.ExecInDir("git show-ref", path).CountLines()
				Expect(nRef).To(Equal(0))
				err = r.RefDelete("refs/heads/master")
			})

			It("should return nil", func() {
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".RefUpdate", func() {
		When("ref commit hash is not a valid sha1 hash", func() {
			BeforeEach(func() {
				err = r.RefUpdate("refs/heads/master", "invalid_sha1_hash")
				Expect(err).ToNot(BeNil())
			})

			It("should return err=...not a valid SHA1", func() {
				Expect(err.Error()).To(ContainSubstring("not a valid SHA1"))
			})
		})

		When("ref commit hash is valid but unknown, non-existent object", func() {
			BeforeEach(func() {
				err = r.RefUpdate("refs/heads/master", "3faa623fa42799dba4089f522784740b9ed49f9a")
				Expect(err).ToNot(BeNil())
			})

			It("should return err=...nonexistent object", func() {
				Expect(err.Error()).To(ContainSubstring("nonexistent object"))
			})
		})

		When("ref commit hash is valid but unknown, non-existent object", func() {
			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
				testutil2.AppendCommit(path, "file.txt", "some text 2", "commit msg 2")
				log := script.ExecInDir(`git --no-pager log --oneline --pretty="%H"`, path)
				nCommits, _ := log.CountLines()
				Expect(nCommits).To(Equal(2))
				commit1, _ := script.ExecInDir(`git --no-pager log --oneline --pretty=%H`, path).Last(1).String()
				err = r.RefUpdate("refs/heads/master", strings.TrimSpace(commit1))
			})

			It("should return no error", func() {
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".TagDelete", func() {
		When("tag exists", func() {
			BeforeEach(func() {
				testutil2.CreateCommitAndAnnotatedTag(path, "file.txt", "some text", "commit msg", "tag_v1")
				nTag, _ := script.ExecInDir(`git --no-pager tag -l`, path).CountLines()
				Expect(nTag).To(Equal(1))
				err = r.TagDelete("tag_v1")
				nTag, _ = script.ExecInDir(`git --no-pager tag -l`, path).CountLines()
				Expect(nTag).To(Equal(0))
			})

			It("should return one tag", func() {
				Expect(err).To(BeNil())
			})
		})

		When("tag does not exists", func() {
			BeforeEach(func() {
				err = r.TagDelete("tag_v1")
			})

			It("should return err=..tag 'tag_v1' not found", func() {
				Expect(err.Error()).To(ContainSubstring("tag 'tag_v1' not found"))
			})
		})
	})

	Describe(".RefGet", func() {
		When("ref does not exist", func() {
			BeforeEach(func() {
				_, err = r.RefGet("master")
			})

			It("should return err=ErrRefNotFound", func() {
				Expect(err).To(Equal(plumbing.ErrRefNotFound))
			})
		})

		When("ref exists", func() {
			var hash string

			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
				hash, err = r.RefGet("master")
			})

			It("should return err=nil", func() {
				Expect(err).To(BeNil())
			})

			It("should return 40 character hash", func() {
				Expect(len(hash)).To(Equal(40))
			})
		})
	})

	Describe(".GetHEAD", func() {
		It("should return the correct branch", func() {
			branch, err := r.GetHEAD(false)
			Expect(err).To(BeNil())
			Expect(branch).To(Equal("refs/heads/master"))
		})

		It("should return the correct branch", func() {
			branch, err := r.GetHEAD(true)
			Expect(err).To(BeNil())
			Expect(branch).To(Equal("master"))
		})
	})

	Describe(".NumCommits", func() {
		When("branch does not exist", func() {
			It("should return 0 and no error", func() {
				count, err := r.NumCommits("refs/heads/master", false)
				Expect(err).To(BeNil())
				Expect(count).To(Equal(0))
			})
		})

		When("branch does exist", func() {
			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
				testutil2.AppendCommit(path, "file.txt", "some text 2", "commit msg 2")
			})
			It("should return 0 and no error", func() {
				count, err := r.NumCommits("master", false)
				Expect(err).To(BeNil())
				Expect(count).To(Equal(2))
			})
		})

		When("branch includes a merge commit", func() {
			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
				testutil2.CreateCheckoutBranch(path, "dev")
				testutil2.AppendCommit(path, "file.txt", "log some good text", "commit msg")
				testutil2.CheckoutBranch(path, "master")
				testutil2.AppendCommit(path, "file.txt", "intro to \n****some nice text", "commit msg")
				testutil2.ForceMergeOurs(path, "dev")
			})

			It("should return 3 and no error when noMerge is false", func() {
				count, err := r.NumCommits("master", false)
				Expect(err).To(BeNil())
				Expect(count).To(Equal(4))
			})

			It("should return 3 and no error when noMerge is true", func() {
				count, err := r.NumCommits("master", true)
				Expect(err).To(BeNil())
				Expect(count).To(Equal(3))
			})
		})
	})

	Describe(".GetRecentCommitHash", func() {
		When("no recent commits", func() {
			It("should return err", func() {
				hash, err := r.GetRecentCommitHash()
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(plumbing.ErrNoCommits))
				Expect(hash).To(BeEmpty())
			})
		})

		When("commit exist", func() {
			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
			})

			It("should return 40 character hash", func() {
				hash, err := r.GetRecentCommitHash()
				Expect(err).To(BeNil())
				Expect(len(hash)).To(Equal(40))
			})
		})
	})

	Describe(".CreateTagWithMsg", func() {

		BeforeEach(func() {
			testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
			msg, _ := script.ExecInDir(`git --no-pager log --oneline -1 --pretty=%s`, path).String()
			Expect(strings.TrimSpace(msg)).To(Equal("commit msg"))
		})

		When("when signingKey is not set", func() {
			It("should create an annotated tag with message", func() {
				err := r.CreateTagWithMsg([]string{"my_tag"}, "a new tag", "")
				Expect(err).To(BeNil())
				out, _ := script.ExecInDir(`git cat-file -p refs/tags/my_tag`, path).Last(1).String()
				Expect(strings.TrimSpace(out)).To(Equal("a new tag"))
			})
		})
	})

	Describe(".ListTreeObjects", func() {
		var err error
		var entries map[string]string

		BeforeEach(func() {
			testutil2.CreateCommitAndNote(path, "file.txt", "hello", "commit 1", "note1")
			testutil2.CreateNoteEntry(path, "note1", "some note")
			entries, err = r.ListTreeObjects("refs/notes/note1", true)
			Expect(err).To(BeNil())
		})

		It("should return 2 entries", func() {
			Expect(entries).To(HaveLen(2))
		})
	})

	Describe(".ListTreeObjectsSlice", func() {
		var err error
		var entries []string

		BeforeEach(func() {
			testutil2.CreateCommitAndNote(path, "file.txt", "hello", "commit 1", "note1")
			testutil2.CreateNoteEntry(path, "note1", "some note")
			entries, err = r.ListTreeObjectsSlice("refs/notes/note1", true, false)
			Expect(err).To(BeNil())
		})

		It("should return a slice containing 2 entries", func() {
			Expect(entries).To(HaveLen(2))
		})
	})

	Describe(".RemoveEntryFromNote", func() {
		var err error
		var entries map[string]string

		BeforeEach(func() {
			testutil2.CreateCommitAndNote(path, "file.txt", "hello", "commit 1", "note1")
			entryHash := testutil2.CreateNoteEntry(path, "note1", "some note")
			entries, err = r.ListTreeObjects("refs/notes/note1", true)
			Expect(err).To(BeNil())
			Expect(entries).To(HaveLen(2))
			err = r.RemoveEntryFromNote("refs/notes/note1", entryHash)
			entries, _ = r.ListTreeObjects("refs/notes/note1", true)
		})

		It("should return 1 entry", func() {
			Expect(err).To(BeNil())
			Expect(entries).To(HaveLen(1))
		})
	})

	Describe(".AddEntryToNote", func() {
		var err error
		var entries map[string]string

		BeforeEach(func() {
			testutil2.CreateCommitAndNote(path, "file.txt", "hello", "commit 1", "note1")
			entries, err = r.ListTreeObjects("refs/notes/note1", true)
			Expect(err).To(BeNil())
			Expect(entries).To(HaveLen(1))

			hash := testutil2.CreateBlob(path, "some content")
			err = r.AddEntryToNote("refs/notes/note1", hash, "a note")
			Expect(err).To(BeNil())

			entries, err = r.ListTreeObjects("refs/notes/note1", true)
		})

		It("should return 2 entries", func() {
			Expect(err).To(BeNil())
			Expect(entries).To(HaveLen(2))
		})
	})

	Describe(".CreateBlob", func() {
		var hash string
		var err error

		BeforeEach(func() {
			hash, err = r.CreateBlob("some content")
			Expect(err).To(BeNil())
		})

		It("should return 40 character hash", func() {
			Expect(hash).To(HaveLen(40))
		})
	})

	Describe(".GetMergeCommits", func() {
		It("should return one hash when branch has a merge commit", func() {
			testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
			testutil2.CreateCheckoutBranch(path, "dev")
			testutil2.AppendCommit(path, "file.txt", "log some good text", "commit msg")
			testutil2.CheckoutBranch(path, "master")
			testutil2.AppendCommit(path, "file.txt", "intro to \n****some nice text", "commit msg")
			testutil2.ForceMergeOurs(path, "dev")
			testutil2.AppendCommit(path, "file.txt", "some other stuff", "commit msg")
			testutil2.AppendCommit(path, "file.txt", "some other other stuff", "commit msg")
			hashes, err := r.GetMergeCommits("master")
			Expect(err).To(BeNil())
			Expect(hashes).To(HaveLen(1))
		})
	})

	Describe(".HasMergeCommits", func() {
		It("should return true when branch has a merge commit", func() {
			testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
			testutil2.CreateCheckoutBranch(path, "dev")
			testutil2.AppendCommit(path, "file.txt", "log some good text", "commit msg")
			testutil2.CheckoutBranch(path, "master")
			testutil2.AppendCommit(path, "file.txt", "intro to \n****some nice text", "commit msg")
			testutil2.ForceMergeOurs(path, "dev")
			testutil2.AppendCommit(path, "file.txt", "some other stuff", "commit msg")
			testutil2.AppendCommit(path, "file.txt", "some other other stuff", "commit msg")
			has, err := r.HasMergeCommits("master")
			Expect(err).To(BeNil())
			Expect(has).To(BeTrue())
		})

		It("should return false when branch has no merge commit", func() {
			testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
			testutil2.AppendCommit(path, "file.txt", "some other text", "commit msg")
			has, err := r.HasMergeCommits("master")
			Expect(err).To(BeNil())
			Expect(has).To(BeFalse())
		})
	})

	Describe(".CreateSingleFileCommit", func() {
		It("should return hash and no error", func() {
			hash, err := r.CreateSingleFileCommit("body", "abc", "commit msg", "")
			Expect(err).To(BeNil())
			Expect(hash).To(HaveLen(40))
		})

		It("should return hash and no error when valid parent is provided", func() {
			parentHash, _ := r.CreateSingleFileCommit("body", "abc", "", "")
			Expect(err).To(BeNil())
			childHash, err := r.CreateSingleFileCommit("body", "abc", "", parentHash)
			Expect(err).To(BeNil())
			Expect(childHash).To(HaveLen(40))
			out := testutil2.ExecGit(path, "cat-file", "-p", childHash)
			Expect(string(out)).To(ContainSubstring("parent " + parentHash))
		})
	})

	Describe(".Checkout", func() {
		It("should return error if unable to checkout a non-existing branch", func() {
			err := r.Checkout("refs/heads/unknown", false, false)
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(plumbing.ErrRefNotFound))
		})

		It("should return no error and create branch if it does not exist but create=true", func() {
			err := r.Checkout("refs/heads/branch1", true, false)
			Expect(err).To(BeNil())
			testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
			hash, err := r.RefGet("refs/heads/branch1")
			Expect(err).To(BeNil())
			Expect(hash).To(HaveLen(40))
		})

		It("should return error when there are uncommitted local modifications", func() {
			testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
			testutil2.CreateCheckoutBranch(path, "dev")
			testutil2.AppendCommit(path, "file.txt", "some text 2", "commit msg 2")
			testutil2.CheckoutBranch(path, "master")
			testutil2.AppendToFile(path, "file.txt", "sample data")
			testutil2.ExecGitAdd(path, "file.txt")
			err = r.Checkout("dev", false, false)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("Your local changes to the following files"))
		})

		It("should return no error when there are uncommitted local modifications but force=true", func() {
			testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
			testutil2.CreateCheckoutBranch(path, "dev")
			testutil2.AppendCommit(path, "file.txt", "some text 2", "commit msg 2")
			testutil2.CheckoutBranch(path, "master")
			testutil2.AppendToFile(path, "file.txt", "sample data")
			testutil2.ExecGitAdd(path, "file.txt")
			err = r.Checkout("dev", false, true)
			Expect(err).To(BeNil())
		})
	})

	Describe(".GetRefCommits", func() {
		It("should return 2 hashes when there are 2 commits in branch", func() {
			testutil2.AppendCommit(path, "file.txt", "some text 1", "commit 1")
			testutil2.AppendCommit(path, "file.txt", "some text 2", "commit 2")
			hashes, err := r.GetRefCommits("refs/heads/master", false)
			Expect(err).To(BeNil())
			Expect(hashes).To(HaveLen(2))
		})

		It("should return 2 hashes when there are 3 commits and 1 merge commit in branch and noMerges = true", func() {
			testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
			testutil2.CreateCheckoutBranch(path, "dev")
			testutil2.AppendCommit(path, "file.txt", "log some good text", "commit msg")
			testutil2.CheckoutBranch(path, "master")
			testutil2.AppendCommit(path, "file.txt", "intro to \n****some nice text", "commit msg")
			testutil2.ForceMergeOurs(path, "dev")
			hashes, err := r.GetRefCommits("refs/heads/master", true)
			Expect(err).To(BeNil())
			Expect(hashes).To(HaveLen(3))
			mergeCommitHash := testutil2.GetRecentCommitHash(path, "master")
			Expect(hashes).ToNot(ContainElement(mergeCommitHash))
		})
	})

	Describe(".GetRefRootCommit", func() {
		It("should return the commit with no parent", func() {
			testutil2.AppendCommit(path, "file.txt", "some text 1", "commit 1")
			actualRootHash := testutil2.GetRecentCommitHash(path, "master")
			testutil2.AppendCommit(path, "file.txt", "some text 2", "commit 1")
			rootHash, err := r.GetRefRootCommit("master")
			Expect(err).To(BeNil())
			Expect(rootHash).To(Equal(actualRootHash))
		})

		It("should return the commit with no parent", func() {
			_, err := r.GetRefRootCommit("dev")
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(plumbing.ErrRefNotFound))
		})
	})

	Describe(".Var", func() {
		It("should return the value of GIT_PAGER", func() {
			val, err := r.Var("GIT_PAGER")
			Expect(err).To(BeNil())
			Expect(val).ToNot(BeEmpty())
		})

		It("should return ErrGitVarNotFound when variable is unknown", func() {
			_, err := r.Var("GIT_STUFF")
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(repo.ErrGitVarNotFound))
		})
	})

	Describe(".ExpandShortHash", func() {
		It("should return long version", func() {
			testutil2.AppendCommit(path, "file.txt", "some text 1", "commit 1")
			hash := testutil2.GetRecentCommitHash(path, "master")
			Expect(hash).To(HaveLen(40))
			res, err := r.ExpandShortHash(hash[:7])
			Expect(err).To(BeNil())
			Expect(res).To(Equal(hash))
		})
	})

	Describe(".GC", func() {
		It("should pack loose objects", func() {
			testutil2.AppendCommit(path, "file.txt", "some text 1", "commit 1")
			hash := testutil2.GetRecentCommitHash(path, "master")
			Expect(hash).To(HaveLen(40))
			Expect(util.IsPathOk(filepath.Join(path, ".git", "objects", hash[:2]))).To(BeTrue())
			Expect(r.GC()).To(BeNil())
			Expect(util.IsPathOk(filepath.Join(path, ".git", "objects", hash[:2]))).To(BeFalse())
		})

		It("should not delete unreachable objects if pruneExpire is unset", func() {
			hash, err := r.CreateBlob("alice is nice")
			Expect(err).To(BeNil())
			Expect(util.IsPathOk(filepath.Join(path, ".git", "objects", hash[:2]))).To(BeTrue())
			Expect(r.GC()).To(BeNil())
			Expect(util.IsPathOk(filepath.Join(path, ".git", "objects", hash[:2]))).To(BeTrue())
		})

		It("should remove all unreachable objects immediately if pruneExpire is now", func() {
			hash, err := r.CreateBlob("alice is nice")
			Expect(err).To(BeNil())
			Expect(util.IsPathOk(filepath.Join(path, ".git", "objects", hash[:2]))).To(BeTrue())
			Expect(r.GC("now")).To(BeNil())
			Expect(util.IsPathOk(filepath.Join(path, ".git", "objects", hash[:2]))).To(BeFalse())
		})
	})

	Describe(".Size", func() {
		It("should return expected size", func() {
			hash, err := r.CreateBlob("alice is nice")
			Expect(err).To(BeNil())
			Expect(util.IsPathOk(filepath.Join(path, ".git", "objects", hash[:2]))).To(BeTrue())
			size, err := r.Size()
			Expect(err).To(BeNil())
			Expect(size).To(Equal(float64(4096)))
		})
	})

	Describe(".GetPathUpdateTime", func() {
		BeforeEach(func() {
			r = repo.NewGitModule(cfg.Node.GitBinPath, "testdata/repo1")
			Expect(err).To(BeNil())
		})

		It("should get expected time", func() {
			t, err := r.GetPathUpdateTime("a")
			Expect(err).To(BeNil())
			Expect(t.Unix()).To(Equal(int64(1617047557)))

			t, err = r.GetPathUpdateTime("a/b")
			Expect(err).To(BeNil())
			Expect(t.Unix()).To(Equal(int64(1617042580)))

			t, err = r.GetPathUpdateTime("a/b/file3.txt")
			Expect(err).To(BeNil())
			Expect(t.Unix()).To(Equal(int64(1617042580)))

			t, err = r.GetPathUpdateTime("x.exe")
			Expect(err).To(BeNil())
			Expect(t.Unix()).To(Equal(int64(1617053884)))
		})

		When("path is unknown", func() {
			It("should get expected time", func() {
				_, err := r.GetPathUpdateTime("unknown")
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("path not found"))
			})
		})
	})
})
