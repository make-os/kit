package refsync

import (
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"github.com/make-os/lobe/config"
	"github.com/make-os/lobe/node/services"
	"github.com/make-os/lobe/pkgs/cache"
	"github.com/make-os/lobe/pkgs/logger"
	rstypes "github.com/make-os/lobe/remote/refsync/types"
	"github.com/make-os/lobe/remote/repo"
	"github.com/make-os/lobe/types"
	"github.com/make-os/lobe/types/core"
	"github.com/make-os/lobe/types/txns"
	"github.com/pkg/errors"
	"github.com/stretchr/objx"
	"gopkg.in/src-d/go-git.v4"
)

// Watcher watches tracked repositories for new updates that have not been synchronized.
// It compares the last synced height of the tracked repository with the last network
// update height to tell when to start traversing the blockchain in search of updates.
type Watcher struct {
	cfg       *config.AppConfig
	log       logger.Logger
	queue     chan *rstypes.WatcherTask
	txHandler TxHandlerFunc
	keepers   core.Keepers
	service   services.Service

	lck        *sync.Mutex
	started    bool
	stopped    bool
	processing *cache.Cache
	ticker     *time.Ticker

	initRepo repo.InitRepositoryFunc
}

// NewWatcher creates an instance of Watcher
func NewWatcher(cfg *config.AppConfig, txHandler TxHandlerFunc, keepers core.Keepers) *Watcher {
	w := &Watcher{
		lck:        &sync.Mutex{},
		cfg:        cfg,
		log:        cfg.G().Log.Module("repo-watcher"),
		queue:      make(chan *rstypes.WatcherTask, 10000),
		txHandler:  txHandler,
		keepers:    keepers,
		processing: cache.NewCache(1000),
		initRepo:   repo.InitRepository,
	}

	service, err := services.NewFromConfig(w.cfg.G().TMConfig)
	if err != nil {
		panic(errors.Wrap(err, "failed to create node service instance"))
	}
	w.service = service

	go func() {
		cfg.G().Interrupt.Wait()
		w.Stop()
	}()

	return w
}

// QueueSize returns the size of the tasks queue
func (w *Watcher) QueueSize() int {
	return len(w.queue)
}

// HasTask checks whether there are one or more unprocessed tasks.
func (w *Watcher) HasTask() bool {
	return w.QueueSize() > 0
}

// addTrackedRepos adds trackable repositories that have fallen behind to the queue.
func (w *Watcher) addTrackedRepos() {
	for repoName, trackInfo := range w.keepers.RepoSyncInfoKeeper().Tracked() {
		repoState := w.keepers.RepoKeeper().Get(repoName)
		if repoState.LastUpdated <= trackInfo.LastUpdated {
			continue
		}

		startHeight := trackInfo.LastUpdated.UInt64()
		if startHeight == 0 {
			startHeight = 1
		}

		w.Watch(repoName, "", startHeight, repoState.LastUpdated.UInt64())
	}
}

// Watch adds a repository to the watch queue
func (w *Watcher) Watch(repo, reference string, startHeight, endHeight uint64) {
	w.queue <- &rstypes.WatcherTask{
		RepoName:    repo,
		StartHeight: startHeight,
		EndHeight:   endHeight,
		Reference:   reference,
	}
}

// Start starts the workers.
// Panics if already started.
func (w *Watcher) Start() {

	w.lck.Lock()
	started := w.started
	w.lck.Unlock()

	w.ticker = time.NewTicker(5 * time.Second)
	go func() {
		for range w.ticker.C {
			if !w.stopped {
				w.addTrackedRepos()
			}
		}
	}()

	if started {
		panic("already started")
	}

	go func() {
		for task := range w.queue {
			task := task
			go func() {
				if err := w.Do(task); err != nil && err != types.ErrSkipped {
					w.log.Error(err.Error(), "Repo", task.RepoName)
				}
			}()
		}
	}()

	w.lck.Lock()
	w.started = true
	w.lck.Unlock()
}

// IsRunning checks if the watcher is running.
func (w *Watcher) IsRunning() bool {
	return w.started
}

// Stop stops the watcher
func (w *Watcher) Stop() {
	w.lck.Lock()
	w.ticker.Stop()
	close(w.queue)
	w.stopped = true
	w.started = false
	w.lck.Unlock()
}

// Do finds push transactions that have not been applied to a repository.
func (w *Watcher) Do(task *rstypes.WatcherTask) error {

	// Skip task if the task is currently being worked on.
	if w.processing.Has(task.GetID()) {
		return types.ErrSkipped
	}
	w.processing.Add(task.RepoName, struct{}{})
	defer w.processing.Remove(task.RepoName)

	w.log.Debug("Scanning chain for new updates", "Repo", task.RepoName, "EndHeight", task.EndHeight)

	isRepoTracked := w.keepers.RepoSyncInfoKeeper().GetTracked(task.RepoName) != nil

	// Walk up the blocks until the task's end height.
	// Find push transactions addressed to the target repository.
	start := task.StartHeight
	for start <= task.EndHeight {

		res, err := w.service.GetBlock(int64(start))
		if err != nil {
			return errors.Wrapf(err, "failed to get block (height=%d)", start)
		}

		// TODO: new tendermint update may have a different block structure.
		block := objx.New(res)
		foundTx := false
		for i, tx := range block.Get("result.block.data.txs").InterSlice() {
			bz, err := base64.StdEncoding.DecodeString(tx.(string))
			if err != nil {
				return fmt.Errorf("failed to decode transaction: %s", err)
			}

			txObj, err := txns.DecodeTx(bz)
			if err != nil {
				return fmt.Errorf("unable to decode transaction #%d in height %d", i, start)
			}

			// Ignore push transaction not addressed to the task's repo
			obj, ok := txObj.(*txns.TxPush)
			if !ok || obj.Note.GetRepoName() != task.RepoName {
				continue
			}

			// Create the git repository if the tracked repo had not
			// been previously synchronized before.
			if task.StartHeight == 1 {
				err := w.initRepo(task.RepoName, w.cfg.GetRepoRoot(), w.cfg.Node.GitBinPath)
				if err != nil && errors.Cause(err) != git.ErrRepositoryAlreadyExists {
					return errors.Wrap(err, "failed to initialize repository")
				}
			}

			w.log.Debug("Found update for repo", "Repo", task.RepoName, "Height", start)
			w.txHandler(obj, task.Reference, i, int64(start))
			foundTx = true
		}

		// If no push transaction was found in this block and the repo is being tracked,
		// update the last synced block height of the tracked repo.
		if !foundTx && isRepoTracked {
			w.keepers.RepoSyncInfoKeeper().Track(task.RepoName, start)
		}

		start++
	}

	return nil
}
