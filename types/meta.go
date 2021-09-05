package types

import (
	"sync"
)

const TxMetaKeyAllowNonceGap = "allowNonceGap"

// BasicMeta stores arbitrary, self-contained state information for a transaction
type BasicMeta struct {
	lck  *sync.RWMutex
	meta map[string]interface{}
}

// NewMeta creates an instance of BasicMeta
func NewMeta() *BasicMeta {
	return &BasicMeta{lck: &sync.RWMutex{}, meta: map[string]interface{}{}}
}

// HasMetaKey returns true if the given key exist in the meta map
func (m *BasicMeta) HasMetaKey(key string) bool {
	if m == nil {
		return false
	}
	m.lck.RLock()
	defer m.lck.RUnlock()
	return m.meta[key] != nil
}

// GetMeta returns the cloned meta information
func (m *BasicMeta) GetMeta() map[string]interface{} {
	m.lck.RLock()
	defer m.lck.RUnlock()
	var clone = make(map[string]interface{})
	for k, v := range m.meta {
		clone[k] = v
	}
	return clone
}

// ConcatMeta will concatenate a map into m
func (m *BasicMeta) Join(d map[string]interface{}) {
	m.lck.Lock()
	defer m.lck.Unlock()
	for k, v := range d {
		m.meta[k] = v
	}
}

// LoadMeta loads d as the m
func (m *BasicMeta) LoadMeta(d map[string]interface{}) {
	m.lck.Lock()
	m.meta = d
	m.lck.Unlock()
}

// SetMeta set key and value
func (m *BasicMeta) SetMeta(key string, val interface{}) {
	m.lck.Lock()
	m.meta[key] = val
	m.lck.Unlock()
}

type Meta interface {
	HasMetaKey(key string) bool
	GetMeta() map[string]interface{}
	Join(d map[string]interface{})
	LoadMeta(d map[string]interface{})
	SetMeta(key string, val interface{})
}
