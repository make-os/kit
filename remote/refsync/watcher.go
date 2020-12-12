package refsync

import (
	"fmt"
	"sync"
	"time"

	"github.com/make-os/kit/config"
	"github.com/make-os/kit/node/services"
	"github.com/make-os/kit/pkgs/cache"
	"github.com/make-os/kit/pkgs/logger"
	rstypes "github.com/make-os/kit/remote/refsync/types"
	"github.com/make-os/kit/remote/repo"
	"github.com/make-os/kit/types"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/types/txns"
	"github.com/pkg/errors"
	"gopkg.in/src-d/go-git.v4"
)

// Watcher watches tracked repositories for new updates that have not been synchronized.
// It compares the last synced height of the tracked repository with the last network
// update height to tell when to start traversing the blockchain in search of updates.
type Watcher struct {
	cfg            *config.AppConfig
	log            logger.Logger
	queue          chan *rstypes.WatcherTask
	refSyncHandler TxHandlerFunc
	keepers        core.Keepers
	service        services.Service
	processing     *cache.Cache

	lck     *sync.Mutex
	started bool
	stopped bool
	ticker  *time.Ticker

	initRepo repo.InitRepositoryFunc
}

// NewWatcher creates an instance of Watcher
func NewWatcher(cfg *config.AppConfig, txHandler TxHandlerFunc, keepers core.Keepers) *Watcher {
	w := &Watcher{
		lck:            &sync.Mutex{},
		cfg:            cfg,
		log:            cfg.G().Log.Module("repo-watcher"),
		queue:          make(chan *rstypes.WatcherTask, 10000),
		refSyncHandler: txHandler,
		keepers:        keepers,
		processing:     cache.NewCache(1000),
		initRepo:       repo.InitRepository,
	}

	w.service = services.New(w.cfg.G().TMConfig.RPC.ListenAddress)

	go func() {
		config.GetInterrupt().Wait()
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

// addTrackedRepos adds trackable repositories to the queue.
func (w *Watcher) addTrackedRepos() {
	for repoName, trackInfo := range w.keepers.RepoSyncInfoKeeper().Tracked() {

		// Skip repo if it is not stale
		repoState := w.keepers.RepoKeeper().Get(repoName)
		if repoState.UpdatedAt == trackInfo.UpdatedAt {
			continue
		}

		// If this is the first time tracking this repo, set
		// the start height to the creation height of the repo.
		startHeight := trackInfo.UpdatedAt.UInt64()
		if startHeight == 0 {
			startHeight = repoState.CreatedAt.UInt64()
		}

		w.Watch(repoName, "", startHeight, repoState.UpdatedAt.UInt64())
	}
}

// Watch adds a repository to the watch queue.
// Returns ErrSkipped if same reference has been queued.
func (w *Watcher) Watch(repo, reference string, startHeight, endHeight uint64) error {

	task := &rstypes.WatcherTask{
		RepoName:    repo,
		StartHeight: startHeight,
		EndHeight:   endHeight,
		Reference:   reference,
	}

	// Skip task if the task is currently being worked on.
	if w.processing.Has(task.GetID()) {
		return types.ErrSkipped
	}
	w.processing.Add(task.GetID(), struct{}{})
	w.queue <- task
	return nil
}

// Start starts the workers.
// Panics if already started.
func (w *Watcher) Start() {

	w.lck.Lock()
	started := w.started
	w.lck.Unlock()

	w.ticker = time.NewTicker(60 * time.Second)
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

	w.log.Debug("Scanning chain for new updates",
		"Repo", task.RepoName, "Ref", task.Reference, "EndHeight", task.EndHeight)

	isRepoTracked := w.keepers.RepoSyncInfoKeeper().GetTracked(task.RepoName) != nil

	// Walk up the blocks until the task's end height.
	// Find push transactions addressed to the target repository.
	start := task.StartHeight
	var foundTx bool
	for start <= task.EndHeight {
		block, err := w.service.GetBlock(int64(start))
		if err != nil {
			w.processing.Remove(task.GetID())
			return errors.Wrapf(err, "failed to get block (height=%d)", start)
		}

		foundTxInBlock := false
		for i, tx := range block.Block.Data.Txs {
			txObj, err := txns.DecodeTx(tx)
			if err != nil {
				w.processing.Remove(task.GetID())
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
					w.processing.Remove(task.GetID())
					return errors.Wrap(err, "failed to initialize repository")
				}
			}

			foundTxInBlock = true
			foundTx = true

			w.log.Debug("Found update for repo",
				"Repo", task.RepoName, "Ref", task.Reference, "Height", start)

			// Pass the transaction to the reference sync handler and pass a callback that removes
			// the task from the watcher queue once the refsync-er finishes with the task.
			w.refSyncHandler(obj, task.Reference, i, int64(start), func() {
				w.processing.Remove(task.GetID())
			})
		}

		// If no push transaction was found in this block and the repo is being tracked,
		// update the last synced block height of the tracked repo.
		if !foundTxInBlock && isRepoTracked {
			w.keepers.RepoSyncInfoKeeper().Track(task.RepoName, start)
		}

		start++
	}

	// Remove task from processing list if no tx was found
	if !foundTx {
		w.processing.Remove(task.GetID())
	}

	return nil
}
