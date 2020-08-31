package types

import (
	"github.com/make-os/lobe/remote/fetcher"
	"github.com/make-os/lobe/remote/push/types"
	"github.com/make-os/lobe/types/txns"
	"github.com/make-os/lobe/util"
)

type Watcher interface {
	Do(task *WatcherTask, workerID int) error
	QueueSize() int
	HasTask() bool
	IsRunning() bool
	Start()
	Stop()
}

// WatcherTask represents a watcher task
type WatcherTask struct {
	RepoName    string
	StartHeight uint64
	EndHeight   uint64
}

func (t *WatcherTask) GetID() interface{} {
	return t.RepoName
}

// RefSync describes an interface for synchronizing a repository's
// reference local state with the network using information from a
// push transaction.
type RefSync interface {

	// Do takes a pushed reference task and attempts to fetch the objects
	// required to update the reference's local state.
	Do(task *RefTask, workerID int) error

	// OnNewTx is called for every push transaction processed.
	// height is the block height that contains the transaction.
	OnNewTx(tx *txns.TxPush, height int64)

	// SetFetcher sets the object fetcher
	SetFetcher(fetcher fetcher.ObjectFetcherService)

	// Start starts the syncer.
	// Panics if reference syncer is already started.
	Start()

	// IsRunning checks if the syncer is running.
	IsRunning() bool

	// HasTask checks whether there are one or more unprocessed tasks.
	HasTask() bool

	// QueueSize returns the size of the tasks queue
	QueueSize() int

	// CanSync checks whether the target repository of a push transaction can be synchronized.
	CanSync(namespace, repoName string) error

	// Stops the syncer
	Stop()
}

// RefTask represents a reference synchronization task
type RefTask struct {
	// ID is the unique ID of the task
	ID string

	// RepoName is the target repository name
	RepoName string

	// Ref is the pushed reference
	Ref *types.PushedReference

	// Height is the block height where the reference updated occurred
	Height int64

	// Endorsements are the endorsements in the push transaction
	Endorsements txns.PushEndorsements

	// NoteCreator is the public key of the note creator
	NoteCreator util.Bytes32
}

func (t *RefTask) GetID() interface{} {
	return t.ID
}
