package types

import (
	"github.com/make-os/lobe/remote/push/types"
	"github.com/make-os/lobe/types/txns"
	"github.com/make-os/lobe/util"
)

type Watcher interface {
	Do(task *WatcherTask) error
	Watch(repo, reference string, startHeight, endHeight uint64) error
	QueueSize() int
	HasTask() bool
	IsRunning() bool
	Start()
	Stop()
}

// WatcherTask represents a watcher task
type WatcherTask struct {
	RepoName    string // The name of the repository
	Reference   string // The target reference to be watched
	StartHeight uint64 // The block height to start syncing from
	EndHeight   uint64 // The block height to end syncing
}

func (t *WatcherTask) GetID() interface{} {
	return t.RepoName + t.Reference
}

// RefSync describes an interface for synchronizing a repository's
// reference local state with the network using information from a
// push transaction.
type RefSync interface {

	// OnNewTx receives push transactions and adds non-delete
	// pushed references to the task queue.
	// targetRef is the specific pushed reference that will be queued. If unset, all references are queued.
	// txIndex is the index of the transaction it its containing block.
	// height is the block height that contains the transaction.
	OnNewTx(tx *txns.TxPush, targetRef string, txIndex int, height int64, doneCb func())

	// Watch adds a repository to the watch queue
	Watch(repo, reference string, startHeight, endHeight uint64) error

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

	// TxIndex is the transaction index in its containing block
	TxIndex int

	// Height is the block height where the reference updated occurred
	Height int64

	// Timestamp is the time the transaction was created
	Timestamp int64

	// Endorsements are the endorsements in the push transaction
	Endorsements txns.PushEndorsements

	// NoteCreator is the public key of the note creator
	NoteCreator util.Bytes32

	// Done is called when the task has been completed
	Done func()
}

func (t *RefTask) GetID() interface{} {
	return t.ID
}
