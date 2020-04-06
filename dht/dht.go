package dht

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"strings"
	"time"

	"gitlab.com/makeos/mosdef/dht/types"

	"github.com/ipfs/go-cid"
	badger "github.com/ipfs/go-ds-badger"
	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multihash"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	dhtopts "github.com/libp2p/go-libp2p-kad-dht/opts"
	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/pkgs/logger"
	"gitlab.com/makeos/mosdef/util"
)

// Errors
var (
	ErrObjNotFound = fmt.Errorf("object not found")
)

// DHTNode provides distributed hash table functionalities
// specifically required for storing and
type DHT struct {
	cfg           *config.AppConfig
	host          host.Host
	dht           *dht.IpfsDHT
	log           logger.Logger
	objectFinders map[string]types.ObjectFinder
	ticker        *time.Ticker
}

// New creates a new DHT Node
func New(
	ctx context.Context,
	cfg *config.AppConfig,
	key crypto.PrivKey,
	addr string) (*DHT, error) {

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

	ipfsDht, err := dht.New(ctx, h,
		dhtopts.Validator(validator{}),
		dhtopts.Datastore(ds))
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize dht")
	}

	log := cfg.G().Log.Module("dht")
	fullAddr := fmt.Sprintf("%s/p2p/%s", h.Addrs()[0].String(), h.ID().Pretty())
	log.Info("DHT service has started", "Address", fullAddr)

	dhtNode := &DHT{
		host:          h,
		dht:           ipfsDht,
		cfg:           cfg,
		log:           log,
		ticker:        time.NewTicker(5 * time.Second),
		objectFinders: make(map[string]types.ObjectFinder),
	}

	h.SetStreamHandler("/fetch/1", func(s network.Stream) {
		if err := dhtNode.handleFetch(s); err != nil {
			log.Error(err.Error())
		}
	})

	return dhtNode, err
}

// Addr returns the p2p multiaddr of the dht host
func (dht *DHT) Addr() string {
	return fmt.Sprintf("%s/p2p/%s", dht.host.Addrs()[0].String(), dht.host.ID().Pretty())
}

func (dht *DHT) getBootstrapPeers() []string {
	if dht.cfg.DHT.BootstrapPeers == "" {
		return []string{}
	}
	return strings.Split(dht.cfg.DHT.BootstrapPeers, ",")
}

// join attempts to connect to peers from the list of bootstrap peers
func (dht *DHT) join() error {

	addrs := dht.getBootstrapPeers()
	if len(addrs) == 0 {
		return fmt.Errorf("no bootstrap peers to connect to")
	}

	connected := false
	for _, addr := range addrs {
		maddr, err := multiaddr.NewMultiaddr(addr)
		if err != nil {
			return errors.Wrap(err, "invalid dht bootstrap address")
		}

		info, err := peer.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			return errors.Wrap(err, "invalid dht bootstrap address")
		}

		dht.host.Peerstore().AddAddrs(info.ID, info.Addrs, peerstore.PermanentAddrTTL)
		ctx, cn := context.WithTimeout(context.Background(), 60*time.Second)
		if err := dht.host.Connect(ctx, *info); err != nil {
			cn()
			continue
		}
		cn()

		connected = true
		break
	}

	if !connected {
		return fmt.Errorf("could not connect to peers")
	}

	return nil
}

// Start starts the DHTNode
func (dht *DHT) Start() error {
	go dht.tryConnect()
	return nil
}

// tryConnect periodically attempts to connect the node to a peer
// if no connection has been established.
func (dht *DHT) tryConnect() {
	for range dht.ticker.C {
		if len(dht.host.Network().Conns()) == 0 {
			dht.join()
		}
	}
}

// RegisterObjFinder registers a finder to handle module-targetted find operations
func (dht *DHT) RegisterObjFinder(module string, finder types.ObjectFinder) {
	dht.objectFinders[module] = finder
}

// Peers returns a list of all peers
func (dht *DHT) Peers() (peers []string) {
	for _, p := range dht.dht.RoutingTable().ListPeers() {
		peers = append(peers, p.String())
	}
	return
}

// Store adds a value corresponding to the given key
func (dht *DHT) Store(ctx context.Context, key string, value []byte) error {
	return dht.dht.PutValue(ctx, key, value)
}

// Lookup searches for a value corresponding to the given key
func (dht *DHT) Lookup(ctx context.Context, key string) ([]byte, error) {
	return dht.dht.GetValue(ctx, key)
}

// GetProviders finds peers that have announced their capability to
// provide a value for the given key.
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
	// Note: We are doing this here because the DHTNode logic does not add them when
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

// Announce informs the network that it can provide value for the given key
func (dht *DHT) Announce(ctx context.Context, key []byte) error {
	id, err := cid.NewPrefixV1(cid.Raw, multihash.BLAKE2B_MAX).Sum(key)
	if err != nil {
		return err
	}
	return dht.dht.Provide(ctx, id, true)
}

// Close closes the host
func (dht *DHT) Close() error {
	if dht.ticker != nil {
		dht.ticker.Stop()
	}
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

	if len(val) == 0 {
		return nil, ErrObjNotFound
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
func (dht *DHT) handleFetch(s network.Stream) error {
	defer s.Close()

	bz := make([]byte, 256)
	if _, err := s.Read(bz); err != nil {
		return errors.Wrap(err, "failed to read query")
	}

	var query types.DHTObjectQuery
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
		return errors.Wrap(err, "failed to write back find result")
	}

	return nil
}
