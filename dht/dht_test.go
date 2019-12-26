package dht

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/types/mocks"
	"github.com/multiformats/go-multiaddr"
	"github.com/tendermint/tendermint/p2p"

	"github.com/golang/mock/gomock"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/phayes/freeport"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/testutil"
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

	Describe(".connect", func() {
		var node1, node2 *DHT
		var err error

		BeforeEach(func() {
			node1, err = New(context.Background(), cfg, key.PrivKey().Key(), randomAddr())
			Expect(err).To(BeNil())
		})

		When("dht info invalid peer id", func() {
			It("should return error", func() {
				peerMock := mocks.NewMockPeer(ctrl)
				err = node1.connect(&types.DHTInfo{ID: "invalid"}, peerMock)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("bad peer id: input isn't valid multihash"))
			})
		})

		When("unable to connect due to self dial", func() {
			It("should return error", func() {
				peerMock := mocks.NewMockPeer(ctrl)
				err = node1.connect(&types.DHTInfo{
					ID:      node1.host.ID().String(),
					Address: "127.0.0.1",
					Port:    "1999",
				}, peerMock)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("failed to connect"))
			})
		})

		When("connection is successful", func() {
			remotePeerID := p2p.ID("xyz")

			BeforeEach(func() {
				remotePeerMock := mocks.NewMockPeer(ctrl)
				remotePeerMock.EXPECT().ID().Return(remotePeerID)

				node2, err = New(context.Background(), cfg2, key2.PrivKey().Key(), randomAddr())
				Expect(err).To(BeNil())
				node2Port, _ := node2.host.Addrs()[0].ValueForProtocol(multiaddr.P_TCP)

				err = node1.connect(&types.DHTInfo{
					ID:      node2.host.ID().String(),
					Address: "127.0.0.1",
					Port:    node2Port,
				}, remotePeerMock)
			})

			It("should not return error", func() {
				Expect(err).To(BeNil())
			})

			Specify("that the remote peer was added to the connected peers map", func() {
				Expect(node1.connectedPeers.Len()).To(Equal(1))
				Expect(node1.connectedPeers.Contains(remotePeerID)).To(BeTrue())
			})
		})
	})

	Describe(".Receive", func() {
		var node1 *DHT
		var err error
		// var remotePeerID = p2p.ID("xyz")
		var remotePeerMock *mocks.MockPeer

		BeforeEach(func() {
			node1, err = New(context.Background(), cfg, key.PrivKey().Key(), randomAddr())
			Expect(err).To(BeNil())
			remotePeerMock = mocks.NewMockPeer(ctrl)
		})

		When("message could not be decoded", func() {
			BeforeEach(func() {
				node1.Receive(0x0, remotePeerMock, []byte("invalid"))
			})

			It("should return error", func() {
				Expect(node1.receiveErr).ToNot(BeNil())
				Expect(node1.receiveErr.Error()).To(ContainSubstring("failed to decode message"))
			})
		})

		When("message could not be decoded", func() {
			BeforeEach(func() {
				msg := &types.DHTInfo{}
				cfg.DHT.Address = ""
				node1.Receive(0x0, remotePeerMock, msg.Bytes())
			})

			It("should return error", func() {
				Expect(node1.receiveErr).ToNot(BeNil())
				Expect(node1.receiveErr.Error()).To(ContainSubstring("failed to parse local peer dht address"))
			})
		})

		When("unable to send DHT info", func() {
			BeforeEach(func() {
				msg := &types.DHTInfo{}
				cfg.DHT.Address = "127.0.0.1:8001"
				remotePeerMock.EXPECT().Send(gomock.Any(), gomock.Any())
				node1.Receive(0x0, remotePeerMock, msg.Bytes())
			})

			It("should return error", func() {
				Expect(node1.receiveErr).ToNot(BeNil())
				Expect(node1.receiveErr.Error()).To(ContainSubstring("failed to send DHT Info"))
			})
		})
	})

	Describe(".requestDHTInfo", func() {
		var node1 *DHT
		var err error
		remotePeerID := p2p.ID("xyz")

		BeforeEach(func() {
			node1, err = New(context.Background(), cfg, key.PrivKey().Key(), randomAddr())
			Expect(err).To(BeNil())
		})

		When("peer is already connected", func() {
			BeforeEach(func() {
				remotePeerMock := mocks.NewMockPeer(ctrl)
				remotePeerMock.EXPECT().ID().Return(remotePeerID)
				node1.connectedPeers.Add(remotePeerID, nil)
				err = node1.requestDHTInfo(remotePeerMock)
			})

			It("should return no error", func() {
				Expect(err).To(BeNil())
			})
		})

		When("peer is not already connected, it should send dht info request", func() {
			BeforeEach(func() {
				remotePeerMock := mocks.NewMockPeer(ctrl)
				remotePeerMock.EXPECT().ID().Return(remotePeerID).AnyTimes()
				remotePeerMock.EXPECT().Send(DHTReactorChannel, types.BareDHTInfo().Bytes()).Return(true)
				err = node1.requestDHTInfo(remotePeerMock)
			})

			It("should return no error", func() {
				Expect(err).To(BeNil())
			})
		})

		When("peer is not already connected and failed to send dht info request", func() {
			BeforeEach(func() {
				remotePeerMock := mocks.NewMockPeer(ctrl)
				remotePeerMock.EXPECT().ID().Return(remotePeerID).AnyTimes()
				remotePeerMock.EXPECT().Send(DHTReactorChannel, types.BareDHTInfo().Bytes()).Return(false)
				err = node1.requestDHTInfo(remotePeerMock)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to send request"))
			})
		})
	})

	Describe(".verifyConnections", func() {
		var node1, node2 *DHT
		var err error
		remotePeerID := p2p.ID("xyz")

		When("node2 is a connected peer of node1", func() {
			BeforeEach(func() {
				node1, err = New(context.Background(), cfg, key.PrivKey().Key(), randomAddr())
				Expect(err).To(BeNil())
				node2, err = New(context.Background(), cfg2, key2.PrivKey().Key(), randomAddr())
				Expect(err).To(BeNil())

				node2Port, _ := node2.host.Addrs()[0].ValueForProtocol(multiaddr.P_TCP)

				remotePeerMock := mocks.NewMockPeer(ctrl)
				remotePeerMock.EXPECT().ID().Return(remotePeerID)

				err = node1.connect(&types.DHTInfo{
					ID:      node2.host.ID().String(),
					Address: "127.0.0.1",
					Port:    node2Port,
				}, remotePeerMock)
				Expect(err).To(BeNil())
				Expect(node1.connectedPeers.Len()).To(Equal(1))
				node1.verifyConnections(context.Background())
			})

			Specify("that remote peer is still in the cache", func() {
				Expect(node1.connectedPeers.Len()).To(Equal(1))
			})
		})

		When("node2 is a connected peer of node1", func() {
			BeforeEach(func() {
				node1, err = New(context.Background(), cfg, key.PrivKey().Key(), randomAddr())
				Expect(err).To(BeNil())
				node2, err = New(context.Background(), cfg2, key2.PrivKey().Key(), randomAddr())
				Expect(err).To(BeNil())

				node2Port, _ := node2.host.Addrs()[0].ValueForProtocol(multiaddr.P_TCP)

				remotePeerMock := mocks.NewMockPeer(ctrl)
				remotePeerMock.EXPECT().ID().Return(remotePeerID)

				err = node1.connect(&types.DHTInfo{
					ID:      node2.host.ID().String(),
					Address: "127.0.0.1",
					Port:    node2Port,
				}, remotePeerMock)
				Expect(err).To(BeNil())
				Expect(node1.connectedPeers.Len()).To(Equal(1))
			})

			Specify("that remote peer is still in the cache", func() {
				node1.host.Network().Conns()[0].Close()
				node1.verifyConnections(context.Background())
				Expect(node1.connectedPeers.Len()).To(Equal(0))
			})
		})
	})

	Describe(".isConnected", func() {
		var node1 *DHT
		var err error
		var remotePeerID = p2p.ID("xyz")
		var connected bool

		When("remote peer exist in the connectedPeers cache", func() {
			BeforeEach(func() {
				node1, err = New(context.Background(), cfg, key.PrivKey().Key(), randomAddr())
				Expect(err).To(BeNil())
				remotePeerMock := mocks.NewMockPeer(ctrl)
				remotePeerMock.EXPECT().ID().Return(remotePeerID)
				node1.connectedPeers.Add(remotePeerID, nil)
				connected = node1.isConnected(remotePeerMock)
			})

			It("should return true", func() {
				Expect(connected).To(BeTrue())
			})
		})

		When("remote peer exist in the connectedPeers cache", func() {
			BeforeEach(func() {
				node1, err = New(context.Background(), cfg, key.PrivKey().Key(), randomAddr())
				Expect(err).To(BeNil())
				remotePeerMock := mocks.NewMockPeer(ctrl)
				remotePeerMock.EXPECT().ID().Return(remotePeerID)
				connected = node1.isConnected(remotePeerMock)
			})

			It("should return false", func() {
				Expect(connected).To(BeFalse())
			})
		})
	})

	Describe(".RemovePeer", func() {
		var node1, node2 *DHT
		var err error
		var remotePeerID = p2p.ID("xyz")

		When("node2 is a connected peer of node1", func() {
			BeforeEach(func() {
				node1, err = New(context.Background(), cfg, key.PrivKey().Key(), randomAddr())
				Expect(err).To(BeNil())
				node2, err = New(context.Background(), cfg2, key2.PrivKey().Key(), randomAddr())
				Expect(err).To(BeNil())

				node2Port, _ := node2.host.Addrs()[0].ValueForProtocol(multiaddr.P_TCP)

				remotePeerMock := mocks.NewMockPeer(ctrl)
				remotePeerMock.EXPECT().ID().Return(remotePeerID).AnyTimes()

				err = node1.connect(&types.DHTInfo{
					ID:      node2.host.ID().String(),
					Address: "127.0.0.1",
					Port:    node2Port,
				}, remotePeerMock)
				Expect(err).To(BeNil())
				Expect(node1.connectedPeers.Len()).To(Equal(1))
				Expect(len(node1.host.Network().Conns())).To(Equal(1))
				node1.RemovePeer(remotePeerMock, nil)
			})

			Specify("that the remote peer is not connected", func() {
				Expect(len(node1.host.Network().Conns())).To(Equal(0))
			})

			Specify("that remote peer is removed from connected peer cache", func() {
				// node1.host.Network().Conns()
				Expect(node1.connectedPeers.Len()).To(Equal(0))
			})
		})
	})

	Describe(".Store & .Lookup", func() {
		var node1, node2 *DHT
		var err error
		var remotePeerID = p2p.ID("xyz")

		BeforeEach(func() {
			node1, err = New(context.Background(), cfg, key.PrivKey().Key(), randomAddr())
			Expect(err).To(BeNil())
			node2, err = New(context.Background(), cfg2, key2.PrivKey().Key(), randomAddr())
			Expect(err).To(BeNil())
			node2Port, _ := node2.host.Addrs()[0].ValueForProtocol(multiaddr.P_TCP)
			remotePeerMock := mocks.NewMockPeer(ctrl)
			remotePeerMock.EXPECT().ID().Return(remotePeerID).AnyTimes()
			err = node1.connect(&types.DHTInfo{
				ID:      node2.host.ID().String(),
				Address: "127.0.0.1",
				Port:    node2Port,
			}, remotePeerMock)
			Expect(err).To(BeNil())
		})

		Context("store and lookup on a single node", func() {
			BeforeEach(func() {
				err = node1.Store(context.Background(), "key1", []byte("value"))
				Expect(err).To(BeNil())
			})

			It("should return no error", func() {
				bz, err := node1.Lookup(context.Background(), "key1")
				Expect(err).To(BeNil())
				Expect(bz).To(Equal([]byte("value")))
			})
		})

		Context("store on node 1 and lookup on node 2", func() {
			BeforeEach(func() {
				err = node1.Store(context.Background(), "key1", []byte("value"))
				Expect(err).To(BeNil())
			})

			It("should return expected value", func() {
				bz, err := node2.Lookup(context.Background(), "key1")
				Expect(err).To(BeNil())
				Expect(bz).To(Equal([]byte("value")))
			})
		})
	})

	Describe(".Announce & .GetProviders", func() {
		var node1, node2 *DHT
		var err error
		var remotePeerID = p2p.ID("xyz")

		BeforeEach(func() {
			node1, err = New(context.Background(), cfg, key.PrivKey().Key(), randomAddr())
			Expect(err).To(BeNil())
			node2, err = New(context.Background(), cfg2, key2.PrivKey().Key(), randomAddr())
			Expect(err).To(BeNil())
			node2Port, _ := node2.host.Addrs()[0].ValueForProtocol(multiaddr.P_TCP)
			remotePeerMock := mocks.NewMockPeer(ctrl)
			remotePeerMock.EXPECT().ID().Return(remotePeerID).AnyTimes()
			err = node1.connect(&types.DHTInfo{
				ID:      node2.host.ID().String(),
				Address: "127.0.0.1",
				Port:    node2Port,
			}, remotePeerMock)
			Expect(err).To(BeNil())
		})

		Context("announce and get providers on a single node", func() {
			BeforeEach(func() {
				err = node1.Annonce(context.Background(), []byte("key1"))
				Expect(err).To(BeNil())
			})

			It("should return 1 address", func() {
				addrs, err := node1.GetProviders(context.Background(), []byte("key1"))
				Expect(err).To(BeNil())
				Expect(addrs).To(HaveLen(1))
			})
		})

		Context("announce on node 1 and get providers on node 2", func() {
			BeforeEach(func() {
				err = node1.Annonce(context.Background(), []byte("key1"))
				Expect(err).To(BeNil())
			})

			It("should return 1 address", func() {
				addrs, err := node2.GetProviders(context.Background(), []byte("key1"))
				Expect(err).To(BeNil())
				Expect(addrs).To(HaveLen(1))
			})
		})
	})

	Describe(".GetObject", func() {
		var node1, node2 *DHT
		var err error
		var remotePeerID = p2p.ID("xyz")
		var objKey = []byte("object_key")

		BeforeEach(func() {
			node1, err = New(context.Background(), cfg, key.PrivKey().Key(), randomAddr())
			Expect(err).To(BeNil())
			node2, err = New(context.Background(), cfg2, key2.PrivKey().Key(), randomAddr())
			Expect(err).To(BeNil())
			node2Port, _ := node2.host.Addrs()[0].ValueForProtocol(multiaddr.P_TCP)
			remotePeerMock := mocks.NewMockPeer(ctrl)
			remotePeerMock.EXPECT().ID().Return(remotePeerID).AnyTimes()
			err = node1.connect(&types.DHTInfo{
				ID:      node2.host.ID().String(),
				Address: "127.0.0.1",
				Port:    node2Port,
			}, remotePeerMock)
			Expect(err).To(BeNil())
		})

		When("no provider is found for the query key", func() {
			BeforeEach(func() {
				query := &types.DHTObjectQuery{Module: "files", ObjectKey: objKey}
				_, err = node1.GetObject(context.Background(), query)
			})

			It("should return error=object not found", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("object not found"))
			})
		})

		When("provider is found but no registered finder for the query module", func() {
			BeforeEach(func() {
				node1.Annonce(context.Background(), objKey)
				query := &types.DHTObjectQuery{Module: "files", ObjectKey: objKey}
				_, err = node1.GetObject(context.Background(), query)
			})

			It("should return error=", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("finder for module `files` not registered"))
			})
		})

		When("provider is found and query finder returns err", func() {
			BeforeEach(func() {
				mockFinder := mocks.NewMockObjectFinder(ctrl)
				mockFinder.EXPECT().FindObject(objKey).Return(nil, fmt.Errorf("bad error"))
				node1.RegisterObjFinder("files", mockFinder)
				node1.Annonce(context.Background(), objKey)
				query := &types.DHTObjectQuery{Module: "files", ObjectKey: objKey}
				_, err = node1.GetObject(context.Background(), query)
			})

			It("should return error=finder error: bad error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("finder error: bad error"))
			})
		})

		When("provider is found and query finder returns nil reader", func() {
			BeforeEach(func() {
				mockFinder := mocks.NewMockObjectFinder(ctrl)
				mockFinder.EXPECT().FindObject(objKey).Return(nil, nil)
				node1.RegisterObjFinder("files", mockFinder)
				node1.Annonce(context.Background(), objKey)
				query := &types.DHTObjectQuery{Module: "files", ObjectKey: objKey}
				_, err = node1.GetObject(context.Background(), query)
			})

			It("should return error=object not found", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("object not found"))
			})
		})

		When("provider is found and query finder returns a non-nil reader", func() {
			var out []byte
			var val = []byte("value")

			BeforeEach(func() {
				mockFinder := mocks.NewMockObjectFinder(ctrl)
				mockFinder.EXPECT().FindObject(objKey).Return(val, nil)
				node1.RegisterObjFinder("files", mockFinder)
				node1.Annonce(context.Background(), objKey)
				query := &types.DHTObjectQuery{Module: "files", ObjectKey: objKey}
				out, err = node1.GetObject(context.Background(), query)
			})

			It("should return no err", func() {
				Expect(err).To(BeNil())
				Expect(string(out)).To(Equal(string(val)))
			})
		})

		When("provider is not the calling node", func() {
			var val = []byte("value")
			var out []byte

			BeforeEach(func() {
				mockFinder := mocks.NewMockObjectFinder(ctrl)
				mockFinder.EXPECT().FindObject(objKey).Return(val, nil)
				node1.RegisterObjFinder("files", mockFinder)
				node1.Annonce(context.Background(), objKey)
				time.Sleep(5 * time.Millisecond)
				query := &types.DHTObjectQuery{Module: "files", ObjectKey: objKey}
				out, err = node2.GetObject(context.Background(), query)
			})

			It("should return no error", func() {
				Expect(err).To(BeNil())
			})

			Specify("that expected output should match returned finder value", func() {
				Expect(out).To(Equal(val))
			})
		})
	})

	Describe(".Peers", func() {
		When("two nodes are connected", func() {
			var node1, node2 *DHT
			var err error
			var remotePeerID = p2p.ID("xyz")

			BeforeEach(func() {
				node1, err = New(context.Background(), cfg, key.PrivKey().Key(), randomAddr())
				Expect(err).To(BeNil())
				node2, err = New(context.Background(), cfg2, key2.PrivKey().Key(), randomAddr())
				Expect(err).To(BeNil())
				node2Port, _ := node2.host.Addrs()[0].ValueForProtocol(multiaddr.P_TCP)
				remotePeerMock := mocks.NewMockPeer(ctrl)
				remotePeerMock.EXPECT().ID().Return(remotePeerID).AnyTimes()
				err = node1.connect(&types.DHTInfo{
					ID:      node2.host.ID().String(),
					Address: "127.0.0.1",
					Port:    node2Port,
				}, remotePeerMock)
				Expect(err).To(BeNil())
			})

			It("should return one peer", func() {
				Expect(node1.Peers()).To(HaveLen(1))
				Expect(node2.Peers()).To(HaveLen(1))
			})
		})
	})
})
