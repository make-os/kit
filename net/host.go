package net

import (
	"context"
	"fmt"
	"net"

	"github.com/libp2p/go-libp2p"
	core "github.com/libp2p/go-libp2p-core"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/pkgs/logger"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
)

// Host describes an object participating in a p2p network.
type Host interface {
	Get() core.Host
	ID() peer.ID
	Addrs() []multiaddr.Multiaddr
	FullAddr() string
}

// BasicHost wraps core.Host for use by the DHT and PubSub
type BasicHost struct {
	host core.Host
	log  logger.Logger
}

// New creates a new host
func New(ctx context.Context, cfg *config.AppConfig) (*BasicHost, error) {

	address, port, err := net.SplitHostPort(cfg.DHT.Address)
	if err != nil {
		return nil, errors.Wrap(err, "invalid address")
	}

	lAddr := libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/%s/tcp/%s", address, port))
	key, _ := cfg.G().PrivVal.GetKey()
	h, err := libp2p.New(ctx, libp2p.Identity(key.UnwrappedPrivKey()), lAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create host")
	}

	bh := &BasicHost{
		host: h,
		log:  cfg.G().Log.Module("host"),
	}

	bh.log.Info("Host is running", "addr", bh.FullAddr())

	return bh, nil
}

// NewWithHost creates an instance of BasicHost with a pre-existing host
func NewWithHost(host core.Host) *BasicHost {
	return &BasicHost{host: host}
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

// FullAddr returns the full host address
func (h *BasicHost) FullAddr() string {
	return fmt.Sprintf("%s/p2p/%s", h.Addrs()[0].String(), h.ID().Pretty())
}
