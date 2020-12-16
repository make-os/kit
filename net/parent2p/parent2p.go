package parent2p

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/net"
	"github.com/make-os/kit/pkgs/logger"
	"github.com/make-os/kit/util"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
	"github.com/thoas/go-funk"
)

var (
	ErrUnknownPeer = fmt.Errorf("unknown peer")
)

const (
	ProtocolID = protocol.ID("parent2p/1.0")

	// MaxMsgSize is the maximum message size.
	MaxMsgSize = 1000000

	hand = "HAND"
	ackh = "ACKH"
	uptl = "UPTL"
	rejt = "REJT"
	okay = "OKAY"
)

type ChildPeer struct {
	TrackList map[string]struct{}
}

// Parent2P describes a module for handling interactions between
// two nodes where one is a parent and the other is a child (e.g light node).
type Parent2P interface {
	ConnectToParent(ctx context.Context, parentAddr string) error
	SendHandshakeMsg(ctx context.Context, trackList []string) (string, error)
	SendUpdateTrackListMsg(ctx context.Context, trackListAdd, trackListRemove []string) error
	Parent() peer.ID
	Peers() Peers
}

type Peers map[string]*ChildPeer

// BasicParent2P implements Parent2P to provide a structure that
// allows a node to become a parent to another node or a child of
// another node. It specifically designed for parent to light client
// long-lived connection and interactions
type BasicParent2P struct {
	log    logger.Logger
	cfg    *config.AppConfig
	host   net.Host
	parent peer.ID
	peers  Peers
}

// New creates an instance of BasicParent2P
func New(cfg *config.AppConfig, h net.Host) *BasicParent2P {
	b := &BasicParent2P{host: h, peers: map[string]*ChildPeer{}, cfg: cfg, log: cfg.G().Log.Module("parent2p")}
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

	b.log.Info("Connecting to parent", "Addr", parentAddr)

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
	_, err := s.Read(bz)
	if err != nil {
		return errors.Wrap(err, "failed to read message")
	}

	var body []interface{}
	if err := util.ToObject(bz, &body); err != nil {
		return fmt.Errorf("failed to decode message")
	}

	if len(body) != 2 {
		return fmt.Errorf("bad message format")
	}

	switch cast.ToString(body[0]) {
	case hand:
		return b.HandleHandshakeMsg(cast.ToStringSlice(body[1]), s)
	case uptl:
		return b.HandleUpdateTrackListMsg(cast.ToStringSlice(body[1]), s)
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
	_, err = io.CopyN(bz, s, MaxMsgSize)
	if err != nil && err != io.EOF {
		return "", errors.Wrap(err, "failed to read handshake response message")
	}

	var body []interface{}
	if err := util.ToObject(bz.Bytes(), &body); err != nil {
		return "", fmt.Errorf("failed to decode message")
	}

	if len(body) != 2 {
		return "", fmt.Errorf("bad message format")
	}

	switch cast.ToString(body[0]) {
	case ackh:
		return cast.ToString(body[1]), nil
	default:
		return "", fmt.Errorf("unknown message type")
	}
}

// HandleHandshakeMsg handles incoming handshake message.
func (b *BasicParent2P) HandleHandshakeMsg(trackList []string, s network.Stream) error {
	defer s.Close()

	p := &ChildPeer{
		TrackList: map[string]struct{}{},
	}

	for _, val := range trackList {
		if val != "" {
			p.TrackList[strings.ToLower(val)] = struct{}{}
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

// SendUpdateTrackListMsg sends a message to update a track list
func (b *BasicParent2P) SendUpdateTrackListMsg(
	ctx context.Context,
	trackListAdd,
	trackListRemove []string,
) error {

	// Create a new list by merging the add and remove list.
	// Entries in the remove list are prefixed with `-` to indicate removal.
	newList := funk.UniqString(trackListAdd)
	trackListRemove = funk.UniqString(trackListRemove)
	for _, val := range trackListRemove {
		newList = append(newList, fmt.Sprintf("-%s", val))
	}

	s, err := b.host.Get().NewStream(ctx, b.parent, ProtocolID)
	if err != nil {
		return errors.Wrap(err, "failed to open stream")
	}
	defer s.Close()

	// Send message
	_, err = s.Write(MakeTrackListUpdateMsg(newList))
	if err != nil {
		return errors.Wrap(err, "failed to write message")
	}

	// Read response message
	var bz = bytes.NewBuffer(nil)
	_, err = io.CopyN(bz, s, MaxMsgSize)
	if err != nil && err != io.EOF {
		return errors.Wrap(err, "failed to read response message")
	}

	var body []interface{}
	if err := util.ToObject(bz.Bytes(), &body); err != nil {
		return fmt.Errorf("failed to decode response message")
	}

	switch cast.ToString(body[0]) {
	case okay:
		return nil
	case rejt:
		return fmt.Errorf("message was rejected: %s", body[1])
	default:
		return fmt.Errorf("unknown message type")
	}
}

// HandleUpdateTrackListMsg handles incoming handshake message.
func (b *BasicParent2P) HandleUpdateTrackListMsg(trackList []string, s network.Stream) error {
	defer s.Close()

	// sort in descending order such that negations are last on the list.
	sort.Slice(trackList, func(i, j int) bool {
		return trackList[i] > trackList[j]
	})

	childPeer, ok := b.peers[s.Conn().RemotePeer().String()]
	if !ok {
		if _, err := s.Write(MakeRejectMsg(ErrUnknownPeer.Error())); err != nil {
			return errors.Wrap(err, "failed to write message")
		}
		return nil
	}

	for _, val := range trackList {
		if val == "" {
			continue
		}

		// Remove negated entries from the peer's tracklist
		if string(val[0]) == "-" {
			delete(childPeer.TrackList, strings.ToLower(val[1:]))
			continue
		}

		childPeer.TrackList[val] = struct{}{}
	}

	if _, err := s.Write(MakeOkayMsg()); err != nil {
		return errors.Wrap(err, "failed to write OKAY message")
	}

	return nil
}

// MakeRejectMsg creates a message indicating a rejection of an action
func MakeRejectMsg(msg string) []byte {
	return util.ToBytes([]interface{}{
		rejt, msg,
	})
}

// MakeOkayMsg creates a message indicating success of an action
func MakeOkayMsg() []byte {
	return util.ToBytes([]interface{}{
		okay,
	})
}

// MakeHandshakeMsg creates a handshake message
func MakeHandshakeMsg(trackList []string) []byte {
	return util.ToBytes([]interface{}{
		hand, trackList,
	})
}

// MakeAckHandshakeMsg creates a handshake acknowledge message
func MakeAckHandshakeMsg(rpcAddr string) []byte {
	return util.ToBytes([]interface{}{
		ackh, rpcAddr,
	})
}

// MakeTrackListUpdateMsg creates a message to update peer's
// tracklist on the the parent node
func MakeTrackListUpdateMsg(trackList []string) []byte {
	return util.ToBytes([]interface{}{
		uptl, trackList,
	})
}
