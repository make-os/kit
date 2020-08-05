package rpc

import (
	"github.com/golang/mock/gomock"
	"github.com/themakeos/lobe/mocks"
	"github.com/themakeos/lobe/modules/types"
	"github.com/themakeos/lobe/rpc"
	"github.com/themakeos/lobe/util"

	. "github.com/onsi/ginkgo"
)

var _ = Describe("Repo", func() {
	var ctrl *gomock.Controller

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".createRepo", func() {
		mods := &types.Modules{}
		api := &RepoAPI{mods}
		testCases(map[string]*TestCase{
			"should return error when params is not a map": {
				params:     "{}",
				statusCode: 400,
				err:        &rpc.Err{Code: "60000", Message: "param must be a map", Data: ""},
			},
			"should return code=200 on success": {
				params:     map[string]interface{}{"name": "repo1"},
				result:     util.Map{"name": "repo1", "hash": "0x123"},
				statusCode: 200,
				mocker: func(tc *TestCase) {
					mockRepoModule := mocks.NewMockRepoModule(ctrl)
					mockRepoModule.EXPECT().Create(tc.params).Return(util.Map{"name": "repo1", "hash": "0x123"})
					mods.Repo = mockRepoModule
				},
			},
		}, api.createRepo)
	})

	Describe(".getRepo", func() {
		mods := &types.Modules{}
		api := &RepoAPI{mods}
		testCases(map[string]*TestCase{
			"should return error when params is not a map": {
				params:     "{}",
				statusCode: 400,
				err:        &rpc.Err{Code: "60000", Message: "param must be a map", Data: ""},
			},
			"should return repo object on success": {
				params:     map[string]interface{}{"name": "repo1", "height": 100, "noProposals": true},
				statusCode: 200,
				result:     util.Map{"name": "repo1", "balance": "210.1"},
				mocker: func(tc *TestCase) {
					mockRepoModule := mocks.NewMockRepoModule(ctrl)
					mockRepoModule.EXPECT().Get("repo1", types.GetOptions{Height: uint64(100), NoProposals: true}).
						Return(util.Map{"name": "repo1", "balance": "210.1"})
					mods.Repo = mockRepoModule
				},
			},
		}, api.getRepo)
	})

	Describe(".addContributors", func() {
		mods := &types.Modules{}
		api := &RepoAPI{mods}
		testCases(map[string]*TestCase{
			"should return error when params is not a map": {
				params:     "{}",
				statusCode: 400,
				err:        &rpc.Err{Code: "60000", Message: "param must be a map", Data: ""},
			},
			"should return code=200 on success": {
				params:     map[string]interface{}{"keys": []string{"pk1k75ztyqr2dq7pc3nlpdfzj2ry58sfzm7l803nz"}},
				result:     util.Map{"hash": "0x123"},
				statusCode: 200,
				mocker: func(tc *TestCase) {
					mockRepoModule := mocks.NewMockRepoModule(ctrl)
					mockRepoModule.EXPECT().AddContributor(tc.params).Return(util.Map{"hash": "0x123"})
					mods.Repo = mockRepoModule
				},
			},
		}, api.addContributors)
	})

	Describe(".vote", func() {
		mods := &types.Modules{}
		api := &RepoAPI{mods}
		testCases(map[string]*TestCase{
			"should return error when params is not a map": {
				params:     "{}",
				statusCode: 400,
				err:        &rpc.Err{Code: "60000", Message: "param must be a map", Data: ""},
			},
			"should return code=200 on success": {
				params:     map[string]interface{}{"keys": []string{"pk1k75ztyqr2dq7pc3nlpdfzj2ry58sfzm7l803nz"}},
				result:     util.Map{"hash": "0x123"},
				statusCode: 200,
				mocker: func(tc *TestCase) {
					mockRepoModule := mocks.NewMockRepoModule(ctrl)
					mockRepoModule.EXPECT().Vote(tc.params).Return(util.Map{"hash": "0x123"})
					mods.Repo = mockRepoModule
				},
			},
		}, api.vote)
	})
})
