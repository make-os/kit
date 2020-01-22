package repo

import (
	"fmt"
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tendermint/tendermint/p2p"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/params"
	"github.com/makeos/mosdef/testutil"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/types/mocks"
	"github.com/makeos/mosdef/util"
)

var _ = Describe("Reactor", func() {
	var err error
	var cfg *config.AppConfig
	var mgr *Manager
	var ctrl *gomock.Controller
	var mockLogic *mocks.MockLogic
	var repoName string
	var mockMempool *mocks.MockMempool
	var mockPeer *mocks.MockPeer
	var mockRepoKeeper *mocks.MockRepoKeeper
	var mockBlockGetter *mocks.MockBlockGetter
	var mockDHT *mocks.MockDHT
	var mockMgr *mocks.MockRepoManager
	var key = crypto.NewKeyFromIntSeed(1)

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
		ctrl = gomock.NewController(GinkgoT())

		repoName = util.RandString(5)
		execGit(cfg.GetRepoRoot(), "init", repoName)

		mockObjects := testutil.MockLogic(ctrl)
		mockLogic = mockObjects.Logic
		mockMgr = mockObjects.RepoManager
		mockRepoKeeper = mockObjects.RepoKeeper
		mockDHT = mocks.NewMockDHT(ctrl)
		mockBlockGetter = mocks.NewMockBlockGetter(ctrl)
		mockMempool = mocks.NewMockMempool(ctrl)
		mgr = NewManager(cfg, ":9000", mockLogic, mockDHT, mockMempool, mockBlockGetter)

		mockPeer = mocks.NewMockPeer(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".onPushNote", func() {
		When("unable to decode msg", func() {
			BeforeEach(func() {
				err = mgr.onPushNote(mockPeer, util.RandBytes(5))
			})

			It("should return err=failed to decoded message...", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("failed to decoded message"))
			})
		})

		When("repo referenced in PushNote is not found", func() {
			BeforeEach(func() {
				mockPeer.EXPECT().ID().Return(p2p.ID("peer-id"))
				mockRepoKeeper.EXPECT().GetRepo("unknown").Return(types.BareRepository())
				pn := &types.PushNote{RepoName: "unknown"}
				err = mgr.onPushNote(mockPeer, pn.Bytes())
			})

			It("should return err=`repo 'unknown' not found`", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("repo 'unknown' not found"))
			})
		})

		When("unable to open the target repo", func() {
			BeforeEach(func() {
				mockPeer.EXPECT().ID().Return(p2p.ID("peer-id"))
				mockRepoKeeper.EXPECT().GetRepo("unknown").Return(&types.Repository{
					CreatorAddress: key.Addr(),
				})
				pn := &types.PushNote{RepoName: "unknown"}
				err = mgr.onPushNote(mockPeer, pn.Bytes())
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to open repo 'unknown': repository does not exist"))
			})
		})
	})

	Describe(".onPushOK", func() {
		When("unable to decode msg", func() {
			BeforeEach(func() {
				err = mgr.onPushOK(mockPeer, util.RandBytes(5))
			})

			It("should return err=failed to decoded message...", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("failed to decoded message"))
			})
		})
	})

	Describe(".MaybeCreatePushTx", func() {
		When("no PushOK for the given note", func() {
			var pushNoteID = "note1"
			BeforeEach(func() {
				err = mgr.MaybeCreatePushTx(pushNoteID)
			})

			It("should return err='no endorsements yet'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("no endorsements yet"))
			})
		})

		When("PushOKs for the given note is not up to the quorum size", func() {
			var pushNoteID = "note1"
			BeforeEach(func() {
				params.PushOKQuorumSize = 2
				mgr.addPushNoteEndorsement(pushNoteID, &types.PushOK{Sig: util.BytesToBytes64(util.RandBytes(5))})
				err = mgr.MaybeCreatePushTx(pushNoteID)
			})

			It("should return err='Not enough PushOKs to satisfy quorum size'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("Not enough PushOKs to satisfy quorum size"))
			})
		})

		When("PushNote does not exist in the pool", func() {
			var pushNoteID = "note1"
			BeforeEach(func() {
				params.PushOKQuorumSize = 1
				mgr.addPushNoteEndorsement(pushNoteID, &types.PushOK{Sig: util.BytesToBytes64(util.RandBytes(5))})
				err = mgr.MaybeCreatePushTx(pushNoteID)
			})

			It("should return err='push note not found in pool'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("push note not found in pool"))
			})
		})

		When("unable to add generated push tx to mempool", func() {
			BeforeEach(func() {
				params.PushOKQuorumSize = 1
				var pushNote = &types.PushNote{RepoName: "repo1"}
				err = mgr.pushPool.Add(pushNote, true)
				Expect(err).To(BeNil())

				mockMempool.EXPECT().Add(gomock.AssignableToTypeOf(&types.TxPush{})).Return(fmt.Errorf("error"))
				mgr.addPushNoteEndorsement(pushNote.ID().String(), &types.PushOK{Sig: util.BytesToBytes64(util.RandBytes(5))})
				err = mgr.MaybeCreatePushTx(pushNote.ID().String())
			})

			It("should return err='failed to add push tx to mempool: error'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to add push tx to mempool: error"))
			})
		})
	})

	Describe(".execTxPush", func() {
		var err error

		BeforeEach(func() {
			mockMgr.EXPECT().Cfg().Return(cfg).AnyTimes()
			mockMgr.EXPECT().GetDHT().Return(mockDHT).AnyTimes()
			mockMgr.EXPECT().Log().Return(cfg.G().Log).AnyTimes()
		})

		When("target repo does not exist locally", func() {
			BeforeEach(func() {
				tx := types.NewBareTxPush()
				tx.PushNote.RepoName = "unknown"
				mockMgr.EXPECT().GetRepo(tx.PushNote.RepoName).Return(nil, fmt.Errorf("error"))
				err = execTxPush(mockMgr, tx)
			})

			It("should return err='unable to find repo locally: error'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("unable to find repo locally: error"))
			})
		})

		When("object existed locally", func() {
			BeforeEach(func() {
				tx := types.NewBareTxPush()
				tx.PushNote.RepoName = "repo1"

				obj := util.RandString(40)
				tx.PushNote.References = types.PushedReferences([]*types.PushedReference{
					&types.PushedReference{Objects: []string{obj}},
				})

				repo := mocks.NewMockBareRepo(ctrl)
				repo.EXPECT().ObjectExist(obj).Return(true)
				mockMgr.EXPECT().UpdateRepoWithTxPush(tx).Return(nil)
				mockMgr.EXPECT().GetRepo(tx.PushNote.RepoName).Return(repo, nil)

				err = execTxPush(mockMgr, tx)
			})

			It("should return no error", func() {
				Expect(err).To(BeNil())
			})
		})

		When("tx merge operation fail", func() {
			BeforeEach(func() {
				tx := types.NewBareTxPush()
				tx.PushNote.RepoName = "repo1"

				obj := util.RandString(40)
				tx.PushNote.References = types.PushedReferences([]*types.PushedReference{
					&types.PushedReference{Objects: []string{obj}},
				})

				repo := mocks.NewMockBareRepo(ctrl)
				repo.EXPECT().ObjectExist(obj).Return(true)
				mockMgr.EXPECT().UpdateRepoWithTxPush(tx).Return(fmt.Errorf("error"))
				mockMgr.EXPECT().GetRepo(tx.PushNote.RepoName).Return(repo, nil)

				err = execTxPush(mockMgr, tx)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("error"))
			})
		})

		When("an object does not exist and dht download failed", func() {
			BeforeEach(func() {
				tx := types.NewBareTxPush()
				tx.PushNote.RepoName = "repo1"

				obj := util.RandString(40)
				tx.PushNote.References = types.PushedReferences([]*types.PushedReference{
					&types.PushedReference{Objects: []string{obj}},
				})

				repo := mocks.NewMockBareRepo(ctrl)
				repo.EXPECT().ObjectExist(obj).Return(false)
				mockDHT.EXPECT().GetObject(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("error"))
				mockMgr.EXPECT().GetRepo(tx.PushNote.RepoName).Return(repo, nil)

				err = execTxPush(mockMgr, tx)
			})

			It("should return error='failed to fetch object...'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("failed to fetch object"))
			})
		})

		When("downloaded object cannot be written to disk", func() {
			BeforeEach(func() {
				tx := types.NewBareTxPush()
				tx.PushNote.RepoName = "repo1"

				obj := util.RandString(40)
				tx.PushNote.References = types.PushedReferences([]*types.PushedReference{
					&types.PushedReference{Objects: []string{obj}},
				})

				repo := mocks.NewMockBareRepo(ctrl)
				repo.EXPECT().ObjectExist(obj).Return(false)

				objBz := util.RandBytes(10)
				mockDHT.EXPECT().GetObject(gomock.Any(), gomock.Any()).Return(objBz, nil)
				repo.EXPECT().WriteObjectToFile(obj, objBz).Return(fmt.Errorf("error"))
				mockMgr.EXPECT().GetRepo(tx.PushNote.RepoName).Return(repo, nil)

				err = execTxPush(mockMgr, tx)
			})

			It("should return error='failed to write fetched object...'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("failed to write fetched object"))
			})
		})

		When("object download succeeded but object announcement fails", func() {
			BeforeEach(func() {
				tx := types.NewBareTxPush()
				tx.PushNote.RepoName = "repo1"

				obj := util.RandString(40)
				tx.PushNote.References = types.PushedReferences([]*types.PushedReference{
					&types.PushedReference{Objects: []string{obj}},
				})

				repo := mocks.NewMockBareRepo(ctrl)
				repo.EXPECT().ObjectExist(obj).Return(false)

				objBz := util.RandBytes(10)
				mockDHT.EXPECT().GetObject(gomock.Any(), gomock.Any()).Return(objBz, nil)
				repo.EXPECT().WriteObjectToFile(obj, objBz).Return(nil)

				mockDHT.EXPECT().Annonce(gomock.Any(), gomock.Any()).Return(fmt.Errorf("error"))
				mockMgr.EXPECT().UpdateRepoWithTxPush(tx).Return(nil)
				mockMgr.EXPECT().GetRepo(tx.PushNote.RepoName).Return(repo, nil)
				err = execTxPush(mockMgr, tx)
			})

			It("should return no error", func() {
				Expect(err).To(BeNil())
			})
		})

		When("object download succeeded but pushed reference has a delete directive", func() {
			var repo *mocks.MockBareRepo
			var obj string
			var tx *types.TxPush
			var objBytes []byte

			BeforeEach(func() {
				tx = types.NewBareTxPush()
				tx.PushNote.RepoName = "repo1"

				obj = util.RandString(40)
				tx.PushNote.References = types.PushedReferences([]*types.PushedReference{
					&types.PushedReference{Name: "ref1", Objects: []string{obj}, Delete: true},
				})

				repo = mocks.NewMockBareRepo(ctrl)
				repo.EXPECT().ObjectExist(obj).Return(false)

				objBytes = util.RandBytes(10)
				mockDHT.EXPECT().GetObject(gomock.Any(), gomock.Any()).Return(objBytes, nil)

				mockDHT.EXPECT().Annonce(gomock.Any(), gomock.Any()).Return(fmt.Errorf("error"))
				mockMgr.EXPECT().UpdateRepoWithTxPush(tx).Return(nil)
				mockMgr.EXPECT().GetRepo(tx.PushNote.RepoName).Return(repo, nil)
			})

			It("should delete the reference", func() {
				repo.EXPECT().WriteObjectToFile(obj, objBytes).Return(nil)
				repo.EXPECT().RefDelete("ref1").Return(nil)
				repo.EXPECT().Path().Return("/path/to/repo")
				err = execTxPush(mockMgr, tx)
				Expect(err).To(BeNil())
			})
		})
	})
})
