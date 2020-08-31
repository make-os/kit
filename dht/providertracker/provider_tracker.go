package providertracker

import (
	"sync"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/make-os/lobe/pkgs/cache"
	"github.com/make-os/lobe/util/crypto"
)

var (
	// MaxFailureBeforeBan is the number of failures before a provider is banned
	MaxFailureBeforeBan = 3

	// BanDueToFailureDur is the duration of a ban due to failure
	BanDueToFailureDur = 15 * time.Minute

	// BackOffDurAfterFailure is the backoff time before a failed provider can be good again.
	BackOffDurAfterFailure = 15 * time.Minute
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

// BasicProviderTracker is used to track status and behaviour of providers.
type BasicProviderTracker struct {
	// banned contains a collection of banned peers.
	// The key is the cache peer ID.
	banned *cache.Cache

	// nopeCache contains a collection of peers that responded
	// with NOPE for a given key. The cache key must be hash(peerID:key).
	nopeCache *cache.Cache

	lck       *sync.Mutex
	providers map[string]*ProviderInfo
}

// NewProviderTracker creates an instance of BasicProviderTracker.
func NewProviderTracker() *BasicProviderTracker {
	return &BasicProviderTracker{
		lck:       &sync.Mutex{},
		banned:    cache.NewCacheWithExpiringEntry(100000),
		nopeCache: cache.NewCacheWithExpiringEntry(100000),
		providers: make(map[string]*ProviderInfo),
	}
}

// Register implements ProviderTracker
func (m *BasicProviderTracker) Register(addrs ...peer.AddrInfo) {
	m.lck.Lock()
	defer m.lck.Unlock()
	for _, addr := range addrs {
		if _, ok := m.providers[addr.ID.Pretty()]; !ok {
			m.providers[addr.ID.Pretty()] = &ProviderInfo{Addr: &addr, LastSeen: time.Now()}
		}
	}
}

// NumProviders implements ProviderTracker
func (m *BasicProviderTracker) NumProviders() int {
	return len(m.providers)
}

// BanCache returns the cache containing banned peers
func (m *BasicProviderTracker) BanCache() *cache.Cache {
	return m.banned
}

// Get implements ProviderTracker
func (m *BasicProviderTracker) Get(id peer.ID, cb func(*ProviderInfo)) *ProviderInfo {
	m.lck.Lock()
	defer m.lck.Unlock()
	provider, ok := m.providers[id.Pretty()]
	if !ok {
		return nil
	}
	if cb != nil {
		cb(provider)
	}
	return provider
}

// PeerSentNope implements ProviderTracker
func (m *BasicProviderTracker) PeerSentNope(id peer.ID, key []byte) {
	cacheKey := crypto.Hash20Hex(append([]byte(id.Pretty()), key...))
	m.nopeCache.Add(cacheKey, struct{}{}, time.Now().Add(10*time.Minute))
}

// DidPeerSendNope implements ProviderTracker
func (m *BasicProviderTracker) DidPeerSendNope(id peer.ID, key []byte) bool {
	cacheKey := crypto.Hash20Hex(append([]byte(id.Pretty()), key...))
	return m.nopeCache.Has(cacheKey)
}

// IsGood implements ProviderTracker
func (m *BasicProviderTracker) IsGood(id peer.ID) bool {

	// Return false if peer is banned
	res := m.banned.Get(id.Pretty())
	if res != nil {
		return false
	}

	// Return true if peer is not registered
	info := m.Get(id, nil)
	if info == nil {
		return true
	}

	// Return false if time since the peer's last failure time is below the backoff duration.
	if time.Now().Sub(info.LastFailure).Seconds() < BackOffDurAfterFailure.Seconds() {
		return false
	}

	return true
}

// Ban implements ProviderTracker
func (m *BasicProviderTracker) Ban(peer peer.ID, dur time.Duration) {

	id := peer.Pretty()

	expTime := m.banned.Get(id)
	if expTime != nil {
		expTime := expTime.(*time.Time).Add(dur)
		m.banned.Add(id, &expTime, expTime)
		return
	}

	exp := time.Now().Add(dur)
	m.banned.Add(id, &exp, exp)
}

// MarkFailure implements ProviderTracker
func (m *BasicProviderTracker) MarkFailure(id peer.ID) {
	m.Get(id, func(info *ProviderInfo) {
		info.Failed++
		info.LastFailure = time.Now()
		if info.Failed >= MaxFailureBeforeBan {
			m.Ban(id, BanDueToFailureDur)
		}
	})
}

// MarkSeen implements ProviderTracker
func (m *BasicProviderTracker) MarkSeen(id peer.ID) {
	m.Get(id, func(info *ProviderInfo) {
		info.Failed = 0
		info.LastSeen = time.Now()
	})
}
