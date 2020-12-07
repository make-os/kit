package refsync

import (
	"fmt"

	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/mocks"
	types2 "github.com/make-os/kit/remote/push/types"
	reftypes "github.com/make-os/kit/remote/refsync/types"
	"github.com/make-os/kit/testutil"
	"github.com/make-os/kit/types"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/types/txns"
	. "github.com/onsi/ginkgo"
	core_types "github.com/tendermint/tendermint/rpc/core/types"
	tmtypes "github.com/tendermint/tendermint/types"
	"gopkg.in/src-d/go-git.v4"

	. "github.com/onsi/gomega"
)

func handleTx(*txns.TxPush, string, int, int64, func()) {}

var _ = Describe("Watcher", func() {
	var err error
	var cfg *config.AppConfig
	var w *Watcher
	var ctrl *gomock.Controller
	var mockKeepers *mocks.MockKeepers
	var mockRepoSyncInfoKeeper *mocks.MockRepoSyncInfoKeeper
	var mockRepoKeeper *mocks.MockRepoKeeper
	var mockService *mocks.MockService

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
		ctrl = gomock.NewController(GinkgoT())
		mockKeepers = mocks.NewMockKeepers(ctrl)
		mockRepoSyncInfoKeeper = mocks.NewMockRepoSyncInfoKeeper(ctrl)
		mockRepoKeeper = mocks.NewMockRepoKeeper(ctrl)
		mockKeepers.EXPECT().RepoSyncInfoKeeper().Return(mockRepoSyncInfoKeeper).AnyTimes()
		mockKeepers.EXPECT().RepoKeeper().Return(mockRepoKeeper)
		mockService = mocks.NewMockService(ctrl)
		w = NewWatcher(cfg, handleTx, mockKeepers)
		w.service = mockService
	})

	Describe(".Start", func() {
		It("should panic if called twice", func() {
			w.Start()
			Expect(func() { w.Start() }).To(Panic())
		})
	})

	Describe(".IsRunning", func() {
		It("should return false if not running", func() {
			Expect(w.IsRunning()).To(BeFalse())
		})

		It("should return true if running", func() {
			w.Start()
			Expect(w.IsRunning()).To(BeTrue())
		})
	})

	Describe(".Stop", func() {
		It("should set running state to false", func() {
			w.Start()
			Expect(w.IsRunning()).To(BeTrue())
			w.Stop()
			Expect(w.IsRunning()).To(BeFalse())
		})
	})

	Describe(".HasTask", func() {
		It("should return false when task queue is empty", func() {
			Expect(w.QueueSize()).To(BeZero())
			Expect(w.HasTask()).To(BeFalse())
		})
	})

	Describe(".Watch", func() {
		It("should return ErrSkipped if reference has been queued", func() {
			err := w.Watch("repo1", "refs/heads/master", 0, 0)
			Expect(err).To(BeNil())
			err = w.Watch("repo1", "", 0, 0)
			Expect(err).To(BeNil())
			err = w.Watch("repo1", "refs/heads/master", 0, 0)
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(types.ErrSkipped))
		})
	})

	Describe(".addTask", func() {
		It("should not add any task if no tracked repository exist", func() {
			mockRepoSyncInfoKeeper.EXPECT().Tracked().Return(map[string]*core.TrackedRepo{})
			w.addTrackedRepos()
			Expect(w.QueueSize()).To(BeZero())
		})

		It("should add tracked repo if its last updated height is less than the repo's last update height", func() {
			mockRepoSyncInfoKeeper.EXPECT().Tracked().Return(map[string]*core.TrackedRepo{
				"repo1": {UpdatedAt: 1000},
			})
			mockRepoKeeper.EXPECT().Get("repo1").Return(&state.Repository{UpdatedAt: 1001})
			w.addTrackedRepos()
			Expect(w.QueueSize()).To(Equal(1))
		})

		It("should set start height to repo's start height if track repo last updated height is 0", func() {
			mockRepoSyncInfoKeeper.EXPECT().Tracked().Return(map[string]*core.TrackedRepo{
				"repo1": {UpdatedAt: 0},
			})
			mockRepoKeeper.EXPECT().Get("repo1").Return(&state.Repository{CreatedAt: 10, UpdatedAt: 1001})
			w.addTrackedRepos()
			Expect(w.QueueSize()).To(Equal(1))
			task := <-w.queue
			Expect(task.StartHeight).To(Equal(uint64(10)))
		})

		It("should not add tracked repo if its last updated height is equal to the repo's last update height", func() {
			mockRepoSyncInfoKeeper.EXPECT().Tracked().Return(map[string]*core.TrackedRepo{
				"repo1": {UpdatedAt: 1000},
			})
			mockRepoKeeper.EXPECT().Get("repo1").Return(&state.Repository{UpdatedAt: 1000})
			w.addTrackedRepos()
			Expect(w.QueueSize()).To(BeZero())
		})
	})

	Describe(".Do", func() {
		It("should return error when unable to get block at a specific height", func() {
			task := &reftypes.WatcherTask{RepoName: "repo1", StartHeight: 1, EndHeight: 2}
			mockRepoSyncInfoKeeper.EXPECT().GetTracked(task.RepoName).Return(&core.TrackedRepo{})
			mockService.EXPECT().GetBlock(int64(task.StartHeight)).Return(nil, fmt.Errorf("error"))
			err := w.Do(task)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to get block (height=1): error"))
		})

		When("no transactions exist in blocks between StartHeight -> EndHeight", func() {
			It("should just fetch the block at the heights and update the tracked repo LastUpdated height", func() {
				task := &reftypes.WatcherTask{RepoName: "repo1", StartHeight: 1, EndHeight: 3}
				mockService.EXPECT().GetBlock(int64(task.StartHeight)).Return(&core_types.ResultBlock{Block: &tmtypes.Block{Data: tmtypes.Data{}}}, nil)
				mockService.EXPECT().GetBlock(int64(task.StartHeight+1)).Return(&core_types.ResultBlock{Block: &tmtypes.Block{Data: tmtypes.Data{}}}, nil)
				mockService.EXPECT().GetBlock(int64(task.StartHeight+2)).Return(&core_types.ResultBlock{Block: &tmtypes.Block{Data: tmtypes.Data{}}}, nil)
				mockRepoSyncInfoKeeper.EXPECT().Track(task.RepoName, task.StartHeight)
				mockRepoSyncInfoKeeper.EXPECT().Track(task.RepoName, task.StartHeight+1)
				mockRepoSyncInfoKeeper.EXPECT().Track(task.RepoName, task.StartHeight+2)
				mockRepoSyncInfoKeeper.EXPECT().GetTracked(task.RepoName).Return(&core.TrackedRepo{})
				err := w.Do(task)
				Expect(err).To(BeNil())
			})
		})

		When("transaction is bad", func() {
			It("should return error when unable to decode transaction into a BaseTx", func() {
				task := &reftypes.WatcherTask{RepoName: "repo1", StartHeight: 1, EndHeight: 2}
				mockService.EXPECT().GetBlock(int64(task.StartHeight)).Return(&core_types.ResultBlock{Block: &tmtypes.Block{Data: tmtypes.Data{
					Txs: []tmtypes.Tx{[]byte("bad tx")},
				}}}, nil)
				mockService.EXPECT().GetBlock(int64(task.StartHeight+1)).Return(&core_types.ResultBlock{Block: &tmtypes.Block{Data: tmtypes.Data{}}}, nil)
				mockRepoSyncInfoKeeper.EXPECT().GetTracked(task.RepoName).Return(&core.TrackedRepo{})
				mockRepoSyncInfoKeeper.EXPECT().Track(task.RepoName, task.StartHeight)
				mockRepoSyncInfoKeeper.EXPECT().Track(task.RepoName, task.StartHeight+1)
				err := w.Do(task)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("unable to decode transaction #0 in height 1"))
			})
		})

		It("should ignore transaction if it is not a TxPush type", func() {
			task := &reftypes.WatcherTask{RepoName: "repo1", StartHeight: 1, EndHeight: 2}
			tx := txns.NewBareTxCoinTransfer()
			tx.Value = "10"
			mockService.EXPECT().GetBlock(int64(task.StartHeight)).Return(&core_types.ResultBlock{Block: &tmtypes.Block{Data: tmtypes.Data{
				Txs: []tmtypes.Tx{tx.Bytes()},
			}}}, nil)
			mockService.EXPECT().GetBlock(int64(task.StartHeight+1)).Return(&core_types.ResultBlock{Block: &tmtypes.Block{Data: tmtypes.Data{}}}, nil)
			mockRepoSyncInfoKeeper.EXPECT().GetTracked(task.RepoName).Return(&core.TrackedRepo{})
			mockRepoSyncInfoKeeper.EXPECT().Track(task.RepoName, task.StartHeight)
			mockRepoSyncInfoKeeper.EXPECT().Track(task.RepoName, task.StartHeight+1)
			err := w.Do(task)
			Expect(err).To(BeNil())
		})

		It("should ignore transaction if it is a TxPush type but not targeting the tracked repo", func() {
			task := &reftypes.WatcherTask{RepoName: "repo1", StartHeight: 1, EndHeight: 2}
			tx := txns.NewBareTxPush()
			tx.Note = &types2.Note{RepoName: "repo2"}
			mockService.EXPECT().GetBlock(int64(task.StartHeight)).Return(&core_types.ResultBlock{Block: &tmtypes.Block{Data: tmtypes.Data{
				Txs: []tmtypes.Tx{tx.Bytes()},
			}}}, nil)
			mockService.EXPECT().GetBlock(int64(task.StartHeight+1)).Return(&core_types.ResultBlock{Block: &tmtypes.Block{Data: tmtypes.Data{}}}, nil)
			mockRepoSyncInfoKeeper.EXPECT().GetTracked(task.RepoName).Return(&core.TrackedRepo{})
			mockRepoSyncInfoKeeper.EXPECT().Track(task.RepoName, task.StartHeight)
			mockRepoSyncInfoKeeper.EXPECT().Track(task.RepoName, task.StartHeight+1)
			err := w.Do(task)
			Expect(err).To(BeNil())
		})

		It("should not update repo's track height if repo is not tracked", func() {
			task := &reftypes.WatcherTask{RepoName: "repo1", StartHeight: 1, EndHeight: 2}
			tx := txns.NewBareTxPush()
			tx.Note = &types2.Note{RepoName: "repo2"}
			mockService.EXPECT().GetBlock(int64(task.StartHeight)).Return(&core_types.ResultBlock{Block: &tmtypes.Block{Data: tmtypes.Data{
				Txs: []tmtypes.Tx{tx.Bytes()},
			}}}, nil)
			mockService.EXPECT().GetBlock(int64(task.StartHeight+1)).Return(&core_types.ResultBlock{Block: &tmtypes.Block{Data: tmtypes.Data{}}}, nil)
			mockRepoSyncInfoKeeper.EXPECT().GetTracked(task.RepoName).Return(nil)
			err := w.Do(task)
			Expect(err).To(BeNil())
		})

		It("should remove task from queued list on completion", func() {
			task := &reftypes.WatcherTask{RepoName: "repo1", StartHeight: 1, EndHeight: 2}
			tx := txns.NewBareTxPush()
			tx.Note = &types2.Note{RepoName: "repo2"}
			mockService.EXPECT().GetBlock(int64(task.StartHeight)).Return(&core_types.ResultBlock{Block: &tmtypes.Block{Data: tmtypes.Data{
				Txs: []tmtypes.Tx{tx.Bytes()},
			}}}, nil)
			mockService.EXPECT().GetBlock(int64(task.StartHeight+1)).Return(&core_types.ResultBlock{Block: &tmtypes.Block{Data: tmtypes.Data{}}}, nil)
			mockRepoSyncInfoKeeper.EXPECT().GetTracked(task.RepoName).Return(nil)
			w.Watch(task.RepoName, task.Reference, 0, 0)
			err := w.Do(task)
			Expect(err).To(BeNil())
			Expect(w.processing.Len()).To(BeZero())
		})

		When("a TxPush transaction type is found and it is addressed to the tracked repo", func() {
			It("should attempt to initialize a new git repo if tracked repo LastUpdated=1 (meaning, first time tracking)", func() {
				task := &reftypes.WatcherTask{RepoName: "repo1", StartHeight: 1, EndHeight: 2}
				tx := txns.NewBareTxPush()
				tx.Note = &types2.Note{RepoName: "repo1"}
				var didInitRepo = false
				var didHandleTx = false

				w = NewWatcher(cfg, func(push *txns.TxPush, ref string, index int, i int64, done func()) {
					didHandleTx = true
				}, mockKeepers)
				w.service = mockService
				w.initRepo = func(name string, rootDir string, gitBinPath string) error {
					didInitRepo = true
					return nil
				}
				mockService.EXPECT().GetBlock(int64(task.StartHeight)).Return(&core_types.ResultBlock{Block: &tmtypes.Block{Data: tmtypes.Data{Txs: []tmtypes.Tx{tx.Bytes()}}}}, nil)
				mockService.EXPECT().GetBlock(int64(task.StartHeight+1)).Return(&core_types.ResultBlock{Block: &tmtypes.Block{Data: tmtypes.Data{}}}, nil)
				mockRepoSyncInfoKeeper.EXPECT().GetTracked(task.RepoName).Return(&core.TrackedRepo{})
				mockRepoSyncInfoKeeper.EXPECT().Track(task.RepoName, task.StartHeight)
				mockRepoSyncInfoKeeper.EXPECT().Track(task.RepoName, task.StartHeight+1)
				err := w.Do(task)
				Expect(err).To(BeNil())
				Expect(didInitRepo).To(BeTrue())
				Expect(didHandleTx).To(BeTrue())
			})

			It("should attempt to initialize a new git repo if tracked repo LastUpdated=1 and return no error if tracked repo is already initialized", func() {
				task := &reftypes.WatcherTask{RepoName: "repo1", StartHeight: 1, EndHeight: 2}
				tx := txns.NewBareTxPush()
				tx.Note = &types2.Note{RepoName: "repo1"}
				var didInitRepo = false
				w.initRepo = func(name string, rootDir string, gitBinPath string) error {
					didInitRepo = true
					return git.ErrRepositoryAlreadyExists
				}
				mockService.EXPECT().GetBlock(int64(task.StartHeight)).Return(&core_types.ResultBlock{Block: &tmtypes.Block{Data: tmtypes.Data{Txs: []tmtypes.Tx{tx.Bytes()}}}}, nil)
				mockService.EXPECT().GetBlock(int64(task.StartHeight+1)).Return(&core_types.ResultBlock{Block: &tmtypes.Block{Data: tmtypes.Data{}}}, nil)
				mockRepoSyncInfoKeeper.EXPECT().GetTracked(task.RepoName).Return(&core.TrackedRepo{})
				mockRepoSyncInfoKeeper.EXPECT().Track(task.RepoName, task.StartHeight)
				mockRepoSyncInfoKeeper.EXPECT().Track(task.RepoName, task.StartHeight+1)
				err := w.Do(task)
				Expect(err).To(BeNil())
				Expect(didInitRepo).To(BeTrue())
			})

			It("should attempt to initialize a new git repo if tracked repo LastUpdated=1 and return error if tracked repo could not be initialized", func() {
				task := &reftypes.WatcherTask{RepoName: "repo1", StartHeight: 1, EndHeight: 2}
				tx := txns.NewBareTxPush()
				tx.Note = &types2.Note{RepoName: "repo1"}
				var didInitRepo = false
				w.initRepo = func(name string, rootDir string, gitBinPath string) error {
					didInitRepo = true
					return fmt.Errorf("error")
				}
				mockService.EXPECT().GetBlock(int64(task.StartHeight)).Return(&core_types.ResultBlock{Block: &tmtypes.Block{Data: tmtypes.Data{Txs: []tmtypes.Tx{tx.Bytes()}}}}, nil)
				mockService.EXPECT().GetBlock(int64(task.StartHeight+1)).Return(&core_types.ResultBlock{Block: &tmtypes.Block{Data: tmtypes.Data{}}}, nil)
				mockRepoSyncInfoKeeper.EXPECT().GetTracked(task.RepoName).Return(&core.TrackedRepo{})
				mockRepoSyncInfoKeeper.EXPECT().Track(task.RepoName, task.StartHeight)
				mockRepoSyncInfoKeeper.EXPECT().Track(task.RepoName, task.StartHeight+1)
				err := w.Do(task)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("failed to initialize repository: error"))
				Expect(didInitRepo).To(BeTrue())
			})
		})
	})
})
