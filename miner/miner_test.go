package miner

import (
	"fmt"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/crypto/ed25519"
	"github.com/make-os/kit/mocks"
	"github.com/make-os/kit/params"
	"github.com/make-os/kit/pkgs/logger"
	"github.com/make-os/kit/testutil"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util"
	epoch2 "github.com/make-os/kit/util/epoch"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestMiner(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Miner Suite")
}

var _ = Describe("Miner", func() {
	var err error
	var cfg *config.AppConfig
	var ctrl *gomock.Controller
	var mockService *mocks.MockService
	var mockLogic *mocks.MockLogic
	var mockSysKeeper *mocks.MockSystemKeeper
	var mockAcctKeeper *mocks.MockAccountKeeper
	var mockMemReactor *mocks.MockMempoolReactor
	var sender = ed25519.NewKeyFromIntSeed(1)
	var m Miner

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		ctrl = gomock.NewController(GinkgoT())
		mockService = mocks.NewMockService(ctrl)
		mockLogic = mocks.NewMockLogic(ctrl)
		mockSysKeeper = mocks.NewMockSystemKeeper(ctrl)
		mockMemReactor = mocks.NewMockMempoolReactor(ctrl)
		mockAcctKeeper = mocks.NewMockAccountKeeper(ctrl)
		mockLogic.EXPECT().SysKeeper().Return(mockSysKeeper).AnyTimes()
		mockLogic.EXPECT().AccountKeeper().Return(mockAcctKeeper).AnyTimes()
		mockLogic.EXPECT().GetMempoolReactor().Return(mockMemReactor).AnyTimes()
		m = NewMiner(cfg, mockLogic, mockService)
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".Start", func() {
		It("should return error if unable to check node sync status", func() {
			mockService.EXPECT().IsSyncing(gomock.Any()).Return(false, fmt.Errorf("error"))
			err := m.Start(false)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to check node's sync status: error"))
		})

		It("should return ErrNodeSyncing if node is syncing and scheduleStart is false", func() {
			mockService.EXPECT().IsSyncing(gomock.Any()).Return(true, nil)
			err := m.Start(false)
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(ErrNodeSyncing))
		})

		It("should set interval to retry start operation if node is syncing and scheduleStart is true", func() {
			Expect(m.(*CPUMiner).retryStartInt).To(BeNil())
			retryInterval = 100 * time.Millisecond

			mockService.EXPECT().IsSyncing(gomock.Any()).Return(true, nil)
			err := m.Start(true)
			Expect(err).To(BeNil())
			Expect(m.(*CPUMiner).retryStartInt).ToNot(BeNil())
			m.(*CPUMiner).retryStartInt.Stop()
		})

		It("should start 2 miners if Threads=2", func() {
			mockService.EXPECT().IsSyncing(gomock.Any()).Return(false, nil)
			ids := make(chan int, 2)
			m.(*CPUMiner).mine = func(id int, minerKey *ed25519.Key, keepers core.Logic, log logger.Logger,
				stopCh chan bool, onAttempt func(nAttempts int64)) (epoch int64, nonce uint64, err error) {
				ids <- id
				return
			}
			cfg.Miner.Threads = 2
			err := m.Start(true)
			Expect(err).To(BeNil())
			Expect(<-ids).To(Or(Equal(1), Equal(2)))
			Expect(<-ids).To(Or(Equal(1), Equal(2)))

			Expect(m.IsMining()).To(BeTrue())
		})

		It("should return error if miner is already running", func() {
			mockService.EXPECT().IsSyncing(gomock.Any()).Return(false, nil)
			ids := make(chan int, 2)
			m.(*CPUMiner).mine = func(id int, minerKey *ed25519.Key, keepers core.Logic, log logger.Logger,
				stopCh chan bool, onAttempt func(nAttempts int64)) (epoch int64, nonce uint64, err error) {
				ids <- id
				return
			}

			cfg.Miner.Threads = 2
			err := m.Start(false)
			Expect(err).To(BeNil())
			<-ids

			err = m.Start(false)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("miner is already running"))
		})
	})

	Describe(".Stop", func() {
		It("should stop the miner", func() {
			mockService.EXPECT().IsSyncing(gomock.Any()).Return(false, nil)
			m.(*CPUMiner).mine = func(id int, minerKey *ed25519.Key, keepers core.Logic, log logger.Logger,
				stopCh chan bool, onAttempt func(nAttempts int64)) (epoch int64, nonce uint64, err error) {
				for !util.IsBoolChanClosed(stopCh) {
				}
				return
			}

			cfg.Miner.Threads = 2
			err := m.Start(false)
			Expect(err).To(BeNil())
			m.Stop()
			Expect(m.IsMining()).To(BeFalse())
			Expect(util.IsBoolChanClosed(m.(*CPUMiner).stopThreads)).To(BeFalse())

			m.Stop()
			Expect(m.IsMining()).To(BeFalse())
			Expect(util.IsBoolChanClosed(m.(*CPUMiner).stopThreads)).To(BeFalse())
		})
	})

	Describe(".mine", func() {
		It("should return error if unable to get last block info", func() {
			stopCh := make(chan bool)
			mockSysKeeper.EXPECT().GetLastBlockInfo().Return(nil, fmt.Errorf("error"))
			_, _, err := mine(1, sender, mockLogic, cfg.G().Log, stopCh, func(int64) {})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to get last block info: error"))
		})

		It("should return error if unable to get block info of epoch start block", func() {
			stopCh := make(chan bool)
			params.NumBlocksPerEpoch = 2
			curBlock := &state.BlockInfo{Height: 20}
			mockSysKeeper.EXPECT().GetLastBlockInfo().Return(curBlock, nil)
			epoch := epoch2.GetEpochAt(curBlock.Height.Int64())
			mockSysKeeper.EXPECT().GetBlockInfo(epoch2.GetFirstInEpoch(epoch)).Return(nil, fmt.Errorf("error"))
			_, _, err := mine(1, sender, mockLogic, cfg.G().Log, stopCh, func(int64) {})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to get current epoch start block: error"))
		})

		It("should return return epoch and nonce on success", func() {
			stopCh := make(chan bool)
			params.NumBlocksPerEpoch = 2
			curBlock := &state.BlockInfo{Height: 20}
			mockSysKeeper.EXPECT().GetLastBlockInfo().Return(curBlock, nil)
			mockSysKeeper.EXPECT().GetCurrentDifficulty().Return(new(big.Int).SetInt64(1000))

			epoch := epoch2.GetEpochAt(curBlock.Height.Int64())
			epochStartBlock := &state.BlockInfo{Height: 20, Hash: util.RandBytes(32)}
			mockSysKeeper.EXPECT().GetBlockInfo(epoch2.GetFirstInEpoch(epoch)).Return(epochStartBlock, nil)

			retEpoch, nonce, err := mine(1, sender, mockLogic, cfg.G().Log, stopCh, func(int64) {})
			Expect(err).To(BeNil())
			Expect(epoch).To(Equal(retEpoch))
			Expect(nonce).ToNot(BeZero())
		})

		It("should return zero epoch and nonce and nil error if mine method stopped before nonce is found", func() {
			stopCh := make(chan bool)
			params.NumBlocksPerEpoch = 2
			curBlock := &state.BlockInfo{Height: 20}
			mockSysKeeper.EXPECT().GetLastBlockInfo().Return(curBlock, nil)
			mockSysKeeper.EXPECT().GetCurrentDifficulty().Return(new(big.Int).SetInt64(10000000000))

			epoch := epoch2.GetEpochAt(curBlock.Height.Int64())
			epochStartBlock := &state.BlockInfo{Height: 20, Hash: util.RandBytes(32)}
			mockSysKeeper.EXPECT().GetBlockInfo(epoch2.GetFirstInEpoch(epoch)).Return(epochStartBlock, nil)

			go func() {
				time.Sleep(10 * time.Millisecond)
				close(stopCh)
			}()

			retEpoch, nonce, err := mine(1, sender, mockLogic, cfg.G().Log, stopCh, func(int64) {})
			Expect(err).To(BeNil())
			Expect(retEpoch).To(BeZero())
			Expect(nonce).To(BeZero())
		})
	})

	Describe(".VerifyWork", func() {
		var retEpoch int64
		var nonce uint64
		var minerAddr = sender.PubKey().AddrRaw()
		var epochStartBlock = &state.BlockInfo{Height: 20, Hash: util.RandBytes(32)}

		BeforeEach(func() {
			stopCh := make(chan bool)
			params.NumBlocksPerEpoch = 2
			curBlock := &state.BlockInfo{Height: 20}
			mockSysKeeper.EXPECT().GetLastBlockInfo().Return(curBlock, nil)
			mockSysKeeper.EXPECT().GetCurrentDifficulty().Return(new(big.Int).SetInt64(1000))

			epoch := epoch2.GetEpochAt(curBlock.Height.Int64())
			mockSysKeeper.EXPECT().GetBlockInfo(epoch2.GetFirstInEpoch(epoch)).Return(epochStartBlock, nil)

			retEpoch, nonce, err = mine(1, sender, mockLogic, cfg.G().Log, stopCh, func(int64) {})
			Expect(err).To(BeNil())
			Expect(epoch).To(Equal(retEpoch))
			Expect(nonce).ToNot(BeZero())
		})

		It("should return true if nonce is valid for epoch", func() {
			mockSysKeeper.EXPECT().GetCurrentDifficulty().Return(new(big.Int).SetInt64(1000))
			good, err := VerifyWork(epochStartBlock.Hash, minerAddr, nonce, mockLogic)
			Expect(err).To(BeNil())
			Expect(good).To(BeTrue())
		})

		It("should return false if nonce is not valid for epoch", func() {
			mockSysKeeper.EXPECT().GetCurrentDifficulty().Return(new(big.Int).SetInt64(1000))
			good, err := VerifyWork(epochStartBlock.Hash, minerAddr, nonce+1, mockLogic)
			Expect(err).To(BeNil())
			Expect(good).To(BeFalse())
		})
	})

	Describe(".SubmitWork", func() {
		It("should return error when unable to add tx to mempool", func() {
			senderAcct := state.NewBareAccount()
			mockAcctKeeper.EXPECT().Get(sender.Addr()).Return(senderAcct)
			mockMemReactor.EXPECT().AddTx(gomock.AssignableToTypeOf(&txns.TxSubmitWork{})).Return(nil, fmt.Errorf("error"))
			_, err := SubmitWork(sender, 100, 2, 3.5, mockLogic, cfg.G().Log)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to add tx to mempool: error"))
		})

		It("should return hash when tx was added to the mempool", func() {
			senderAcct := state.NewBareAccount()
			mockAcctKeeper.EXPECT().Get(sender.Addr()).Return(senderAcct)
			hash := util.HexBytes(util.RandBytes(32))
			mockMemReactor.EXPECT().AddTx(gomock.AssignableToTypeOf(&txns.TxSubmitWork{})).Return(hash, nil)
			res, err := SubmitWork(sender, 100, 2, 3.5, mockLogic, cfg.G().Log)
			Expect(err).To(BeNil())
			Expect(res).To(Equal(hash.String()))
		})
	})
})
