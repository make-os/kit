package net

import (
	"context"
	"fmt"
	"net"

	"github.com/libp2p/go-libp2p"
	core "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/make-os/kit/config"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
)

// Host describes an object participating in a p2p network.
type Host interface {
	Get() core.Host
	ID() peer.ID
	Addrs() []multiaddr.Multiaddr
}

// BasicHost wraps core.Host for use by the DHT and PubSub
type BasicHost struct {
	host core.Host
}

// New creates a new host
func New(ctx context.Context, cfg *config.AppConfig) (*BasicHost, error) {
	key, _ := cfg.G().PrivVal.GetKey()

	address, port, err := net.SplitHostPort(cfg.DHT.Address)
	if err != nil {
		return nil, errors.Wrap(err, "invalid address")
	}

	lAddr := libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/%s/tcp/%s", address, port))
	h, err := libp2p.New(ctx, libp2p.Identity(key.PrivKey().Key()), lAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create host")
	}

	return &BasicHost{
		host: h,
	}, nil
}

// Get returns the host object
func (h *BasicHost) Get() core.Host {
	return h.host
}

// ID returns the peer ID of the host
func (h *BasicHost) ID() peer.ID {
	return h.host.ID()
}

// Addrs returns the addresses of the host
func (h *BasicHost) Addrs() []multiaddr.Multiaddr {
	return h.host.Addrs()
}
