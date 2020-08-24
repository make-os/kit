package providertracker

import (
	"sync"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/make-os/lobe/pkgs/cache"
)

var (
	// MaxFailureBeforeBan is the number of failures before a provider is banned
	MaxFailureBeforeBan = 3

	// BanDueToFailureDur is the duration of a ban due to failure
	BanDueToFailureDur = 1 * time.Hour

	// BackOffDurAfterFailure is the backoff time before a failed provider can be good again.
	BackOffDurAfterFailure = 15 * time.Minute
)

// ProviderTracker describes a structure for tracking provider performance
type ProviderTracker interface {
	Register(addrs ...peer.AddrInfo)
	NumProviders() int
	Get(id peer.ID, cb func(*ProviderInfo)) *ProviderInfo
	IsGood(id peer.ID) bool
	Ban(peer peer.ID, dur time.Duration)
	MarkFailure(id peer.ID)
	MarkSeen(id peer.ID)
}

// ProviderInfo contains information about a provider
type ProviderInfo struct {
	Addr        *peer.AddrInfo
	Failed      int
	LastFailure time.Time
	LastSeen    time.Time
}

// BasicProviderTracker providers historic and status information on providers.
type BasicProviderTracker struct {
	banned    *cache.Cache
	lck       *sync.Mutex
	providers map[string]*ProviderInfo
}

// NewProviderTracker creates an instance of BasicProviderTracker.
func NewProviderTracker() *BasicProviderTracker {
	return &BasicProviderTracker{
		lck:       &sync.Mutex{},
		banned:    cache.NewCacheWithExpiringEntry(100000),
		providers: make(map[string]*ProviderInfo),
	}
}

// Register registers a new provider so it can be tracked
func (m *BasicProviderTracker) Register(addrs ...peer.AddrInfo) {
	m.lck.Lock()
	defer m.lck.Unlock()
	for _, addr := range addrs {
		if _, ok := m.providers[addr.ID.Pretty()]; !ok {
			m.providers[addr.ID.Pretty()] = &ProviderInfo{Addr: &addr, LastSeen: time.Now()}
		}
	}
}

// NumProviders returns the number of registered providers.
func (m *BasicProviderTracker) NumProviders() int {
	return len(m.providers)
}

// BanCache returns the cache containing banned peers
func (m *BasicProviderTracker) BanCache() *cache.Cache {
	return m.banned
}

// Get a provider's information. If cb is provided, it is called with the provider
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

// IsGood checks whether the given peer has a good record.
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

// Ban bans a provider for the given duration.
// If a peer is currently banned, the duration is added to its current ban time.
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

// MarkFailure increments a provider's failure count.
// When failure count reaches a max, ban the provider.
func (m *BasicProviderTracker) MarkFailure(id peer.ID) {
	m.Get(id, func(info *ProviderInfo) {
		info.Failed++
		info.LastFailure = time.Now()
		if info.Failed >= MaxFailureBeforeBan {
			m.Ban(id, BanDueToFailureDur)
		}
	})
}

// MarkSeen marks the provider's last seen time and resets its failure count.
func (m *BasicProviderTracker) MarkSeen(id peer.ID) {
	m.Get(id, func(info *ProviderInfo) {
		info.Failed = 0
		info.LastSeen = time.Now()
	})
}
