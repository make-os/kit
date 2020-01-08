package dht

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"time"

	"github.com/ipfs/go-cid"
	badger "github.com/ipfs/go-ds-badger"
	"github.com/multiformats/go-multihash"

	lru "github.com/hashicorp/golang-lru"
	ma "github.com/multiformats/go-multiaddr"

	libp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	dhtopts "github.com/libp2p/go-libp2p-kad-dht/opts"
	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
	"github.com/makeos/mosdef/util/logger"
	"github.com/pkg/errors"
	"github.com/tendermint/tendermint/p2p"
)

// Errors
var (
	ErrObjNotFound        = fmt.Errorf("object not found")
	ErrInvalidObjIDFormat = fmt.Errorf("invalid object id format")
)

// DHTReactorChannel is the channel id for the reactor
const DHTReactorChannel = byte(0x31)

// DHT provides distributed hash table functionalities
// specifically required for storing and
type DHT struct {
	p2p.BaseReactor
	cfg            *config.AppConfig
	host           host.Host
	dht            *dht.IpfsDHT
	log            logger.Logger
	connectedPeers *lru.Cache
	objectFinders  map[string]types.ObjectFinder
	receiveErr     error // holds Receive() error (for testing)
}

// New creates a new DHT service
func New(
	ctx context.Context,
	cfg *config.AppConfig,
	key crypto.PrivKey,
	addr string) (*DHT, error) {

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, errors.Wrap(err, "invalid address")
	}

	lAddr := libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/%s/tcp/%s", host, port))
	h, err := libp2p.New(ctx, libp2p.Identity(key), lAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create host")
	}

	opts := &badger.DefaultOptions
	ds, err := badger.NewDatastore(cfg.GetDHTStoreDir(), opts)
	if err != nil {
		return nil, err
	}

	ipfsDht, err := dht.New(ctx, h,
		dhtopts.Validator(validator{}),
		dhtopts.Datastore(ds))
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize dht")
	}

	log := cfg.G().Log.Module("dht")
	log.Info("DHT service has started", "Address", h.Addrs()[0].String())

	dht := &DHT{
		host:          h,
		dht:           ipfsDht,
		cfg:           cfg,
		log:           log,
		objectFinders: make(map[string]types.ObjectFinder),
	}

	h.SetStreamHandler("/fetch/1", dht.handleFetch)

	dht.BaseReactor = *p2p.NewBaseReactor("Reactor", dht)
	dht.connectedPeers, err = lru.New(100)

	return dht, err
}

// RegisterObjFinder registers a finder to handle module-targetted find operations
func (dht *DHT) RegisterObjFinder(module string, finder types.ObjectFinder) {
	dht.objectFinders[module] = finder
}

// connect connects the host to a remote dht peer using the given DHT info
func (dht *DHT) connect(peerInfo *types.DHTInfo, p p2p.Peer) error {
	ctx, cn := context.WithTimeout(context.Background(), 60*time.Second)
	defer cn()

	var err error
	addrInfo := peer.AddrInfo{}
	addr, _ := ma.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%s", peerInfo.Address, peerInfo.Port))
	addrInfo.Addrs = append(addrInfo.Addrs, addr)
	addrInfo.ID, err = peer.IDB58Decode(peerInfo.ID)
	if err != nil {
		return errors.Wrap(err, "bad peer id")
	}

	if err := dht.host.Connect(ctx, addrInfo); err != nil {
		dht.log.Error("failed to connect to peer", "Err", err.Error())
		return errors.Wrap(err, "failed to connect")
	}

	// Add to the connected peers index the tendermint ID of the remote peer
	// corresponding to it DHT host ID
	dht.connectedPeers.Add(p.ID(), addrInfo.ID)

	// Update the dht routing table
	dht.dht.Update(ctx, addrInfo.ID)

	dht.log.Info("Connected to peer", "PeerID", addrInfo.ID)

	return nil
}

// AddPeer implements Reactor. It is called when a new peer connects to the
// switch. We respond by requesting the peer to send its dht info
func (dht *DHT) AddPeer(peer p2p.Peer) {
	dht.requestDHTInfo(peer)
}

// RemovePeer implements Reactor. It is called when a peer disconnects from the
// switch. We respond by checking if the peer is dht node we are connected to
// and then disconnect from it
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
		remoteDHTHostID, ok := dht.connectedPeers.Peek(peerID)
		if !ok {
			continue
		}
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

// Peers returns a list of all peers
func (dht *DHT) Peers() (peers []string) {
	for _, p := range dht.dht.RoutingTable().ListPeers() {
		peers = append(peers, p.String())
	}
	return
}

func (dht *DHT) requestDHTInfo(p p2p.Peer) error {

	// Do nothing if peer is already connected
	if dht.isConnected(p) {
		return nil
	}

	if p.Send(DHTReactorChannel, types.BareDHTInfo().Bytes()) {
		dht.log.Debug("Requested DHTInfo information from peer", "PeerID", p.ID())
		return nil
	}

	return fmt.Errorf("failed to send request")
}

// GetChannels implements Reactor.
func (dht *DHT) GetChannels() []*p2p.ChannelDescriptor {
	return []*p2p.ChannelDescriptor{
		{ID: DHTReactorChannel, Priority: 5},
	}
}

// Receive implements Reactor
func (dht *DHT) Receive(chID byte, peer p2p.Peer, msgBytes []byte) {

	var dhtInfo types.DHTInfo
	if err := util.BytesToObject(msgBytes, &dhtInfo); err != nil {
		dht.receiveErr = errors.Wrap(err, "failed to decode message")
		dht.log.Error("failed to decoded message to types.DHTInfo")
		return
	}

	// If info is empty, then this a request for our own DHT info
	if dhtInfo.IsEmpty() {
		host, port, err := net.SplitHostPort(dht.cfg.DHT.Address)
		if err != nil {
			dht.receiveErr = errors.Wrap(err, "failed to parse local peer dht address")
			dht.log.Error("failed parse dht address")
			return
		}

		info := types.DHTInfo{
			ID:      dht.host.ID().String(),
			Address: host,
			Port:    port,
		}
		if peer.Send(DHTReactorChannel, info.Bytes()) {
			dht.log.Debug("Sent DHT information to peer", "PeerID", peer.ID())
		}

		dht.receiveErr = fmt.Errorf("failed to send DHT Info")
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

	// For providers whose address are not included, find their address(es) from the
	// peer store and attach it to them.
	// Note: We are doing this here because the DHT logic does not add them when
	// it should have. (remove once fixed in go-libp2p-kad-dht)
	for i, prov := range peers {
		if len(prov.Addrs) == 0 {
			pi := dht.host.Peerstore().PeerInfo(prov.ID)
			prov.Addrs = pi.Addrs
			peers[i] = prov
		}
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

// fetchValue requests sends a query to a peer
func (dht *DHT) fetchValue(
	ctx context.Context,
	query *types.DHTObjectQuery,
	peer peer.AddrInfo) ([]byte, error) {

	if len(peer.Addrs) == 0 {
		return nil, fmt.Errorf("no known provider")
	}

	dht.host.Peerstore().AddAddr(peer.ID, peer.Addrs[0], peerstore.TempAddrTTL)
	str, err := dht.host.NewStream(ctx, peer.ID, "/fetch/1")
	if err != nil {
		return nil, err
	}
	defer str.Close()

	_, err = str.Write(query.Bytes())
	if err != nil {
		return nil, err
	}

	val, err := ioutil.ReadAll(str)
	if err != nil {
		return nil, err
	}

	return val, nil
}

// GetObject returns an object from a provider
func (dht *DHT) GetObject(
	ctx context.Context,
	query *types.DHTObjectQuery) (obj []byte, err error) {

	addrs, err := dht.GetProviders(ctx, query.ObjectKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get providers")
	}

	err = ErrObjNotFound
	for i := 0; i < len(addrs); i++ {
		addr := addrs[i]

		// If this address matches the local host, we should get the object locally
		if addr.ID.String() == dht.host.ID().String() {
			finder := dht.objectFinders[query.Module]
			if finder == nil {
				return nil, fmt.Errorf("finder for module `%s` not registered", query.Module)
			}
			obj, err = finder.FindObject(query.ObjectKey)
			if err != nil {
				err = errors.Wrap(err, "finder error")
				continue
			} else if obj == nil {
				err = ErrObjNotFound
				continue
			}
			return
		}

		obj, err = dht.fetchValue(ctx, query, addr)
		if err != nil {
			continue
		}

		return
	}

	return nil, err
}

// handleFetch processes incoming fetch requests
func (dht *DHT) handleFetch(s network.Stream) {
	defer s.Close()

	bz := make([]byte, 256)
	if _, err := s.Read(bz); err != nil {
		dht.log.Error("failed to read query", "Err", err.Error())
		return
	}

	var query types.DHTObjectQuery
	if err := util.BytesToObject(bz, &query); err != nil {
		dht.log.Error("failed to decode query", "Err", err.Error())
		return
	}

	finder := dht.objectFinders[query.Module]
	if finder == nil {
		msg := fmt.Sprintf("finder for module `%s` not registered", query.Module)
		dht.log.Error("failed to process query", "Err", msg)
		return
	}

	objBz, err := finder.FindObject(query.ObjectKey)
	if err != nil {
		dht.log.Error("failed finder execution", "Err", err.Error())
		return
	}

	if _, err := s.Write(objBz); err != nil {
		dht.log.Error("failed to write back find result", "Err", err.Error())
		return
	}
}
