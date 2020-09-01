package modules_test

import (
	"fmt"

	"github.com/golang/mock/gomock"
	types2 "github.com/make-os/lobe/api/types"
	"github.com/make-os/lobe/mocks"
	mocks2 "github.com/make-os/lobe/mocks/rpc"
	"github.com/make-os/lobe/modules"
	"github.com/make-os/lobe/modules/types"
	"github.com/make-os/lobe/types/constants"
	"github.com/make-os/lobe/types/core"
	"github.com/make-os/lobe/types/state"
	"github.com/make-os/lobe/types/txns"
	"github.com/make-os/lobe/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/robertkrimen/otto"
	"github.com/stretchr/testify/assert"
)

var _ = Describe("RepoModule", func() {
	var m *modules.RepoModule
	var ctrl *gomock.Controller
	var mockService *mocks.MockService
	var mockLogic *mocks.MockLogic
	var mockRepoSrv *mocks.MockRemoteServer
	var mockMempoolReactor *mocks.MockMempoolReactor
	var mockRepoKeeper *mocks.MockRepoKeeper
	var mockTrackedRepoKeeper *mocks.MockTrackedRepoKeeper

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockService = mocks.NewMockService(ctrl)
		mockRepoSrv = mocks.NewMockRemoteServer(ctrl)
		mockMempoolReactor = mocks.NewMockMempoolReactor(ctrl)
		mockLogic = mocks.NewMockLogic(ctrl)
		mockRepoKeeper = mocks.NewMockRepoKeeper(ctrl)
		mockTrackedRepoKeeper = mocks.NewMockTrackedRepoKeeper(ctrl)
		mockLogic.EXPECT().GetMempoolReactor().Return(mockMempoolReactor).AnyTimes()
		mockLogic.EXPECT().RepoKeeper().Return(mockRepoKeeper).AnyTimes()
		mockLogic.EXPECT().GetRemoteServer().Return(mockRepoSrv).AnyTimes()
		mockLogic.EXPECT().TrackedRepoKeeper().Return(mockTrackedRepoKeeper).AnyTimes()
		m = modules.NewRepoModule(mockService, mockRepoSrv, mockLogic)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".ConfigureVM", func() {
		It("should configure namespace(s) into VM context", func() {
			vm := otto.New()
			m.ConfigureVM(vm)
			val, err := vm.Get(constants.NamespaceRepo)
			Expect(err).To(BeNil())
			Expect(val.IsObject()).To(BeTrue())
		})
	})

	Describe(".Create", func() {
		It("should panic when unable to decode params", func() {
			params := map[string]interface{}{"name": struct{}{}}
			err := &util.ReqError{Code: "invalid_param", HttpCode: 400, Msg: "1 error(s) decoding:\n\n* 'name' expected type 'string', got unconvertible type 'struct {}'", Field: "params"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Create(params)
			})
		})

		It("should return tx map equivalent if payloadOnly=true", func() {
			key := ""
			payloadOnly := true
			params := map[string]interface{}{"name": "repo1"}
			res := m.Create(params, key, payloadOnly)
			Expect(res).To(HaveKey("name"))
			Expect(res["name"]).To(Equal("repo1"))
			Expect(res).ToNot(HaveKey("hash"))
			Expect(res["type"]).To(Equal(float64(txns.TxTypeRepoCreate)))
			Expect(res).To(And(
				HaveKey("timestamp"),
				HaveKey("nonce"),
				HaveKey("value"),
				HaveKey("name"),
				HaveKey("config"),
				HaveKey("type"),
				HaveKey("senderPubKey"),
				HaveKey("fee"),
				HaveKey("sig"),
			))
		})

		It("should panic if in attach mode and RPC client method returns error", func() {
			mockClient := mocks2.NewMockClient(ctrl)
			m.AttachedClient = mockClient
			mockClient.EXPECT().CreateRepo(gomock.Any()).Return(nil, fmt.Errorf("error"))
			params := map[string]interface{}{"name": "repo1"}
			err := fmt.Errorf("error")
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Create(params)
			})
		})

		It("should not panic if in attach mode and RPC client method returns no error", func() {
			mockClient := mocks2.NewMockClient(ctrl)
			m.AttachedClient = mockClient
			mockClient.EXPECT().CreateRepo(gomock.Any()).Return(&types2.CreateRepoResponse{}, nil)
			params := map[string]interface{}{"name": "repo1"}
			assert.NotPanics(GinkgoT(), func() {
				m.Create(params)
			})
		})

		It("should panic if unable to add tx to mempool", func() {
			params := map[string]interface{}{"name": "repo1"}
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(nil, fmt.Errorf("error"))
			err := &util.ReqError{Code: "err_mempool", HttpCode: 400, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Create(params, "", false)
			})
		})

		It("should return tx hash on success", func() {
			params := map[string]interface{}{"name": "repo1"}
			hash := util.StrToHexBytes("tx_hash")
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(hash, nil)
			res := m.Create(params, "", false)
			Expect(res).To(HaveKey("hash"))
			Expect(res["hash"]).To(Equal(hash))
			Expect(res["address"]).To(Equal("r/repo1"))
		})
	})

	Describe(".UpsertOwner", func() {
		It("should panic when unable to decode params", func() {
			params := map[string]interface{}{"addresses": struct{}{}}
			err := &util.ReqError{Code: "invalid_param", HttpCode: 400, Msg: "1 error(s) decoding:\n\n* 'addresses[0]' expected type 'string', got unconvertible type 'struct {}'", Field: "params"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.UpsertOwner(params)
			})
		})

		It("should return tx map equivalent if payloadOnly=true", func() {
			key := ""
			payloadOnly := true
			params := map[string]interface{}{"addresses": []string{"addr1"}}
			res := m.UpsertOwner(params, key, payloadOnly)
			Expect(res).To(HaveKey("addresses"))
			Expect(res["addresses"]).To(Equal([]interface{}{"addr1"}))
			Expect(res["veto"]).To(BeFalse())
			Expect(res).ToNot(HaveKey("hash"))
			Expect(res["type"]).To(Equal(float64(txns.TxTypeRepoProposalUpsertOwner)))
			Expect(res).To(And(
				HaveKey("timestamp"),
				HaveKey("nonce"),
				HaveKey("veto"),
				HaveKey("addresses"),
				HaveKey("type"),
				HaveKey("senderPubKey"),
				HaveKey("fee"),
				HaveKey("sig"),
			))
		})

		It("should panic if unable to add tx to mempool", func() {
			params := map[string]interface{}{"addresses": []string{"addr1"}}
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(nil, fmt.Errorf("error"))
			err := &util.ReqError{Code: "err_mempool", HttpCode: 400, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.UpsertOwner(params, "", false)
			})
		})

		It("should return tx hash on success", func() {
			params := map[string]interface{}{"addresses": []string{"addr1"}}
			hash := util.StrToHexBytes("tx_hash")
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(hash, nil)
			res := m.UpsertOwner(params, "", false)
			Expect(res).To(HaveKey("hash"))
			Expect(res["hash"]).To(Equal(hash))
		})
	})

	Describe(".Vote", func() {
		It("should panic when unable to decode params", func() {
			params := map[string]interface{}{"name": struct{}{}}
			err := &util.ReqError{Code: "invalid_param", HttpCode: 400, Msg: "1 error(s) decoding:\n\n* 'name' expected type 'string', got unconvertible type 'struct {}'", Field: "params"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Vote(params)
			})
		})

		It("should return tx map equivalent if payloadOnly=true", func() {
			key := ""
			payloadOnly := true
			params := map[string]interface{}{"name": "repo1"}
			res := m.Vote(params, key, payloadOnly)
			Expect(res["name"]).To(Equal("repo1"))
			Expect(res).ToNot(HaveKey("hash"))
			Expect(res["type"]).To(Equal(float64(txns.TxTypeRepoProposalVote)))
			Expect(res).To(And(
				HaveKey("timestamp"),
				HaveKey("nonce"),
				HaveKey("vote"),
				HaveKey("id"),
				HaveKey("type"),
				HaveKey("senderPubKey"),
				HaveKey("fee"),
				HaveKey("sig"),
			))
		})

		It("should panic if unable to add tx to mempool", func() {
			params := map[string]interface{}{"name": "repo1"}
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(nil, fmt.Errorf("error"))
			err := &util.ReqError{Code: "err_mempool", HttpCode: 400, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Vote(params, "", false)
			})
		})

		It("should return tx hash on success", func() {
			params := map[string]interface{}{"name": "repo1"}
			hash := util.StrToHexBytes("tx_hash")
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(hash, nil)
			res := m.Vote(params, "", false)
			Expect(res).To(HaveKey("hash"))
			Expect(res["hash"]).To(Equal(hash))
		})
	})

	Describe(".Get", func() {
		It("should panic when height option field is not valid", func() {
			err := &util.ReqError{Code: "invalid_param", HttpCode: 400, Msg: "unexpected type", Field: "opts.height"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Get("repo1", types.GetOptions{Height: struct{}{}})
			})
		})

		It("should request for repo with proposals when noProposal=false", func() {
			repo := state.BareRepository()
			repo.Balance = "100"
			mockRepoKeeper.EXPECT().Get("repo1", uint64(0)).Return(repo)
			res := m.Get("repo1", types.GetOptions{Height: 0, NoProposals: false})
			Expect(res).ToNot(BeNil())
			Expect(res["balance"]).To(Equal(util.String("100")))
		})

		It("should panic if in attach mode and RPC client method returns error", func() {
			mockClient := mocks2.NewMockClient(ctrl)
			m.AttachedClient = mockClient
			mockClient.EXPECT().GetRepo("repo1", &types2.GetRepoOpts{Height: 1}).Return(nil, fmt.Errorf("error"))
			err := fmt.Errorf("error")
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Get("repo1", types.GetOptions{Height: 1})
			})
		})

		It("should not panic if in attach mode and RPC client method returns no error", func() {
			mockClient := mocks2.NewMockClient(ctrl)
			m.AttachedClient = mockClient
			mockClient.EXPECT().GetRepo("repo1", &types2.GetRepoOpts{Height: 1}).Return(&types2.GetRepoResponse{}, nil)
			assert.NotPanics(GinkgoT(), func() {
				m.Get("repo1", types.GetOptions{Height: 1})
			})
		})

		It("should request for repo without proposals (using GetNoPopulate) when noProposal=true", func() {
			repo := state.BareRepository()
			repo.Balance = "100"
			mockRepoKeeper.EXPECT().GetNoPopulate("repo1", uint64(0)).Return(repo)
			res := m.Get("repo1", types.GetOptions{Height: 0, NoProposals: true})
			Expect(res).ToNot(BeNil())
			Expect(res["balance"]).To(Equal(util.String("100")))
		})

		It("should panic when repo does not exist", func() {
			repo := state.BareRepository()
			mockRepoKeeper.EXPECT().Get("repo1", uint64(0)).Return(repo)
			err := &util.ReqError{Code: "repo_not_found", HttpCode: 404, Msg: "repo not found", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Get("repo1")
			})
		})
	})

	Describe(".Update", func() {
		It("should panic when unable to decode params", func() {
			params := map[string]interface{}{"config": 123}
			err := &util.ReqError{Code: "invalid_param", HttpCode: 400, Msg: "1 error(s) decoding:\n\n* 'config' expected a map, got 'int'", Field: "params"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Update(params)
			})
		})

		It("should return tx map equivalent if payloadOnly=true", func() {
			key := ""
			payloadOnly := true
			params := map[string]interface{}{"id": 1}
			res := m.Update(params, key, payloadOnly)
			Expect(res["id"]).To(Equal("1"))
			Expect(res).ToNot(HaveKey("hash"))
			Expect(res["type"]).To(Equal(float64(txns.TxTypeRepoProposalUpdate)))
			Expect(res).To(And(
				HaveKey("timestamp"),
				HaveKey("nonce"),
				HaveKey("config"),
				HaveKey("id"),
				HaveKey("type"),
				HaveKey("senderPubKey"),
				HaveKey("fee"),
				HaveKey("sig"),
			))
		})

		It("should panic if unable to add tx to mempool", func() {
			params := map[string]interface{}{"id": 1}
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(nil, fmt.Errorf("error"))
			err := &util.ReqError{Code: "err_mempool", HttpCode: 400, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Update(params, "", false)
			})
		})

		It("should return tx hash on success", func() {
			params := map[string]interface{}{"id": 1}
			hash := util.StrToHexBytes("tx_hash")
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(hash, nil)
			res := m.Update(params, "", false)
			Expect(res).To(HaveKey("hash"))
			Expect(res["hash"]).To(Equal(hash))
		})
	})

	Describe(".DepositProposalFee", func() {
		It("should panic when unable to decode params", func() {
			params := map[string]interface{}{"id": struct{}{}}
			err := &util.ReqError{Code: "invalid_param", HttpCode: 400, Msg: "1 error(s) decoding:\n\n* 'id' expected type 'string', got unconvertible type 'struct {}'", Field: "params"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.DepositProposalFee(params)
			})
		})

		It("should return tx map equivalent if payloadOnly=true", func() {
			key := ""
			payloadOnly := true
			params := map[string]interface{}{"id": 1}
			res := m.DepositProposalFee(params, key, payloadOnly)
			Expect(res["id"]).To(Equal("1"))
			Expect(res).ToNot(HaveKey("hash"))
			Expect(res["type"]).To(Equal(float64(txns.TxTypeRepoProposalSendFee)))
			Expect(res).To(And(
				HaveKey("timestamp"),
				HaveKey("nonce"),
				HaveKey("id"),
				HaveKey("type"),
				HaveKey("senderPubKey"),
				HaveKey("fee"),
				HaveKey("sig"),
			))
		})

		It("should panic if unable to add tx to mempool", func() {
			params := map[string]interface{}{"id": 1}
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(nil, fmt.Errorf("error"))
			err := &util.ReqError{Code: "err_mempool", HttpCode: 400, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.DepositProposalFee(params, "", false)
			})
		})

		It("should return tx hash on success", func() {
			params := map[string]interface{}{"id": 1}
			hash := util.StrToHexBytes("tx_hash")
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(hash, nil)
			res := m.DepositProposalFee(params, "", false)
			Expect(res).To(HaveKey("hash"))
			Expect(res["hash"]).To(Equal(hash))
		})
	})

	Describe(".AddContributor", func() {
		It("should panic when unable to decode params", func() {
			params := map[string]interface{}{"id": struct{}{}}
			err := &util.ReqError{Code: "invalid_param", HttpCode: 400, Msg: "1 error(s) decoding:\n\n* 'id' expected type 'string', got unconvertible type 'struct {}'", Field: "params"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.AddContributor(params)
			})
		})

		It("should return tx map equivalent if payloadOnly=true", func() {
			key := ""
			payloadOnly := true
			params := map[string]interface{}{"id": 1}
			res := m.AddContributor(params, key, payloadOnly)
			Expect(res["id"]).To(Equal("1"))
			Expect(res).ToNot(HaveKey("hash"))
			Expect(res["type"]).To(Equal(float64(txns.TxTypeRepoProposalRegisterPushKey)))
			Expect(res).To(And(
				HaveKey("timestamp"),
				HaveKey("nonce"),
				HaveKey("policies"),
				HaveKey("namespace"),
				HaveKey("namespaceOnly"),
				HaveKey("keys"),
				HaveKey("id"),
				HaveKey("type"),
				HaveKey("senderPubKey"),
				HaveKey("fee"),
				HaveKey("sig"),
			))
		})

		It("should panic if in attach mode and RPC client method returns error", func() {
			mockClient := mocks2.NewMockClient(ctrl)
			m.AttachedClient = mockClient
			mockClient.EXPECT().AddRepoContributors(gomock.Any()).Return(&types2.HashResponse{}, fmt.Errorf("error"))
			params := map[string]interface{}{"id": 1}
			err := fmt.Errorf("error")
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.AddContributor(params)
			})
		})

		It("should not panic if in attach mode and RPC client method returns no error", func() {
			mockClient := mocks2.NewMockClient(ctrl)
			m.AttachedClient = mockClient
			mockClient.EXPECT().AddRepoContributors(gomock.Any()).Return(&types2.HashResponse{}, nil)
			params := map[string]interface{}{"id": 1}
			assert.NotPanics(GinkgoT(), func() {
				m.AddContributor(params)
			})
		})

		It("should panic if unable to add tx to mempool", func() {
			params := map[string]interface{}{"id": 1}
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(nil, fmt.Errorf("error"))
			err := &util.ReqError{Code: "err_mempool", HttpCode: 400, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.AddContributor(params, "", false)
			})
		})

		It("should return tx hash on success", func() {
			params := map[string]interface{}{"id": 1}
			hash := util.StrToHexBytes("tx_hash")
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(hash, nil)
			res := m.AddContributor(params, "", false)
			Expect(res).To(HaveKey("hash"))
			Expect(res["hash"]).To(Equal(hash))
		})
	})

	Describe(".Track", func() {
		It("should panic if unable to add repo", func() {
			mockTrackedRepoKeeper.EXPECT().Add("repo1", []uint64{100}).Return(fmt.Errorf("error"))
			err := &util.ReqError{Code: "server_err", HttpCode: 500, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Track("repo1", 100)
			})
		})

		It("should not panic if able to add repo", func() {
			mockTrackedRepoKeeper.EXPECT().Add("repo1", []uint64{100}).Return(nil)
			assert.NotPanics(GinkgoT(), func() {
				m.Track("repo1", 100)
			})
		})
	})

	Describe(".UnTrack", func() {
		It("should panic if unable to untrack repo", func() {
			mockTrackedRepoKeeper.EXPECT().Remove("repo1").Return(fmt.Errorf("error"))
			err := &util.ReqError{Code: "server_err", HttpCode: 500, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.UnTrack("repo1")
			})
		})

		It("should not panic if able to untrack repo", func() {
			mockTrackedRepoKeeper.EXPECT().Remove("repo1").Return(nil)
			assert.NotPanics(GinkgoT(), func() {
				m.UnTrack("repo1")
			})
		})
	})

	Describe(".GetTracked", func() {
		It("should panic if unable to untrack repo", func() {
			tracked := map[string]*core.TrackedRepo{
				"repo1": {LastUpdated: 10},
			}
			mockTrackedRepoKeeper.EXPECT().Tracked().Return(tracked)
			res := m.GetTracked()
			Expect(res).To(Equal(util.Map(util.ToBasicMap(tracked))))
		})
	})
})
