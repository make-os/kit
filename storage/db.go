package storage

import (
	"sync"

	tmdb "github.com/tendermint/tm-db"
)

// NewBadgerTMDB creates a tendermint database.
// If dir is unset, an in-memory tendermint DB is returned
func NewBadgerTMDB(dir string) (tmdb.DB, error) {
	if dir == "" {
		return tmdb.NewDB("", tmdb.MemDBBackend, "")
	}
	return tmdb.NewDB("", tmdb.BadgerDBBackend, dir)
}

// NewBadger creates an instance of BadgerStore.
func NewBadger(dir string) (*BadgerStore, error) {
	s := &BadgerStore{lck: &sync.Mutex{}}
	return s, s.init(dir)
}
