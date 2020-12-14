package server_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	routing2 "github.com/libp2p/go-libp2p-core/routing"
	record "github.com/libp2p/go-libp2p-record"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/dht"
	"github.com/make-os/kit/dht/announcer"
	"github.com/make-os/kit/dht/server"
	"github.com/make-os/kit/mocks"
	testutil2 "github.com/make-os/kit/remote/testutil"
	"github.com/make-os/kit/testutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Server Suite")
}

type testObjectFinder struct {
	value []byte
	err   error
}

func (t *testObjectFinder) FindObject(key []byte) ([]byte, error) {
	return t.value, t.err
}

type errValidator struct {
	err error
}

// Validate conforms to the Validator interface.
func (v errValidator) Validate(key string, value []byte) error {
	return v.err
}

// Select conforms to the Validator interface.
func (v errValidator) Select(key string, values [][]byte) (int, error) {
	return 0, v.err
}

type okValidator struct{ err error }

func (v okValidator) Validate(key string, value []byte) error         { return nil }
func (v okValidator) Select(key string, values [][]byte) (int, error) { return 0, nil }

var _ = Describe("Server", func() {
	var err error
	var cfg, cfg2 *config.AppConfig
	var ctrl *gomock.Controller
	var dhtB *server.Server
	var dhtA *server.Server
	var keepers *mocks.MockKeepers
	var dhtKeepers *mocks.MockDHTKeeper

	BeforeEach(func() {
		server.ConnectTickerInterval = 1 * time.Millisecond

		ctrl = gomock.NewController(GinkgoT())
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg2, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.DHT.Address = testutil2.RandomAddr()
		cfg2.DHT.Address = testutil2.RandomAddr()

		keepers = mocks.NewMockKeepers(ctrl)
		dhtKeepers = mocks.NewMockDHTKeeper(ctrl)
		keepers.EXPECT().DHTKeeper().Return(dhtKeepers).AnyTimes()
	})

	AfterEach(func() {
		ctrl.Finish()

		if dhtA != nil {
			dhtA.Stop()
		}

		if dhtB != nil {
			dhtB.Stop()
		}

		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
		err = os.RemoveAll(cfg2.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".New", func() {
		When("address format is not valid", func() {
			It("should return err", func() {
				cfg.DHT.Address = "invalid"
				_, err = server.New(context.Background(), keepers, cfg)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("invalid address: address invalid: missing port in address"))
			})
		})

		When("unable to create host", func() {
			It("should return err", func() {
				cfg.DHT.Address = "0.1.1.1.0:999999"
				_, err = server.New(context.Background(), keepers, cfg)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("failed to create host"))
			})
		})

		When("no problem", func() {
			It("should return nil", func() {
				_, err = server.New(context.Background(), keepers, cfg)
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".Bootstrap", func() {
		BeforeEach(func() {
			dhtA, err = server.New(context.Background(), keepers, cfg)
			Expect(err).To(BeNil())
		})

		When("no bootstrap address exist", func() {
			It("should return error", func() {
				cfg.DHT.BootstrapPeers = ""
				err = dhtA.Bootstrap()
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("no bootstrap peers to connect to"))
			})
		})

		When("an address is not a valid P2p multi addr", func() {
			It("should return error", func() {
				cfg.DHT.BootstrapPeers = "/invalid/addr"
				err = dhtA.Bootstrap()
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("unknown protocol invalid"))
			})
		})

		When("an address exist and is valid but not reachable", func() {
			It("should not return error and ", func() {
				addr := "/ip4/127.0.0.1/tcp/9111/p2p/12D3KooWFtwJ7hUhHGCSiJNNwANjfsrTzbTdBw9GdmLNZHwyMPcd"
				cfg.DHT.BootstrapPeers = addr
				err = dhtA.Bootstrap()
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("failed to dial 12D3KooWFtwJ7hUhHGCSiJNNwANjfsrTzbTdBw9GdmLNZHwyMPcd: all dials failed"))
			})
		})

		When("a reachable address exist", func() {
			BeforeEach(func() {
				dhtB, err = server.New(context.Background(), nil, cfg2)
				Expect(err).To(BeNil())
				cfg.DHT.BootstrapPeers = dhtB.Addr()
			})

			It("should connect without error", func() {
				err = dhtA.Bootstrap()
				Expect(err).To(BeNil())
				Expect(dhtA.Host().Network().Conns()).To(HaveLen(1))
				Expect(dhtB.Host().Network().Conns()).To(HaveLen(1))
				Expect(dhtA.Host().Network().ConnsToPeer(dhtB.DHT().PeerID())).To(HaveLen(1))
				Expect(dhtB.Host().Network().ConnsToPeer(dhtA.DHT().PeerID())).To(HaveLen(1))
			})
		})
	})

	When(".Peers", func() {
		BeforeEach(func() {
			dhtA, err = server.New(context.Background(), keepers, cfg)
			Expect(err).To(BeNil())
		})

		When("not connected to any peers", func() {
			It("should return empty result", func() {
				Expect(dhtA.Peers()).To(BeEmpty())
			})
		})

		When("not connected to any peers", func() {
			It("should return empty result", func() {
				Expect(dhtA.Peers()).To(BeEmpty())
			})
		})

		When("connected to a peer", func() {
			BeforeEach(func() {
				dhtB, err = server.New(context.Background(), nil, cfg2)
				Expect(err).To(BeNil())
				cfg.DHT.BootstrapPeers = dhtB.Addr()
				err = dhtA.Bootstrap()
				Expect(err).To(BeNil())
				time.Sleep(10 * time.Millisecond)
			})

			It("should return 1 peer", func() {
				Expect(dhtA.Peers()).To(HaveLen(1))
			})
		})
	})

	Describe(".Store", func() {
		BeforeEach(func() {
			dhtA, err = server.New(context.Background(), keepers, cfg)
			Expect(err).To(BeNil())
			dhtA.DHT().Validator.(record.NamespacedValidator)[dht.ObjectNamespace] = okValidator{}
			dhtB, err = server.New(context.Background(), nil, cfg2)
			Expect(err).To(BeNil())
			cfg.DHT.BootstrapPeers = dhtB.Addr()
			err = dhtA.Bootstrap()
			Expect(err).To(BeNil())
			time.Sleep(10 * time.Millisecond)
		})

		It("should return error when keytype used to store is invalid", func() {
			err := dhtA.Store(context.Background(), "key", []byte("value"))
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("invalid record keytype"))
		})

		It("should return validation failed", func() {
			dhtA.DHT().Validator.(record.NamespacedValidator)[dht.ObjectNamespace] = errValidator{
				err: fmt.Errorf("validation error"),
			}
			key := dht.MakeKey("hash1")
			err = dhtA.Store(context.Background(), key, []byte("value"))
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("validation error"))
		})
	})

	Describe(".Lookup", func() {
		BeforeEach(func() {
			dhtA, err = server.New(context.Background(), keepers, cfg)
			Expect(err).To(BeNil())
			dhtA.DHT().Validator.(record.NamespacedValidator)[dht.ObjectNamespace] = okValidator{}
			dhtB, err = server.New(context.Background(), nil, cfg2)
			Expect(err).To(BeNil())
			cfg.DHT.BootstrapPeers = dhtB.Addr()
			err = dhtA.Bootstrap()
			Expect(err).To(BeNil())
			time.Sleep(10 * time.Millisecond)
		})

		When("key is not found", func() {
			It("should return nil", func() {
				_, err := dhtA.Lookup(context.Background(), "key")
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(routing2.ErrNotFound))
			})
		})

		When("key is found", func() {
			It("should return error when unable to find object", func() {
				key := dht.MakeKey("hash1")
				err := dhtA.Store(context.Background(), key, []byte("value"))
				Expect(err).To(BeNil())
				val, err := dhtA.Lookup(context.Background(), "unknown_key")
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError(routing2.ErrNotFound))
				Expect(val).To(BeEmpty())
			})

			It("should return its corresponding value", func() {
				key := dht.MakeKey("hash1")
				dhtA.Store(context.Background(), key, []byte("value"))
				val, err := dhtA.Lookup(context.Background(), key)
				Expect(err).To(BeNil())
				Expect(val).To(Equal([]byte("value")))
			})
		})

		Context("both peers lookup check", func() {
			Specify("that connected peer can also lookup the key's value", func() {
				key := dht.MakeKey("hash1")
				err = dhtA.Store(context.Background(), key, []byte("value"))
				Expect(err).To(BeNil())
				val, err := dhtB.Lookup(context.Background(), key)
				Expect(err).To(BeNil())
				Expect(val).To(Equal([]byte("value")))
			})
		})
	})

	Describe(".Announce and .GetRepoObjectProviders", func() {
		var key = []byte("key")

		BeforeEach(func() {
			dhtA, err = server.New(context.Background(), keepers, cfg)
			Expect(err).To(BeNil())
			dhtB, err = server.New(context.Background(), nil, cfg2)
			Expect(err).To(BeNil())
			cfg.DHT.BootstrapPeers = dhtB.Addr()
			err = dhtA.Bootstrap()
			Expect(err).To(BeNil())
			time.Sleep(10 * time.Millisecond)
		})

		When("a peer announces a key", func() {
			BeforeEach(func() {
				dhtKeepers.EXPECT().AddToAnnounceList(key, "repo1", announcer.ObjTypeAny, gomock.Any())
				dhtA.Announce(announcer.ObjTypeAny, "repo1", key, nil)
				dhtA.Start()
				time.Sleep(1 * time.Millisecond)
			})

			It("should return no error", func() {
				Expect(err).To(BeNil())
			})

			It("should be returned as a provider on all connected peers", func() {
				addrs, err := dhtA.GetProviders(context.Background(), key)
				Expect(err).To(BeNil())
				Expect(addrs).To(HaveLen(1))
				Expect(addrs[0].ID.Pretty()).To(Equal(dhtA.Host().ID().Pretty()))
				Expect(addrs[0].Addrs).To(BeEmpty())

				addrs, err = dhtB.GetProviders(context.Background(), key)
				Expect(err).To(BeNil())
				Expect(addrs).To(HaveLen(1))
				Expect(addrs[0].ID.Pretty()).To(Equal(dhtA.Host().ID().Pretty()))
				Expect(addrs[0].Addrs).To(HaveLen(1))
				Expect(addrs[0].Addrs[0].String()).To(Equal(dhtA.Host().Addrs()[0].String()))
			})
		})
	})
})
