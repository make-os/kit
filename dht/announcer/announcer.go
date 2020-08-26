package announcer

import (
	"context"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	dht2 "github.com/make-os/lobe/dht"
	"github.com/make-os/lobe/pkgs/logger"
	"github.com/make-os/lobe/pkgs/queue"
	"github.com/make-os/lobe/util"
	"github.com/thoas/go-funk"
)

type Announcer interface {
	// Announce adds a key to the queue to be announced
	Announce(key []byte, doneCB func(error))

	// Start starts the announcer.
	// Panics if reference announcer is already started.
	Start()

	// IsRunning checks if the announcer is running.
	IsRunning() bool

	// HasTask checks whether there are one or more unprocessed tasks.
	HasTask() bool

	// Stops the announcer
	Stop()
}

// Task represents a task
type Task struct {
	Key  util.Bytes
	Done func(err error)
}

func (t *Task) GetID() interface{} {
	return t.Key.String()
}

// BasicAnnouncer implements Announcer.
// It provides the mechanism for announcing keys on the DHT.
// Announcement requests are queued up an concurrently executed by n workers.
// When an announcement fails, it is retried several times.
type BasicAnnouncer struct {
	log      logger.Logger
	nWorkers int
	dht      *dht.IpfsDHT
	lck      *sync.Mutex
	queue    *queue.UniqueQueue
	started  bool
	stopped  bool
}

// NewBasicAnnouncer creates an instance of BasicAnnouncer
func NewBasicAnnouncer(dht *dht.IpfsDHT, nWorkers int, log logger.Logger) *BasicAnnouncer {
	rs := &BasicAnnouncer{
		dht:      dht,
		nWorkers: nWorkers,
		lck:      &sync.Mutex{},
		log:      log,
		queue:    queue.NewUnique(),
	}
	return rs
}

// Announce adds a key to the queue to be announced
func (a *BasicAnnouncer) Announce(key []byte, doneCB func(error)) {
	if doneCB == nil {
		doneCB = func(error) {}
	}
	a.queue.Append(&Task{Key: key, Done: doneCB})
}

// HasTask checks whether there are one or more unprocessed tasks.
func (a *BasicAnnouncer) HasTask() bool {
	return !a.queue.Empty()
}

// Start starts the workers.
// Panics if already started.
func (a *BasicAnnouncer) Start() {

	a.lck.Lock()
	started := a.started
	a.lck.Unlock()

	if started {
		panic("already started")
	}

	for i := 0; i < a.nWorkers; i++ {
		go a.createWorker(i)
	}

	a.lck.Lock()
	a.started = true
	a.lck.Unlock()
}

// IsRunning checks if the announcer is running.
func (a *BasicAnnouncer) IsRunning() bool {
	return a.started
}

// createWorker creates a worker that performs tasks in the queue
func (a *BasicAnnouncer) createWorker(id int) {
	for !a.hasStopped() {
		task := a.getTask()
		if task != nil {
			if err := a.Do(id, task); err != nil {
				a.log.Error(err.Error())
			}
			continue
		}
		time.Sleep(time.Duration(funk.RandomInt(1, 5)) * time.Second)
	}
}

// hasStopped checks whether the announcer has been stopped
func (a *BasicAnnouncer) hasStopped() bool {
	a.lck.Lock()
	defer a.lck.Unlock()
	return a.stopped
}

// getTask returns a task
func (a *BasicAnnouncer) getTask() *Task {
	item := a.queue.Head()
	if item == nil {
		return nil
	}
	return item.(*Task)
}

// Stop stops the announcer
func (a *BasicAnnouncer) Stop() {
	a.lck.Lock()
	a.stopped = true
	a.started = false
	a.lck.Unlock()
}

// Do announces the key in the given task
func (a *BasicAnnouncer) Do(workerID int, task *Task) error {

	// Make CID out of the key
	key := task.Key
	cid, err := dht2.MakeCid(key)
	if err != nil {
		task.Done(err)
		return err
	}

	// Broadcast as provider of the key to the DHT.
	// Allow exponential backoff retries on failure with a max. of 5 tries.
	err = backoff.Retry(func() error {
		ctx, cn := context.WithTimeout(context.Background(), 60*time.Second)
		if err := a.dht.Provide(ctx, cid, true); err != nil {
			cn()
			return err
		}
		cn()
		return nil
	}, backoff.WithMaxRetries(backoff.NewExponentialBackOff(), 5))

	if err != nil {
		a.log.Error("Failed to announce key", "Err", err, "Key", task.Key.HexStr(true))
		task.Done(err)
		return err
	}

	a.log.Debug("Successfully announced a key", "Key", task.Key.HexStr(true))

	task.Done(nil)

	return nil
}
