package dht

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multihash"

	lru "github.com/hashicorp/golang-lru"
	ma "github.com/multiformats/go-multiaddr"

	libp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	dhtopts "github.com/libp2p/go-libp2p-kad-dht/opts"
	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
	"github.com/makeos/mosdef/util/logger"
	"github.com/pkg/errors"
	"github.com/tendermint/tendermint/p2p"
)

// DHTReactor is the channel id for the reactor
const DHTReactor = byte(0x31)

// DHT provides distributed hash table functionalities
// specifically required for storing and
type DHT struct {
	p2p.BaseReactor
	cfg            *config.AppConfig
	host           host.Host
	dht            *dht.IpfsDHT
	log            logger.Logger
	connectedPeers *lru.Cache
}

// New creates a new DHT service
func New(ctx context.Context, cfg *config.AppConfig, key crypto.PrivKey, addr string) (*DHT, error) {

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, errors.Wrap(err, "invalid address")
	}

	lAddr := libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/%s/tcp/%s", host, port))
	h, err := libp2p.New(ctx, libp2p.Identity(key), lAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create host")
	}

	// opts := &badger.DefaultOptions
	// ds, err := badger.NewDatastore(cfg.GetDHTStoreDir(), opts)
	// if err != nil {
	// 	return nil, err
	// }
	// dhtopts.Datastore(ds)

	ipfsDht, err := dht.New(ctx, h, dhtopts.Validator(validator{}))
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize dht")
	}

	log := cfg.G().Log.Module("dht")
	log.Info("DHT service has started", "Address", h.Addrs()[0].String())

	dht := &DHT{
		host: h,
		dht:  ipfsDht,
		cfg:  cfg,
		log:  log,
	}

	dht.BaseReactor = *p2p.NewBaseReactor("Reactor", dht)
	dht.connectedPeers, err = lru.New(100)

	return dht, err
}

// connect connects the host to a remote dht peer using the given DHT info
func (dht *DHT) connect(remotePeerDHTInfo *types.DHTInfo, p p2p.Peer) error {
	ctx, cn := context.WithTimeout(context.Background(), 60*time.Second)
	defer cn()

	var err error
	addrInfo := peer.AddrInfo{}
	addr, _ := ma.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%s", remotePeerDHTInfo.Address, remotePeerDHTInfo.Port))
	addrInfo.Addrs = append(addrInfo.Addrs, addr)
	addrInfo.ID, err = peer.IDB58Decode(remotePeerDHTInfo.ID)
	if err != nil {
		return errors.Wrap(err, "bad peer id")
	}

	if err := dht.host.Connect(ctx, addrInfo); err != nil {
		dht.log.Error("failed to connect to peer", "Err", err.Error())
	}

	// Add to the connected peers index the tendermint ID of the remote peer
	// corresponding to it DHT host ID
	dht.connectedPeers.Add(p.ID(), addrInfo.ID)

	// Update the dht routing table
	dht.dht.Update(ctx, addrInfo.ID)

	dht.log.Info("Connected to peer", "PeerID", addrInfo.ID)

	return nil
}

// AddPeer is called when a peer connects to the switch
func (dht *DHT) AddPeer(peer p2p.Peer) {
	dht.requestDHTInfo(peer)
}

// RemovePeer is called when a peer disconnects from the switch.
// We respond by checking if the peer is dht node we are connected to and then
// disconnect from it
func (dht *DHT) RemovePeer(tmPeer p2p.Peer, reason interface{}) {

	dhtPeerID, ok := dht.connectedPeers.Peek(tmPeer.ID())
	if !ok {
		return
	}

	// Close the connections with the dht node
	for _, con := range dht.host.Network().ConnsToPeer(dhtPeerID.(peer.ID)) {
		con.Close()
	}

	dht.verifyConnections(context.Background())
}

// isConnected checks whether a peer is connected to the host
func (dht *DHT) isConnected(p p2p.Peer) bool {
	return dht.connectedPeers.Contains(p.ID())
}

// verifyConnections verifies the connectedness of peers in the connected cache
// and remove peers that are no longer connected to the host.
func (dht *DHT) verifyConnections(ctx context.Context) {
	for _, peerID := range dht.connectedPeers.Keys() {
		remoteDHTHostID, _ := dht.connectedPeers.Peek(peerID)
		found := false
		for _, con := range dht.host.Network().Conns() {
			if con.RemotePeer() == remoteDHTHostID.(peer.ID) {
				found = true
				break
			}
		}
		if !found {
			dht.connectedPeers.Remove(peerID)
			dht.dht.Update(ctx, remoteDHTHostID.(peer.ID))
		}
	}
}

// OnStart implements p2p.BaseReactor.
func (dht *DHT) OnStart() error {
	return nil
}

func (dht *DHT) requestDHTInfo(p p2p.Peer) error {

	// Do nothing if peer is already connected
	if dht.isConnected(p) {
		return nil
	}

	if p.Send(DHTReactor, types.BareDHTInfo().Bytes()) {
		dht.log.Debug("Requested DHTInfo information from peer", "PeerID", p.ID())
	}

	return nil
}

// GetChannels implements Reactor.
func (dht *DHT) GetChannels() []*p2p.ChannelDescriptor {
	return []*p2p.ChannelDescriptor{
		{ID: DHTReactor, Priority: 5},
	}
}

// Receive implements Reactor
func (dht *DHT) Receive(chID byte, peer p2p.Peer, msgBytes []byte) {

	var dhtInfo types.DHTInfo
	if err := util.BytesToObject(msgBytes, &dhtInfo); err != nil {
		dht.log.Error("failed to decoded message to types.DHTInfo")
		return
	}

	// If info is empty, then this a request for our own DHT info
	if dhtInfo.IsEmpty() {
		host, port, err := net.SplitHostPort(dht.cfg.DHT.Address)
		if err != nil {
			dht.log.Error("failed parse dht address")
			return
		}

		info := types.DHTInfo{
			ID:      dht.host.ID().String(),
			Address: host,
			Port:    port,
		}
		if peer.Send(DHTReactor, info.Bytes()) {
			dht.log.Debug("Sent DHT information to peer", "PeerID", peer.ID())
		}
		return
	}

	// At this point, a peer has sent us their info, so we should connect to it
	dht.connect(&dhtInfo, peer)
}

// Store adds a value corresponding to the given key
func (dht *DHT) Store(ctx context.Context, key string, value []byte) error {
	return dht.dht.PutValue(ctx, key, value)
}

// Lookup searches for a value corresponding to the given key
func (dht *DHT) Lookup(ctx context.Context, key string) ([]byte, error) {
	return dht.dht.GetValue(ctx, key)
}

// GetProviders finds peers capable of providing value for the given key
func (dht *DHT) GetProviders(ctx context.Context, key []byte) ([]peer.AddrInfo, error) {
	cid, err := cid.NewPrefixV1(cid.Raw, multihash.BLAKE2B_MAX).Sum(key)
	if err != nil {
		return nil, err
	}
	peers, err := dht.dht.FindProviders(ctx, cid)
	if err != nil {
		return nil, err
	}
	return peers, nil
}

// Annonce informs the network that it can provide value for the given key
func (dht *DHT) Annonce(ctx context.Context, key []byte) error {
	cid, err := cid.NewPrefixV1(cid.Raw, multihash.BLAKE2B_MAX).Sum(key)
	if err != nil {
		return err
	}
	return dht.dht.Provide(ctx, cid, true)
}

// Close closes the host
func (dht *DHT) Close() error {
	if dht.host != nil {
		return dht.host.Close()
	}
	return nil
}
