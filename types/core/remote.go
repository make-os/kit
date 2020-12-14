package core

import (
	"context"

	"github.com/make-os/kit/config"
	"github.com/make-os/kit/crypto/ed25519"
	dht2 "github.com/make-os/kit/net/dht"
	"github.com/make-os/kit/pkgs/logger"
	"github.com/make-os/kit/remote/fetcher"
	pushtypes "github.com/make-os/kit/remote/push/types"
	remotetypes "github.com/make-os/kit/remote/types"
	"github.com/make-os/kit/rpc"
)

// PushKeyGetter represents a function used for fetching a push key
type PushKeyGetter func(pushKeyID string) (ed25519.PublicKey, error)

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
	GetPrivateValidatorKey() *ed25519.Key

	// Start starts the server
	Start() error

	// GetRPCHandler returns the RPC handler
	GetRPCHandler() *rpc.Handler

	// Wait can be used by the caller to wait till the server terminates
	Wait()

	// InitRepository creates a local git repository
	InitRepository(name string) error

	// BroadcastMsg broadcast messages to peers
	BroadcastMsg(ch byte, msg []byte)

	// BroadcastNoteAndEndorsement broadcasts repo push note and push endorsement
	BroadcastNoteAndEndorsement(note pushtypes.PushNote) error

	// Announce announces a key on the DHT network.
	// Returns true if object was successfully queued.
	Announce(objType int, repo string, hash []byte, doneCB func(error)) bool

	// GetFetcher returns the fetcher service
	GetFetcher() fetcher.ObjectFetcher

	// CheckNote validates a push note
	CheckNote(note pushtypes.PushNote) error

	// TryScheduleReSync may schedule a local reference for resynchronization if the pushed
	// reference old state does not match the current network state of the reference
	TryScheduleReSync(note pushtypes.PushNote, ref string, fromBeginning bool) error

	// GetDHT returns the dht service
	GetDHT() dht2.DHT

	// Shutdown shuts down the server
	Shutdown(ctx context.Context)

	// Stop implements Reactor
	Stop() error
}
