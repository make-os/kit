package remote

import (
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	"gitlab.com/makeos/mosdef/mocks"
	"gitlab.com/makeos/mosdef/modules/types"
	"gitlab.com/makeos/mosdef/pkgs/logger"
	"gitlab.com/makeos/mosdef/util"
)

var _ = Describe("PushKeyAPI", func() {
	var ctrl *gomock.Controller

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".CreateRepo", func() {
		modules := &types.Modules{}
		api := &API{modules: modules, log: logger.NewLogrusNoOp()}
		testPostRequestCases(map[string]TestCase{
			"should return error when params could not be json decoded": {
				paramsRaw:  []byte("{"),
				resp:       `{"error":{"code":"0","msg":"malformed body"}}`,
				statusCode: 400,
			},
		}, api.CreateRepo)
	})

	Describe(".GetRepo", func() {
		modules := &types.Modules{}
		api := &API{modules: modules, log: logger.NewLogrusNoOp()}
		testGetRequestCases(map[string]TestCase{
			"should return repository": {
				params:     map[string]string{"name": "repo1", "height": "1", "noProposals": "true"},
				resp:       `{"name":"repo1"}`,
				statusCode: 200,
				mocker: func(tc *TestCase) {
					mockRepoModule := mocks.NewMockRepoModule(ctrl)
					mockRepoModule.EXPECT().
						Get("repo1", types.GetOptions{Height: uint64(1), NoProposals: true}).
						Return(util.Map{"name": "repo1"})
					modules.Repo = mockRepoModule
				},
			},
		}, api.GetRepo)
	})
})
