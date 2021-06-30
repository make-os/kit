package modules_test

import (
	"fmt"

	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/crypto/ed25519"
	"github.com/make-os/kit/mocks"
	"github.com/make-os/kit/modules"
	"github.com/make-os/kit/params"
	"github.com/make-os/kit/types/constants"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/errors"
	"github.com/make-os/kit/util/identifier"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/robertkrimen/otto"
	"github.com/stretchr/testify/assert"
	core_types "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

var _ = Describe("NodeModule", func() {
	var m *modules.NodeModule
	var ctrl *gomock.Controller
	var mockService *mocks.MockService
	var mockKeepers *mocks.MockKeepers
	var mockSysKeeper *mocks.MockSystemKeeper
	var mockValKeeper *mocks.MockValidatorKeeper

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockService = mocks.NewMockService(ctrl)
		mockSysKeeper = mocks.NewMockSystemKeeper(ctrl)
		mockKeepers = mocks.NewMockKeepers(ctrl)
		mockValKeeper = mocks.NewMockValidatorKeeper(ctrl)
		mockKeepers.EXPECT().SysKeeper().Return(mockSysKeeper).AnyTimes()
		mockKeepers.EXPECT().ValidatorKeeper().Return(mockValKeeper).AnyTimes()
		m = modules.NewChainModule(mockService, mockKeepers)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".ConfigureVM", func() {
		It("should configure namespace(s) into VM context", func() {
			vm := otto.New()
			m.ConfigureVM(vm)
			val, err := vm.Get(constants.NamespaceNode)
			Expect(err).To(BeNil())
			Expect(val.IsObject()).To(BeTrue())
		})
	})

	Describe(".GetBlock", func() {
		It("should panic when height is not a valid number", func() {
			Expect(func() { m.GetBlock("one") }).To(Panic())
		})

		It("should panic when unable to get block at height", func() {
			var height = int64(1)
			mockService.EXPECT().GetBlock(gomock.Any(), &height).Return(nil, fmt.Errorf("error"))
			Expect(func() { m.GetBlock("1") }).To(Panic())
		})

		It("should return expected result on success", func() {
			expected := &core_types.ResultBlock{Block: &types.Block{}}
			var height = int64(1)
			mockService.EXPECT().GetBlock(gomock.Any(), &height).Return(expected, nil)
			res := m.GetBlock("1")
			Expect(map[string]interface{}(res)).To(Equal(util.ToMap(expected)))
		})
	})

	Describe(".GetCurHeight", func() {
		It("should panic when unable to get last block info from system keeper", func() {
			mockSysKeeper.EXPECT().GetLastBlockInfo().Return(nil, fmt.Errorf("error"))
			Expect(func() { m.GetCurHeight() }).To(Panic())
		})

		It("should expected result on success", func() {
			mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&state.BlockInfo{Height: 100}, nil)
			height := m.GetCurHeight()
			Expect(height).To(Equal("100"))
		})
	})

	Describe(".GetBlockInfo", func() {
		It("should panic when height is not a valid number", func() {
			Expect(func() { m.GetBlockInfo("one") }).To(Panic())
		})

		It("should panic when unable to get block info at height", func() {
			mockSysKeeper.EXPECT().GetBlockInfo(int64(1)).Return(nil, fmt.Errorf("error"))
			Expect(func() { m.GetBlockInfo("1") }).To(Panic())
		})

		It("should return expected block info on success", func() {
			bi := &state.BlockInfo{Height: 100}
			mockSysKeeper.EXPECT().GetBlockInfo(int64(1)).Return(bi, nil)
			res := m.GetBlockInfo("1")
			Expect(res).To(Equal(util.Map(util.ToJSONMap(bi))))
		})
	})

	Describe(".GetValidators", func() {
		It("should panic when height is not a valid number", func() {
			Expect(func() { m.GetValidators("one") }).To(Panic())
		})

		It("should panic when unable to get validators at height", func() {
			mockValKeeper.EXPECT().Get(int64(1)).Return(nil, fmt.Errorf("error"))
			Expect(func() { m.GetValidators("1") }).To(Panic())
		})

		It("should return a list of validators on success", func() {
			key := ed25519.NewKeyFromIntSeed(1)
			ticketID := util.StrToHexBytes("ticket_id")
			vals := core.BlockValidators{
				key.PubKey().MustBytes32(): &core.Validator{PubKey: key.PubKey().MustBytes32(), TicketID: ticketID},
			}
			mockValKeeper.EXPECT().Get(int64(1)).Return(vals, nil)
			res := m.GetValidators("1")
			Expect(res).To(HaveLen(1))
			Expect(res[0]["pubkey"]).To(Equal("48d9u6L7tWpSVYmTE4zBDChMUasjP5pvoXE7kPw5HbJnXRnZBNC"))
			Expect(res[0]["address"]).To(Equal(identifier.Address("os1dmqxfznwyhmkcgcfthlvvt88vajyhnxq7c07k8")))
			Expect(res[0]["tmAddr"]).To(Equal("171E68F02E6F66BF9FF65C13C75D9B2B492C2F40"))
			Expect(res[0]["ticketId"]).To(Equal("0x7469636b65745f6964"))
		})
	})

	Describe(".IsSyncing", func() {
		It("should panic if unable to check sync status", func() {
			mockService.EXPECT().IsSyncing(gomock.Any()).Return(false, fmt.Errorf("error"))
			Expect(func() { m.IsSyncing() }).To(Panic())
		})

		It("should return no error if able to check sync status", func() {
			mockService.EXPECT().IsSyncing(gomock.Any()).Return(true, nil)
			Expect(func() {
				res := m.IsSyncing()
				Expect(res).To(BeTrue())
			}).ToNot(Panic())
		})
	})

	Describe(".GetCurrentEpoch", func() {
		It("should return error if unable to get current epoch", func() {
			mockSysKeeper.EXPECT().GetCurrentEpoch().Return(int64(0), fmt.Errorf("error"))
			err := &errors.ReqError{Code: "server_err", HttpCode: 500, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetCurrentEpoch()
			})
		})

		It("should return epoch on success", func() {
			mockSysKeeper.EXPECT().GetCurrentEpoch().Return(int64(10), nil)
			res := m.GetCurrentEpoch()
			Expect(res).To(Equal("10"))
		})
	})

	Describe(".GetEpoch", func() {
		It("should return expected epoch", func() {
			params.NumBlocksPerEpoch = 5
			Expect(m.GetEpoch(2)).To(Equal("1"))
			Expect(m.GetEpoch(6)).To(Equal("2"))
		})
	})

	Describe(".GetTotalGasMinedInEpoch", func() {
		It("should return expected total gas mined in epoch", func() {
			mockSysKeeper.EXPECT().GetTotalGasMinedInCurEpoch().Return(util.String("100"), nil)
			Expect(m.GetTotalGasMinedInEpoch()).To(Equal("100"))
		})
	})
})
