package repo

import (
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/mr-tron/base58"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/crypto"
	dhttypes "gitlab.com/makeos/mosdef/dht/types"
	"gitlab.com/makeos/mosdef/mocks"
	"gitlab.com/makeos/mosdef/testutil"
	tickettypes "gitlab.com/makeos/mosdef/ticket/types"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/util"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

var _ = Describe("Validation", func() {
	var err error
	var cfg *config.AppConfig
	var repo core.BareRepo
	var path string
	var pubKey *crypto.PubKey
	var privKey *crypto.Key
	var ctrl *gomock.Controller
	var mockLogic *mocks.MockLogic
	var mockTickMgr *mocks.MockTicketManager
	var mockRepoKeeper *mocks.MockRepoKeeper
	var mockNSKeeper *mocks.MockNamespaceKeeper
	var mockPushKeyKeeper *mocks.MockPushKeyKeeper
	var mockAcctKeeper *mocks.MockAccountKeeper
	var mockSysKeeper *mocks.MockSystemKeeper
	var mockTxLogic *mocks.MockTxLogic
	var baseTxDetail *types.TxDetail

	var pushKeyGetter = func(pushKeyID string) (crypto.PublicKey, error) {
		return pubKey.ToPublicKey(), nil
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
		baseTxDetail = &types.TxDetail{PushKeyID: privKey.PushAddr().String()}
		pubKey = privKey.PubKey()

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

		When("commit was not signed", func() {
			BeforeEach(func() {
				appendCommit(path, "file.txt", "line 1", "commit 1")
				commitHash, _ := repo.GetRecentCommit()
				commit, _ = repo.CommitObject(plumbing.NewHash(strings.TrimSpace(commitHash)))
				err = checkCommit(commit, baseTxDetail, pushKeyGetter)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("commit (.*) was not signed"))
			})
		})

		When("commit is signed but unable to get public key using the pushKeyID", func() {
			BeforeEach(func() {
				appendCommit(path, "file.txt", "line 1", "commit message")
				commitHash, _ := repo.GetRecentCommit()
				commit, _ = repo.CommitObject(plumbing.NewHash(strings.TrimSpace(commitHash)))
				commit.PGPSignature = "signature"
				err = checkCommit(commit, baseTxDetail, pushKeyGetterWithErr(fmt.Errorf("not found")))
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("failed to get push key (.*): not found"))
			})
		})

		When("commit has a signature but the signature is malformed", func() {
			BeforeEach(func() {
				appendCommit(path, "file.txt", "line 1", "commit message")
				commitHash, _ := repo.GetRecentCommit()
				commit, _ = repo.CommitObject(plumbing.NewHash(strings.TrimSpace(commitHash)))
				commit.PGPSignature = "signature"
				err = checkCommit(commit, baseTxDetail, pushKeyGetter)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("signature is malformed"))
			})
		})

		When("commit signature header could not be decoded", func() {
			BeforeEach(func() {
				appendCommit(path, "file.txt", "line 1", "commit message")
				commitHash, _ := repo.GetRecentCommit()
				commit, _ = repo.CommitObject(plumbing.NewHash(strings.TrimSpace(commitHash)))
				commit.PGPSignature = string(pem.EncodeToMemory(&pem.Block{
					Bytes:   []byte{1, 2, 3},
					Headers: map[string]string{"nonce": "invalid"},
					Type:    "SIGNATURE"}))
				err = checkCommit(commit, baseTxDetail, pushKeyGetter)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("failed to decode PEM header: nonce must be numeric"))
			})
		})

		When("commit has a signature but the signature is not valid", func() {
			BeforeEach(func() {
				appendCommit(path, "file.txt", "line 1", "commit message")
				commitHash, _ := repo.GetRecentCommit()
				commit, _ = repo.CommitObject(plumbing.NewHash(strings.TrimSpace(commitHash)))
				txDetail, _ := types.MakeAndValidateTxDetail("0", "0", pubKey.PushAddr().String(), nil)
				commit.PGPSignature = string(pem.EncodeToMemory(&pem.Block{
					Bytes:   []byte{1, 2, 3},
					Headers: txDetail.ToMapForPEMHeader(),
					Type:    "SIGNATURE"}))
				err = checkCommit(commit, baseTxDetail, pushKeyGetter)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("object (.*) signature is invalid"))
			})
		})

		When("commit has a valid signature but its decoded header did not match the request transaction info", func() {
			var err error
			var sig []byte
			BeforeEach(func() {
				appendCommit(path, "file.txt", "line 1", "commit message")
				commitHash, _ := repo.GetRecentCommit()
				commit, _ = repo.CommitObject(plumbing.NewHash(commitHash))
				sigMsg := getCommitOrTagSigMsg(commit)

				txDetail, _ := types.MakeAndValidateTxDetail("0", "0", pubKey.PushAddr().String(), nil)
				pemHeader := txDetail.ToMapForPEMHeader()

				sig, err = privKey.PrivKey().Sign(append([]byte(sigMsg), txDetail.BytesNoSig()...))
				Expect(err).To(BeNil())

				commit.PGPSignature = string(pem.EncodeToMemory(&pem.Block{Bytes: sig, Headers: pemHeader, Type: "SIGNATURE"}))
				err = checkCommit(commit, baseTxDetail, pushKeyGetter)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(ErrSigHeaderAndReqParamsMismatch))
			})
		})

		When("commit has a valid signature and the decoded signature header matches the request transaction info", func() {
			var err error
			var sig []byte
			BeforeEach(func() {
				appendCommit(path, "file.txt", "line 1", "commit message")
				commitHash, _ := repo.GetRecentCommit()
				commit, _ = repo.CommitObject(plumbing.NewHash(commitHash))
				sigMsg := getCommitOrTagSigMsg(commit)

				txDetail, _ := types.MakeAndValidateTxDetail("0", "0", pubKey.PushAddr().String(), nil)
				pemHeader := txDetail.ToMapForPEMHeader()

				sig, err = privKey.PrivKey().Sign(append([]byte(sigMsg), txDetail.BytesNoSig()...))
				Expect(err).To(BeNil())

				commit.PGPSignature = string(pem.EncodeToMemory(&pem.Block{Bytes: sig, Headers: pemHeader, Type: "SIGNATURE"}))
				err = checkCommit(commit, txDetail, pushKeyGetter)
			})

			It("should return err", func() {
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".checkAnnotatedTag", func() {
		var err error
		var tob *object.Tag

		When("tag is not signed", func() {
			BeforeEach(func() {
				createCommitAndAnnotatedTag(path, "file.txt", "first file", "commit 1", "v1")
				tagRef, _ := repo.Tag("v1")
				tob, _ = repo.TagObject(tagRef.Hash())
				err = checkAnnotatedTag(tob, baseTxDetail, pushKeyGetter)
			})

			It("should return err='txDetail was not set'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("tag (.*) is unsigned. Sign the tag with your push key"))
			})
		})

		When("tag is signed but unable to get public key using the pushKeyID", func() {
			BeforeEach(func() {
				txDetail := types.MakeTxDetail("0", "0", pubKey.PushAddr().String(), nil)
				createCommitAndAnnotatedTag(path, "file.txt", "first file", txDetail, "v1")
				tagRef, _ := repo.Tag("v1")
				tob, _ = repo.TagObject(tagRef.Hash())
				tob.PGPSignature = "signature"
				err = checkAnnotatedTag(tob, baseTxDetail, pushKeyGetterWithErr(fmt.Errorf("bad error")))
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("failed to get pusher key(.*) to verify commit .*"))
			})
		})

		When("tag has a signature but the signature is malformed", func() {
			BeforeEach(func() {
				createCommitAndAnnotatedTag(path, "file.txt", "first file", "tag message", "v1")
				tagRef, _ := repo.Tag("v1")
				tob, _ = repo.TagObject(tagRef.Hash())
				tob.PGPSignature = "signature"
				err = checkAnnotatedTag(tob, baseTxDetail, pushKeyGetter)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("signature is malformed"))
			})
		})

		When("tag has a signature but the signature is invalid", func() {
			BeforeEach(func() {
				createCommitAndAnnotatedTag(path, "file.txt", "first file", "tag message", "v1")
				tagRef, _ := repo.Tag("v1")
				tob, _ = repo.TagObject(tagRef.Hash())

				txDetail, _ := types.MakeAndValidateTxDetail("0", "0", pubKey.PushAddr().String(), nil)
				sig := pem.EncodeToMemory(&pem.Block{Bytes: []byte("invalid sig"), Headers: txDetail.ToMapForPEMHeader(), Type: "SIGNATURE"})
				tob.PGPSignature = string(sig)

				err = checkAnnotatedTag(tob, baseTxDetail, pushKeyGetter)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("object (.*) signature is invalid"))
			})
		})

		When("tag has a valid signature but the signature header does not match the request transaction info", func() {
			BeforeEach(func() {
				createCommitAndAnnotatedTag(path, "file.txt", "first file", "tag message", "v1")
				tagRef, _ := repo.Tag("v1")
				tob, _ = repo.TagObject(tagRef.Hash())

				txDetail, _ := types.MakeAndValidateTxDetail("0", "0", pubKey.PushAddr().String(), nil)
				msg := getCommitOrTagSigMsg(tob)
				sig, _ := privKey.PrivKey().Sign(append([]byte(msg), txDetail.BytesNoSig()...))
				pemData := pem.EncodeToMemory(&pem.Block{Bytes: sig, Headers: txDetail.ToMapForPEMHeader(), Type: "SIGNATURE"})
				tob.PGPSignature = string(pemData)

				err = checkAnnotatedTag(tob, baseTxDetail, pushKeyGetter)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(ErrSigHeaderAndReqParamsMismatch))
			})
		})

		When("tag has signature and header are valid but the referenced commit is unsigned", func() {
			BeforeEach(func() {
				createCommitAndAnnotatedTag(path, "file.txt", "first file", "tag message", "v1")
				tagRef, _ := repo.Tag("v1")
				tob, _ = repo.TagObject(tagRef.Hash())

				txDetail, _ := types.MakeAndValidateTxDetail("0", "0", pubKey.PushAddr().String(), nil)
				msg := getCommitOrTagSigMsg(tob)
				sig, _ := privKey.PrivKey().Sign(append([]byte(msg), txDetail.BytesNoSig()...))
				pemData := pem.EncodeToMemory(&pem.Block{Bytes: sig, Headers: txDetail.ToMapForPEMHeader(), Type: "SIGNATURE"})
				tob.PGPSignature = string(pemData)

				err = checkAnnotatedTag(tob, txDetail, pushKeyGetter)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("commit (.*) was not signed"))
			})
		})
	})

	Describe(".checkNote", func() {
		var err error

		When("unable to get note", func() {
			BeforeEach(func() {
				detail := &types.TxDetail{Reference: "refs/notes/note1"}
				mockRepo := mocks.NewMockBareRepo(ctrl)
				mockRepo.EXPECT().RefGet(detail.Reference).Return("", fmt.Errorf("bad error"))
				err = checkNote(mockRepo, detail)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("failed to get note: bad error"))
			})
		})

		When("current note hash is different from tx detail hash", func() {
			BeforeEach(func() {
				hash := util.RandString(40)
				detail := &types.TxDetail{Reference: "refs/notes/note1", Head: hash}
				mockRepo := mocks.NewMockBareRepo(ctrl)
				noteHash := util.RandString(40)
				mockRepo.EXPECT().RefGet(detail.Reference).Return(noteHash, nil)
				err = checkNote(mockRepo, detail)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("current note hash differs from signed note hash"))
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
				err = validateChange(repo, change, baseTxDetail, pushKeyGetter)
			})

			It("should return err='unrecognised change item'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("unrecognised change item"))
			})
		})

		When("change item referenced object is an unknown commit object", func() {
			BeforeEach(func() {
				change := &core.ItemChange{Item: &Obj{Name: "refs/heads/unknown", Data: "unknown_hash"}}
				err = validateChange(repo, change, baseTxDetail, pushKeyGetter)
			})

			It("should return err='unable to get commit object: object not found'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("unable to get commit object: object not found"))
			})
		})

		When("change item referenced object is an unknown tag object", func() {
			BeforeEach(func() {
				change := &core.ItemChange{Item: &Obj{Name: "refs/tags/unknown", Data: "unknown_hash"}}
				err = validateChange(repo, change, baseTxDetail, pushKeyGetter)
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
			{&core.PushNote{RepoName: "repo", PushKeyID: util.RandBytes(20), Timestamp: time.Now().Unix(), PusherAcctNonce: 1}, "field:nodePubKey, msg:push node public key is required"},
			{&core.PushNote{RepoName: "repo", PushKeyID: util.RandBytes(20), Timestamp: time.Now().Unix(), PusherAcctNonce: 1, NodePubKey: key.PubKey().MustBytes32()}, "field:nodeSig, msg:push node signature is required"},
			{&core.PushNote{RepoName: "repo", PushKeyID: util.RandBytes(20), Timestamp: time.Now().Unix(), PusherAcctNonce: 1, NodePubKey: key.PubKey().MustBytes32(), NodeSig: []byte("invalid signature")}, "field:nodeSig, msg:failed to verify signature"},
			{&core.PushNote{RepoName: "repo", References: []*core.PushedReference{{}}}, "index:0, field:references.name, msg:name is required"},
			{&core.PushNote{RepoName: "repo", References: []*core.PushedReference{{Name: "ref1"}}}, "index:0, field:references.oldHash, msg:old hash is required"},
			{&core.PushNote{RepoName: "repo", References: []*core.PushedReference{{Name: "ref1", OldHash: "invalid"}}}, "index:0, field:references.oldHash, msg:old hash is not valid"},
			{&core.PushNote{RepoName: "repo", References: []*core.PushedReference{{Name: "ref1", OldHash: util.RandString(40)}}}, "index:0, field:references.newHash, msg:new hash is required"},
			{&core.PushNote{RepoName: "repo", References: []*core.PushedReference{{Name: "ref1", OldHash: util.RandString(40), NewHash: "invalid"}}}, "index:0, field:references.newHash, msg:new hash is not valid"},
			{&core.PushNote{RepoName: "repo", References: []*core.PushedReference{{Name: "ref1", OldHash: util.RandString(40), NewHash: util.RandString(40)}}}, "index:0, field:references.nonce, msg:reference nonce must be greater than zero"},
			{&core.PushNote{RepoName: "repo", References: []*core.PushedReference{{Name: "ref1", OldHash: util.RandString(40), NewHash: util.RandString(40), Nonce: 1, Objects: []string{"invalid object"}}}}, "index:0, field:references.objects.0, msg:object hash is not valid"},
			{&core.PushNote{RepoName: "repo", References: []*core.PushedReference{{Name: "ref1", OldHash: util.RandString(40), NewHash: util.RandString(40), Nonce: 1}}}, "index:0, field:fee, msg:fee is required"},
			{&core.PushNote{RepoName: "repo", References: []*core.PushedReference{{Name: "ref1", OldHash: util.RandString(40), NewHash: util.RandString(40), Nonce: 1, Fee: "ten"}}}, "index:0, field:fee, msg:fee must be numeric"},
			{&core.PushNote{RepoName: "repo", References: []*core.PushedReference{{Name: "ref1", OldHash: util.RandString(40), NewHash: util.RandString(40), Nonce: 1, Fee: "0", MergeProposalID: "1a"}}}, "index:0, field:mergeID, msg:merge proposal id must be numeric"},
			{&core.PushNote{RepoName: "repo", References: []*core.PushedReference{{Name: "ref1", OldHash: util.RandString(40), NewHash: util.RandString(40), Nonce: 1, Fee: "0", MergeProposalID: "123456789"}}}, "index:0, field:mergeID, msg:merge proposal id exceeded 8 bytes limit"},
			{&core.PushNote{RepoName: "repo", References: []*core.PushedReference{{Name: "ref1", OldHash: util.RandString(40), NewHash: util.RandString(40), Nonce: 1, Fee: "0"}}}, "index:0, field:pushSig, msg:signature is required"},
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

	Describe(".checkPushedReferenceConsistency", func() {
		var mockRepo *mocks.MockBareRepo
		var oldHash = fmt.Sprintf("%x", util.RandBytes(20))
		var newHash = fmt.Sprintf("%x", util.RandBytes(20))

		BeforeEach(func() {
			mockRepo = mocks.NewMockBareRepo(ctrl)
		})

		When("old hash is non-zero and pushed reference does not exist", func() {
			BeforeEach(func() {
				refs := &core.PushedReference{Name: "refs/heads/master", OldHash: oldHash}
				repository := &state.Repository{References: map[string]*state.Reference{}}
				err = checkPushedReferenceConsistency(mockRepo, refs, repository)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:references, msg:reference 'refs/heads/master' is unknown"))
			})
		})

		When("old hash is zero and pushed reference does not exist", func() {
			BeforeEach(func() {
				refs := &core.PushedReference{Name: "refs/heads/master", OldHash: strings.Repeat("0", 40)}
				repository := &state.Repository{References: map[string]*state.Reference{}}
				err = checkPushedReferenceConsistency(mockRepo, refs, repository)
			})

			It("should not return error about unknown reference", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).ToNot(Equal("field:references, msg:reference 'refs/heads/master' is unknown"))
			})
		})

		When("old hash of reference is different from the local hash of same reference", func() {
			BeforeEach(func() {
				refName := "refs/heads/master"
				ref := &core.PushedReference{Name: refName, OldHash: oldHash}
				repository := &state.Repository{References: map[string]*state.Reference{refName: {Nonce: 0}}}
				mockRepo.EXPECT().Reference(plumbing.ReferenceName(refName), false).
					Return(plumbing.NewReferenceFromStrings("", util.RandString(40)), nil)
				err = checkPushedReferenceConsistency(mockRepo, ref, repository)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:references, msg:reference 'refs/heads/master' old hash does not match its local version"))
			})
		})

		When("old hash of reference is non-zero and the local equivalent reference is not accessible", func() {
			BeforeEach(func() {
				refName := "refs/heads/master"
				ref := &core.PushedReference{Name: refName, OldHash: oldHash}
				repository := &state.Repository{References: map[string]*state.Reference{refName: {Nonce: 0}}}
				mockRepo.EXPECT().Reference(plumbing.ReferenceName(refName), false).
					Return(nil, plumbing.ErrReferenceNotFound)
				err = checkPushedReferenceConsistency(mockRepo, ref, repository)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:references, msg:reference 'refs/heads/master' does not exist locally"))
			})
		})

		When("old hash of reference is non-zero and nil repo passed", func() {
			BeforeEach(func() {
				refName := "refs/heads/master"
				ref := &core.PushedReference{Name: refName, OldHash: oldHash}
				repository := &state.Repository{References: map[string]*state.Reference{refName: {Nonce: 0}}}
				err = checkPushedReferenceConsistency(nil, ref, repository)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).ToNot(Equal("field:references, msg:reference 'refs/heads/master' does not exist locally"))
			})
		})

		When("old hash of reference is non-zero and it is different from the hash of the equivalent local reference", func() {
			BeforeEach(func() {
				refName := "refs/heads/master"
				ref := &core.PushedReference{Name: refName, OldHash: oldHash}
				repository := &state.Repository{References: map[string]*state.Reference{refName: {Nonce: 0}}}
				mockRepo.EXPECT().Reference(plumbing.ReferenceName(refName), false).
					Return(plumbing.NewReferenceFromStrings("", util.RandString(40)), nil)
				err = checkPushedReferenceConsistency(mockRepo, ref, repository)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:references, msg:reference 'refs/heads/master' old hash does not match its local version"))
			})
		})

		When("pushed reference nonce is unexpected", func() {
			BeforeEach(func() {
				refName := "refs/heads/master"
				ref := &core.PushedReference{OldHash: oldHash, Name: refName, NewHash: newHash, Objects: []string{newHash}, Nonce: 2}
				repository := &state.Repository{References: map[string]*state.Reference{refName: {Nonce: 0}}}
				mockRepo.EXPECT().
					Reference(plumbing.ReferenceName(refName), false).
					Return(plumbing.NewHashReference("", plumbing.NewHash(oldHash)), nil)
				err = checkPushedReferenceConsistency(mockRepo, ref, repository)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:references, msg:reference 'refs/heads/master' has nonce '2', expecting '1'"))
			})
		})

		When("nonce is unset", func() {
			BeforeEach(func() {
				refName := "refs/heads/master"
				refs := &core.PushedReference{Name: refName, OldHash: oldHash, NewHash: newHash, Objects: []string{newHash}, Nonce: 0}
				repository := &state.Repository{References: map[string]*state.Reference{refName: {Nonce: 0}}}
				mockRepo.EXPECT().
					Reference(plumbing.ReferenceName(refName), false).
					Return(plumbing.NewHashReference("", plumbing.NewHash(oldHash)), nil)
				err = checkPushedReferenceConsistency(mockRepo, refs, repository)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:references, msg:reference 'refs/heads/master' has nonce '0', expecting '1'"))
			})
		})

		When("no validation error", func() {
			BeforeEach(func() {
				refName := "refs/heads/master"
				newHash := util.RandString(40)
				refs := &core.PushedReference{Name: refName, OldHash: oldHash, NewHash: newHash, Objects: []string{newHash}, Nonce: 1, Fee: "1"}
				repository := &state.Repository{References: map[string]*state.Reference{refName: {Nonce: 0}}}
				mockRepo.EXPECT().
					Reference(plumbing.ReferenceName(refName), false).
					Return(plumbing.NewHashReference("", plumbing.NewHash(oldHash)), nil)

				err = checkPushedReferenceConsistency(mockRepo, refs, repository)
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
				pushKey.Address = "address2"
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
				pushKey.Address = "address1"
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
				tx := &core.PushNote{RepoName: "repo1", PushKeyID: util.RandBytes(20), PusherAddress: "address1", PusherAcctNonce: 3}
				mockRepoKeeper.EXPECT().Get(tx.RepoName).Return(&state.Repository{Balance: "10"})

				pushKey := state.BarePushKey()
				pushKey.Address = "address1"
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

		When("reference signature is invalid", func() {
			BeforeEach(func() {
				tx := &core.PushNote{RepoName: "repo1", PushKeyID: util.RandBytes(20), PusherAddress: "address1", PusherAcctNonce: 2}
				tx.References = append(tx.References, &core.PushedReference{
					Name:    "refs/heads/master",
					Nonce:   1,
					PushSig: util.RandBytes(64),
				})
				mockRepoKeeper.EXPECT().Get(tx.RepoName).Return(&state.Repository{Balance: "10"})

				pushKey := state.BarePushKey()
				pushKey.Address = "address1"
				pushKey.PubKey = privKey.PubKey().ToPublicKey()
				mockPushKeyKeeper.EXPECT().Get(crypto.BytesToPushKeyID(tx.PushKeyID)).Return(pushKey)

				acct := state.BareAccount()
				acct.Nonce = 1
				mockAcctKeeper.EXPECT().Get(tx.PusherAddress).Return(acct)

				err = CheckPushNoteConsistency(tx, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("index:0, field:references, msg:reference (.*) signature is not valid"))
			})
		})

		When("pusher account balance not sufficient to pay fee", func() {
			BeforeEach(func() {

				tx := &core.PushNote{
					RepoName:        "repo1",
					PushKeyID:       util.RandBytes(20),
					PusherAddress:   "address1",
					PusherAcctNonce: 2,
				}

				mockRepoKeeper.EXPECT().Get(tx.RepoName).Return(&state.Repository{Balance: "10"})

				pushKey := state.BarePushKey()
				pushKey.Address = "address1"
				mockPushKeyKeeper.EXPECT().Get(crypto.BytesToPushKeyID(tx.PushKeyID)).Return(pushKey)

				acct := state.BareAccount()
				acct.Nonce = 1
				mockAcctKeeper.EXPECT().Get(tx.PusherAddress).Return(acct)

				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&core.BlockInfo{Height: 1}, nil)
				mockTxLogic.EXPECT().
					CanExecCoinTransfer(tx.PusherAddress, util.String("0"), tx.GetFee(), uint64(2), uint64(1)).
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

	Describe(".checkTxDetailSanity", func() {
		It("should return error when push key is unset", func() {
			detail := &types.TxDetail{}
			err := checkTxDetailSanity(detail, false)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("field:pkID, msg:push key id is required"))
		})

		It("should return error when push key is not valid", func() {
			detail := &types.TxDetail{PushKeyID: "invalid_key"}
			err := checkTxDetailSanity(detail, false)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("field:pkID, msg:push key id is not valid"))
		})

		It("should return error when nonce is not set", func() {
			detail := &types.TxDetail{PushKeyID: privKey.PushAddr().String()}
			err := checkTxDetailSanity(detail, false)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("field:nonce, msg:nonce is required"))
		})

		It("should return error when fee is not set", func() {
			detail := &types.TxDetail{PushKeyID: privKey.PushAddr().String(), Nonce: 1, Fee: ""}
			err := checkTxDetailSanity(detail, false)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("field:fee, msg:fee is required"))
		})

		It("should return error when fee is not numeric", func() {
			detail := &types.TxDetail{PushKeyID: privKey.PushAddr().String(), Nonce: 1, Fee: "1_invalid"}
			err := checkTxDetailSanity(detail, false)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("field:fee, msg:fee must be numeric"))
		})

		It("should return error when signature is malformed", func() {
			detail := &types.TxDetail{PushKeyID: privKey.PushAddr().String(), Nonce: 1, Fee: "1", Signature: "0x_invalid"}
			err := checkTxDetailSanity(detail, false)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("field:sig, msg:signature format is not valid"))
		})

		It("should return error when merge proposal ID is not numeric", func() {
			detail := &types.TxDetail{
				PushKeyID:       privKey.PushAddr().String(),
				Nonce:           1,
				Fee:             "1",
				Signature:       base58.Encode([]byte("data")),
				MergeProposalID: "invalid",
			}
			err := checkTxDetailSanity(detail, false)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("field:mergeID, msg:merge proposal id must be numeric"))
		})

		It("should return error when merge proposal ID surpasses 8 bytes", func() {
			detail := &types.TxDetail{
				PushKeyID:       privKey.PushAddr().String(),
				Nonce:           1,
				Fee:             "1",
				Signature:       base58.Encode([]byte("data")),
				MergeProposalID: "1234567890",
			}
			err := checkTxDetailSanity(detail, false)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("field:mergeID, msg:merge proposal id exceeded 8 bytes limit"))
		})

		It("should return no error", func() {
			detail := &types.TxDetail{
				PushKeyID:       privKey.PushAddr().String(),
				Nonce:           1,
				Fee:             "1",
				Signature:       base58.Encode([]byte("data")),
				MergeProposalID: "12",
			}
			err := checkTxDetailSanity(detail, false)
			Expect(err).To(BeNil())
		})
	})

	Describe(".isBlockedByScope", func() {
		It("should return true when scopes has r/repo1 and tx repo=repo2 and namespace=''", func() {
			scopes := []string{"r/repo1"}
			detail := &types.TxDetail{RepoName: "repo2", RepoNamespace: ""}
			ns := state.BareNamespace()
			Expect(isBlockedByScope(scopes, detail, ns)).To(BeTrue())
		})

		It("should return false when scopes has r/repo1 and tx repo=repo1 and namespace=''", func() {
			scopes := []string{"r/repo1"}
			detail := &types.TxDetail{RepoName: "repo1", RepoNamespace: ""}
			ns := state.BareNamespace()
			Expect(isBlockedByScope(scopes, detail, ns)).To(BeFalse())
		})

		It("should return true when scopes has ns1/repo1 and tx repo=repo1 and namespace=ns2", func() {
			scopes := []string{"ns1/repo1"}
			detail := &types.TxDetail{RepoName: "repo1", RepoNamespace: "ns2"}
			ns := state.BareNamespace()
			Expect(isBlockedByScope(scopes, detail, ns)).To(BeTrue())
		})

		It("should return false when scopes has ns1/repo1 and tx repo=repo1 and namespace=ns1", func() {
			scopes := []string{"ns1/repo1"}
			detail := &types.TxDetail{RepoName: "repo1", RepoNamespace: "ns1"}
			ns := state.BareNamespace()
			Expect(isBlockedByScope(scopes, detail, ns)).To(BeFalse())
		})

		It("should return true when scopes has ns1/ and tx repo=repo1 and namespace=ns2", func() {
			scopes := []string{"ns1/"}
			detail := &types.TxDetail{RepoName: "repo1", RepoNamespace: "ns2"}
			ns := state.BareNamespace()
			Expect(isBlockedByScope(scopes, detail, ns)).To(BeTrue())
		})

		It("should return false when scopes has ns1/ and tx repo=repo1 and namespace=ns1", func() {
			scopes := []string{"ns1/"}
			detail := &types.TxDetail{RepoName: "repo1", RepoNamespace: "ns1"}
			ns := state.BareNamespace()
			Expect(isBlockedByScope(scopes, detail, ns)).To(BeFalse())
		})

		It("should return false when scopes has repo1 and tx repo=repo1 and namespace=''", func() {
			scopes := []string{"repo1"}
			detail := &types.TxDetail{RepoName: "repo1", RepoNamespace: ""}
			ns := state.BareNamespace()
			Expect(isBlockedByScope(scopes, detail, ns)).To(BeFalse())
		})

		It("should return true when scopes has repo1 and tx repo=repo2 and namespace=''", func() {
			scopes := []string{"repo1"}
			detail := &types.TxDetail{RepoName: "repo2", RepoNamespace: ""}
			ns := state.BareNamespace()
			Expect(isBlockedByScope(scopes, detail, ns)).To(BeTrue())
		})

		It("should return true when scopes has repo1 and tx repo=repo2 and "+
			"namespace='ns1' "+
			"but ns1/repo2 does not point to repo1", func() {
			scopes := []string{"repo1"}
			detail := &types.TxDetail{RepoName: "repo2", RepoNamespace: "ns1"}
			ns := state.BareNamespace()
			ns.Domains["repo2"] = "repo100"
			Expect(isBlockedByScope(scopes, detail, ns)).To(BeTrue())
		})

		It("should return false when scopes has repo1 and tx repo=repo2 and "+
			"namespace='ns1' "+
			"but ns1/repo2 does not point to r/repo1", func() {
			scopes := []string{"repo1"}
			detail := &types.TxDetail{RepoName: "repo2", RepoNamespace: "ns1"}
			ns := state.BareNamespace()
			ns.Domains["repo2"] = "r/repo1"
			Expect(isBlockedByScope(scopes, detail, ns)).To(BeFalse())
		})
	})

	Describe(".checkTxDetailConsistency", func() {
		It("should return error when push key is unknown", func() {
			detail := &types.TxDetail{PushKeyID: privKey.PushAddr().String()}
			mockPushKeyKeeper.EXPECT().Get(detail.PushKeyID).Return(state.BarePushKey())
			err := checkTxDetailConsistency(detail, mockLogic, false)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("field:pkID, msg:push key not found"))
		})

		It("should return error when repo namespace and push key scopes are set but namespace does not exist", func() {
			detail := &types.TxDetail{PushKeyID: privKey.PushAddr().String(), Nonce: 9, RepoName: "repo1", RepoNamespace: "ns1"}
			pk := state.BarePushKey()
			pk.Address = privKey.Addr()
			pk.Scopes = []string{"r/repo1"}
			mockPushKeyKeeper.EXPECT().Get(detail.PushKeyID).Return(pk)

			mockNSKeeper.EXPECT().Get(detail.RepoNamespace).Return(state.BareNamespace())

			err := checkTxDetailConsistency(detail, mockLogic, false)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("field:repoNamespace, msg:namespace (ns1) is unknown"))
		})

		It("should return scope error when key scope is r/repo1 and tx repo=repo2 and namespace is unset", func() {
			detail := &types.TxDetail{PushKeyID: privKey.PushAddr().String(), Nonce: 9, RepoName: "repo2", RepoNamespace: ""}
			pk := state.BarePushKey()
			pk.Address = privKey.Addr()
			pk.Scopes = []string{"r/repo1"}
			mockPushKeyKeeper.EXPECT().Get(detail.PushKeyID).Return(pk)

			err := checkTxDetailConsistency(detail, mockLogic, false)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("not permitted due to scope limitation"))
		})

		It("should return scope error when key scope is ns1/repo1 and tx repo=repo2 and namespace=ns1", func() {
			detail := &types.TxDetail{PushKeyID: privKey.PushAddr().String(), Nonce: 9, RepoName: "repo2", RepoNamespace: "ns1"}
			pk := state.BarePushKey()
			pk.Address = privKey.Addr()
			pk.Scopes = []string{"ns1/repo1"}
			mockPushKeyKeeper.EXPECT().Get(detail.PushKeyID).Return(pk)

			ns := state.BareNamespace()
			ns.Domains["ns1"] = "real-repo"
			mockNSKeeper.EXPECT().Get(detail.RepoNamespace).Return(ns)

			err := checkTxDetailConsistency(detail, mockLogic, false)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("not permitted due to scope limitation"))
		})

		It("should return scope error when key scope is ns1/ and tx repo=repo2 and namespace=ns2", func() {
			detail := &types.TxDetail{PushKeyID: privKey.PushAddr().String(), Nonce: 9, RepoName: "repo2", RepoNamespace: "ns1"}
			pk := state.BarePushKey()
			pk.Address = privKey.Addr()
			pk.Scopes = []string{"ns1/repo1"}
			mockPushKeyKeeper.EXPECT().Get(detail.PushKeyID).Return(pk)

			ns := state.BareNamespace()
			ns.Domains["ns1"] = "real-repo"
			mockNSKeeper.EXPECT().Get(detail.RepoNamespace).Return(ns)

			err := checkTxDetailConsistency(detail, mockLogic, false)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("not permitted due to scope limitation"))
		})

		It("should return error when nonce is not greater than push key owner account nonce", func() {
			detail := &types.TxDetail{PushKeyID: privKey.PushAddr().String(), Nonce: 9}

			pk := state.BarePushKey()
			pk.Address = privKey.Addr()
			mockPushKeyKeeper.EXPECT().Get(detail.PushKeyID).Return(pk)

			acct := state.BareAccount()
			acct.Nonce = 10
			mockAcctKeeper.EXPECT().Get(pk.Address).Return(acct)

			err := checkTxDetailConsistency(detail, mockLogic, false)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("field:nonce, msg:nonce (9) must be greater than current key owner nonce (10)"))
		})

		It("should return error when signature could not be verified", func() {
			detail := &types.TxDetail{PushKeyID: privKey.PushAddr().String(), Nonce: 9}
			sig, err := privKey.PrivKey().Sign(detail.BytesNoSig())
			Expect(err).To(BeNil())
			detail.Signature = base58.Encode(sig)

			pk := state.BarePushKey()
			pk.Address = privKey.Addr()
			pk.PubKey = crypto.BytesToPublicKey([]byte("bad key"))
			mockPushKeyKeeper.EXPECT().Get(detail.PushKeyID).Return(pk)

			acct := state.BareAccount()
			acct.Nonce = 8
			mockAcctKeeper.EXPECT().Get(pk.Address).Return(acct)

			err = checkTxDetailConsistency(detail, mockLogic, false)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("field:sig, msg:signature is not valid"))
		})

		It("should return nil when signature is valid", func() {
			detail := &types.TxDetail{PushKeyID: privKey.PushAddr().String(), Nonce: 9}
			sig, err := privKey.PrivKey().Sign(detail.BytesNoSig())
			Expect(err).To(BeNil())
			detail.Signature = base58.Encode(sig)

			pk := state.BarePushKey()
			pk.Address = privKey.Addr()
			pk.PubKey = privKey.PubKey().ToPublicKey()
			mockPushKeyKeeper.EXPECT().Get(detail.PushKeyID).Return(pk)

			acct := state.BareAccount()
			acct.Nonce = 8
			mockAcctKeeper.EXPECT().Get(pk.Address).Return(acct)

			err = checkTxDetailConsistency(detail, mockLogic, false)
			Expect(err).To(BeNil())
		})
	})

	Describe(".checkTxDetail", func() {
		It("should return nil when no error ", func() {
			detail := &types.TxDetail{
				PushKeyID:       privKey.PushAddr().String(),
				Nonce:           9,
				Fee:             "1",
				MergeProposalID: "12",
			}
			sig, err := privKey.PrivKey().Sign(detail.BytesNoSig())
			Expect(err).To(BeNil())
			detail.Signature = base58.Encode(sig)

			pk := state.BarePushKey()
			pk.Address = privKey.Addr()
			pk.PubKey = privKey.PubKey().ToPublicKey()
			mockPushKeyKeeper.EXPECT().Get(detail.PushKeyID).Return(pk)

			acct := state.BareAccount()
			acct.Nonce = 8
			mockAcctKeeper.EXPECT().Get(pk.Address).Return(acct)

			err = checkTxDetail(detail, mockLogic, false)
			Expect(err).To(BeNil())
		})
	})
})
