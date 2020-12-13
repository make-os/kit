package server

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/crypto/ed25519"
	"github.com/make-os/kit/dht/announcer"
	"github.com/make-os/kit/mocks"
	"github.com/make-os/kit/params"
	plumbing2 "github.com/make-os/kit/remote/plumbing"
	"github.com/make-os/kit/remote/policy"
	"github.com/make-os/kit/remote/push"
	"github.com/make-os/kit/remote/push/types"
	"github.com/make-os/kit/remote/refsync"
	"github.com/make-os/kit/remote/repo"
	testutil2 "github.com/make-os/kit/remote/testutil"
	remotetypes "github.com/make-os/kit/remote/types"
	"github.com/make-os/kit/remote/validation"
	"github.com/make-os/kit/testutil"
	tickettypes "github.com/make-os/kit/ticket/types"
	types2 "github.com/make-os/kit/types"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util"
	crypto2 "github.com/make-os/kit/util/crypto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tendermint/tendermint/p2p"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

var _ = Describe("Reactor", func() {
	var err error
	var cfg *config.AppConfig
	var svr *Server
	var ctrl *gomock.Controller
	var mockLogic *mocks.MockLogic
	var repoName, path string
	var mockMempool *mocks.MockMempool
	var mockPeer *mocks.MockPeer
	var mockRepoKeeper *mocks.MockRepoKeeper
	var mockBlockGetter *mocks.MockBlockGetter
	var mockRepoSyncInfoKeeper *mocks.MockRepoSyncInfoKeeper
	var mockDHT *mocks.MockDHT
	var mockService *mocks.MockService
	var mockTickMgr *mocks.MockTicketManager
	var mockNS *mocks.MockNamespaceKeeper
	var key = ed25519.NewKeyFromIntSeed(1)
	var testRepo remotetypes.LocalRepo
	var refname = "refs/heads/master"

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
		ctrl = gomock.NewController(GinkgoT())

		repoName = util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		testutil2.ExecGit(cfg.GetRepoRoot(), "init", repoName)
		testRepo, err = repo.GetWithLiteGit(cfg.Node.GitBinPath, path)
		Expect(err).To(BeNil())

		mockObjects := testutil.Mocks(ctrl)
		mockLogic = mockObjects.Logic
		mockRepoKeeper = mockObjects.RepoKeeper
		mockRepoSyncInfoKeeper = mockObjects.RepoSyncInfoKeeper

		mockDHT = mocks.NewMockDHT(ctrl)
		mockDHT.EXPECT().RegisterChecker(announcer.ObjTypeRepoName, gomock.Any())
		mockDHT.EXPECT().RegisterChecker(announcer.ObjTypeGit, gomock.Any())

		mockBlockGetter = mocks.NewMockBlockGetter(ctrl)
		mockMempool = mocks.NewMockMempool(ctrl)
		mockTickMgr = mockObjects.TicketManager
		mockNS = mockObjects.NamespaceKeeper
		mockService = mockObjects.Service
		svr = New(cfg, ":9000", mockLogic, mockDHT, mockMempool, mockService, mockBlockGetter)

		mockPeer = mocks.NewMockPeer(ctrl)
	})

	AfterEach(func() {
		svr.Stop()
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".onPushNoteReceived", func() {
		When("unable to decode msg", func() {
			It("should return err=failed to decoded message...", func() {
				err = svr.onPushNoteReceived(mockPeer, util.RandBytes(5))
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("failed to decoded message"))
			})
		})

		When("push note has been seen before", func() {
			It("should return nil", func() {
				pn := &types.Note{RepoName: "repo1"}
				svr.markNoteAsSeen(pn.ID().String())
				err = svr.onPushNoteReceived(mockPeer, pn.Bytes())
				Expect(err).To(BeNil())
			})
		})

		When("checking if push note has been processed in a block", func() {
			It("should return nil if note has not been processed", func() {
				pn := &types.Note{RepoName: "repo1"}
				mockService.EXPECT().GetTx(gomock.Any(), pn.ID().Bytes(), cfg.IsLightNode()).Return(nil, nil, nil)
				err = svr.onPushNoteReceived(mockPeer, pn.Bytes())
				Expect(err).To(BeNil())
			})

			It("should return error if unable to check due to error", func() {
				pn := &types.Note{RepoName: "repo1"}
				mockService.EXPECT().GetTx(gomock.Any(), pn.ID().Bytes(), cfg.IsLightNode()).
					Return(nil, nil, fmt.Errorf("error"))
				err = svr.onPushNoteReceived(mockPeer, pn.Bytes())
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("failed to check if note has been processed: error"))
			})
		})

		When("target repo does not exist locally", func() {
			BeforeEach(func() {
				pn := &types.Note{RepoName: "unknown"}
				mockService.EXPECT().GetTx(gomock.Any(), pn.ID().Bytes(), cfg.IsLightNode()).
					Return(nil, nil, types2.ErrTxNotFound)
				mockPeer.EXPECT().ID().Return(p2p.ID("peer-id"))
				mockRepoKeeper.EXPECT().Get("unknown").Return(state.BareRepository())
				err = svr.onPushNoteReceived(mockPeer, pn.Bytes())
			})

			It("should return err=`repo 'unknown' not found`", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("repo 'unknown' not found"))
			})
		})

		When("namespace is set but it is unknown", func() {
			BeforeEach(func() {
				pn := &types.Note{RepoName: repoName, Namespace: "ns1"}
				mockService.EXPECT().GetTx(gomock.Any(), pn.ID().Bytes(), cfg.IsLightNode()).
					Return(nil, nil, types2.ErrTxNotFound)
				mockPeer.EXPECT().ID().Return(p2p.ID("peer-id"))
				repoState := state.BareRepository()
				repoState.Balance = "100"
				mockRepoKeeper.EXPECT().Get(repoName).Return(repoState)
				mockNS.EXPECT().Get(crypto2.MakeNamespaceHash("ns1")).Return(state.BareNamespace())
				err = svr.onPushNoteReceived(mockPeer, pn.Bytes())
			})

			It("should return err=`namespace 'ns1' not found`", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("namespace 'ns1' not found"))
			})
		})

		When("authentication fails", func() {
			BeforeEach(func() {
				pn := &types.Note{RepoName: repoName}
				mockService.EXPECT().GetTx(gomock.Any(), pn.ID().Bytes(), cfg.IsLightNode()).
					Return(nil, nil, types2.ErrTxNotFound)
				mockPeer.EXPECT().ID().Return(p2p.ID("peer-id"))
				repoState := state.BareRepository()
				repoState.Balance = "100"
				mockRepoKeeper.EXPECT().Get(repoName).Return(repoState)
				svr.authenticate = func(txDetails []*remotetypes.TxDetail, repo *state.Repository, namespace *state.Namespace, keepers core.Keepers, checkTxDetail validation.TxDetailChecker) (enforcer policy.EnforcerFunc, err error) {
					return nil, fmt.Errorf("bad error")
				}
				err = svr.onPushNoteReceived(mockPeer, pn.Bytes())
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("authorization failed: bad error"))
			})
		})

		When("target repository cannot be synced", func() {
			var broadcastNote bool
			var validated bool
			BeforeEach(func() {
				pn := &types.Note{RepoName: "repo1"}
				mockService.EXPECT().GetTx(gomock.Any(), pn.ID().Bytes(), cfg.IsLightNode()).
					Return(nil, nil, types2.ErrTxNotFound)
				mockPeer.EXPECT().ID().Return(p2p.ID("peer-id"))
				repoState := state.BareRepository()
				repoState.Balance = "100"
				mockRepoKeeper.EXPECT().Get("repo1").Return(repoState)
				mockRefSyncer := mocks.NewMockRefSync(ctrl)
				mockRefSyncer.EXPECT().CanSync(pn.Namespace, pn.RepoName).Return(refsync.ErrUntracked)
				svr.refSyncer = mockRefSyncer
				svr.authenticate = func(txDetails []*remotetypes.TxDetail, repo *state.Repository, namespace *state.Namespace, keepers core.Keepers, checkTxDetail validation.TxDetailChecker) (enforcer policy.EnforcerFunc, err error) {
					return nil, nil
				}
				svr.noteBroadcaster = func(pushNote types.PushNote) {
					broadcastNote = true
				}
				svr.checkPushNote = func(tx types.PushNote, logic core.Logic) error {
					validated = true
					return nil
				}
				err = svr.onPushNoteReceived(mockPeer, pn.Bytes())
			})

			It("should return no error", func() {
				Expect(err).To(BeNil())
			})

			It("should broadcast the push note", func() {
				Expect(broadcastNote).To(BeTrue())
			})

			It("should validate the push note", func() {
				Expect(validated).To(BeTrue())
			})
		})

		When("target repository can be synced but the node is in validator mode", func() {
			var broadcastNote, validated bool
			BeforeEach(func() {
				cfg.Node.Validator = true

				pn := &types.Note{RepoName: "repo1"}
				mockService.EXPECT().GetTx(gomock.Any(), pn.ID().Bytes(), cfg.IsLightNode()).
					Return(nil, nil, types2.ErrTxNotFound)
				mockPeer.EXPECT().ID().Return(p2p.ID("peer-id"))
				repoState := state.BareRepository()
				repoState.Balance = "100"
				mockRepoKeeper.EXPECT().Get("repo1").Return(repoState)
				mockRefSyncer := mocks.NewMockRefSync(ctrl)
				mockRefSyncer.EXPECT().CanSync(pn.Namespace, pn.RepoName).Return(nil)
				svr.refSyncer = mockRefSyncer
				svr.authenticate = func(txDetails []*remotetypes.TxDetail, repo *state.Repository, namespace *state.Namespace, keepers core.Keepers, checkTxDetail validation.TxDetailChecker) (enforcer policy.EnforcerFunc, err error) {
					return nil, nil
				}
				svr.noteBroadcaster = func(pushNote types.PushNote) {
					broadcastNote = true
				}
				svr.checkPushNote = func(tx types.PushNote, logic core.Logic) error {
					validated = true
					return nil
				}
				err = svr.onPushNoteReceived(mockPeer, pn.Bytes())
			})

			It("should return no error", func() {
				Expect(err).To(BeNil())
			})

			It("should broadcast the push note", func() {
				Expect(broadcastNote).To(BeTrue())
			})

			It("should validate the push note", func() {
				Expect(validated).To(BeTrue())
			})
		})

		When("target repository cannot be synced and the push note failed validation", func() {
			var broadcastNote bool
			var validated bool
			BeforeEach(func() {
				pn := &types.Note{RepoName: "repo1"}
				mockService.EXPECT().GetTx(gomock.Any(), pn.ID().Bytes(), cfg.IsLightNode()).
					Return(nil, nil, types2.ErrTxNotFound)
				mockPeer.EXPECT().ID().Return(p2p.ID("peer-id"))
				repoState := state.BareRepository()
				repoState.Balance = "100"
				mockRepoKeeper.EXPECT().Get("repo1").Return(repoState)
				mockRefSyncer := mocks.NewMockRefSync(ctrl)
				mockRefSyncer.EXPECT().CanSync(pn.Namespace, pn.RepoName).Return(refsync.ErrUntracked)
				svr.refSyncer = mockRefSyncer
				svr.authenticate = func(txDetails []*remotetypes.TxDetail, repo *state.Repository, namespace *state.Namespace, keepers core.Keepers, checkTxDetail validation.TxDetailChecker) (enforcer policy.EnforcerFunc, err error) {
					return nil, nil
				}
				svr.noteBroadcaster = func(pushNote types.PushNote) {
					broadcastNote = true
				}
				svr.checkPushNote = func(tx types.PushNote, logic core.Logic) error {
					validated = true
					return fmt.Errorf("error")
				}
				err = svr.onPushNoteReceived(mockPeer, pn.Bytes())
			})

			It("should validate the push note", func() {
				Expect(validated).To(BeTrue())
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("failed push note validation: error"))
			})

			It("should not broadcast the push note", func() {
				Expect(broadcastNote).To(BeFalse())
			})
		})

		When("unable to open target repository", func() {
			BeforeEach(func() {
				pn := &types.Note{RepoName: "repo1"}
				mockService.EXPECT().GetTx(gomock.Any(), pn.ID().Bytes(), cfg.IsLightNode()).
					Return(nil, nil, types2.ErrTxNotFound)
				mockPeer.EXPECT().ID().Return(p2p.ID("peer-id"))
				repoState := state.BareRepository()
				repoState.Balance = "100"
				mockRepoKeeper.EXPECT().Get("repo1").Return(repoState)
				mockRefSyncer := mocks.NewMockRefSync(ctrl)
				mockRefSyncer.EXPECT().CanSync(pn.Namespace, pn.RepoName).Return(nil)
				svr.refSyncer = mockRefSyncer
				svr.authenticate = func(txDetails []*remotetypes.TxDetail, repo *state.Repository, namespace *state.Namespace, keepers core.Keepers, checkTxDetail validation.TxDetailChecker) (enforcer policy.EnforcerFunc, err error) {
					return nil, nil
				}
				err = svr.onPushNoteReceived(mockPeer, pn.Bytes())
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("failed to open repo '.*': repository does not exist"))
			})
		})

		When("push note validation fail", func() {
			BeforeEach(func() {
				pn := &types.Note{RepoName: repoName}
				mockService.EXPECT().GetTx(gomock.Any(), pn.ID().Bytes(), cfg.IsLightNode()).
					Return(nil, nil, types2.ErrTxNotFound)
				mockPeer.EXPECT().ID().Return(p2p.ID("peer-id"))
				repoState := state.BareRepository()
				repoState.Balance = "100"
				mockRepoKeeper.EXPECT().Get(repoName).Return(repoState)
				mockRefSyncer := mocks.NewMockRefSync(ctrl)
				mockRefSyncer.EXPECT().CanSync(pn.Namespace, pn.RepoName).Return(nil)
				svr.refSyncer = mockRefSyncer
				svr.authenticate = func(txDetails []*remotetypes.TxDetail, repo *state.Repository, namespace *state.Namespace, keepers core.Keepers, checkTxDetail validation.TxDetailChecker) (policy.EnforcerFunc, error) {
					return nil, nil
				}
				svr.checkPushNote = func(tx types.PushNote, logic core.Logic) error {
					return util.FieldErrorWithIndex(-1, "", "error")
				}
				err = svr.onPushNoteReceived(mockPeer, pn.Bytes())
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed push note validation: field:, msg:error"))
			})
		})

		When("push note validation fail due to pushed reference hash and local/network reference hash mismatch", func() {
			var reSyncScheduled bool
			It("should schedule repo resync", func() {
				pn := &types.Note{RepoName: repoName}
				mockService.EXPECT().GetTx(gomock.Any(), pn.ID().Bytes(), cfg.IsLightNode()).
					Return(nil, nil, types2.ErrTxNotFound)
				mockPeer.EXPECT().ID().Return(p2p.ID("peer-id"))
				repoState := state.BareRepository()
				repoState.Balance = "100"
				mockRepoKeeper.EXPECT().Get(repoName).Return(repoState)
				mockRefSyncer := mocks.NewMockRefSync(ctrl)
				mockRefSyncer.EXPECT().CanSync(pn.Namespace, pn.RepoName).Return(nil)
				svr.refSyncer = mockRefSyncer
				svr.authenticate = func(txDetails []*remotetypes.TxDetail, repo *state.Repository, namespace *state.Namespace, keepers core.Keepers, checkTxDetail validation.TxDetailChecker) (policy.EnforcerFunc, error) {
					return nil, nil
				}
				svr.checkPushNote = func(tx types.PushNote, logic core.Logic) error {
					mmErr := &validation.RefMismatchErr{MismatchLocal: true, Ref: "ref/heads/master"}
					return util.FieldErrorWithIndex(-1, "", "error", mmErr)
				}
				svr.tryScheduleReSync = func(note types.PushNote, ref string, fresh bool) error {
					reSyncScheduled = true
					Expect(fresh).To(BeFalse())
					return nil
				}
				err = svr.onPushNoteReceived(mockPeer, pn.Bytes())
				Expect(err).ToNot(BeNil())
				Expect(reSyncScheduled).To(BeTrue())
			})
		})

		When("push note validation passes", func() {
			var pn *types.Note

			BeforeEach(func() {
				pn = &types.Note{RepoName: repoName}
				mockService.EXPECT().GetTx(gomock.Any(), pn.ID().Bytes(), cfg.IsLightNode()).
					Return(nil, nil, types2.ErrTxNotFound)
				mockPeer.EXPECT().ID().Return(p2p.ID("peer-id"))
				repoState := state.BareRepository()
				repoState.Balance = "100"
				mockRepoKeeper.EXPECT().Get(repoName).Return(repoState)
				mockRefSyncer := mocks.NewMockRefSync(ctrl)
				mockRefSyncer.EXPECT().CanSync(pn.Namespace, pn.RepoName).Return(nil)
				svr.refSyncer = mockRefSyncer
				svr.authenticate = func(txDetails []*remotetypes.TxDetail, repo *state.Repository, namespace *state.Namespace, keepers core.Keepers, checkTxDetail validation.TxDetailChecker) (policy.EnforcerFunc, error) {
					return nil, nil
				}
				svr.checkPushNote = func(tx types.PushNote, logic core.Logic) error {
					return nil
				}
				err = svr.onPushNoteReceived(mockPeer, pn.Bytes())
			})

			It("should return no err", func() {
				Expect(err).To(BeNil())
			})

			It("should mark note as seen", func() {
				Expect(svr.isNoteSeen(pn.ID().String())).To(BeTrue())
			})

			It("should register peer as note sender", func() {
				yes := svr.isNoteSender("peer-id", pn.ID().String())
				Expect(yes).To(BeTrue())
			})

			It("should add fetch task to object fetcher", func() {
				Expect(svr.objFetcher.QueueSize()).To(Equal(1))
			})
		})
	})

	Describe(".onObjectsFetched", func() {
		It("should return error when err is passed", func() {
			polEnforcer := func(subject, object, action string) (bool, int) { return false, 0 }
			err := svr.onObjectsFetched(fmt.Errorf("error"), &types.Note{}, []*remotetypes.TxDetail{}, polEnforcer)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		It("should return error when unable to reload repo handle", func() {
			mockNote := mocks.NewMockPushNote(ctrl)
			mockRepo := mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().Reload().Return(fmt.Errorf("error reloading"))
			mockNote.EXPECT().GetTargetRepo().Return(mockRepo)
			polEnforcer := func(subject, object, action string) (bool, int) { return false, 0 }
			err := svr.onObjectsFetched(nil, mockNote, []*remotetypes.TxDetail{}, polEnforcer)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to reload repo handle: error reloading"))
		})

		It("should return error when unable to get pushed objects size", func() {
			mockNote := mocks.NewMockPushNote(ctrl)
			mockNote.EXPECT().GetRepoName().Return(repoName)
			mockNote.EXPECT().GetTargetRepo().Return(testRepo)
			mockNote.EXPECT().GetTargetRepo().Return(nil)
			polEnforcer := func(subject, object, action string) (bool, int) { return false, 0 }
			err := svr.onObjectsFetched(nil, mockNote, []*remotetypes.TxDetail{}, polEnforcer)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to get pushed refs objects size: repo is required"))
		})

		It("should return error when note object size and local size don't match", func() {
			mockNote := mocks.NewMockPushNote(ctrl)
			mockNote.EXPECT().GetRepoName().Return(repoName)
			mockNote.EXPECT().GetTargetRepo().Return(testRepo).Times(2)
			mockNote.EXPECT().GetPushedReferences().Return(types.PushedReferences{})
			mockNote.EXPECT().IsFromRemotePeer().Return(true)
			mockNote.EXPECT().GetSize().Return(uint64(100))
			polEnforcer := func(subject, object, action string) (bool, int) { return false, 0 }
			err := svr.onObjectsFetched(nil, mockNote, []*remotetypes.TxDetail{}, polEnforcer)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("note's objects size and local size differs"))
		})

		It("should return error when unable to process push note", func() {
			mockNote := mocks.NewMockPushNote(ctrl)
			mockNote.EXPECT().ID().Return(util.StrToBytes32("note_123"))
			mockNote.EXPECT().GetRepoName().Return(repoName)
			mockNote.EXPECT().GetTargetRepo().Return(testRepo).Times(2)
			mockNote.EXPECT().GetPushedReferences().Return(types.PushedReferences{})
			mockNote.EXPECT().IsFromRemotePeer().Return(true)
			mockNote.EXPECT().GetSize().Return(uint64(0))
			polEnforcer := func(subject, object, action string) (bool, int) { return false, 0 }
			svr.processPushNote = func(note types.PushNote, txDetails []*remotetypes.TxDetail, polEnforcer policy.EnforcerFunc) error {
				return fmt.Errorf("error")
			}
			err := svr.onObjectsFetched(nil, mockNote, []*remotetypes.TxDetail{}, polEnforcer)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		It("should return no error when able to process push note", func() {
			mockNote := mocks.NewMockPushNote(ctrl)
			mockNote.EXPECT().GetRepoName().Return(repoName)
			mockNote.EXPECT().GetTargetRepo().Return(testRepo).Times(2)
			mockNote.EXPECT().GetPushedReferences().Return(types.PushedReferences{})
			mockNote.EXPECT().IsFromRemotePeer().Return(true)
			mockNote.EXPECT().GetSize().Return(uint64(0))
			polEnforcer := func(subject, object, action string) (bool, int) { return false, 0 }
			svr.processPushNote = func(note types.PushNote, txDetails []*remotetypes.TxDetail, polEnforcer policy.EnforcerFunc) error {
				return nil
			}
			mockDHT.EXPECT().Announce(announcer.ObjTypeRepoName, repoName, []byte(repoName), nil)
			err := svr.onObjectsFetched(nil, mockNote, []*remotetypes.TxDetail{}, polEnforcer)
			Expect(err).To(BeNil())
		})
	})

	Describe(".createEndorsement", func() {
		When("a pushed reference exists locally", func() {
			var err error
			var end *types.PushEndorsement
			var refHash = "8d998c7de21bbe561f7992bb983cef4b1554993b"

			BeforeEach(func() {
				note := &types.Note{References: []*types.PushedReference{{Name: refname, OldHash: refHash}}}
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				note.SetTargetRepo(mockRepo)
				end, err = createEndorsement(svr.validatorKey, note)
			})

			It("should return no error", func() {
				Expect(err).To(BeNil())
			})

			It("should create endorsement with 1 reference", func() {
				Expect(end.References).To(HaveLen(1))
			})

			Specify("that the reference hash is set", func() {
				Expect(end.References[0].Hash).To(Equal(util.MustFromHex(refHash)))
			})
		})
	})

	Describe(".maybeScheduleReSync", func() {
		It("should return error when unable to get local reference", func() {
			note := &types.Note{}
			mockRepo := mocks.NewMockLocalRepo(ctrl)
			note.SetTargetRepo(mockRepo)
			mockRepo.EXPECT().Reference(plumbing.ReferenceName(refname), false).Return(nil, fmt.Errorf("error"))
			err := svr.maybeScheduleReSync(note, refname, false)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		It("should return nil when the target reference's local and network hash match", func() {
			refHash := "29314f0828b3596ca954e83118f30c8f91a2241b"
			note := &types.Note{}
			mockRepo := mocks.NewMockLocalRepo(ctrl)
			repoState := state.BareRepository()
			repoState.References[refname] = &state.Reference{Hash: plumbing2.HashToBytes(refHash)}
			mockRepo.EXPECT().GetState().Return(repoState)
			note.SetTargetRepo(mockRepo)
			refObj := plumbing.NewReferenceFromStrings(refname, refHash)
			mockRepo.EXPECT().Reference(plumbing.ReferenceName(refname), false).Return(refObj, nil)
			err := svr.maybeScheduleReSync(note, refname, false)
			Expect(err).To(BeNil())
		})

		When("reference does not exist locally and on the network state", func() {
			It("should return nil", func() {
				note := &types.Note{}
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				repoState := state.BareRepository()
				mockRepo.EXPECT().GetState().Return(repoState)
				note.SetTargetRepo(mockRepo)
				mockRepo.EXPECT().Reference(plumbing.ReferenceName(refname), false).Return(nil, plumbing.ErrReferenceNotFound)
				err := svr.maybeScheduleReSync(note, refname, false)
				Expect(err).To(BeNil())
			})
		})

		When("the target reference's local and network do not match", func() {
			It("should return error when unable to get the reference last sync height", func() {
				refHash := "29314f0828b3596ca954e83118f30c8f91a2241b"
				refHash2 := "d303b49c0858c6552c73c6c168099aea3e6a28ba"
				note := &types.Note{RepoName: "repo1"}
				repoState := state.BareRepository()
				repoState.References[refname] = &state.Reference{Hash: plumbing2.HashToBytes(refHash)}

				mockRepo := mocks.NewMockLocalRepo(ctrl)
				mockRepo.EXPECT().GetState().Return(repoState)
				note.SetTargetRepo(mockRepo)

				mockRepoSyncInfoKeeper.EXPECT().GetRefLastSyncHeight(note.RepoName, refname).Return(uint64(0), fmt.Errorf("error"))

				refObj := plumbing.NewReferenceFromStrings(refname, refHash2)
				mockRepo.EXPECT().Reference(plumbing.ReferenceName(refname), false).Return(refObj, nil)
				err := svr.maybeScheduleReSync(note, refname, false)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("error"))
			})

			It("should return error when unable to get the reference last update height", func() {
				refHash := "29314f0828b3596ca954e83118f30c8f91a2241b"
				refHash2 := "d303b49c0858c6552c73c6c168099aea3e6a28ba"
				note := &types.Note{RepoName: "repo1"}
				repoState := state.BareRepository()
				repoState.References[refname] = &state.Reference{Hash: plumbing2.HashToBytes(refHash)}

				mockRepo := mocks.NewMockLocalRepo(ctrl)
				mockRepo.EXPECT().GetState().Return(repoState)
				note.SetTargetRepo(mockRepo)

				mockRepoSyncInfoKeeper.EXPECT().GetRefLastSyncHeight(note.RepoName, refname).Return(uint64(0), fmt.Errorf("error"))

				refObj := plumbing.NewReferenceFromStrings(refname, refHash2)
				mockRepo.EXPECT().Reference(plumbing.ReferenceName(refname), false).Return(refObj, nil)
				err := svr.maybeScheduleReSync(note, refname, false)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("error"))
			})

			It("should send task to the watcher", func() {
				refHash := "29314f0828b3596ca954e83118f30c8f91a2241b"
				refHash2 := "d303b49c0858c6552c73c6c168099aea3e6a28ba"
				note := &types.Note{RepoName: "repo1"}
				repoState := state.BareRepository()
				repoState.UpdatedAt = 200
				repoState.References[refname] = &state.Reference{Hash: plumbing2.HashToBytes(refHash)}

				mockRepo := mocks.NewMockLocalRepo(ctrl)
				mockRepo.EXPECT().GetState().Return(repoState)
				note.SetTargetRepo(mockRepo)

				mockRepoSyncInfoKeeper.EXPECT().GetRefLastSyncHeight(note.RepoName, refname).Return(uint64(100), nil)
				mockRefSync := mocks.NewMockRefSync(ctrl)
				mockRefSync.EXPECT().Watch(note.RepoName, refname, uint64(100), repoState.UpdatedAt.UInt64())
				svr.refSyncer = mockRefSync

				refObj := plumbing.NewReferenceFromStrings(refname, refHash2)
				mockRepo.EXPECT().Reference(plumbing.ReferenceName(refname), false).Return(refObj, nil)
				err := svr.maybeScheduleReSync(note, refname, false)
				Expect(err).To(BeNil())
			})

			It("should return error if watcher returns error", func() {
				refHash := "29314f0828b3596ca954e83118f30c8f91a2241b"
				refHash2 := "d303b49c0858c6552c73c6c168099aea3e6a28ba"
				note := &types.Note{RepoName: "repo1"}
				repoState := state.BareRepository()
				repoState.UpdatedAt = 200
				repoState.References[refname] = &state.Reference{Hash: plumbing2.HashToBytes(refHash)}

				mockRepo := mocks.NewMockLocalRepo(ctrl)
				mockRepo.EXPECT().GetState().Return(repoState)
				note.SetTargetRepo(mockRepo)

				mockRepoSyncInfoKeeper.EXPECT().GetRefLastSyncHeight(note.RepoName, refname).Return(uint64(100), nil)
				mockRefSync := mocks.NewMockRefSync(ctrl)
				mockRefSync.EXPECT().Watch(note.RepoName, refname, uint64(100), repoState.UpdatedAt.UInt64()).Return(fmt.Errorf("error"))
				svr.refSyncer = mockRefSync

				refObj := plumbing.NewReferenceFromStrings(refname, refHash2)
				mockRepo.EXPECT().Reference(plumbing.ReferenceName(refname), false).Return(refObj, nil)
				err := svr.maybeScheduleReSync(note, refname, false)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("refs/heads/master: reference is still being resynchronized (try again later)"))
			})

			When("reference last update height is the same as the repo's last update height", func() {
				It("should send task to the watcher with start height set to the repo's creation height", func() {
					refHash := "29314f0828b3596ca954e83118f30c8f91a2241b"
					refHash2 := "d303b49c0858c6552c73c6c168099aea3e6a28ba"
					note := &types.Note{RepoName: "repo1"}
					repoState := state.BareRepository()
					repoState.CreatedAt = 4
					repoState.UpdatedAt = 100
					repoState.References[refname] = &state.Reference{Hash: plumbing2.HashToBytes(refHash)}

					mockRepo := mocks.NewMockLocalRepo(ctrl)
					mockRepo.EXPECT().GetState().Return(repoState)
					note.SetTargetRepo(mockRepo)

					mockRepoSyncInfoKeeper.EXPECT().GetRefLastSyncHeight(note.RepoName, refname).Return(uint64(100), nil)
					mockRefSync := mocks.NewMockRefSync(ctrl)
					mockRefSync.EXPECT().Watch(note.RepoName, refname, uint64(4), repoState.UpdatedAt.UInt64())
					svr.refSyncer = mockRefSync

					refObj := plumbing.NewReferenceFromStrings(refname, refHash2)
					mockRepo.EXPECT().Reference(plumbing.ReferenceName(refname), false).Return(refObj, nil)
					err := svr.maybeScheduleReSync(note, refname, false)
					Expect(err).To(BeNil())
				})
			})

			When("fromBeginning is true", func() {
				It("should send task to the watcher with start height set to the repo's creation height", func() {
					refHash := "29314f0828b3596ca954e83118f30c8f91a2241b"
					refHash2 := "d303b49c0858c6552c73c6c168099aea3e6a28ba"

					note := &types.Note{RepoName: "repo1"}
					repoState := state.BareRepository()
					repoState.UpdatedAt = 100
					repoState.CreatedAt = 3
					repoState.References[refname] = &state.Reference{Hash: plumbing2.HashToBytes(refHash)}

					mockRepo := mocks.NewMockLocalRepo(ctrl)
					mockRepo.EXPECT().GetState().Return(repoState)
					note.SetTargetRepo(mockRepo)

					mockRepoSyncInfoKeeper.EXPECT().GetRefLastSyncHeight(note.RepoName, refname).Return(uint64(30), nil)
					mockRefSync := mocks.NewMockRefSync(ctrl)
					mockRefSync.EXPECT().Watch(note.RepoName, refname, uint64(3), repoState.UpdatedAt.UInt64())
					svr.refSyncer = mockRefSync

					refObj := plumbing.NewReferenceFromStrings(refname, refHash2)
					mockRepo.EXPECT().Reference(plumbing.ReferenceName(refname), false).Return(refObj, nil)
					err := svr.maybeScheduleReSync(note, refname, true)
					Expect(err).To(BeNil())
				})
			})
		})
	})

	Describe(".maybeProcessPushNote", func() {
		It("should return error when unable to create a packfile from push note", func() {
			svr.makeReferenceUpdatePack = func(tx types.PushNote) (io.ReadSeeker, error) { return nil, fmt.Errorf("error") }
			err := svr.maybeProcessPushNote(&types.Note{}, []*remotetypes.TxDetail{}, nil)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to create packfile from push note: error"))
		})

		When("push note is convertible to a packfile", func() {
			var pack io.ReadSeeker

			BeforeEach(func() {
				testutil2.AppendCommit(path, "file.txt", "some text", "commit msg")
				commitHash := testutil2.GetRecentCommitHash(path, refname)
				commit, _ := testRepo.CommitObject(plumbing.NewHash(commitHash))
				p, _, err := plumbing2.PackObject(testRepo, &plumbing2.PackObjectArgs{Obj: commit})
				Expect(err).To(BeNil())
				pack = testutil.WrapReadSeeker{Rdr: p}
			})

			It("should return error when unable to run git command", func() {
				svr.gitBinPath = "unknown-cmd"
				svr.makeReferenceUpdatePack = func(tx types.PushNote) (io.ReadSeeker, error) { return pack, nil }
				note := &types.Note{}
				note.SetTargetRepo(testRepo)
				err := svr.maybeProcessPushNote(note, []*remotetypes.TxDetail{}, nil)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("git-receive-pack failed to start"))
			})

			It("should return error if unable to handle incoming stream", func() {
				svr.makeReferenceUpdatePack = func(tx types.PushNote) (io.ReadSeeker, error) { return pack, nil }
				svr.makePushHandler = func(targetRepo remotetypes.LocalRepo, txDetails []*remotetypes.TxDetail, enforcer policy.EnforcerFunc) push.Handler {
					mockHandler := mocks.NewMockHandler(ctrl)
					mockHandler.EXPECT().HandleStream(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("error"))
					return mockHandler
				}
				note := &types.Note{}
				note.SetTargetRepo(testRepo)
				err := svr.maybeProcessPushNote(note, []*remotetypes.TxDetail{}, nil)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("HandleStream error: error"))
			})

			It("should return error if unable to handle the pushed updates", func() {
				note := &types.Note{}
				svr.makeReferenceUpdatePack = func(tx types.PushNote) (io.ReadSeeker, error) { return pack, nil }
				svr.makePushHandler = func(targetRepo remotetypes.LocalRepo, txDetails []*remotetypes.TxDetail,
					enforcer policy.EnforcerFunc) push.Handler {
					mockHandler := mocks.NewMockHandler(ctrl)
					mockHandler.EXPECT().HandleStream(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
					mockHandler.EXPECT().HandleUpdate(note).Return(fmt.Errorf("error"))
					return mockHandler
				}
				note.SetTargetRepo(testRepo)
				err := svr.maybeProcessPushNote(note, []*remotetypes.TxDetail{}, nil)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("HandleUpdate error: error"))
			})

			It("should return no error on success", func() {
				note := &types.Note{}
				svr.makeReferenceUpdatePack = func(tx types.PushNote) (io.ReadSeeker, error) { return pack, nil }
				svr.makePushHandler = func(targetRepo remotetypes.LocalRepo, txDetails []*remotetypes.TxDetail,
					enforcer policy.EnforcerFunc) push.Handler {
					mockHandler := mocks.NewMockHandler(ctrl)
					mockHandler.EXPECT().HandleStream(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
					mockHandler.EXPECT().HandleUpdate(note).Return(nil)
					return mockHandler
				}
				note.SetTargetRepo(testRepo)
				err := svr.maybeProcessPushNote(note, []*remotetypes.TxDetail{}, nil)
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".onEndorsementReceived", func() {
		var senderPeerID p2p.ID = "peer_id"

		BeforeEach(func() {
			mockPeer.EXPECT().ID().Return(senderPeerID)
		})

		It("should return error when unable to decode endorsement", func() {
			err = svr.onEndorsementReceived(mockPeer, util.RandBytes(5))
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to decode endorsement"))
		})

		It("should return error when endorsement fail validation", func() {
			svr.checkEndorsement = func(end *types.PushEndorsement, logic core.Logic, index int) error {
				return fmt.Errorf("error")
			}
			end := &types.PushEndorsement{}
			err = svr.onEndorsementReceived(mockPeer, end.Bytes())
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("endorsement validation failed: error"))
		})

		When("endorsement passed validation", func() {
			var end *types.PushEndorsement
			var endorsementWasBroadcast bool

			BeforeEach(func() {
				svr.checkEndorsement = func(end *types.PushEndorsement, logic core.Logic, index int) error { return nil }
				svr.endorsementBroadcaster = func(endorsement types.Endorsement) {
					endorsementWasBroadcast = true
				}
				end = &types.PushEndorsement{NoteID: util.RandBytes(32)}
				err = svr.onEndorsementReceived(mockPeer, end.Bytes())
				Expect(err).To(BeNil())
			})

			Specify("that the sending peer is register as the sender of the push endorsement", func() {
				Expect(svr.isEndorsementSender(string(senderPeerID), end.ID().String())).To(BeTrue())
			})

			Specify("that the endorsement was register to the push note", func() {
				ends := svr.endorsements.Get(end.NoteID.String())
				Expect(ends).ToNot(BeNil())
				Expect(ends).To(HaveKey(end.ID().String()))
				expected := ends.(map[string]*types.PushEndorsement)[end.ID().String()]
				Expect(expected.Bytes()).To(Equal(end.Bytes()))
			})

			Specify("that the endorsement was broadcast", func() {
				Expect(endorsementWasBroadcast).To(BeTrue())
			})
		})

		When("endorsement passed validation but node is in validator mode", func() {
			var end *types.PushEndorsement
			var endorsementWasBroadcast bool

			BeforeEach(func() {
				cfg.Node.Validator = true
				svr.checkEndorsement = func(end *types.PushEndorsement, logic core.Logic, index int) error { return nil }
				svr.endorsementBroadcaster = func(endorsement types.Endorsement) {
					endorsementWasBroadcast = true
				}
				end = &types.PushEndorsement{NoteID: util.RandBytes(32)}
				err = svr.onEndorsementReceived(mockPeer, end.Bytes())
				Expect(err).To(BeNil())
			})

			It("should broadcast endorsement", func() {
				Expect(endorsementWasBroadcast).To(BeTrue())
			})
		})

		When("unable to create push transaction in response to a valid endorsement", func() {
			var end *types.PushEndorsement
			var endorsementWasBroadcast bool

			Specify("that the endorsement was still broadcast to peers", func() {
				svr.checkEndorsement = func(end *types.PushEndorsement, logic core.Logic, index int) error { return nil }
				svr.endorsementBroadcaster = func(endorsement types.Endorsement) {
					endorsementWasBroadcast = true
				}
				svr.makePushTx = func(noteID string) error {
					return fmt.Errorf("error")
				}
				end = &types.PushEndorsement{NoteID: util.RandBytes(32)}
				err = svr.onEndorsementReceived(mockPeer, end.Bytes())
				Expect(err).To(BeNil())
				Expect(endorsementWasBroadcast).To(BeTrue())
			})
		})
	})

	Describe(".markNoteAsSeen & .isNoteSeen", func() {
		It("should return false if note was not marked as seen", func() {
			svr.markNoteAsSeen("note1")
			Expect(svr.isNoteSeen("note2")).To(BeFalse())
			Expect(svr.isNoteSeen("note1")).To(BeTrue())
		})
	})

	Describe(".createPushTx", func() {
		When("no Endorsement for the given note", func() {
			var pushNoteID = "note1"
			BeforeEach(func() {
				err = svr.createPushTx(pushNoteID)
			})

			It("should return err='no endorsements yet'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("no endorsements yet"))
			})
		})

		When("push endorsements for the given note is not up to the quorum size", func() {
			var pushNoteID = "note1"
			BeforeEach(func() {
				params.PushEndorseQuorumSize = 2
				svr.registerNoteEndorsement(pushNoteID, &types.PushEndorsement{SigBLS: util.RandBytes(5)})
				err = svr.createPushTx(pushNoteID)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("cannot create push transaction; note has 1 endorsements, wants 2"))
			})
		})

		When("PushNote does not exist in the pool", func() {
			var pushNoteID = "note1"
			BeforeEach(func() {
				params.PushEndorseQuorumSize = 1
				svr.registerNoteEndorsement(pushNoteID, &types.PushEndorsement{SigBLS: util.RandBytes(5)})
				err = svr.createPushTx(pushNoteID)
			})

			It("should return err='push note not found in pool'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("push note not found in pool"))
			})
		})

		When("unable to get top hosts", func() {
			BeforeEach(func() {
				params.PushEndorseQuorumSize = 1
				var pushNote = &types.Note{RepoName: repoName}
				err = svr.pushPool.Add(pushNote)
				Expect(err).To(BeNil())

				mockTickMgr.EXPECT().GetTopHosts(gomock.Any()).Return(nil, fmt.Errorf("error"))
				svr.registerNoteEndorsement(pushNote.ID().String(), &types.PushEndorsement{SigBLS: util.RandBytes(5)})
				err = svr.createPushTx(pushNote.ID().String())
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to get top hosts: error"))
			})
		})

		When("unable to get ticket of push endorsement sender", func() {
			BeforeEach(func() {
				params.PushEndorseQuorumSize = 1
				var pushNote = &types.Note{RepoName: repoName}
				err = svr.pushPool.Add(pushNote)
				Expect(err).To(BeNil())

				mockTickMgr.EXPECT().GetTopHosts(gomock.Any()).Return([]*tickettypes.SelectedTicket{}, nil)
				end := &types.PushEndorsement{
					SigBLS:         util.RandBytes(5),
					EndorserPubKey: util.BytesToBytes32(util.RandBytes(32)),
				}
				svr.registerNoteEndorsement(pushNote.ID().String(), end)
				err = svr.createPushTx(pushNote.ID().String())
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("endorsement[0]: ticket not found in top hosts list"))
			})
		})

		When("a push endorsement has invalid bls public key", func() {
			BeforeEach(func() {
				params.PushEndorseQuorumSize = 1
				var pushNote = &types.Note{RepoName: repoName}
				err = svr.pushPool.Add(pushNote)
				Expect(err).To(BeNil())

				mockTickMgr.EXPECT().GetTopHosts(gomock.Any()).Return([]*tickettypes.SelectedTicket{
					{Ticket: &tickettypes.Ticket{ProposerPubKey: key.PubKey().MustBytes32(), BLSPubKey: []byte("invalid bls public key")}},
				}, nil)

				end := &types.PushEndorsement{
					EndorserPubKey: key.PubKey().MustBytes32(),
				}

				svr.registerNoteEndorsement(pushNote.ID().String(), end)
				err = svr.createPushTx(pushNote.ID().String())
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("endorsement[0]: bls public key is invalid: bn256.G2: not enough data"))
			})
		})

		When("endorsement signature is invalid", func() {
			BeforeEach(func() {
				params.PushEndorseQuorumSize = 1
				var pushNote = &types.Note{RepoName: repoName}
				err = svr.pushPool.Add(pushNote)
				Expect(err).To(BeNil())

				mockTickMgr.EXPECT().GetTopHosts(gomock.Any()).Return([]*tickettypes.SelectedTicket{
					{Ticket: &tickettypes.Ticket{ProposerPubKey: key.PubKey().MustBytes32(), BLSPubKey: key.PrivKey().BLSKey().Public().Bytes()}},
				}, nil)

				end := &types.PushEndorsement{
					EndorserPubKey: key.PubKey().MustBytes32(),
					SigBLS:         util.RandBytes(5),
				}

				svr.registerNoteEndorsement(pushNote.ID().String(), end)
				err = svr.createPushTx(pushNote.ID().String())
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("unable to create aggregated signature"))
			})
		})

		When("push note is ok", func() {
			It("should return no error", func() {
				params.PushEndorseQuorumSize = 1
				var pushNote = &types.Note{RepoName: repoName}
				err = svr.pushPool.Add(pushNote)
				Expect(err).To(BeNil())

				mockTickMgr.EXPECT().GetTopHosts(gomock.Any()).Return([]*tickettypes.SelectedTicket{
					{Ticket: &tickettypes.Ticket{ProposerPubKey: key.PubKey().MustBytes32(), BLSPubKey: key.PrivKey().BLSKey().Public().Bytes()}},
				}, nil)

				end := &types.PushEndorsement{NoteID: []byte{1, 2, 3}, EndorserPubKey: key.PubKey().MustBytes32()}
				end.SigBLS, err = key.PrivKey().BLSKey().Sign(end.BytesForBLSSig())
				Expect(err).To(BeNil())
				svr.registerNoteEndorsement(pushNote.ID().String(), end)

				end2 := &types.PushEndorsement{NoteID: []byte{1, 2, 4}, EndorserPubKey: key.PubKey().MustBytes32()}
				end2.SigBLS, err = key.PrivKey().BLSKey().Sign(end2.BytesForBLSSig())
				Expect(err).To(BeNil())
				svr.registerNoteEndorsement(pushNote.ID().String(), end2)

				mockMempool.EXPECT().Add(gomock.AssignableToTypeOf(&txns.TxPush{})).DoAndReturn(func(tx types2.BaseTx) (bool, error) {
					// NoteID and SigBLS fields must be unset
					Expect(tx.(*txns.TxPush).Endorsements).To(HaveLen(2))
					Expect(tx.(*txns.TxPush).Endorsements[0].NoteID).To(BeNil())
					Expect(tx.(*txns.TxPush).Endorsements[0].SigBLS).To(BeNil())
					Expect(tx.(*txns.TxPush).Endorsements[1].NoteID).To(BeNil())
					Expect(tx.(*txns.TxPush).Endorsements[1].SigBLS).To(BeNil())

					// 0-index reference must be non-nil. Greater than 0-index must be nil
					Expect(tx.(*txns.TxPush).Endorsements[0].References).ToNot(BeNil())
					Expect(tx.(*txns.TxPush).Endorsements[1].References).To(BeNil())
					return false, nil
				})
				err = svr.createPushTx(pushNote.ID().String())

				Expect(err).To(BeNil())
			})
		})
	})
})
