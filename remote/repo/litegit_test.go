package repo_test

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/bitfield/script"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/remote/repo"
	testutil2 "gitlab.com/makeos/mosdef/remote/testutil"
	"gitlab.com/makeos/mosdef/testutil"
	"gitlab.com/makeos/mosdef/util"
)

var _ = Describe("Gitops", func() {
	var err error
	var cfg *config.AppConfig
	var path string
	var lg *repo.LiteGit

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
	})

	BeforeEach(func() {
		repoName := util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		testutil2.ExecGit(cfg.GetRepoRoot(), "init", repoName)
		lg = repo.NewLiteGit(cfg.Node.GitBinPath, path)
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
				err := lg.RefDelete("refs/heads/master")
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
				err = lg.RefDelete("refs/heads/master")
			})

			It("should return nil", func() {
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".RefUpdate", func() {
		When("ref commit hash is not a valid sha1 hash", func() {
			BeforeEach(func() {
				err = lg.RefUpdate("refs/heads/master", "invalid_sha1_hash")
				Expect(err).ToNot(BeNil())
			})

			It("should return err=...not a valid SHA1", func() {
				Expect(err.Error()).To(ContainSubstring("not a valid SHA1"))
			})
		})

		When("ref commit hash is valid but unknown, non-existent object", func() {
			BeforeEach(func() {
				err = lg.RefUpdate("refs/heads/master", "3faa623fa42799dba4089f522784740b9ed49f9a")
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
				err = lg.RefUpdate("refs/heads/master", strings.TrimSpace(commit1))
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
				err = lg.TagDelete("tag_v1")
				nTag, _ = script.ExecInDir(`git --no-pager tag -l`, path).CountLines()
				Expect(nTag).To(Equal(0))
			})

			It("should return one tag", func() {
				Expect(err).To(BeNil())
			})
		})

		When("tag does not exists", func() {
			BeforeEach(func() {
				err = lg.TagDelete("tag_v1")
			})

			It("should return err=..tag 'tag_v1' not found", func() {
				Expect(err.Error()).To(ContainSubstring("tag 'tag_v1' not found"))
			})
		})
	})

	Describe(".RefGet", func() {
		When("ref does not exist", func() {
			BeforeEach(func() {
				_, err = lg.RefGet("master")
			})

			It("should return err=ErrRefNotFound", func() {
				Expect(err).To(Equal(repo.ErrRefNotFound))
			})
		})

		When("ref exists", func() {
			var hash string

			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
				hash, err = lg.RefGet("master")
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
			branch, err := lg.GetHEAD(false)
			Expect(err).To(BeNil())
			Expect(branch).To(Equal("refs/heads/master"))
		})

		It("should return the correct branch", func() {
			branch, err := lg.GetHEAD(true)
			Expect(err).To(BeNil())
			Expect(branch).To(Equal("master"))
		})
	})

	Describe(".NumCommits", func() {
		When("branch does not exist", func() {
			It("should return 0 and no error", func() {
				count, err := lg.NumCommits("master")
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
				count, err := lg.NumCommits("master")
				Expect(err).To(BeNil())
				Expect(count).To(Equal(2))
			})
		})
	})

	Describe(".GetRecentCommit", func() {
		When("no recent commits", func() {
			It("should return err", func() {
				hash, err := lg.GetRecentCommit()
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(repo.ErrNoCommits))
				Expect(hash).To(BeEmpty())
			})
		})

		When("commit exist", func() {
			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
			})

			It("should return 40 character hash", func() {
				hash, err := lg.GetRecentCommit()
				Expect(err).To(BeNil())
				Expect(len(hash)).To(Equal(40))
			})
		})
	})

	Describe(".GetConfig", func() {
		It("should return empty string when not found", func() {
			val := lg.GetConfig("some.config.key")
			Expect(val).To(BeEmpty())
		})

		It("should return correct value when found", func() {
			testutil2.ExecGit(path, "config", "some.config.key", "value")
			val := lg.GetConfig("some.config.key")
			Expect(val).To(Equal("value"))
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
				err := lg.CreateTagWithMsg([]string{"my_tag"}, "a new tag", "")
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
			entries, err = lg.ListTreeObjects("refs/notes/note1", true)
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
			entries, err = lg.ListTreeObjectsSlice("refs/notes/note1", true, false)
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
			entries, err = lg.ListTreeObjects("refs/notes/note1", true)
			Expect(err).To(BeNil())
			Expect(entries).To(HaveLen(2))
			err = lg.RemoveEntryFromNote("refs/notes/note1", entryHash)
			entries, _ = lg.ListTreeObjects("refs/notes/note1", true)
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
			entries, err = lg.ListTreeObjects("refs/notes/note1", true)
			Expect(err).To(BeNil())
			Expect(entries).To(HaveLen(1))

			hash := testutil2.CreateBlob(path, "some content")
			err = lg.AddEntryToNote("refs/notes/note1", hash, "a note")
			Expect(err).To(BeNil())

			entries, err = lg.ListTreeObjects("refs/notes/note1", true)
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
			hash, err = lg.CreateBlob("some content")
			Expect(err).To(BeNil())
		})

		It("should return 40 character hash", func() {
			Expect(hash).To(HaveLen(40))
		})
	})

	Describe(".IsDescendant", func() {
		It("should return no error when child is a descendant of parent", func() {
			testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
			rootHash := testutil2.GetRecentCommitHash(path, "refs/heads/master")
			testutil2.AppendCommit(path, "file.txt", "some text appended", "commit msg")
			childOfRootHash := testutil2.GetRecentCommitHash(path, "refs/heads/master")
			Expect(lg.IsDescendant(childOfRootHash, rootHash)).To(BeNil())
		})

		It("should return error when child is not a descendant of parent", func() {
			testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
			rootHash := testutil2.GetRecentCommitHash(path, "refs/heads/master")
			testutil2.AppendCommit(path, "file.txt", "some text appended", "commit msg")
			childOfRootHash := testutil2.GetRecentCommitHash(path, "refs/heads/master")
			err = lg.IsDescendant(rootHash, childOfRootHash)
			Expect(err).ToNot(BeNil())
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
			hashes, err := lg.GetMergeCommits("master")
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
			has, err := lg.HasMergeCommits("master")
			Expect(err).To(BeNil())
			Expect(has).To(BeTrue())
		})

		It("should return false when branch has no merge commit", func() {
			testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
			testutil2.AppendCommit(path, "file.txt", "some other text", "commit msg")
			has, err := lg.HasMergeCommits("master")
			Expect(err).To(BeNil())
			Expect(has).To(BeFalse())
		})
	})

	Describe(".CreateSingleFileCommit", func() {
		It("should return hash and no error", func() {
			hash, err := lg.CreateSingleFileCommit("body", "abc", "")
			Expect(err).To(BeNil())
			Expect(hash).To(HaveLen(40))
		})

		It("should return hash and no error when valid parent is provided", func() {
			parentHash, _ := lg.CreateSingleFileCommit("body", "abc", "")
			Expect(err).To(BeNil())
			childHash, err := lg.CreateSingleFileCommit("body", "abc", parentHash)
			Expect(err).To(BeNil())
			Expect(childHash).To(HaveLen(40))
			out := testutil2.ExecGit(path, "cat-file", "-p", childHash)
			Expect(string(out)).To(ContainSubstring("parent " + parentHash))
		})
	})
})
