package dht

import (
	"context"
	"fmt"
	types2 "gitlab.com/makeos/mosdef/dht/types"
	"io/ioutil"
	"net"
	"strings"
	"time"

	"github.com/ipfs/go-cid"
	badger "github.com/ipfs/go-ds-badger"
	"github.com/multiformats/go-multiaddr"
	"github.com/multiformats/go-multihash"

	libp2p "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	dhtopts "github.com/libp2p/go-libp2p-kad-dht/opts"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/util"
	"gitlab.com/makeos/mosdef/util/logger"
	"github.com/pkg/errors"
)

// Errors
var (
	ErrObjNotFound = fmt.Errorf("object not found")
)

// DHT provides distributed hash table functionalities
// specifically required for storing and
type DHT struct {
	cfg           *config.AppConfig
	host          host.Host
	dht           *dht.IpfsDHT
	log           logger.Logger
	objectFinders map[string]types2.ObjectFinder
	ticker        *time.Ticker
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
	fullAddr := fmt.Sprintf("%s/p2p/%s", h.Addrs()[0].String(), h.ID().Pretty())
	log.Info("DHT service has started", "Address", fullAddr)

	dht := &DHT{
		host:          h,
		dht:           ipfsDht,
		cfg:           cfg,
		log:           log,
		objectFinders: make(map[string]types2.ObjectFinder),
	}

	h.SetStreamHandler("/fetch/1", dht.handleFetch)

	return dht, err
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
		defer cn()
		if err := dht.host.Connect(ctx, *info); err != nil {
			continue
		}

		connected = true
		break
	}

	if !connected {
		dht.log.Error("Could not connect to any bootstrap peers", "KnownAddrs", len(addrs))
		return fmt.Errorf("could not connect to peers")
	}

	return nil
}

// Start starts the DHT
func (dht *DHT) Start() error {
	go dht.attemptToJoinPeers()
	return nil
}

// attemptToJoinPeers periodically attempts to connect the DHT to a peer
// if no connection has been established.
func (dht *DHT) attemptToJoinPeers() {
	dht.ticker = time.NewTicker(5 * time.Second)
	for range dht.ticker.C {
		if len(dht.host.Network().Conns()) == 0 {
			dht.join()
		}
	}
}

// RegisterObjFinder registers a finder to handle module-targetted find operations
func (dht *DHT) RegisterObjFinder(module string, finder types2.ObjectFinder) {
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

// Announce informs the network that it can provide value for the given key
func (dht *DHT) Announce(ctx context.Context, key []byte) error {
	cid, err := cid.NewPrefixV1(cid.Raw, multihash.BLAKE2B_MAX).Sum(key)
	if err != nil {
		return err
	}
	return dht.dht.Provide(ctx, cid, true)
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
	query *types2.DHTObjectQuery,
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
	query *types2.DHTObjectQuery) (obj []byte, err error) {

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

	var query types2.DHTObjectQuery
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
		dht.log.Error("failed to find requested object", "Err", err.Error(),
			"ObjectKey", string(query.ObjectKey))
	}

	if _, err := s.Write(objBz); err != nil {
		dht.log.Error("failed to write back find result", "Err", err.Error())
		return
	}
}
