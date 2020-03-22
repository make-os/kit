package repo

import (
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	dhttypes "gitlab.com/makeos/mosdef/dht/types"
	tickettypes "gitlab.com/makeos/mosdef/ticket/types"
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"

	"github.com/golang/mock/gomock"
	"gitlab.com/makeos/mosdef/mocks"

	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/testutil"
	"gitlab.com/makeos/mosdef/util"
)

var _ = Describe("Validation", func() {
	var err error
	var cfg *config.AppConfig
	var repo core.BareRepo
	var path string
	var pushKeyID string
	var pubKey *crypto.PubKey
	var privKey, privKey2 *crypto.Key
	var ctrl *gomock.Controller
	var mockLogic *mocks.MockLogic
	var mockTickMgr *mocks.MockTicketManager
	var mockRepoKeeper *mocks.MockRepoKeeper
	var mockNSKeeper *mocks.MockNamespaceKeeper
	var mockPushKeyKeeper *mocks.MockPushKeyKeeper
	var mockAcctKeeper *mocks.MockAccountKeeper
	var mockSysKeeper *mocks.MockSystemKeeper
	var mockTxLogic *mocks.MockTxLogic

	var pushKeyGetter = func(pushKeyID string) (crypto.PublicKey, error) {
		return pubKey.ToPublicKey(), nil
	}

	var invalidPushKeyGetter = func(pushKeyID string) (crypto.PublicKey, error) {
		return crypto.BytesToPublicKey(util.RandBytes(32)), nil
	}

	var pushKeyGetterWithErr = func(err error) func(pushKeyID string) (crypto.PublicKey, error) {
		return func(pushKeyID string) (crypto.PublicKey, error) {
			return crypto.EmptyPublicKey, err
		}
	}

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"

		privKey = crypto.NewKeyFromIntSeed(1)
		privKey2 = crypto.NewKeyFromIntSeed(2)
		pushKeyID = privKey.PushAddr().String()
		pubKey = privKey.PubKey()
		Expect(err).To(BeNil())
		_ = pushKeyID
		_ = privKey2

		GitEnv = append(GitEnv, "GNUPGHOME="+cfg.DataDir())
		mockObjs := testutil.MockLogic(ctrl)
		mockLogic = mockObjs.Logic
		mockTickMgr = mockObjs.TicketManager
		mockRepoKeeper = mockObjs.RepoKeeper
		mockPushKeyKeeper = mockObjs.PushKeyKeeper
		mockAcctKeeper = mockObjs.AccountKeeper
		mockSysKeeper = mockObjs.SysKeeper
		mockTxLogic = mockObjs.Tx
		mockNSKeeper = mockObjs.NamespaceKeeper

		repoName := util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		execGit(cfg.GetRepoRoot(), "init", repoName)
		repo, err = getRepoWithGitOpt(cfg.Node.GitBinPath, path)
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".checkCommit", func() {
		var commit *object.Commit
		var err error

		When("commit does not include transaction parameters", func() {
			BeforeEach(func() {
				appendCommit(path, "file.txt", "line 1", "commit 1")
				commitHash, _ := repo.GetRecentCommit()
				commit, _ = repo.CommitObject(plumbing.NewHash(strings.TrimSpace(commitHash)))
				_, err = checkCommit(commit, repo, pushKeyGetter)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("transaction params was not set"))
			})
		})

		When("a commit transaction parameters contains invalid push key", func() {
			BeforeEach(func() {
				txParams := fmt.Sprintf("tx: fee=%s, nonce=%s, pkID=%s", "0", "0", "invalid_pk_id")
				appendCommit(path, "file.txt", "line 1", txParams)
				commitHash, _ := repo.GetRecentCommit()
				commit, _ = repo.CommitObject(plumbing.NewHash(strings.TrimSpace(commitHash)))
				_, err = checkCommit(commit, repo, pushKeyGetter)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("push key id is invalid"))
			})
		})

		When("the commit is not signed", func() {
			BeforeEach(func() {
				txParams := util.MakeTxParams("0", "0", pubKey.PushAddr().String(), nil)
				appendCommit(path, "file.txt", "line 1", txParams)
				commitHash, _ := repo.GetRecentCommit()
				commit, _ = repo.CommitObject(plumbing.NewHash(strings.TrimSpace(commitHash)))
				_, err = checkCommit(commit, repo, pushKeyGetter)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("commit .* was not signed"))
			})
		})

		When("commit is signed but unable to get public key using the pushKeyID", func() {
			BeforeEach(func() {
				txParams := util.MakeTxParams("0", "0", pubKey.PushAddr().String(), nil)
				appendCommit(path, "file.txt", "line 1", txParams)
				commitHash, _ := repo.GetRecentCommit()
				commit, _ = repo.CommitObject(plumbing.NewHash(strings.TrimSpace(commitHash)))
				commit.PGPSignature = "signature"
				_, err = checkCommit(commit, repo, pushKeyGetterWithErr(fmt.Errorf("bad error")))
			})

			It("should return err'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("failed to get push key (.*): bad error"))
			})
		})

		When("commit has a signature but the signature is malformed", func() {
			BeforeEach(func() {
				txParams := util.MakeTxParams("0", "0", pubKey.PushAddr().String(), nil)
				appendCommit(path, "file.txt", "line 1", txParams)
				commitHash, _ := repo.GetRecentCommit()
				commit, _ = repo.CommitObject(plumbing.NewHash(strings.TrimSpace(commitHash)))
				commit.PGPSignature = "signature"
				_, err = checkCommit(commit, repo, pushKeyGetter)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("signature is malformed"))
			})
		})

		When("commit has a signature but the signature is not valid", func() {
			BeforeEach(func() {
				txParams := util.MakeTxParams("0", "0", pubKey.PushAddr().String(), nil)
				appendCommit(path, "file.txt", "line 1", txParams)
				commitHash, _ := repo.GetRecentCommit()
				commit, _ = repo.CommitObject(plumbing.NewHash(strings.TrimSpace(commitHash)))
				commit.PGPSignature = string(pem.EncodeToMemory(&pem.Block{Bytes: []byte{1, 2, 3}, Type: "SIGNATURE"}))
				_, err = checkCommit(commit, repo, pushKeyGetter)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("commit (.*) signature is invalid"))
			})
		})

		When("commit has a signature and it is valid", func() {
			var err error
			var sig []byte
			BeforeEach(func() {
				txParams := util.MakeTxParams("0", "0", pubKey.PushAddr().String(), nil)
				appendCommit(path, "file.txt", "line 1", txParams)
				commitHash, _ := repo.GetRecentCommit()
				commit, _ = repo.CommitObject(plumbing.NewHash(commitHash))
				sigMsg := getCommitOrTagSigMsg(commit)
				sig, err = privKey.PrivKey().Sign([]byte(sigMsg))
				Expect(err).To(BeNil())
				commit.PGPSignature = string(pem.EncodeToMemory(&pem.Block{Bytes: sig, Type: "SIGNATURE"}))
				_, err = checkCommit(commit, repo, pushKeyGetter)
			})

			It("should return nil", func() {
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".checkAnnotatedTag", func() {
		var err error
		var tob *object.Tag

		When("txparams is not set", func() {
			BeforeEach(func() {
				createCommitAndAnnotatedTag(path, "file.txt", "first file", "commit 1", "v1")
				tagRef, _ := repo.Tag("v1")
				tob, _ = repo.TagObject(tagRef.Hash())
				_, err = checkAnnotatedTag(tob, repo, pushKeyGetter)
			})

			It("should return err='txparams was not set'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("tag (.*): transaction params was not set"))
			})
		})

		When("txparams.pkID is not valid", func() {
			BeforeEach(func() {
				txParams := util.MakeTxParams("0", "0", "invalid_pk_id", nil)
				createCommitAndAnnotatedTag(path, "file.txt", "first file", txParams, "v1")
				tagRef, _ := repo.Tag("v1")
				tob, _ = repo.TagObject(tagRef.Hash())
				_, err = checkAnnotatedTag(tob, repo, pushKeyGetter)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("tag (.*): field:pkID, msg:push key id is invalid"))
			})
		})

		When("tag is not signed", func() {
			BeforeEach(func() {
				txParams := util.MakeTxParams("0", "0", pubKey.PushAddr().String(), nil)
				createCommitAndAnnotatedTag(path, "file.txt", "first file", txParams, "v1")
				tagRef, _ := repo.Tag("v1")
				tob, _ = repo.TagObject(tagRef.Hash())
				_, err = checkAnnotatedTag(tob, repo, pushKeyGetter)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("tag (.*) is unsigned. Sign the tag with your push key"))
			})
		})

		When("tag is signed but unable to get public key using the pushKeyID", func() {
			BeforeEach(func() {
				txParams := util.MakeTxParams("0", "0", pubKey.PushAddr().String(), nil)
				createCommitAndAnnotatedTag(path, "file.txt", "first file", txParams, "v1")
				tagRef, _ := repo.Tag("v1")
				tob, _ = repo.TagObject(tagRef.Hash())
				tob.PGPSignature = "signature"
				_, err = checkAnnotatedTag(tob, repo, pushKeyGetterWithErr(fmt.Errorf("bad error")))
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("failed to get pusher key(.*) to verify commit .*"))
			})
		})

		When("tag has a signature but the signature is malformed", func() {
			BeforeEach(func() {
				txParams := util.MakeTxParams("0", "0", pubKey.PushAddr().String(), nil)
				createCommitAndAnnotatedTag(path, "file.txt", "first file", txParams, "v1")
				tagRef, _ := repo.Tag("v1")
				tob, _ = repo.TagObject(tagRef.Hash())
				tob.PGPSignature = "signature"
				_, err = checkAnnotatedTag(tob, repo, pushKeyGetter)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("signature is malformed"))
			})
		})

		When("tag has a signature but the signature is invalid", func() {
			BeforeEach(func() {
				txParams := util.MakeTxParams("0", "0", pubKey.PushAddr().String(), nil)
				createCommitAndAnnotatedTag(path, "file.txt", "first file", txParams, "v1")
				tagRef, _ := repo.Tag("v1")
				tob, _ = repo.TagObject(tagRef.Hash())
				sig := pem.EncodeToMemory(&pem.Block{Bytes: []byte("invalid sig"), Type: "SIGNATURE"})
				tob.PGPSignature = string(sig)
				_, err = checkAnnotatedTag(tob, repo, pushKeyGetter)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("commit (.*) signature is invalid"))
			})
		})

		When("tag has a valid signature but the referenced commit is unsigned", func() {
			BeforeEach(func() {
				txParams := util.MakeTxParams("0", "0", pubKey.PushAddr().String(), nil)
				createCommitAndAnnotatedTag(path, "file.txt", "first file", txParams, "v1")
				tagRef, _ := repo.Tag("v1")
				tob, _ = repo.TagObject(tagRef.Hash())

				msg := getCommitOrTagSigMsg(tob)
				sig, _ := privKey.PrivKey().Sign([]byte(msg))
				pemData := pem.EncodeToMemory(&pem.Block{Bytes: sig, Type: "SIGNATURE"})
				tob.PGPSignature = string(pemData)

				_, err = checkAnnotatedTag(tob, repo, pushKeyGetter)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("commit (.*) was not signed"))
			})
		})
	})

	Describe(".checkNote", func() {
		var err error

		When("target note does not exist", func() {
			BeforeEach(func() {
				_, err = checkNote(repo, "unknown", pushKeyGetter)
			})

			It("should return err='unable to fetch note entries (unknown)'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("unable to fetch note entries (unknown)"))
			})
		})

		When("a note does not have a tx blob object", func() {
			BeforeEach(func() {
				createCommitAndNote(path, "file.txt", "a file", "commit msg", "note1")
				_, err = checkNote(repo, "refs/notes/note1", pushKeyGetter)
			})

			It("should return err='note does not include a transaction parameters blob'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("note does not include a transaction parameters blob"))
			})
		})

		When("a notes tx blob has invalid transaction parameters format", func() {
			BeforeEach(func() {
				createCommitAndNote(path, "file.txt", "a file", "commit msg", "note1")
				createNoteEntry(path, "note1", "tx: fee=0 nonce=0 pushKeyID=push1wfx7vp8qfyv98cctvamqwec5xjrj48tpxaa77t sig=xyz")
				_, err = checkNote(repo, "refs/notes/note1", pushKeyGetter)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("note (refs/notes/note1) has invalid transaction parameters: field:pushKeyID, msg:unknown field"))
			})
		})

		When("a notes tx blob has invalid signature format", func() {
			BeforeEach(func() {
				createCommitAndNote(path, "file.txt", "a file", "commit msg", "note1")
				createNoteEntry(path, "note1", "tx: fee=0 nonce=0 pkID=push1wfx7vp8qfyv98cctvamqwec5xjrj48tpxaa77t sig=xyz")
				_, err = checkNote(repo, "refs/notes/note1", pushKeyGetter)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("note (refs/notes/note1) has invalid transaction " +
					"parameters: field:sig, msg:signature format is not valid"))
			})
		})

		When("a notes tx blob has an unknown public key id", func() {
			BeforeEach(func() {
				createCommitAndNote(path, "file.txt", "a file", "commit msg", "note1")
				createNoteEntry(path, "note1", "tx: fee=0 nonce=0 pkID=push1wfx7vp8qfyv98cctvamqwec5xjrj48tpxaa77t sig=0x616263")
				_, err = checkNote(repo, "refs/notes/note1", pushKeyGetterWithErr(fmt.Errorf("error finding pub key")))
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("failed to get push key (.*): error finding pub key"))
			})
		})

		When("a notes tx blob includes a public key id to an invalid public key", func() {
			BeforeEach(func() {
				createCommitAndNote(path, "file.txt", "a file", "commit msg", "note1")
				createNoteEntry(path, "note1", "tx: fee=0 nonce=0 pkID=push1wfx7vp8qfyv98cctvamqwec5xjrj48tpxaa77t sig=0x616263")
				_, err = checkNote(repo, "refs/notes/note1", invalidPushKeyGetter)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("note (refs/notes/note1) signature verification failed"))
			})
		})

		When("a note's signature is valid", func() {
			var sig []byte
			BeforeEach(func() {
				createCommitAndNote(path, "file.txt", "a file", "commit msg", "note1")
				commitHash := getRecentCommitHash(path, "refs/notes/note1")

				msg := MakeNoteSigMsg("0", "0", "push1wfx7vp8qfyv98cctvamqwec5xjrj48tpxaa77t", commitHash, false)
				sig, err = privKey.PrivKey().Sign(msg)
				Expect(err).To(BeNil())
				sigHex := util.ToHex(sig)

				createNoteEntry(path, "note1", "tx: fee=0 nonce=0 pkID=push1wfx7vp8qfyv98cctvamqwec5xjrj48tpxaa77t sig="+sigHex)
				_, err = checkNote(repo, "refs/notes/note1", pushKeyGetter)
			})

			It("should return nil", func() {
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".checkMergeCompliance", func() {
		When("pushed reference is not a branch", func() {
			BeforeEach(func() {
				repo := mocks.NewMockBareRepo(ctrl)
				change := &core.ItemChange{Item: &Obj{Name: "refs/others/name", Data: "stuff"}}
				oldRef := &Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}
				err = checkMergeCompliance(repo, change, oldRef, "0001", "push_key_id", mockLogic)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("merge error: pushed reference must be a branch"))
			})
		})

		When("target merge proposal does not exist", func() {
			BeforeEach(func() {
				repo := mocks.NewMockBareRepo(ctrl)
				repo.EXPECT().State().Return(state.BareRepository())
				change := &core.ItemChange{Item: &Obj{Name: "refs/heads/master", Data: "stuff"}}
				oldRef := &Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}
				err = checkMergeCompliance(repo, change, oldRef, "0001", "push_key_id", mockLogic)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("merge error: merge proposal (0001) not found"))
			})
		})

		When("signer did not create the proposal", func() {
			BeforeEach(func() {
				repo := mocks.NewMockBareRepo(ctrl)
				repoState := state.BareRepository()
				prop := state.BareRepoProposal()
				prop.Creator = "address_of_creator"
				repoState.Proposals.Add("0001", prop)
				repo.EXPECT().State().Return(repoState)

				mockPushKeyKeeper.EXPECT().Get("push_key_id").Return(&state.PushKey{Address: "address_xyz"})

				change := &core.ItemChange{Item: &Obj{Name: "refs/heads/master", Data: "stuff"}}
				oldRef := &Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}

				err = checkMergeCompliance(repo, change, oldRef, "0001", "push_key_id", mockLogic)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("merge error: signer did not create the proposal (0001)"))
			})
		})

		When("unable to check whether proposal is closed", func() {
			BeforeEach(func() {
				repo := mocks.NewMockBareRepo(ctrl)
				repo.EXPECT().GetName().Return("repo1")
				repoState := state.BareRepository()
				repoState.Proposals.Add("0001", state.BareRepoProposal())
				repo.EXPECT().State().Return(repoState)

				mockPushKeyKeeper.EXPECT().Get("push_key_id").Return(&state.PushKey{})

				change := &core.ItemChange{Item: &Obj{Name: "refs/heads/master", Data: "stuff"}}
				oldRef := &Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}

				mockRepoKeeper.EXPECT().IsProposalClosed("repo1", "0001").Return(false, fmt.Errorf("error"))

				err = checkMergeCompliance(repo, change, oldRef, "0001", "push_key_id", mockLogic)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("merge error: error"))
			})
		})

		When("target merge proposal is closed", func() {
			BeforeEach(func() {
				repo := mocks.NewMockBareRepo(ctrl)
				repo.EXPECT().GetName().Return("repo1")
				repoState := state.BareRepository()
				repoState.Proposals.Add("0001", state.BareRepoProposal())
				repo.EXPECT().State().Return(repoState)

				mockPushKeyKeeper.EXPECT().Get("push_key_id").Return(&state.PushKey{})

				change := &core.ItemChange{Item: &Obj{Name: "refs/heads/master", Data: "stuff"}}
				oldRef := &Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}

				mockRepoKeeper.EXPECT().IsProposalClosed("repo1", "0001").Return(true, nil)

				err = checkMergeCompliance(repo, change, oldRef, "0001", "push_key_id", mockLogic)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("merge error: merge proposal (0001) is already closed"))
			})
		})

		When("target merge proposal's base branch name does not match the pushed branch name", func() {
			BeforeEach(func() {
				repo := mocks.NewMockBareRepo(ctrl)
				repo.EXPECT().GetName().Return("repo1")
				repoState := state.BareRepository()
				prop := state.BareRepoProposal()
				prop.Outcome = state.ProposalOutcomeAccepted
				prop.ActionData = map[string][]byte{
					constants.ActionDataKeyBaseBranch: util.ToBytes("release"),
				}
				repoState.Proposals.Add("0001", prop)
				repo.EXPECT().State().Return(repoState)

				mockPushKeyKeeper.EXPECT().Get("push_key_id").Return(&state.PushKey{})

				change := &core.ItemChange{Item: &Obj{Name: "refs/heads/master", Data: "stuff"}}
				oldRef := &Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}

				mockRepoKeeper.EXPECT().IsProposalClosed("repo1", "0001").Return(false, nil)

				err = checkMergeCompliance(repo, change, oldRef, "0001", "push_key_id", mockLogic)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("merge error: pushed branch name and proposal base branch name must match"))
			})
		})

		When("target merge proposal outcome is not 'accepted'", func() {
			BeforeEach(func() {
				repo := mocks.NewMockBareRepo(ctrl)
				repo.EXPECT().GetName().Return("repo1")
				repoState := state.BareRepository()
				prop := state.BareRepoProposal()
				prop.ActionData = map[string][]byte{
					constants.ActionDataKeyBaseBranch: util.ToBytes("master"),
				}
				repoState.Proposals.Add("0001", prop)
				repo.EXPECT().State().Return(repoState)

				mockPushKeyKeeper.EXPECT().Get("push_key_id").Return(&state.PushKey{})

				change := &core.ItemChange{Item: &Obj{Name: "refs/heads/master", Data: "stuff"}}
				oldRef := &Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}

				mockRepoKeeper.EXPECT().IsProposalClosed("repo1", "0001").Return(false, nil)

				err = checkMergeCompliance(repo, change, oldRef, "0001", "push_key_id", mockLogic)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("merge error: merge proposal (0001) has not been accepted"))
			})
		})

		When("unable to get merger initiator commit", func() {
			BeforeEach(func() {
				repo := mocks.NewMockBareRepo(ctrl)
				repo.EXPECT().GetName().Return("repo1")
				repoState := state.BareRepository()
				prop := state.BareRepoProposal()
				prop.Outcome = state.ProposalOutcomeAccepted
				prop.ActionData = map[string][]byte{
					constants.ActionDataKeyBaseBranch: util.ToBytes("master"),
				}
				repoState.Proposals.Add("0001", prop)
				repo.EXPECT().State().Return(repoState)

				mockPushKeyKeeper.EXPECT().Get("push_key_id").Return(&state.PushKey{})

				change := &core.ItemChange{Item: &Obj{Name: "refs/heads/master", Data: "stuff"}}
				oldRef := &Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}
				repo.EXPECT().WrappedCommitObject(plumbing.NewHash(change.Item.GetData())).Return(nil, fmt.Errorf("error"))

				mockRepoKeeper.EXPECT().IsProposalClosed("repo1", "0001").Return(false, nil)

				err = checkMergeCompliance(repo, change, oldRef, "0001", "push_key_id", mockLogic)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("unable to get commit object: error"))
			})
		})

		When("unable to get merger initiator commit has more than 1 parents", func() {
			BeforeEach(func() {
				repo := mocks.NewMockBareRepo(ctrl)
				repo.EXPECT().GetName().Return("repo1")
				repoState := state.BareRepository()
				prop := state.BareRepoProposal()
				prop.Outcome = state.ProposalOutcomeAccepted
				prop.ActionData = map[string][]byte{
					constants.ActionDataKeyBaseBranch: util.ToBytes("master"),
				}
				repoState.Proposals.Add("0001", prop)
				repo.EXPECT().State().Return(repoState)

				mockPushKeyKeeper.EXPECT().Get("push_key_id").Return(&state.PushKey{})

				change := &core.ItemChange{Item: &Obj{Name: "refs/heads/master", Data: "stuff"}}
				oldRef := &Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}
				mergerCommit := mocks.NewMockCommit(ctrl)
				mergerCommit.EXPECT().NumParents().Return(2)
				repo.EXPECT().WrappedCommitObject(plumbing.NewHash(change.Item.GetData())).Return(mergerCommit, nil)

				mockRepoKeeper.EXPECT().IsProposalClosed("repo1", "0001").Return(false, nil)

				err = checkMergeCompliance(repo, change, oldRef, "0001", "push_key_id", mockLogic)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("merge error: multiple targets not allowed"))
			})
		})

		When("merger commit modified worktree history of parent", func() {
			When("tree hash is modified", func() {
				BeforeEach(func() {
					repo := mocks.NewMockBareRepo(ctrl)
					repo.EXPECT().GetName().Return("repo1")
					repoState := state.BareRepository()
					prop := state.BareRepoProposal()
					prop.Outcome = state.ProposalOutcomeAccepted
					prop.ActionData = map[string][]byte{
						constants.ActionDataKeyBaseBranch: util.ToBytes("master"),
					}
					repoState.Proposals.Add("0001", prop)
					repo.EXPECT().State().Return(repoState)

					mockPushKeyKeeper.EXPECT().Get("push_key_id").Return(&state.PushKey{})

					change := &core.ItemChange{Item: &Obj{Name: "refs/heads/master", Data: "stuff"}}
					oldRef := &Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}

					mergerCommit := mocks.NewMockCommit(ctrl)
					mergerCommit.EXPECT().NumParents().Return(1)
					treeHash := plumbing.ComputeHash(plumbing.CommitObject, util.RandBytes(20))
					mergerCommit.EXPECT().GetTreeHash().Return(treeHash)

					targetCommit := mocks.NewMockCommit(ctrl)
					treeHash = plumbing.ComputeHash(plumbing.CommitObject, util.RandBytes(20))
					targetCommit.EXPECT().GetTreeHash().Return(treeHash)

					mergerCommit.EXPECT().Parent(0).Return(targetCommit, nil)
					repo.EXPECT().WrappedCommitObject(plumbing.NewHash(change.Item.GetData())).Return(mergerCommit, nil)

					mockRepoKeeper.EXPECT().IsProposalClosed("repo1", "0001").Return(false, nil)

					err = checkMergeCompliance(repo, change, oldRef, "0001", "push_key_id", mockLogic)
				})

				It("should return error", func() {
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(Equal("merge error: merger commit cannot modify history as seen from target commit"))
				})
			})

			When("author is modified", func() {
				BeforeEach(func() {
					repo := mocks.NewMockBareRepo(ctrl)
					repo.EXPECT().GetName().Return("repo1")
					repoState := state.BareRepository()
					prop := state.BareRepoProposal()
					prop.Outcome = state.ProposalOutcomeAccepted
					prop.ActionData = map[string][]byte{
						constants.ActionDataKeyBaseBranch: util.ToBytes("master"),
					}
					repoState.Proposals.Add("0001", prop)
					repo.EXPECT().State().Return(repoState)

					mockPushKeyKeeper.EXPECT().Get("push_key_id").Return(&state.PushKey{})

					change := &core.ItemChange{Item: &Obj{Name: "refs/heads/master", Data: "stuff"}}
					oldRef := &Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}

					mergerCommit := mocks.NewMockCommit(ctrl)
					mergerCommit.EXPECT().NumParents().Return(1)
					treeHash := plumbing.ComputeHash(plumbing.CommitObject, []byte("hash"))
					mergerCommit.EXPECT().GetTreeHash().Return(treeHash)
					author := &object.Signature{Name: "author1", Email: "author@email.com"}
					mergerCommit.EXPECT().GetAuthor().Return(author)

					targetCommit := mocks.NewMockCommit(ctrl)
					treeHash = plumbing.ComputeHash(plumbing.CommitObject, []byte("hash"))
					targetCommit.EXPECT().GetTreeHash().Return(treeHash)
					author = &object.Signature{Name: "author1", Email: "author2@email.com"}
					targetCommit.EXPECT().GetAuthor().Return(author)

					mergerCommit.EXPECT().Parent(0).Return(targetCommit, nil)
					repo.EXPECT().WrappedCommitObject(plumbing.NewHash(change.Item.GetData())).Return(mergerCommit, nil)

					mockRepoKeeper.EXPECT().IsProposalClosed("repo1", "0001").Return(false, nil)

					err = checkMergeCompliance(repo, change, oldRef, "0001", "push_key_id", mockLogic)
				})

				It("should return error", func() {
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(Equal("merge error: merger commit cannot modify history as seen from target commit"))
				})
			})

			When("committer is modified", func() {
				BeforeEach(func() {
					repo := mocks.NewMockBareRepo(ctrl)
					repo.EXPECT().GetName().Return("repo1")
					repoState := state.BareRepository()
					prop := state.BareRepoProposal()
					prop.Outcome = state.ProposalOutcomeAccepted
					prop.ActionData = map[string][]byte{
						constants.ActionDataKeyBaseBranch: util.ToBytes("master"),
					}
					repoState.Proposals.Add("0001", prop)
					repo.EXPECT().State().Return(repoState)

					mockPushKeyKeeper.EXPECT().Get("push_key_id").Return(&state.PushKey{})

					change := &core.ItemChange{Item: &Obj{Name: "refs/heads/master", Data: "stuff"}}
					oldRef := &Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}

					mergerCommit := mocks.NewMockCommit(ctrl)
					mergerCommit.EXPECT().NumParents().Return(1)
					treeHash := plumbing.ComputeHash(plumbing.CommitObject, []byte("hash"))
					mergerCommit.EXPECT().GetTreeHash().Return(treeHash)
					author := &object.Signature{Name: "author1", Email: "author@email.com"}
					mergerCommit.EXPECT().GetAuthor().Return(author)
					committer := &object.Signature{Name: "committer1", Email: "committer@email.com"}
					mergerCommit.EXPECT().GetCommitter().Return(committer)

					targetCommit := mocks.NewMockCommit(ctrl)
					treeHash = plumbing.ComputeHash(plumbing.CommitObject, []byte("hash"))
					targetCommit.EXPECT().GetTreeHash().Return(treeHash)
					author = &object.Signature{Name: "author1", Email: "author@email.com"}
					targetCommit.EXPECT().GetAuthor().Return(author)
					committer = &object.Signature{Name: "committer1", Email: "committer2@email.com"}
					targetCommit.EXPECT().GetCommitter().Return(committer)

					mergerCommit.EXPECT().Parent(0).Return(targetCommit, nil)
					repo.EXPECT().WrappedCommitObject(plumbing.NewHash(change.Item.GetData())).Return(mergerCommit, nil)

					mockRepoKeeper.EXPECT().IsProposalClosed("repo1", "0001").Return(false, nil)

					err = checkMergeCompliance(repo, change, oldRef, "0001", "push_key_id", mockLogic)
				})

				It("should return error", func() {
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(Equal("merge error: merger commit cannot modify history as seen from target commit"))
				})
			})
		})

		When("old pushed branch hash is different from old branch hash described in the merge proposal", func() {
			BeforeEach(func() {
				repo := mocks.NewMockBareRepo(ctrl)
				repo.EXPECT().GetName().Return("repo1")
				repoState := state.BareRepository()
				prop := state.BareRepoProposal()
				prop.Outcome = state.ProposalOutcomeAccepted
				prop.ActionData = map[string][]byte{
					constants.ActionDataKeyBaseBranch: util.ToBytes("master"),
					constants.ActionDataKeyBaseHash:   util.ToBytes("xyz"),
				}
				repoState.Proposals.Add("0001", prop)
				repo.EXPECT().State().Return(repoState)

				mockPushKeyKeeper.EXPECT().Get("push_key_id").Return(&state.PushKey{})

				change := &core.ItemChange{Item: &Obj{Name: "refs/heads/master", Data: "stuff"}}
				oldRef := &Obj{Name: "refs/heads/unknown", Data: "abc"}

				mergerCommit := mocks.NewMockCommit(ctrl)
				mergerCommit.EXPECT().NumParents().Return(1)
				treeHash := plumbing.ComputeHash(plumbing.CommitObject, []byte("hash"))
				mergerCommit.EXPECT().GetTreeHash().Return(treeHash)
				author := &object.Signature{Name: "author1", Email: "author@email.com"}
				mergerCommit.EXPECT().GetAuthor().Return(author)
				committer := &object.Signature{Name: "committer1", Email: "committer@email.com"}
				mergerCommit.EXPECT().GetCommitter().Return(committer)

				targetCommit := mocks.NewMockCommit(ctrl)
				treeHash = plumbing.ComputeHash(plumbing.CommitObject, []byte("hash"))
				targetCommit.EXPECT().GetTreeHash().Return(treeHash)
				author = &object.Signature{Name: "author1", Email: "author@email.com"}
				targetCommit.EXPECT().GetAuthor().Return(author)
				committer = &object.Signature{Name: "committer1", Email: "committer@email.com"}
				targetCommit.EXPECT().GetCommitter().Return(committer)

				mergerCommit.EXPECT().Parent(0).Return(targetCommit, nil)
				repo.EXPECT().WrappedCommitObject(plumbing.NewHash(change.Item.GetData())).Return(mergerCommit, nil)

				mockRepoKeeper.EXPECT().IsProposalClosed("repo1", "0001").Return(false, nil)

				err = checkMergeCompliance(repo, change, oldRef, "0001", "push_key_id", mockLogic)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("merge error: merge proposal base branch hash is stale or invalid"))
			})
		})

		When("merge proposal target hash does not match the expected target hash", func() {
			BeforeEach(func() {
				repo := mocks.NewMockBareRepo(ctrl)
				repo.EXPECT().GetName().Return("repo1")
				repoState := state.BareRepository()
				prop := state.BareRepoProposal()
				prop.Outcome = state.ProposalOutcomeAccepted
				prop.ActionData = map[string][]byte{
					constants.ActionDataKeyBaseBranch: util.ToBytes("master"),
					constants.ActionDataKeyBaseHash:   util.ToBytes("abc"),
					constants.ActionDataKeyTargetHash: util.ToBytes("target_xyz"),
				}
				repoState.Proposals.Add("0001", prop)
				repo.EXPECT().State().Return(repoState)

				mockPushKeyKeeper.EXPECT().Get("push_key_id").Return(&state.PushKey{})

				change := &core.ItemChange{Item: &Obj{Name: "refs/heads/master", Data: "stuff"}}
				oldRef := &Obj{Name: "refs/heads/unknown", Data: "abc"}

				mergerCommit := mocks.NewMockCommit(ctrl)
				mergerCommit.EXPECT().NumParents().Return(1)
				treeHash := plumbing.ComputeHash(plumbing.CommitObject, []byte("hash"))
				mergerCommit.EXPECT().GetTreeHash().Return(treeHash)
				author := &object.Signature{Name: "author1", Email: "author@email.com"}
				mergerCommit.EXPECT().GetAuthor().Return(author)
				committer := &object.Signature{Name: "committer1", Email: "committer@email.com"}
				mergerCommit.EXPECT().GetCommitter().Return(committer)

				targetCommit := mocks.NewMockCommit(ctrl)
				targetHash := plumbing.ComputeHash(plumbing.CommitObject, []byte("target_abc"))
				targetCommit.EXPECT().GetHash().Return(targetHash)
				treeHash = plumbing.ComputeHash(plumbing.CommitObject, []byte("hash"))
				targetCommit.EXPECT().GetTreeHash().Return(treeHash)
				author = &object.Signature{Name: "author1", Email: "author@email.com"}
				targetCommit.EXPECT().GetAuthor().Return(author)
				committer = &object.Signature{Name: "committer1", Email: "committer@email.com"}
				targetCommit.EXPECT().GetCommitter().Return(committer)

				mergerCommit.EXPECT().Parent(0).Return(targetCommit, nil)
				repo.EXPECT().WrappedCommitObject(plumbing.NewHash(change.Item.GetData())).Return(mergerCommit, nil)

				mockRepoKeeper.EXPECT().IsProposalClosed("repo1", "0001").Return(false, nil)

				err = checkMergeCompliance(repo, change, oldRef, "0001", "push_key_id", mockLogic)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("merge error: target commit " +
					"hash and the merge proposal target hash must match"))
			})
		})
	})

	Describe(".validateChange", func() {
		var err error

		When("change item has a reference name format that is not known", func() {
			BeforeEach(func() {
				change := &core.ItemChange{Item: &Obj{Name: "refs/others/name", Data: "stuff"}}
				_, err = validateChange(repo, change, pushKeyGetter)
			})

			It("should return err='unrecognised change item'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("unrecognised change item"))
			})
		})

		When("change item referenced object is an unknown commit object", func() {
			BeforeEach(func() {
				change := &core.ItemChange{Item: &Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}}
				_, err = validateChange(repo, change, pushKeyGetter)
			})

			It("should return err='unable to get commit object: object not found'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("unable to get commit object: object not found"))
			})
		})

		When("change item referenced object is an unknown tag object", func() {
			BeforeEach(func() {
				change := &core.ItemChange{Item: &Obj{Name: "refs/tags/unknown", Data: "unknown_hash"}}
				_, err = validateChange(repo, change, pushKeyGetter)
			})

			It("should return err='unable to get tag object: tag not found'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("unable to get tag object: tag not found"))
			})
		})
	})

	Describe(".CheckPushOK", func() {
		It("should return error when push note id is not set", func() {
			err := CheckPushOK(&core.PushOK{}, -1)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("field:endorsements.pushNoteID, msg:push note id is required"))
		})

		It("should return error when public key is not valid", func() {
			err := CheckPushOK(&core.PushOK{
				PushNoteID:   util.StrToBytes32("id"),
				SenderPubKey: util.EmptyBytes32,
			}, -1)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("field:endorsements.senderPubKey, msg:sender public key is required"))
		})
	})

	Describe(".CheckPushOKConsistency", func() {
		When("unable to fetch top hosts", func() {
			BeforeEach(func() {
				mockTickMgr.EXPECT().GetTopHosts(gomock.Any()).Return(nil, fmt.Errorf("error"))
				err = CheckPushOKConsistency(&core.PushOK{
					PushNoteID:   util.StrToBytes32("id"),
					SenderPubKey: util.EmptyBytes32,
				}, mockLogic, false, -1)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to get top hosts: error"))
			})
		})

		When("sender is not a host", func() {
			BeforeEach(func() {
				key := crypto.NewKeyFromIntSeed(1)
				mockTickMgr.EXPECT().GetTopHosts(gomock.Any()).Return([]*tickettypes.SelectedTicket{}, nil)
				err = CheckPushOKConsistency(&core.PushOK{
					PushNoteID:   util.StrToBytes32("id"),
					SenderPubKey: key.PubKey().MustBytes32(),
				}, mockLogic, false, -1)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:endorsements.senderPubKey, msg:sender public key does not belong to an active host"))
			})
		})

		When("unable to decode host's BLS public key", func() {
			BeforeEach(func() {
				key := crypto.NewKeyFromIntSeed(1)
				mockTickMgr.EXPECT().GetTopHosts(gomock.Any()).Return([]*tickettypes.SelectedTicket{
					{
						Ticket: &tickettypes.Ticket{
							ProposerPubKey: key.PubKey().MustBytes32(),
							BLSPubKey:      util.RandBytes(128),
						},
					},
				}, nil)
				err = CheckPushOKConsistency(&core.PushOK{
					PushNoteID:   util.StrToBytes32("id"),
					SenderPubKey: key.PubKey().MustBytes32(),
				}, mockLogic, false, -1)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("failed to decode bls public key of endorser"))
			})
		})

		When("unable to verify signature", func() {
			BeforeEach(func() {
				key := crypto.NewKeyFromIntSeed(1)
				key2 := crypto.NewKeyFromIntSeed(2)
				mockTickMgr.EXPECT().GetTopHosts(gomock.Any()).Return([]*tickettypes.SelectedTicket{
					{
						Ticket: &tickettypes.Ticket{
							ProposerPubKey: key.PubKey().MustBytes32(),
							BLSPubKey:      key2.PrivKey().BLSKey().Public().Bytes(),
						},
					},
				}, nil)
				err = CheckPushOKConsistency(&core.PushOK{
					PushNoteID:   util.StrToBytes32("id"),
					SenderPubKey: key.PubKey().MustBytes32(),
				}, mockLogic, false, -1)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("field:endorsements.sig, msg:signature could not be verified"))
			})
		})

		When("noSigCheck is true", func() {
			BeforeEach(func() {
				key := crypto.NewKeyFromIntSeed(1)
				key2 := crypto.NewKeyFromIntSeed(2)
				mockTickMgr.EXPECT().GetTopHosts(gomock.Any()).Return([]*tickettypes.SelectedTicket{
					{
						Ticket: &tickettypes.Ticket{
							ProposerPubKey: key.PubKey().MustBytes32(),
							BLSPubKey:      key2.PrivKey().BLSKey().Public().Bytes(),
						},
					},
				}, nil)
				err = CheckPushOKConsistency(&core.PushOK{
					PushNoteID:   util.StrToBytes32("id"),
					SenderPubKey: key.PubKey().MustBytes32(),
				}, mockLogic, true, -1)
			})

			It("should not check signature", func() {
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".CheckPushNoteSyntax", func() {
		key := crypto.NewKeyFromIntSeed(1)
		okTx := &core.PushNote{RepoName: "repo", PushKeyID: util.RandBytes(20), Timestamp: time.Now().Unix(), NodePubKey: key.PubKey().MustBytes32()}
		bz, _ := key.PrivKey().Sign(okTx.Bytes())
		okTx.NodeSig = bz

		var cases = [][]interface{}{
			{&core.PushNote{}, "field:repoName, msg:repo name is required"},
			{&core.PushNote{RepoName: "repo"}, "field:pusherKeyId, msg:push key id is required"},
			{&core.PushNote{RepoName: "re*&po"}, "field:repoName, msg:repo name is not valid"},
			{&core.PushNote{RepoName: "repo", Namespace: "*&ns"}, "field:namespace, msg:namespace is not valid"},
			{&core.PushNote{RepoName: "repo", PushKeyID: []byte("xyz")}, "field:pusherKeyId, msg:push key id is not valid"},
			{&core.PushNote{RepoName: "repo", PushKeyID: util.RandBytes(20), Timestamp: 0}, "field:timestamp, msg:timestamp is required"},
			{&core.PushNote{RepoName: "repo", PushKeyID: util.RandBytes(20), Timestamp: 2000000000}, "field:timestamp, msg:timestamp cannot be a future time"},
			{&core.PushNote{RepoName: "repo", PushKeyID: util.RandBytes(20), Timestamp: time.Now().Unix()}, "field:accountNonce, msg:account nonce must be greater than zero"},
			{&core.PushNote{RepoName: "repo", PushKeyID: util.RandBytes(20), Timestamp: time.Now().Unix(), PusherAcctNonce: 1, Fee: ""}, "field:fee, msg:fee is required"},
			{&core.PushNote{RepoName: "repo", PushKeyID: util.RandBytes(20), Timestamp: time.Now().Unix(), PusherAcctNonce: 1, Fee: "one"}, "field:fee, msg:fee must be numeric"},
			{&core.PushNote{RepoName: "repo", PushKeyID: util.RandBytes(20), Timestamp: time.Now().Unix(), PusherAcctNonce: 1, Fee: "1"}, "field:nodePubKey, msg:push node public key is required"},
			{&core.PushNote{RepoName: "repo", PushKeyID: util.RandBytes(20), Timestamp: time.Now().Unix(), PusherAcctNonce: 1, Fee: "1", NodePubKey: key.PubKey().MustBytes32()}, "field:nodeSig, msg:push node signature is required"},
			{&core.PushNote{RepoName: "repo", PushKeyID: util.RandBytes(20), Timestamp: time.Now().Unix(), PusherAcctNonce: 1, Fee: "1", NodePubKey: key.PubKey().MustBytes32(), NodeSig: []byte("invalid signature")}, "field:nodeSig, msg:failed to verify signature"},
			{&core.PushNote{RepoName: "repo", References: []*core.PushedReference{{}}}, "index:0, field:references.name, msg:name is required"},
			{&core.PushNote{RepoName: "repo", References: []*core.PushedReference{{Name: "ref1"}}}, "index:0, field:references.oldHash, msg:old hash is required"},
			{&core.PushNote{RepoName: "repo", References: []*core.PushedReference{{Name: "ref1", OldHash: "invalid"}}}, "index:0, field:references.oldHash, msg:old hash is not valid"},
			{&core.PushNote{RepoName: "repo", References: []*core.PushedReference{{Name: "ref1", OldHash: util.RandString(40)}}}, "index:0, field:references.newHash, msg:new hash is required"},
			{&core.PushNote{RepoName: "repo", References: []*core.PushedReference{{Name: "ref1", OldHash: util.RandString(40), NewHash: "invalid"}}}, "index:0, field:references.newHash, msg:new hash is not valid"},
			{&core.PushNote{RepoName: "repo", References: []*core.PushedReference{{Name: "ref1", OldHash: util.RandString(40), NewHash: util.RandString(40)}}}, "index:0, field:references.nonce, msg:reference nonce must be greater than zero"},
			{&core.PushNote{RepoName: "repo", References: []*core.PushedReference{{Name: "ref1", OldHash: util.RandString(40), NewHash: util.RandString(40), Nonce: 1, Objects: []string{"invalid object"}}}}, "index:0, field:references.objects.0, msg:object hash is not valid"},
		}

		It("should check cases", func() {
			for _, c := range cases {
				_c := c
				if _c[1] != nil {
					Expect(CheckPushNoteSyntax(_c[0].(*core.PushNote)).Error()).To(Equal(_c[1]))
				} else {
					Expect(CheckPushNoteSyntax(_c[0].(*core.PushNote))).To(BeNil())
				}
			}
		})
	})

	Describe(".checkPushedReference", func() {
		var mockKeepers *mocks.MockKeepers
		var mockRepo *mocks.MockBareRepo
		var oldHash = fmt.Sprintf("%x", util.Hash20(util.RandBytes(16)))

		BeforeEach(func() {
			mockKeepers = mocks.NewMockKeepers(ctrl)
			mockRepo = mocks.NewMockBareRepo(ctrl)
		})

		When("old hash is non-zero and pushed reference does not exist", func() {
			BeforeEach(func() {
				refs := []*core.PushedReference{
					{Name: "refs/heads/master", OldHash: oldHash},
				}
				repository := &state.Repository{
					References: map[string]*state.Reference{},
				}
				err = checkPushedReference(mockRepo, refs, repository, mockKeepers)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("index:0, field:references, msg:reference 'refs/heads/master' is unknown"))
			})
		})

		When("old hash is zero and pushed reference does not exist", func() {
			BeforeEach(func() {
				refs := []*core.PushedReference{
					{Name: "refs/heads/master", OldHash: strings.Repeat("0", 40)},
				}
				repository := &state.Repository{
					References: map[string]*state.Reference{},
				}
				err = checkPushedReference(mockRepo, refs, repository, mockKeepers)
			})

			It("should not return error about unknown reference", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).ToNot(Equal("index:0, field:references, msg:reference 'refs/heads/master' is unknown"))
			})
		})

		When("old hash of reference is different from the local hash of same reference", func() {
			BeforeEach(func() {
				refName := "refs/heads/master"
				refs := []*core.PushedReference{
					{Name: refName, OldHash: oldHash},
				}
				repository := &state.Repository{
					References: map[string]*state.Reference{
						refName: {Nonce: 0},
					},
				}
				mockRepo.EXPECT().Reference(plumbing.ReferenceName(refName), false).
					Return(plumbing.NewReferenceFromStrings("", util.RandString(40)), nil)

				err = checkPushedReference(mockRepo, refs, repository, mockKeepers)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("index:0, field:references, msg:reference 'refs/heads/master' old hash does not match its local version"))
			})
		})

		When("old hash of reference is non-zero and the local equivalent reference is not accessible", func() {
			BeforeEach(func() {
				refName := "refs/heads/master"
				refs := []*core.PushedReference{
					{Name: refName, OldHash: oldHash},
				}
				repository := &state.Repository{
					References: map[string]*state.Reference{
						refName: {Nonce: 0},
					},
				}
				mockRepo.EXPECT().Reference(plumbing.ReferenceName(refName), false).
					Return(nil, plumbing.ErrReferenceNotFound)

				err = checkPushedReference(mockRepo, refs, repository, mockKeepers)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("index:0, field:references, msg:reference 'refs/heads/master' does not exist locally"))
			})
		})

		When("old hash of reference is non-zero and nil repo passed", func() {
			BeforeEach(func() {
				refName := "refs/heads/master"
				refs := []*core.PushedReference{
					{Name: refName, OldHash: oldHash},
				}
				repository := &state.Repository{
					References: map[string]*state.Reference{
						refName: {Nonce: 0},
					},
				}

				err = checkPushedReference(nil, refs, repository, mockKeepers)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).ToNot(Equal("index:0, field:references, msg:reference 'refs/heads/master' does not exist locally"))
			})
		})

		When("old hash of reference is non-zero and it is different from the hash of the equivalent local reference", func() {
			BeforeEach(func() {
				refName := "refs/heads/master"
				refs := []*core.PushedReference{
					{Name: refName, OldHash: oldHash},
				}
				repository := &state.Repository{
					References: map[string]*state.Reference{
						refName: {Nonce: 0},
					},
				}
				mockRepo.EXPECT().Reference(plumbing.ReferenceName(refName), false).
					Return(plumbing.NewReferenceFromStrings("", util.RandString(40)), nil)

				err = checkPushedReference(mockRepo, refs, repository, mockKeepers)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("index:0, field:references, msg:reference 'refs/heads/master' old hash does not match its local version"))
			})
		})

		When("pushed reference nonce is unexpected", func() {
			BeforeEach(func() {
				refName := "refs/heads/master"
				newHash := util.RandString(40)
				refs := []*core.PushedReference{
					{
						Name:    refName,
						OldHash: oldHash,
						NewHash: newHash,
						Objects: []string{newHash},
						Nonce:   2,
					},
				}

				repository := &state.Repository{
					References: map[string]*state.Reference{
						refName: {Nonce: 0},
					},
				}

				mockRepo.EXPECT().
					Reference(plumbing.ReferenceName(refName), false).
					Return(plumbing.NewHashReference("", plumbing.NewHash(oldHash)), nil)

				err = checkPushedReference(mockRepo, refs, repository, mockKeepers)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("index:0, field:references, msg:reference 'refs/heads/master' has nonce '2', expecting '1'"))
			})
		})

		When("no validation error", func() {
			BeforeEach(func() {
				refName := "refs/heads/master"
				newHash := util.RandString(40)
				refs := []*core.PushedReference{
					{
						Name:    refName,
						OldHash: oldHash,
						NewHash: newHash,
						Objects: []string{newHash},
						Nonce:   1,
					},
				}

				repository := &state.Repository{
					References: map[string]*state.Reference{
						refName: {Nonce: 0},
					},
				}

				mockRepo.EXPECT().
					Reference(plumbing.ReferenceName(refName), false).
					Return(plumbing.NewHashReference("", plumbing.NewHash(oldHash)), nil)

				err = checkPushedReference(mockRepo, refs, repository, mockKeepers)
			})

			It("should return err", func() {
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".CheckPushNoteConsistency", func() {

		When("no repository with matching name exist", func() {
			BeforeEach(func() {
				tx := &core.PushNote{RepoName: "unknown"}
				mockRepoKeeper.EXPECT().Get(tx.RepoName).Return(state.BareRepository())
				err = CheckPushNoteConsistency(tx, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:repoName, msg:repository named 'unknown' is unknown"))
			})
		})

		When("namespace is set but does not exist", func() {
			BeforeEach(func() {
				tx := &core.PushNote{Namespace: "ns1"}
				mockRepoKeeper.EXPECT().Get(gomock.Any()).Return(&state.Repository{Balance: "10"})
				mockNSKeeper.EXPECT().Get(util.HashNamespace(tx.Namespace)).Return(state.BareNamespace())
				err = CheckPushNoteConsistency(tx, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:namespace, msg:namespace 'ns1' is unknown"))
			})
		})

		When("pusher public key id is unknown", func() {
			BeforeEach(func() {
				tx := &core.PushNote{RepoName: "repo1", PushKeyID: util.RandBytes(20)}
				mockRepoKeeper.EXPECT().Get(tx.RepoName).Return(&state.Repository{Balance: "10"})
				mockPushKeyKeeper.EXPECT().Get(crypto.BytesToPushKeyID(tx.PushKeyID)).Return(state.BarePushKey())
				err = CheckPushNoteConsistency(tx, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("field:pusherKeyId, msg:pusher's public key id '.*' is unknown"))
			})
		})

		When("push owner address not the same as the pusher address", func() {
			BeforeEach(func() {
				tx := &core.PushNote{
					RepoName:      "repo1",
					PushKeyID:     util.RandBytes(20),
					PusherAddress: "address1",
				}
				mockRepoKeeper.EXPECT().Get(tx.RepoName).Return(&state.Repository{Balance: "10"})

				pushKey := state.BarePushKey()
				pushKey.Address = util.Address("address2")
				mockPushKeyKeeper.EXPECT().Get(crypto.BytesToPushKeyID(tx.PushKeyID)).Return(pushKey)
				err = CheckPushNoteConsistency(tx, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:pusherAddr, msg:push key does not belong to pusher"))
			})
		})

		When("unable to find pusher account", func() {
			BeforeEach(func() {
				tx := &core.PushNote{
					RepoName:      "repo1",
					PushKeyID:     util.RandBytes(20),
					PusherAddress: "address1",
				}
				mockRepoKeeper.EXPECT().Get(tx.RepoName).Return(&state.Repository{Balance: "10"})

				pushKey := state.BarePushKey()
				pushKey.Address = util.Address("address1")
				mockPushKeyKeeper.EXPECT().Get(crypto.BytesToPushKeyID(tx.PushKeyID)).Return(pushKey)

				mockAcctKeeper.EXPECT().Get(tx.PusherAddress).Return(state.BareAccount())

				err = CheckPushNoteConsistency(tx, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:pusherAddr, msg:pusher account not found"))
			})
		})

		When("pusher account nonce is not correct", func() {
			BeforeEach(func() {
				tx := &core.PushNote{
					RepoName:        "repo1",
					PushKeyID:       util.RandBytes(20),
					PusherAddress:   "address1",
					PusherAcctNonce: 3,
				}
				mockRepoKeeper.EXPECT().Get(tx.RepoName).Return(&state.Repository{Balance: "10"})

				pushKey := state.BarePushKey()
				pushKey.Address = util.Address("address1")
				mockPushKeyKeeper.EXPECT().Get(crypto.BytesToPushKeyID(tx.PushKeyID)).Return(pushKey)

				acct := state.BareAccount()
				acct.Nonce = 1
				mockAcctKeeper.EXPECT().Get(tx.PusherAddress).Return(acct)

				err = CheckPushNoteConsistency(tx, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:accountNonce, msg:wrong account nonce '3', expecting '2'"))
			})
		})

		When("pusher account balance not sufficient to pay fee", func() {
			BeforeEach(func() {

				tx := &core.PushNote{
					RepoName:        "repo1",
					PushKeyID:       util.RandBytes(20),
					PusherAddress:   "address1",
					PusherAcctNonce: 2,
					Fee:             "10",
				}

				mockRepoKeeper.EXPECT().Get(tx.RepoName).Return(&state.Repository{Balance: "10"})

				pushKey := state.BarePushKey()
				pushKey.Address = util.Address("address1")
				mockPushKeyKeeper.EXPECT().Get(crypto.BytesToPushKeyID(tx.PushKeyID)).Return(pushKey)

				acct := state.BareAccount()
				acct.Nonce = 1
				mockAcctKeeper.EXPECT().Get(tx.PusherAddress).Return(acct)

				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&core.BlockInfo{Height: 1}, nil)
				mockTxLogic.EXPECT().
					CanExecCoinTransfer(tx.PusherAddress, util.String("0"), tx.Fee, uint64(2), uint64(1)).
					Return(fmt.Errorf("insufficient"))

				err = CheckPushNoteConsistency(tx, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("insufficient"))
			})
		})
	})

	Describe(".fetchAndCheckReferenceObjects", func() {
		When("object does not exist in the dht", func() {
			BeforeEach(func() {
				objHash := "obj_hash"

				tx := &core.PushNote{RepoName: "repo1", References: []*core.PushedReference{
					{Name: "refs/heads/master", Objects: []string{objHash}},
				}}

				mockRepo := mocks.NewMockBareRepo(ctrl)
				mockRepo.EXPECT().GetObjectSize(objHash).Return(int64(0), fmt.Errorf("object not found"))
				tx.TargetRepo = mockRepo

				mockDHT := mocks.NewMockDHTNode(ctrl)
				dhtKey := MakeRepoObjectDHTKey(tx.GetRepoName(), objHash)
				mockDHT.EXPECT().GetObject(gomock.Any(), &dhttypes.DHTObjectQuery{
					Module:    core.RepoObjectModule,
					ObjectKey: []byte(dhtKey),
				}).Return(nil, fmt.Errorf("object not found"))

				err = fetchAndCheckReferenceObjects(tx, mockDHT)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to fetch object 'obj_hash': object not found"))
			})
		})

		When("object exist in the dht but failed to write to repository", func() {
			BeforeEach(func() {
				objHash := "obj_hash"

				tx := &core.PushNote{RepoName: "repo1", References: []*core.PushedReference{
					{Name: "refs/heads/master", Objects: []string{objHash}},
				}}

				mockRepo := mocks.NewMockBareRepo(ctrl)
				mockRepo.EXPECT().GetObjectSize(objHash).Return(int64(0), fmt.Errorf("object not found"))
				tx.TargetRepo = mockRepo

				mockDHT := mocks.NewMockDHTNode(ctrl)
				dhtKey := MakeRepoObjectDHTKey(tx.GetRepoName(), objHash)
				content := []byte("content")
				mockDHT.EXPECT().GetObject(gomock.Any(), &dhttypes.DHTObjectQuery{
					Module:    core.RepoObjectModule,
					ObjectKey: []byte(dhtKey),
				}).Return(content, nil)

				mockRepo.EXPECT().WriteObjectToFile(objHash, content).Return(fmt.Errorf("something bad"))

				err = fetchAndCheckReferenceObjects(tx, mockDHT)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to write fetched object 'obj_hash' to disk: something bad"))
			})
		})

		When("object exist in the dht and successfully written to disk", func() {
			BeforeEach(func() {
				objHash := "obj_hash"

				tx := &core.PushNote{RepoName: "repo1", References: []*core.PushedReference{
					{Name: "refs/heads/master", Objects: []string{objHash}},
				}, Size: 7}

				mockRepo := mocks.NewMockBareRepo(ctrl)
				mockRepo.EXPECT().GetObjectSize(objHash).Return(int64(0), fmt.Errorf("object not found"))
				tx.TargetRepo = mockRepo

				mockDHT := mocks.NewMockDHTNode(ctrl)
				dhtKey := MakeRepoObjectDHTKey(tx.GetRepoName(), objHash)
				content := []byte("content")
				mockDHT.EXPECT().GetObject(gomock.Any(), &dhttypes.DHTObjectQuery{
					Module:    core.RepoObjectModule,
					ObjectKey: []byte(dhtKey),
				}).Return(content, nil)

				mockRepo.EXPECT().WriteObjectToFile(objHash, content).Return(nil)
				mockRepo.EXPECT().GetObjectSize(objHash).Return(int64(len(content)), nil)

				err = fetchAndCheckReferenceObjects(tx, mockDHT)
			})

			It("should return no error", func() {
				Expect(err).To(BeNil())
			})
		})

		When("object exist in the dht, successfully written to disk and object size is different from actual size", func() {
			BeforeEach(func() {
				objHash := "obj_hash"

				tx := &core.PushNote{RepoName: "repo1", References: []*core.PushedReference{
					{Name: "refs/heads/master", Objects: []string{objHash}},
				}, Size: 10}

				mockRepo := mocks.NewMockBareRepo(ctrl)
				mockRepo.EXPECT().GetObjectSize(objHash).Return(int64(0), fmt.Errorf("object not found"))
				tx.TargetRepo = mockRepo

				mockDHT := mocks.NewMockDHTNode(ctrl)
				dhtKey := MakeRepoObjectDHTKey(tx.GetRepoName(), objHash)
				content := []byte("content")
				mockDHT.EXPECT().GetObject(gomock.Any(), &dhttypes.DHTObjectQuery{
					Module:    core.RepoObjectModule,
					ObjectKey: []byte(dhtKey),
				}).Return(content, nil)

				mockRepo.EXPECT().WriteObjectToFile(objHash, content).Return(nil)
				mockRepo.EXPECT().GetObjectSize(objHash).Return(int64(len(content)), nil)

				err = fetchAndCheckReferenceObjects(tx, mockDHT)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:size, msg:invalid size (10 bytes). actual object size (7 bytes) is different"))
			})
		})
	})

	Describe(".checkPushNoteAgainstTxParams", func() {
		When("pusher key in push note is different from txparams pusher key", func() {
			BeforeEach(func() {
				pn := &core.PushNote{PushKeyID: util.MustDecodePushKeyID("push1wfx7vp8qfyv98cctvamqwec5xjrj48tpxaa77t")}
				txParamss := map[string]*util.TxParams{
					"refs/heads/master": {PushKeyID: crypto.BytesToPushKeyID(util.RandBytes(20))},
				}
				err = checkPushNoteAgainstTxParams(pn, txParamss)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("push note pusher key id does not match " +
					"push key in tx parameter"))
			})
		})

		When("fee do not match", func() {
			BeforeEach(func() {
				pushKeyID := util.RandBytes(20)
				pn := &core.PushNote{PushKeyID: pushKeyID, Fee: "9"}
				txParamss := map[string]*util.TxParams{
					"refs/heads/master": {
						PushKeyID: crypto.BytesToPushKeyID(pushKeyID),
						Fee:       "10",
					},
				}
				err = checkPushNoteAgainstTxParams(pn, txParamss)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("push note fees does not match total txparams fees"))
			})
		})

		When("push note has unexpected pushed reference", func() {
			BeforeEach(func() {
				pushKeyID := util.RandBytes(20)
				pn := &core.PushNote{
					PushKeyID: pushKeyID,
					Fee:       "10",
					References: []*core.PushedReference{
						{Name: "refs/heads/dev"},
					},
				}
				txParamss := map[string]*util.TxParams{
					"refs/heads/master": {
						PushKeyID: crypto.BytesToPushKeyID(pushKeyID),
						Fee:       "10",
					},
				}
				err = checkPushNoteAgainstTxParams(pn, txParamss)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("push note has unexpected pushed reference (refs/heads/dev)"))
			})
		})
	})
})
