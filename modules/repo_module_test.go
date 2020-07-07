package modules_test

import (
	"fmt"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/robertkrimen/otto"
	"github.com/stretchr/testify/assert"
	"gitlab.com/makeos/mosdef/mocks"
	"gitlab.com/makeos/mosdef/modules"
	"gitlab.com/makeos/mosdef/modules/types"
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/types/txns"
	"gitlab.com/makeos/mosdef/util"
	"gopkg.in/src-d/go-git.v4"
)

var _ = Describe("RepoModule", func() {
	var m *modules.RepoModule
	var ctrl *gomock.Controller
	var mockService *mocks.MockService
	var mockLogic *mocks.MockLogic
	var mockRepoSrv *mocks.MockRemoteServer
	var mockMempoolReactor *mocks.MockMempoolReactor
	var mockPruner *mocks.MockRepoPruner
	var mockRepoKeeper *mocks.MockRepoKeeper

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockService = mocks.NewMockService(ctrl)
		mockRepoSrv = mocks.NewMockRemoteServer(ctrl)
		mockMempoolReactor = mocks.NewMockMempoolReactor(ctrl)
		mockPruner = mocks.NewMockRepoPruner(ctrl)
		mockLogic = mocks.NewMockLogic(ctrl)
		mockRepoKeeper = mocks.NewMockRepoKeeper(ctrl)
		mockLogic.EXPECT().GetMempoolReactor().Return(mockMempoolReactor).AnyTimes()
		mockLogic.EXPECT().RepoKeeper().Return(mockRepoKeeper).AnyTimes()
		mockLogic.EXPECT().GetRemoteServer().Return(mockRepoSrv).AnyTimes()
		mockRepoSrv.EXPECT().GetPruner().Return(mockPruner).AnyTimes()
		m = modules.NewRepoModule(mockService, mockRepoSrv, mockLogic)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".ConsoleOnlyMode", func() {
		It("should return false", func() {
			Expect(m.ConsoleOnlyMode()).To(BeFalse())
		})
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
			params := map[string]interface{}{"name": 123}
			err := &util.StatusError{Code: "invalid_param", HttpCode: 400, Msg: "field:name, msg:invalid value type: has int, wants string", Field: "params"}
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
			Expect(res["type"]).To(Equal(int(txns.TxTypeRepoCreate)))
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

		It("should return panic if unable to add tx to mempool", func() {
			params := map[string]interface{}{"name": "repo1"}
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(nil, fmt.Errorf("error"))
			err := &util.StatusError{Code: "mempool_add_err", HttpCode: 400, Msg: "error", Field: ""}
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
			params := map[string]interface{}{"addresses": 123}
			err := &util.StatusError{Code: "invalid_param", HttpCode: 400, Msg: "field:addresses, msg:invalid value type: has int, wants string|[]string", Field: "params"}
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
			Expect(res["addresses"]).To(Equal([]string{"addr1"}))
			Expect(res["veto"]).To(BeFalse())
			Expect(res).ToNot(HaveKey("hash"))
			Expect(res["type"]).To(Equal(int(txns.TxTypeRepoProposalUpsertOwner)))
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

		It("should return panic if unable to add tx to mempool", func() {
			params := map[string]interface{}{"addresses": []string{"addr1"}}
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(nil, fmt.Errorf("error"))
			err := &util.StatusError{Code: "mempool_add_err", HttpCode: 400, Msg: "error", Field: ""}
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

	Describe(".VoteOnProposal", func() {
		It("should panic when unable to decode params", func() {
			params := map[string]interface{}{"name": 123}
			err := &util.StatusError{Code: "invalid_param", HttpCode: 400, Msg: "field:name, msg:invalid value type: has int, wants string", Field: "params"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.VoteOnProposal(params)
			})
		})

		It("should return tx map equivalent if payloadOnly=true", func() {
			key := ""
			payloadOnly := true
			params := map[string]interface{}{"name": "repo1"}
			res := m.VoteOnProposal(params, key, payloadOnly)
			Expect(res["name"]).To(Equal("repo1"))
			Expect(res).ToNot(HaveKey("hash"))
			Expect(res["type"]).To(Equal(int(txns.TxTypeRepoProposalVote)))
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

		It("should return panic if unable to add tx to mempool", func() {
			params := map[string]interface{}{"name": "repo1"}
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(nil, fmt.Errorf("error"))
			err := &util.StatusError{Code: "mempool_add_err", HttpCode: 400, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.VoteOnProposal(params, "", false)
			})
		})

		It("should return tx hash on success", func() {
			params := map[string]interface{}{"name": "repo1"}
			hash := util.StrToHexBytes("tx_hash")
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(hash, nil)
			res := m.VoteOnProposal(params, "", false)
			Expect(res).To(HaveKey("hash"))
			Expect(res["hash"]).To(Equal(hash))
		})
	})

	Describe(".Prune", func() {
		It("should panic when forced prune failed", func() {
			mockPruner.EXPECT().Prune("repo1", true).Return(fmt.Errorf("error"))
			err := &util.StatusError{Code: "server_err", HttpCode: 500, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Prune("repo1", true)
			})
		})

		It("should schedule prune operation when force=false", func() {
			mockPruner.EXPECT().Schedule("repo1")
			m.Prune("repo1", false)
		})
	})

	Describe(".Get", func() {
		It("should panic when height option field is not valid", func() {
			err := &util.StatusError{Code: "invalid_param", HttpCode: 400, Msg: "unexpected type", Field: "opts.height"}
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
			err := &util.StatusError{Code: "repo_not_found", HttpCode: 404, Msg: "repo not found", Field: "name"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Get("repo1")
			})
		})
	})

	Describe(".Update", func() {
		It("should panic when unable to decode params", func() {
			params := map[string]interface{}{"config": 123}
			err := &util.StatusError{Code: "invalid_param", HttpCode: 400, Msg: "field:config, msg:invalid value type: has int, wants map", Field: "params"}
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
			Expect(res["type"]).To(Equal(int(txns.TxTypeRepoProposalUpdate)))
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

		It("should return panic if unable to add tx to mempool", func() {
			params := map[string]interface{}{"id": 1}
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(nil, fmt.Errorf("error"))
			err := &util.StatusError{Code: "mempool_add_err", HttpCode: 400, Msg: "error", Field: ""}
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

	Describe(".DepositFee", func() {
		It("should panic when unable to decode params", func() {
			params := map[string]interface{}{"id": struct{}{}}
			err := &util.StatusError{Code: "invalid_param", HttpCode: 400, Msg: "field:id, msg:invalid value type: has struct {}, wants string|int", Field: "params"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.DepositFee(params)
			})
		})

		It("should return tx map equivalent if payloadOnly=true", func() {
			key := ""
			payloadOnly := true
			params := map[string]interface{}{"id": 1}
			res := m.DepositFee(params, key, payloadOnly)
			Expect(res["id"]).To(Equal("1"))
			Expect(res).ToNot(HaveKey("hash"))
			Expect(res["type"]).To(Equal(int(txns.TxTypeRepoProposalSendFee)))
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

		It("should return panic if unable to add tx to mempool", func() {
			params := map[string]interface{}{"id": 1}
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(nil, fmt.Errorf("error"))
			err := &util.StatusError{Code: "mempool_add_err", HttpCode: 400, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.DepositFee(params, "", false)
			})
		})

		It("should return tx hash on success", func() {
			params := map[string]interface{}{"id": 1}
			hash := util.StrToHexBytes("tx_hash")
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(hash, nil)
			res := m.DepositFee(params, "", false)
			Expect(res).To(HaveKey("hash"))
			Expect(res["hash"]).To(Equal(hash))
		})
	})

	Describe(".Register", func() {
		It("should panic when unable to decode params", func() {
			params := map[string]interface{}{"id": struct{}{}}
			err := &util.StatusError{Code: "invalid_param", HttpCode: 400, Msg: "field:id, msg:invalid value type: has struct {}, wants string|int", Field: "params"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Register(params)
			})
		})

		It("should return tx map equivalent if payloadOnly=true", func() {
			key := ""
			payloadOnly := true
			params := map[string]interface{}{"id": 1}
			res := m.Register(params, key, payloadOnly)
			Expect(res["id"]).To(Equal("1"))
			Expect(res).ToNot(HaveKey("hash"))
			Expect(res["type"]).To(Equal(int(txns.TxTypeRepoProposalRegisterPushKey)))
			Expect(res).To(And(
				HaveKey("timestamp"),
				HaveKey("nonce"),
				HaveKey("policies"),
				HaveKey("namespace"),
				HaveKey("namespaceOnly"),
				HaveKey("ids"),
				HaveKey("id"),
				HaveKey("type"),
				HaveKey("senderPubKey"),
				HaveKey("fee"),
				HaveKey("sig"),
			))
		})

		It("should return panic if unable to add tx to mempool", func() {
			params := map[string]interface{}{"id": 1}
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(nil, fmt.Errorf("error"))
			err := &util.StatusError{Code: "mempool_add_err", HttpCode: 400, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.Register(params, "", false)
			})
		})

		It("should return tx hash on success", func() {
			params := map[string]interface{}{"id": 1}
			hash := util.StrToHexBytes("tx_hash")
			mockMempoolReactor.EXPECT().AddTx(gomock.Any()).Return(hash, nil)
			res := m.Register(params, "", false)
			Expect(res).To(HaveKey("hash"))
			Expect(res["hash"]).To(Equal(hash))
		})
	})

	Describe(".AnnounceObjects", func() {
		It("should panic if target repository does not exist locally", func() {
			mockRepoSrv.EXPECT().AnnounceRepoObjects("repo1").Return(git.ErrRepositoryNotExists)
			err := &util.StatusError{Code: "repo_not_found", HttpCode: 404, Msg: "repository does not exist", Field: "repoName"}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.AnnounceObjects("repo1")
			})
		})

		It("should panic if unable to announce repo object due to unknown error", func() {
			mockRepoSrv.EXPECT().AnnounceRepoObjects("repo1").Return(fmt.Errorf("error"))
			err := &util.StatusError{Code: "server_err", HttpCode: 500, Msg: "error", Field: ""}
			assert.PanicsWithError(GinkgoT(), err.Error(), func() {
				m.AnnounceObjects("repo1")
			})
		})
	})
})
