package core

import (
	"context"

	"github.com/make-os/lobe/config"
	"github.com/make-os/lobe/crypto"
	"github.com/make-os/lobe/dht/types"
	"github.com/make-os/lobe/pkgs/logger"
	"github.com/make-os/lobe/remote/fetcher"
	pushtypes "github.com/make-os/lobe/remote/push/types"
	remotetypes "github.com/make-os/lobe/remote/types"
	"github.com/make-os/lobe/rpc"
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
	GetRepoState(target remotetypes.LocalRepo, options ...remotetypes.KVOption) (remotetypes.RepoRefsState, error)

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

	// GetRPCHandler returns the RPC handler
	GetRPCHandler() *rpc.Handler

	// Wait can be used by the caller to wait till the server terminates
	Wait()

	// CreateRepository creates a local git repository
	InitRepository(name string) error

	// BroadcastMsg broadcast messages to peers
	BroadcastMsg(ch byte, msg []byte)

	// BroadcastNoteAndEndorsement broadcasts repo push note and push endorsement
	BroadcastNoteAndEndorsement(note pushtypes.PushNote) error

	// AnnounceObject announces a key on the DHT network
	Announce(objType int, repo string, hash []byte, doneCB func(error))

	// GetFetcher returns the fetcher service
	GetFetcher() fetcher.ObjectFetcher

	// GetDHT returns the dht service
	GetDHT() types.DHT

	// Shutdown shuts down the server
	Shutdown(ctx context.Context)

	// Stop implements Reactor
	Stop() error
}
