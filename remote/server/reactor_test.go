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
	"github.com/themakeos/lobe/config"
	"github.com/themakeos/lobe/crypto"
	"github.com/themakeos/lobe/mocks"
	"github.com/themakeos/lobe/params"
	plumbing2 "github.com/themakeos/lobe/remote/plumbing"
	"github.com/themakeos/lobe/remote/policy"
	"github.com/themakeos/lobe/remote/push"
	"github.com/themakeos/lobe/remote/push/types"
	"github.com/themakeos/lobe/remote/repo"
	testutil2 "github.com/themakeos/lobe/remote/testutil"
	remotetypes "github.com/themakeos/lobe/remote/types"
	"github.com/themakeos/lobe/remote/validation"
	"github.com/themakeos/lobe/testutil"
	tickettypes "github.com/themakeos/lobe/ticket/types"
	types2 "github.com/themakeos/lobe/types"
	"github.com/themakeos/lobe/types/core"
	"github.com/themakeos/lobe/types/state"
	"github.com/themakeos/lobe/types/txns"
	"github.com/themakeos/lobe/util"
	crypto2 "github.com/themakeos/lobe/util/crypto"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

func configIgnoreDenyCurrentBranch(r remotetypes.LocalRepo) error {
	c, _ := r.Config()
	c.Raw.SetOption("receive", "", "denyCurrentBranch", "ignore")
	return r.SetConfig(c)
}

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
	var mockDHT *mocks.MockDHT
	var mockTickMgr *mocks.MockTicketManager
	var mockNS *mocks.MockNamespaceKeeper
	var key = crypto.NewKeyFromIntSeed(1)
	var testRepo remotetypes.LocalRepo

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

		mockObjects := testutil.MockLogic(ctrl)
		mockLogic = mockObjects.Logic
		mockRepoKeeper = mockObjects.RepoKeeper
		mockDHT = mocks.NewMockDHT(ctrl)
		mockBlockGetter = mocks.NewMockBlockGetter(ctrl)
		mockMempool = mocks.NewMockMempool(ctrl)
		mockTickMgr = mockObjects.TicketManager
		mockNS = mockObjects.NamespaceKeeper
		svr = NewRemoteServer(cfg, ":9000", mockLogic, mockDHT, mockMempool, mockBlockGetter)

		mockPeer = mocks.NewMockPeer(ctrl)
	})

	AfterEach(func() {
		svr.Stop()
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".onPushNoteReceived", func() {
		When("in validator mode", func() {
			It("should return nil", func() {
				cfg.Node.Validator = true
				err = svr.onPushNoteReceived(mockPeer, util.RandBytes(10))
				Expect(err).To(BeNil())
			})
		})

		When("unable to decode msg", func() {
			It("should return err=failed to decoded message...", func() {
				err = svr.onPushNoteReceived(mockPeer, util.RandBytes(5))
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("failed to decoded message"))
			})
		})

		When("target repo does not exist locally", func() {
			BeforeEach(func() {
				mockPeer.EXPECT().ID().Return(p2p.ID("peer-id"))
				mockRepoKeeper.EXPECT().Get("unknown").Return(state.BareRepository())
				pn := &types.Note{RepoName: "unknown"}
				err = svr.onPushNoteReceived(mockPeer, pn.Bytes())
			})

			It("should return err=`repo 'unknown' not found`", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("repo 'unknown' not found"))
			})
		})

		When("namespace is set but it is unknown", func() {
			BeforeEach(func() {
				mockPeer.EXPECT().ID().Return(p2p.ID("peer-id"))
				repoState := state.BareRepository()
				repoState.Balance = "100"
				mockRepoKeeper.EXPECT().Get("repo1").Return(repoState)
				mockNS.EXPECT().Get(crypto2.MakeNamespaceHash("ns1")).Return(state.BareNamespace())
				pn := &types.Note{RepoName: "repo1", Namespace: "ns1"}
				err = svr.onPushNoteReceived(mockPeer, pn.Bytes())
			})

			It("should return err=`namespace 'ns1' not found`", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("namespace 'ns1' not found"))
			})
		})

		When("unable to open target repository", func() {
			BeforeEach(func() {
				mockPeer.EXPECT().ID().Return(p2p.ID("peer-id"))
				repoState := state.BareRepository()
				repoState.Balance = "100"
				mockRepoKeeper.EXPECT().Get("repo1").Return(repoState)
				svr.authenticate = func(txDetails []*remotetypes.TxDetail, repo *state.Repository, namespace *state.Namespace, keepers core.Keepers, checkTxDetail validation.TxDetailChecker) (enforcer policy.EnforcerFunc, err error) {
					return nil, nil
				}
				pn := &types.Note{RepoName: "repo1"}
				err = svr.onPushNoteReceived(mockPeer, pn.Bytes())
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to open repo 'repo1': repository does not exist"))
			})
		})

		When("authentication fails", func() {
			BeforeEach(func() {
				mockPeer.EXPECT().ID().Return(p2p.ID("peer-id"))
				repoState := state.BareRepository()
				repoState.Balance = "100"
				mockRepoKeeper.EXPECT().Get(repoName).Return(repoState)
				svr.authenticate = func(txDetails []*remotetypes.TxDetail, repo *state.Repository, namespace *state.Namespace, keepers core.Keepers, checkTxDetail validation.TxDetailChecker) (enforcer policy.EnforcerFunc, err error) {
					return nil, fmt.Errorf("bad error")
				}
				pn := &types.Note{RepoName: repoName}
				err = svr.onPushNoteReceived(mockPeer, pn.Bytes())
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("authorization failed: bad error"))
			})
		})

		When("push note validation fail", func() {
			BeforeEach(func() {
				mockPeer.EXPECT().ID().Return(p2p.ID("peer-id"))
				repoState := state.BareRepository()
				repoState.Balance = "100"
				mockRepoKeeper.EXPECT().Get(repoName).Return(repoState)
				svr.authenticate = func(txDetails []*remotetypes.TxDetail, repo *state.Repository, namespace *state.Namespace, keepers core.Keepers, checkTxDetail validation.TxDetailChecker) (policy.EnforcerFunc, error) {
					return nil, nil
				}
				svr.checkPushNote = func(tx types.PushNote, logic core.Logic) error {
					return fmt.Errorf("error")
				}
				pn := &types.Note{RepoName: repoName}
				err = svr.onPushNoteReceived(mockPeer, pn.Bytes())
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed push note validation: error"))
			})
		})

		When("push note validation passes", func() {
			var pn *types.Note

			BeforeEach(func() {
				mockPeer.EXPECT().ID().Return(p2p.ID("peer-id"))
				repoState := state.BareRepository()
				repoState.Balance = "100"
				mockRepoKeeper.EXPECT().Get(repoName).Return(repoState)
				svr.authenticate = func(txDetails []*remotetypes.TxDetail, repo *state.Repository, namespace *state.Namespace, keepers core.Keepers, checkTxDetail validation.TxDetailChecker) (policy.EnforcerFunc, error) {
					return nil, nil
				}
				svr.checkPushNote = func(tx types.PushNote, logic core.Logic) error {
					return nil
				}
				pn = &types.Note{RepoName: repoName}
				err = svr.onPushNoteReceived(mockPeer, pn.Bytes())
			})

			It("should return no err", func() {
				Expect(err).To(BeNil())
			})

			It("should register peer as note sender", func() {
				yes := svr.isNoteSender("peer-id", pn.ID().String())
				Expect(yes).To(BeTrue())
			})

			It("should add fetch task to object fetcher", func() {
				Expect(svr.objfetcher.QueueSize()).To(Equal(1))
			})
		})

	})

	Describe(".onFetch", func() {
		It("should return error when err is passed", func() {
			polEnforcer := func(subject, object, action string) (bool, int) { return false, 0 }
			err := svr.onFetch(fmt.Errorf("error"), &types.Note{}, []*remotetypes.TxDetail{}, polEnforcer)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		It("should return error when unable to get pushed objects size", func() {
			mockNote := mocks.NewMockPushNote(ctrl)
			mockNote.EXPECT().ID().Return(util.StrToBytes32("note_123"))
			mockNote.EXPECT().GetRepoName().Return("repo1")
			mockNote.EXPECT().GetTargetRepo().Return(nil)
			polEnforcer := func(subject, object, action string) (bool, int) { return false, 0 }
			err := svr.onFetch(nil, mockNote, []*remotetypes.TxDetail{}, polEnforcer)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to get pushed refs objects size: repo is required"))
		})

		It("should return error when note object size and local size don't match", func() {
			mockNote := mocks.NewMockPushNote(ctrl)
			mockNote.EXPECT().ID().Return(util.StrToBytes32("note_123"))
			mockNote.EXPECT().GetRepoName().Return("repo1")
			mockNote.EXPECT().GetTargetRepo().Return(testRepo)
			mockNote.EXPECT().GetPushedReferences().Return(types.PushedReferences{})
			mockNote.EXPECT().SetLocalSize(uint64(0))
			mockNote.EXPECT().IsFromRemotePeer().Return(true)
			mockNote.EXPECT().GetSize().Return(uint64(100))
			polEnforcer := func(subject, object, action string) (bool, int) { return false, 0 }
			err := svr.onFetch(nil, mockNote, []*remotetypes.TxDetail{}, polEnforcer)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("note's objects size and local size differs"))
		})

		It("should return error when unable to process push note", func() {
			mockNote := mocks.NewMockPushNote(ctrl)
			mockNote.EXPECT().ID().Return(util.StrToBytes32("note_123"))
			mockNote.EXPECT().GetRepoName().Return("repo1")
			mockNote.EXPECT().GetTargetRepo().Return(testRepo)
			mockNote.EXPECT().GetPushedReferences().Return(types.PushedReferences{})
			mockNote.EXPECT().SetLocalSize(uint64(0))
			mockNote.EXPECT().IsFromRemotePeer().Return(true)
			mockNote.EXPECT().GetSize().Return(uint64(0))
			polEnforcer := func(subject, object, action string) (bool, int) { return false, 0 }
			svr.processPushNote = func(note types.PushNote, txDetails []*remotetypes.TxDetail, polEnforcer policy.EnforcerFunc) error {
				return fmt.Errorf("error")
			}
			err := svr.onFetch(nil, mockNote, []*remotetypes.TxDetail{}, polEnforcer)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		It("should return no error when able to process push note", func() {
			mockNote := mocks.NewMockPushNote(ctrl)
			mockNote.EXPECT().ID().Return(util.StrToBytes32("note_123"))
			mockNote.EXPECT().GetRepoName().Return("repo1")
			mockNote.EXPECT().GetTargetRepo().Return(testRepo)
			mockNote.EXPECT().GetPushedReferences().Return(types.PushedReferences{})
			mockNote.EXPECT().SetLocalSize(uint64(0))
			mockNote.EXPECT().IsFromRemotePeer().Return(true)
			mockNote.EXPECT().GetSize().Return(uint64(0))
			polEnforcer := func(subject, object, action string) (bool, int) { return false, 0 }
			svr.processPushNote = func(note types.PushNote, txDetails []*remotetypes.TxDetail, polEnforcer policy.EnforcerFunc) error {
				return nil
			}
			err := svr.onFetch(nil, mockNote, []*remotetypes.TxDetail{}, polEnforcer)
			Expect(err).To(BeNil())
		})
	})

	Describe(".createEndorsement", func() {
		It("should return error when unable to get reference from repo", func() {
			note := &types.Note{References: []*types.PushedReference{{Name: "refs/heads/master"}}}
			mockRepo := mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().RefGet(note.References[0].Name).Return("", fmt.Errorf("error"))
			note.SetTargetRepo(mockRepo)
			_, err := createEndorsement(svr.validatorKey, note)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to get hash of reference (refs/heads/master): error"))
		})

		When("a pushed reference does not exist locally", func() {
			var err error
			var end *types.PushEndorsement
			BeforeEach(func() {
				note := &types.Note{References: []*types.PushedReference{{Name: "refs/heads/master"}}}
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				mockRepo.EXPECT().RefGet(note.References[0].Name).Return("", plumbing2.ErrRefNotFound)
				note.SetTargetRepo(mockRepo)
				end, err = createEndorsement(svr.validatorKey, note)
			})

			It("should return no error", func() {
				Expect(err).To(BeNil())
			})

			It("should create endorsement with 1 reference", func() {
				Expect(end.References).To(HaveLen(1))
			})

			Specify("that the reference hash is unset", func() {
				Expect(end.References[0].Hash).To(BeEmpty())
			})

			Specify("that the endorsement is signed", func() {
				Expect(end.SigBLS).To(HaveLen(64))
			})
		})

		When("a pushed reference exists locally", func() {
			var err error
			var end *types.PushEndorsement
			var refHash = "8d998c7de21bbe561f7992bb983cef4b1554993b"

			BeforeEach(func() {
				note := &types.Note{References: []*types.PushedReference{{Name: "refs/heads/master"}}}
				mockRepo := mocks.NewMockLocalRepo(ctrl)
				mockRepo.EXPECT().RefGet(note.References[0].Name).Return(refHash, nil)
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
				commitHash := testutil2.GetRecentCommitHash(path, "refs/heads/master")
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
				Expect(err.Error()).To(MatchRegexp("failed to start git-receive-pack command"))
			})

			It("should return error if unable to handle incoming stream", func() {
				svr.makeReferenceUpdatePack = func(tx types.PushNote) (io.ReadSeeker, error) { return pack, nil }
				svr.makePushHandler = func(targetRepo remotetypes.LocalRepo, txDetails []*remotetypes.TxDetail, enforcer policy.EnforcerFunc) push.Handler {
					mockHandler := mocks.NewMockHandler(ctrl)
					mockHandler.EXPECT().SetGitReceivePackCmd(gomock.Any())
					mockHandler.EXPECT().HandleStream(gomock.Any(), gomock.Any()).Return(fmt.Errorf("error"))
					return mockHandler
				}
				note := &types.Note{}
				note.SetTargetRepo(testRepo)
				err := svr.maybeProcessPushNote(note, []*remotetypes.TxDetail{}, nil)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("HandleStream error: error"))
			})

			It("should return error if unable to handle references", func() {
				svr.makeReferenceUpdatePack = func(tx types.PushNote) (io.ReadSeeker, error) { return pack, nil }
				svr.makePushHandler = func(targetRepo remotetypes.LocalRepo, txDetails []*remotetypes.TxDetail,
					enforcer policy.EnforcerFunc) push.Handler {
					mockHandler := mocks.NewMockHandler(ctrl)
					mockHandler.EXPECT().SetGitReceivePackCmd(gomock.Any())
					mockHandler.EXPECT().HandleStream(gomock.Any(), gomock.Any()).Return(nil)
					mockHandler.EXPECT().HandleReferences().Return(fmt.Errorf("error"))
					return mockHandler
				}
				note := &types.Note{}
				note.SetTargetRepo(testRepo)
				err := svr.maybeProcessPushNote(note, []*remotetypes.TxDetail{}, nil)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("HandleReferences error: error"))
			})

			It("should return error if unable to add note to push pool", func() {
				svr.makeReferenceUpdatePack = func(tx types.PushNote) (io.ReadSeeker, error) { return pack, nil }
				svr.makePushHandler = func(targetRepo remotetypes.LocalRepo, txDetails []*remotetypes.TxDetail,
					enforcer policy.EnforcerFunc) push.Handler {
					mockHandler := mocks.NewMockHandler(ctrl)
					mockHandler.EXPECT().SetGitReceivePackCmd(gomock.Any())
					mockHandler.EXPECT().HandleStream(gomock.Any(), gomock.Any()).Return(nil)
					mockHandler.EXPECT().HandleReferences().Return(nil)
					return mockHandler
				}
				note := &types.Note{}
				note.SetTargetRepo(testRepo)
				mockPushPool := mocks.NewMockPushPool(ctrl)
				mockPushPool.EXPECT().Add(note, true).Return(fmt.Errorf("error"))
				svr.pushPool = mockPushPool
				err = svr.maybeProcessPushNote(note, []*remotetypes.TxDetail{}, nil)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("failed to add push note to push pool: error"))
			})

			It("should broadcast note and return no error", func() {
				svr.makeReferenceUpdatePack = func(tx types.PushNote) (io.ReadSeeker, error) { return pack, nil }
				svr.makePushHandler = func(targetRepo remotetypes.LocalRepo, txDetails []*remotetypes.TxDetail,
					enforcer policy.EnforcerFunc) push.Handler {
					mockHandler := mocks.NewMockHandler(ctrl)
					mockHandler.EXPECT().SetGitReceivePackCmd(gomock.Any())
					mockHandler.EXPECT().HandleStream(gomock.Any(), gomock.Any()).Return(nil)
					mockHandler.EXPECT().HandleReferences().Return(nil)
					return mockHandler
				}
				note := &types.Note{}
				note.SetTargetRepo(testRepo)
				mockPushPool := mocks.NewMockPushPool(ctrl)
				mockPushPool.EXPECT().Add(note, true).Return(nil)
				svr.pushPool = mockPushPool
				broadcasted := false
				svr.noteAndEndorserBroadcaster = func(types.PushNote) error {
					broadcasted = true
					return nil
				}
				err = svr.maybeProcessPushNote(note, []*remotetypes.TxDetail{}, nil)
				Expect(err).To(BeNil())
				Expect(broadcasted).To(BeTrue())
			})

			It("should return no error if note failed broadcast", func() {
				svr.makeReferenceUpdatePack = func(tx types.PushNote) (io.ReadSeeker, error) { return pack, nil }
				svr.makePushHandler = func(targetRepo remotetypes.LocalRepo, txDetails []*remotetypes.TxDetail,
					enforcer policy.EnforcerFunc) push.Handler {
					mockHandler := mocks.NewMockHandler(ctrl)
					mockHandler.EXPECT().SetGitReceivePackCmd(gomock.Any())
					mockHandler.EXPECT().HandleStream(gomock.Any(), gomock.Any()).Return(nil)
					mockHandler.EXPECT().HandleReferences().Return(nil)
					return mockHandler
				}
				note := &types.Note{}
				note.SetTargetRepo(testRepo)
				mockPushPool := mocks.NewMockPushPool(ctrl)
				mockPushPool.EXPECT().Add(note, true).Return(nil)
				svr.pushPool = mockPushPool
				broadcastDone := false
				svr.noteAndEndorserBroadcaster = func(types.PushNote) error {
					broadcastDone = true
					return fmt.Errorf("error")
				}
				err = svr.maybeProcessPushNote(note, []*remotetypes.TxDetail{}, nil)
				Expect(err).To(BeNil())
				Expect(broadcastDone).To(BeTrue())
			})
		})
	})

	Describe(".onEndorsementReceived", func() {
		It("should return no error (do nothing) if node is a validator", func() {
			cfg.Node.Validator = true
			err = svr.onEndorsementReceived(mockPeer, util.RandBytes(5))
			Expect(err).To(BeNil())
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
			var senderPeerID p2p.ID = "peer_id"
			var end *types.PushEndorsement
			var endorsementWasBroadcast bool

			BeforeEach(func() {
				svr.checkEndorsement = func(end *types.PushEndorsement, logic core.Logic, index int) error { return nil }
				svr.endorsementBroadcaster = func(endorsement types.Endorsement) {
					endorsementWasBroadcast = true
				}
				end = &types.PushEndorsement{NoteID: util.RandBytes(32)}
				mockPeer.EXPECT().ID().Return(senderPeerID)
				err = svr.onEndorsementReceived(mockPeer, end.Bytes())
				Expect(err).To(BeNil())
			})

			Specify("that the sending peer is register as the sender of the push endorsement", func() {
				Expect(svr.isEndorsementSender(string(senderPeerID), end.ID().String())).To(BeTrue())
			})

			Specify("that the endorsement was register to the push note", func() {
				ends := svr.endorsementsReceived.Get(end.NoteID.String())
				Expect(ends).ToNot(BeNil())
				Expect(ends).To(HaveKey(end.ID().String()))
				expected := ends.(map[string]*types.PushEndorsement)[end.ID().String()]
				Expect(expected.Bytes()).To(Equal(end.Bytes()))
			})

			Specify("that the endorsement was broadcast", func() {
				Expect(endorsementWasBroadcast).To(BeTrue())
			})
		})

		When("unable to create push transaction in response to a valid endorsement", func() {
			var senderPeerID p2p.ID = "peer_id"
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
				mockPeer.EXPECT().ID().Return(senderPeerID)
				err = svr.onEndorsementReceived(mockPeer, end.Bytes())
				Expect(err).To(BeNil())
				Expect(endorsementWasBroadcast).To(BeTrue())
			})
		})
	})

	Describe(".BroadcastNoteAndEndorsement", func() {
		It("should return error when unable to get top tickets", func() {
			svr.noteBroadcaster = func(pushNote types.PushNote) {}
			mockTickMgr.EXPECT().GetTopHosts(gomock.Any()).Return(nil, fmt.Errorf("error"))
			err := svr.BroadcastNoteAndEndorsement(&types.Note{})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to get top hosts: error"))
		})

		It("should return nil when no top selected tickets", func() {
			svr.noteBroadcaster = func(pushNote types.PushNote) {}
			tickets := tickettypes.SelectedTickets{}
			mockTickMgr.EXPECT().GetTopHosts(gomock.Any()).Return(tickets, nil)
			err := svr.BroadcastNoteAndEndorsement(&types.Note{})
			Expect(err).To(BeNil())
		})

		It("should return error when unable to create endorsement", func() {
			svr.noteBroadcaster = func(pushNote types.PushNote) {}
			ticket := &tickettypes.SelectedTicket{Ticket: &tickettypes.Ticket{
				ProposerPubKey: svr.validatorKey.PubKey().MustBytes32(),
			}}
			tickets := tickettypes.SelectedTickets{ticket}
			svr.endorsementCreator = func(validatorKey *crypto.Key, note types.PushNote) (*types.PushEndorsement, error) {
				return nil, fmt.Errorf("error")
			}
			mockTickMgr.EXPECT().GetTopHosts(gomock.Any()).Return(tickets, nil)
			err := svr.BroadcastNoteAndEndorsement(&types.Note{})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		When("endorsement is created successfully", func() {
			var endorsementBroadcast bool
			var madePushTx bool
			var note = &types.Note{RepoName: "repo1"}
			var end = &types.PushEndorsement{NoteID: []byte{1, 2, 3}}

			BeforeEach(func() {
				svr.noteBroadcaster = func(pushNote types.PushNote) {}
				ticket := &tickettypes.SelectedTicket{Ticket: &tickettypes.Ticket{
					ProposerPubKey: svr.validatorKey.PubKey().MustBytes32(),
				}}
				tickets := tickettypes.SelectedTickets{ticket}

				svr.endorsementCreator = func(validatorKey *crypto.Key, note types.PushNote) (*types.PushEndorsement, error) {
					return end, nil
				}

				svr.endorsementBroadcaster = func(endorsement types.Endorsement) {
					endorsementBroadcast = true
				}

				svr.makePushTx = func(noteID string) error {
					madePushTx = true
					return nil
				}
				mockTickMgr.EXPECT().GetTopHosts(gomock.Any()).Return(tickets, nil)
				err := svr.BroadcastNoteAndEndorsement(note)
				Expect(err).To(BeNil())
			})

			It("should broadcast the endorsement", func() {
				Expect(endorsementBroadcast).To(BeTrue())
			})

			It("should make push transaction", func() {
				Expect(madePushTx).To(BeTrue())
			})

			It("should register endorsement to the push note", func() {
				noteEnds := svr.endorsementsReceived.Get(note.ID().String())
				Expect(noteEnds).To(HaveLen(1))
				Expect(noteEnds).To(HaveKey(end.ID().String()))
			})
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
				svr.registerEndorsementOfNote(pushNoteID, &types.PushEndorsement{SigBLS: util.RandBytes(5)})
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
				svr.registerEndorsementOfNote(pushNoteID, &types.PushEndorsement{SigBLS: util.RandBytes(5)})
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
				var pushNote = &types.Note{RepoName: "repo1"}
				err = svr.pushPool.Add(pushNote, true)
				Expect(err).To(BeNil())

				mockTickMgr.EXPECT().GetTopHosts(gomock.Any()).Return(nil, fmt.Errorf("error"))
				svr.registerEndorsementOfNote(pushNote.ID().String(), &types.PushEndorsement{SigBLS: util.RandBytes(5)})
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
				var pushNote = &types.Note{RepoName: "repo1"}
				err = svr.pushPool.Add(pushNote, true)
				Expect(err).To(BeNil())

				mockTickMgr.EXPECT().GetTopHosts(gomock.Any()).Return([]*tickettypes.SelectedTicket{}, nil)
				end := &types.PushEndorsement{
					SigBLS:         util.RandBytes(5),
					EndorserPubKey: util.BytesToBytes32(util.RandBytes(32)),
				}
				svr.registerEndorsementOfNote(pushNote.ID().String(), end)
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
				var pushNote = &types.Note{RepoName: "repo1"}
				err = svr.pushPool.Add(pushNote, true)
				Expect(err).To(BeNil())

				mockTickMgr.EXPECT().GetTopHosts(gomock.Any()).Return([]*tickettypes.SelectedTicket{
					{Ticket: &tickettypes.Ticket{ProposerPubKey: key.PubKey().MustBytes32(), BLSPubKey: []byte("invalid bls public key")}},
				}, nil)

				end := &types.PushEndorsement{
					EndorserPubKey: key.PubKey().MustBytes32(),
				}

				svr.registerEndorsementOfNote(pushNote.ID().String(), end)
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
				var pushNote = &types.Note{RepoName: "repo1"}
				err = svr.pushPool.Add(pushNote, true)
				Expect(err).To(BeNil())

				mockTickMgr.EXPECT().GetTopHosts(gomock.Any()).Return([]*tickettypes.SelectedTicket{
					{Ticket: &tickettypes.Ticket{ProposerPubKey: key.PubKey().MustBytes32(), BLSPubKey: key.PrivKey().BLSKey().Public().Bytes()}},
				}, nil)

				end := &types.PushEndorsement{
					EndorserPubKey: key.PubKey().MustBytes32(),
					SigBLS:         util.RandBytes(5),
				}

				svr.registerEndorsementOfNote(pushNote.ID().String(), end)
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
				var pushNote = &types.Note{RepoName: "repo1"}
				err = svr.pushPool.Add(pushNote, true)
				Expect(err).To(BeNil())

				mockTickMgr.EXPECT().GetTopHosts(gomock.Any()).Return([]*tickettypes.SelectedTicket{
					{Ticket: &tickettypes.Ticket{ProposerPubKey: key.PubKey().MustBytes32(), BLSPubKey: key.PrivKey().BLSKey().Public().Bytes()}},
				}, nil)

				end := &types.PushEndorsement{NoteID: []byte{1, 2, 3}, EndorserPubKey: key.PubKey().MustBytes32()}
				end.SigBLS, err = key.PrivKey().BLSKey().Sign(end.BytesForBLSSig())
				Expect(err).To(BeNil())
				svr.registerEndorsementOfNote(pushNote.ID().String(), end)

				end2 := &types.PushEndorsement{NoteID: []byte{1, 2, 4}, EndorserPubKey: key.PubKey().MustBytes32()}
				end2.SigBLS, err = key.PrivKey().BLSKey().Sign(end2.BytesForBLSSig())
				Expect(err).To(BeNil())
				svr.registerEndorsementOfNote(pushNote.ID().String(), end2)

				mockMempool.EXPECT().Add(gomock.AssignableToTypeOf(&txns.TxPush{})).DoAndReturn(func(tx types2.BaseTx) error {
					// NoteID and SigBLS fields must be unset
					Expect(tx.(*txns.TxPush).Endorsements).To(HaveLen(2))
					Expect(tx.(*txns.TxPush).Endorsements[0].NoteID).To(BeNil())
					Expect(tx.(*txns.TxPush).Endorsements[0].SigBLS).To(BeNil())
					Expect(tx.(*txns.TxPush).Endorsements[1].NoteID).To(BeNil())
					Expect(tx.(*txns.TxPush).Endorsements[1].SigBLS).To(BeNil())

					// 0-index reference must be non-nil. Greater than 0-index must be nil
					Expect(tx.(*txns.TxPush).Endorsements[0].References).ToNot(BeNil())
					Expect(tx.(*txns.TxPush).Endorsements[1].References).To(BeNil())
					return nil
				})
				err = svr.createPushTx(pushNote.ID().String())

				Expect(err).To(BeNil())
			})
		})
	})
})
