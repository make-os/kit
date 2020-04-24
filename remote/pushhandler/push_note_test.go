package pushhandler

import (
	"bytes"
	"io"
	"os"
	"path/filepath"

	plumbing2 "gitlab.com/makeos/mosdef/remote/plumbing"
	repo3 "gitlab.com/makeos/mosdef/remote/repo"
	testutil2 "gitlab.com/makeos/mosdef/remote/testutil"
	"gitlab.com/makeos/mosdef/types/core"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/testutil"
	"gitlab.com/makeos/mosdef/util"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/format/packfile"
)

var _ = Describe("PushNote", func() {
	var pushNote *core.PushNote
	var cfg *config.AppConfig
	var repo core.BareRepo
	var path string
	var err error

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
		repoName := util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		testutil2.ExecGit(cfg.GetRepoRoot(), "init", repoName)
		repo, err = repo3.GetRepoWithLiteGit(cfg.Node.GitBinPath, path)
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	BeforeEach(func() {
		var pushKeyID = []byte("pk_id")
		pushNote = &core.PushNote{
			RepoName:        "repo",
			NodeSig:         []byte("node_signer_sig"),
			PushKeyID:       pushKeyID,
			PusherAcctNonce: 2,
			References: []*core.PushedReference{
				{
					Nonce:   1,
					NewHash: "new_object_hash",
					Name:    "refs/heads/master",
					OldHash: "old_object_hash",
					Fee:     "0.2",
				},
			},
		}
	})

	Describe(".Bytes", func() {
		It("should return expected bytes", func() {
			Expect(pushNote.Bytes()).ToNot(HaveLen(0))
		})
	})

	Describe(".ID", func() {
		It("should return expected bytes", func() {
			Expect(len(pushNote.ID())).To(Equal(32))
		})
	})

	Describe(".Fee", func() {
		It("should return expected total fee", func() {
			Expect(pushNote.GetFee()).To(Equal(util.String("0.2")))
		})

		It("should return expected total fee", func() {
			pushNote.References = append(pushNote.References, &core.PushedReference{
				Nonce:   1,
				NewHash: "new_object_hash",
				Name:    "refs/heads/master",
				OldHash: "old_object_hash",
			})
			Expect(pushNote.GetFee()).To(Equal(util.String("0.2")))
		})
	})

	Describe(".Len", func() {
		It("should not return zero", func() {
			Expect(pushNote.Len()).ToNot(Equal(0))
		})
	})

	Describe(".TxSize", func() {
		It("should not return zero", func() {
			Expect(pushNote.TxSize()).ToNot(Equal(0))
		})
	})

	Describe(".OverallSize", func() {
		It("should not return zero", func() {
			Expect(pushNote.BillableSize()).ToNot(Equal(0))
		})
	})

	Describe(".makePackfileFromPushNote", func() {
		var buf io.ReadSeeker
		var commitHash, commitHash2 string

		When("a commit hash is added to a reference object list", func() {
			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
				commitHash, _ = repo.GetRecentCommitHash()
				tx := &core.PushNote{
					References: []*core.PushedReference{
						{Objects: []string{commitHash}},
					},
				}
				buf, err = makePackfileFromPushNote(repo, tx)
				Expect(err).To(BeNil())
			})

			It("should return a packfile containing the commit hash", func() {
				sc := packfile.NewScanner(buf)
				_, objCount, _ := sc.Header()
				Expect(objCount).To(Equal(uint32(1)))

				oh, err := sc.NextObjectHeader()
				Expect(err).To(BeNil())

				enc := &plumbing.MemoryObject{}
				_, _, err = sc.NextObject(enc)
				Expect(err).To(BeNil())
				enc.SetType(oh.Type)

				Expect(enc.Hash().String()).To(Equal(commitHash))
			})
		})

		When("a two commit hashes are added to a reference object list", func() {
			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
				commitHash, _ = repo.GetRecentCommitHash()
				testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
				commitHash2, _ = repo.GetRecentCommitHash()
				tx := &core.PushNote{
					References: []*core.PushedReference{
						{Objects: []string{commitHash, commitHash2}},
					},
				}
				buf, err = makePackfileFromPushNote(repo, tx)
				Expect(err).To(BeNil())
			})

			It("should return a packfile containing the 2 objects", func() {
				sc := packfile.NewScanner(buf)
				_, objCount, _ := sc.Header()
				Expect(objCount).To(Equal(uint32(2)))
			})
		})

		When("a hash that does not exist in the repository is added to a reference object list", func() {
			BeforeEach(func() {
				commitHash = "c212fb1166aeb2f42a54203f9f9315107265028f"
				tx := &core.PushNote{
					References: []*core.PushedReference{
						{Objects: []string{commitHash}},
					},
				}
				buf, err = makePackfileFromPushNote(repo, tx)
			})

			It("should return error about missing object", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to encoded push note to pack format: object not found"))
			})
		})
	})

	Describe(".MakeReferenceUpdateRequest", func() {
		var buf io.ReadSeeker
		var commitHash string

		BeforeEach(func() {
			testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
			commitHash, _ = repo.GetRecentCommitHash()
			tx := &core.PushNote{
				References: []*core.PushedReference{
					{Name: "refs/heads/master", OldHash: plumbing.ZeroHash.String(), NewHash: commitHash, Objects: []string{commitHash}},
				},
			}
			buf, err = MakeReferenceUpdateRequest(repo, tx)
		})

		It("should successfully return update request", func() {
			Expect(err).To(BeNil())
			Expect(buf).ToNot(BeNil())
			Expect(buf.(*bytes.Reader).Len() > 0).To(BeTrue())
		})
	})

	Describe("makePushNoteFromStateChange", func() {
		Context("branch changes", func() {
			When("an empty repository is updated with a new branch with 1 commit (with 1 file)", func() {
				var tx *core.PushNote
				var latestCommitHash string

				BeforeEach(func() {
					oldState := plumbing2.GetRepoState(repo)
					testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
					latestCommitHash = testutil2.GetRecentCommitHash(path, "refs/heads/master")
					newState := plumbing2.GetRepoState(repo)
					tx, err = makePushNoteFromStateChange(repo, oldState, newState)
				})

				It("should successfully return expected push note", func() {
					Expect(err).To(BeNil())
					Expect(tx.References).To(HaveLen(1))
					Expect(tx.References[0].Name).To(Equal("refs/heads/master"))
					Expect(tx.References[0].OldHash).To(Equal(plumbing.ZeroHash.String()))
					Expect(tx.References[0].NewHash).To(Equal(latestCommitHash))
					Expect(tx.References[0].Objects).To(HaveLen(3))
				})
			})

			When("a repo's old state has 1 branch, with 1 commit (1 file) and new state adds 1 commit (1 file)", func() {
				var tx *core.PushNote
				var oldCommitHash, latestCommitHash string

				BeforeEach(func() {
					testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
					oldCommitHash = testutil2.GetRecentCommitHash(path, "refs/heads/master")
					oldState := plumbing2.GetRepoState(repo)
					testutil2.AppendCommit(path, "file.txt", "some update", "commit updated")
					latestCommitHash = testutil2.GetRecentCommitHash(path, "refs/heads/master")
					newState := plumbing2.GetRepoState(repo)
					tx, err = makePushNoteFromStateChange(repo, oldState, newState)
				})

				It("should successfully return expected push note", func() {
					Expect(err).To(BeNil())
					Expect(tx.References).To(HaveLen(1))
					Expect(tx.References[0].Name).To(Equal("refs/heads/master"))
					Expect(tx.References[0].OldHash).To(Equal(oldCommitHash))
					Expect(tx.References[0].NewHash).To(Equal(latestCommitHash))
					Expect(tx.References[0].Objects).To(HaveLen(3))
				})
			})

			When("old state has 2 branches with 1 commit (1 file each); new state has only 1 branch with 1 commit (1 file)", func() {
				var tx *core.PushNote
				var oldCommitHash string

				BeforeEach(func() {
					testutil2.AppendCommit(path, "file.txt", "line 1", "commit 1 of master branch")
					testutil2.CreateCheckoutBranch(path, "branch2")
					testutil2.AppendCommit(path, "file.txt", "line 1", "commit 1 of branch 2")
					oldCommitHash = testutil2.GetRecentCommitHash(path, "refs/heads/branch2")
					oldState := plumbing2.GetRepoState(repo)
					testutil2.ExecGit(path, "update-ref", "-d", "refs/heads/branch2")
					newState := plumbing2.GetRepoState(repo)
					tx, err = makePushNoteFromStateChange(repo, oldState, newState)
				})

				It("should successfully return expected push note", func() {
					Expect(err).To(BeNil())
					Expect(tx.References).To(HaveLen(1))
					Expect(tx.References[0].Name).To(Equal("refs/heads/branch2"))
					Expect(tx.References[0].OldHash).To(Equal(oldCommitHash))
					Expect(tx.References[0].NewHash).To(Equal(plumbing.ZeroHash.String()))
					Expect(tx.References[0].Objects).To(HaveLen(0))
				})
			})

			When("old state has 1 branch with 1 commit (1 file each); new state has 0 branch", func() {
				var tx *core.PushNote
				var oldCommitHash string

				BeforeEach(func() {
					testutil2.AppendCommit(path, "file.txt", "line 1", "commit 1 of master branch")
					oldCommitHash = testutil2.GetRecentCommitHash(path, "refs/heads/master")
					oldState := plumbing2.GetRepoState(repo)
					testutil2.ExecGit(path, "update-ref", "-d", "refs/heads/master")
					newState := plumbing2.GetRepoState(repo)
					tx, err = makePushNoteFromStateChange(repo, oldState, newState)
				})

				It("should successfully return expected push note", func() {
					Expect(err).To(BeNil())
					Expect(tx.References).To(HaveLen(1))
					Expect(tx.References[0].Name).To(Equal("refs/heads/master"))
					Expect(tx.References[0].OldHash).To(Equal(oldCommitHash))
					Expect(tx.References[0].NewHash).To(Equal(plumbing.ZeroHash.String()))
					Expect(tx.References[0].Objects).To(HaveLen(0))
				})
			})
		})

		Context("annotated tag changes", func() {
			When("old state is empty; new state has 1 annotated tag and 1 commit (with 1 file)", func() {
				var tx *core.PushNote
				var newState core.BareRepoState

				BeforeEach(func() {
					oldState := plumbing2.GetRepoState(repo)
					testutil2.CreateCommitAndAnnotatedTag(path, "file.txt", "first file", "first commit", "v1")
					newState = plumbing2.GetRepoState(repo)
					tx, err = makePushNoteFromStateChange(repo, oldState, newState)
				})

				It("should successfully return expected push note", func() {
					Expect(err).To(BeNil())
					Expect(tx.References).To(HaveLen(2))
					newTag := newState.GetReferences().Get("refs/tags/v1")
					tag := tx.References.GetByName("refs/tags/v1")
					Expect(tag.OldHash).To(Equal(plumbing.ZeroHash.String()))
					Expect(tag.NewHash).To(Equal(newTag.GetData()))
					Expect(tag.Objects).To(HaveLen(4))
				})
			})

			When("old state has tag A; new state updates tag A", func() {
				var tx *core.PushNote
				var oldState core.BareRepoState

				BeforeEach(func() {
					testutil2.CreateCommitAndAnnotatedTag(path, "file.txt", "first file", "commit", "v1")
					oldState = plumbing2.GetRepoState(repo)
					testutil2.CreateCommitAndAnnotatedTag(path, "file.txt", "first file 2", "commit 2", "v1")
					newState := plumbing2.GetRepoState(repo)
					tx, err = makePushNoteFromStateChange(repo, oldState, newState)
				})

				It("should successfully return expected push note", func() {
					Expect(err).To(BeNil())
					Expect(tx.References).To(HaveLen(2))
					oldTag := oldState.GetReferences().Get("refs/tags/v1")
					tag := tx.References.GetByName("refs/tags/v1")
					Expect(tag.OldHash).To(Equal(oldTag.GetData()))
					Expect(tag.Objects).To(HaveLen(4))
				})
			})

			When("old state has tag A; new state deletes tag A", func() {
				var tx *core.PushNote
				var oldState core.BareRepoState

				BeforeEach(func() {
					testutil2.CreateCommitAndAnnotatedTag(path, "file.txt", "first file", "commit", "v1")
					oldState = plumbing2.GetRepoState(repo)
					testutil2.DeleteTag(path, "v1")
					newState := plumbing2.GetRepoState(repo)
					tx, err = makePushNoteFromStateChange(repo, oldState, newState)
				})

				It("should successfully return expected push note", func() {
					Expect(err).To(BeNil())
					Expect(tx.References).To(HaveLen(1))
					oldTag := oldState.GetReferences().Get("refs/tags/v1")
					tag := tx.References.GetByName("refs/tags/v1")
					Expect(tag.OldHash).To(Equal(oldTag.GetData()))
					Expect(tag.NewHash).To(Equal(plumbing.ZeroHash.String()))
					Expect(tag.Objects).To(HaveLen(0))
				})
			})
		})

		Context("lightweight tag changes", func() {
			When("old state is empty; new state has 1 tag and 1 commit (with 1 file)", func() {
				var tx *core.PushNote
				var newState core.BareRepoState

				BeforeEach(func() {
					oldState := plumbing2.GetRepoState(repo)
					testutil2.CreateCommitAndLightWeightTag(path, "file.txt", "first file", "first commit", "v1")
					newState = plumbing2.GetRepoState(repo)
					tx, err = makePushNoteFromStateChange(repo, oldState, newState)
				})

				It("should successfully return expected push note", func() {
					Expect(err).To(BeNil())
					Expect(tx.References).To(HaveLen(2))
					newTag := newState.GetReferences().Get("refs/tags/v1")
					tag := tx.References.GetByName("refs/tags/v1")
					Expect(tag.OldHash).To(Equal(plumbing.ZeroHash.String()))
					Expect(tag.NewHash).To(Equal(newTag.GetData()))
					Expect(tag.Objects).To(HaveLen(3))
				})
			})

			When("old state has tag A; new state updates tag A", func() {
				var tx *core.PushNote
				var oldState core.BareRepoState

				BeforeEach(func() {
					testutil2.CreateCommitAndLightWeightTag(path, "file.txt", "first file", "commit", "v1")
					oldState = plumbing2.GetRepoState(repo)
					testutil2.CreateCommitAndLightWeightTag(path, "file.txt", "first file 2", "commit 2", "v1")
					newState := plumbing2.GetRepoState(repo)
					tx, err = makePushNoteFromStateChange(repo, oldState, newState)
				})

				It("should successfully return expected push note", func() {
					Expect(err).To(BeNil())
					Expect(tx.References).To(HaveLen(2))
					oldTag := oldState.GetReferences().Get("refs/tags/v1")
					tag := tx.References.GetByName("refs/tags/v1")
					Expect(tag.OldHash).To(Equal(oldTag.GetData()))
					Expect(tag.Objects).To(HaveLen(3))
				})
			})

			When("old state has tag A; new state deletes tag A", func() {
				var tx *core.PushNote
				var oldState core.BareRepoState

				BeforeEach(func() {
					testutil2.CreateCommitAndLightWeightTag(path, "file.txt", "first file", "commit", "v1")
					oldState = plumbing2.GetRepoState(repo)
					testutil2.DeleteTag(path, "v1")
					newState := plumbing2.GetRepoState(repo)
					tx, err = makePushNoteFromStateChange(repo, oldState, newState)
				})

				It("should successfully return expected push note", func() {
					Expect(err).To(BeNil())
					Expect(tx.References).To(HaveLen(1))
					oldTag := oldState.GetReferences().Get("refs/tags/v1")
					tag := tx.References.GetByName("refs/tags/v1")
					Expect(tag.OldHash).To(Equal(oldTag.GetData()))
					Expect(tag.NewHash).To(Equal(plumbing.ZeroHash.String()))
					Expect(tag.Objects).To(HaveLen(0))
				})
			})
		})

		Context("note changes", func() {
			When("an empty repo is updated with a note and 1 commit (with 1 file)", func() {
				var tx *core.PushNote
				var newState core.BareRepoState

				BeforeEach(func() {
					oldState := plumbing2.GetRepoState(repo)
					testutil2.CreateCommitAndNote(path, "file.txt", "v1 file", "v1 commit", "note1")
					newState = plumbing2.GetRepoState(repo)
					tx, err = makePushNoteFromStateChange(repo, oldState, newState)
				})

				It("should successfully return update request", func() {
					Expect(err).To(BeNil())
					Expect(tx.References).To(HaveLen(2))
					note := tx.References.GetByName("refs/notes/note1")
					newRef := newState.GetReferences().Get("refs/notes/note1")
					Expect(note.OldHash).To(Equal(plumbing.ZeroHash.String()))
					Expect(note.NewHash).To(Equal(newRef.GetData()))
					Expect(note.Objects).To(HaveLen(3))
				})
			})

			When("repo has note A for commit A and note A is updated for commit B", func() {
				var tx *core.PushNote
				var newState, oldState core.BareRepoState

				BeforeEach(func() {
					testutil2.CreateCommitAndNote(path, "file.txt", "v1 file", "v1 commit", "noteA")
					oldState = plumbing2.GetRepoState(repo)
					testutil2.CreateCommitAndNote(path, "file.txt", "v2 file", "v2 commit", "noteA")
					newState = plumbing2.GetRepoState(repo)
					tx, err = makePushNoteFromStateChange(repo, oldState, newState)
				})

				It("should successfully return update request", func() {
					Expect(err).To(BeNil())
					Expect(tx.References).To(HaveLen(2))
					note := tx.References.GetByName("refs/notes/noteA")
					newRef := newState.GetReferences().Get("refs/notes/noteA")
					oldRef := oldState.GetReferences().Get("refs/notes/noteA")
					Expect(note.OldHash).To(Equal(oldRef.GetData()))
					Expect(note.NewHash).To(Equal(newRef.GetData()))
					Expect(note.Objects).To(HaveLen(4))
				})
			})

			When("repo has note A for commit A and note A's message is updated", func() {
				var tx *core.PushNote
				var newState, oldState core.BareRepoState

				BeforeEach(func() {
					testutil2.CreateCommitAndNote(path, "file.txt", "v1 file", "v1 commit", "noteA")
					oldState = plumbing2.GetRepoState(repo)
					testutil2.CreateNote(path, "msg updated", "noteA")
					newState = plumbing2.GetRepoState(repo)
					tx, err = makePushNoteFromStateChange(repo, oldState, newState)
				})

				It("should successfully return update request", func() {
					Expect(err).To(BeNil())
					Expect(tx.References).To(HaveLen(1))
					note := tx.References.GetByName("refs/notes/noteA")
					newRef := newState.GetReferences().Get("refs/notes/noteA")
					oldRef := oldState.GetReferences().Get("refs/notes/noteA")
					Expect(note.OldHash).To(Equal(oldRef.GetData()))
					Expect(note.NewHash).To(Equal(newRef.GetData()))
					Expect(note.Objects).To(HaveLen(3))
				})
			})

			When("old state has note A and new state has no note A", func() {
				var tx *core.PushNote
				var newState, oldState core.BareRepoState

				BeforeEach(func() {
					testutil2.CreateCommitAndNote(path, "file.txt", "v1 file", "v1 commit", "noteA")
					oldState = plumbing2.GetRepoState(repo)
					testutil2.DeleteNote(path, "refs/notes/noteA")
					newState = plumbing2.GetRepoState(repo)
					tx, err = makePushNoteFromStateChange(repo, oldState, newState)
				})

				It("should successfully return update request", func() {
					Expect(err).To(BeNil())
					Expect(tx.References).To(HaveLen(1))
					note := tx.References.GetByName("refs/notes/noteA")
					oldRef := oldState.GetReferences().Get("refs/notes/noteA")
					Expect(note.OldHash).To(Equal(oldRef.GetData()))
					Expect(note.NewHash).To(Equal(plumbing.ZeroHash.String()))
				})
			})
		})
	})
})
