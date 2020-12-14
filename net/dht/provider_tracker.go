package dht

import (
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
)

// ProviderTracker describes a structure for tracking provider performance
type ProviderTracker interface {
	// Register registers a new provider so it can be tracked
	Register(addrs ...peer.AddrInfo)

	// NumProviders returns the number of registered providers.
	NumProviders() int

	// Get a provider's information. If cb is provided, it is called with the provider
	Get(id peer.ID, cb func(*ProviderInfo)) *ProviderInfo

	// IsGood checks whether the given peer has a good record.
	IsGood(id peer.ID) bool

	// Ban bans a provider for the given duration.
	// If a peer is currently banned, the duration is added to its current ban time.
	Ban(peer peer.ID, dur time.Duration)

	// MarkFailure increments a provider's failure count.
	// When failure count reaches a max, ban the provider.
	MarkFailure(id peer.ID)

	// MarkSeen marks the provider's last seen time and resets its failure count.
	MarkSeen(id peer.ID)

	// PeerSentNope registers a NOPE response from the given peer indicating that
	// the object represented by the given key is unknown to it.
	PeerSentNope(id peer.ID, key []byte)

	// DidPeerSendNope checks whether the given peer previously sent NOPE for a key
	DidPeerSendNope(id peer.ID, key []byte) bool
}

// ProviderInfo contains information about a provider
type ProviderInfo struct {
	Addr        *peer.AddrInfo
	Failed      int
	LastFailure time.Time
	LastSeen    time.Time
}
