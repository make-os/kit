package repo

import (
	"bytes"
	"io"
	"os"
	"path/filepath"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/testutil"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/format/packfile"
)

var _ = Describe("PushTx", func() {
	var pushTx *PushTx
	var cfg *config.AppConfig
	var repo types.BareRepo
	var path string
	var err error

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
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

	BeforeEach(func() {
		pushTx = &PushTx{
			RepoName:    "repo",
			NodeSig:     []byte("node_signer_sig"),
			PusherKeyID: "pk_id",
			References: []*types.PushedReference{
				{
					Nonce:        1,
					NewHash:      "new_object_hash",
					Name:         "refs/heads/master",
					OldHash:      "old_object_hash",
					Fee:          "0.2",
					AccountNonce: 2,
				},
			},
		}
	})

	Describe(".Bytes", func() {
		It("should return expected bytes", func() {
			Expect(pushTx.Bytes()).ToNot(HaveLen(0))
		})
	})

	Describe(".ID", func() {
		It("should return expected bytes", func() {
			Expect(len(pushTx.ID())).To(Equal(32))
		})
	})

	Describe(".TotalFee", func() {
		It("should return expected total fee", func() {
			Expect(pushTx.TotalFee()).To(Equal(util.String("0.2")))
		})

		It("should return expected total fee", func() {
			pushTx.References = append(pushTx.References, &types.PushedReference{
				Nonce:        1,
				NewHash:      "new_object_hash",
				Name:         "refs/heads/master",
				OldHash:      "old_object_hash",
				Fee:          "0.2",
				AccountNonce: 2,
			})
			Expect(pushTx.TotalFee()).To(Equal(util.String("0.4")))
		})
	})

	Describe(".LenMinusFee", func() {
		It("should return expected length without the fee fields", func() {
			lenFee := len(util.ObjectToBytes(pushTx.References[0].Fee))
			lenWithFee := pushTx.Len()
			expected := lenWithFee - uint64(lenFee)
			Expect(pushTx.LenMinusFee()).To(Equal(expected))
		})
	})

	Describe(".Len", func() {
		It("should return expected length", func() {
			Expect(pushTx.Len()).To(Equal(uint64(120)))
		})
	})

	Describe(".TxSize", func() {
		It("should be non-zero", func() {
			Expect(pushTx.TxSize()).ToNot(Equal(0))
		})
	})

	Describe(".OverallSize", func() {
		It("should be non-zero", func() {
			Expect(pushTx.BillableSize()).ToNot(Equal(0))
		})
	})

	Describe(".makePackfileFromPushTx", func() {
		var buf io.ReadSeeker
		var commitHash, commitHash2 string

		When("a commit hash is added to a reference object list", func() {
			BeforeEach(func() {
				appendCommit(path, "file.txt", "some text", "commit msg")
				commitHash, _ = repo.GetRecentCommit()
				tx := &PushTx{
					References: []*types.PushedReference{
						{Objects: []string{commitHash}},
					},
				}
				buf, err = makePackfileFromPushTx(repo, tx)
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
				appendCommit(path, "file.txt", "some text", "commit msg")
				commitHash, _ = repo.GetRecentCommit()
				appendCommit(path, "file.txt", "some text", "commit msg")
				commitHash2, _ = repo.GetRecentCommit()
				tx := &PushTx{
					References: []*types.PushedReference{
						{Objects: []string{commitHash, commitHash2}},
					},
				}
				buf, err = makePackfileFromPushTx(repo, tx)
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
				tx := &PushTx{
					References: []*types.PushedReference{
						{Objects: []string{commitHash}},
					},
				}
				buf, err = makePackfileFromPushTx(repo, tx)
			})

			It("should return error about missing object", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to encoded push tx to pack format: object not found"))
			})
		})
	})

	Describe(".makeReferenceUpdateRequest", func() {
		var buf io.ReadSeeker
		var commitHash string

		BeforeEach(func() {
			appendCommit(path, "file.txt", "some text", "commit msg")
			commitHash, _ = repo.GetRecentCommit()
			tx := &PushTx{
				References: []*types.PushedReference{
					{Name: "refs/heads/master", OldHash: plumbing.ZeroHash.String(), NewHash: commitHash, Objects: []string{commitHash}},
				},
			}
			buf, err = makeReferenceUpdateRequest(repo, tx)
		})

		It("should successfully return update request", func() {
			Expect(err).To(BeNil())
			Expect(buf).ToNot(BeNil())
			Expect(buf.(*bytes.Reader).Len() > 0).To(BeTrue())
		})
	})

	Describe("makePushTxFromStateChange", func() {
		Context("branch changes", func() {
			When("an empty repository is updated with a new branch with 1 commit (with 1 file)", func() {
				var tx *PushTx
				var latestCommitHash string

				BeforeEach(func() {
					oldState := getRepoState(repo)
					appendCommit(path, "file.txt", "some text", "commit msg")
					latestCommitHash = getRecentCommitHash(path, "refs/heads/master")
					newState := getRepoState(repo)
					tx, err = makePushTxFromStateChange(repo, oldState, newState)
				})

				It("should successfully return expected push tx", func() {
					Expect(err).To(BeNil())
					Expect(tx.References).To(HaveLen(1))
					Expect(tx.References[0].Name).To(Equal("refs/heads/master"))
					Expect(tx.References[0].OldHash).To(Equal(plumbing.ZeroHash.String()))
					Expect(tx.References[0].NewHash).To(Equal(latestCommitHash))
					Expect(tx.References[0].Objects).To(HaveLen(3))
				})
			})

			When("a repo's old state has 1 branch, with 1 commit (1 file) and new state adds 1 commit (1 file)", func() {
				var tx *PushTx
				var oldCommitHash, latestCommitHash string

				BeforeEach(func() {
					appendCommit(path, "file.txt", "some text", "commit msg")
					oldCommitHash = getRecentCommitHash(path, "refs/heads/master")
					oldState := getRepoState(repo)
					appendCommit(path, "file.txt", "some update", "commit updated")
					latestCommitHash = getRecentCommitHash(path, "refs/heads/master")
					newState := getRepoState(repo)
					tx, err = makePushTxFromStateChange(repo, oldState, newState)
				})

				It("should successfully return expected push tx", func() {
					Expect(err).To(BeNil())
					Expect(tx.References).To(HaveLen(1))
					Expect(tx.References[0].Name).To(Equal("refs/heads/master"))
					Expect(tx.References[0].OldHash).To(Equal(oldCommitHash))
					Expect(tx.References[0].NewHash).To(Equal(latestCommitHash))
					Expect(tx.References[0].Objects).To(HaveLen(3))
				})
			})

			When("old state has 2 branches with 1 commit (1 file each); new state has only 1 branch with 1 commit (1 file)", func() {
				var tx *PushTx
				var oldCommitHash string

				BeforeEach(func() {
					appendCommit(path, "file.txt", "line 1", "commit 1 of master branch")
					createCheckoutBranch(path, "branch2")
					appendCommit(path, "file.txt", "line 1", "commit 1 of branch 2")
					oldCommitHash = getRecentCommitHash(path, "refs/heads/branch2")
					oldState := getRepoState(repo)
					execGit(path, "update-ref", "-d", "refs/heads/branch2")
					newState := getRepoState(repo)
					tx, err = makePushTxFromStateChange(repo, oldState, newState)
				})

				It("should successfully return expected push tx", func() {
					Expect(err).To(BeNil())
					Expect(tx.References).To(HaveLen(1))
					Expect(tx.References[0].Name).To(Equal("refs/heads/branch2"))
					Expect(tx.References[0].OldHash).To(Equal(oldCommitHash))
					Expect(tx.References[0].NewHash).To(Equal(plumbing.ZeroHash.String()))
					Expect(tx.References[0].Objects).To(HaveLen(0))
				})
			})

			When("old state has 1 branch with 1 commit (1 file each); new state has 0 branch", func() {
				var tx *PushTx
				var oldCommitHash string

				BeforeEach(func() {
					appendCommit(path, "file.txt", "line 1", "commit 1 of master branch")
					oldCommitHash = getRecentCommitHash(path, "refs/heads/master")
					oldState := getRepoState(repo)
					execGit(path, "update-ref", "-d", "refs/heads/master")
					newState := getRepoState(repo)
					tx, err = makePushTxFromStateChange(repo, oldState, newState)
				})

				It("should successfully return expected push tx", func() {
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
				var tx *PushTx
				var newState types.BareRepoState

				BeforeEach(func() {
					oldState := getRepoState(repo)
					createCommitAndAnnotatedTag(path, "file.txt", "first file", "first commit", "v1")
					newState = getRepoState(repo)
					tx, err = makePushTxFromStateChange(repo, oldState, newState)
				})

				It("should successfully return expected push tx", func() {
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
				var tx *PushTx
				var oldState types.BareRepoState

				BeforeEach(func() {
					createCommitAndAnnotatedTag(path, "file.txt", "first file", "commit", "v1")
					oldState = getRepoState(repo)
					createCommitAndAnnotatedTag(path, "file.txt", "first file 2", "commit 2", "v1")
					newState := getRepoState(repo)
					tx, err = makePushTxFromStateChange(repo, oldState, newState)
				})

				It("should successfully return expected push tx", func() {
					Expect(err).To(BeNil())
					Expect(tx.References).To(HaveLen(2))
					oldTag := oldState.GetReferences().Get("refs/tags/v1")
					tag := tx.References.GetByName("refs/tags/v1")
					Expect(tag.OldHash).To(Equal(oldTag.GetData()))
					Expect(tag.Objects).To(HaveLen(4))
				})
			})

			When("old state has tag A; new state deletes tag A", func() {
				var tx *PushTx
				var oldState types.BareRepoState

				BeforeEach(func() {
					createCommitAndAnnotatedTag(path, "file.txt", "first file", "commit", "v1")
					oldState = getRepoState(repo)
					deleteTag(path, "v1")
					newState := getRepoState(repo)
					tx, err = makePushTxFromStateChange(repo, oldState, newState)
				})

				It("should successfully return expected push tx", func() {
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
				var tx *PushTx
				var newState types.BareRepoState

				BeforeEach(func() {
					oldState := getRepoState(repo)
					createCommitAndLightWeightTag(path, "file.txt", "first file", "first commit", "v1")
					newState = getRepoState(repo)
					tx, err = makePushTxFromStateChange(repo, oldState, newState)
				})

				It("should successfully return expected push tx", func() {
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
				var tx *PushTx
				var oldState types.BareRepoState

				BeforeEach(func() {
					createCommitAndLightWeightTag(path, "file.txt", "first file", "commit", "v1")
					oldState = getRepoState(repo)
					createCommitAndLightWeightTag(path, "file.txt", "first file 2", "commit 2", "v1")
					newState := getRepoState(repo)
					tx, err = makePushTxFromStateChange(repo, oldState, newState)
				})

				It("should successfully return expected push tx", func() {
					Expect(err).To(BeNil())
					Expect(tx.References).To(HaveLen(2))
					oldTag := oldState.GetReferences().Get("refs/tags/v1")
					tag := tx.References.GetByName("refs/tags/v1")
					Expect(tag.OldHash).To(Equal(oldTag.GetData()))
					Expect(tag.Objects).To(HaveLen(3))
				})
			})

			When("old state has tag A; new state deletes tag A", func() {
				var tx *PushTx
				var oldState types.BareRepoState

				BeforeEach(func() {
					createCommitAndLightWeightTag(path, "file.txt", "first file", "commit", "v1")
					oldState = getRepoState(repo)
					deleteTag(path, "v1")
					newState := getRepoState(repo)
					tx, err = makePushTxFromStateChange(repo, oldState, newState)
				})

				It("should successfully return expected push tx", func() {
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
				var tx *PushTx
				var newState types.BareRepoState

				BeforeEach(func() {
					oldState := getRepoState(repo)
					createCommitAndNote(path, "file.txt", "v1 file", "v1 commit", "note1")
					newState = getRepoState(repo)
					tx, err = makePushTxFromStateChange(repo, oldState, newState)
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
				var tx *PushTx
				var newState, oldState types.BareRepoState

				BeforeEach(func() {
					createCommitAndNote(path, "file.txt", "v1 file", "v1 commit", "noteA")
					oldState = getRepoState(repo)
					createCommitAndNote(path, "file.txt", "v2 file", "v2 commit", "noteA")
					newState = getRepoState(repo)
					tx, err = makePushTxFromStateChange(repo, oldState, newState)
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
				var tx *PushTx
				var newState, oldState types.BareRepoState

				BeforeEach(func() {
					createCommitAndNote(path, "file.txt", "v1 file", "v1 commit", "noteA")
					oldState = getRepoState(repo)
					createNote(path, "msg updated", "noteA")
					newState = getRepoState(repo)
					tx, err = makePushTxFromStateChange(repo, oldState, newState)
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
				var tx *PushTx
				var newState, oldState types.BareRepoState

				BeforeEach(func() {
					createCommitAndNote(path, "file.txt", "v1 file", "v1 commit", "noteA")
					oldState = getRepoState(repo)
					deleteNote(path, "refs/notes/noteA")
					newState = getRepoState(repo)
					tx, err = makePushTxFromStateChange(repo, oldState, newState)
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
