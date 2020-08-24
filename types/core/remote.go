package core

import (
	"context"

	"github.com/make-os/lobe/config"
	"github.com/make-os/lobe/crypto"
	"github.com/make-os/lobe/dht/server/types"
	types2 "github.com/make-os/lobe/modules/types"
	"github.com/make-os/lobe/pkgs/logger"
	"github.com/make-os/lobe/remote/fetcher"
	pushtypes "github.com/make-os/lobe/remote/push/types"
	remotetypes "github.com/make-os/lobe/remote/types"
)

// PushKeyGetter represents a function used for fetching a push key
type PushKeyGetter func(pushKeyID string) (crypto.PublicKey, error)

// PoolGetter returns various pools
type PoolGetter interface {

	// GetPushPool returns the push pool
	GetPushPool() pushtypes.PushPool

	// GetMempool returns the transaction pool
	GetMempool() Mempool
}

// RemoteServer provides functionality for manipulating repositories.
type RemoteServer interface {
	PoolGetter

	// Log returns the logger
	Log() logger.Logger

	// Cfg returns the application config
	Cfg() *config.AppConfig

	// GetRepoState returns the state of the repository at the given path
	// options: Allows the caller to configure how and what state are gathered
	GetRepoState(target remotetypes.LocalRepo, options ...remotetypes.KVOption) (remotetypes.BareRepoRefsState, error)

	// GetPushKeyGetter returns getter function for fetching a push key
	GetPushKeyGetter() PushKeyGetter

	// GetLogic returns the application logic provider
	GetLogic() Logic

	// GetRepo get a local repository
	GetRepo(name string) (remotetypes.LocalRepo, error)

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

	// BroadcastNoteAndEndorsement broadcasts repo push note and push endorsement
	BroadcastNoteAndEndorsement(note pushtypes.PushNote) error

	// RegisterAPIHandlers registers server API handlers
	RegisterAPIHandlers(agg types2.ModulesHub)

	// AnnounceObject announces a git object to the DHT network
	AnnounceObject(hash []byte, doneCB func(error))

	// AnnounceRepoObjects announces all objects in a repository
	AnnounceRepoObjects(repoName string) error

	// GetFetcher returns the fetcher service
	GetFetcher() fetcher.ObjectFetcher

	// GetPruner returns the repo pruner
	GetPruner() remotetypes.RepoPruner

	// SetPruner sets the pruner
	SetPruner(pruner remotetypes.RepoPruner)

	// GetDHT returns the dht service
	GetDHT() types.DHT

	// Shutdown shuts down the server
	Shutdown(ctx context.Context)

	// Stop implements Reactor
	Stop() error
}
