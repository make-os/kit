package core

import (
	"context"

	config2 "gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/crypto"
	types2 "gitlab.com/makeos/mosdef/dht/types"
	"gitlab.com/makeos/mosdef/pkgs/logger"
	types4 "gitlab.com/makeos/mosdef/remote/pushpool/types"
	"gitlab.com/makeos/mosdef/remote/types"
	types3 "gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/modules"
	"gitlab.com/makeos/mosdef/util"
)

// Constants
const (
	RepoObjectModule = "repo-object"
)

// PushKeyGetter represents a function used for fetching a push key
type PushKeyGetter func(pushKeyID string) (crypto.PublicKey, error)

// PoolGetter returns various pools
type PoolGetter interface {

	// GetPushPool returns the push pool
	GetPushPool() types4.PushPool

	// GetMempool returns the transaction pool
	GetMempool() Mempool
}

// RepoGetter describes an interface for getting a local repository
type RepoGetter interface {

	// Get returns a repo handle
	GetRepo(name string) (types4.LocalRepo, error)
}

// RepoUpdater describes an interface for updating a repository from a push transaction
type RepoUpdater interface {
	// UpdateRepoWithTxPush attempts to merge a push transaction to a repository and
	// also update the repository's state tree.
	UpdateRepoWithTxPush(tx types3.BaseTx) error
}

type (
	// ColChangeType describes a change to a collection item
	ColChangeType int
)

const (
	// ChangeTypeNew represents a new, unique item added to a collection
	ChangeTypeNew ColChangeType = iota
	// ChangeTypeRemove represents a removal of a collection item
	ChangeTypeRemove
	// ChangeTypeUpdate represents an update to the value of a collection item
	ChangeTypeUpdate
)

// KVOption holds key-value structure of options
type KVOption struct {
	Key   string
	Value interface{}
}

// ItemChange describes a change event
type ItemChange struct {
	Item   Item
	Action ColChangeType
}

// ChangeResult includes information about changes
type ChangeResult struct {
	Changes []*ItemChange
}

// BareRepoState represents a repositories state
type BareRepoState interface {
	// GetReferences returns the references.
	GetReferences() Items
	// IsEmpty checks whether the state is empty
	IsEmpty() bool
	// GetChanges summarizes the changes between GetState s and y.
	GetChanges(y BareRepoState) *Changes
}

// Changes describes reference changes that happened to a repository
// from a previous state to its current state.
type Changes struct {
	References *ChangeResult
}

// Item represents a git object or reference
type Item interface {
	GetName() string
	Equal(o interface{}) bool
	GetData() string
	GetType() string
}

// Items represents a collection of git objects or references identified by a name
type Items interface {
	Has(name interface{}) bool
	Get(name interface{}) Item
	Equal(o interface{}) bool
	ForEach(func(i Item) bool)
	Len() int64
	Bytes() []byte
	Hash() util.Bytes32
}

// RepoPushEndorsement represents a push endorsement
type RepoPushEndorsement interface {
	// ID returns the hash of the object
	ID() util.Bytes32
	// Bytes returns a serialized version of the object
	Bytes() []byte
	// BytesAndID returns the serialized version of the tx and the id
	BytesAndID() ([]byte, util.Bytes32)
}

// RemoteServer provides functionality for manipulating repositories.
type RemoteServer interface {
	PoolGetter
	RepoGetter
	RepoUpdater

	// Log returns the logger
	Log() logger.Logger

	// Cfg returns the application config
	Cfg() *config2.AppConfig

	// GetRepoState returns the state of the repository at the given path
	// options: Allows the caller to configure how and what state are gathered
	GetRepoState(target types4.LocalRepo, options ...KVOption) (BareRepoState, error)

	// GetPushKeyGetter returns getter function for fetching a push key
	GetPushKeyGetter() PushKeyGetter

	// GetLogic returns the application logic provider
	GetLogic() Logic

	// GetPrivateValidatorKey returns the node's private key
	GetPrivateValidatorKey() *crypto.Key

	// Start starts the server
	Start() error

	// Wait can be used by the caller to wait till the server terminates
	Wait()

	// CreateRepository creates a local git repository
	CreateRepository(name string) error

	// BroadcastMsg broadcast messages to peers
	BroadcastMsg(ch byte, msg []byte)

	// BroadcastPushObjects broadcasts repo push note and push endorsement
	BroadcastPushObjects(note types4.PushNotice) error

	// SetPushKeyPubKeyGetter sets the PGP public key query function
	SetPushKeyPubKeyGetter(pkGetter PushKeyGetter)

	// RegisterAPIHandlers registers server API handlers
	RegisterAPIHandlers(agg modules.ModuleHub)

	// GetPruner returns the repo pruner
	GetPruner() types.RepoPruner

	// GetDHT returns the dht service
	GetDHT() types2.DHTNode

	// ExecTxPush applies a push transaction to the local repository.
	// If the node is a validator, only the target reference trees are updated.
	ExecTxPush(tx types3.BaseTx) error

	// Shutdown shuts down the server
	Shutdown(ctx context.Context)

	// Stop implements Reactor
	Stop() error
}
