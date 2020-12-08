package api

import (
	"fmt"

	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/crypto/ed25519"
	mocks2 "github.com/make-os/kit/mocks/rpc"
	"github.com/make-os/kit/modules/types"
	"github.com/make-os/kit/types/api"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ClientUtils", func() {
	var ctrl *gomock.Controller
	var key = ed25519.NewKeyFromIntSeed(1)
	var rpcClient *mocks2.MockClient
	var rpcUserClient *mocks2.MockUser
	var rpcPkClient *mocks2.MockPushKey
	var rpcRepoClient *mocks2.MockRepo
	var rpcTxClient *mocks2.MockTx

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		rpcClient = mocks2.NewMockClient(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".GetNextNonceOfPushKeyOwner", func() {
		BeforeEach(func() {
			rpcPkClient = mocks2.NewMockPushKey(ctrl)
			rpcClient.EXPECT().PushKey().Return(rpcPkClient).AnyTimes()
		})

		It("should return err when rpc client failed", func() {
			rpcClientErr := util.ReqErr(400, "100", "field", "error")
			rpcPkClient.EXPECT().GetOwner("pk-id").Return(nil, rpcClientErr)
			_, err := GetNextNonceOfPushKeyOwner("pk-id", rpcClient)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:'field', msg:'error', httpCode:'400', code:'100'"))
		})

		It("should return no err when rpc client succeeded", func() {
			rpcPkClient.EXPECT().GetOwner("pk-id").Return(&api.ResultAccount{Account: &state.Account{Nonce: 10}}, nil)
			nextNonce, err := GetNextNonceOfPushKeyOwner("pk-id", rpcClient)
			Expect(err).To(BeNil())
			Expect(nextNonce).To(Equal("11"))
		})
	})

	Describe(".GetNextNonceOfAccount", func() {
		BeforeEach(func() {
			rpcUserClient = mocks2.NewMockUser(ctrl)
			rpcClient.EXPECT().User().Return(rpcUserClient).AnyTimes()
		})

		It("should return err when rpc client failed", func() {
			rpcClientErr := util.ReqErr(400, "100", "field", "error")
			rpcUserClient.EXPECT().Get("address1").Return(nil, rpcClientErr)
			_, err := GetNextNonceOfAccount("address1", rpcClient)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:'field', msg:'error', httpCode:'400', code:'100'"))
		})

		It("should return no err when rpc client succeeded", func() {
			rpcUserClient.EXPECT().Get("address1").Return(&api.ResultAccount{Account: &state.Account{Nonce: 10}}, nil)
			nextNonce, err := GetNextNonceOfAccount("address1", rpcClient)
			Expect(err).To(BeNil())
			Expect(nextNonce).To(Equal("11"))
		})
	})

	Describe(".CreateRepo", func() {
		BeforeEach(func() {
			rpcRepoClient = mocks2.NewMockRepo(ctrl)
			rpcClient.EXPECT().Repo().Return(rpcRepoClient).AnyTimes()
		})

		It("should return err when rpc client failed", func() {
			args := &api.BodyCreateRepo{Name: "repo1"}
			rpcRepoClient.EXPECT().Create(args).Return(nil, fmt.Errorf("error"))
			_, err := CreateRepo(args, rpcClient)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		It("should return no err when rpc client succeeded", func() {
			args := &api.BodyCreateRepo{Name: "repo1"}
			rpcRepoClient.EXPECT().Create(args).Return(&api.ResultCreateRepo{Hash: "0x123"}, nil)
			hash, err := CreateRepo(args, rpcClient)
			Expect(err).To(BeNil())
			Expect(hash).To(Equal("0x123"))
		})
	})

	Describe(".VoteRepoProposal", func() {
		BeforeEach(func() {
			rpcRepoClient = mocks2.NewMockRepo(ctrl)
			rpcClient.EXPECT().Repo().Return(rpcRepoClient).AnyTimes()
		})

		It("should return err when rpc client failed", func() {
			args := &api.BodyRepoVote{RepoName: "repo1"}
			rpcRepoClient.EXPECT().VoteProposal(args).Return(nil, fmt.Errorf("error"))
			_, err := VoteRepoProposal(args, rpcClient)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		It("should return no err when rpc client succeeded", func() {
			args := &api.BodyRepoVote{RepoName: "repo1"}
			rpcRepoClient.EXPECT().VoteProposal(args).Return(&api.ResultHash{Hash: "0x123"}, nil)
			hash, err := VoteRepoProposal(args, rpcClient)
			Expect(err).To(BeNil())
			Expect(hash).To(Equal("0x123"))
		})
	})

	Describe(".RegisterPushKey", func() {
		BeforeEach(func() {
			rpcPkClient = mocks2.NewMockPushKey(ctrl)
			rpcClient.EXPECT().PushKey().Return(rpcPkClient).AnyTimes()
		})

		It("should return err when rpc client failed", func() {
			args := &api.BodyRegisterPushKey{PublicKey: key.PubKey().ToPublicKey()}
			rpcPkClient.EXPECT().Register(args).Return(nil, fmt.Errorf("error"))
			_, err := RegisterPushKey(args, rpcClient)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		It("should return no err when rpc client succeeded", func() {
			args := &api.BodyRegisterPushKey{PublicKey: key.PubKey().ToPublicKey()}
			rpcPkClient.EXPECT().Register(args).Return(&api.ResultRegisterPushKey{Hash: "0x123"}, nil)
			hash, err := RegisterPushKey(args, rpcClient)
			Expect(err).To(BeNil())
			Expect(hash).To(Equal("0x123"))
		})
	})

	Describe(".AddRepoContributors", func() {
		BeforeEach(func() {
			rpcRepoClient = mocks2.NewMockRepo(ctrl)
			rpcClient.EXPECT().Repo().Return(rpcRepoClient).AnyTimes()
		})

		It("should return err when rpc client failed", func() {
			args := &api.BodyAddRepoContribs{RepoName: "repo1"}
			rpcRepoClient.EXPECT().AddContributors(args).Return(nil, fmt.Errorf("error"))
			_, err := AddRepoContributors(args, rpcClient)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		It("should return no err when rpc client succeeded", func() {
			args := &api.BodyAddRepoContribs{RepoName: "repo1"}
			rpcRepoClient.EXPECT().AddContributors(args).Return(&api.ResultHash{Hash: "0x123"}, nil)
			hash, err := AddRepoContributors(args, rpcClient)
			Expect(err).To(BeNil())
			Expect(hash).To(Equal("0x123"))
		})
	})

	Describe(".SendCoin", func() {
		BeforeEach(func() {
			rpcUserClient = mocks2.NewMockUser(ctrl)
			rpcClient.EXPECT().User().Return(rpcUserClient).AnyTimes()
		})

		It("should return err when rpc client failed", func() {
			args := &api.BodySendCoin{Value: 10.20}
			rpcUserClient.EXPECT().Send(args).Return(nil, fmt.Errorf("error"))
			_, err := SendCoin(args, rpcClient)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		It("should return no err when rpc client succeeded", func() {
			args := &api.BodySendCoin{Value: 10.20}
			rpcUserClient.EXPECT().Send(args).Return(&api.ResultHash{Hash: "0x123"}, nil)
			hash, err := SendCoin(args, rpcClient)
			Expect(err).To(BeNil())
			Expect(hash).To(Equal("0x123"))
		})
	})

	Describe(".GetTransaction", func() {
		BeforeEach(func() {
			rpcTxClient = mocks2.NewMockTx(ctrl)
			rpcClient.EXPECT().Tx().Return(rpcTxClient).AnyTimes()
		})

		It("should return err when rpc client failed", func() {
			hash := "0x123"
			rpcTxClient.EXPECT().Get(hash).Return(nil, fmt.Errorf("error"))
			_, err := GetTransaction(hash, rpcClient)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		It("should return no err when only a rpc API client was provided and it succeeded", func() {
			hash := "0x123"
			expectedRes := &api.ResultTx{
				Data:   map[string]interface{}{"type": "1", "value": "10.2"},
				Status: types.TxStatusInBlock,
			}
			rpcTxClient.EXPECT().Get(hash).Return(expectedRes, nil)
			res, err := GetTransaction(hash, rpcClient)
			Expect(err).To(BeNil())
			Expect(res).To(Equal(expectedRes))
		})
	})
})
