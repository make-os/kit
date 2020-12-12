package announcer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	cid2 "github.com/ipfs/go-cid"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/make-os/kit/config"
	dht2 "github.com/make-os/kit/dht"
	"github.com/make-os/kit/dht/types"
	"github.com/make-os/kit/pkgs/logger"
	"github.com/make-os/kit/remote/plumbing"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/util"
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

// Session represents a multi-announcement session.
type Session struct {
	a        types.Announcer
	wg       *sync.WaitGroup
	errCount int
}

// NewSession creates an instance of Session
func NewSession(a types.Announcer) *Session {
	return &Session{a: a, wg: &sync.WaitGroup{}}
}

// Announce an object
func (s *Session) Announce(objType int, repo string, key []byte) bool {
	s.wg.Add(1)
	announced := s.a.Announce(objType, repo, key, func(err error) {
		if err != nil {
			s.errCount++
		}
		s.wg.Done()
	})
	if !announced {
		s.wg.Done()
	}
	return announced
}

// OnDone calls the callback with the number of failed announcements in the session.
func (s *Session) OnDone(cb func(errCount int)) {
	s.wg.Wait()
	cb(s.errCount)
}

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
	queue       chan *Task
	queued      map[string]struct{}
	reannouncer *time.Ticker
	started     bool
	stopped     bool
}

// New creates an instance of Announcer
func New(cfg *config.AppConfig, dht *dht.IpfsDHT, keepers core.Keepers) *Announcer {
	rs := &Announcer{
		keepers:  keepers,
		dht:      dht,
		checkers: &sync.Map{},
		lck:      &sync.Mutex{},
		log:      cfg.G().Log.Module("announcer"),
		queue:    make(chan *Task, 10000),
		queued:   make(map[string]struct{}),
	}

	go func() {
		config.GetInterrupt().Wait()
		rs.Stop()
	}()

	return rs
}

// addTask adds a new announcement task to the queue.
// Returns true if task was added or false if it already exist in the queue.
func (a *Announcer) addTask(task *Task) bool {
	a.lck.Lock()
	defer a.lck.Unlock()
	if _, ok := a.queued[task.GetID().(string)]; !ok {
		a.queue <- task
		a.queued[task.GetID().(string)] = struct{}{}
		return true
	}
	return false
}

// GetQueued returns the index containing tasks awaiting processing.
func (a *Announcer) GetQueued() map[string]struct{} {
	return a.queued
}

// finishedTask removes the task from the queued index
func (a *Announcer) finishedTask(task *Task) {
	a.lck.Lock()
	defer a.lck.Unlock()
	delete(a.queued, task.GetID().(string))
}

// Announce queues an object to be announced.
// objType is the type of the object.
// repo is the name of the repository where the object can be found.
// key is the unique identifier of the object.
// doneCB is called after successful announcement
// Returns true if object has been successfully queued
func (a *Announcer) Announce(objType int, repo string, key []byte, doneCB func(error)) bool {
	task := &Task{Type: objType, RepoName: repo, Key: key, Done: doneCB}
	return a.addTask(task)
}

// NewSession creates an instance of Session
func (a *Announcer) NewSession() types.Session {
	return NewSession(a)
}

// HasTask checks whether there are one or more unprocessed tasks.
func (a *Announcer) HasTask() bool {
	return len(a.queue) > 0
}

// QueueSize returns the size of the queue
func (a *Announcer) QueueSize() int {
	return len(a.queue)
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

	go func() {
		for task := range a.queue {
			go a.Do(task)
		}
	}()

	a.lck.Lock()
	a.started = true
	a.lck.Unlock()
}

// IsRunning checks if the announcer is running.
func (a *Announcer) IsRunning() bool {
	return a.started
}

// hasStopped checks whether the announcer has been stopped
func (a *Announcer) hasStopped() bool {
	a.lck.Lock()
	defer a.lck.Unlock()
	return a.stopped
}

// getTask returns a task from the queue
func (a *Announcer) getTask() *Task {
	return <-a.queue
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
func (a *Announcer) Do(task *Task) (err error) {

	if task.Done == nil {
		task.Done = func(err error) {}
	}
	defer func() {
		a.finishedTask(task)
		task.Done(err)
	}()

	// If the key's existence needs to be checked, perform check operation.
	if task.CheckExistence && !a.keyExist(task) {
		err = ErrDelisted
		return
	}

	// Make CID out of the key
	var key = task.Key
	var cid cid2.Cid
	cid, err = dht2.MakeCID(key)
	if err != nil {
		return
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
		return
	}

	// (Re)add the key to the announce list
	a.keepers.DHTKeeper().AddToAnnounceList(key, task.RepoName, task.Type, time.Now().Add(KeyReannounceDur).Unix())

	a.log.Debug("Successfully announced a key", "Key", plumbing.BytesToHex(key))

	return
}

// Reannounce finds keys that need to be re-announced
func (a *Announcer) Reannounce() {
	a.keepers.DHTKeeper().IterateAnnounceList(func(key []byte, entry *core.AnnounceListEntry) {
		annTime := time.Unix(entry.NextTime, 0)
		if time.Now().After(annTime) || time.Now().Equal(annTime) {
			a.addTask(&Task{Type: entry.Type, RepoName: entry.Repo, Key: key, CheckExistence: true})
		}
	})
}

// RegisterChecker allows external caller to register existence checker
// for a given object type. Only one checker per object type.
func (a *Announcer) RegisterChecker(objType int, checker types.CheckFunc) {
	a.checkers.Store(objType, checker)
}
