package tree

import (
	"sync"

	tmdb "github.com/tendermint/tm-db"

	"github.com/tendermint/iavl"
)

// SafeTree is a wrapper around Tendermint's IAVL that
// provides thread-safe operations
type SafeTree struct {
	*sync.RWMutex
	state *iavl.MutableTree
}

// NewSafeTree creates an instance of SafeTree
func NewSafeTree(db tmdb.DB, cacheSize int) *SafeTree {
	return &SafeTree{
		RWMutex: &sync.RWMutex{},
		state:   iavl.NewMutableTree(db, cacheSize),
	}
}

// Raw returns the underlying tree
func (s *SafeTree) Raw() interface{} {
	return s.state
}

// Version returns the version of the tree.
func (s *SafeTree) Version() int64 {
	s.RLock()
	defer s.RUnlock()
	return s.state.Version()
}

// GetVersioned gets the value at the specified key and version.
func (s *SafeTree) GetVersioned(key []byte, version int64) (index int64, value []byte) {
	s.RLock()
	defer s.RUnlock()
	return s.state.GetVersioned(key, version)
}

// Get returns the index and value of the specified key if it exists, or nil
// and the next index, if it doesn't.
func (s *SafeTree) Get(key []byte) (index int64, value []byte) {
	s.RLock()
	defer s.RUnlock()
	return s.state.Get(key)
}

// Set sets a key in the working tree. Nil values are not supported.
// Returns true if the key existed and was updated.
func (s *SafeTree) Set(key, value []byte) bool {
	s.Lock()
	defer s.Unlock()
	return s.state.Set(key, value)
}

// Remove removes a key and returns 'true' if successful
func (s *SafeTree) Remove(key []byte) bool {
	s.Lock()
	defer s.Unlock()
	_, removed := s.state.Remove(key)
	return removed
}

// SaveVersion saves a new tree version to disk, based on the current state of
// the tree. Returns the hash and new version number.
func (s *SafeTree) SaveVersion() ([]byte, int64, error) {
	s.Lock()
	defer s.Unlock()
	return s.state.SaveVersion()
}

// Load the latest versioned tree from disk.
func (s *SafeTree) Load() (int64, error) {
	s.Lock()
	defer s.Unlock()
	return s.state.Load()
}

// WorkingHash returns the hash of the current working tree.
func (s *SafeTree) WorkingHash() []byte {
	s.RLock()
	defer s.RUnlock()
	return s.state.WorkingHash()
}

// Hash returns the hash of the latest saved version of the tree, as returned
// by SaveVersion. If no versions have been saved, Hash returns nil.
func (s *SafeTree) Hash() []byte {
	s.RLock()
	defer s.RUnlock()
	return s.state.Hash()
}

// Rollback rolls back the tree to the latest version, discarding unsaved changes.
func (s *SafeTree) Rollback() {
	s.Lock()
	defer s.Unlock()
	s.state.Rollback()
}
