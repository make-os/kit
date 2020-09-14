package modules

import (
	"os"

	"github.com/golang/mock/gomock"
	"github.com/make-os/lobe/config"
	mocks2 "github.com/make-os/lobe/mocks/rpc"
	"github.com/make-os/lobe/rpc/types"
	"github.com/make-os/lobe/testutil"
	"github.com/make-os/lobe/types/constants"
	"github.com/make-os/lobe/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/robertkrimen/otto"
)

var _ = Describe("RPCModule", func() {
	var err error
	var cfg *config.AppConfig
	var m *RPCModule
	var ctrl *gomock.Controller

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		ctrl = gomock.NewController(GinkgoT())
		m = NewRPCModule(cfg)
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
			val, err := vm.Get(constants.NamespaceRPC)
			Expect(err).To(BeNil())
			Expect(val.IsObject()).To(BeTrue())
		})
	})

	Describe(".connectLocal", func() {
		BeforeEach(func() {
			cfg.Remote.Address = "127.0.0.1:4000"
		})

		It("should return client context object with only 'call' property when no methods from RPC", func() {
			mockClient := mocks2.NewMockClient(ctrl)
			mockClient.EXPECT().Call("rpc_methods", nil).Return(util.Map{"methods": []interface{}{}}, 200, nil)
			m.ClientContextMaker = func(types.Client) *ClientContext {
				return &ClientContext{Client: mockClient, Objects: map[string]interface{}{}}
			}
			objs := m.connectLocal()
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
			m.ClientContextMaker = func(types.Client) *ClientContext {
				return &ClientContext{Client: mockClient, Objects: map[string]interface{}{}}
			}
			objs := m.connectLocal()
			Expect(objs).To(HaveLen(2))
			Expect(objs).To(HaveKey("call"))
			Expect(objs).To(HaveKey("method_ns"))
			Expect(objs["method_ns"]).To(HaveLen(1))
			Expect(objs["method_ns"]).To(HaveKey("method_name"))
		})
	})
})
