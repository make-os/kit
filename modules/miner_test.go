package modules_test

import (
	"fmt"
	"os"

	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/mocks"
	"github.com/make-os/kit/modules"
	"github.com/make-os/kit/testutil"
	"github.com/make-os/kit/types/constants"
	"github.com/make-os/kit/util/errors"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/robertkrimen/otto"
	"github.com/stretchr/testify/assert"
)

var _ = Describe("MinerModule", func() {
	var err error
	var cfg *config.AppConfig
	var m *modules.MinerModule
	var ctrl *gomock.Controller
	var mockMiner *mocks.MockMiner

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		ctrl = gomock.NewController(GinkgoT())
		mockMiner = mocks.NewMockMiner(ctrl)
		m = modules.NewMinerModule(cfg, mockMiner)
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
			val, err := vm.Get(constants.NamespaceMiner)
			Expect(err).To(BeNil())
			Expect(val.IsObject()).To(BeTrue())
		})
	})

	Describe(".Start", func() {
		It("should panic if failed to start miner", func() {
			mockMiner.EXPECT().Start(false).Return(fmt.Errorf("error"))
			err := &errors.ReqError{Code: "server_err", HttpCode: 500, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Start()
			})
		})

		It("should pass true as scheduleStart if true is the only param", func() {
			mockMiner.EXPECT().Start(true).Return(nil)
			assert.NotPanics(GinkgoT(), func() {
				m.Start(true)
			})
		})
	})

	Describe(".Stop", func() {
		It("should call stop on the miner", func() {
			mockMiner.EXPECT().Stop()
			m.Stop()
		})
	})

	Describe(".IsRunning", func() {
		It("should return mining status", func() {
			mockMiner.EXPECT().IsMining().Return(true)
			Expect(m.IsRunning()).To(BeTrue())
		})
	})

	Describe(".GetHashrate", func() {
		It("should return hashrate", func() {
			mockMiner.EXPECT().GetHashrate().Return(1000.0)
			Expect(m.GetHashrate()).To(Equal(float64(1000)))
		})
	})
})
