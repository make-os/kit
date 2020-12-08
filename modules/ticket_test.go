package modules_test

import (
	"fmt"

	"github.com/golang/mock/gomock"
	crypto2 "github.com/make-os/kit/crypto/ed25519"
	"github.com/make-os/kit/mocks"
	"github.com/make-os/kit/modules"
	"github.com/make-os/kit/ticket/types"
	"github.com/make-os/kit/types/constants"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/robertkrimen/otto"
	"github.com/stretchr/testify/assert"
)

var _ = Describe("TicketModule", func() {
	var m *modules.TicketModule
	var ctrl *gomock.Controller
	var mockService *mocks.MockService
	var mockLogic *mocks.MockLogic
	var mockMempoolReactor *mocks.MockMempoolReactor
	var mockTicketMgr *mocks.MockTicketManager
	var mockAcctKeeper *mocks.MockAccountKeeper
	var pk = crypto2.NewKeyFromIntSeed(1)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockService = mocks.NewMockService(ctrl)
		mockMempoolReactor = mocks.NewMockMempoolReactor(ctrl)
		mockTicketMgr = mocks.NewMockTicketManager(ctrl)
		mockAcctKeeper = mocks.NewMockAccountKeeper(ctrl)
		mockLogic = mocks.NewMockLogic(ctrl)
		mockLogic.EXPECT().GetMempoolReactor().Return(mockMempoolReactor).AnyTimes()
		mockLogic.EXPECT().GetTicketManager().Return(mockTicketMgr).AnyTimes()
		mockLogic.EXPECT().AccountKeeper().Return(mockAcctKeeper).AnyTimes()
		m = modules.NewTicketModule(mockService, mockLogic, mockTicketMgr)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".ConfigureVM", func() {
		It("should configure namespace(s) into VM context", func() {
			vm := otto.New()
			m.ConfigureVM(vm)
			val, err := vm.Get(constants.NamespaceTicket)
			Expect(err).To(BeNil())
			Expect(val.IsObject()).To(BeTrue())
		})
	})

	Describe(".BuyValidatorTicket", func() {
		It("should panic when unable to decode params", func() {
			params := map[string]interface{}{"delegate": struct{}{}}
			err := &util.ReqError{Code: "invalid_param", HttpCode: 400, Msg: "1 error(s) decoding:\n\n* 'delegate[0]' expected type 'uint8', got unconvertible type 'struct {}'", Field: "params"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.BuyValidatorTicket(params)
			})
		})

		It("should return tx map equivalent if payloadOnly=true", func() {
			key := ""
			payloadOnly := true
			pk := crypto2.NewKeyFromIntSeed(1)
			params := map[string]interface{}{"delegate": pk.PubKey().Base58()}
			res := m.BuyValidatorTicket(params, key, payloadOnly)
			Expect(res).ToNot(HaveKey("hash"))
			Expect(res["type"]).To(Equal(float64(txns.TxTypeValidatorTicket)))
			Expect(res["blsPubKey"]).To(BeEmpty())
			Expect(res).To(And(
				HaveKey("delegate"),
				HaveKey("type"),
				HaveKey("senderPubKey"),
				HaveKey("blsPubKey"),
				HaveKey("fee"),
				HaveKey("sig"),
				HaveKey("value"),
			))
		})

		It("should panic if unable to add tx to mempool", func() {
			params := map[string]interface{}{"value": "100"}
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(nil, fmt.Errorf("error"))
			err := &util.ReqError{Code: "err_mempool", HttpCode: 400, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.BuyValidatorTicket(params, "", false)
			})
		})

		It("should return tx hash on success", func() {
			params := map[string]interface{}{"value": "100"}
			hash := util.StrToHexBytes("tx_hash")
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(hash, nil)
			res := m.BuyValidatorTicket(params, "", false)
			Expect(res).To(HaveKey("hash"))
			Expect(res["hash"]).To(Equal(hash))
		})
	})

	Describe(".BuyHostTicket()", func() {
		It("should panic when unable to decode params", func() {
			params := map[string]interface{}{"delegate": struct{}{}}
			err := &util.ReqError{Code: "invalid_param", HttpCode: 400, Msg: "1 error(s) decoding:\n\n* 'delegate[0]' expected type 'uint8', got unconvertible type 'struct {}'", Field: "params"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.BuyHostTicket(params)
			})
		})

		It("should return tx map equivalent if payloadOnly=true", func() {
			key := pk.PrivKey().Base58()
			payloadOnly := true
			params := map[string]interface{}{"delegate": pk.PubKey().Base58()}

			acct := state.BareAccount()
			acct.Nonce = 100
			mockAcctKeeper.EXPECT().Get(pk.Addr()).Return(acct)

			res := m.BuyHostTicket(params, key, payloadOnly)
			Expect(res).ToNot(HaveKey("hash"))
			Expect(res["type"]).To(Equal(float64(txns.TxTypeHostTicket)))
			Expect(res).To(And(
				HaveKey("delegate"),
				HaveKey("type"),
				HaveKey("senderPubKey"),
				HaveKey("blsPubKey"),
				HaveKey("fee"),
				HaveKey("sig"),
				HaveKey("value"),
			))
			Expect(res["blsPubKey"]).ToNot(BeEmpty())
		})

		It("should panic if unable to add tx to mempool", func() {
			key := pk.PrivKey().Base58()
			payloadOnly := false
			acct := state.BareAccount()
			acct.Nonce = 100
			mockAcctKeeper.EXPECT().Get(pk.Addr()).Return(acct)

			params := map[string]interface{}{"value": "100"}
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(nil, fmt.Errorf("error"))
			err := &util.ReqError{Code: "err_mempool", HttpCode: 400, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.BuyHostTicket(params, key, payloadOnly)
			})
		})

		It("should return tx hash on success", func() {
			key := pk.PrivKey().Base58()
			payloadOnly := false
			acct := state.BareAccount()
			acct.Nonce = 100
			mockAcctKeeper.EXPECT().Get(pk.Addr()).Return(acct)

			params := map[string]interface{}{"value": "100"}
			hash := util.StrToHexBytes("tx_hash")
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(hash, nil)
			res := m.BuyHostTicket(params, key, payloadOnly)
			Expect(res).To(HaveKey("hash"))
			Expect(res["hash"]).To(Equal(hash))
		})
	})

	Describe(".GetValidatorTicketsByProposer", func() {
		It("should panic when proposer public key is invalid", func() {
			err := &util.ReqError{Code: "invalid_proposer_pub_key", HttpCode: 400, Msg: "invalid format: version and/or checksum bytes missing", Field: "proposerPubKey"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetValidatorTicketsByProposer("prop_pub_key", map[string]interface{}{})
			})
		})

		It("should panic when unable to get ticket by proposer public key", func() {
			propPubKey := pk.PubKey().Base58()
			mockTicketMgr.EXPECT().GetByProposer(txns.TxTypeValidatorTicket, pk.PubKey().MustBytes32(), types.QueryOptions{
				Limit:        0,
				SortByHeight: -1,
				Active:       true,
			}).Return(nil, fmt.Errorf("error"))
			err := &util.ReqError{Code: "server_err", HttpCode: 500, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetValidatorTicketsByProposer(propPubKey, map[string]interface{}{})
			})
		})

		It("should ticket on success", func() {
			propPubKey := pk.PubKey().Base58()
			tickets := []*types.Ticket{
				{Type: txns.TxTypeValidatorTicket},
			}
			mockTicketMgr.EXPECT().GetByProposer(txns.TxTypeValidatorTicket, pk.PubKey().MustBytes32(), types.QueryOptions{
				Limit:        0,
				SortByHeight: -1,
				Active:       true,
			}).Return(tickets, nil)
			res := m.GetValidatorTicketsByProposer(propPubKey, map[string]interface{}{})
			Expect(res).To(HaveLen(1))
			Expect(res[0]["type"]).To(Equal(txns.TxTypeValidatorTicket))
		})

		It("should no ticket when proposer has no ticket", func() {
			propPubKey := pk.PubKey().Base58()
			tickets := []*types.Ticket{}
			mockTicketMgr.EXPECT().GetByProposer(txns.TxTypeValidatorTicket, pk.PubKey().MustBytes32(), types.QueryOptions{
				Limit:        1,
				SortByHeight: -1,
				Active:       true,
			}).Return(tickets, nil)
			res := m.GetValidatorTicketsByProposer(propPubKey, map[string]interface{}{"limit": 1})
			Expect(res).To(HaveLen(0))
		})

		It("should set queryOption.UnExpiredOnly to the value of 'active'", func() {
			propPubKey := pk.PubKey().Base58()
			tickets := []*types.Ticket{}
			mockTicketMgr.EXPECT().GetByProposer(txns.TxTypeValidatorTicket, pk.PubKey().MustBytes32(), types.QueryOptions{
				Limit:        0,
				SortByHeight: -1,
				Active:       false,
			}).Return(tickets, nil)
			res := m.GetValidatorTicketsByProposer(propPubKey, map[string]interface{}{"active": false})
			Expect(res).To(HaveLen(0))
		})
	})

	Describe(".GetHostTicketsByProposer", func() {
		It("should panic when proposer public key is invalid", func() {
			err := &util.ReqError{Code: "invalid_proposer_pub_key", HttpCode: 400, Msg: "invalid format: version and/or checksum bytes missing", Field: "params"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetHostTicketsByProposer("prop_pub_key", map[string]interface{}{})
			})
		})

		It("should panic when unable to get ticket by proposer public key", func() {
			propPubKey := pk.PubKey().Base58()
			mockTicketMgr.EXPECT().GetByProposer(txns.TxTypeHostTicket, pk.PubKey().MustBytes32(), types.QueryOptions{Limit: 0, SortByHeight: -1}).Return(nil, fmt.Errorf("error"))
			err := &util.ReqError{Code: "server_err", HttpCode: 500, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetHostTicketsByProposer(propPubKey, map[string]interface{}{})
			})
		})

		It("should ticket on success", func() {
			propPubKey := pk.PubKey().Base58()
			tickets := []*types.Ticket{
				{Type: txns.TxTypeHostTicket},
			}
			mockTicketMgr.EXPECT().GetByProposer(txns.TxTypeHostTicket, pk.PubKey().MustBytes32(), types.QueryOptions{Limit: 1, SortByHeight: -1}).Return(tickets, nil)
			res := m.GetHostTicketsByProposer(propPubKey, map[string]interface{}{"limit": 1})
			Expect(res).To(HaveLen(1))
			Expect(res[0]["type"]).To(Equal(txns.TxTypeHostTicket))
		})

		It("should no ticket when proposer has no ticket", func() {
			propPubKey := pk.PubKey().Base58()
			tickets := []*types.Ticket{}
			mockTicketMgr.EXPECT().GetByProposer(txns.TxTypeHostTicket, pk.PubKey().MustBytes32(), types.QueryOptions{Limit: 1, SortByHeight: -1}).Return(tickets, nil)
			res := m.GetHostTicketsByProposer(propPubKey, map[string]interface{}{"limit": 1})
			Expect(res).To(HaveLen(0))
		})
	})

	Describe(".GetTopValidators", func() {
		It("should panic when unable to get top validators", func() {
			mockTicketMgr.EXPECT().GetTopValidators(0).Return(nil, fmt.Errorf("error"))
			err := &util.ReqError{Code: "server_err", HttpCode: 500, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetTopValidators()
			})
		})

		It("should return empty result when no top validators was returned", func() {
			tickets := []*types.SelectedTicket{}
			mockTicketMgr.EXPECT().GetTopValidators(0).Return(tickets, nil)
			res := m.GetTopValidators()
			Expect(res).To(BeEmpty())
		})

		It("should return empty result when no top validators was returned", func() {
			tickets := []*types.SelectedTicket{
				{Ticket: &types.Ticket{Type: txns.TxTypeHostTicket}},
			}
			mockTicketMgr.EXPECT().GetTopValidators(1).Return(tickets, nil)
			res := m.GetTopValidators(1)
			Expect(res).To(HaveLen(1))
		})
	})

	Describe(".GetTopHosts()", func() {
		It("should panic when unable to get top validators", func() {
			mockTicketMgr.EXPECT().GetTopHosts(0).Return(nil, fmt.Errorf("error"))
			err := &util.ReqError{Code: "server_err", HttpCode: 500, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetTopHosts()
			})
		})

		It("should return empty result when no top validators was returned", func() {
			tickets := []*types.SelectedTicket{}
			mockTicketMgr.EXPECT().GetTopHosts(0).Return(tickets, nil)
			res := m.GetTopHosts()
			Expect(res).To(BeEmpty())
		})

		It("should return empty result when no top validators was returned", func() {
			tickets := []*types.SelectedTicket{
				{Ticket: &types.Ticket{Type: txns.TxTypeHostTicket}},
			}
			mockTicketMgr.EXPECT().GetTopHosts(1).Return(tickets, nil)
			res := m.GetTopHosts(1)
			Expect(res).To(HaveLen(1))
		})
	})

	Describe(".GetStats", func() {
		It("should panic when unable to get all tickets value", func() {
			mockTicketMgr.EXPECT().ValueOfAllTickets(uint64(0)).Return(float64(0), fmt.Errorf("error"))
			err := &util.ReqError{Code: "server_err", HttpCode: 500, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetStats()
			})
		})

		It("should return value of 'all' tickets only if proposer public key is not provided", func() {
			mockTicketMgr.EXPECT().ValueOfAllTickets(uint64(0)).Return(float64(123.44), nil)
			res := m.GetStats()
			Expect(res).To(HaveLen(1))
			Expect(res).To(HaveKey("all"))
			Expect(res["all"]).To(Equal(float64(123.44)))
		})

		It("should panic when proposer public key is invalid", func() {
			mockTicketMgr.EXPECT().ValueOfAllTickets(uint64(0)).Return(float64(123.44), nil)
			err := &util.ReqError{Code: "invalid_proposer_pub_key", HttpCode: 400, Msg: "invalid format: version and/or checksum bytes missing", Field: "params"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetStats("invalid_pub_key")
			})
		})

		It("should panic when unable to get value of non-delegated tickets of proposer public key", func() {
			mockTicketMgr.EXPECT().ValueOfAllTickets(uint64(0)).Return(float64(123.44), nil)
			mockTicketMgr.EXPECT().ValueOfNonDelegatedTickets(pk.PubKey().MustBytes32(), uint64(0)).Return(float64(0), fmt.Errorf("error"))
			err := &util.ReqError{Code: "server_err", HttpCode: 500, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetStats(pk.PubKey().Base58())
			})
		})

		It("should panic when unable to get value of delegated tickets of proposer public key", func() {
			mockTicketMgr.EXPECT().ValueOfAllTickets(uint64(0)).Return(float64(123.44), nil)
			mockTicketMgr.EXPECT().ValueOfNonDelegatedTickets(pk.PubKey().MustBytes32(), uint64(0)).Return(float64(230), nil)
			mockTicketMgr.EXPECT().ValueOfDelegatedTickets(pk.PubKey().MustBytes32(), uint64(0)).Return(float64(0), fmt.Errorf("error"))
			err := &util.ReqError{Code: "server_err", HttpCode: 500, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetStats(pk.PubKey().Base58())
			})
		})

		It("should return expected fields and value on success", func() {
			mockTicketMgr.EXPECT().ValueOfAllTickets(uint64(0)).Return(float64(123.44), nil)
			mockTicketMgr.EXPECT().ValueOfNonDelegatedTickets(pk.PubKey().MustBytes32(), uint64(0)).Return(float64(230), nil)
			mockTicketMgr.EXPECT().ValueOfDelegatedTickets(pk.PubKey().MustBytes32(), uint64(0)).Return(float64(100), nil)
			res := m.GetStats(pk.PubKey().Base58())
			Expect(res).ToNot(BeEmpty())
			Expect(res).To(Equal(util.Map{
				"delegated":    100.000000,
				"total":        "330",
				"all":          123.440000,
				"nonDelegated": 230.000000,
			}))
		})
	})

	Describe(".GetAll", func() {
		It("should return tickets", func() {
			tickets := []*types.Ticket{{Type: txns.TxTypeHostTicket, Hash: util.StrToHexBytes("hash1")}}
			qo := types.QueryOptions{Limit: 1, SortByHeight: -1}
			mockTicketMgr.EXPECT().Query(gomock.Any(), qo).Return(tickets)
			res := m.GetAll(1)
			Expect(res).To(HaveLen(1))
			Expect(res).To(Equal(util.StructSliceToMap(tickets)))
		})

		It("should return no tickets when query returned no tickets", func() {
			tickets := []*types.Ticket{}
			qo := types.QueryOptions{Limit: 0, SortByHeight: -1}
			mockTicketMgr.EXPECT().Query(gomock.Any(), qo).Return(tickets)
			res := m.GetAll(0)
			Expect(res).To(HaveLen(0))
		})
	})

	Describe(".UnbondHostTicket", func() {
		It("should panic when unable to decode params", func() {
			params := map[string]interface{}{"hash": 123}
			err := &util.ReqError{Code: "invalid_param", HttpCode: 400, Msg: "1 error(s) decoding:\n\n* 'hash': source data must be an array or slice, got int", Field: "params"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.UnbondHostTicket(params)
			})
		})

		It("should return tx map equivalent if payloadOnly=true", func() {
			key := ""
			payloadOnly := true
			pk := crypto2.NewKeyFromIntSeed(1)
			params := map[string]interface{}{"delegate": pk.PubKey().Base58()}
			res := m.UnbondHostTicket(params, key, payloadOnly)
			Expect(res).To(And(
				HaveKey("hash"),
				HaveKey("type"),
				HaveKey("senderPubKey"),
				HaveKey("fee"),
				HaveKey("sig"),
			))
			Expect(res["type"]).To(Equal(float64(txns.TxTypeUnbondHostTicket)))
		})

		It("should panic if unable to add tx to mempool", func() {
			params := map[string]interface{}{"value": "100"}
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(nil, fmt.Errorf("error"))
			err := &util.ReqError{Code: "err_mempool", HttpCode: 400, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.UnbondHostTicket(params, "", false)
			})
		})

		It("should return tx hash on success", func() {
			params := map[string]interface{}{"value": "100"}
			hash := util.StrToHexBytes("tx_hash")
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(hash, nil)
			res := m.UnbondHostTicket(params, "", false)
			Expect(res).To(HaveKey("hash"))
			Expect(res["hash"]).To(Equal(hash))
		})
	})
})
