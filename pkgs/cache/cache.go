package cache

import (
	"time"

	lru "github.com/hashicorp/golang-lru"
)

// DefaultRemovalInterval is the duration when expired cache
// items are checked and remove.
var DefaultRemovalInterval = 5 * time.Second

// Sec returns current time + sec
func Sec(sec int) time.Time {
	return time.Now().Add(time.Duration(sec) * time.Second)
}

type cacheValue struct {
	value interface{}
	expAt time.Time
}

// Cache is a thread-safe LRU cache.
type Cache struct {
	container *lru.Cache
	rmExpired bool
}

// NewCache creates a new instance of Cache
func NewCache(capacity int) *Cache {
	cache := new(Cache)
	cache.container, _ = lru.New(capacity)
	return cache
}

// NewCacheWithExpiringEntry creates a new Cache instance that removes expired entries
// periodically and during insertions. This cache is useful for when an item
// needs to be removed even before it is evicted by the LRU logic.
func NewCacheWithExpiringEntry(capacity int) *Cache {
	cache := NewCache(capacity)
	cache.rmExpired = true
	go func() {
		for {
			<-time.NewTicker(DefaultRemovalInterval).C
			cache.removeExpired()
		}
	}()
	return cache
}

// Register adds an item. It the cache becomes full, the oldest item is evicted
// to make room for the new item. If this cache supports removal of explicit
// expiring entries, they are first removed before addition of a new entry.
func (c *Cache) Add(key, val interface{}, expireAt ...time.Time) {
	var expAt time.Time
	if len(expireAt) > 0 {
		expAt = expireAt[0]
	}

	c.removeExpired()

	c.container.Add(key, &cacheValue{
		value: val,
		expAt: expAt,
	})
}

// Peek gets an item without updating the newness of the item
func (c *Cache) Peek(key interface{}) interface{} {
	v, _ := c.container.Peek(key)
	if v == nil {
		return nil
	}
	return v.(*cacheValue).value
}

// Get gets an item and updates the newness of the item
func (c *Cache) Get(key interface{}) interface{} {
	v, _ := c.container.Get(key)
	if v == nil {
		return nil
	}
	return v.(*cacheValue).value
}

// removeExpired removes expired items
func (c *Cache) removeExpired() {
	if !c.rmExpired {
		return
	}
	for _, k := range c.container.Keys() {

		cVal, ok := c.container.Peek(k)
		if !ok {
			continue
		}

		if cVal.(*cacheValue).expAt.IsZero() {
			continue
		}

		if time.Now().After(cVal.(*cacheValue).expAt) {
			c.container.Remove(k)
		}
	}
}

// Keys returns all keys in the cache
func (c *Cache) Keys() []interface{} {
	return c.container.Keys()
}

// Remove removes an item from the cache
func (c *Cache) Remove(key interface{}) {
	c.container.Remove(key)
}

// Has checks whether an item is in the cache without updating the newness of the item
func (c *Cache) Has(key interface{}) bool {
	return c.container.Contains(key)
}

// Len returns the length of the cache
func (c *Cache) Len() int {
	return c.container.Len()
}
