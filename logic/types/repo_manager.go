package types

import (
	"context"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/crypto"
	types2 "gitlab.com/makeos/mosdef/dht/types"
	"gitlab.com/makeos/mosdef/modules/types"
	"gitlab.com/makeos/mosdef/repo/types"
	"gitlab.com/makeos/mosdef/repo/types/core"
	"gitlab.com/makeos/mosdef/types/msgs"
	"gitlab.com/makeos/mosdef/util/logger"
)

// RepoManager provides functionality for manipulating repositories.
type RepoManager interface {
	core.PoolGetter
	core.RepoGetter
	core.TxPushMerger

	// Log returns the logger
	Log() logger.Logger

	// Cfg returns the application config
	Cfg() *config.AppConfig

	// GetRepoState returns the state of the repository at the given path
	// options: Allows the caller to configure how and what state are gathered
	GetRepoState(target core.BareRepo, options ...core.KVOption) (core.BareRepoState, error)

	// GetPGPPubKeyGetter returns the gpg getter function for finding GPG public
	// keys by their ID
	GetPGPPubKeyGetter() core.PGPPubKeyGetter

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

	// BroadcastPushObjects broadcasts repo push note and push OK
	BroadcastPushObjects(pushNote core.RepoPushNote) error

	// SetPGPPubKeyGetter sets the PGP public key query function
	SetPGPPubKeyGetter(pkGetter core.PGPPubKeyGetter)

	// RegisterAPIHandlers registers server API handlers
	RegisterAPIHandlers(agg types.ModulesAggregator)

	// GetPruner returns the repo pruner
	GetPruner() core.Pruner

	// GetDHT returns the dht service
	GetDHT() types2.DHT

	// ExecTxPush applies a push transaction to the local repository.
	// If the node is a validator, only the target reference trees are updated.
	ExecTxPush(tx *msgs.TxPush) error

	// Shutdown shuts down the server
	Shutdown(ctx context.Context)

	// Stop implements Reactor
	Stop() error
}

