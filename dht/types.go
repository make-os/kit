package dht

import (
	"context"
	"fmt"
	"strings"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"gitlab.com/makeos/mosdef/util"
)

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

	// Announce informs the network that it can provide value for the given key
	Announce(ctx context.Context, key []byte) error

	// GetObject returns an object from a provider
	GetObject(ctx context.Context, query *DHTObjectQuery) ([]byte, error)

	// RegisterObjFinder registers a finder for an object type
	RegisterObjFinder(objType string, finder ObjectFinder)

	// BasicCommitStreamer returns the commit exchanger
	CommitStreamer() CommitStreamer

	// Host returns the wrapped IPFS host
	Host() host.Host

	// Start starts the DHT
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

const (
	GitObjectNamespace = "obj"
)

// MakeGitObjectKey returns a key for announcing a git object in a given repository
func MakeGitObjectKey(repoName, hash string) string {
	return fmt.Sprintf("/%s/%s/%s", GitObjectNamespace, repoName, hash)
}

// ParseGitObjectKey parses a key for finding git objects in a given repository
func ParseGitObjectKey(key string) (repoName string, hash string, err error) {
	parts := strings.Split(key, "/")
	if len(parts) != 4 {
		return "", "", fmt.Errorf("invalid repo object key")
	}
	return parts[2], parts[3], nil
}
