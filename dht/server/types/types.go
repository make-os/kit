package types

import (
	"context"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/make-os/lobe/dht/streamer/types"
)

// DHT represents a distributed hash table
type DHT interface {

	// Store adds a value corresponding to the given key
	Store(ctx context.Context, key string, value []byte) error

	// Lookup searches for a value corresponding to the given key
	Lookup(ctx context.Context, key string) ([]byte, error)

	// GetRepoObjectProviders finds peers capable of providing value for the given key
	GetProviders(ctx context.Context, key []byte) ([]peer.AddrInfo, error)

	// Announce informs the network that it can provide value for the given key
	Announce(key []byte, doneCB func(error))

	// BasicObjectStreamer returns the object streamer
	ObjectStreamer() types.ObjectStreamer

	// Host returns the wrapped IPFS host
	Host() host.Host

	// Start starts the DHT
	Start() error

	// Peers returns a list of all peers
	Peers() (peers []string)

	// Stop closes the host
	Stop() error
}
