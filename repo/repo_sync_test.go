package repo

import (
	"fmt"
	tmtypes "github.com/tendermint/tendermint/types"
	"os"
	"path/filepath"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/testutil"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/types/mocks"
	"github.com/makeos/mosdef/util"
)

var _ = Describe("Syncher", func() {
	var err error
	var cfg *config.AppConfig
	var path, dotGitPath string
	var repo types.BareRepo
	var mockMgr *mocks.MockRepoManager
	var ctrl *gomock.Controller
	var mockLogic *mocks.MockLogic
	var mockBlockGetter *mocks.MockBlockGetter
	var mockDHT *mocks.MockDHT
	var mockSysKeeper *mocks.MockSystemKeeper

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
	})

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		repoName := util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		dotGitPath = filepath.Join(path, ".git")
		execGit(cfg.GetRepoRoot(), "init", repoName)
		repo, err = getRepoWithGitOpt(cfg.Node.GitBinPath, path)
		Expect(err).To(BeNil())

		mockObjs := testutil.MockLogic(ctrl)
		mockMgr = mockObjs.RepoManager
		mockLogic = mockObjs.Logic
		mockBlockGetter = mockObjs.BlockGetter
		mockDHT = mocks.NewMockDHT(ctrl)
		mockSysKeeper = mockObjs.SysKeeper

		_ = dotGitPath
		_ = repo
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".start", func() {
		var syncher *Syncher
		var err error

		Context("start block not found", func() {
			BeforeEach(func() {
				mockBlockGetter.EXPECT().GetBlock(int64(1)).Return(nil)
				syncher = newSyncher(mockBlockGetter, mockMgr, mockMgr, mockLogic, mockDHT, cfg.G().Log)
				syncher.isSyncing = true
				err = syncher.start()
			})

			It("should return nil and set isSyncing to false", func() {
				Expect(err).To(BeNil())
				Expect(syncher.isSyncing).To(BeFalse())
			})
		})

		When("unable to decode a transaction in a block", func() {
			BeforeEach(func() {
				block := &tmtypes.Block{}
				block.Txs = append(block.Txs, []byte("abcdef"))
				mockBlockGetter.EXPECT().GetBlock(int64(1)).Return(block)
				syncher = newSyncher(mockBlockGetter, mockMgr, mockMgr, mockLogic, mockDHT, cfg.G().Log)
				err = syncher.start()
			})

			It("should return err='failed to decode tx'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to decode tx"))
			})
		})

		When("no transaction in the block is a TxPush", func() {
			BeforeEach(func() {
				tx := types.NewBareTxCoinTransfer()
				block := &tmtypes.Block{}
				block.Txs = append(block.Txs, tx.Bytes())

				mockBlockGetter.EXPECT().GetBlock(int64(1)).Return(block)
				mockBlockGetter.EXPECT().GetBlock(int64(2)).Return(nil)

				syncher = newSyncher(mockBlockGetter, mockMgr, mockMgr, mockLogic, mockDHT, cfg.G().Log)
				err = syncher.start()
			})

			It("should return nil", func() {
				Expect(err).To(BeNil())
			})
		})

		When("more blocks greater than the last recorded height have been processed ", func() {
			BeforeEach(func() {
				tx := types.NewBareTxCoinTransfer()
				block := &tmtypes.Block{}
				block.Txs = append(block.Txs, tx.Bytes())

				mockBlockGetter.EXPECT().GetBlock(int64(1)).Return(block)
				mockBlockGetter.EXPECT().GetBlock(int64(2)).Return(block)
				mockBlockGetter.EXPECT().GetBlock(int64(3)).Return(nil)
				mockSysKeeper.EXPECT().SetLastRepoObjectsSyncHeight(uint64(2)).Return(nil)
				mockLogic.EXPECT().ManagedSysKeeper().Return(mockSysKeeper)

				syncher = newSyncher(mockBlockGetter, mockMgr, mockMgr, mockLogic, mockDHT, cfg.G().Log)
				err = syncher.start()
			})

			It("should return nil", func() {
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".syncTx", func() {
		var syncher *Syncher
		var err error

		When("target repo does not exist locally", func() {
			BeforeEach(func() {
				tx := types.NewBareTxPush()
				tx.PushNote.RepoName = "unknown"
				mockMgr.EXPECT().GetRepo(tx.PushNote.RepoName).Return(nil, fmt.Errorf("error"))
				syncher = newSyncher(mockBlockGetter, mockMgr, mockMgr, mockLogic, mockDHT, cfg.G().Log)
				err = syncher.syncTx(tx)
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

				mockMgr.EXPECT().MergeTxPushToRepo(tx).Return(nil)

				mockMgr.EXPECT().GetRepo(tx.PushNote.RepoName).Return(repo, nil)
				syncher = newSyncher(mockBlockGetter, mockMgr, mockMgr, mockLogic, mockDHT, cfg.G().Log)
				err = syncher.syncTx(tx)
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

				mockMgr.EXPECT().MergeTxPushToRepo(tx).Return(fmt.Errorf("error"))

				mockMgr.EXPECT().GetRepo(tx.PushNote.RepoName).Return(repo, nil)
				syncher = newSyncher(mockBlockGetter, mockMgr, mockMgr, mockLogic, mockDHT, cfg.G().Log)
				err = syncher.syncTx(tx)
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
				syncher = newSyncher(mockBlockGetter, mockMgr, mockMgr, mockLogic, mockDHT, cfg.G().Log)
				err = syncher.syncTx(tx)
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
				syncher = newSyncher(mockBlockGetter, mockMgr, mockMgr, mockLogic, mockDHT, cfg.G().Log)
				err = syncher.syncTx(tx)
			})

			It("should return error='failed to write fetched object...'", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("failed to write fetched object"))
			})
		})

		When("downloaded succeeded but object announcement fails", func() {
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

				mockMgr.EXPECT().MergeTxPushToRepo(tx).Return(nil)

				mockMgr.EXPECT().GetRepo(tx.PushNote.RepoName).Return(repo, nil)
				syncher = newSyncher(mockBlockGetter, mockMgr, mockMgr, mockLogic, mockDHT, cfg.G().Log)
				err = syncher.syncTx(tx)
			})

			It("should return no error", func() {
				Expect(err).To(BeNil())
			})
		})
	})
})
