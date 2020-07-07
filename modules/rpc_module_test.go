package modules_test

import (
	"net"
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/robertkrimen/otto"
	"github.com/stretchr/testify/assert"
	"gitlab.com/makeos/mosdef/api/rpc/client"
	"gitlab.com/makeos/mosdef/config"
	mocks2 "gitlab.com/makeos/mosdef/mocks/rpc"
	"gitlab.com/makeos/mosdef/modules"
	"gitlab.com/makeos/mosdef/testutil"
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/util"
)

var _ = Describe("RPCModule", func() {
	var err error
	var cfg *config.AppConfig
	var m *modules.RPCModule
	var ctrl *gomock.Controller
	var mockServer *mocks2.MockServer

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		ctrl = gomock.NewController(GinkgoT())
		mockServer = mocks2.NewMockServer(ctrl)
		m = modules.NewRPCModule(cfg, mockServer)
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".ConsoleOnlyMode", func() {
		It("should return false", func() {
			Expect(m.ConsoleOnlyMode()).To(BeTrue())
		})
	})

	Describe(".ConfigureVM", func() {
		It("should configure namespace(s) into VM context", func() {
			vm := otto.New()
			m.ConfigureVM(vm)
			val, err := vm.Get(constants.NamespaceRPC)
			Expect(err).To(BeNil())
			Expect(val.IsObject()).To(BeTrue())
		})
	})

	Describe(".IsRunning", func() {
		It("should return false server is not initialized", func() {
			m := modules.NewRPCModule(cfg, nil)
			Expect(m.IsRunning()).To(BeFalse())
		})

		It("should return true server is initialized and running", func() {
			mockServer.EXPECT().IsRunning().Return(true)
			Expect(m.IsRunning()).To(BeTrue())
		})
	})

	Describe(".ConnectLocal", func() {
		BeforeEach(func() {
			cfg.RPC.Address = "127.0.0.1:4000"
		})

		It("should panic when unable to parse local rpc address", func() {
			cfg.RPC.Address = "invalid"
			err := &net.AddrError{Err: "missing port in address", Addr: "invalid"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.ConnectLocal()
			})
		})

		It("should panic when unable to connect to RPC address", func() {
			err := &util.ReqError{Code: "connect_error", HttpCode: 500, Msg: "Post http://127.0.0.1:4000: dial tcp 127.0.0.1:4000: connect: connection refused", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.ConnectLocal()
			})
		})

		It("should return client context object with only 'call' property when no methods from RPC", func() {
			mockClient := mocks2.NewMockClient(ctrl)
			mockClient.EXPECT().Call("rpc_methods", nil).Return(util.Map{"methods": []interface{}{}}, 200, nil)
			m.ClientContextMaker = func(client.Client) *modules.ClientContext {
				return &modules.ClientContext{Client: mockClient, Objects: map[string]interface{}{}}
			}
			objs := m.ConnectLocal()
			Expect(objs).To(HaveLen(1))
			Expect(objs).To(HaveKey("call"))
		})

		It("should return client context object loaded with methods received from RPC", func() {
			mockClient := mocks2.NewMockClient(ctrl)
			mockClient.EXPECT().Call("rpc_methods", nil).Return(util.Map{"methods": []interface{}{
				map[string]interface{}{
					"name":      "method_name",
					"namespace": "method_ns",
				},
			}}, 200, nil)
			m.ClientContextMaker = func(client.Client) *modules.ClientContext {
				return &modules.ClientContext{Client: mockClient, Objects: map[string]interface{}{}}
			}
			objs := m.ConnectLocal()
			Expect(objs).To(HaveLen(2))
			Expect(objs).To(HaveKey("call"))
			Expect(objs).To(HaveKey("method_ns"))
			Expect(objs["method_ns"]).To(HaveLen(1))
			Expect(objs["method_ns"]).To(HaveKey("method_name"))
		})
	})
})
