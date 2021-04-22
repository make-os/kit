package miner

import (
	"fmt"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/mocks"
	"github.com/make-os/kit/params"
	"github.com/make-os/kit/pkgs/logger"
	"github.com/make-os/kit/testutil"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/util"
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
	var mockKeepers *mocks.MockKeepers
	var mockSysKeeper *mocks.MockSystemKeeper
	var m Miner

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		ctrl = gomock.NewController(GinkgoT())
		mockService = mocks.NewMockService(ctrl)
		mockKeepers = mocks.NewMockKeepers(ctrl)
		mockSysKeeper = mocks.NewMockSystemKeeper(ctrl)
		mockKeepers.EXPECT().SysKeeper().Return(mockSysKeeper).AnyTimes()
		m = NewMiner(cfg, mockKeepers, mockService)
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
			m.(*CPUMiner).mine = func(id int, minerAddr []byte, keepers core.Keepers, log logger.Logger, stopCh chan bool, onAttempt func(nAttempts int64)) (epoch int64, nonce uint64, err error) {
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
			m.(*CPUMiner).mine = func(id int, minerAddr []byte, keepers core.Keepers, log logger.Logger, stopCh chan bool, onAttempt func(nAttempts int64)) (epoch int64, nonce uint64, err error) {
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
			m.(*CPUMiner).mine = func(id int, minerAddr []byte, keepers core.Keepers, log logger.Logger, stopCh chan bool, onAttempt func(nAttempts int64)) (epoch int64, nonce uint64, err error) {
				for !util.IsBoolChanClosed(stopCh) {
				}
				return
			}

			cfg.Miner.Threads = 2
			err := m.Start(false)
			Expect(err).To(BeNil())
			m.Stop()
			Expect(m.IsMining()).To(BeFalse())
			Expect(util.IsBoolChanClosed(m.(*CPUMiner).stopThreads)).To(BeTrue())

			m.Stop()
			Expect(m.IsMining()).To(BeFalse())
			Expect(util.IsBoolChanClosed(m.(*CPUMiner).stopThreads)).To(BeTrue())
		})
	})

	Describe(".mine", func() {
		It("should return error if unable to get last block info", func() {
			stopCh := make(chan bool)
			mockSysKeeper.EXPECT().GetLastBlockInfo().Return(nil, fmt.Errorf("error"))
			_, _, err := mine(1, util.RandBytes(20), mockKeepers, cfg.G().Log, stopCh, func(int64) {})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to get last block info: error"))
		})

		It("should return error if unable to get block info of epoch start block", func() {
			stopCh := make(chan bool)
			params.NumBlocksPerEpoch = 2
			curBlock := &state.BlockInfo{Height: 20}
			mockSysKeeper.EXPECT().GetLastBlockInfo().Return(curBlock, nil)
			epoch := params.GetEpochOfHeight(curBlock.Height.Int64())
			mockSysKeeper.EXPECT().GetBlockInfo(params.GetFirstInEpoch(epoch)).Return(nil, fmt.Errorf("error"))
			_, _, err := mine(1, util.RandBytes(20), mockKeepers, cfg.G().Log, stopCh, func(int64) {})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to get current epoch start block: error"))
		})

		It("should return return epoch and nonce on success", func() {
			stopCh := make(chan bool)
			params.NumBlocksPerEpoch = 2
			curBlock := &state.BlockInfo{Height: 20}
			mockSysKeeper.EXPECT().GetLastBlockInfo().Return(curBlock, nil)
			mockSysKeeper.EXPECT().GetCurrentDifficulty().Return(new(big.Int).SetInt64(1000))

			epoch := params.GetEpochOfHeight(curBlock.Height.Int64())
			epochStartBlock := &state.BlockInfo{Height: 20, Hash: util.RandBytes(32)}
			mockSysKeeper.EXPECT().GetBlockInfo(params.GetFirstInEpoch(epoch)).Return(epochStartBlock, nil)

			retEpoch, nonce, err := mine(1, util.RandBytes(20), mockKeepers, cfg.G().Log, stopCh, func(int64) {})
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

			epoch := params.GetEpochOfHeight(curBlock.Height.Int64())
			epochStartBlock := &state.BlockInfo{Height: 20, Hash: util.RandBytes(32)}
			mockSysKeeper.EXPECT().GetBlockInfo(params.GetFirstInEpoch(epoch)).Return(epochStartBlock, nil)

			go func() {
				time.Sleep(10 * time.Millisecond)
				close(stopCh)
			}()

			retEpoch, nonce, err := mine(1, util.RandBytes(20), mockKeepers, cfg.G().Log, stopCh, func(int64) {})
			Expect(err).To(BeNil())
			Expect(retEpoch).To(BeZero())
			Expect(nonce).To(BeZero())
		})
	})
})
