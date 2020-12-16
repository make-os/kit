package parent2p

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/net"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
)

const (
	ProtocolID = protocol.ID("parent2p/1.0")

	// MaxMsgSize is the maximum message size.
	MaxMsgSize = 1000000

	hand = "HAND"
	ackh = "ACKH"
)

type LightPeer struct {
	TrackList map[string]struct{}
}

// Parent2 describes a module for handling interactions between
// two nodes where one is a parent and the other is a child (e.g light node).
type Parent2P interface {
	ConnectToParent(ctx context.Context, parentAddr string) error
	SendHandshakeMsg(ctx context.Context, trackList []string) (string, error)
	Parent() peer.ID
	Peers() Peers
}

type Peers map[string]*LightPeer

// BasicParent2P implements Parent2P to provide a structure that
// allows a node to become a parent to another node or a child of
// another node. It specifically designed for parent to light client
// long-lived connection and interactions
type BasicParent2P struct {
	cfg    *config.AppConfig
	host   net.Host
	parent peer.ID
	peers  Peers
}

// New creates an instance of BasicParent2P
func New(cfg *config.AppConfig, h net.Host) *BasicParent2P {
	b := &BasicParent2P{host: h, peers: map[string]*LightPeer{}, cfg: cfg}
	h.Get().SetStreamHandler(ProtocolID, func(s network.Stream) {
		_ = b.Handler(s)
	})
	return b
}

// Parent returns the parent peer
func (b *BasicParent2P) Parent() peer.ID {
	return b.parent
}

// Peers returns the peers under this parent
func (b *BasicParent2P) Peers() Peers {
	return b.peers
}

// ConnectToParent creates a permanent connection to the parent
func (b *BasicParent2P) ConnectToParent(ctx context.Context, parentAddr string) error {

	maddr, err := multiaddr.NewMultiaddr(parentAddr)
	if err != nil {
		return errors.Wrap(err, "bad parent address")
	}

	info, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		return errors.Wrap(err, "cannot convert parent address to addrinfo")
	}

	if info.ID == b.host.ID() {
		return fmt.Errorf("cannot connect to self")
	}

	b.host.Get().Peerstore().AddAddrs(info.ID, info.Addrs, peerstore.PermanentAddrTTL)
	err = b.host.Get().Connect(ctx, *info)
	if err != nil {
		return errors.Wrap(err, "connect failed")
	}

	b.parent = info.ID

	return nil
}

// Handler takes a stream and begins processing it
func (b *BasicParent2P) Handler(s network.Stream) error {

	bz := make([]byte, MaxMsgSize)
	read, err := s.Read(bz)
	if err != nil {
		return errors.Wrap(err, "failed to read message")
	} else if read < 4 {
		return fmt.Errorf("bad message length")
	}

	bz = bytes.Trim(bz, "\x00")
	parts := bytes.Split(bz, []byte(" "))
	if len(parts) > 2 {
		return fmt.Errorf("bad message format")
	}

	switch string(parts[0]) {
	case hand:
		return b.HandleHandshake(string(parts[1]), s)
	default:
		return fmt.Errorf("unknown message type")
	}
}

// SendHandshake sends a HAND message
func (b *BasicParent2P) SendHandshakeMsg(ctx context.Context, trackList []string) (string, error) {
	s, err := b.host.Get().NewStream(ctx, b.parent, ProtocolID)
	if err != nil {
		return "", errors.Wrap(err, "failed to open stream")
	}
	defer s.Close()

	// Send handshake message
	_, err = s.Write(MakeHandshakeMsg(trackList))
	if err != nil {
		return "", errors.Wrap(err, "failed to send handshake")
	}

	// Read ack. message
	var bz = bytes.NewBuffer(nil)
	read, err := io.CopyN(bz, s, MaxMsgSize)
	if err != nil && err != io.EOF {
		return "", errors.Wrap(err, "failed to read handshake response message")
	} else if read < 4 {
		return "", fmt.Errorf("bad message length")
	}

	fields := strings.Split(bz.String(), " ")
	if len(fields) > 2 {
		return "", fmt.Errorf("bad message format")
	}

	switch fields[0] {
	case ackh:
		return fields[1], nil
	default:
		return "", fmt.Errorf("unknown message type")
	}
}

// HandleHandshake handles incoming handshake message.
func (b *BasicParent2P) HandleHandshake(msg string, s network.Stream) error {
	defer s.Close()

	p := &LightPeer{
		TrackList: map[string]struct{}{},
	}

	for _, val := range strings.Split(msg, ",") {
		if val != "" {
			p.TrackList[val] = struct{}{}
		}
	}

	b.peers[s.Conn().RemotePeer().String()] = p

	// Send an acknowledge message
	if err := b.SendAckHandshakeMsg(s); err != nil {
		return err
	}

	return nil
}

// SendAckHandshakeMsg sends an ACKH message to a peer
func (b *BasicParent2P) SendAckHandshakeMsg(s network.Stream) error {

	// Send ack. handshake message
	_, err := s.Write(MakeAckHandshakeMsg(b.cfg.RPC.TMRPCAddress))
	if err != nil {
		return errors.Wrap(err, "failed to send ack handshake")
	}

	return nil
}

// MakeHandshakeMsg creates a handshake message
func MakeHandshakeMsg(trackList []string) []byte {
	return []byte(fmt.Sprintf("%s %s", hand, strings.Join(trackList, ",")))
}

// MakeAckHandshakeMsg creates a handshake acknowledge message
func MakeAckHandshakeMsg(rpcAddr string) []byte {
	return []byte(fmt.Sprintf("%s %s", ackh, rpcAddr))
}
