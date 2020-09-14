package modules_test

import (
	"fmt"

	"github.com/golang/mock/gomock"
	"github.com/make-os/lobe/crypto"
	"github.com/make-os/lobe/mocks"
	"github.com/make-os/lobe/modules"
	"github.com/make-os/lobe/types/constants"
	"github.com/make-os/lobe/types/core"
	"github.com/make-os/lobe/types/state"
	"github.com/make-os/lobe/util"
	"github.com/make-os/lobe/util/identifier"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/robertkrimen/otto"
	core_types "github.com/tendermint/tendermint/rpc/core/types"
	"github.com/tendermint/tendermint/types"
)

var _ = Describe("ChainModule", func() {
	var m *modules.ChainModule
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
			val, err := vm.Get(constants.NamespaceChain)
			Expect(err).To(BeNil())
			Expect(val.IsObject()).To(BeTrue())
		})
	})

	Describe(".GetBlock", func() {
		It("should panic when height is not a valid number", func() {
			Expect(func() { m.GetBlock("one") }).To(Panic())
		})

		It("should panic when unable to get block at height", func() {
			mockService.EXPECT().GetBlock(int64(1)).Return(nil, fmt.Errorf("error"))
			Expect(func() { m.GetBlock("1") }).To(Panic())
		})

		It("should return expected result on success", func() {
			expected := &core_types.ResultBlock{Block: &types.Block{}}
			mockService.EXPECT().GetBlock(int64(1)).Return(expected, nil)
			res := m.GetBlock("1")
			Expect(map[string]interface{}(res)).To(Equal(util.ToMap(expected)))
		})
	})

	Describe(".GetHeight", func() {
		It("should panic when unable to get last block info from system keeper", func() {
			mockSysKeeper.EXPECT().GetLastBlockInfo().Return(nil, fmt.Errorf("error"))
			Expect(func() { m.GetHeight() }).To(Panic())
		})

		It("should expected result on success", func() {
			mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&state.BlockInfo{Height: 100}, nil)
			height := m.GetHeight()
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
			Expect(res).To(Equal(util.Map(util.ToBasicMap(bi))))
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
			key := crypto.NewKeyFromIntSeed(1)
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
})
