package server

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	badger "github.com/ipfs/go-ds-badger"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	record "github.com/libp2p/go-libp2p-record"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
	"gitlab.com/makeos/lobe/config"
	dht2 "gitlab.com/makeos/lobe/dht"
	announcer2 "gitlab.com/makeos/lobe/dht/announcer"
	"gitlab.com/makeos/lobe/dht/streamer"
	"gitlab.com/makeos/lobe/dht/streamer/types"
	"gitlab.com/makeos/lobe/pkgs/logger"
)

var (
	ProtocolPrefix = dht.ProtocolPrefix("/makeos")
)

// Server provides distributed hash table functionalities.
type Server struct {
	cfg            *config.AppConfig
	host           host.Host
	dht            *dht.IpfsDHT
	log            logger.Logger
	connTicker     *time.Ticker
	objectStreamer types.ObjectStreamer
	announcer      announcer2.Announcer
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
		dht.NamespacedValidator(dht2.ObjectNamespace, validator{}),
		dht.Mode(dht.ModeServer),
		dht.Datastore(ds))
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize dht")
	}

	log := cfg.G().Log.Module("dht")
	fullAddr := fmt.Sprintf("%s/p2p/%s", h.Addrs()[0].String(), h.ID().Pretty())
	log.Info("Server service has started", "Address", fullAddr)

	node := &Server{
		host:       h,
		dht:        server,
		cfg:        cfg,
		log:        log,
		connTicker: time.NewTicker(5 * time.Second),
		announcer:  announcer2.NewBasicAnnouncer(server, 10, log.Module("announcer")),
	}

	node.objectStreamer = streamer.NewObjectStreamer(node, cfg)

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
	go dht.connector()
	dht.announcer.Start()
	return nil
}

// connector periodically attempts to connect the node to a peer
// if the routing table has no peer
func (dht *Server) connector() {
	for range dht.connTicker.C {
		if len(dht.dht.RoutingTable().ListPeers()) == 0 {
			dht.Bootstrap()
		}
	}
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
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return dht.dht.PutValue(ctx, key, value)
}

// Lookup searches for a value corresponding to the given key
func (dht *Server) Lookup(ctx context.Context, key string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return dht.dht.GetValue(ctx, key)
}

// GetRepoObjectProviders finds peers that have announced their capability to
// provide a value for the given key.
func (dht *Server) GetProviders(ctx context.Context, key []byte) ([]peer.AddrInfo, error) {
	id, err := dht2.MakeCid(key)
	if err != nil {
		return nil, err
	}

	peers, err := dht.dht.FindProviders(ctx, id)
	if err != nil {
		return nil, err
	}

	// For providers whose addresses are not included, find their address(es) from the
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

// ObjectStreamer returns the commit streamer
func (dht *Server) ObjectStreamer() types.ObjectStreamer {
	return dht.objectStreamer
}

// Announce asynchronously informs the network that it can provide value for the given key
func (dht *Server) Announce(key []byte, doneCB func(error)) {
	dht.announcer.Announce(key, doneCB)
}

// Close closes the host
func (dht *Server) Stop() error {
	var err error

	if dht.connTicker != nil {
		dht.connTicker.Stop()
	}

	if dht.announcer != nil {
		dht.announcer.Stop()
	}

	if dht.host != nil {
		err = dht.host.Close()
	}

	if dht.dht != nil {
		dht.dht.RoutingTable().Close()
		dht.dht.Close()
	}

	return err
}
