package refsync

import (
	"encoding/base64"
	"fmt"

	"github.com/golang/mock/gomock"
	"github.com/make-os/lobe/config"
	"github.com/make-os/lobe/mocks"
	types2 "github.com/make-os/lobe/remote/push/types"
	types3 "github.com/make-os/lobe/remote/refsync/types"
	"github.com/make-os/lobe/testutil"
	"github.com/make-os/lobe/types"
	"github.com/make-os/lobe/types/core"
	"github.com/make-os/lobe/types/state"
	"github.com/make-os/lobe/types/txns"
	. "github.com/onsi/ginkgo"
	"gopkg.in/src-d/go-git.v4"

	. "github.com/onsi/gomega"
)

func handleTx(*txns.TxPush, int, int64) {}

var _ = Describe("Watcher", func() {
	var err error
	var cfg *config.AppConfig
	var w *Watcher
	var ctrl *gomock.Controller
	var mockKeepers *mocks.MockKeepers
	var MockRepoSyncInfoKeeper *mocks.MockRepoSyncInfoKeeper
	var mockRepoKeeper *mocks.MockRepoKeeper
	var mockService *mocks.MockService

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
		ctrl = gomock.NewController(GinkgoT())
		mockKeepers = mocks.NewMockKeepers(ctrl)
		MockRepoSyncInfoKeeper = mocks.NewMockRepoSyncInfoKeeper(ctrl)
		mockRepoKeeper = mocks.NewMockRepoKeeper(ctrl)
		mockKeepers.EXPECT().RepoSyncInfoKeeper().Return(MockRepoSyncInfoKeeper).AnyTimes()
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

	Describe(".addTask", func() {
		It("should not add any task if no tracked repository exist", func() {
			MockRepoSyncInfoKeeper.EXPECT().Tracked().Return(map[string]*core.TrackedRepo{})
			w.addTasks()
			Expect(w.QueueSize()).To(BeZero())
		})

		It("should add tracked repo if its last updated height is less than the repo's last update height", func() {
			MockRepoSyncInfoKeeper.EXPECT().Tracked().Return(map[string]*core.TrackedRepo{
				"repo1": {LastUpdated: 1000},
			})
			mockRepoKeeper.EXPECT().Get("repo1").Return(&state.Repository{LastUpdated: 1001})
			w.addTasks()
			Expect(w.QueueSize()).To(Equal(1))
		})

		It("should not add tracked repo if its last updated height is equal to the repo's last update height", func() {
			MockRepoSyncInfoKeeper.EXPECT().Tracked().Return(map[string]*core.TrackedRepo{
				"repo1": {LastUpdated: 1000},
			})
			mockRepoKeeper.EXPECT().Get("repo1").Return(&state.Repository{LastUpdated: 1000})
			w.addTasks()
			Expect(w.QueueSize()).To(BeZero())
		})
	})

	Describe(".Do", func() {
		It("should return ErrSkipped when a task with matching ID is already being processed", func() {
			task := &types3.WatcherTask{RepoName: "repo1"}
			w.processing.Add(task.GetID(), struct{}{})
			err := w.Do(task)
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(types.ErrSkipped))
		})

		It("should return error when unable to get block at a specific height", func() {
			task := &types3.WatcherTask{RepoName: "repo1", StartHeight: 1, EndHeight: 2}
			mockService.EXPECT().GetBlock(int64(task.StartHeight)).Return(nil, fmt.Errorf("error"))
			err := w.Do(task)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to get block (height=1): error"))
		})

		When("no transactions exist in blocks between StartHeight -> EndHeight", func() {
			It("should just fetch the block at the heights and update the tracked repo LastUpdated height", func() {
				task := &types3.WatcherTask{RepoName: "repo1", StartHeight: 1, EndHeight: 3}
				mockService.EXPECT().GetBlock(int64(task.StartHeight)).Return(map[string]interface{}{}, nil)
				mockService.EXPECT().GetBlock(int64(task.StartHeight+1)).Return(map[string]interface{}{}, nil)
				mockService.EXPECT().GetBlock(int64(task.StartHeight+2)).Return(map[string]interface{}{}, nil)
				MockRepoSyncInfoKeeper.EXPECT().Track(task.RepoName, task.StartHeight)
				MockRepoSyncInfoKeeper.EXPECT().Track(task.RepoName, task.StartHeight+1)
				MockRepoSyncInfoKeeper.EXPECT().Track(task.RepoName, task.StartHeight+2)
				err := w.Do(task)
				Expect(err).To(BeNil())
			})
		})

		When("transaction is bad", func() {
			It("should return error when unable to decode transaction from base64", func() {
				task := &types3.WatcherTask{RepoName: "repo1", StartHeight: 1, EndHeight: 2}
				mockService.EXPECT().GetBlock(int64(task.StartHeight)).Return(map[string]interface{}{
					"result": map[string]interface{}{
						"block": map[string]interface{}{
							"data": map[string]interface{}{
								"txs": []interface{}{
									"bad_base64",
								},
							},
						},
					},
				}, nil)
				mockService.EXPECT().GetBlock(int64(task.StartHeight+1)).Return(map[string]interface{}{}, nil)
				MockRepoSyncInfoKeeper.EXPECT().Track(task.RepoName, task.StartHeight)
				MockRepoSyncInfoKeeper.EXPECT().Track(task.RepoName, task.StartHeight+1)
				err := w.Do(task)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("failed to decode transaction: illegal base64 data at input byte 3"))
			})

			It("should return error when unable to decode transaction into a BaseTx", func() {
				task := &types3.WatcherTask{RepoName: "repo1", StartHeight: 1, EndHeight: 2}
				mockService.EXPECT().GetBlock(int64(task.StartHeight)).Return(map[string]interface{}{
					"result": map[string]interface{}{
						"block": map[string]interface{}{
							"data": map[string]interface{}{
								"txs": []interface{}{
									base64.StdEncoding.EncodeToString([]byte("bad tx")),
								},
							},
						},
					},
				}, nil)
				mockService.EXPECT().GetBlock(int64(task.StartHeight+1)).Return(map[string]interface{}{}, nil)
				MockRepoSyncInfoKeeper.EXPECT().Track(task.RepoName, task.StartHeight)
				MockRepoSyncInfoKeeper.EXPECT().Track(task.RepoName, task.StartHeight+1)
				err := w.Do(task)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("unable to decode transaction #0 in height 1"))
			})
		})

		It("should ignore transaction if it is not a TxPush type", func() {
			task := &types3.WatcherTask{RepoName: "repo1", StartHeight: 1, EndHeight: 2}
			tx := txns.NewBareTxCoinTransfer()
			tx.Value = "10"
			mockService.EXPECT().GetBlock(int64(task.StartHeight)).Return(map[string]interface{}{
				"result": map[string]interface{}{
					"block": map[string]interface{}{
						"data": map[string]interface{}{
							"txs": []interface{}{
								base64.StdEncoding.EncodeToString(tx.Bytes()),
							},
						},
					},
				},
			}, nil)
			mockService.EXPECT().GetBlock(int64(task.StartHeight+1)).Return(map[string]interface{}{}, nil)
			MockRepoSyncInfoKeeper.EXPECT().Track(task.RepoName, task.StartHeight)
			MockRepoSyncInfoKeeper.EXPECT().Track(task.RepoName, task.StartHeight+1)
			err := w.Do(task)
			Expect(err).To(BeNil())
		})

		It("should ignore transaction if it is a TxPush type but not targeting the tracked repo", func() {
			task := &types3.WatcherTask{RepoName: "repo1", StartHeight: 1, EndHeight: 2}
			tx := txns.NewBareTxPush()
			tx.Note = &types2.Note{RepoName: "repo2"}
			mockService.EXPECT().GetBlock(int64(task.StartHeight)).Return(map[string]interface{}{
				"result": map[string]interface{}{
					"block": map[string]interface{}{
						"data": map[string]interface{}{
							"txs": []interface{}{
								base64.StdEncoding.EncodeToString(tx.Bytes()),
							},
						},
					},
				},
			}, nil)
			mockService.EXPECT().GetBlock(int64(task.StartHeight+1)).Return(map[string]interface{}{}, nil)
			MockRepoSyncInfoKeeper.EXPECT().Track(task.RepoName, task.StartHeight)
			MockRepoSyncInfoKeeper.EXPECT().Track(task.RepoName, task.StartHeight+1)
			err := w.Do(task)
			Expect(err).To(BeNil())
		})

		When("a TxPush transaction type is found and it is addressed to the tracked repo", func() {
			It("should attempt to initialize a new git repo if tracked repo LastUpdated=1 (meaning, first time tracking)", func() {
				task := &types3.WatcherTask{RepoName: "repo1", StartHeight: 1, EndHeight: 2}
				tx := txns.NewBareTxPush()
				tx.Note = &types2.Note{RepoName: "repo1"}
				var didInitRepo = false
				var didHandleTx = false

				w = NewWatcher(cfg, func(push *txns.TxPush, index int, i int64) {
					didHandleTx = true
				}, mockKeepers)
				w.service = mockService
				w.initRepo = func(name string, rootDir string, gitBinPath string) error {
					didInitRepo = true
					return nil
				}
				mockService.EXPECT().GetBlock(int64(task.StartHeight)).Return(map[string]interface{}{
					"result": map[string]interface{}{
						"block": map[string]interface{}{
							"data": map[string]interface{}{
								"txs": []interface{}{
									base64.StdEncoding.EncodeToString(tx.Bytes()),
								},
							},
						},
					},
				}, nil)
				mockService.EXPECT().GetBlock(int64(task.StartHeight+1)).Return(map[string]interface{}{}, nil)
				MockRepoSyncInfoKeeper.EXPECT().Track(task.RepoName, task.StartHeight)
				MockRepoSyncInfoKeeper.EXPECT().Track(task.RepoName, task.StartHeight+1)
				err := w.Do(task)
				Expect(err).To(BeNil())
				Expect(didInitRepo).To(BeTrue())
				Expect(didHandleTx).To(BeTrue())
			})

			It("should attempt to initialize a new git repo if tracked repo LastUpdated=1 and return no error if tracked repo is already initialized", func() {
				task := &types3.WatcherTask{RepoName: "repo1", StartHeight: 1, EndHeight: 2}
				tx := txns.NewBareTxPush()
				tx.Note = &types2.Note{RepoName: "repo1"}
				var didInitRepo = false
				w.initRepo = func(name string, rootDir string, gitBinPath string) error {
					didInitRepo = true
					return git.ErrRepositoryAlreadyExists
				}
				mockService.EXPECT().GetBlock(int64(task.StartHeight)).Return(map[string]interface{}{
					"result": map[string]interface{}{
						"block": map[string]interface{}{
							"data": map[string]interface{}{
								"txs": []interface{}{
									base64.StdEncoding.EncodeToString(tx.Bytes()),
								},
							},
						},
					},
				}, nil)
				mockService.EXPECT().GetBlock(int64(task.StartHeight+1)).Return(map[string]interface{}{}, nil)
				MockRepoSyncInfoKeeper.EXPECT().Track(task.RepoName, task.StartHeight)
				MockRepoSyncInfoKeeper.EXPECT().Track(task.RepoName, task.StartHeight+1)
				err := w.Do(task)
				Expect(err).To(BeNil())
				Expect(didInitRepo).To(BeTrue())
			})

			It("should attempt to initialize a new git repo if tracked repo LastUpdated=1 and return error if tracked repo could not be initialized", func() {
				task := &types3.WatcherTask{RepoName: "repo1", StartHeight: 1, EndHeight: 2}
				tx := txns.NewBareTxPush()
				tx.Note = &types2.Note{RepoName: "repo1"}
				var didInitRepo = false
				w.initRepo = func(name string, rootDir string, gitBinPath string) error {
					didInitRepo = true
					return fmt.Errorf("error")
				}
				mockService.EXPECT().GetBlock(int64(task.StartHeight)).Return(map[string]interface{}{
					"result": map[string]interface{}{
						"block": map[string]interface{}{
							"data": map[string]interface{}{
								"txs": []interface{}{
									base64.StdEncoding.EncodeToString(tx.Bytes()),
								},
							},
						},
					},
				}, nil)
				mockService.EXPECT().GetBlock(int64(task.StartHeight+1)).Return(map[string]interface{}{}, nil)
				MockRepoSyncInfoKeeper.EXPECT().Track(task.RepoName, task.StartHeight)
				MockRepoSyncInfoKeeper.EXPECT().Track(task.RepoName, task.StartHeight+1)
				err := w.Do(task)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("failed to initialize repository: error"))
				Expect(didInitRepo).To(BeTrue())
			})
		})
	})
})
