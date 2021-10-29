package modules_test

import (
	"bytes"
	"fmt"

	"github.com/go-git/go-git/v5"
	config2 "github.com/go-git/go-git/v5/config"
	plumbing2 "github.com/go-git/go-git/v5/plumbing"
	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/cmd/issuecmd"
	"github.com/make-os/kit/cmd/mergecmd"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/crypto/ed25519"
	"github.com/make-os/kit/mocks"
	mocks2 "github.com/make-os/kit/mocks/rpc"
	"github.com/make-os/kit/modules"
	"github.com/make-os/kit/modules/types"
	"github.com/make-os/kit/remote/plumbing"
	remotetypes "github.com/make-os/kit/remote/types"
	"github.com/make-os/kit/testutil"
	"github.com/make-os/kit/types/api"
	"github.com/make-os/kit/types/constants"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/crypto"
	"github.com/make-os/kit/util/errors"
	"github.com/make-os/kit/util/pushtoken"
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
	var mockAccountKeeper *mocks.MockAccountKeeper
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
		mockAccountKeeper = mocks.NewMockAccountKeeper(ctrl)
		mockRepoSyncInfoKeeper = mocks.NewMockRepoSyncInfoKeeper(ctrl)
		mockNSKeeper = mocks.NewMockNamespaceKeeper(ctrl)
		mockLogic.EXPECT().Config().Return(cfg).AnyTimes()
		mockLogic.EXPECT().GetMempoolReactor().Return(mockMempoolReactor).AnyTimes()
		mockLogic.EXPECT().RepoKeeper().Return(mockRepoKeeper).AnyTimes()
		mockLogic.EXPECT().GetRemoteServer().Return(mockRepoSrv).AnyTimes()
		mockLogic.EXPECT().RepoSyncInfoKeeper().Return(mockRepoSyncInfoKeeper).AnyTimes()
		mockLogic.EXPECT().NamespaceKeeper().Return(mockNSKeeper).AnyTimes()
		mockLogic.EXPECT().AccountKeeper().Return(mockAccountKeeper).AnyTimes()
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
			err := &errors.ReqError{Code: "invalid_param", HttpCode: 400, Msg: "1 error(s) decoding:\n\n* 'name' expected type 'string', got unconvertible type 'struct {}', value: '{}'", Field: "params"}
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
			err := &errors.ReqError{Code: "invalid_param", HttpCode: 400, Msg: "1 error(s) decoding:\n\n* 'addresses[0]' expected type 'string', got unconvertible type 'struct {}', value: '{}'", Field: "params"}
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
			err := &errors.ReqError{Code: modules.StatusCodeInvalidParam, HttpCode: 400, Msg: "1 error(s) decoding:\n\n* 'name' expected type 'string', got unconvertible type 'struct {}', value: '{}'", Field: "params"}
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
		It("should panic when height option field was not valid", func() {
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
			err := &errors.ReqError{Code: modules.StatusCodeInvalidParam, HttpCode: 400, Msg: "1 error(s) decoding:\n\n* 'id' expected type 'string', got unconvertible type 'struct {}', value: '{}'", Field: "params"}
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
			err := &errors.ReqError{Code: modules.StatusCodeInvalidParam, HttpCode: 400, Msg: "1 error(s) decoding:\n\n* 'id' expected type 'string', got unconvertible type 'struct {}', value: '{}'", Field: "params"}
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

		It("should panic if repo name was not provided", func() {
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

		It("should return empty list when revision is invalid", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			res := m.ListPath("repo1", ".", "something")
			Expect(res).To(BeEmpty())
		})
	})

	Describe(".ReadFileLines", func() {
		It("should panic if repo name was not provided", func() {
			err := &errors.ReqError{Code: modules.StatusCodeInvalidParam, HttpCode: 400, Msg: "repo name is required", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.ReadFileLines("", "")
			})
		})

		It("should panic if file path was not provided", func() {
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

		It("should panic if file path was not a file", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			err := &errors.ReqError{Code: "path_not_file", HttpCode: 400, Msg: "path is not a file", Field: "file"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.ReadFileLines("repo1", "a")
			})
		})

		It("should return lines of a file", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			lines := m.ReadFileLines("repo1", "file.txt")
			Expect(lines).To(Equal([]string{"Hello World", "Hello Friend", "Hello Degens"}))
		})
	})

	Describe(".GetBranches", func() {
		It("should panic if repo name was not provided", func() {
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
			Expect(lines).To(Equal([]string{"refs/heads/dev", "refs/heads/master"}))
		})
	})

	Describe(".GetLatestBranchCommit", func() {
		It("should panic if repo name was not provided", func() {
			err := &errors.ReqError{Code: modules.StatusCodeInvalidParam, HttpCode: 400, Msg: "repo name is required", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetLatestBranchCommit("", "")
			})
		})

		It("should panic if branch name was not provided", func() {
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
		It("should panic if repo name was not provided", func() {
			err := &errors.ReqError{Code: modules.StatusCodeInvalidParam, HttpCode: 400, Msg: "repo name is required", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetCommits("", "")
			})
		})

		It("should panic if branch name was not provided", func() {
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
			Expect(bc).To(HaveLen(7))
		})

		It("should return limited commits when limit is > 0", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			bc := m.GetCommits("repo1", "master", 2)
			Expect(bc).ToNot(BeEmpty())
			Expect(bc).To(HaveLen(2))
		})
	})

	Describe(".GetCommit", func() {
		It("should panic if repo name was not provided", func() {
			err := &errors.ReqError{Code: modules.StatusCodeInvalidParam, HttpCode: 400, Msg: "repo name is required", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetCommit("", "")
			})
		})

		It("should panic if commit hash was not provided", func() {
			err := &errors.ReqError{Code: modules.StatusCodeInvalidParam, HttpCode: 400, Msg: "commit hash is required", Field: "hash"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetCommit("repo", "")
			})
		})

		It("should panic if repo was not found", func() {
			err := &errors.ReqError{Code: "invalid_param", HttpCode: 404, Msg: "repository does not exist", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetCommit("unknown", "hash")
			})
		})

		It("should panic if commit was not found", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			err := &errors.ReqError{Code: "commit_not_found", HttpCode: 404, Msg: "commit does not exist", Field: "hash"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetCommit("repo1", "f23482ae207b19498049ec7b35c8274c34ba6093")
			})
		})

		It("should not panic if commit was found", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			assert.NotPanics(GinkgoT(), func() {
				hash := "932401fb0bf48f602c501334b773fbc3422ceb31"
				res := m.GetCommit("repo1", hash)
				Expect(res).ToNot(BeNil())
				Expect(res["hash"]).To(Equal(hash))
			})
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
		It("should panic if repo name was not provided", func() {
			err := &errors.ReqError{Code: modules.StatusCodeInvalidParam, HttpCode: 400, Msg: "repo name is required", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetCommitAncestors("", "")
			})
		})

		It("should panic if commit hash was not provided", func() {
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

	Describe(".GetParentsAndCommitDiff", func() {
		It("should panic when repo name was not provided", func() {
			err := &errors.ReqError{Code: "invalid_param", HttpCode: 400, Msg: "repo name is required", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetParentsAndCommitDiff("", "")
			})
		})

		It("should panic when commit hash was not provided", func() {
			err := &errors.ReqError{Code: "invalid_param", HttpCode: 400, Msg: "commit hash is required", Field: "commitHash"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetParentsAndCommitDiff("repo", "")
			})
		})

		It("should panic when repo does not exist", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			err := &errors.ReqError{Code: "invalid_param", HttpCode: 404, Msg: "repository does not exist", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetParentsAndCommitDiff("unknown", "abc")
			})
		})

		It("should panic when unable to get diff", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			err := &errors.ReqError{Code: "commit_not_found", HttpCode: 404, Msg: "commit not found", Field: "commitHash"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.GetParentsAndCommitDiff("repo3", "abc")
			})
		})

		It("should not panic when successful", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			assert.NotPanics(GinkgoT(), func() {
				res := m.GetParentsAndCommitDiff("repo3", "8c427dcc0d582cd7387b4c529185b7c1ab28f20c")
				Expect(res).To(HaveKey("patches"))
				Expect(res["patches"]).To(HaveLen(1))
				Expect(res["patches"].([]map[string]string)[0]).To(HaveKey("a77021f4aead5f5ab8934a94154b0b4da6a551b5"))
				Expect(res["patches"].([]map[string]string)[0]["a77021f4aead5f5ab8934a94154b0b4da6a551b5"]).To(Equal(`diff --git a/file_b.txt b/file_b.txt
new file mode 100644
index 0000000..3b0c2f1
--- /dev/null
+++ b/file_b.txt
@@ -0,0 +1 @@
+We made games
\ No newline at end of file`))
			})
		})
	})

	Describe(".CreateIssue", func() {
		It("should panic when repo name was not provided", func() {
			err := &errors.ReqError{Code: "invalid_param", HttpCode: 400, Msg: "repo name is required", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.CreateIssue("", nil)
			})
		})

		It("should panic when repo was not found", func() {
			err := &errors.ReqError{Code: "invalid_param", HttpCode: 404, Msg: "repository does not exist", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.CreateIssue("unknown", nil)
			})
		})

		It("should panic when unable to find a free post ID", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			m.PostIDFinder = func(_ plumbing.LocalRepo, _ int, _ string) (int, error) {
				return 0, fmt.Errorf("error here")
			}
			err := &errors.ReqError{Code: "server_err", HttpCode: 500, Msg: "error here", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.CreateIssue("repo3", nil)
			})
		})

		It("should panic when unable to clone repository", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			var mockRepo = mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().RefGet(plumbing.MakeIssueReference("1")).Return("", plumbing2.ErrReferenceNotFound)
			mockRepo.EXPECT().Clone(plumbing.CloneOptions{
				Bare:          false,
				ReferenceName: "",
				Depth:         1,
			}).Return(nil, "", fmt.Errorf("error here"))
			m.GetLocalRepo = func(_, _ string) (plumbing.LocalRepo, error) {
				return mockRepo, nil
			}
			err := &errors.ReqError{Code: "server_err", HttpCode: 500, Msg: "failed to clone repo: error here", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.CreateIssue("repo3", map[string]interface{}{
					"id": 1,
				})
			})
		})

		It("should panic when unable to create issue", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")

			var mockRepo = mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().RefGet(plumbing.MakeIssueReference("1")).Return("", plumbing2.ErrReferenceNotFound)
			m.GetLocalRepo = func(_, _ string) (plumbing.LocalRepo, error) { return mockRepo, nil }

			var mockCloneRepo = mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().Clone(gomock.Any()).Return(mockCloneRepo, "", nil)
			mockCloneRepo.EXPECT().Delete()

			m.IssueCreate = func(_ plumbing.LocalRepo, _ *issuecmd.IssueCreateArgs) (*issuecmd.IssueCreateResult, error) {
				return nil, fmt.Errorf("error here")
			}

			err := &errors.ReqError{Code: "server_err", HttpCode: 500, Msg: "error here", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.CreateIssue("repo3", map[string]interface{}{
					"id": 1,
				})
			})
		})

		It("should panic when unable to get create issue reference", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")

			issueRef := plumbing.MakeIssueReference("1")
			var mockRepo = mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().RefGet(issueRef).Return("", plumbing2.ErrReferenceNotFound)
			m.GetLocalRepo = func(_, _ string) (plumbing.LocalRepo, error) { return mockRepo, nil }

			var mockCloneRepo = mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().Clone(gomock.Any()).Return(mockCloneRepo, "", nil)
			mockCloneRepo.EXPECT().Delete()

			m.IssueCreate = func(_ plumbing.LocalRepo, _ *issuecmd.IssueCreateArgs) (*issuecmd.IssueCreateResult, error) {
				return &issuecmd.IssueCreateResult{Reference: issueRef}, nil
			}

			mockCloneRepo.EXPECT().RefGet(issueRef).Return("", fmt.Errorf("error here"))

			err := &errors.ReqError{Code: "server_err", HttpCode: 500, Msg: "error here", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.CreateIssue("repo3", map[string]interface{}{
					"id": 1,
				})
			})
		})

		It("should panic when unable to get create issue reference", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")

			issueRef := plumbing.MakeIssueReference("1")
			var mockRepo = mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().RefGet(issueRef).Return("", plumbing2.ErrReferenceNotFound)
			m.GetLocalRepo = func(_, _ string) (plumbing.LocalRepo, error) { return mockRepo, nil }

			var mockCloneRepo = mocks.NewMockLocalRepo(ctrl)
			mockCloneRepo.EXPECT().GetPath().Return("/repo/path")
			mockRepo.EXPECT().Clone(gomock.Any()).Return(mockCloneRepo, "", nil)

			m.IssueCreate = func(_ plumbing.LocalRepo, _ *issuecmd.IssueCreateArgs) (*issuecmd.IssueCreateResult, error) {
				return &issuecmd.IssueCreateResult{Reference: issueRef}, nil
			}

			mockCloneRepo.EXPECT().RefGet(issueRef).Return("hash123", nil)

			mockTempRepoMgr := mocks.NewMockTempRepoManager(ctrl)
			mockTempRepoMgr.EXPECT().Add("/repo/path").Return("repoId_123")
			mockRepoSrv.EXPECT().GetTempRepoManager().Return(mockTempRepoMgr)

			assert.NotPanics(GinkgoT(), func() {
				res := m.CreateIssue("repo3", map[string]interface{}{
					"id": 1,
				})
				Expect(res).To(Equal(util.Map(map[string]interface{}{
					"hash":      "hash123",
					"reference": issueRef,
					"repoID":    "repoId_123",
				})))
			})
		})
	})

	Describe(".CreateMergeRequest()", func() {
		It("should panic when repo name was not provided", func() {
			err := &errors.ReqError{Code: "invalid_param", HttpCode: 400, Msg: "repo name is required", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.CreateMergeRequest("", nil)
			})
		})

		It("should panic when repo was not found", func() {
			err := &errors.ReqError{Code: "invalid_param", HttpCode: 404, Msg: "repository does not exist", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.CreateMergeRequest("unknown", nil)
			})
		})

		It("should panic when unable to find a free post ID", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			m.PostIDFinder = func(_ plumbing.LocalRepo, _ int, _ string) (int, error) {
				return 0, fmt.Errorf("error here")
			}
			err := &errors.ReqError{Code: "server_err", HttpCode: 500, Msg: "error here", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.CreateMergeRequest("repo3", nil)
			})
		})

		It("should panic when unable to clone repository", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			var mockRepo = mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().RefGet(plumbing.MakeMergeRequestReference("1")).Return("", plumbing2.ErrReferenceNotFound)
			mockRepo.EXPECT().Clone(plumbing.CloneOptions{
				Bare:          false,
				ReferenceName: "",
				Depth:         1,
			}).Return(nil, "", fmt.Errorf("error here"))
			m.GetLocalRepo = func(_, _ string) (plumbing.LocalRepo, error) {
				return mockRepo, nil
			}
			err := &errors.ReqError{Code: "server_err", HttpCode: 500, Msg: "failed to clone repo: error here", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.CreateMergeRequest("repo3", map[string]interface{}{
					"id": 1,
				})
			})
		})

		It("should panic when unable to create merge request", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")

			var mockRepo = mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().RefGet(plumbing.MakeMergeRequestReference("1")).Return("", plumbing2.ErrReferenceNotFound)
			m.GetLocalRepo = func(_, _ string) (plumbing.LocalRepo, error) { return mockRepo, nil }

			var mockCloneRepo = mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().Clone(gomock.Any()).Return(mockCloneRepo, "", nil)
			mockCloneRepo.EXPECT().Delete()

			m.MergeRequestCreate = func(r plumbing.LocalRepo, args *mergecmd.MergeRequestCreateArgs) (*mergecmd.MergeRequestCreateResult, error) {
				return nil, fmt.Errorf("error here")
			}

			err := &errors.ReqError{Code: "server_err", HttpCode: 500, Msg: "error here", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.CreateMergeRequest("repo3", map[string]interface{}{
					"id": 1,
				})
			})
		})

		It("should panic when unable to get create merge request reference", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")

			mrRef := plumbing.MakeMergeRequestReference("1")
			var mockRepo = mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().RefGet(mrRef).Return("", plumbing2.ErrReferenceNotFound)
			m.GetLocalRepo = func(_, _ string) (plumbing.LocalRepo, error) { return mockRepo, nil }

			var mockCloneRepo = mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().Clone(gomock.Any()).Return(mockCloneRepo, "", nil)
			mockCloneRepo.EXPECT().Delete()

			m.MergeRequestCreate = func(r plumbing.LocalRepo, args *mergecmd.MergeRequestCreateArgs) (*mergecmd.MergeRequestCreateResult, error) {
				return &mergecmd.MergeRequestCreateResult{Reference: mrRef}, nil
			}

			mockCloneRepo.EXPECT().RefGet(mrRef).Return("", fmt.Errorf("error here"))

			err := &errors.ReqError{Code: "server_err", HttpCode: 500, Msg: "error here", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.CreateMergeRequest("repo3", map[string]interface{}{
					"id": 1,
				})
			})
		})

		It("should panic when unable to get create issue reference", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")

			mrRef := plumbing.MakeMergeRequestReference("1")
			var mockRepo = mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().RefGet(mrRef).Return("", plumbing2.ErrReferenceNotFound)
			m.GetLocalRepo = func(_, _ string) (plumbing.LocalRepo, error) { return mockRepo, nil }

			var mockCloneRepo = mocks.NewMockLocalRepo(ctrl)
			mockCloneRepo.EXPECT().GetPath().Return("/repo/path")
			mockRepo.EXPECT().Clone(gomock.Any()).Return(mockCloneRepo, "", nil)

			m.MergeRequestCreate = func(r plumbing.LocalRepo, args *mergecmd.MergeRequestCreateArgs) (*mergecmd.MergeRequestCreateResult, error) {
				return &mergecmd.MergeRequestCreateResult{Reference: mrRef}, nil
			}

			mockCloneRepo.EXPECT().RefGet(mrRef).Return("hash123", nil)

			mockTempRepoMgr := mocks.NewMockTempRepoManager(ctrl)
			mockTempRepoMgr.EXPECT().Add("/repo/path").Return("repoId_123")
			mockRepoSrv.EXPECT().GetTempRepoManager().Return(mockTempRepoMgr)

			assert.NotPanics(GinkgoT(), func() {
				res := m.CreateMergeRequest("repo3", map[string]interface{}{
					"id": 1,
				})
				Expect(res).To(Equal(util.Map(map[string]interface{}{
					"hash":      "hash123",
					"reference": mrRef,
					"repoID":    "repoId_123",
				})))
			})
		})
	})

	Describe(".CloseIssue", func() {
		It("should panic when repo name was not provided", func() {
			err := &errors.ReqError{Code: "invalid_param", HttpCode: 400, Msg: "repo name is required", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.CloseIssue("", plumbing.MakeIssueReference(1))
			})
		})

		It("should panic when repo was not found", func() {
			err := &errors.ReqError{Code: "invalid_param", HttpCode: 404, Msg: "repository does not exist", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.CloseIssue("unknown", plumbing.MakeIssueReference(1))
			})
		})

		It("should panic when issue reference was not found", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			err := &errors.ReqError{Code: "issue_not_found", HttpCode: 404, Msg: "issue not found", Field: "reference"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.CloseIssue("repo3", plumbing.MakeIssueReference(1))
			})
		})

		It("should panic when unable to clone repository", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			var mockRepo = mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().RefGet(plumbing.MakeIssueReference("1")).Return("", nil)
			mockRepo.EXPECT().Clone(plumbing.CloneOptions{
				Bare:          false,
				ReferenceName: "",
				Depth:         1,
			}).Return(nil, "", fmt.Errorf("error here"))
			m.GetLocalRepo = func(_, _ string) (plumbing.LocalRepo, error) {
				return mockRepo, nil
			}
			err := &errors.ReqError{Code: "server_err", HttpCode: 500, Msg: "failed to clone repo: error here", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.CloseIssue("repo3", plumbing.MakeIssueReference(1))
			})
		})

		It("should panic when unable to close issue", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			var mockRepo = mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().RefGet(plumbing.MakeIssueReference("1")).Return("", nil)
			var mockCloneRepo = mocks.NewMockLocalRepo(ctrl)
			mockCloneRepo.EXPECT().Delete()
			mockRepo.EXPECT().Clone(gomock.Any()).Return(mockCloneRepo, "", nil)
			m.IssueClose = func(r plumbing.LocalRepo, args *issuecmd.IssueCloseArgs) (*issuecmd.IssueCloseResult, error) {
				return nil, fmt.Errorf("error here; cant close")
			}
			m.GetLocalRepo = func(_, _ string) (plumbing.LocalRepo, error) {
				return mockRepo, nil
			}
			err := &errors.ReqError{Code: "server_err", HttpCode: 500, Msg: "error here; cant close", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.CloseIssue("repo3", plumbing.MakeIssueReference(1))
			})
		})

		It("should panic when unable to get hash of updated reference", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			ref := plumbing.MakeIssueReference("1")
			var mockRepo = mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().RefGet(ref).Return("", nil)
			var mockCloneRepo = mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().Clone(gomock.Any()).Return(mockCloneRepo, "", nil)
			m.IssueClose = func(r plumbing.LocalRepo, args *issuecmd.IssueCloseArgs) (*issuecmd.IssueCloseResult, error) {
				return &issuecmd.IssueCloseResult{Reference: ref}, nil
			}
			m.GetLocalRepo = func(_, _ string) (plumbing.LocalRepo, error) {
				return mockRepo, nil
			}
			mockCloneRepo.EXPECT().RefGet(ref).Return("", fmt.Errorf("error here"))
			mockCloneRepo.EXPECT().Delete()
			err := &errors.ReqError{Code: "server_err", HttpCode: 500, Msg: "error here", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.CloseIssue("repo3", plumbing.MakeIssueReference(1))
			})
		})

		It("should add repo to temporary repo manager; "+
			"return ID from temporary repo manager;"+
			"return name and hash of new issue reference;", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			ref := plumbing.MakeIssueReference("1")
			hash := "hash_123"

			var mockRepo = mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().RefGet(ref).Return("", nil)
			var mockCloneRepo = mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().Clone(gomock.Any()).Return(mockCloneRepo, "", nil)
			m.IssueClose = func(r plumbing.LocalRepo, args *issuecmd.IssueCloseArgs) (*issuecmd.IssueCloseResult, error) {
				return &issuecmd.IssueCloseResult{Reference: ref}, nil
			}
			m.GetLocalRepo = func(_, _ string) (plumbing.LocalRepo, error) {
				return mockRepo, nil
			}

			mockCloneRepo.EXPECT().RefGet(ref).Return(hash, nil)

			tempRepoId := "repoId_123"
			mockCloneRepo.EXPECT().GetPath().Return("/repo/path")
			mockTempRepoMgr := mocks.NewMockTempRepoManager(ctrl)
			mockTempRepoMgr.EXPECT().Add("/repo/path").Return(tempRepoId)
			mockRepoSrv.EXPECT().GetTempRepoManager().Return(mockTempRepoMgr)

			assert.NotPanics(GinkgoT(), func() {
				res := m.CloseIssue("repo3", plumbing.MakeIssueReference(1))
				Expect(res["reference"]).To(Equal(ref))
				Expect(res["hash"]).To(Equal(hash))
				Expect(res["repoID"]).To(Equal(tempRepoId))
			})
		})
	})

	Describe(".ReadIssue", func() {
		It("should panic when repo name was not provided", func() {
			err := &errors.ReqError{Code: "invalid_param", HttpCode: 400, Msg: "repo name is required", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.ReadIssue("", plumbing.MakeIssueReference(1))
			})
		})

		It("should panic when repo was not found", func() {
			err := &errors.ReqError{Code: "invalid_param", HttpCode: 404, Msg: "repository does not exist", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.ReadIssue("unknown", plumbing.MakeIssueReference(1))
			})
		})

		It("should panic when unable to get the issue", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			m.IssueRead = func(_ plumbing.LocalRepo, _ *issuecmd.IssueReadArgs) (plumbing.Comments, error) {
				return nil, fmt.Errorf("error here")
			}
			err := &errors.ReqError{Code: "server_err", HttpCode: 500, Msg: "error here", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.ReadIssue("repo3", plumbing.MakeIssueReference(1))
			})
		})

		It("should not panic on success", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			m.IssueRead = func(_ plumbing.LocalRepo, _ *issuecmd.IssueReadArgs) (plumbing.Comments, error) {
				return []*plumbing.Comment{
					{Author: "a"},
				}, nil
			}
			assert.NotPanics(GinkgoT(), func() {
				res := m.ReadIssue("repo3", plumbing.MakeIssueReference(1))
				Expect(res).To(HaveLen(1))
				Expect(res[0]).To(HaveKey("reference"))
				Expect(res[0]["reference"]).To(Equal("a"))
			})
		})
	})

	Describe(".ReopenIssue", func() {
		It("should panic when repo name was not provided", func() {
			err := &errors.ReqError{Code: "invalid_param", HttpCode: 400, Msg: "repo name is required", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.ReopenIssue("", plumbing.MakeIssueReference(1))
			})
		})

		It("should panic when repo was not found", func() {
			err := &errors.ReqError{Code: "invalid_param", HttpCode: 404, Msg: "repository does not exist", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.ReopenIssue("unknown", plumbing.MakeIssueReference(1))
			})
		})

		It("should panic when issue reference was not found", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			err := &errors.ReqError{Code: "issue_not_found", HttpCode: 404, Msg: "issue not found", Field: "reference"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.ReopenIssue("repo3", plumbing.MakeIssueReference(1))
			})
		})
		//
		It("should panic when unable to clone repository", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			var mockRepo = mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().RefGet(plumbing.MakeIssueReference("1")).Return("", nil)
			mockRepo.EXPECT().Clone(plumbing.CloneOptions{
				Bare:          false,
				ReferenceName: "",
				Depth:         1,
			}).Return(nil, "", fmt.Errorf("error here"))
			m.GetLocalRepo = func(_, _ string) (plumbing.LocalRepo, error) {
				return mockRepo, nil
			}
			err := &errors.ReqError{Code: "server_err", HttpCode: 500, Msg: "failed to clone repo: error here", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.ReopenIssue("repo3", plumbing.MakeIssueReference(1))
			})
		})

		It("should panic when unable to reopen issue", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			var mockRepo = mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().RefGet(plumbing.MakeIssueReference("1")).Return("", nil)
			var mockCloneRepo = mocks.NewMockLocalRepo(ctrl)
			mockCloneRepo.EXPECT().Delete()
			mockRepo.EXPECT().Clone(gomock.Any()).Return(mockCloneRepo, "", nil)
			m.IssueReopen = func(r plumbing.LocalRepo, args *issuecmd.IssueReopenArgs) (*issuecmd.IssueReopenResult, error) {
				return nil, fmt.Errorf("error here; cant reopen")
			}
			m.GetLocalRepo = func(_, _ string) (plumbing.LocalRepo, error) {
				return mockRepo, nil
			}
			err := &errors.ReqError{Code: "server_err", HttpCode: 500, Msg: "error here; cant reopen", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.ReopenIssue("repo3", plumbing.MakeIssueReference(1))
			})
		})

		It("should panic when unable to get hash of updated reference", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			ref := plumbing.MakeIssueReference("1")
			var mockRepo = mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().RefGet(ref).Return("", nil)
			var mockCloneRepo = mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().Clone(gomock.Any()).Return(mockCloneRepo, "", nil)
			m.IssueReopen = func(r plumbing.LocalRepo, args *issuecmd.IssueReopenArgs) (*issuecmd.IssueReopenResult, error) {
				return &issuecmd.IssueReopenResult{Reference: ref}, nil
			}
			m.GetLocalRepo = func(_, _ string) (plumbing.LocalRepo, error) {
				return mockRepo, nil
			}
			mockCloneRepo.EXPECT().RefGet(ref).Return("", fmt.Errorf("error here"))
			mockCloneRepo.EXPECT().Delete()
			err := &errors.ReqError{Code: "server_err", HttpCode: 500, Msg: "error here", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.ReopenIssue("repo3", plumbing.MakeIssueReference(1))
			})
		})

		It("should add repo to temporary repo manager; "+
			"return ID from temporary repo manager;"+
			"return name and hash of new merge request reference;", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			ref := plumbing.MakeIssueReference("1")
			hash := "hash_123"

			var mockRepo = mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().RefGet(ref).Return("", nil)
			var mockCloneRepo = mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().Clone(gomock.Any()).Return(mockCloneRepo, "", nil)
			m.IssueReopen = func(r plumbing.LocalRepo, args *issuecmd.IssueReopenArgs) (*issuecmd.IssueReopenResult, error) {
				return &issuecmd.IssueReopenResult{Reference: ref}, nil
			}
			m.GetLocalRepo = func(_, _ string) (plumbing.LocalRepo, error) {
				return mockRepo, nil
			}

			mockCloneRepo.EXPECT().RefGet(ref).Return(hash, nil)

			tempRepoId := "repoId_123"
			mockCloneRepo.EXPECT().GetPath().Return("/repo/path")
			mockTempRepoMgr := mocks.NewMockTempRepoManager(ctrl)
			mockTempRepoMgr.EXPECT().Add("/repo/path").Return(tempRepoId)
			mockRepoSrv.EXPECT().GetTempRepoManager().Return(mockTempRepoMgr)

			assert.NotPanics(GinkgoT(), func() {
				res := m.ReopenIssue("repo3", plumbing.MakeIssueReference(1))
				Expect(res["reference"]).To(Equal(ref))
				Expect(res["hash"]).To(Equal(hash))
				Expect(res["repoID"]).To(Equal(tempRepoId))
			})
		})
	})

	Describe(".ListIssues", func() {
		It("should panic when repo name was not provided", func() {
			err := &errors.ReqError{Code: "invalid_param", HttpCode: 400, Msg: "repo name is required", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.ListIssues("")
			})
		})

		It("should panic when repo was not found", func() {
			err := &errors.ReqError{Code: "invalid_param", HttpCode: 404, Msg: "repository does not exist", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.ListIssues("unknown")
			})
		})

		It("should panic when unable to list issues", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			m.IssueList = func(_ plumbing.LocalRepo, _ *issuecmd.IssueListArgs) (plumbing.Posts, error) {
				return nil, fmt.Errorf("error here")
			}
			err := &errors.ReqError{Code: "server_err", HttpCode: 500, Msg: "error here", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.ListIssues("repo3")
			})
		})

		It("should not panic on success", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			m.IssueList = func(_ plumbing.LocalRepo, _ *issuecmd.IssueListArgs) (plumbing.Posts, error) {
				return []plumbing.PostEntry{
					&plumbing.Post{Title: "title"},
				}, nil
			}
			assert.NotPanics(GinkgoT(), func() {
				res := m.ListIssues("repo3")
				Expect(res).To(HaveLen(1))
				Expect(res[0]).To(HaveKey("title"))
				Expect(res[0]["title"]).To(Equal("title"))
			})
		})
	})

	Describe(".CloseMergeRequest()", func() {
		It("should panic when repo name was not provided", func() {
			err := &errors.ReqError{Code: "invalid_param", HttpCode: 400, Msg: "repo name is required", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.CloseMergeRequest("", plumbing.MakeMergeRequestReference(1))
			})
		})

		It("should panic when repo was not found", func() {
			err := &errors.ReqError{Code: "invalid_param", HttpCode: 404, Msg: "repository does not exist", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.CloseMergeRequest("unknown", plumbing.MakeMergeRequestReference(1))
			})
		})

		It("should panic when merge request reference was not found", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			err := &errors.ReqError{Code: "merge_request_not_found", HttpCode: 404, Msg: "merge request not found", Field: "reference"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.CloseMergeRequest("repo3", plumbing.MakeMergeRequestReference(1))
			})
		})

		It("should panic when unable to clone repository", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			var mockRepo = mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().RefGet(plumbing.MakeMergeRequestReference("1")).Return("", nil)
			mockRepo.EXPECT().Clone(plumbing.CloneOptions{
				Bare:          false,
				ReferenceName: "",
				Depth:         1,
			}).Return(nil, "", fmt.Errorf("error here"))
			m.GetLocalRepo = func(_, _ string) (plumbing.LocalRepo, error) {
				return mockRepo, nil
			}
			err := &errors.ReqError{Code: "server_err", HttpCode: 500, Msg: "failed to clone repo: error here", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.CloseMergeRequest("repo3", plumbing.MakeMergeRequestReference(1))
			})
		})

		It("should panic when unable to close merge request", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			var mockRepo = mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().RefGet(plumbing.MakeMergeRequestReference("1")).Return("", nil)
			var mockCloneRepo = mocks.NewMockLocalRepo(ctrl)
			mockCloneRepo.EXPECT().Delete()
			mockRepo.EXPECT().Clone(gomock.Any()).Return(mockCloneRepo, "", nil)
			m.MergeRequestClose = func(r plumbing.LocalRepo, args *mergecmd.MergeReqCloseArgs) (*mergecmd.MergeReqCloseResult, error) {
				return nil, fmt.Errorf("error here; cant close")
			}
			m.GetLocalRepo = func(_, _ string) (plumbing.LocalRepo, error) {
				return mockRepo, nil
			}
			err := &errors.ReqError{Code: "server_err", HttpCode: 500, Msg: "error here; cant close", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.CloseMergeRequest("repo3", plumbing.MakeMergeRequestReference(1))
			})
		})

		It("should panic when unable to get reference of created merge request", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			ref := plumbing.MakeMergeRequestReference("1")
			var mockRepo = mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().RefGet(ref).Return("", nil)
			var mockCloneRepo = mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().Clone(gomock.Any()).Return(mockCloneRepo, "", nil)
			m.MergeRequestClose = func(r plumbing.LocalRepo, args *mergecmd.MergeReqCloseArgs) (*mergecmd.MergeReqCloseResult, error) {
				return &mergecmd.MergeReqCloseResult{Reference: ref}, nil
			}
			m.GetLocalRepo = func(_, _ string) (plumbing.LocalRepo, error) {
				return mockRepo, nil
			}
			mockCloneRepo.EXPECT().RefGet(ref).Return("", fmt.Errorf("error here"))
			mockCloneRepo.EXPECT().Delete()
			err := &errors.ReqError{Code: "server_err", HttpCode: 500, Msg: "error here", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.CloseMergeRequest("repo3", ref)
			})
		})

		It("should add repo to temporary repo manager; "+
			"return ID from temporary repo manager;"+
			"return name and hash of new merge request reference;", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			ref := plumbing.MakeMergeRequestReference("1")
			hash := "hash_123"

			var mockRepo = mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().RefGet(ref).Return("", nil)
			var mockCloneRepo = mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().Clone(gomock.Any()).Return(mockCloneRepo, "", nil)
			m.MergeRequestClose = func(r plumbing.LocalRepo, args *mergecmd.MergeReqCloseArgs) (*mergecmd.MergeReqCloseResult, error) {
				return &mergecmd.MergeReqCloseResult{Reference: ref}, nil
			}
			m.GetLocalRepo = func(_, _ string) (plumbing.LocalRepo, error) {
				return mockRepo, nil
			}

			mockCloneRepo.EXPECT().RefGet(ref).Return(hash, nil)

			tempRepoId := "repoId_123"
			mockCloneRepo.EXPECT().GetPath().Return("/repo/path")
			mockTempRepoMgr := mocks.NewMockTempRepoManager(ctrl)
			mockTempRepoMgr.EXPECT().Add("/repo/path").Return(tempRepoId)
			mockRepoSrv.EXPECT().GetTempRepoManager().Return(mockTempRepoMgr)

			assert.NotPanics(GinkgoT(), func() {
				res := m.CloseMergeRequest("repo3", ref)
				Expect(res["reference"]).To(Equal(ref))
				Expect(res["hash"]).To(Equal(hash))
				Expect(res["repoID"]).To(Equal(tempRepoId))
			})
		})
	})

	Describe(".ReopenMergeRequest()", func() {
		It("should panic when repo name was not provided", func() {
			err := &errors.ReqError{Code: "invalid_param", HttpCode: 400, Msg: "repo name is required", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.ReopenMergeRequest("", plumbing.MakeMergeRequestReference(1))
			})
		})

		It("should panic when repo was not found", func() {
			err := &errors.ReqError{Code: "invalid_param", HttpCode: 404, Msg: "repository does not exist", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.ReopenMergeRequest("unknown", plumbing.MakeMergeRequestReference(1))
			})
		})

		It("should panic when issue reference was not found", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			err := &errors.ReqError{Code: "merge_request_not_found", HttpCode: 404, Msg: "merge request not found", Field: "reference"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.ReopenMergeRequest("repo3", plumbing.MakeMergeRequestReference(1))
			})
		})

		It("should panic when unable to clone repository", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			var mockRepo = mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().RefGet(plumbing.MakeMergeRequestReference("1")).Return("", nil)
			mockRepo.EXPECT().Clone(plumbing.CloneOptions{
				Bare:          false,
				ReferenceName: "",
				Depth:         1,
			}).Return(nil, "", fmt.Errorf("error here"))
			m.GetLocalRepo = func(_, _ string) (plumbing.LocalRepo, error) {
				return mockRepo, nil
			}
			err := &errors.ReqError{Code: "server_err", HttpCode: 500, Msg: "failed to clone repo: error here", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.ReopenMergeRequest("repo3", plumbing.MakeMergeRequestReference(1))
			})
		})

		It("should panic when unable to reopen merge request", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			var mockRepo = mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().RefGet(plumbing.MakeMergeRequestReference("1")).Return("", nil)
			var mockCloneRepo = mocks.NewMockLocalRepo(ctrl)
			mockCloneRepo.EXPECT().Delete()
			mockRepo.EXPECT().Clone(gomock.Any()).Return(mockCloneRepo, "", nil)
			m.MergeRequestReopen = func(r plumbing.LocalRepo, args *mergecmd.MergeReqReopenArgs) (*mergecmd.MergeReqReopenResult, error) {
				return nil, fmt.Errorf("error here; cant reopen")
			}
			m.GetLocalRepo = func(_, _ string) (plumbing.LocalRepo, error) {
				return mockRepo, nil
			}
			err := &errors.ReqError{Code: "server_err", HttpCode: 500, Msg: "error here; cant reopen", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.ReopenMergeRequest("repo3", plumbing.MakeMergeRequestReference(1))
			})
		})

		It("should panic when unable to get hash of updated reference", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			ref := plumbing.MakeMergeRequestReference("1")
			var mockRepo = mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().RefGet(ref).Return("", nil)
			var mockCloneRepo = mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().Clone(gomock.Any()).Return(mockCloneRepo, "", nil)
			m.MergeRequestReopen = func(r plumbing.LocalRepo, args *mergecmd.MergeReqReopenArgs) (*mergecmd.MergeReqReopenResult, error) {
				return &mergecmd.MergeReqReopenResult{Reference: ref}, nil
			}
			m.GetLocalRepo = func(_, _ string) (plumbing.LocalRepo, error) {
				return mockRepo, nil
			}
			mockCloneRepo.EXPECT().RefGet(ref).Return("", fmt.Errorf("error here"))
			mockCloneRepo.EXPECT().Delete()
			err := &errors.ReqError{Code: "server_err", HttpCode: 500, Msg: "error here", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.ReopenMergeRequest("repo3", plumbing.MakeMergeRequestReference(1))
			})
		})

		It("should add repo to temporary repo manager; "+
			"return ID from temporary repo manager;"+
			"return name and hash of new merge request reference;", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			ref := plumbing.MakeMergeRequestReference("1")
			hash := "hash_123"

			var mockRepo = mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().RefGet(ref).Return("", nil)
			var mockCloneRepo = mocks.NewMockLocalRepo(ctrl)
			mockRepo.EXPECT().Clone(gomock.Any()).Return(mockCloneRepo, "", nil)
			m.MergeRequestReopen = func(r plumbing.LocalRepo, args *mergecmd.MergeReqReopenArgs) (*mergecmd.MergeReqReopenResult, error) {
				return &mergecmd.MergeReqReopenResult{Reference: ref}, nil
			}
			m.GetLocalRepo = func(_, _ string) (plumbing.LocalRepo, error) {
				return mockRepo, nil
			}

			mockCloneRepo.EXPECT().RefGet(ref).Return(hash, nil)

			tempRepoId := "repoId_123"
			mockCloneRepo.EXPECT().GetPath().Return("/repo/path")
			mockTempRepoMgr := mocks.NewMockTempRepoManager(ctrl)
			mockTempRepoMgr.EXPECT().Add("/repo/path").Return(tempRepoId)
			mockRepoSrv.EXPECT().GetTempRepoManager().Return(mockTempRepoMgr)

			assert.NotPanics(GinkgoT(), func() {
				res := m.ReopenMergeRequest("repo3", plumbing.MakeMergeRequestReference(1))
				Expect(res["reference"]).To(Equal(ref))
				Expect(res["hash"]).To(Equal(hash))
				Expect(res["repoID"]).To(Equal(tempRepoId))
			})
		})
	})

	Describe(".ListMergeRequests", func() {
		It("should panic when repo name was not provided", func() {
			err := &errors.ReqError{Code: "invalid_param", HttpCode: 400, Msg: "repo name is required", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.ListMergeRequests("")
			})
		})

		It("should panic when repo was not found", func() {
			err := &errors.ReqError{Code: "invalid_param", HttpCode: 404, Msg: "repository does not exist", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.ListMergeRequests("unknown")
			})
		})

		It("should panic when unable to list issues", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			m.MergeRequestList = func(_ plumbing.LocalRepo, _ *mergecmd.MergeRequestListArgs) (plumbing.Posts, error) {
				return nil, fmt.Errorf("error here")
			}
			err := &errors.ReqError{Code: "server_err", HttpCode: 500, Msg: "error here", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.ListMergeRequests("repo3")
			})
		})

		It("should not panic on success", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			m.MergeRequestList = func(_ plumbing.LocalRepo, _ *mergecmd.MergeRequestListArgs) (plumbing.Posts, error) {
				return []plumbing.PostEntry{
					&plumbing.Post{Title: "title"},
				}, nil
			}
			assert.NotPanics(GinkgoT(), func() {
				res := m.ListMergeRequests("repo3")
				Expect(res).To(HaveLen(1))
				Expect(res[0]).To(HaveKey("title"))
				Expect(res[0]["title"]).To(Equal("title"))
			})
		})
	})

	Describe(".ReadMergeRequest()", func() {
		It("should panic when repo name was not provided", func() {
			err := &errors.ReqError{Code: "invalid_param", HttpCode: 400, Msg: "repo name is required", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.ReadMergeRequest("", plumbing.MakeMergeRequestReference(1))
			})
		})

		It("should panic when repo was not found", func() {
			err := &errors.ReqError{Code: "invalid_param", HttpCode: 404, Msg: "repository does not exist", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.ReadMergeRequest("unknown", plumbing.MakeMergeRequestReference(1))
			})
		})

		It("should panic when unable to get the merge request", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			m.MergeRequestRead = func(_ plumbing.LocalRepo, _ *mergecmd.MergeRequestReadArgs) (plumbing.Comments, error) {
				return nil, fmt.Errorf("error here")
			}
			err := &errors.ReqError{Code: "server_err", HttpCode: 500, Msg: "error here", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.ReadMergeRequest("repo3", plumbing.MakeMergeRequestReference(1))
			})
		})

		It("should not panic on success", func() {
			cfg.SetRepoRoot("../remote/repo/testdata")
			m.MergeRequestRead = func(_ plumbing.LocalRepo, _ *mergecmd.MergeRequestReadArgs) (plumbing.Comments, error) {
				return []*plumbing.Comment{
					{Author: "a"},
				}, nil
			}
			assert.NotPanics(GinkgoT(), func() {
				res := m.ReadMergeRequest("repo3", plumbing.MakeMergeRequestReference(1))
				Expect(res).To(HaveLen(1))
				Expect(res[0]).To(HaveKey("reference"))
				Expect(res[0]["reference"]).To(Equal("a"))
			})
		})
	})

	Describe(".Push", func() {
		It("should panic if id is not associated with a temporary repo", func() {
			param := map[string]interface{}{"id": "repo_123"}
			mockTempRepoMgr := mocks.NewMockTempRepoManager(ctrl)
			mockRepoSrv.EXPECT().GetTempRepoManager().Return(mockTempRepoMgr)
			mockTempRepoMgr.EXPECT().GetPath(param["id"]).Return("")
			err := &errors.ReqError{Code: "invalid_temp_repo_id", HttpCode: 404, Msg: "id is expired or invalid", Field: "id"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Push(param, "privKey")
			})
		})

		It("should panic if reference is not a branch, note or tag reference", func() {
			param := map[string]interface{}{"id": "repo_123", "reference": "some/type/of/reference"}
			mockTempRepoMgr := mocks.NewMockTempRepoManager(ctrl)
			mockRepoSrv.EXPECT().GetTempRepoManager().Return(mockTempRepoMgr)
			mockTempRepoMgr.EXPECT().GetPath(param["id"]).Return("/path/repo")
			err := &errors.ReqError{Code: "invalid_reference_name", HttpCode: 400, Msg: "reference name is not valid", Field: "reference"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Push(param, "privKey")
			})
		})

		It("should panic if private key was not set or is invalid", func() {
			param := map[string]interface{}{"id": "repo_123", "reference": "refs/heads/master"}
			mockTempRepoMgr := mocks.NewMockTempRepoManager(ctrl)
			mockRepoSrv.EXPECT().GetTempRepoManager().Return(mockTempRepoMgr).Times(2)
			mockTempRepoMgr.EXPECT().GetPath(param["id"]).Return("/path/repo").Times(2)
			err := &errors.ReqError{Code: "invalid_private_key", HttpCode: 400, Msg: "private key is required", Field: "privateKeyOrPushToken"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Push(param, "")
			})

			err = &errors.ReqError{Code: "invalid_private_key", HttpCode: 400, Msg: "private key or push token is not a valid", Field: "privateKeyOrPushToken"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Push(param, "invalid_pk")
			})
		})

		It("should panic if repo was not found", func() {
			key := ed25519.NewKeyFromIntSeed(1)
			param := map[string]interface{}{"id": "repo_123", "reference": "refs/heads/master"}
			mockTempRepoMgr := mocks.NewMockTempRepoManager(ctrl)
			mockRepoSrv.EXPECT().GetTempRepoManager().Return(mockTempRepoMgr).Times(2)
			mockTempRepoMgr.EXPECT().GetPath(param["id"]).Return("/path/repo").Times(2)
			m.GetLocalRepo = func(_, path string) (plumbing.LocalRepo, error) {
				Expect(path).To(Equal("/path/repo"))
				return nil, fmt.Errorf("error here")
			}
			err := &errors.ReqError{Code: "server_err", HttpCode: 500, Msg: "error here", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Push(param, key.PrivKey().Base58())
			})

			m.GetLocalRepo = func(_, path string) (plumbing.LocalRepo, error) {
				Expect(path).To(Equal("/path/repo"))
				return nil, git.ErrRepositoryNotExists
			}
			err = &errors.ReqError{Code: "repo_not_found", HttpCode: 404, Msg: "repository does not exist", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Push(param, key.PrivKey().Base58())
			})
		})

		It("should panic if unable to get config", func() {
			key := ed25519.NewKeyFromIntSeed(1)
			param := map[string]interface{}{
				"id":        "repo_123",
				"reference": "refs/heads/master",
				"nonce":     "0",
			}
			mockTempRepoMgr := mocks.NewMockTempRepoManager(ctrl)
			mockRepoSrv.EXPECT().GetTempRepoManager().Return(mockTempRepoMgr)
			mockTempRepoMgr.EXPECT().GetPath(param["id"]).Return("/path/repo")

			var mockRepo = mocks.NewMockLocalRepo(ctrl)
			m.GetLocalRepo = func(_, path string) (plumbing.LocalRepo, error) {
				Expect(path).To(Equal("/path/repo"))
				return mockRepo, nil
			}

			// expected to be called if nonce is unset or zero
			mockAccountKeeper.EXPECT().Get(key.PubKey().Addr()).Return(state.NewBareAccount())

			// expect origin remote to be set with correct url
			mockRepo.EXPECT().GetName().Return("repo1").Times(2)
			mockRepo.EXPECT().Config().Return(nil, fmt.Errorf("error here"))

			err := &errors.ReqError{Code: "server_err", HttpCode: 500, Msg: "error here", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Push(param, key.PrivKey().Base58())
			})
		})

		It("should panic if unable to set config", func() {
			key := ed25519.NewKeyFromIntSeed(1)
			param := map[string]interface{}{
				"id":        "repo_123",
				"reference": "refs/heads/master",
				"nonce":     "0",
			}
			mockTempRepoMgr := mocks.NewMockTempRepoManager(ctrl)
			mockRepoSrv.EXPECT().GetTempRepoManager().Return(mockTempRepoMgr)
			mockTempRepoMgr.EXPECT().GetPath(param["id"]).Return("/path/repo")

			var mockRepo = mocks.NewMockLocalRepo(ctrl)
			m.GetLocalRepo = func(_, path string) (plumbing.LocalRepo, error) {
				Expect(path).To(Equal("/path/repo"))
				return mockRepo, nil
			}

			// expected to be called if nonce is unset or zero
			mockAccountKeeper.EXPECT().Get(key.PubKey().Addr()).Return(state.NewBareAccount())

			// expect origin remote to be set with correct url
			mockRepo.EXPECT().GetName().Return("repo1").Times(2)
			mockRepo.EXPECT().Config().Return(&config2.Config{Remotes: map[string]*config2.RemoteConfig{}}, nil)
			mockRepo.EXPECT().SetConfig(gomock.Any()).Return(fmt.Errorf("error here"))

			err := &errors.ReqError{Code: "server_err", HttpCode: 500, Msg: "error here", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Push(param, key.PrivKey().Base58())
			})
		})

		It("should attempt to push; "+
			"it should set correct nonce when one was not provided; "+
			"it should add correct remote to repo;"+
			"it should nil on successful push", func() {
			key := ed25519.NewKeyFromIntSeed(1)
			param := map[string]interface{}{
				"id":        "repo_123",
				"reference": "refs/heads/master",
				"nonce":     "0",
			}
			mockTempRepoMgr := mocks.NewMockTempRepoManager(ctrl)
			mockRepoSrv.EXPECT().GetTempRepoManager().Return(mockTempRepoMgr)
			mockTempRepoMgr.EXPECT().GetPath(param["id"]).Return("/path/repo")

			var mockRepo = mocks.NewMockLocalRepo(ctrl)
			m.GetLocalRepo = func(_, path string) (plumbing.LocalRepo, error) {
				Expect(path).To(Equal("/path/repo"))
				return mockRepo, nil
			}

			// expected to be called if nonce is unset or zero
			mockAccountKeeper.EXPECT().Get(key.PubKey().Addr()).Return(state.NewBareAccount())

			// expect origin remote to be set with correct url
			mockRepo.EXPECT().GetName().Return("repo1").Times(2)
			mockRepo.EXPECT().Config().Return(&config2.Config{Remotes: map[string]*config2.RemoteConfig{}}, nil)
			mockRepo.EXPECT().SetConfig(gomock.Any()).Do(func(cfg *config2.Config) {
				Expect(cfg.Remotes).To(HaveLen(1))
				Expect(cfg.Remotes).To(HaveKey("origin"))
				Expect(cfg.Remotes["origin"].URLs).To(HaveLen(1))
				Expect(cfg.Remotes["origin"].URLs[0]).To(Equal("http://127.0.0.1:9002/r/repo1"))
			})

			mockRepo.EXPECT().Push(gomock.Any()).DoAndReturn(func(opts plumbing.PushOptions) (bytes.Buffer, error) {
				Expect(opts.RemoteName).To(BeEmpty())
				Expect(opts.Token).ToNot(BeEmpty())
				Expect(opts.RefSpec).To(Equal(fmt.Sprintf("+%s:%s", param["reference"], param["reference"])))
				return *bytes.NewBuffer([]byte("hash: tx_hash_123")), nil
			})

			// it should remove repo from temp. repo manager cache
			mockTempRepoMgr.EXPECT().Remove(param["id"])

			assert.NotPanics(GinkgoT(), func() {
				txHash := m.Push(param, key.PrivKey().Base58())
				Expect(txHash).To(Equal("tx_hash_123"))
			})
		})

		It("should panic if push failed", func() {
			key := ed25519.NewKeyFromIntSeed(1)
			param := map[string]interface{}{
				"id":        "repo_123",
				"reference": "refs/heads/master",
				"nonce":     "0",
			}
			mockTempRepoMgr := mocks.NewMockTempRepoManager(ctrl)
			mockRepoSrv.EXPECT().GetTempRepoManager().Return(mockTempRepoMgr)
			mockTempRepoMgr.EXPECT().GetPath(param["id"]).Return("/path/repo")

			var mockRepo = mocks.NewMockLocalRepo(ctrl)
			m.GetLocalRepo = func(_, path string) (plumbing.LocalRepo, error) {
				Expect(path).To(Equal("/path/repo"))
				return mockRepo, nil
			}

			// expected to be called if nonce is unset or zero
			mockAccountKeeper.EXPECT().Get(key.PubKey().Addr()).Return(state.NewBareAccount())

			// expect origin remote to be set with correct url
			mockRepo.EXPECT().GetName().Return("repo1").Times(2)
			mockRepo.EXPECT().Config().Return(&config2.Config{Remotes: map[string]*config2.RemoteConfig{}}, nil)
			mockRepo.EXPECT().SetConfig(gomock.Any())

			mockRepo.EXPECT().Push(gomock.Any()).DoAndReturn(func(opts plumbing.PushOptions) (bytes.Buffer, error) {
				return bytes.Buffer{}, fmt.Errorf("error here")
			})

			err := &errors.ReqError{Code: "push_failure", HttpCode: 500, Msg: "error here", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Push(param, key.PrivKey().Base58())
			})
		})

		When("push token is provided as key", func() {
			It("should use push token directly", func() {
				key := ed25519.NewKeyFromIntSeed(1)
				token := pushtoken.MakeFromKey(key, &remotetypes.TxDetail{
					RepoName:  "repo1",
					PushKeyID: key.PushAddr().String(),
				})

				param := map[string]interface{}{
					"id":        "repo_123",
					"reference": "refs/heads/master",
					"nonce":     "0",
				}
				mockTempRepoMgr := mocks.NewMockTempRepoManager(ctrl)
				mockRepoSrv.EXPECT().GetTempRepoManager().Return(mockTempRepoMgr)
				mockTempRepoMgr.EXPECT().GetPath(param["id"]).Return("/path/repo")

				var mockRepo = mocks.NewMockLocalRepo(ctrl)
				m.GetLocalRepo = func(_, path string) (plumbing.LocalRepo, error) {
					Expect(path).To(Equal("/path/repo"))
					return mockRepo, nil
				}

				// expect origin remote to be set with correct url
				mockRepo.EXPECT().GetName().Return("repo1")
				mockRepo.EXPECT().Config().Return(&config2.Config{Remotes: map[string]*config2.RemoteConfig{}}, nil)
				mockRepo.EXPECT().SetConfig(gomock.Any()).Do(func(cfg *config2.Config) {
					Expect(cfg.Remotes).To(HaveLen(1))
					Expect(cfg.Remotes).To(HaveKey("origin"))
					Expect(cfg.Remotes["origin"].URLs).To(HaveLen(1))
					Expect(cfg.Remotes["origin"].URLs[0]).To(Equal("http://127.0.0.1:9002/r/repo1"))
				})

				mockRepo.EXPECT().Push(gomock.Any()).DoAndReturn(func(opts plumbing.PushOptions) (bytes.Buffer, error) {
					Expect(opts.RemoteName).To(BeEmpty())
					Expect(opts.Token).To(Equal(token))
					Expect(opts.RefSpec).To(Equal(fmt.Sprintf("+%s:%s", param["reference"], param["reference"])))
					return *bytes.NewBuffer([]byte("hash: tx_hash_123")), nil
				})

				// it should remove repo from temp. repo manager cache
				mockTempRepoMgr.EXPECT().Remove(param["id"])

				assert.NotPanics(GinkgoT(), func() {
					txHash := m.Push(param, token)
					Expect(txHash).To(Equal("tx_hash_123"))
				})
			})
		})
	})
})
