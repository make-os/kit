package types

import (
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peerstore"
)

type Host interface {
	host.Host
}

type Peerstore interface {
	peerstore.Peerstore
}

type Stream interface {
	network.Stream
}

type Conn interface {
	network.Conn
}
