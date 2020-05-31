package server

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tendermint/tendermint/p2p"
	dhttypes "gitlab.com/makeos/mosdef/dht/types"
	plumbing2 "gitlab.com/makeos/mosdef/remote/plumbing"
	"gitlab.com/makeos/mosdef/remote/policy"
	"gitlab.com/makeos/mosdef/remote/push"
	"gitlab.com/makeos/mosdef/remote/push/types"
	"gitlab.com/makeos/mosdef/remote/repo"
	testutil2 "gitlab.com/makeos/mosdef/remote/testutil"
	remotetypes "gitlab.com/makeos/mosdef/remote/types"
	"gitlab.com/makeos/mosdef/remote/validation"
	tickettypes "gitlab.com/makeos/mosdef/ticket/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/types/txns"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp"

	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/mocks"
	"gitlab.com/makeos/mosdef/params"
	"gitlab.com/makeos/mosdef/testutil"
	"gitlab.com/makeos/mosdef/util"
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
	var mockDHT *mocks.MockDHTNode
	var mockRemoteSrv *mocks.MockRemoteServer
	var mockTickMgr *mocks.MockTicketManager
	var mockNS *mocks.MockNamespaceKeeper
	var key = crypto.NewKeyFromIntSeed(1)

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
		ctrl = gomock.NewController(GinkgoT())

		repoName = util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		testutil2.ExecGit(cfg.GetRepoRoot(), "init", repoName)
		_, err = repo.GetWithLiteGit(cfg.Node.GitBinPath, path)
		Expect(err).To(BeNil())

		mockObjects := testutil.MockLogic(ctrl)
		mockLogic = mockObjects.Logic
		mockRemoteSrv = mockObjects.RemoteServer
		mockRepoKeeper = mockObjects.RepoKeeper
		mockDHT = mocks.NewMockDHTNode(ctrl)
		mockBlockGetter = mocks.NewMockBlockGetter(ctrl)
		mockMempool = mocks.NewMockMempool(ctrl)
		mockTickMgr = mockObjects.TicketManager
		mockNS = mockObjects.NamespaceKeeper
		svr = NewManager(cfg, ":9000", mockLogic, mockDHT, mockMempool, mockBlockGetter)

		mockPeer = mocks.NewMockPeer(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".onPushNote", func() {
		When("in validator mode", func() {
			It("should return nil", func() {
				cfg.Node.Validator = true
				err = svr.onPushNote(mockPeer, util.RandBytes(10))
				Expect(err).To(BeNil())
			})
		})

		When("unable to decode msg", func() {
			It("should return err=failed to decoded message...", func() {
				err = svr.onPushNote(mockPeer, util.RandBytes(5))
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("failed to decoded message"))
			})
		})

		When("repo is not found", func() {
			BeforeEach(func() {
				mockPeer.EXPECT().ID().Return(p2p.ID("peer-id"))
				mockRepoKeeper.EXPECT().Get("unknown").Return(state.BareRepository())
				pn := &types.PushNote{RepoName: "unknown"}
				err = svr.onPushNote(mockPeer, pn.Bytes())
			})

			It("should return err=`repo 'unknown' not found`", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("repo 'unknown' not found"))
			})
		})

		When("namespace is set but not found", func() {
			BeforeEach(func() {
				mockPeer.EXPECT().ID().Return(p2p.ID("peer-id"))
				repoState := state.BareRepository()
				repoState.Balance = "100"
				mockRepoKeeper.EXPECT().Get("repo1").Return(repoState)
				mockNS.EXPECT().Get(util.HashNamespace("ns1")).Return(state.BareNamespace())
				pn := &types.PushNote{RepoName: "repo1", Namespace: "ns1"}
				err = svr.onPushNote(mockPeer, pn.Bytes())
			})

			It("should return err=`namespace 'ns1' not found`", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("namespace 'ns1' not found"))
			})
		})

		When("authentication fails", func() {
			BeforeEach(func() {
				mockPeer.EXPECT().ID().Return(p2p.ID("peer-id"))
				repoState := state.BareRepository()
				repoState.Balance = "100"
				mockRepoKeeper.EXPECT().Get("repo1").Return(repoState)

				svr.authenticate = func(
					txDetails []*remotetypes.TxDetail,
					repo *state.Repository,
					namespace *state.Namespace,
					keepers core.Keepers,
					checkTxDetail validation.TxDetailChecker) (enforcer policy.EnforcerFunc, err error) {
					return nil, fmt.Errorf("bad error")
				}

				pn := &types.PushNote{RepoName: "repo1"}
				err = svr.onPushNote(mockPeer, pn.Bytes())
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("authorization failed: bad error"))
			})
		})

		When("unable to open target repository", func() {
			BeforeEach(func() {
				mockPeer.EXPECT().ID().Return(p2p.ID("peer-id"))
				repoState := state.BareRepository()
				repoState.Balance = "100"
				mockRepoKeeper.EXPECT().Get("repo1").Return(repoState)

				svr.authenticate = func(
					txDetails []*remotetypes.TxDetail,
					repo *state.Repository,
					namespace *state.Namespace,
					keepers core.Keepers,
					checkTxDetail validation.TxDetailChecker) (enforcer policy.EnforcerFunc, err error) {
					return nil, nil
				}

				pn := &types.PushNote{RepoName: "repo1"}
				err = svr.onPushNote(mockPeer, pn.Bytes())
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to open repo 'repo1': repository does not exist"))
			})
		})

		When("push note failed validation", func() {
			var peerID = p2p.ID("peer-id")
			var pn *types.PushNote

			BeforeEach(func() {
				mockPeer.EXPECT().ID().Return(peerID)
				repoState := state.BareRepository()
				repoState.Balance = "100"
				mockRepoKeeper.EXPECT().Get(repoName).Return(repoState)

				svr.authenticate = func(
					txDetails []*remotetypes.TxDetail,
					repo *state.Repository,
					namespace *state.Namespace,
					keepers core.Keepers,
					checkTxDetail validation.TxDetailChecker) (enforcer policy.EnforcerFunc, err error) {
					return nil, nil
				}
				svr.checkPushNote = func(tx types.PushNotice, dht dhttypes.DHTNode, logic core.Logic) error {
					return fmt.Errorf("error")
				}

				pn = &types.PushNote{RepoName: repoName}
				err = svr.onPushNote(mockPeer, pn.Bytes())
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed push note validation: error"))
			})

			Specify("that the peer is registered as the sender of the push note", func() {
				Expect(svr.isPushNoteSender(string(peerID), pn.ID().String())).To(BeTrue())
			})
		})

		When("unable to create packfile from push note", func() {
			var peerID = p2p.ID("peer-id")
			var pn *types.PushNote

			BeforeEach(func() {
				mockPeer.EXPECT().ID().Return(peerID)
				repoState := state.BareRepository()
				repoState.Balance = "100"
				mockRepoKeeper.EXPECT().Get(repoName).Return(repoState)

				svr.authenticate = func(
					txDetails []*remotetypes.TxDetail,
					repo *state.Repository,
					namespace *state.Namespace,
					keepers core.Keepers,
					checkTxDetail validation.TxDetailChecker) (enforcer policy.EnforcerFunc, err error) {
					return nil, nil
				}
				svr.checkPushNote = func(tx types.PushNotice, dht dhttypes.DHTNode, logic core.Logic) error {
					return nil
				}
				svr.packfileMaker = func(repo remotetypes.LocalRepo, tx types.PushNotice) (seeker io.ReadSeeker, err error) {
					return nil, fmt.Errorf("bad error")
				}

				pn = &types.PushNote{RepoName: repoName}
				err = svr.onPushNote(mockPeer, pn.Bytes())
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to create packfile from push note: bad error"))
			})
		})

		When("push handler failed to handle packfile stream", func() {
			var peerID = p2p.ID("peer-id")
			var pn *types.PushNote

			BeforeEach(func() {
				mockPeer.EXPECT().ID().Return(peerID)
				repoState := state.BareRepository()
				repoState.Balance = "100"
				mockRepoKeeper.EXPECT().Get(repoName).Return(repoState)

				svr.authenticate = func(txDetails []*remotetypes.TxDetail, repo *state.Repository, namespace *state.Namespace, keepers core.Keepers, checkTxDetail validation.TxDetailChecker) (enforcer policy.EnforcerFunc, err error) {
					return nil, nil
				}
				svr.checkPushNote = func(tx types.PushNotice, dht dhttypes.DHTNode, logic core.Logic) error {
					return nil
				}
				svr.packfileMaker = func(repo remotetypes.LocalRepo, tx types.PushNotice) (seeker io.ReadSeeker, err error) {
					oldState := plumbing2.GetRepoState(repo)
					testutil2.AppendCommit(path, "file.txt", "line 1\n", "commit 1")
					newState := plumbing2.GetRepoState(repo)
					packfile, err := push.MakePackfile(repo, oldState, newState)
					Expect(err).To(BeNil())
					return packfile, nil
				}

				svr.makePushHandler = func(targetRepo remotetypes.LocalRepo, txDetails []*remotetypes.TxDetail, enforcer policy.EnforcerFunc) *push.Handler {
					mockRemoteSrv.EXPECT().GetRepoState(gomock.Any()).Return(nil, fmt.Errorf("bad error"))
					return &push.Handler{Server: mockRemoteSrv}
				}

				pn = &types.PushNote{RepoName: repoName}
				err = svr.onPushNote(mockPeer, pn.Bytes())
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("HandleStream error: bad error"))
			})
		})

		When("push handler failed to handle reference without error", func() {
			var peerID = p2p.ID("peer-id")
			var pn *types.PushNote

			BeforeEach(func() {
				mockPeer.EXPECT().ID().Return(peerID)
				repoState := state.BareRepository()
				repoState.Balance = "100"
				mockRepoKeeper.EXPECT().Get(repoName).Return(repoState)

				svr.authenticate = func(txDetails []*remotetypes.TxDetail, repo *state.Repository, namespace *state.Namespace,
					keepers core.Keepers, checkTxDetail validation.TxDetailChecker) (enforcer policy.EnforcerFunc, err error) {
					return nil, nil
				}
				svr.checkPushNote = func(tx types.PushNotice, dht dhttypes.DHTNode, logic core.Logic) error {
					return nil
				}

				pushHandler := &push.Handler{Server: mockRemoteSrv}
				svr.packfileMaker = func(repo remotetypes.LocalRepo, tx types.PushNotice) (seeker io.ReadSeeker, err error) {
					pushHandler.OldState = plumbing2.GetRepoState(repo)
					pushHandler.Repo = repo
					testutil2.AppendCommit(path, "file.txt", "line 1\n", "commit 1")
					newState := plumbing2.GetRepoState(repo)
					packfile, err := push.MakePackfile(repo, pushHandler.OldState, newState)
					Expect(err).To(BeNil())
					return packfile, nil
				}
				svr.makePushHandler = func(targetRepo remotetypes.LocalRepo, txDetails []*remotetypes.TxDetail, enforcer policy.EnforcerFunc) *push.Handler {
					return pushHandler
				}

				pushHandler.AuthorizationHandler = func(ur *packp.ReferenceUpdateRequest) error {
					return nil
				}
				pushHandler.ReferenceHandler = func(ref string, revertOnly bool) []error {
					Expect(ref).To(Equal("refs/heads/master"))
					return []error{fmt.Errorf("bad reference")}
				}

				pn = &types.PushNote{RepoName: repoName}
				err = svr.onPushNote(mockPeer, pn.Bytes())
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("HandleReferences error: bad reference"))
			})
		})

		When("push pool addition failed", func() {
			var peerID = p2p.ID("peer-id")
			var pn *types.PushNote

			BeforeEach(func() {
				mockPeer.EXPECT().ID().Return(peerID)
				repoState := state.BareRepository()
				repoState.Balance = "100"
				mockRepoKeeper.EXPECT().Get(repoName).Return(repoState)

				svr.authenticate = func(txDetails []*remotetypes.TxDetail, repo *state.Repository, namespace *state.Namespace,
					keepers core.Keepers, checkTxDetail validation.TxDetailChecker) (enforcer policy.EnforcerFunc, err error) {
					return nil, nil
				}
				svr.checkPushNote = func(tx types.PushNotice, dht dhttypes.DHTNode, logic core.Logic) error {
					return nil
				}

				pushHandler := &push.Handler{Server: mockRemoteSrv}
				svr.packfileMaker = func(repo remotetypes.LocalRepo, tx types.PushNotice) (seeker io.ReadSeeker, err error) {
					pushHandler.OldState = plumbing2.GetRepoState(repo)
					pushHandler.Repo = repo
					testutil2.AppendCommit(path, "file.txt", "line 1\n", "commit 1")
					newState := plumbing2.GetRepoState(repo)
					packfile, err := push.MakePackfile(repo, pushHandler.OldState, newState)
					Expect(err).To(BeNil())
					return packfile, nil
				}
				svr.makePushHandler = func(targetRepo remotetypes.LocalRepo, txDetails []*remotetypes.TxDetail, enforcer policy.EnforcerFunc) *push.Handler {
					return pushHandler
				}

				pushHandler.AuthorizationHandler = func(ur *packp.ReferenceUpdateRequest) error {
					return nil
				}
				pushHandler.ReferenceHandler = func(ref string, revertOnly bool) []error {
					Expect(ref).To(Equal("refs/heads/master"))
					return nil
				}
				svr.pushedObjectsBroadcaster = func(pn *types.PushNote) (err error) {
					return nil
				}

				mockPushPool := mocks.NewMockPushPooler(ctrl)
				mockPushPool.EXPECT().Add(gomock.Any(), true).Return(fmt.Errorf("push pool error"))
				svr.pushPool = mockPushPool

				pn = &types.PushNote{RepoName: repoName}
				err = svr.onPushNote(mockPeer, pn.Bytes())
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to add push note to push pool: push pool error"))
			})
		})

		When("push note is successfully processed", func() {
			var peerID = p2p.ID("peer-id")
			var pn *types.PushNote

			BeforeEach(func() {
				mockPeer.EXPECT().ID().Return(peerID)
				repoState := state.BareRepository()
				repoState.Balance = "100"
				mockRepoKeeper.EXPECT().Get(repoName).Return(repoState)

				svr.authenticate = func(txDetails []*remotetypes.TxDetail, repo *state.Repository, namespace *state.Namespace,
					keepers core.Keepers, checkTxDetail validation.TxDetailChecker) (enforcer policy.EnforcerFunc, err error) {
					return nil, nil
				}
				svr.checkPushNote = func(tx types.PushNotice, dht dhttypes.DHTNode, logic core.Logic) error {
					return nil
				}

				pushHandler := &push.Handler{Server: mockRemoteSrv}
				svr.packfileMaker = func(repo remotetypes.LocalRepo, tx types.PushNotice) (seeker io.ReadSeeker, err error) {
					pushHandler.OldState = plumbing2.GetRepoState(repo)
					pushHandler.Repo = repo
					testutil2.AppendCommit(path, "file.txt", "line 1\n", "commit 1")
					newState := plumbing2.GetRepoState(repo)
					packfile, err := push.MakePackfile(repo, pushHandler.OldState, newState)
					Expect(err).To(BeNil())
					return packfile, nil
				}
				svr.makePushHandler = func(targetRepo remotetypes.LocalRepo, txDetails []*remotetypes.TxDetail, enforcer policy.EnforcerFunc) *push.Handler {
					return pushHandler
				}

				pushHandler.AuthorizationHandler = func(ur *packp.ReferenceUpdateRequest) error {
					return nil
				}
				pushHandler.ReferenceHandler = func(ref string, revertOnly bool) []error {
					Expect(ref).To(Equal("refs/heads/master"))
					return nil
				}
				svr.pushedObjectsBroadcaster = func(pn *types.PushNote) (err error) {
					return nil
				}

				mockPushPool := mocks.NewMockPushPooler(ctrl)
				mockPushPool.EXPECT().Add(gomock.Any(), true).Return(nil)
				svr.pushPool = mockPushPool

				pn = &types.PushNote{RepoName: repoName}
				err = svr.onPushNote(mockPeer, pn.Bytes())
			})

			It("should return nil", func() {
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".onPushEnd", func() {
		When("unable to decode msg", func() {
			BeforeEach(func() {
				err = svr.onPushEnd(mockPeer, util.RandBytes(5))
			})

			It("should return err=failed to decoded message...", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("failed to decoded message"))
			})
		})
	})

	Describe(".MaybeCreatePushTx", func() {
		When("no PushEndorsement for the given note", func() {
			var pushNoteID = "note1"
			BeforeEach(func() {
				err = svr.MaybeCreatePushTx(pushNoteID)
			})

			It("should return err='no endorsements yet'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("no endorsements yet"))
			})
		})

		When("PushEnds for the given note is not up to the quorum size", func() {
			var pushNoteID = "note1"
			BeforeEach(func() {
				params.PushEndorseQuorumSize = 2
				svr.addPushNoteEndorsement(pushNoteID, &types.PushEndorsement{Sig: util.BytesToBytes64(util.RandBytes(5))})
				err = svr.MaybeCreatePushTx(pushNoteID)
			})

			It("should return err='Not enough push endorsements to satisfy quorum size'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("not enough push endorsements to satisfy quorum size"))
			})
		})

		When("PushNotice does not exist in the pool", func() {
			var pushNoteID = "note1"
			BeforeEach(func() {
				params.PushEndorseQuorumSize = 1
				svr.addPushNoteEndorsement(pushNoteID, &types.PushEndorsement{Sig: util.BytesToBytes64(util.RandBytes(5))})
				err = svr.MaybeCreatePushTx(pushNoteID)
			})

			It("should return err='push note not found in pool'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("push note not found in pool"))
			})
		})

		When("unable to get top hosts", func() {
			BeforeEach(func() {
				params.PushEndorseQuorumSize = 1
				var pushNote = &types.PushNote{RepoName: "repo1"}
				err = svr.pushPool.Add(pushNote, true)
				Expect(err).To(BeNil())

				mockTickMgr.EXPECT().GetTopHosts(gomock.Any()).Return(nil, fmt.Errorf("error"))
				svr.addPushNoteEndorsement(pushNote.ID().String(), &types.PushEndorsement{Sig: util.BytesToBytes64(util.RandBytes(5))})
				err = svr.MaybeCreatePushTx(pushNote.ID().String())
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to get top hosts: error"))
			})
		})

		When("unable to get ticket of push endorsement sender", func() {
			BeforeEach(func() {
				params.PushEndorseQuorumSize = 1
				var pushNote = &types.PushNote{RepoName: "repo1"}
				err = svr.pushPool.Add(pushNote, true)
				Expect(err).To(BeNil())

				mockTickMgr.EXPECT().GetTopHosts(gomock.Any()).Return([]*tickettypes.SelectedTicket{}, nil)
				pushEnd := &types.PushEndorsement{
					Sig:            util.BytesToBytes64(util.RandBytes(5)),
					EndorserPubKey: util.BytesToBytes32(util.RandBytes(32)),
				}
				svr.addPushNoteEndorsement(pushNote.ID().String(), pushEnd)
				err = svr.MaybeCreatePushTx(pushNote.ID().String())
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("endorsement[0]: ticket not found in top hosts list"))
			})
		})

		When("a push endorsement has invalid bls public key", func() {
			BeforeEach(func() {
				params.PushEndorseQuorumSize = 1
				var pushNote = &types.PushNote{RepoName: "repo1"}
				err = svr.pushPool.Add(pushNote, true)
				Expect(err).To(BeNil())

				mockTickMgr.EXPECT().GetTopHosts(gomock.Any()).Return([]*tickettypes.SelectedTicket{
					{
						Ticket: &tickettypes.Ticket{
							ProposerPubKey: key.PubKey().MustBytes32(),
							BLSPubKey:      []byte("invalid bls public key"),
						},
					},
				}, nil)
				pushEnd := &types.PushEndorsement{
					EndorserPubKey: key.PubKey().MustBytes32(),
				}

				svr.addPushNoteEndorsement(pushNote.ID().String(), pushEnd)
				err = svr.MaybeCreatePushTx(pushNote.ID().String())
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("endorsement[0]: bls public key is invalid: bn256.G2: not enough data"))
			})
		})

		When("endorsement signature is invalid", func() {
			BeforeEach(func() {
				params.PushEndorseQuorumSize = 1
				var pushNote = &types.PushNote{RepoName: "repo1"}
				err = svr.pushPool.Add(pushNote, true)
				Expect(err).To(BeNil())

				mockTickMgr.EXPECT().GetTopHosts(gomock.Any()).Return([]*tickettypes.SelectedTicket{
					{
						Ticket: &tickettypes.Ticket{
							ProposerPubKey: key.PubKey().MustBytes32(),
							BLSPubKey:      key.PrivKey().BLSKey().Public().Bytes(),
						},
					},
				}, nil)
				pushEnd := &types.PushEndorsement{
					EndorserPubKey: key.PubKey().MustBytes32(),
				}
				pushEnd.Sig = util.BytesToBytes64(util.RandBytes(64))

				svr.addPushNoteEndorsement(pushNote.ID().String(), pushEnd)
				err = svr.MaybeCreatePushTx(pushNote.ID().String())
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("unable to create aggregated signature"))
			})
		})

		When("push note is ok", func() {
			BeforeEach(func() {
				params.PushEndorseQuorumSize = 1
				var pushNote = &types.PushNote{RepoName: "repo1"}
				err = svr.pushPool.Add(pushNote, true)
				Expect(err).To(BeNil())

				mockTickMgr.EXPECT().GetTopHosts(gomock.Any()).Return([]*tickettypes.SelectedTicket{
					{
						Ticket: &tickettypes.Ticket{
							ProposerPubKey: key.PubKey().MustBytes32(),
							BLSPubKey:      key.PrivKey().BLSKey().Public().Bytes(),
						},
					},
				}, nil)
				pushEnd := &types.PushEndorsement{
					EndorserPubKey: key.PubKey().MustBytes32(),
				}
				var pushEndSig []byte
				pushEndSig, err = key.PrivKey().BLSKey().Sign(pushEnd.BytesNoSigAndSenderPubKey())
				Expect(err).To(BeNil())
				pushEnd.Sig = util.BytesToBytes64(pushEndSig)

				mockMempool.EXPECT().Add(gomock.AssignableToTypeOf(&txns.TxPush{})).Return(nil)
				svr.addPushNoteEndorsement(pushNote.ID().String(), pushEnd)
				err = svr.MaybeCreatePushTx(pushNote.ID().String())
			})

			It("should return no error", func() {
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".execTxPush", func() {
		var err error

		BeforeEach(func() {
			mockRemoteSrv.EXPECT().Cfg().Return(cfg).AnyTimes()
			mockRemoteSrv.EXPECT().GetDHT().Return(mockDHT).AnyTimes()
			mockRemoteSrv.EXPECT().Log().Return(cfg.G().Log).AnyTimes()

			mockPruner := mocks.NewMockRepoPruner(ctrl)
			mockPruner.EXPECT().Schedule(gomock.Any()).AnyTimes()
			mockRemoteSrv.EXPECT().GetPruner().Return(mockPruner).AnyTimes()
		})

		When("target repo does not exist locally", func() {
			BeforeEach(func() {
				tx := txns.NewBareTxPush()
				tx.PushNote.(*types.PushNote).RepoName = "unknown"
				mockRemoteSrv.EXPECT().GetRepo(tx.PushNote.(*types.PushNote).RepoName).Return(nil, fmt.Errorf("error"))
				err = execTxPush(mockRemoteSrv, tx)
			})

			It("should return err='unable to find repo locally: error'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("unable to find repo locally: error"))
			})
		})

		When("object existed locally", func() {
			BeforeEach(func() {
				tx := txns.NewBareTxPush()
				tx.PushNote.(*types.PushNote).RepoName = "repo1"

				obj := util.RandString(40)
				tx.PushNote.(*types.PushNote).References = []*types.PushedReference{
					{Objects: []string{obj}},
				}

				repo := mocks.NewMockLocalRepo(ctrl)
				repo.EXPECT().ObjectExist(obj).Return(true)
				mockRemoteSrv.EXPECT().UpdateRepoWithTxPush(tx).Return(nil)
				mockRemoteSrv.EXPECT().GetRepo(tx.PushNote.(*types.PushNote).RepoName).Return(repo, nil)

				err = execTxPush(mockRemoteSrv, tx)
			})

			It("should return no error", func() {
				Expect(err).To(BeNil())
			})
		})

		When("tx merge operation fail", func() {
			BeforeEach(func() {
				tx := txns.NewBareTxPush()
				tx.PushNote.(*types.PushNote).RepoName = "repo1"

				obj := util.RandString(40)
				tx.PushNote.(*types.PushNote).References = []*types.PushedReference{
					{Objects: []string{obj}},
				}

				repo := mocks.NewMockLocalRepo(ctrl)
				repo.EXPECT().ObjectExist(obj).Return(true)
				mockRemoteSrv.EXPECT().UpdateRepoWithTxPush(tx).Return(fmt.Errorf("error"))
				mockRemoteSrv.EXPECT().GetRepo(tx.PushNote.(*types.PushNote).RepoName).Return(repo, nil)

				err = execTxPush(mockRemoteSrv, tx)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("error"))
			})
		})

		When("an object does not exist and dht download failed", func() {
			BeforeEach(func() {
				tx := txns.NewBareTxPush()
				tx.PushNote.(*types.PushNote).RepoName = "repo1"

				obj := util.RandString(40)
				tx.PushNote.(*types.PushNote).References = []*types.PushedReference{
					{Objects: []string{obj}},
				}

				repo := mocks.NewMockLocalRepo(ctrl)
				repo.EXPECT().ObjectExist(obj).Return(false)
				mockDHT.EXPECT().GetObject(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("error"))
				mockRemoteSrv.EXPECT().GetRepo(tx.PushNote.(*types.PushNote).RepoName).Return(repo, nil)

				err = execTxPush(mockRemoteSrv, tx)
			})

			It("should return error='failed to fetch object...'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("failed to fetch object"))
			})
		})

		When("downloaded object cannot be written to disk", func() {
			BeforeEach(func() {
				tx := txns.NewBareTxPush()
				tx.PushNote.(*types.PushNote).RepoName = "repo1"

				obj := util.RandString(40)
				tx.PushNote.(*types.PushNote).References = []*types.PushedReference{
					{Objects: []string{obj}},
				}

				repo := mocks.NewMockLocalRepo(ctrl)
				repo.EXPECT().ObjectExist(obj).Return(false)

				objBz := util.RandBytes(10)
				mockDHT.EXPECT().GetObject(gomock.Any(), gomock.Any()).Return(objBz, nil)
				repo.EXPECT().WriteObjectToFile(obj, objBz).Return(fmt.Errorf("error"))
				mockRemoteSrv.EXPECT().GetRepo(tx.PushNote.(*types.PushNote).RepoName).Return(repo, nil)

				err = execTxPush(mockRemoteSrv, tx)
			})

			It("should return error='failed to write fetched object...'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("failed to write fetched object"))
			})
		})

		When("object download succeeded but object announcement fails", func() {
			BeforeEach(func() {
				tx := txns.NewBareTxPush()
				tx.PushNote.(*types.PushNote).RepoName = "repo1"

				obj := util.RandString(40)
				tx.PushNote.(*types.PushNote).References = []*types.PushedReference{
					{Objects: []string{obj}},
				}

				repo := mocks.NewMockLocalRepo(ctrl)
				repo.EXPECT().ObjectExist(obj).Return(false)

				objBz := util.RandBytes(10)
				mockDHT.EXPECT().GetObject(gomock.Any(), gomock.Any()).Return(objBz, nil)
				repo.EXPECT().WriteObjectToFile(obj, objBz).Return(nil)

				mockDHT.EXPECT().Announce(gomock.Any(), gomock.Any()).Return(fmt.Errorf("error"))
				mockRemoteSrv.EXPECT().UpdateRepoWithTxPush(tx).Return(nil)
				mockRemoteSrv.EXPECT().GetRepo(tx.PushNote.(*types.PushNote).RepoName).Return(repo, nil)
				err = execTxPush(mockRemoteSrv, tx)
			})

			It("should return no error", func() {
				Expect(err).To(BeNil())
			})
		})

		When("push note contains a pushed reference new hash set to zero-hash", func() {
			It("should attempt to delete the pushed reference and return error if it failed", func() {
				tx := txns.NewBareTxPush()
				tx.PushNote.(*types.PushNote).RepoName = repoName

				tx.PushNote.(*types.PushNote).References = []*types.PushedReference{
					{NewHash: plumbing.ZeroHash.String(), Name: "refs/heads/master"},
				}

				repo := mocks.NewMockLocalRepo(ctrl)
				repo.EXPECT().RefDelete("refs/heads/master").Return(fmt.Errorf("failed to delete"))

				mockRemoteSrv.EXPECT().UpdateRepoWithTxPush(tx).Return(nil)
				mockRemoteSrv.EXPECT().GetRepo(tx.PushNote.(*types.PushNote).RepoName).Return(repo, nil)
				err = execTxPush(mockRemoteSrv, tx)

				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to delete reference (refs/heads/master): failed to delete"))
			})

			It("should attempt to delete the pushed reference and return nil if it succeeded", func() {
				tx := txns.NewBareTxPush()
				tx.PushNote.(*types.PushNote).RepoName = repoName

				tx.PushNote.(*types.PushNote).References = []*types.PushedReference{
					{NewHash: plumbing.ZeroHash.String(), Name: "refs/heads/master"},
				}

				repo := mocks.NewMockLocalRepo(ctrl)
				repo.EXPECT().RefDelete("refs/heads/master").Return(nil)

				mockRemoteSrv.EXPECT().UpdateRepoWithTxPush(tx).Return(nil)
				mockRemoteSrv.EXPECT().GetRepo(tx.PushNote.(*types.PushNote).RepoName).Return(repo, nil)
				err = execTxPush(mockRemoteSrv, tx)

				Expect(err).To(BeNil())
			})
		})
	})
})
