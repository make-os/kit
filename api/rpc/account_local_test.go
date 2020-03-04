package rpc

import (
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/mocks"
	"gitlab.com/makeos/mosdef/rpc"
	"gitlab.com/makeos/mosdef/types/modules"
)

var _ = Describe("AccountLocal", func() {
	var ctrl *gomock.Controller
	var localAcctApi *LocalAccountAPI
	var mods *modules.Modules

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mods = &modules.Modules{}
		localAcctApi = &LocalAccountAPI{mods}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".listAccounts", func() {
		testCases := map[string]testCase{
			"when nonce is successfully returned": {
				params: nil,
				result: map[string]interface{}{
					"accounts": []string{"addr1", "addr2"},
				},
				mocker: func(tp testCase) {
					mockAcctMod := mocks.NewMockAccountModule(ctrl)
					mockAcctMod.EXPECT().ListLocalAccounts().Return([]string{"addr1", "addr2"})
					mods.Account = mockAcctMod
				},
			},
		}

		for tc, tp := range testCases {
			It(tc, func() {
				if tp.mocker != nil {
					tp.mocker(tp)
				}
				resp := localAcctApi.listAccounts(tp.params)
				Expect(resp).To(Equal(&rpc.Response{
					JSONRPCVersion: "2.0", Err: tp.err, Result: tp.result,
				}))
			})
		}
	})
})
