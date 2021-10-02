package modules_test

import (
	"fmt"

	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/mocks"
	mocks2 "github.com/make-os/kit/mocks/rpc"
	"github.com/make-os/kit/modules"
	"github.com/make-os/kit/modules/types"
	"github.com/make-os/kit/testutil"
	"github.com/make-os/kit/types/api"
	"github.com/make-os/kit/types/constants"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/crypto"
	"github.com/make-os/kit/util/errors"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/robertkrimen/otto"
	"github.com/stretchr/testify/assert"
)

var _ = Describe("RepoModule", func() {
	var m *modules.RepoModule
	var ctrl *gomock.Controller
	var cfg *config.AppConfig
	var mockService *mocks.MockService
	var mockLogic *mocks.MockLogic
	var mockRepoSrv *mocks.MockRemoteServer
	var mockMempoolReactor *mocks.MockMempoolReactor
	var mockRepoKeeper *mocks.MockRepoKeeper
	var mockNSKeeper *mocks.MockNamespaceKeeper
	var mockRepoSyncInfoKeeper *mocks.MockRepoSyncInfoKeeper

	BeforeEach(func() {
		var err error
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())

		ctrl = gomock.NewController(GinkgoT())
		mockService = mocks.NewMockService(ctrl)
		mockRepoSrv = mocks.NewMockRemoteServer(ctrl)
		mockMempoolReactor = mocks.NewMockMempoolReactor(ctrl)
		mockLogic = mocks.NewMockLogic(ctrl)
		mockRepoKeeper = mocks.NewMockRepoKeeper(ctrl)
		mockRepoSyncInfoKeeper = mocks.NewMockRepoSyncInfoKeeper(ctrl)
		mockNSKeeper = mocks.NewMockNamespaceKeeper(ctrl)
		mockLogic.EXPECT().Config().Return(cfg).AnyTimes()
		mockLogic.EXPECT().GetMempoolReactor().Return(mockMempoolReactor).AnyTimes()
		mockLogic.EXPECT().RepoKeeper().Return(mockRepoKeeper).AnyTimes()
		mockLogic.EXPECT().GetRemoteServer().Return(mockRepoSrv).AnyTimes()
		mockLogic.EXPECT().RepoSyncInfoKeeper().Return(mockRepoSyncInfoKeeper).AnyTimes()
		mockLogic.EXPECT().NamespaceKeeper().Return(mockNSKeeper).AnyTimes()
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
			err := &errors.ReqError{Code: modules.StatusCodeInvalidParam, HttpCode: 400, Msg: "1 error(s) decoding:\n\n* 'name' expected type 'string', got unconvertible type 'struct {}'", Field: "params"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Create(params)
			})
		})

		It("should return tx map equivalent if payloadOnly=true", func() {
			key := ""
			params := map[string]interface{}{"name": "repo1"}
			res := m.Create(params, key, true)
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
			mockRepoClient := mocks2.NewMockRepo(ctrl)
			mockClient.EXPECT().Repo().Return(mockRepoClient)
			m.Client = mockClient

			mockRepoClient.EXPECT().Create(gomock.Any()).Return(nil, fmt.Errorf("error"))
			params := map[string]interface{}{"name": "repo1"}
			err := fmt.Errorf("error")
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Create(params)
			})
		})

		It("should not panic if in attach mode and RPC client method returns no error", func() {
			mockClient := mocks2.NewMockClient(ctrl)
			mockRepoClient := mocks2.NewMockRepo(ctrl)
			mockClient.EXPECT().Repo().Return(mockRepoClient)
			m.Client = mockClient

			mockRepoClient.EXPECT().Create(gomock.Any()).Return(&api.ResultCreateRepo{}, nil)
			params := map[string]interface{}{"name": "repo1"}
			assert.NotPanics(GinkgoT(), func() {
				m.Create(params)
			})
		})

		It("should panic if unable to add tx to mempool", func() {
			params := map[string]interface{}{"name": "repo1"}
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(nil, fmt.Errorf("error"))
			err := &errors.ReqError{Code: "err_mempool", HttpCode: 400, Msg: "error", Field: ""}
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
			err := &errors.ReqError{Code: modules.StatusCodeInvalidParam, HttpCode: 400, Msg: "1 error(s) decoding:\n\n* 'addresses[0]' expected type 'string', got unconvertible type 'struct {}'", Field: "params"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.UpsertOwner(params)
			})
		})

		It("should return tx map equivalent if payloadOnly=true", func() {
			key := ""
			params := map[string]interface{}{"addresses": []string{"addr1"}}
			res := m.UpsertOwner(params, key, true)
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
			err := &errors.ReqError{Code: "err_mempool", HttpCode: 400, Msg: "error", Field: ""}
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
			err := &errors.ReqError{Code: modules.StatusCodeInvalidParam, HttpCode: 400, Msg: "1 error(s) decoding:\n\n* 'name' expected type 'string', got unconvertible type 'struct {}'", Field: "params"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Vote(params)
			})
		})

		It("should return tx map equivalent if payloadOnly=true", func() {
			key := ""
			params := map[string]interface{}{"name": "repo1"}
			res := m.Vote(params, key, true)
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
			err := &errors.ReqError{Code: "err_mempool", HttpCode: 400, Msg: "error", Field: ""}
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
			err := &errors.ReqError{Code: modules.StatusCodeInvalidParam, HttpCode: 400, Msg: "unexpected type", Field: "opts.height"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Get("repo1", types.GetOptions{Height: struct{}{}})
			})
		})

		It("should panic if in attach mode and RPC client method returns error", func() {
			mockClient := mocks2.NewMockClient(ctrl)
			mockRepoClient := mocks2.NewMockRepo(ctrl)
			mockClient.EXPECT().Repo().Return(mockRepoClient)
			m.Client = mockClient

			mockRepoClient.EXPECT().Get("repo1", &api.GetRepoOpts{Height: 1}).Return(nil, fmt.Errorf("error"))
			err := fmt.Errorf("error")
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Get("repo1", types.GetOptions{Height: 1})
			})
		})

		It("should not panic if in attach mode and RPC client method returns no error", func() {
			mockClient := mocks2.NewMockClient(ctrl)
			mockRepoClient := mocks2.NewMockRepo(ctrl)
			mockClient.EXPECT().Repo().Return(mockRepoClient)
			m.Client = mockClient

			mockRepoClient.EXPECT().Get("repo1", &api.GetRepoOpts{Height: 1}).Return(&api.ResultRepository{}, nil)
			assert.NotPanics(GinkgoT(), func() {
				m.Get("repo1", types.GetOptions{Height: 1})
			})
		})

		It("should return repo when it exist", func() {
			repo := state.BareRepository()
			repo.Balance = "100"
			mockRepoKeeper.EXPECT().Get("repo1", uint64(0)).Return(repo)
			res := m.Get("repo1", types.GetOptions{Height: 0})
			Expect(res).ToNot(BeNil())
			Expect(res["balance"]).To(Equal(util.String("100")))
		})

		It("should panic when repo does not exist", func() {
			repo := state.BareRepository()
			mockRepoKeeper.EXPECT().Get("repo1", uint64(0)).Return(repo)
			err := &errors.ReqError{Code: "repo_not_found", HttpCode: 404, Msg: "repo not found", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Get("repo1")
			})
		})

		When("full namespace URI is provided", func() {
			When("uri=r/repo1", func() {
				It("should attempt to get repo1", func() {
					repo := state.BareRepository()
					repo.Balance = "100"
					mockRepoKeeper.EXPECT().Get("repo1", uint64(0)).Return(repo)
					res := m.Get("r/repo1")
					Expect(res).ToNot(BeNil())
					Expect(res["balance"]).To(Equal(util.String("100")))
				})
			})

			When("uri=ns1/repo1", func() {
				It("should panic if namespace=ns1 is unknown", func() {
					mockNSKeeper.EXPECT().Get(crypto.MakeNamespaceHash("ns1")).Return(state.BareNamespace())
					err := &errors.ReqError{Code: modules.StatusCodeInvalidParam, HttpCode: 404, Msg: "namespace not found", Field: "name"}
					assert.PanicsWithError(GinkgoT(), err.Error(), func() {
						m.Get("ns1/repo1")
					})
				})

				It("should panic if domain=repo1 does not exist in the namespace", func() {
					ns := state.BareNamespace()
					ns.Domains["something"] = "r/target"
					mockNSKeeper.EXPECT().Get(crypto.MakeNamespaceHash("ns1")).Return(ns)
					err := &errors.ReqError{Code: modules.StatusCodeInvalidParam, HttpCode: 404, Msg: "namespace domain not found", Field: "name"}
					assert.PanicsWithError(GinkgoT(), err.Error(), func() {
						m.Get("ns1/repo1")
					})
				})

				It("should panic if domain=repo1 points does not point to a native repo URI", func() {
					ns := state.BareNamespace()
					ns.Domains["repo1"] = "a/target"
					mockNSKeeper.EXPECT().Get(crypto.MakeNamespaceHash("ns1")).Return(ns)
					err := &errors.ReqError{Code: modules.StatusCodeInvalidParam, HttpCode: 404, Msg: "namespace domain target is not a repository", Field: "name"}
					assert.PanicsWithError(GinkgoT(), err.Error(), func() {
						m.Get("ns1/repo1")
					})
				})

				It("should successfully return repo if domain and target are valid", func() {
					repo := state.BareRepository()
					repo.Balance = "100"
					mockRepoKeeper.EXPECT().Get("repo1", uint64(0)).Return(repo)
					ns := state.BareNamespace()
					ns.Domains["repo1"] = "r/repo1"
					mockNSKeeper.EXPECT().Get(crypto.MakeNamespaceHash("ns1")).Return(ns)
					res := m.Get("ns1/repo1")
					Expect(res).ToNot(BeNil())
					Expect(res["balance"]).To(Equal(util.String("100")))
				})
			})
		})

		When("selector is provided", func() {
			It("should return repo and selected fields when it exist", func() {
				repo := state.BareRepository()
				repo.Balance = "100"
				repo.CreatedAt = 1000000
				mockRepoKeeper.EXPECT().Get("repo1", uint64(0)).Return(repo)
				res := m.Get("repo1", types.GetOptions{Height: 0, Select: []string{"createdAt"}})
				Expect(res).ToNot(BeNil())
				Expect(res["createdAt"]).To(Equal("1000000"))
				Expect(res).NotTo(HaveKey("balance"))
			})

			It("should panic when a selector is malformed", func() {
				repo := state.BareRepository()
				repo.Balance = "100"
				mockRepoKeeper.EXPECT().Get("repo1", uint64(0)).Return(repo)
				ns := state.BareNamespace()
				ns.Domains["repo1"] = "r/repo1"
				mockNSKeeper.EXPECT().Get(crypto.MakeNamespaceHash("ns1")).Return(ns)
				err := &errors.ReqError{Code: "invalid_param", HttpCode: 400, Msg: "selector at index=0 is malformed", Field: "select"}
				assert.PanicsWithError(GinkgoT(), err.Error(), func() {
					m.Get("ns1/repo1", types.GetOptions{Select: []string{"some*key"}})
				})
			})
		})
	})

	Describe(".Update", func() {
		It("should panic when unable to decode params", func() {
			params := map[string]interface{}{"config": 123}
			err := &errors.ReqError{Code: modules.StatusCodeInvalidParam, HttpCode: 400, Msg: "1 error(s) decoding:\n\n* 'config' expected a map, got 'int'", Field: "params"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Update(params)
			})
		})

		It("should return tx map equivalent if payloadOnly=true", func() {
			key := ""
			params := map[string]interface{}{"id": 1}
			res := m.Update(params, key, true)
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
			err := &errors.ReqError{Code: "err_mempool", HttpCode: 400, Msg: "error", Field: ""}
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
			err := &errors.ReqError{Code: modules.StatusCodeInvalidParam, HttpCode: 400, Msg: "1 error(s) decoding:\n\n* 'id' expected type 'string', got unconvertible type 'struct {}'", Field: "params"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.DepositProposalFee(params)
			})
		})

		It("should return tx map equivalent if payloadOnly=true", func() {
			key := ""
			params := map[string]interface{}{"id": 1}
			res := m.DepositProposalFee(params, key, true)
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
			err := &errors.ReqError{Code: "err_mempool", HttpCode: 400, Msg: "error", Field: ""}
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
			err := &errors.ReqError{Code: modules.StatusCodeInvalidParam, HttpCode: 400, Msg: "1 error(s) decoding:\n\n* 'id' expected type 'string', got unconvertible type 'struct {}'", Field: "params"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.AddContributor(params)
			})
		})

		It("should return tx map equivalent if payloadOnly=true", func() {
			key := ""
			params := map[string]interface{}{"id": 1}
			res := m.AddContributor(params, key, true)
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
			mockRepoClient := mocks2.NewMockRepo(ctrl)
			mockClient.EXPECT().Repo().Return(mockRepoClient)
			m.Client = mockClient

			mockRepoClient.EXPECT().AddContributors(gomock.Any()).Return(&api.ResultHash{}, fmt.Errorf("error"))
			params := map[string]interface{}{"id": 1}
			err := fmt.Errorf("error")
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.AddContributor(params)
			})
		})

		It("should not panic if in attach mode and RPC client method returns no error", func() {
			mockClient := mocks2.NewMockClient(ctrl)
			mockRepoClient := mocks2.NewMockRepo(ctrl)
			mockClient.EXPECT().Repo().Return(mockRepoClient)
			m.Client = mockClient

			mockRepoClient.EXPECT().AddContributors(gomock.Any()).Return(&api.ResultHash{}, nil)
			params := map[string]interface{}{"id": 1}
			assert.NotPanics(GinkgoT(), func() {
				m.AddContributor(params)
			})
		})

		It("should panic if unable to add tx to mempool", func() {
			params := map[string]interface{}{"id": 1}
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(nil, fmt.Errorf("error"))
			err := &errors.ReqError{Code: "err_mempool", HttpCode: 400, Msg: "error", Field: ""}
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
			mockRepoSyncInfoKeeper.EXPECT().Track("repo1", []uint64{100}).Return(fmt.Errorf("error"))
			err := &errors.ReqError{Code: "server_err", HttpCode: 500, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Track("repo1", 100)
			})
		})

		It("should not panic if able to add repo", func() {
			mockRepoSyncInfoKeeper.EXPECT().Track("repo1", []uint64{100}).Return(nil)
			assert.NotPanics(GinkgoT(), func() {
				m.Track("repo1", 100)
			})
		})
	})

	Describe(".UnTrack", func() {
		It("should panic if unable to untrack repo", func() {
			mockRepoSyncInfoKeeper.EXPECT().UnTrack("repo1").Return(fmt.Errorf("error"))
			err := &errors.ReqError{Code: "server_err", HttpCode: 500, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.UnTrack("repo1")
			})
		})

		It("should not panic if able to untrack repo", func() {
			mockRepoSyncInfoKeeper.EXPECT().UnTrack("repo1").Return(nil)
			assert.NotPanics(GinkgoT(), func() {
				m.UnTrack("repo1")
			})
		})
	})

	Describe(".Get", func() {
		It("should panic if unable to untrack repo", func() {
			tracked := map[string]*core.TrackedRepo{
				"repo1": {UpdatedAt: 10},
			}
			mockRepoSyncInfoKeeper.EXPECT().Tracked().Return(tracked)
			res := m.GetTracked()
			Expect(res).To(Equal(util.Map(util.ToJSONMap(tracked))))
		})
	})

	Describe(".ListPath", func() {

		It("should panic if repo name is not provided", func() {
			err := &errors.ReqError{Code: modules.StatusCodeInvalidParam, HttpCode: 400, Msg: "repo name is required", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.ListPath("", "")
			})
		})

		It("should panic if repo does not exist", func() {
			err := &errors.ReqError{Code: modules.StatusCodeInvalidParam, HttpCode: 404, Msg: "repository does not exist", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.ListPath("unknown", "")
			})
		})

		It("should return entries when successful", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			res := m.ListPath("repo1", "")
			Expect(res).To(HaveLen(4))
		})

		It("should return error when path is unknown", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			err := &errors.ReqError{Code: modules.StatusCodePathNotFound, HttpCode: 404, Msg: "path not found", Field: "path"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.ListPath("repo1", "unknown")
			})
		})

		It("should return error when revision is invalid", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			err := &errors.ReqError{Code: modules.StatusCodeReferenceNotFound, HttpCode: 404, Msg: "reference not found", Field: "revision"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.ListPath("repo1", ".", "something")
			})
		})
	})

	Describe(".ReadFileLines", func() {
		It("should panic if repo name is not provided", func() {
			err := &errors.ReqError{Code: modules.StatusCodeInvalidParam, HttpCode: 400, Msg: "repo name is required", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.ReadFileLines("", "")
			})
		})

		It("should panic if file path is not provided", func() {
			err := &errors.ReqError{Code: modules.StatusCodeInvalidParam, HttpCode: 400, Msg: "file path is required", Field: "file"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.ReadFileLines("repo1", "")
			})
		})

		It("should panic if repo does not exist", func() {
			err := &errors.ReqError{Code: modules.StatusCodeInvalidParam, HttpCode: 404, Msg: "repository does not exist", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.ReadFileLines("unknown", "a/b")
			})
		})

		It("should panic if file path was not provided", func() {
			err := &errors.ReqError{Code: "invalid_param", HttpCode: 400, Msg: "file path is required", Field: "file"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.ReadFileLines("repo1", "")
			})
		})

		It("should panic if file path does not exist", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			err := &errors.ReqError{Code: "path_not_found", HttpCode: 404, Msg: "path not found", Field: "file"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.ReadFileLines("repo1", "unknown")
			})
		})

		It("should panic if file path is not a file", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			err := &errors.ReqError{Code: "path_not_file", HttpCode: 400, Msg: "path is not a file", Field: "file"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.ReadFileLines("repo1", "a")
			})
		})

		It("should return lines of a file", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			lines := m.ReadFileLines("repo1", "file.txt")
			Expect(lines).To(Equal([]string{"Hello World", "Hello Friend"}))
		})
	})

	Describe(".GetBranches", func() {
		It("should panic if repo name is not provided", func() {
			err := &errors.ReqError{Code: modules.StatusCodeInvalidParam, HttpCode: 400, Msg: "repo name is required", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetBranches("")
			})
		})

		It("should panic if repo does not exist", func() {
			err := &errors.ReqError{Code: modules.StatusCodeInvalidParam, HttpCode: 404, Msg: "repository does not exist", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetBranches("unknown")
			})
		})

		It("should return expected branch(es)", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			lines := m.GetBranches("repo1")
			Expect(lines).To(Equal([]string{"refs/heads/master"}))
		})
	})

	Describe(".GetLatestBranchCommit", func() {
		It("should panic if repo name is not provided", func() {
			err := &errors.ReqError{Code: modules.StatusCodeInvalidParam, HttpCode: 400, Msg: "repo name is required", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetLatestBranchCommit("", "")
			})
		})

		It("should panic if branch name is not provided", func() {
			err := &errors.ReqError{Code: modules.StatusCodeInvalidParam, HttpCode: 400, Msg: "branch name is required", Field: "branch"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetLatestBranchCommit("repo", "")
			})
		})

		It("should panic if repo does not exist", func() {
			err := &errors.ReqError{Code: modules.StatusCodeInvalidParam, HttpCode: 404, Msg: "repository does not exist", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetLatestBranchCommit("unknown", "branch")
			})
		})

		It("should panic if branch does not exist", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			err := &errors.ReqError{Code: "branch_not_found", HttpCode: 404, Msg: "branch does not exist", Field: "branch"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetLatestBranchCommit("repo1", "unknown")
			})
		})

		It("should be successful if branch is known", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			bc := m.GetLatestBranchCommit("repo1", "master")
			Expect(bc).ToNot(BeEmpty())
		})
	})

	Describe(".GetCommits", func() {
		It("should panic if repo name is not provided", func() {
			err := &errors.ReqError{Code: modules.StatusCodeInvalidParam, HttpCode: 400, Msg: "repo name is required", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetCommits("", "")
			})
		})

		It("should panic if branch name is not provided", func() {
			err := &errors.ReqError{Code: modules.StatusCodeInvalidParam, HttpCode: 400, Msg: "branch name is required", Field: "branch"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetCommits("repo", "")
			})
		})

		It("should panic if repo does not exist", func() {
			err := &errors.ReqError{Code: modules.StatusCodeInvalidParam, HttpCode: 404, Msg: "repository does not exist", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetCommits("unknown", "branch")
			})
		})

		It("should panic if branch does not exist", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			err := &errors.ReqError{Code: "branch_not_found", HttpCode: 404, Msg: "branch does not exist", Field: "branch"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetCommits("repo1", "unknown")
			})
		})

		It("should return commits on success", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			bc := m.GetCommits("repo1", "master", 0)
			Expect(bc).ToNot(BeEmpty())
			Expect(bc).To(HaveLen(6))
		})

		It("should return limited commits when limit is > 0", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			bc := m.GetCommits("repo1", "master", 2)
			Expect(bc).ToNot(BeEmpty())
			Expect(bc).To(HaveLen(2))
		})
	})

	Describe(".CountCommits", func() {
		It("should return correct commit count", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			count := m.CountCommits("repo1", "master")
			Expect(count).To(Equal(7))
			count = m.CountCommits("repo1", "cbc329e7e912227d58edea6d6a74d550cd664adf")
			Expect(count).To(Equal(2))
		})
	})

	Describe(".GetCommitAncestors", func() {
		It("should panic if repo name is not provided", func() {
			err := &errors.ReqError{Code: modules.StatusCodeInvalidParam, HttpCode: 400, Msg: "repo name is required", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetCommitAncestors("", "")
			})
		})

		It("should panic if commit hash is not provided", func() {
			err := &errors.ReqError{Code: modules.StatusCodeInvalidParam, HttpCode: 400, Msg: "commit hash is required", Field: "commitHash"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetCommitAncestors("repo", "")
			})
		})

		It("should panic if repo does not exist", func() {
			err := &errors.ReqError{Code: modules.StatusCodeInvalidParam, HttpCode: 404, Msg: "repository does not exist", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetCommitAncestors("unknown", "hash")
			})
		})

		It("should panic if commit does not exist", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			err := &errors.ReqError{Code: "commit_not_found", HttpCode: 404, Msg: "commit does not exist", Field: "commitHash"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetCommitAncestors("repo1", "unknown")
			})
		})

		It("should return commits on success", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			commits := m.GetCommitAncestors("repo1", "aef606780a3f857fdd7fe8270efa547f118bef5f")
			Expect(commits).ToNot(BeEmpty())
			Expect(commits).To(HaveLen(5))
		})

		It("should return limited commits when limit is > 0", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			commits := m.GetCommitAncestors("repo1", "aef606780a3f857fdd7fe8270efa547f118bef5f", 1)
			Expect(commits).ToNot(BeEmpty())
			Expect(commits).To(HaveLen(1))
		})
	})
})
