package providertracker

import (
	"sync"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/make-os/kit/dht"
	"github.com/make-os/kit/pkgs/cache"
	"github.com/make-os/kit/util/crypto"
)

var (
	// MaxFailureBeforeBan is the number of failures before a provider is banned
	MaxFailureBeforeBan = 30

	// BanDueToFailureDur is the duration of a ban due to failure
	BanDueToFailureDur = 15 * time.Minute

	// BackOffDurAfterFailure is the backoff time before a failed provider can be tried again.
	BackOffDurAfterFailure = 1 * time.Minute
)

// ProviderTracker is used to track status and behaviour of providers.
type ProviderTracker struct {
	// banned contains a collection of banned peers.
	// The key is the cache peer ID.
	banned *cache.Cache

	// nopeCache contains a collection of peers that responded
	// with NOPE for a given key. The cache key must be hash(peerID:key).
	nopeCache *cache.Cache

	lck       *sync.Mutex
	providers map[string]*dht.ProviderInfo
}

// New creates an instance of dht.ProviderTracker.
func New() *ProviderTracker {
	return &ProviderTracker{
		lck:       &sync.Mutex{},
		banned:    cache.NewCacheWithExpiringEntry(100000),
		nopeCache: cache.NewCacheWithExpiringEntry(100000),
		providers: make(map[string]*dht.ProviderInfo),
	}
}

// Register implements ProviderTracker
func (m *ProviderTracker) Register(addrs ...peer.AddrInfo) {
	m.lck.Lock()
	defer m.lck.Unlock()
	for _, addr := range addrs {
		if _, ok := m.providers[addr.ID.Pretty()]; !ok {
			m.providers[addr.ID.Pretty()] = &dht.ProviderInfo{Addr: &addr, LastSeen: time.Now()}
		}
	}
}

// NumProviders implements ProviderTracker
func (m *ProviderTracker) NumProviders() int {
	return len(m.providers)
}

// BanCache returns the cache containing banned peers
func (m *ProviderTracker) BanCache() *cache.Cache {
	return m.banned
}

// Get implements ProviderTracker
func (m *ProviderTracker) Get(id peer.ID, cb func(*dht.ProviderInfo)) *dht.ProviderInfo {
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
func (m *ProviderTracker) PeerSentNope(id peer.ID, key []byte) {
	cacheKey := crypto.Hash20Hex(append([]byte(id.Pretty()), key...))
	m.nopeCache.Add(cacheKey, struct{}{}, time.Now().Add(10*time.Minute))
}

// DidPeerSendNope implements ProviderTracker
func (m *ProviderTracker) DidPeerSendNope(id peer.ID, key []byte) bool {
	cacheKey := crypto.Hash20Hex(append([]byte(id.Pretty()), key...))
	return m.nopeCache.Has(cacheKey)
}

// IsGood implements ProviderTracker
func (m *ProviderTracker) IsGood(id peer.ID) bool {

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
func (m *ProviderTracker) Ban(peer peer.ID, dur time.Duration) {

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
func (m *ProviderTracker) MarkFailure(id peer.ID) {
	m.Get(id, func(info *dht.ProviderInfo) {
		info.Failed++
		info.LastFailure = time.Now()
		if info.Failed >= MaxFailureBeforeBan {
			m.Ban(id, BanDueToFailureDur)
		}
	})
}

// MarkSeen implements ProviderTracker
func (m *ProviderTracker) MarkSeen(id peer.ID) {
	m.Get(id, func(info *dht.ProviderInfo) {
		info.Failed = 0
		info.LastSeen = time.Now()
	})
}
