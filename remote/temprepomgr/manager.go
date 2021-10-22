// Package temprepomgr tracks temporary repositories created by other processes.
// It ensures that temporary repositories are deleted when not accessed
// over a specified period.
package temprepomgr

import (
	"os"
	"sync"
	"time"

	"github.com/satori/go.uuid"
)

var maxDuration = 15 * time.Minute
var timerDuration = 1 * time.Minute

type Entry struct {
	path      string
	touchedAt time.Time
}

// BasicTempRepoManager manages temporary repositories created on this machine.
type BasicTempRepoManager struct {
	lck     *sync.Mutex
	entries map[string]*Entry
}

// New creates an instance of BasicTempRepoManager
func New() *BasicTempRepoManager {
	m := &BasicTempRepoManager{lck: &sync.Mutex{}, entries: make(map[string]*Entry)}
	t := time.NewTicker(timerDuration)
	go func() {
		for range t.C {
			m.removeOld()
		}
	}()
	return m
}

// Add adds a path to a temporary repository and returns an identifier
func (m *BasicTempRepoManager) Add(path string) string {
	m.lck.Lock()
	defer m.lck.Unlock()
	ent, id := m.getByPath(path)
	if ent == nil {
		id = uuid.NewV4().String()
		m.entries[id] = &Entry{path: path, touchedAt: time.Now()}
		return id
	}
	ent.touchedAt = time.Now()
	m.entries[id] = ent
	return id
}

// getByPath finds and returns an Entry by path
// Note: not thread-safe
func (m *BasicTempRepoManager) getByPath(path string) (*Entry, string) {
	for id, entry := range m.entries {
		if entry.path == path {
			return entry, id
		}
	}
	return nil, ""
}

// GetPath finds and returns a path by an identifier
func (m *BasicTempRepoManager) GetPath(id string) string {
	m.lck.Lock()
	defer m.lck.Unlock()
	entry, ok := m.entries[id]
	if !ok {
		return ""
	}
	return entry.path
}

// Remove will delete an Entry by its identifier
func (m *BasicTempRepoManager) Remove(id string) error {
	m.lck.Lock()
	defer m.lck.Unlock()
	entry, ok := m.entries[id]
	if !ok {
		return nil
	}
	if err := os.RemoveAll(entry.path); err != nil {
		return err
	}
	delete(m.entries, id)
	return nil
}

// removeOld will remove any path that has not been touched recently
func (m *BasicTempRepoManager) removeOld() {
	for id, entry := range m.entries {
		if entry.touchedAt.Add(maxDuration).Before(time.Now()) {
			m.Remove(id)
		}
	}
}
