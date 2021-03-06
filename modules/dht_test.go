package modules_test

import (
	"encoding/base64"
	"fmt"
	"os"

	"github.com/golang/mock/gomock"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/mocks"
	"github.com/make-os/kit/modules"
	dht2 "github.com/make-os/kit/net/dht"
	"github.com/make-os/kit/net/dht/announcer"
	"github.com/make-os/kit/remote/plumbing"
	"github.com/make-os/kit/testutil"
	"github.com/make-os/kit/types/constants"
	"github.com/make-os/kit/util/errors"
	"github.com/multiformats/go-multiaddr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/robertkrimen/otto"
	"github.com/stretchr/testify/assert"
)

var _ = Describe("DHTModule", func() {
	var err error
	var cfg *config.AppConfig
	var m *modules.DHTModule
	var ctrl *gomock.Controller
	var mockDHT *mocks.MockDHT

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		ctrl = gomock.NewController(GinkgoT())
		mockDHT = mocks.NewMockDHT(ctrl)
		m = modules.NewDHTModule(cfg, mockDHT)
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".ConfigureVM", func() {
		It("should configure namespace(s) into VM context", func() {
			vm := otto.New()
			m.ConfigureVM(vm)
			val, err := vm.Get(constants.NamespaceDHT)
			Expect(err).To(BeNil())
			Expect(val.IsObject()).To(BeTrue())
		})
	})

	Describe(".Store", func() {
		It("should panic if unable to store data", func() {
			mockDHT.EXPECT().Store(gomock.Any(), dht2.MakeKey("key"), []byte("val")).Return(fmt.Errorf("error"))
			err := &errors.ReqError{Code: "server_err", HttpCode: 500, Msg: "error", Field: "key"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Store("key", "val")
			})
		})

		It("should not panic on success", func() {
			mockDHT.EXPECT().Store(gomock.Any(), dht2.MakeKey("key"), []byte("val")).Return(nil)
			Expect(func() { m.Store("key", "val") }).ToNot(Panic())
		})
	})

	Describe(".Lookup", func() {
		It("should panic if unable to lookup key", func() {
			mockDHT.EXPECT().Lookup(gomock.Any(), dht2.MakeKey("key")).Return(nil, fmt.Errorf("error"))
			err := &errors.ReqError{Code: "server_err", HttpCode: 500, Msg: "error", Field: "key"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Lookup("key")
			})
		})

		It("should return base64-encoded data on success", func() {
			mockDHT.EXPECT().Lookup(gomock.Any(), dht2.MakeKey("key")).Return([]byte("abc"), nil)
			data := m.Lookup("key")
			Expect(data).ToNot(BeEmpty())
			Expect(data).To(Equal(base64.StdEncoding.EncodeToString([]byte("abc"))))
		})
	})

	Describe(".Announce", func() {
		It("should announce the key", func() {
			mockDHT.EXPECT().Announce(announcer.ObjTypeAny, "", []byte("key"), nil)
			m.Announce("key")
		})
	})

	Describe(".GetRepoObjectProviders", func() {
		It("should panic if object key is not SHA1 and not a valid hex string", func() {
			err := &errors.ReqError{Code: "invalid_param", HttpCode: 400, Msg: "invalid object key", Field: "hash"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetRepoObjectProviders("invalid_hex")
			})
		})

		It("should attempt to get providers from the DHT panic on error", func() {
			objHash := "8be2869859870fbdf9cb1265e27f202363d6e618"
			key := plumbing.HashToBytes(objHash)
			mockDHT.EXPECT().GetProviders(gomock.Any(), key).Return(nil, fmt.Errorf("error"))
			err := &errors.ReqError{Code: "server_err", HttpCode: 500, Msg: "error", Field: "key"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetRepoObjectProviders(objHash)
			})
		})

		It("should return provider peers when attempt to get providers from the DHT succeeds", func() {
			objHash := "8be2869859870fbdf9cb1265e27f202363d6e618"
			key := plumbing.HashToBytes(objHash)
			peerID := peer.ID("peer-id")
			mockDHT.EXPECT().GetProviders(gomock.Any(), key).Return([]peer.AddrInfo{
				{ID: peerID, Addrs: []multiaddr.Multiaddr{multiaddr.StringCast("/ip4/127.0.0.1/tcp/5000")}},
			}, nil)
			res := m.GetRepoObjectProviders(objHash)
			Expect(res).To(HaveLen(1))
			Expect(res[0]["id"]).To(Equal(peerID.String()))
			Expect(res[0]["addresses"]).To(HaveLen(1))
			Expect(res[0]["addresses"].([]string)[0]).To(Equal("/ip4/127.0.0.1/tcp/5000"))
		})
	})

	Describe(".GetProviders", func() {
		It("should panic if unable to get providers", func() {
			err := &errors.ReqError{Code: "server_err", HttpCode: 500, Msg: "error", Field: "key"}
			mockDHT.EXPECT().GetProviders(gomock.Any(), []byte("key")).Return(nil, fmt.Errorf("error"))
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetProviders("key")
			})
		})

		It("should return provider peers on success", func() {
			peerID := peer.ID("peer-id")
			mockDHT.EXPECT().GetProviders(gomock.Any(), []byte("key")).Return([]peer.AddrInfo{
				{ID: peerID, Addrs: []multiaddr.Multiaddr{multiaddr.StringCast("/ip4/127.0.0.1/tcp/5000")}},
			}, nil)
			res := m.GetProviders("key")
			Expect(res).To(HaveLen(1))
			Expect(res[0]["id"]).To(Equal(peerID.String()))
			Expect(res[0]["addresses"]).To(HaveLen(1))
			Expect(res[0]["addresses"].([]string)[0]).To(Equal("/ip4/127.0.0.1/tcp/5000"))
		})
	})

	Describe(".GetPeers", func() {
		It("should return DHT peers", func() {
			expected := []string{"peer1", "peer2"}
			mockDHT.EXPECT().Peers().Return(expected)
			peers := m.GetPeers()
			Expect(peers).To(Equal(expected))
		})
	})
})
