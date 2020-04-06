package types

import (
	"context"

	"github.com/libp2p/go-libp2p-core/mux"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"gitlab.com/makeos/mosdef/util"
)

// ObjectFinder describes an interface for finding objects
type ObjectFinder interface {
	// FindObject finds objects corresponding to the given key.
	FindObject(key []byte) ([]byte, error)
}

// DHTNode represents a distributed hash table
type DHTNode interface {

	// Store adds a value corresponding to the given key
	Store(ctx context.Context, key string, value []byte) error

	// Lookup searches for a value corresponding to the given key
	Lookup(ctx context.Context, key string) ([]byte, error)

	// GetProviders finds peers capable of providing value for the given key
	GetProviders(ctx context.Context, key []byte) ([]peer.AddrInfo, error)

	// Announce informs the network that it can provide value for the given key
	Announce(ctx context.Context, key []byte) error

	// GetObject returns an object from a provider
	GetObject(ctx context.Context, query *DHTObjectQuery) ([]byte, error)

	// RegisterObjFinder registers a finder for an object type
	RegisterObjFinder(objType string, finder ObjectFinder)

	// Start starts the DHTNode
	Start() error

	// Peers returns a list of all peers
	Peers() (peers []string)

	// Close closes the host
	Close() error
}

// DHTObjectQuery describes a query for an object that is maintained by a module
type DHTObjectQuery struct {
	Module    string // The module that will handle the query
	ObjectKey []byte // The object key
}

// Bytes serializes the object
func (q *DHTObjectQuery) Bytes() []byte {
	return util.ToBytes(q)
}

// DHTObjectResponse represents a response containing an object
type DHTObjectResponse struct {
	Data []byte
}

// Stream represents a bidirectional channel between two agents in
// a libp2p network. "agent" is as granular as desired, potentially
// being a "request -> reply" pair, or whole protocols.
//
// Streams are backed by a multiplexer underneath the hood.
type Stream interface {
	mux.MuxedStream

	Protocol() protocol.ID
	SetProtocol(id protocol.ID)

	// Stat returns metadata pertaining to this stream.
	Stat() network.Stat

	// Conn returns the connection this stream is part of.
	Conn() network.Conn
}
