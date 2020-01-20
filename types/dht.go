package types

import (
	"context"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/makeos/mosdef/util"
)

// DHTInfo represents information about a node's DHT service

// ObjectFinder describes an interface for finding objects
type ObjectFinder interface {
	// FindObject finds objects corresponding to the given key.
	FindObject(key []byte) ([]byte, error)
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

	// GetObject returns an object from a provider
	GetObject(ctx context.Context, query *DHTObjectQuery) ([]byte, error)

	// RegisterObjFinder registers a finder for an object type
	RegisterObjFinder(objType string, finder ObjectFinder)

	// Start starts the DHT
	Start() error

	// Peers returns a list of all peers
	Peers() (peers []string)
}

// DHTObjectQuery describes a query for an object that is maintained by a module
type DHTObjectQuery struct {
	Module    string // The module that will handle the query
	ObjectKey []byte // The object key
}

// Bytes serializes the object
func (q *DHTObjectQuery) Bytes() []byte {
	return util.ObjectToBytes(q)
}

// DHTObjectResponse represents a response containing an object
type DHTObjectResponse struct {
	Data []byte
}
