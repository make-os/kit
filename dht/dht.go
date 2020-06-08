package dht

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"strings"
	"time"

	"github.com/ipfs/go-cid"
	badger "github.com/ipfs/go-ds-badger"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	record "github.com/libp2p/go-libp2p-record"
	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multihash"
	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/pkgs/logger"
	"gitlab.com/makeos/mosdef/util"
)

var (
	ProtocolPrefix = dht.ProtocolPrefix("/makeos")
	ErrObjNotFound = fmt.Errorf("object not found")
)

// Server provides distributed hash table functionalities.
type Server struct {
	cfg            *config.AppConfig
	host           host.Host
	dht            *dht.IpfsDHT
	log            logger.Logger
	objectFinders  map[string]ObjectFinder
	connTicker     *time.Ticker
	commitStreamer CommitStreamer
}

// New creates a new Server
func New(
	ctx context.Context,
	cfg *config.AppConfig,
	key crypto.PrivKey,
	addr string) (*Server, error) {

	address, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, errors.Wrap(err, "invalid address")
	}

	lAddr := libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/%s/tcp/%s", address, port))
	h, err := libp2p.New(ctx, libp2p.Identity(key), lAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create host")
	}

	opts := &badger.DefaultOptions
	ds, err := badger.NewDatastore(cfg.GetDHTStoreDir(), opts)
	if err != nil {
		return nil, err
	}

	server, err := dht.New(ctx, h,
		ProtocolPrefix,
		dht.Validator(record.NamespacedValidator{}),
		dht.NamespacedValidator(GitObjectNamespace, validator{}),
		dht.Mode(dht.ModeServer),
		dht.Datastore(ds))
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize dht")
	}

	log := cfg.G().Log.Module("dht")
	fullAddr := fmt.Sprintf("%s/p2p/%s", h.Addrs()[0].String(), h.ID().Pretty())
	log.Info("Server service has started", "Address", fullAddr)

	node := &Server{
		host:          h,
		dht:           server,
		cfg:           cfg,
		log:           log,
		connTicker:    time.NewTicker(5 * time.Second),
		objectFinders: make(map[string]ObjectFinder),
	}

	node.commitStreamer = NewCommitStreamer(node, cfg)

	return node, err
}

// DHT returns the wrapped IPFS dht
func (dht *Server) DHT() *dht.IpfsDHT {
	return dht.dht
}

// Host returns the wrapped IPFS host
func (dht *Server) Host() host.Host {
	return dht.host
}

// Host returns the wrapped IPFS host
func (dht *Server) Finders() map[string]ObjectFinder {
	return dht.objectFinders
}

// Addr returns the p2p multiaddr of the dht host
func (dht *Server) Addr() string {
	return fmt.Sprintf("%s/p2p/%s", dht.host.Addrs()[0].String(), dht.host.ID().Pretty())
}

func (dht *Server) getBootstrapPeers() []string {
	if dht.cfg.DHT.BootstrapPeers == "" {
		return []string{}
	}
	return strings.Split(dht.cfg.DHT.BootstrapPeers, ",")
}

// Bootstrap attempts to connect to peers from the list of bootstrap peers
func (dht *Server) Bootstrap() error {

	// Get bootstrap addresses
	addrs := dht.getBootstrapPeers()
	if len(addrs) == 0 {
		return fmt.Errorf("no bootstrap peers to connect to")
	}

	// Attempt to connect to the bootstrap addresses and add them to the routing table
	for _, addr := range addrs {
		maddr, err := multiaddr.NewMultiaddr(addr)
		if err != nil {
			return errors.Wrap(err, "invalid dht bootstrap address")
		}

		info, err := peer.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			return errors.Wrap(err, "invalid dht bootstrap address")
		}

		if info.ID == dht.host.ID() {
			continue
		}

		dht.host.Peerstore().AddAddrs(info.ID, info.Addrs, peerstore.PermanentAddrTTL)
		ctx, cn := context.WithTimeout(context.Background(), 30*time.Second)
		if err := dht.host.Connect(ctx, *info); err != nil {
			dht.log.Error("failed to connect to peer", "PeerID", info.ID.Pretty(), "Err", err.Error())
		}
		cn()

		if _, err := dht.dht.RoutingTable().TryAddPeer(info.ID, true); err != nil {
			dht.log.Error("failed to add peer", "PeerID", info.ID.Pretty(), "Err", err.Error())
			continue
		}
	}

	return nil
}

// Start starts the DHT
func (dht *Server) Start() error {
	go dht.tryConnect()
	return nil
}

// tryConnect periodically attempts to connect the node to a peer
// if the routing table has no peer
func (dht *Server) tryConnect() {
	for range dht.connTicker.C {
		if len(dht.dht.RoutingTable().ListPeers()) == 0 {
			dht.Bootstrap()
		}
	}
}

// RegisterObjFinder registers a finder to handle module-targetted find operations
func (dht *Server) RegisterObjFinder(module string, finder ObjectFinder) {
	dht.objectFinders[module] = finder
}

// Peers returns a list of all peers
func (dht *Server) Peers() (peers []string) {
	for _, p := range dht.dht.RoutingTable().ListPeers() {
		peers = append(peers, p.String())
	}
	return
}

// Store adds a value corresponding to the given key
func (dht *Server) Store(ctx context.Context, key string, value []byte) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second*2)
	defer cancel()
	return dht.dht.PutValue(ctx, key, value)
}

// Lookup searches for a value corresponding to the given key
func (dht *Server) Lookup(ctx context.Context, key string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*15)
	defer cancel()
	return dht.dht.GetValue(ctx, key)
}

// GetProviders finds peers that have announced their capability to
// provide a value for the given key.
func (dht *Server) GetProviders(ctx context.Context, key []byte) ([]peer.AddrInfo, error) {
	id, err := MakeCid(key)
	if err != nil {
		return nil, err
	}

	peers, err := dht.dht.FindProviders(ctx, id)
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

// BasicCommitStreamer returns the commit streamer
func (dht *Server) CommitStreamer() CommitStreamer {
	return dht.commitStreamer
}

// MakeCid creates a content ID
func MakeCid(data []byte) (cid.Cid, error) {
	hash, err := multihash.Sum(data, multihash.BLAKE2B_MAX, -1)
	if err != nil {
		return cid.Cid{}, err
	}
	return cid.NewCidV1(cid.GitRaw, hash), nil
}

// Announce informs the network that it can provide value for the given key
func (dht *Server) Announce(ctx context.Context, key []byte) error {
	id, err := MakeCid(key)
	if err != nil {
		return err
	}
	return dht.dht.Provide(ctx, id, true)
}

// Close closes the host
func (dht *Server) Close() error {
	if dht.connTicker != nil {
		dht.connTicker.Stop()
	}
	if dht.host != nil {
		return dht.host.Close()
	}
	return nil
}

// fetchValue requests sends a query to a peer
func (dht *Server) fetchValue(
	ctx context.Context,
	query *DHTObjectQuery,
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

	if len(val) == 0 {
		return nil, ErrObjNotFound
	}

	return val, nil
}

// GetObject returns an object from a provider
func (dht *Server) GetObject(
	ctx context.Context,
	query *DHTObjectQuery) (obj []byte, err error) {

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

// HandleFetch processes incoming fetch requests
func (dht *Server) HandleFetch(s network.Stream) error {
	defer s.Close()

	bz := make([]byte, 256)
	if _, err := s.Read(bz); err != nil {
		return errors.Wrap(err, "failed to read query")
	}

	var query DHTObjectQuery
	if err := util.ToObject(bz, &query); err != nil {
		return errors.Wrap(err, "failed to decode query")
	}

	finder := dht.objectFinders[query.Module]
	if finder == nil {
		msg := fmt.Sprintf("finder for module `%s` not registered", query.Module)
		return fmt.Errorf("failed to process query: %s", msg)
	}

	objBz, err := finder.FindObject(query.ObjectKey)
	if err != nil {
		return errors.Wrapf(err, "failed to find requested object (%s)", string(query.ObjectKey))
	}

	if _, err := s.Write(objBz); err != nil {
		return errors.Wrap(err, "failed to Write back find result")
	}

	return nil
}
