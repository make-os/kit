package server

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	badger "github.com/ipfs/go-ds-badger"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	record "github.com/libp2p/go-libp2p-record"
	"github.com/make-os/kit/config"
	dht2 "github.com/make-os/kit/dht"
	announcer2 "github.com/make-os/kit/dht/announcer"
	"github.com/make-os/kit/dht/streamer"
	"github.com/make-os/kit/dht/streamer/types"
	types2 "github.com/make-os/kit/dht/types"
	"github.com/make-os/kit/pkgs/logger"
	"github.com/make-os/kit/types/core"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"
)

var (
	ProtocolPrefix        = dht.ProtocolPrefix("/make-os")
	ConnectTickerInterval = 5 * time.Second
)

// Server provides distributed hash table functionalities.
type Server struct {
	cfg            *config.AppConfig
	host           host.Host
	dht            *dht.IpfsDHT
	log            logger.Logger
	connTicker     *time.Ticker
	objectStreamer types.ObjectStreamer
	announcer      types2.Announcer
	stopped        bool
	stopOnce       *sync.Once
}

// New creates a new Server
func New(ctx context.Context, keepers core.Keepers, cfg *config.AppConfig) (*Server, error) {

	key, _ := cfg.G().PrivVal.GetKey()

	address, port, err := net.SplitHostPort(cfg.DHT.Address)
	if err != nil {
		return nil, errors.Wrap(err, "invalid address")
	}

	lAddr := libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/%s/tcp/%s", address, port))
	h, err := libp2p.New(ctx, libp2p.Identity(key.PrivKey().Key()), lAddr)
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
		stopOnce:   &sync.Once{},
		connTicker: time.NewTicker(ConnectTickerInterval),
		announcer:  announcer2.New(cfg, server, keepers),
	}

	node.objectStreamer = streamer.NewObjectStreamer(node, cfg)

	go func() {
		cfg.G().Interrupt.Wait()
		node.Stop()
	}()

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
		return nil
	}
	return strings.Split(dht.cfg.DHT.BootstrapPeers, ",")
}

// Bootstrap attempts to connect to peers from the list of bootstrap peers
func (dht *Server) Bootstrap() (err error) {

	addrs := dht.getBootstrapPeers()
	if len(addrs) == 0 {
		return fmt.Errorf("no bootstrap peers to connect to")
	}

	for _, addr := range addrs {
		if addr == "" {
			continue
		}

		var maddr multiaddr.Multiaddr
		maddr, err = multiaddr.NewMultiaddr(addr)
		if err != nil {
			dht.log.Error("Invalid bootstrap address", "Addr", addr, "Err", err)
			continue
		}

		var info *peer.AddrInfo
		info, err = peer.AddrInfoFromP2pAddr(maddr)
		if err != nil {
			dht.log.Error("Invalid bootstrap address", "Addr", addr, "Err", err)
			continue
		}

		if info.ID == dht.host.ID() {
			continue
		}

		dht.host.Peerstore().AddAddrs(info.ID, info.Addrs, peerstore.PermanentAddrTTL)
		ctx, cn := context.WithTimeout(context.Background(), 30*time.Second)
		if err = dht.host.Connect(ctx, *info); err != nil {
			dht.log.Error("Failed to connect to peer", "PeerID", info.ID.Pretty(), "Err", err.Error())
			cn()
			continue
		}
		cn()

		if _, err = dht.dht.RoutingTable().TryAddPeer(info.ID, true); err != nil {
			dht.log.Error("failed to add peer", "PeerID", info.ID.Pretty(), "Err", err.Error())
			continue
		}
	}

	return err
}

// Start starts the DHT
func (dht *Server) Start() error {

	// Attempt to connect the network
	go dht.connector()

	// Start the announcer
	dht.announcer.Start()

	return nil
}

// connector periodically attempts to connect the node to a peer
// if the routing table has no peer
func (dht *Server) connector() {
	for range dht.connTicker.C {
		if len(dht.Peers()) == 0 {
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

// Store adds a value corresponding to the given key.
// It will store the value locally even when an error occurred due
// to a lack of peer in the routing table.
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

// GetRepoObjectProviders finds peers that have announced
// their readiness to provide a value for the given key.
func (dht *Server) GetProviders(ctx context.Context, key []byte) ([]peer.AddrInfo, error) {
	id, err := dht2.MakeCID(key)
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

// Announce a repository object
func (dht *Server) Announce(objType int, repo string, key []byte, doneCB func(error)) bool {
	return dht.announcer.Announce(objType, repo, key, doneCB)
}

// NewAnnouncerSession creates an announcer session
func (dht *Server) NewAnnouncerSession() types2.Session {
	return dht.announcer.NewSession()
}

// RegisterChecker registers an object checker to the announcer.
func (dht *Server) RegisterChecker(objType int, f types2.CheckFunc) {
	dht.announcer.RegisterChecker(objType, f)
}

// Stop stops the server
func (dht *Server) Stop() (err error) {
	dht.stopOnce.Do(func() {
		dht.stopped = true

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
	})
	return
}
