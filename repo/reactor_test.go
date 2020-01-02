package repo

import (
	"fmt"
	"os"
	"path/filepath"

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
	var path string
	var repo types.BareRepo
	var mockMgr *mocks.MockRepoManager
	var mgr *Manager
	var ctrl *gomock.Controller
	var mockLogic *mocks.MockLogic
	var repoName string
	var mockMempool *mocks.MockMempool
	var mockPeer *mocks.MockPeer
	var mockRepoKeeper *mocks.MockRepoKeeper
	var key = crypto.NewKeyFromIntSeed(1)

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
		ctrl = gomock.NewController(GinkgoT())

		repoName = util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		execGit(cfg.GetRepoRoot(), "init", repoName)
		repo, err = getRepoWithGitOpt(cfg.Node.GitBinPath, path)
		Expect(err).To(BeNil())

		mockObjects := testutil.MockLogic(ctrl)
		mockLogic = mockObjects.Logic
		mockRepoKeeper = mockObjects.RepoKeeper
		mockDHT := mocks.NewMockDHT(ctrl)
		mockMempool = mocks.NewMockMempool(ctrl)
		mgr = NewManager(cfg, ":9000", mockLogic, mockDHT, mockMempool)

		mockMgr = mocks.NewMockRepoManager(ctrl)
		mockPeer = mocks.NewMockPeer(ctrl)
		_ = repo
		_ = mgr
		_ = mockMgr
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

		When("unable to verify PushOK signature", func() {
			BeforeEach(func() {
				mockPeer.EXPECT().ID().Return(p2p.ID("peer-id")).Times(2)
				pushOK := &types.PushOK{
					PushNoteID:   util.StrToBytes32("pushID"),
					Sig:          util.BytesToBytes64(util.RandBytes(5)),
					SenderPubKey: util.BytesToBytes32(util.RandBytes(5)),
				}
				err = mgr.onPushOK(mockPeer, pushOK.Bytes())
			})

			It("should return err=push ok signature failed verification...", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("push ok signature failed verification: invalid signature"))
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
})
