package dht_test

import (
	"context"
	"fmt"
	"os"
	"time"

	routing2 "github.com/libp2p/go-libp2p-core/routing"
	record "github.com/libp2p/go-libp2p-record"
	"gitlab.com/makeos/mosdef/dht"
	"gitlab.com/makeos/mosdef/mocks"

	"github.com/golang/mock/gomock"
	"github.com/phayes/freeport"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/testutil"
)

func randomAddr() string {
	port, err := freeport.GetFreePort()
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("127.0.0.1:%d", port)
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
	var addr string
	var cfg, cfg2 *config.AppConfig
	var key = crypto.NewKeyFromIntSeed(1)
	var ctrl *gomock.Controller
	var key2 = crypto.NewKeyFromIntSeed(2)
	var dhtB *dht.Server
	var dhtA *dht.Server

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg2, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
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
				_, err = dht.New(context.Background(), cfg, key.PrivKey().Key(), "invalid")
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("invalid address: address invalid: missing port in address"))
			})
		})

		When("unable to create host", func() {
			It("should return err", func() {
				_, err = dht.New(context.Background(), cfg, key.PrivKey().Key(), "0.1.1.1.0:999999")
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("failed to create host"))
			})
		})

		When("no problem", func() {
			It("should return nil", func() {
				_, err = dht.New(context.Background(), cfg, key.PrivKey().Key(), addr)
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".Bootstrap", func() {
		BeforeEach(func() {
			dhtA, err = dht.New(context.Background(), cfg, key.PrivKey().Key(), addr)
			Expect(err).To(BeNil())
		})

		AfterEach(func() {
			dhtA.Close()
		})

		When("no bootstrap address exist", func() {
			It("should return error", func() {
				err = dhtA.Bootstrap()
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("no bootstrap peers to connect to"))
			})
		})

		When("an address is not a valid P2p multi addr", func() {
			It("should return error", func() {
				cfg.DHT.BootstrapPeers = "invalid/addr"
				err = dhtA.Bootstrap()
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("invalid dht bootstrap address: failed to parse multiaddr"))
			})
			It("should return error", func() {
				cfg.DHT.BootstrapPeers = "invalid/addr"
				err = dhtA.Bootstrap()
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("invalid dht bootstrap address: failed to parse multiaddr"))
			})
		})

		When("an address exist and is valid but not reachable", func() {
			It("should not return error and ", func() {
				addr := "/ip4/127.0.0.1/tcp/9111/p2p/12D3KooWFtwJ7hUhHGCSiJNNwANjfsrTzbTdBw9GdmLNZHwyMPcd"
				cfg.DHT.BootstrapPeers = addr
				err = dhtA.Bootstrap()
				Expect(err).To(BeNil())
			})
		})

		When("a reachable address exist", func() {
			BeforeEach(func() {
				dhtB, err = dht.New(context.Background(), cfg2, key2.PrivKey().Key(), randomAddr())
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
			dhtA, err = dht.New(context.Background(), cfg, key.PrivKey().Key(), addr)
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
				dhtB, err = dht.New(context.Background(), cfg2, key2.PrivKey().Key(), randomAddr())
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

	Describe(".RegisterObjFinder", func() {
		BeforeEach(func() {
			dhtA, err = dht.New(context.Background(), cfg, key.PrivKey().Key(), addr)
			Expect(err).To(BeNil())
		})

		It("should register a finder", func() {
			dhtA.RegisterObjFinder("module_name", &testObjectFinder{})
			Expect(dhtA.Finders()).To(HaveKey("module_name"))
		})
	})

	Describe(".Store", func() {
		BeforeEach(func() {
			dhtA, err = dht.New(context.Background(), cfg, key.PrivKey().Key(), addr)
			Expect(err).To(BeNil())
			dhtA.DHT().Validator.(record.NamespacedValidator)[dht.GitObjectNamespace] = okValidator{}
			dhtB, err = dht.New(context.Background(), cfg2, key2.PrivKey().Key(), randomAddr())
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
			dhtA.DHT().Validator.(record.NamespacedValidator)[dht.GitObjectNamespace] = errValidator{
				err: fmt.Errorf("validation error"),
			}
			key := dht.MakeGitObjectKey("r1", "hash1")
			err = dhtA.Store(context.Background(), key, []byte("value"))
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("validation error"))
		})
	})

	Describe(".Lookup", func() {
		BeforeEach(func() {
			dhtA, err = dht.New(context.Background(), cfg, key.PrivKey().Key(), addr)
			Expect(err).To(BeNil())
			dhtA.DHT().Validator.(record.NamespacedValidator)[dht.GitObjectNamespace] = okValidator{}
			dhtB, err = dht.New(context.Background(), cfg2, key2.PrivKey().Key(), randomAddr())
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
				key := dht.MakeGitObjectKey("r1", "hash1")
				err := dhtA.Store(context.Background(), key, []byte("value"))
				Expect(err).To(BeNil())
				val, err := dhtA.Lookup(context.Background(), "unknown_key")
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError(routing2.ErrNotFound))
				Expect(val).To(BeEmpty())
			})

			It("should return its corresponding value", func() {
				key := dht.MakeGitObjectKey("r1", "hash1")
				dhtA.Store(context.Background(), key, []byte("value"))
				val, err := dhtA.Lookup(context.Background(), key)
				Expect(err).To(BeNil())
				Expect(val).To(Equal([]byte("value")))
			})
		})

		Context("both peers lookup check", func() {
			Specify("that connected peer can also lookup the key's value", func() {
				key := dht.MakeGitObjectKey("r1", "hash1")
				err = dhtA.Store(context.Background(), key, []byte("value"))
				Expect(err).To(BeNil())
				val, err := dhtB.Lookup(context.Background(), key)
				Expect(err).To(BeNil())
				Expect(val).To(Equal([]byte("value")))
			})
		})
	})

	Describe(".Announce and .GetProviders", func() {
		BeforeEach(func() {
			dhtA, err = dht.New(context.Background(), cfg, key.PrivKey().Key(), addr)
			Expect(err).To(BeNil())
			dhtB, err = dht.New(context.Background(), cfg2, key2.PrivKey().Key(), randomAddr())
			Expect(err).To(BeNil())
			cfg.DHT.BootstrapPeers = dhtB.Addr()
			err = dhtA.Bootstrap()
			Expect(err).To(BeNil())
			time.Sleep(10 * time.Millisecond)
		})

		When("a peer annonce a key", func() {
			BeforeEach(func() {
				err = dhtA.Announce(context.Background(), []byte("key"))
			})

			It("should return no error", func() {
				Expect(err).To(BeNil())
			})

			It("should be returned as a provider on all connected peers", func() {
				addrs, err := dhtA.GetProviders(context.Background(), []byte("key"))
				Expect(err).To(BeNil())
				Expect(addrs).To(HaveLen(1))
				Expect(addrs[0].ID.Pretty()).To(Equal(dhtA.Host().ID().Pretty()))
				Expect(addrs[0].Addrs).To(BeEmpty())

				addrs, err = dhtB.GetProviders(context.Background(), []byte("key"))
				Expect(err).To(BeNil())
				Expect(addrs).To(HaveLen(1))
				Expect(addrs[0].ID.Pretty()).To(Equal(dhtA.Host().ID().Pretty()))
				Expect(addrs[0].Addrs).To(HaveLen(1))
				Expect(addrs[0].Addrs[0].String()).To(Equal(dhtA.Host().Addrs()[0].String()))
			})
		})
	})

	Describe(".GetObject", func() {
		BeforeEach(func() {
			dhtA, err = dht.New(context.Background(), cfg, key.PrivKey().Key(), addr)
			Expect(err).To(BeNil())
			dhtB, err = dht.New(context.Background(), cfg2, key2.PrivKey().Key(), randomAddr())
			Expect(err).To(BeNil())
			cfg.DHT.BootstrapPeers = dhtB.Addr()
			err = dhtA.Bootstrap()
			Expect(err).To(BeNil())
			time.Sleep(10 * time.Millisecond)
		})

		When("no providers exist", func() {
			It("should return err=ErrObjNotFound", func() {
				_, err = dhtA.GetObject(context.Background(), &dht.DHTObjectQuery{ObjectKey: []byte("key")})
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(dht.ErrObjNotFound))
			})
		})

		When("provider is the local address, but the target finder module was not registered", func() {
			BeforeEach(func() {
				err = dhtA.Announce(context.Background(), []byte("key"))
				Expect(err).To(BeNil())
			})
			It("should return err about unregistered module", func() {
				_, err = dhtA.GetObject(context.Background(), &dht.DHTObjectQuery{
					Module:    "unknown",
					ObjectKey: []byte("key"),
				})
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("finder for module `unknown` not registered"))
			})
		})

		When("provider is the local address, but the target finder module returns an error", func() {
			BeforeEach(func() {
				dhtA.RegisterObjFinder("my-finder", &testObjectFinder{err: fmt.Errorf("bad error")})
				err = dhtA.Announce(context.Background(), []byte("key"))
				Expect(err).To(BeNil())
			})
			It("should return err the finder error", func() {
				_, err = dhtA.GetObject(context.Background(), &dht.DHTObjectQuery{
					Module:    "my-finder",
					ObjectKey: []byte("key"),
				})
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("finder error: bad error"))
			})
		})

		When("provider is the local address, but the target finder module returns no error and nil value", func() {
			BeforeEach(func() {
				dhtA.RegisterObjFinder("my-finder", &testObjectFinder{})
				err = dhtA.Announce(context.Background(), []byte("key"))
				Expect(err).To(BeNil())
			})
			It("should return err=ErrObjNotFound", func() {
				_, err = dhtA.GetObject(context.Background(), &dht.DHTObjectQuery{
					Module:    "my-finder",
					ObjectKey: []byte("key"),
				})
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(dht.ErrObjNotFound))
			})
		})

		When("provider is the local address and the target finder module returns a value and no error", func() {
			BeforeEach(func() {
				dhtA.RegisterObjFinder("my-finder", &testObjectFinder{value: []byte("value")})
				err = dhtA.Announce(context.Background(), []byte("key"))
				Expect(err).To(BeNil())
			})

			It("should return value returned by the object finder", func() {
				retVal, err := dhtA.GetObject(context.Background(), &dht.DHTObjectQuery{
					Module:    "my-finder",
					ObjectKey: []byte("key"),
				})
				Expect(err).To(BeNil())
				Expect(retVal).To(Equal([]byte("value")))
			})

			XSpecify("that non-local peers can also find the key and value", func() {
				retVal, err := dhtB.GetObject(context.Background(), &dht.DHTObjectQuery{
					Module:    "my-finder",
					ObjectKey: []byte("key"),
				})
				Expect(err).To(BeNil())
				Expect(retVal).To(Equal([]byte("value")))
			})
		})
	})

	Describe(".HandleFetch", func() {
		var err error
		var mockStream *mocks.MockStream

		BeforeEach(func() {
			mockStream = mocks.NewMockStream(ctrl)
			dhtA, err = dht.New(context.Background(), cfg, key.PrivKey().Key(), addr)
			Expect(err).To(BeNil())
			mockStream.EXPECT().Close()
		})

		When("unable to read from stream", func() {
			BeforeEach(func() {
				mockStream.EXPECT().Read(gomock.Any()).Return(0, fmt.Errorf("error"))
				err = dhtA.HandleFetch(mockStream)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to read query: error"))
			})
		})

		When("unable to decode stream output", func() {
			BeforeEach(func() {
				mockStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
					p = append(p, []byte("invalid data")...)
					return len(p), nil
				})
				err = dhtA.HandleFetch(mockStream)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to decode query: msgpack: invalid code=0 decoding map length"))
			})
		})

		When("module finder for the query is not registered", func() {
			BeforeEach(func() {
				query := dht.DHTObjectQuery{
					Module:    "module-name",
					ObjectKey: []byte("object_key"),
				}
				mockStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
					copy(p, query.Bytes())
					return len(p), nil
				})
				err = dhtA.HandleFetch(mockStream)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("finder for module `module-name` not registered"))
			})
		})

		When("module finder returns error", func() {
			BeforeEach(func() {
				query := dht.DHTObjectQuery{
					Module:    "module-name",
					ObjectKey: []byte("object_key"),
				}

				mockObjFinder := mocks.NewMockObjectFinder(ctrl)
				mockObjFinder.EXPECT().FindObject(query.ObjectKey).Return(nil, fmt.Errorf("error"))
				dhtA.Finders()["module-name"] = mockObjFinder

				mockStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
					copy(p, query.Bytes())
					return len(p), nil
				})
				err = dhtA.HandleFetch(mockStream)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to find requested object (object_key): error"))
			})
		})

		When("unable to Write to stream", func() {
			var objData = []byte("object data")

			BeforeEach(func() {
				query := dht.DHTObjectQuery{
					Module:    "module-name",
					ObjectKey: []byte("object_key"),
				}

				mockObjFinder := mocks.NewMockObjectFinder(ctrl)
				mockObjFinder.EXPECT().FindObject(query.ObjectKey).Return(objData, nil)
				dhtA.Finders()["module-name"] = mockObjFinder

				mockStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
					copy(p, query.Bytes())
					return len(p), nil
				})
				mockStream.EXPECT().Write(objData).Return(0, fmt.Errorf("error"))
				err = dhtA.HandleFetch(mockStream)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to Write back find result: error"))
			})
		})

		When("object is written successfully", func() {
			var objData = []byte("object data")

			BeforeEach(func() {
				query := dht.DHTObjectQuery{
					Module:    "module-name",
					ObjectKey: []byte("object_key"),
				}

				mockObjFinder := mocks.NewMockObjectFinder(ctrl)
				mockObjFinder.EXPECT().FindObject(query.ObjectKey).Return(objData, nil)
				dhtA.Finders()["module-name"] = mockObjFinder

				mockStream.EXPECT().Read(gomock.Any()).DoAndReturn(func(p []byte) (int, error) {
					copy(p, query.Bytes())
					return len(p), nil
				})
				mockStream.EXPECT().Write(objData).Return(len(objData), nil)
				err = dhtA.HandleFetch(mockStream)
			})

			It("should return no error", func() {
				Expect(err).To(BeNil())
			})
		})
	})
})
