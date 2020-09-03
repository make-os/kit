package announcer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	dht2 "github.com/make-os/lobe/dht"
	"github.com/make-os/lobe/dht/types"
	"github.com/make-os/lobe/params"
	"github.com/make-os/lobe/pkgs/logger"
	"github.com/make-os/lobe/pkgs/queue"
	"github.com/make-os/lobe/remote/plumbing"
	"github.com/make-os/lobe/types/core"
	"github.com/make-os/lobe/util"
	"github.com/thoas/go-funk"
)

const (
	ObjTypeAny int = iota
	ObjTypeGit
	ObjTypeRepoName
)

// ErrDelisted indicates that a task failed to be announced because it was delisted
var ErrDelisted = fmt.Errorf("key delisted")

const (
	// KeyReannounceDur is the duration that passes between key re-announcement.
	KeyReannounceDur = 5 * time.Hour

	// ReannouncerInterval is the duration between each reannoucement process.
	ReannouncerInterval = 30 * time.Second
)

// MaxRetry is the number of times to try to reannounce a key
var MaxRetry = 3

// Task represents a key that needs to be announced.
type Task struct {
	Type           int             // The type of object the key represents.
	RepoName       string          // The name of the repository where the object can be found
	Key            util.Bytes      // The unique object key
	CheckExistence bool            // Indicates that existence check should be performed on the key.
	Done           func(err error) // Done is called on success or failure
}

func (t *Task) GetID() interface{} {
	return t.Key.String()
}

// Announcer implements types.Announcer.
// It provides the mechanism for announcing keys on the DHT.
// Announcement requests are queued up an concurrently executed by n workers.
// When an announcement fails, it is retried several times.
type Announcer struct {
	keepers     core.Keepers
	log         logger.Logger
	dht         *dht.IpfsDHT
	checkers    *sync.Map
	lck         *sync.Mutex
	queue       *queue.UniqueQueue
	reannouncer *time.Ticker
	started     bool
	stopped     bool
}

// New creates an instance of Announcer
func New(dht *dht.IpfsDHT, keepers core.Keepers, log logger.Logger) *Announcer {
	rs := &Announcer{
		keepers:  keepers,
		dht:      dht,
		checkers: &sync.Map{},
		lck:      &sync.Mutex{},
		log:      log,
		queue:    queue.NewUnique(),
	}
	return rs
}

// Announce queues an object to be announced.
// objType is the type of the object.
// repo is the name of the repository where the object can be found.
// key is the unique identifier of the object.
// doneCB is called after successful announcement
func (a *Announcer) Announce(objType int, repo string, key []byte, doneCB func(error)) {
	a.queue.Append(&Task{Type: objType, RepoName: repo, Key: key, Done: doneCB})
}

// HasTask checks whether there are one or more unprocessed tasks.
func (a *Announcer) HasTask() bool {
	return !a.queue.Empty()
}

// QueueSize returns the size of the queue
func (a *Announcer) QueueSize() int {
	return a.queue.Size()
}

// Start starts the workers.
// Panics if already started.
func (a *Announcer) Start() {

	a.lck.Lock()
	started := a.started
	a.lck.Unlock()

	if started {
		panic("already started")
	}

	a.reannouncer = time.NewTicker(ReannouncerInterval)
	go func() {
		for range a.reannouncer.C {
			a.Reannounce()
		}
	}()

	for i := 0; i < params.NumAnnouncerWorker; i++ {
		go a.createWorker(i)
	}

	a.lck.Lock()
	a.started = true
	a.lck.Unlock()
}

// IsRunning checks if the announcer is running.
func (a *Announcer) IsRunning() bool {
	return a.started
}

// createWorker creates a worker that performs tasks in the queue
func (a *Announcer) createWorker(id int) {
	for !a.hasStopped() {
		task := a.getTask()
		if task != nil {
			if err := a.Do(id, task); err != nil && err != ErrDelisted {
				a.log.Error(err.Error())
			}
			continue
		}
		time.Sleep(time.Duration(funk.RandomInt(1, 5)) * time.Second)
	}
}

// hasStopped checks whether the announcer has been stopped
func (a *Announcer) hasStopped() bool {
	a.lck.Lock()
	defer a.lck.Unlock()
	return a.stopped
}

// getTask returns a task
func (a *Announcer) getTask() *Task {
	item := a.queue.Head()
	if item == nil {
		return nil
	}
	return item.(*Task)
}

// Stop stops the announcer
func (a *Announcer) Stop() {
	a.lck.Lock()
	a.stopped = true
	a.started = false
	if a.reannouncer != nil {
		a.reannouncer.Stop()
	}
	a.lck.Unlock()
}

// keyExist performs existence check for a given task's key.
// If the checker for the object type is not found, the key
// is removed from the announce list
func (a *Announcer) keyExist(task *Task) bool {
	cf, ok := a.checkers.Load(task.Type)
	if !ok {
		a.keepers.DHTKeeper().RemoveFromAnnounceList(task.Key)
		return false
	}
	return cf.(types.CheckFunc)(task.RepoName, task.Key)
}

// Do announces the key in the given task.
// After announcement, the key is (re)added to the announce list.
func (a *Announcer) Do(workerID int, task *Task) error {

	// If the key's existence needs to be checked, perform check operation.
	if task.CheckExistence && !a.keyExist(task) {
		return ErrDelisted
	}

	if task.Done == nil {
		task.Done = func(err error) {}
	}

	// Make CID out of the key
	key := task.Key
	cid, err := dht2.MakeCID(key)
	if err != nil {
		task.Done(err)
		return err
	}

	// Broadcast as provider of the key to the DHT.
	// Allow exponential backoff retries on failure.
	err = backoff.Retry(func() error {
		ctx, cn := context.WithTimeout(context.Background(), 60*time.Second)
		if err := a.dht.Provide(ctx, cid, true); err != nil {
			cn()
			return err
		}
		cn()
		return nil
	}, backoff.WithMaxRetries(backoff.NewExponentialBackOff(), uint64(MaxRetry)))
	if err != nil {
		a.log.Error("Failed to announce key", "Err", err, "Key", plumbing.BytesToHex(key))
		task.Done(err)
		return err
	}

	// (Re)add the key to the announce list
	a.keepers.DHTKeeper().AddToAnnounceList(key, task.RepoName, task.Type, time.Now().Add(KeyReannounceDur).Unix())

	// Call the task's callback function if set
	task.Done(nil)

	a.log.Debug("Successfully announced a key", "Key", plumbing.BytesToHex(key))

	return nil
}

// Reannounce finds keys that need to be re-announced
func (a *Announcer) Reannounce() {
	a.keepers.DHTKeeper().IterateAnnounceList(func(key []byte, entry *core.AnnounceListEntry) {
		annTime := time.Unix(entry.NextTime, 0)
		if time.Now().After(annTime) || time.Now().Equal(annTime) {
			a.queue.Append(&Task{Type: entry.Type, RepoName: entry.Repo, Key: key, CheckExistence: true})
		}
	})
}

// RegisterChecker allows external caller to register existence checker
// for a given object type. Only one checker per object type.
func (a *Announcer) RegisterChecker(objType int, checker types.CheckFunc) {
	a.checkers.Store(objType, checker)
}
