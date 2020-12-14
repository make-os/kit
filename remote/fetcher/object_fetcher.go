package fetcher

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/make-os/kit/config"
	dht2 "github.com/make-os/kit/net/dht"
	"github.com/make-os/kit/pkgs/logger"
	"github.com/make-os/kit/remote/plumbing"
	"github.com/make-os/kit/remote/push/types"
	io2 "github.com/make-os/kit/util/io"
	plumbing2 "gopkg.in/src-d/go-git.v4/plumbing"
)

// ObjectFetcher describes a module for fetching git objects from a given DHT service.
type ObjectFetcher interface {
	// FetchAsync adds a new task to the queue and returns immediately.
	// cb will be called when the task has been processed.
	FetchAsync(note types.PushNote, cb func(err error))

	// QueueSize returns the size of the queue
	QueueSize() int

	// OnPackReceived registers a callback that is called each time a packfile
	// of an object is fetched
	OnPackReceived(cb func(hash string, packfile io.ReadSeeker))

	// Start starts the fetcher service
	Start()

	// Stops the fetcher service
	Stop()
}

// ObjectFetcherService is like ObjectFetcher but exposes limited methods.
type ObjectFetcherService interface {

	// FetchAsync adds a new task to the queue.
	// cb will be called when the task has been processed.
	FetchAsync(note types.PushNote, cb func(err error))

	// OnPackReceived registers a callback that is called each time a packfile
	// of an object is fetched
	OnPackReceived(cb func(hash string, packfile io.ReadSeeker))
}

// Task represents a fetch task
type Task struct {
	note  types.PushNote
	resCb func(err error)
}

func (t *Task) GetID() interface{} {
	return t.note.ID().String()
}

// NewTask creates an instance of Task
func NewTask(note types.PushNote, resCb func(err error)) *Task {
	return &Task{note: note, resCb: resCb}
}

// BasicObjectFetcher provides the ability to download objects from the DHT.
type BasicObjectFetcher struct {
	cfg                *config.AppConfig
	dht                dht2.DHT
	lck                *sync.Mutex
	log                logger.Logger
	stopped            bool
	started            bool
	queue              chan *Task
	onObjFetchedCb     func(string, io.ReadSeeker)
	PackToRepoUnpacker plumbing.PackToRepoUnpacker
}

// NewFetcher creates an instance of BasicObjectFetcher
func NewFetcher(dht dht2.DHT, cfg *config.AppConfig) *BasicObjectFetcher {
	return &BasicObjectFetcher{
		log:                cfg.G().Log.Module("object-fetcher"),
		dht:                dht,
		lck:                &sync.Mutex{},
		queue:              make(chan *Task, 10000),
		cfg:                cfg,
		PackToRepoUnpacker: plumbing.UnpackPackfileToRepo,
	}
}

// addTask appends a tasks to the task queue
func (f *BasicObjectFetcher) addTask(task *Task) {
	f.queue <- task
}

// IsQueueEmpty checks whether the task queue is empty
func (f *BasicObjectFetcher) IsQueueEmpty() bool {
	return len(f.queue) == 0
}

// Fetch adds a new task to the queue.
// cb will be called when the task has been processed.
func (f *BasicObjectFetcher) FetchAsync(note types.PushNote, cb func(error)) {
	f.addTask(&Task{note: note, resCb: cb})
}

// QueueSize returns the size of the queue
func (f *BasicObjectFetcher) QueueSize() int {
	return len(f.queue)
}

// OnPackReceived registers a callback that is called each time an object's packfile is received
func (f *BasicObjectFetcher) OnPackReceived(cb func(string, io.ReadSeeker)) {
	f.onObjFetchedCb = cb
}

// Start starts processing tasks in the queue.
// It does not block.
// Panics if already started
func (f *BasicObjectFetcher) Start() {

	f.lck.Lock()
	started := f.started
	f.lck.Unlock()

	if started {
		panic("already started")
	}

	go func() {
		for task := range f.queue {
			go f.do(task)
		}
	}()

	f.lck.Lock()
	f.started = true
	f.lck.Unlock()
}

// Operation attempts to fetch objects of the pushed references, one reference
// at a time and will immediate return error if on failure.
// For each object fetched, the resulting packfile decoded into the repository.
// Therefore, on error, objects of successfully fetched references will remain
// in the repository to be garbage collected by the pruner.
func (f *BasicObjectFetcher) Operation(task *Task) error {
	streamer := f.dht.ObjectStreamer()

	for _, ref := range task.note.GetPushedReferences() {
		if plumbing.IsBranch(ref.Name) || plumbing.IsNote(ref.Name) {

			// Set end hash only if the pushed reference end hash is non-zero
			var endHash []byte
			if !plumbing.IsZeroHash(ref.OldHash) {
				endHash = plumbing.HashToBytes(ref.OldHash)
			}

			ctx, cn := context.WithTimeout(context.Background(), 60*time.Second)
			_, err := streamer.GetCommitWithAncestors(ctx, dht2.GetAncestorArgs{
				RepoName:         task.note.GetRepoName(),
				StartHash:        plumbing.HashToBytes(ref.NewHash),
				EndHash:          endHash,
				ExcludeEndCommit: true,
				GitBinPath:       f.cfg.Node.GitBinPath,
				ReposDir:         f.cfg.GetRepoRoot(),
				ResultCB: func(packfile io2.ReadSeekerCloser, hash string) error {
					err := f.PackToRepoUnpacker(task.note.GetTargetRepo(), packfile)
					if err != nil {
						packfile.Close()
						return err
					}
					if f.onObjFetchedCb != nil {
						f.onObjFetchedCb(hash, packfile)
					}
					packfile.Close()
					return nil
				},
			})
			if err != nil {
				f.log.Error("failed to fetch object(s) of reference",
					"Name", ref.Name, "OldHash", ref.OldHash, "NewHash", ref.NewHash, "Err", err)
				cn()
				return err
			}
			cn()
			f.log.Debug("Reference object(s) successfully fetched", "Ref", ref.Name)
		}

		if plumbing.IsTag(ref.Name) {

			// Set end hash only if the pushed reference end hash is non-zero
			var endHash []byte
			if !plumbing.IsZeroHash(ref.OldHash) {
				endHash = plumbing.HashToBytes(ref.OldHash)
			}

			// If end hash is set but does not point to a commit, it's no use for ancestry query.
			// If points to a tag object, recursively walk up the tag target till a commit target
			// is found. If target is not a commit or a tag, set endHash to nil which will cause
			// the object streamer to only fetch the tag.
			if len(endHash) > 0 {
			checkEndHash:
				endTag, err := task.note.GetTargetRepo().TagObject(plumbing.BytesToHash(endHash))
				if err != nil {
					return err
				}
				switch endTag.TargetType {
				case plumbing2.CommitObject:
					endHash = endTag.Target[:]
				case plumbing2.TagObject:
					endHash = endTag.Target[:]
					goto checkEndHash
				default:
					endHash = nil
				}
			}

			ctx, cn := context.WithTimeout(context.Background(), 30*time.Second)
			_, err := streamer.GetTaggedCommitWithAncestors(ctx, dht2.GetAncestorArgs{
				RepoName:         task.note.GetRepoName(),
				StartHash:        plumbing.HashToBytes(ref.NewHash),
				EndHash:          endHash,
				ExcludeEndCommit: true,
				GitBinPath:       f.cfg.Node.GitBinPath,
				ReposDir:         f.cfg.GetRepoRoot(),
				ResultCB: func(packfile io2.ReadSeekerCloser, hash string) error {
					err := f.PackToRepoUnpacker(task.note.GetTargetRepo(), packfile)
					if err != nil {
						packfile.Close()
						return err
					}
					if f.onObjFetchedCb != nil {
						f.onObjFetchedCb(hash, packfile)
					}
					return nil
				},
			})
			if err != nil {
				f.log.Error("failed to fetch object(s) of reference",
					"Name", ref.Name, "OldHash", ref.OldHash, "NewHash", ref.NewHash, "Err", err)
				cn()
				return err
			}
			cn()
			f.log.Debug("Reference object(s) successfully fetched", "Ref", ref.Name)
		}
	}

	return nil
}

// do processes a task
// Try the Operation multiple times using an exponential backoff function.
// On error, call the task's callback function with the error.
func (f *BasicObjectFetcher) do(task *Task) {
	bf := backoff.NewExponentialBackOff()
	bf.MaxElapsedTime = 15 * time.Minute
	task.resCb(backoff.Retry(func() error { return f.Operation(task) }, bf))
}

// Stop stops the fetcher service
func (f *BasicObjectFetcher) Stop() {
	f.lck.Lock()
	close(f.queue)
	f.stopped = true
	f.started = false
	f.lck.Unlock()
}
