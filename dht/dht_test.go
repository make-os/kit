package dht

import (
	"context"
	"fmt"
	types2 "gitlab.com/makeos/mosdef/dht/types"
	"os"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/libp2p/go-libp2p-core/peer"
	routing "github.com/libp2p/go-libp2p-routing"
	"github.com/phayes/freeport"

	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/testutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func randomAddr() string {
	port, err := freeport.GetFreePort()
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("127.0.0.1:%d", port)
}

func connect(node1, node2 *DHT) {
	node2AddrInfo := peer.AddrInfo{ID: node2.host.ID(), Addrs: node2.host.Addrs()}
	err := node1.host.Connect(context.Background(), node2AddrInfo)
	if err != nil {
		panic(err)
	}
}

type testObjectFinder struct {
	value []byte
	err   error
}

func (t *testObjectFinder) FindObject(key []byte) ([]byte, error) {
	return t.value, t.err
}

var _ = Describe("App", func() {
	var err error
	var addr string
	var cfg, cfg2 *config.AppConfig
	var key = crypto.NewKeyFromIntSeed(1)
	var ctrl *gomock.Controller
	var key2 = crypto.NewKeyFromIntSeed(2)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg2, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
	})

	BeforeEach(func() {
		port := freeport.GetPort()
		addr = fmt.Sprintf("127.0.0.1:%d", port)
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
		err = os.RemoveAll(cfg2.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".New", func() {
		When("address format is not valid", func() {
			It("should return err", func() {
				_, err = New(context.Background(), cfg, key.PrivKey().Key(), "invalid")
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("invalid address: address invalid: missing port in address"))
			})
		})

		When("unable to create host", func() {
			It("should return err", func() {
				_, err = New(context.Background(), cfg, key.PrivKey().Key(), "0.1.1.1.0:999999")
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("failed to create host"))
			})
		})

		When("no problem", func() {
			It("should return nil", func() {
				_, err = New(context.Background(), cfg, key.PrivKey().Key(), addr)
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".join", func() {
		var dht *DHT

		BeforeEach(func() {
			dht, err = New(context.Background(), cfg, key.PrivKey().Key(), addr)
			Expect(err).To(BeNil())
		})

		When("no bootstrap address exist", func() {
			It("should return error", func() {
				err = dht.join()
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("no bootstrap peers to connect to"))
			})
		})

		When("an address is not a valid P2p multi addr", func() {
			It("should return error", func() {
				cfg.DHT.BootstrapPeers = "invalid/addr"
				err = dht.join()
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("invalid dht bootstrap address: failed to parse multiaddr"))
			})
		})

		When("an address exist and is valid but not reachable", func() {
			It("should return error", func() {
				addr := "/ip4/127.0.0.1/tcp/9003/p2p/12D3KooWFtwJ7hUhHGCSiJNNwANjfsrTzbTdBw9GdmLNZHwyMPcd"
				cfg.DHT.BootstrapPeers = addr
				err = dht.join()
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("could not connect to peers"))
			})
		})

		When("a reachable address exist", func() {
			var peerDHT *DHT

			BeforeEach(func() {
				peerDHT, err = New(context.Background(), cfg2, key2.PrivKey().Key(), randomAddr())
				Expect(err).To(BeNil())
				cfg.DHT.BootstrapPeers = peerDHT.Addr()
			})

			It("should connect without error", func() {
				err = dht.join()
				Expect(err).To(BeNil())
				Expect(dht.host.Network().Conns()).To(HaveLen(1))
				Expect(peerDHT.host.Network().Conns()).To(HaveLen(1))
				Expect(dht.host.Network().ConnsToPeer(peerDHT.dht.PeerID())).To(HaveLen(1))
				Expect(peerDHT.host.Network().ConnsToPeer(dht.dht.PeerID())).To(HaveLen(1))
			})
		})
	})

	When(".Peers", func() {
		var dht *DHT

		BeforeEach(func() {
			dht, err = New(context.Background(), cfg, key.PrivKey().Key(), addr)
			Expect(err).To(BeNil())
		})

		When("not connected to any peers", func() {
			It("should return empty result", func() {
				Expect(dht.Peers()).To(BeEmpty())
			})
		})

		When("not connected to any peers", func() {
			It("should return empty result", func() {
				Expect(dht.Peers()).To(BeEmpty())
			})
		})

		When("connected to a peer", func() {
			var peerDHT *DHT

			BeforeEach(func() {
				peerDHT, err = New(context.Background(), cfg2, key2.PrivKey().Key(), randomAddr())
				Expect(err).To(BeNil())
				cfg.DHT.BootstrapPeers = peerDHT.Addr()
				err = dht.join()
				Expect(err).To(BeNil())
				time.Sleep(10 * time.Millisecond)
			})

			It("should return 1 peer", func() {
				Expect(dht.Peers()).To(HaveLen(1))
			})
		})
	})

	Describe(".RegisterObjFinder", func() {
		var dht *DHT

		BeforeEach(func() {
			dht, err = New(context.Background(), cfg, key.PrivKey().Key(), addr)
			Expect(err).To(BeNil())
		})

		It("should register a finder", func() {
			dht.RegisterObjFinder("module_name", &testObjectFinder{})
			Expect(dht.objectFinders).To(HaveKey("module_name"))
		})
	})

	Describe(".Store & .Lookup", func() {
		var peerDHT *DHT
		var dht *DHT

		BeforeEach(func() {
			dht, err = New(context.Background(), cfg, key.PrivKey().Key(), addr)
			Expect(err).To(BeNil())
			peerDHT, err = New(context.Background(), cfg2, key2.PrivKey().Key(), randomAddr())
			Expect(err).To(BeNil())
			cfg.DHT.BootstrapPeers = peerDHT.Addr()
			err = dht.join()
			Expect(err).To(BeNil())
			time.Sleep(10 * time.Millisecond)
		})

		When("key is not found", func() {
			It("should return nil", func() {
				_, err := dht.Lookup(context.Background(), "key")
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(routing.ErrNotFound))
			})
		})

		When("key is found", func() {
			It("should return its corresponding value", func() {
				dht.Store(context.Background(), "key", []byte("value"))
				val, err := dht.Lookup(context.Background(), "key")
				Expect(err).To(BeNil())
				Expect(val).To(Equal([]byte("value")))
			})
		})

		Context("both peers lookup check", func() {
			Specify("that connected peer can also lookup the key's value", func() {
				dht.Store(context.Background(), "key", []byte("value"))
				val, err := peerDHT.Lookup(context.Background(), "key")
				Expect(err).To(BeNil())
				Expect(val).To(Equal([]byte("value")))
			})
		})
	})

	Describe(".Announce and .GetProviders", func() {
		var peerDHT *DHT
		var dht *DHT

		BeforeEach(func() {
			dht, err = New(context.Background(), cfg, key.PrivKey().Key(), addr)
			Expect(err).To(BeNil())
			peerDHT, err = New(context.Background(), cfg2, key2.PrivKey().Key(), randomAddr())
			Expect(err).To(BeNil())
			cfg.DHT.BootstrapPeers = peerDHT.Addr()
			err = dht.join()
			Expect(err).To(BeNil())
			time.Sleep(10 * time.Millisecond)
		})

		When("a peer annonce a key", func() {
			BeforeEach(func() {
				err = dht.Annonce(context.Background(), []byte("key"))
			})

			It("should return no error", func() {
				Expect(err).To(BeNil())
			})

			It("should be returned as a provider on all connected peers", func() {
				addrs, err := dht.GetProviders(context.Background(), []byte("key"))
				Expect(err).To(BeNil())
				Expect(addrs).To(HaveLen(1))
				Expect(addrs[0].ID.Pretty()).To(Equal(dht.host.ID().Pretty()))
				Expect(addrs[0].Addrs).To(BeEmpty())

				addrs, err = peerDHT.GetProviders(context.Background(), []byte("key"))
				Expect(err).To(BeNil())
				Expect(addrs).To(HaveLen(1))
				Expect(addrs[0].ID.Pretty()).To(Equal(dht.host.ID().Pretty()))
				Expect(addrs[0].Addrs).To(HaveLen(1))
				Expect(addrs[0].Addrs[0].String()).To(Equal(dht.host.Addrs()[0].String()))
			})
		})
	})

	Describe(".GetObject", func() {
		var peerDHT *DHT
		var dht *DHT

		BeforeEach(func() {
			dht, err = New(context.Background(), cfg, key.PrivKey().Key(), addr)
			Expect(err).To(BeNil())
			peerDHT, err = New(context.Background(), cfg2, key2.PrivKey().Key(), randomAddr())
			Expect(err).To(BeNil())
			cfg.DHT.BootstrapPeers = peerDHT.Addr()
			err = dht.join()
			Expect(err).To(BeNil())
			time.Sleep(10 * time.Millisecond)
		})

		When("no providers exist", func() {
			It("should return err=ErrObjNotFound", func() {
				_, err = dht.GetObject(context.Background(), &types2.DHTObjectQuery{ObjectKey: []byte("key")})
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(ErrObjNotFound))
			})
		})

		When("provider is the local address, but the target finder module was not registered", func() {
			BeforeEach(func() {
				err = dht.Announce(context.Background(), []byte("key"))
				Expect(err).To(BeNil())
			})
			It("should return err about unregistered module", func() {
				_, err = dht.GetObject(context.Background(), &types2.DHTObjectQuery{
					Module:    "unknown",
					ObjectKey: []byte("key"),
				})
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("finder for module `unknown` not registered"))
			})
		})

		When("provider is the local address, but the target finder module returns an error", func() {
			BeforeEach(func() {
				dht.RegisterObjFinder("my-finder", &testObjectFinder{err: fmt.Errorf("bad error")})
				err = dht.Announce(context.Background(), []byte("key"))
				Expect(err).To(BeNil())
			})
			It("should return err the finder error", func() {
				_, err = dht.GetObject(context.Background(), &types2.DHTObjectQuery{
					Module:    "my-finder",
					ObjectKey: []byte("key"),
				})
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("finder error: bad error"))
			})
		})

		When("provider is the local address, but the target finder module returns no error and nil value", func() {
			BeforeEach(func() {
				dht.RegisterObjFinder("my-finder", &testObjectFinder{})
				err = dht.Announce(context.Background(), []byte("key"))
				Expect(err).To(BeNil())
			})
			It("should return err=ErrObjNotFound", func() {
				_, err = dht.GetObject(context.Background(), &types2.DHTObjectQuery{
					Module:    "my-finder",
					ObjectKey: []byte("key"),
				})
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(ErrObjNotFound))
			})
		})

		When("provider is the local address and the target finder module returns a value and no error", func() {
			BeforeEach(func() {
				dht.RegisterObjFinder("my-finder", &testObjectFinder{value: []byte("value")})
				err = dht.Announce(context.Background(), []byte("key"))
				Expect(err).To(BeNil())
			})

			It("should return value returned by the object finder", func() {
				retVal, err := dht.GetObject(context.Background(), &types2.DHTObjectQuery{
					Module:    "my-finder",
					ObjectKey: []byte("key"),
				})
				Expect(err).To(BeNil())
				Expect(retVal).To(Equal([]byte("value")))
			})

			Specify("that non-local peers can also find the key and value", func() {
				retVal, err := peerDHT.GetObject(context.Background(), &types2.DHTObjectQuery{
					Module:    "my-finder",
					ObjectKey: []byte("key"),
				})
				Expect(err).To(BeNil())
				Expect(retVal).To(Equal([]byte("value")))
			})
		})
	})
})
