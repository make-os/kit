package dht

import (
	"context"
	"fmt"
	"net"

	libp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/pkg/errors"
)

// DHT provides distributed hash table functionalities
// specifically required for storing and
type DHT struct {
	host host.Host
	dht  *dht.IpfsDHT
}

// New creates a new DHT client
func New(ctx context.Context, key crypto.PrivKey, addr string) (*DHT, error) {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, errors.Wrap(err, "invalid address")
	}

	// To construct a simple host with all the default settings, just use `New`
	lAddr := libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/%s/tcp/%s", host, port))
	h, err := libp2p.New(ctx, libp2p.Identity(key), lAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create host")
	}

	ipfsDht, err := dht.New(ctx, h)
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize dht")
	}

	return &DHT{host: h, dht: ipfsDht}, nil
}

// Close closes the host
func (dht *DHT) Close() error {
	if dht.host != nil {
		return dht.host.Close()
	}
	return nil
}
