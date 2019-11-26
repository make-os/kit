package repo

import (
	"os"
	"path/filepath"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/makeos/mosdef/storage"
	"github.com/pkg/errors"
)

var dbCacheCleanerInt = 60 * time.Second

// DBEntry describes a db connection entry
type DBEntry struct {
	db       storage.Engine
	expireAt int64
}

// DBCache manages a collection of db instances belonging to many
// repositories. It closes the least recently used db when the
// capacity of the pool is reached
type DBCache struct {
	cache   *lru.Cache
	repoDir string
	ttl     time.Duration
}

// NewDBCache creates an instance of DBCache
// cap: The cache capacity
// repoDir: The path where repositories are stored
// ttl: The max duration a db connection can remain in the cache untouched
func NewDBCache(cap int, repoDir string, ttl time.Duration) (*DBCache, error) {
	cache, err := lru.New(cap)
	if err != nil {
		return nil, err
	}

	dbCache := &DBCache{
		cache:   cache,
		repoDir: repoDir,
		ttl:     ttl,
	}

	ticker := time.NewTicker(dbCacheCleanerInt)
	go func() {
		for range ticker.C {
			dbCache.cleanExpired()
		}
	}()

	return dbCache, nil
}

// Get returns a connection to a repositories db.
// If an existing connection for a repo exist, it returns it, otherwise it
// creates a new one, caches it and returns it.
// When an existing db connection is retrieved, its TTL is reset.
func (c *DBCache) Get(repoName string) (storage.Engine, error) {

	repoPath := filepath.Join(c.repoDir, repoName)
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return nil, ErrRepoNotFound
	}

	if c.cache.Contains(repoName) {
		item, _ := c.cache.Get(repoName)
		entry := item.(*DBEntry)
		entry.expireAt = time.Now().Add(c.ttl).Unix()
		return entry.db, nil
	}

	db := storage.NewBadger()
	if err := db.Init(filepath.Join(repoPath, "db")); err != nil {
		return nil, errors.Wrap(err, "failed to load repo db")
	}

	c.cache.Add(repoName, &DBEntry{
		db:       db,
		expireAt: time.Now().Add(c.ttl).Unix(),
	})

	return db, nil
}

func (c *DBCache) cleanExpired() {
	keys := c.cache.Keys()
	for _, k := range keys {
		entry, _ := c.cache.Peek(k)
		if entry != nil && time.Now().After(time.Unix(entry.(*DBEntry).expireAt, 0)) {
			entry.(*DBEntry).db.Close()
			c.cache.Remove(k)
		}
	}
}

// Clear closes all db connection and clears the cache
func (c *DBCache) Clear() {
	for _, k := range c.cache.Keys() {
		entry, _ := c.cache.Peek(k)
		entry.(*DBEntry).db.Close()
	}
	c.cache.Purge()
}
