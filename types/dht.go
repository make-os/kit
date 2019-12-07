package types

import (
	"context"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/makeos/mosdef/util"
)

// DHTInfo represents information about a node's DHT service
type DHTInfo struct {
	ID      string
	Address string
	Port    string
}

// BareDHTInfo returns an empty DHTInfo instance
func BareDHTInfo() *DHTInfo {
	return &DHTInfo{}
}

// Bytes returns the serialized version
func (o *DHTInfo) Bytes() []byte {
	return util.ObjectToBytes(o)
}

// IsEmpty checks if the o is empty
func (o *DHTInfo) IsEmpty() bool {
	return o.Address == "" && o.Port == "" && o.ID == ""
}

// DHT represents a distributed hash table
type DHT interface {

	// Store adds a value corresponding to the given key
	Store(ctx context.Context, key string, value []byte) error

	// Lookup searches for a value corresponding to the given key
	Lookup(ctx context.Context, key string) ([]byte, error)

	// GetProviders finds peers capable of providing value for the given key
	GetProviders(ctx context.Context, key []byte) ([]peer.AddrInfo, error)

	// Annonce informs the network that it can provide value for the given key
	Annonce(ctx context.Context, key []byte) error
}
