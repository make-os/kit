package dht

// import (
// 	"context"
// 	"fmt"
// 	"os"
// 	"time"

// 	"github.com/makeos/mosdef/types"
// 	"github.com/makeos/mosdef/types/mocks"
// 	"github.com/multiformats/go-multiaddr"
// 	"github.com/tendermint/tendermint/p2p"

// 	"github.com/golang/mock/gomock"
// 	"github.com/libp2p/go-libp2p-core/peer"
// 	"github.com/phayes/freeport"

// 	"github.com/makeos/mosdef/config"
// 	"github.com/makeos/mosdef/crypto"
// 	"github.com/makeos/mosdef/testutil"
// 	. "github.com/onsi/ginkgo"
// 	. "github.com/onsi/gomega"
// )

// func randomAddr() string {
// 	port, err := freeport.GetFreePort()
// 	if err != nil {
// 		panic(err)
// 	}
// 	return fmt.Sprintf("127.0.0.1:%d", port)
// }

// func connect(node1, node2 *DHT) {
// 	node2AddrInfo := peer.AddrInfo{ID: node2.host.ID(), Addrs: node2.host.Addrs()}
// 	err := node1.host.Connect(context.Background(), node2AddrInfo)
// 	if err != nil {
// 		panic(err)
// 	}
// }

// var _ = Describe("App", func() {
// 	var err error
// 	var addr string
// 	var cfg, cfg2 *config.AppConfig
// 	var key = crypto.NewKeyFromIntSeed(1)
// 	var ctrl *gomock.Controller
// 	var key2 = crypto.NewKeyFromIntSeed(2)

// 	BeforeEach(func() {
// 		ctrl = gomock.NewController(GinkgoT())
// 		cfg, err = testutil.SetTestCfg()
// 		Expect(err).To(BeNil())
// 		cfg2, err = testutil.SetTestCfg()
// 		Expect(err).To(BeNil())
// 	})

// 	BeforeEach(func() {
// 		port := freeport.GetPort()
// 		addr = fmt.Sprintf("127.0.0.1:%d", port)
// 	})

// 	AfterEach(func() {
// 		ctrl.Finish()
// 		err = os.RemoveAll(cfg.DataDir())
// 		Expect(err).To(BeNil())
// 		err = os.RemoveAll(cfg2.DataDir())
// 		Expect(err).To(BeNil())
// 	})

// 	Describe(".New", func() {

// 		When("address format is not valid", func() {
// 			It("should return err", func() {
// 				_, err = New(context.Background(), cfg, key.PrivKey().Key(), "invalid")
// 				Expect(err).ToNot(BeNil())
// 				Expect(err.Error()).To(Equal("invalid address: address invalid: missing port in address"))
// 			})
// 		})

// 		When("unable to create host", func() {
// 			It("should return err", func() {
// 				_, err = New(context.Background(), cfg, key.PrivKey().Key(), "0.1.1.1.0:999999")
// 				Expect(err).ToNot(BeNil())
// 				Expect(err.Error()).To(ContainSubstring("failed to create host"))
// 			})
// 		})

// 		When("no problem", func() {
// 			It("should return nil", func() {
// 				_, err = New(context.Background(), cfg, key.PrivKey().Key(), addr)
// 				Expect(err).To(BeNil())
// 			})
// 		})
// 	})

// 	Describe(".Store & .Lookup", func() {
// 		var node1, node2 *DHT
// 		var err error
// 		var remotePeerID = p2p.ID("xyz")

// 		BeforeEach(func() {
// 			node1, err = New(context.Background(), cfg, key.PrivKey().Key(), randomAddr())
// 			Expect(err).To(BeNil())
// 			node2, err = New(context.Background(), cfg2, key2.PrivKey().Key(), randomAddr())
// 			Expect(err).To(BeNil())
// 			node2Port, _ := node2.host.Addrs()[0].ValueForProtocol(multiaddr.P_TCP)
// 			remotePeerMock := mocks.NewMockPeer(ctrl)
// 			remotePeerMock.EXPECT().ID().Return(remotePeerID).AnyTimes()
// 			err = node1.connect(&types.DHTInfo{
// 				ID:      node2.host.ID().String(),
// 				Address: "127.0.0.1",
// 				Port:    node2Port,
// 			}, remotePeerMock)
// 			Expect(err).To(BeNil())
// 		})

// 		Context("store and lookup on a single node", func() {
// 			BeforeEach(func() {
// 				err = node1.Store(context.Background(), "key1", []byte("value"))
// 				Expect(err).To(BeNil())
// 			})

// 			It("should return no error", func() {
// 				bz, err := node1.Lookup(context.Background(), "key1")
// 				Expect(err).To(BeNil())
// 				Expect(bz).To(Equal([]byte("value")))
// 			})
// 		})

// 		Context("store on node 1 and lookup on node 2", func() {
// 			BeforeEach(func() {
// 				err = node1.Store(context.Background(), "key1", []byte("value"))
// 				Expect(err).To(BeNil())
// 			})

// 			It("should return expected value", func() {
// 				bz, err := node2.Lookup(context.Background(), "key1")
// 				Expect(err).To(BeNil())
// 				Expect(bz).To(Equal([]byte("value")))
// 			})
// 		})
// 	})

// 	Describe(".Announce & .GetProviders", func() {
// 		var node1, node2 *DHT
// 		var err error
// 		var remotePeerID = p2p.ID("xyz")

// 		BeforeEach(func() {
// 			node1, err = New(context.Background(), cfg, key.PrivKey().Key(), randomAddr())
// 			Expect(err).To(BeNil())
// 			node2, err = New(context.Background(), cfg2, key2.PrivKey().Key(), randomAddr())
// 			Expect(err).To(BeNil())
// 			node2Port, _ := node2.host.Addrs()[0].ValueForProtocol(multiaddr.P_TCP)
// 			remotePeerMock := mocks.NewMockPeer(ctrl)
// 			remotePeerMock.EXPECT().ID().Return(remotePeerID).AnyTimes()
// 			err = node1.connect(&types.DHTInfo{
// 				ID:      node2.host.ID().String(),
// 				Address: "127.0.0.1",
// 				Port:    node2Port,
// 			}, remotePeerMock)
// 			Expect(err).To(BeNil())
// 		})

// 		Context("announce and get providers on a single node", func() {
// 			BeforeEach(func() {
// 				err = node1.Annonce(context.Background(), []byte("key1"))
// 				Expect(err).To(BeNil())
// 			})

// 			It("should return 1 address", func() {
// 				addrs, err := node1.GetProviders(context.Background(), []byte("key1"))
// 				Expect(err).To(BeNil())
// 				Expect(addrs).To(HaveLen(1))
// 			})
// 		})

// 		Context("announce on node 1 and get providers on node 2", func() {
// 			BeforeEach(func() {
// 				err = node1.Annonce(context.Background(), []byte("key1"))
// 				Expect(err).To(BeNil())
// 			})

// 			It("should return 1 address", func() {
// 				addrs, err := node2.GetProviders(context.Background(), []byte("key1"))
// 				Expect(err).To(BeNil())
// 				Expect(addrs).To(HaveLen(1))
// 			})
// 		})
// 	})

// 	Describe(".GetObject", func() {
// 		var node1, node2 *DHT
// 		var err error
// 		var remotePeerID = p2p.ID("xyz")
// 		var objKey = []byte("object_key")

// 		BeforeEach(func() {
// 			node1, err = New(context.Background(), cfg, key.PrivKey().Key(), randomAddr())
// 			Expect(err).To(BeNil())
// 			node2, err = New(context.Background(), cfg2, key2.PrivKey().Key(), randomAddr())
// 			Expect(err).To(BeNil())
// 			node2Port, _ := node2.host.Addrs()[0].ValueForProtocol(multiaddr.P_TCP)
// 			remotePeerMock := mocks.NewMockPeer(ctrl)
// 			remotePeerMock.EXPECT().ID().Return(remotePeerID).AnyTimes()
// 			err = node1.connect(&types.DHTInfo{
// 				ID:      node2.host.ID().String(),
// 				Address: "127.0.0.1",
// 				Port:    node2Port,
// 			}, remotePeerMock)
// 			Expect(err).To(BeNil())
// 		})

// 		When("no provider is found for the query key", func() {
// 			BeforeEach(func() {
// 				query := &types.DHTObjectQuery{Module: "files", ObjectKey: objKey}
// 				_, err = node1.GetObject(context.Background(), query)
// 			})

// 			It("should return error=object not found", func() {
// 				Expect(err).ToNot(BeNil())
// 				Expect(err.Error()).To(Equal("object not found"))
// 			})
// 		})

// 		When("provider is found but no registered finder for the query module", func() {
// 			BeforeEach(func() {
// 				node1.Annonce(context.Background(), objKey)
// 				query := &types.DHTObjectQuery{Module: "files", ObjectKey: objKey}
// 				_, err = node1.GetObject(context.Background(), query)
// 			})

// 			It("should return error=", func() {
// 				Expect(err).ToNot(BeNil())
// 				Expect(err.Error()).To(Equal("finder for module `files` not registered"))
// 			})
// 		})

// 		When("provider is found and query finder returns err", func() {
// 			BeforeEach(func() {
// 				mockFinder := mocks.NewMockObjectFinder(ctrl)
// 				mockFinder.EXPECT().FindObject(objKey).Return(nil, fmt.Errorf("bad error"))
// 				node1.RegisterObjFinder("files", mockFinder)
// 				node1.Annonce(context.Background(), objKey)
// 				query := &types.DHTObjectQuery{Module: "files", ObjectKey: objKey}
// 				_, err = node1.GetObject(context.Background(), query)
// 			})

// 			It("should return error=finder error: bad error", func() {
// 				Expect(err).ToNot(BeNil())
// 				Expect(err.Error()).To(Equal("finder error: bad error"))
// 			})
// 		})

// 		When("provider is found and query finder returns nil reader", func() {
// 			BeforeEach(func() {
// 				mockFinder := mocks.NewMockObjectFinder(ctrl)
// 				mockFinder.EXPECT().FindObject(objKey).Return(nil, nil)
// 				node1.RegisterObjFinder("files", mockFinder)
// 				node1.Annonce(context.Background(), objKey)
// 				query := &types.DHTObjectQuery{Module: "files", ObjectKey: objKey}
// 				_, err = node1.GetObject(context.Background(), query)
// 			})

// 			It("should return error=object not found", func() {
// 				Expect(err).ToNot(BeNil())
// 				Expect(err.Error()).To(Equal("object not found"))
// 			})
// 		})

// 		When("provider is found and query finder returns a non-nil reader", func() {
// 			var out []byte
// 			var val = []byte("value")

// 			BeforeEach(func() {
// 				mockFinder := mocks.NewMockObjectFinder(ctrl)
// 				mockFinder.EXPECT().FindObject(objKey).Return(val, nil)
// 				node1.RegisterObjFinder("files", mockFinder)
// 				node1.Annonce(context.Background(), objKey)
// 				query := &types.DHTObjectQuery{Module: "files", ObjectKey: objKey}
// 				out, err = node1.GetObject(context.Background(), query)
// 			})

// 			It("should return no err", func() {
// 				Expect(err).To(BeNil())
// 				Expect(string(out)).To(Equal(string(val)))
// 			})
// 		})

// 		When("provider is not the calling node", func() {
// 			var val = []byte("value")
// 			var out []byte

// 			BeforeEach(func() {
// 				mockFinder := mocks.NewMockObjectFinder(ctrl)
// 				mockFinder.EXPECT().FindObject(objKey).Return(val, nil)
// 				node1.RegisterObjFinder("files", mockFinder)
// 				node1.Annonce(context.Background(), objKey)
// 				time.Sleep(5 * time.Millisecond)
// 				query := &types.DHTObjectQuery{Module: "files", ObjectKey: objKey}
// 				out, err = node2.GetObject(context.Background(), query)
// 			})

// 			It("should return no error", func() {
// 				Expect(err).To(BeNil())
// 			})

// 			Specify("that expected output should match returned finder value", func() {
// 				Expect(out).To(Equal(val))
// 			})
// 		})
// 	})

// 	Describe(".Peers", func() {
// 		When("two nodes are connected", func() {
// 			var node1, node2 *DHT
// 			var err error
// 			var remotePeerID = p2p.ID("xyz")

// 			BeforeEach(func() {
// 				node1, err = New(context.Background(), cfg, key.PrivKey().Key(), randomAddr())
// 				Expect(err).To(BeNil())
// 				node2, err = New(context.Background(), cfg2, key2.PrivKey().Key(), randomAddr())
// 				Expect(err).To(BeNil())
// 				node2Port, _ := node2.host.Addrs()[0].ValueForProtocol(multiaddr.P_TCP)
// 				remotePeerMock := mocks.NewMockPeer(ctrl)
// 				remotePeerMock.EXPECT().ID().Return(remotePeerID).AnyTimes()
// 				err = node1.connect(&types.DHTInfo{
// 					ID:      node2.host.ID().String(),
// 					Address: "127.0.0.1",
// 					Port:    node2Port,
// 				}, remotePeerMock)
// 				Expect(err).To(BeNil())
// 			})

// 			It("should return one peer", func() {
// 				Expect(node1.Peers()).To(HaveLen(1))
// 				Expect(node2.Peers()).To(HaveLen(1))
// 			})
// 		})
// 	})
// })
