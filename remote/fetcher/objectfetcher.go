package fetcher

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/themakeos/lobe/config"
	types3 "github.com/themakeos/lobe/dht/server/types"
	types2 "github.com/themakeos/lobe/dht/streamer/types"
	"github.com/themakeos/lobe/params"
	"github.com/themakeos/lobe/pkgs/logger"
	"github.com/themakeos/lobe/pkgs/queue"
	"github.com/themakeos/lobe/remote/plumbing"
	"github.com/themakeos/lobe/remote/push/types"
	io2 "github.com/themakeos/lobe/util/io"
	plumbing2 "gopkg.in/src-d/go-git.v4/plumbing"
)

// ObjectFetcher describes a module for fetching git objects from a given DHT service.
type ObjectFetcher interface {
	// Fetch adds a new task to the queue.
	// cb will be called when the task has been processed.
	Fetch(note types.PushNote, cb func(err error))

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

// ObjectFetcherService is like ObjectFetcher but exposes only commands
// necessary for safe use by other packages.
type ObjectFetcherService interface {

	// Fetch adds a new task to the queue.
	// cb will be called when the task has been processed.
	Fetch(note types.PushNote, cb func(err error))
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
	dht                types3.DHT
	lck                *sync.Mutex
	queue              *queue.UniqueQueue
	nWorkers           int
	log                logger.Logger
	stopped            bool
	started            bool
	onObjFetchedCb     func(string, io.ReadSeeker)
	PackToRepoUnpacker plumbing.PackToRepoUnpacker
}

// NewFetcher creates an instance of BasicObjectFetcher
func NewFetcher(dht types3.DHT, nWorkers int, cfg *config.AppConfig) *BasicObjectFetcher {
	return &BasicObjectFetcher{
		dht:                dht,
		lck:                &sync.Mutex{},
		queue:              queue.NewUnique(),
		nWorkers:           nWorkers,
		log:                cfg.G().Log.Module("object-fetcher"),
		cfg:                cfg,
		PackToRepoUnpacker: plumbing.UnpackPackfileToRepo,
	}
}

// addTask appends a tasks to the task queue
func (f *BasicObjectFetcher) addTask(task *Task) {
	f.queue.Append(task)
}

// IsQueueEmpty checks whether the task queue is empty
func (f *BasicObjectFetcher) IsQueueEmpty() bool {
	return f.queue.Empty()
}

// getTask returns a task
func (f *BasicObjectFetcher) getTask() *Task {
	item := f.queue.Head()
	if item == nil {
		return nil
	}
	return item.(*Task)
}

// Fetch adds a new task to the queue.
// cb will be called when the task has been processed.
func (f *BasicObjectFetcher) Fetch(note types.PushNote, cb func(error)) {
	f.addTask(&Task{note: note, resCb: cb})
	return
}

// QueueSize returns the size of the queue
func (f *BasicObjectFetcher) QueueSize() int {
	return f.queue.Size()
}

// OnPackReceived registers a callback that is called each time an object's packfile is received
func (f *BasicObjectFetcher) OnPackReceived(cb func(string, io.ReadSeeker)) {
	f.onObjFetchedCb = cb
}

// Start starts the workers.
// Panics if already started
func (f *BasicObjectFetcher) Start() {

	f.lck.Lock()
	started := f.started
	f.lck.Unlock()

	if started {
		panic("already started")
	}

	for i := 0; i < f.nWorkers; i++ {
		go f.createWorker(i)
	}

	f.lck.Lock()
	f.started = true
	f.lck.Unlock()
}

// createWorker creates a worker that fetches tasks from the queue
func (f *BasicObjectFetcher) createWorker(id int) {
	for !f.hasStopped() {
		task := f.getTask()
		if task != nil {
			f.do(id, task)
			continue
		}
		time.Sleep(5 * time.Second)
	}
}

// Operation attempts to fetch objects of the pushed references, one reference
// at a time and will immediate return error if on failure.
// For each object fetched, the resulting packfile decoded into the repository.
// Therefore, on error, objects of successfully fetched references will remain
// in the repository to be garbage collected by the pruner.
func (f *BasicObjectFetcher) Operation(id int, task *Task) error {
	streamer := f.dht.ObjectStreamer()

	for _, ref := range task.note.GetPushedReferences() {
		if plumbing.IsBranch(ref.Name) || plumbing.IsNote(ref.Name) {

			// Set end hash only if the pushed reference end hash is non-zero
			var endHash []byte
			if !plumbing.IsZeroHash(ref.OldHash) {
				endHash = plumbing.HashToBytes(ref.OldHash)
			}

			ctx, cn := context.WithTimeout(context.Background(), 60*time.Second)
			_, err := streamer.GetCommitWithAncestors(ctx, types2.GetAncestorArgs{
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
				cn()
				return err
			}
			cn()
			f.log.Debug("Reference objects successfully fetched", "ID", id, "Ref", ref.Name)
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
			check_end_hash:
				endTag, err := task.note.GetTargetRepo().TagObject(plumbing.BytesToHash(endHash))
				if err != nil {
					return err
				}
				switch endTag.TargetType {
				case plumbing2.CommitObject:
					endHash = endTag.Target[:]
				case plumbing2.TagObject:
					endHash = endTag.Target[:]
					goto check_end_hash
				default:
					endHash = nil
				}
			}

			ctx, cn := context.WithTimeout(context.Background(), 30*time.Second)
			_, err := streamer.GetTaggedCommitWithAncestors(ctx, types2.GetAncestorArgs{
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
				cn()
				return err
			}
			cn()
			f.log.Debug("Reference objects successfully fetched", "ID", id, "Ref", ref.Name)
		}
	}

	return nil
}

// do processes a task
// Try the Operation multiple times using an exponential backoff function
// as long as the max fetch attempt is not reached.
// On error, call the task's callback function with the error.
func (f *BasicObjectFetcher) do(id int, task *Task) {
	bf := backoff.WithMaxRetries(backoff.NewExponentialBackOff(), uint64(params.MaxNoteObjectFetchAttempts))
	task.resCb(backoff.Retry(func() error { return f.Operation(id, task) }, bf))
}

// hasStopped checks whether the fetcher has been stopped
func (f *BasicObjectFetcher) hasStopped() bool {
	f.lck.Lock()
	defer f.lck.Unlock()
	return f.stopped
}

// Stop stops the fetcher service
func (f *BasicObjectFetcher) Stop() {
	f.lck.Lock()
	f.stopped = true
	f.started = false
	f.lck.Unlock()
}
