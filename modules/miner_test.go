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
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util"
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
	var mockLogic *mocks.MockLogic
	var mockSysKeeper *mocks.MockSystemKeeper
	var mockMempoolReactor *mocks.MockMempoolReactor

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		ctrl = gomock.NewController(GinkgoT())
		mockMiner = mocks.NewMockMiner(ctrl)
		mockSysKeeper = mocks.NewMockSystemKeeper(ctrl)
		mockLogic = mocks.NewMockLogic(ctrl)
		mockMempoolReactor = mocks.NewMockMempoolReactor(ctrl)
		mockLogic.EXPECT().GetMempoolReactor().Return(mockMempoolReactor).AnyTimes()
		mockLogic.EXPECT().SysKeeper().Return(mockSysKeeper).AnyTimes()
		m = modules.NewMinerModule(cfg, mockLogic, mockMiner)
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

	Describe(".SubmitWork", func() {
		It("should panic when unable to decode params", func() {
			params := map[string]interface{}{"epoch": struct{}{}}
			err := &errors.ReqError{Code: "invalid_param", HttpCode: 400, Msg: "1 error(s) decoding:\n\n* 'epoch' " +
				"expected type 'int64', got unconvertible type 'struct {}'", Field: "params"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.SubmitWork(params)
			})
		})

		It("should return tx map equivalent if payloadOnly=true", func() {
			key := ""
			params := map[string]interface{}{"epoch": 1, "wnonce": 2}
			res := m.SubmitWork(params, key, true)
			Expect(res["epoch"]).To(Equal(float64(1)))
			Expect(res["wnonce"]).To(Equal(float64(2)))
			Expect(res).ToNot(HaveKey("hash"))
			Expect(res["type"]).To(Equal(float64(txns.TxTypeSubmitWork)))
			Expect(res).To(And(
				HaveKey("timestamp"),
				HaveKey("nonce"),
				HaveKey("type"),
				HaveKey("senderPubKey"),
				HaveKey("fee"),
				HaveKey("sig"),
			))
		})

		It("should panic if unable to add tx to mempool", func() {
			params := map[string]interface{}{"id": 1}
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(nil, fmt.Errorf("error"))
			err := &errors.ReqError{Code: "err_mempool", HttpCode: 400, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.SubmitWork(params, "", false)
			})
		})

		It("should return tx hash on success", func() {
			params := map[string]interface{}{"id": 1}
			hash := util.StrToHexBytes("tx_hash")
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(hash, nil)
			res := m.SubmitWork(params, "", false)
			Expect(res).To(HaveKey("hash"))
			Expect(res["hash"]).To(Equal(hash))
		})
	})

	Describe(".GetPreviousWork", func() {
		It("should return error when unable to query previous mined nonce", func() {
			mockSysKeeper.EXPECT().GetWorkByNode().Return(nil, fmt.Errorf("error"))
			err := &errors.ReqError{Code: "server_err", HttpCode: 400, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetPreviousWork()
			})
		})

		It("should return error when unable to query previous mined nonce", func() {
			works := []*core.NodeWork{
				{Epoch: 1, Nonce: 100},
				{Epoch: 2, Nonce: 100},
			}
			mockSysKeeper.EXPECT().GetWorkByNode().Return(works, nil)
			res := m.GetPreviousWork()
			Expect(util.StructSliceToMap(works)).To(Equal(res))
		})
	})
})
