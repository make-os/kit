package core

import (
	"context"

	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/dht"
	"gitlab.com/makeos/mosdef/pkgs/logger"
	pushtypes "gitlab.com/makeos/mosdef/remote/push/types"
	remotetypes "gitlab.com/makeos/mosdef/remote/types"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/modules"
)

// PushKeyGetter represents a function used for fetching a push key
type PushKeyGetter func(pushKeyID string) (crypto.PublicKey, error)

// PoolGetter returns various pools
type PoolGetter interface {

	// GetPushPool returns the push pool
	GetPushPool() pushtypes.PushPooler

	// GetMempool returns the transaction pool
	GetMempool() Mempool
}

// RepoGetter describes an interface for getting a local repository
type RepoGetter interface {

	// Get returns a repo handle
	GetRepo(name string) (remotetypes.LocalRepo, error)
}

// RepoUpdater describes an interface for updating a repository from a push transaction
type RepoUpdater interface {
	// UpdateRepoWithTxPush attempts to merge a push transaction to a repository and
	// also update the repository's state tree.
	UpdateRepoWithTxPush(tx types.BaseTx) error
}

// RemoteServer provides functionality for manipulating repositories.
type RemoteServer interface {
	PoolGetter
	RepoGetter
	RepoUpdater

	// Log returns the logger
	Log() logger.Logger

	// Cfg returns the application config
	Cfg() *config.AppConfig

	// GetRepoState returns the state of the repository at the given path
	// options: Allows the caller to configure how and what state are gathered
	GetRepoState(target remotetypes.LocalRepo, options ...remotetypes.KVOption) (remotetypes.BareRepoState, error)

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
	BroadcastPushObjects(note pushtypes.PushNotice) error

	// SetPushKeyPubKeyGetter sets the PGP public key query function
	SetPushKeyPubKeyGetter(pkGetter PushKeyGetter)

	// RegisterAPIHandlers registers server API handlers
	RegisterAPIHandlers(agg modules.ModuleHub)

	// GetPruner returns the repo pruner
	GetPruner() remotetypes.RepoPruner

	// GetDHT returns the dht service
	GetDHT() dht.DHT

	// ExecTxPush applies a push transaction to the local repository.
	// If the node is a validator, only the target reference trees are updated.
	ExecTxPush(tx types.BaseTx) error

	// Shutdown shuts down the server
	Shutdown(ctx context.Context)

	// Stop implements Reactor
	Stop() error
}
