package utils

import (
	"fmt"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rest "gitlab.com/makeos/mosdef/api/remote/client"
	"gitlab.com/makeos/mosdef/api/rpc/client"
	"gitlab.com/makeos/mosdef/api/types"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/mocks"
	mocks2 "gitlab.com/makeos/mosdef/mocks/rpc"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/util"
)

var _ = Describe("ClientUtils", func() {
	var ctrl *gomock.Controller
	var key = crypto.NewKeyFromIntSeed(1)
	var client1 *mocks.MockClient
	var client2 *mocks.MockClient
	var remoteClient *mocks.MockClient
	var rpcClient *mocks2.MockClient

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		client1 = mocks.NewMockClient(ctrl)
		client2 = mocks.NewMockClient(ctrl)
		remoteClient = mocks.NewMockClient(ctrl)
		rpcClient = mocks2.NewMockClient(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".GetNextNonceOfPushKeyOwner", func() {
		It("should return error when no rpc client and remote clients were provided", func() {
			_, err := GetNextNonceOfPushKeyOwner("pk-id", nil, nil)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("no remote client or rpc client provided"))
		})

		It("should return err when only one remote client is provided and it failed", func() {
			client1.EXPECT().GetPushKeyOwnerNonce("pk-id").Return(nil, fmt.Errorf("error"))
			_, err := GetNextNonceOfPushKeyOwner("pk-id", nil, []rest.Client{client1})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("Remote API: error"))
		})

		It("should return err when two remote clients are provided and both failed", func() {
			client1.EXPECT().GetPushKeyOwnerNonce("pk-id").Return(nil, fmt.Errorf("error"))
			client2.EXPECT().GetPushKeyOwnerNonce("pk-id").Return(nil, fmt.Errorf("error"))
			_, err := GetNextNonceOfPushKeyOwner("pk-id", nil, []rest.Client{client1, client2})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("Remote API: error"))
		})

		It("should return no err and response when two remote clients are provided and one succeeds", func() {
			client1.EXPECT().GetPushKeyOwnerNonce("pk-id").Return(nil, fmt.Errorf("error"))
			client2.EXPECT().GetPushKeyOwnerNonce("pk-id").Return(&types.GetAccountNonceResponse{Nonce: "10"}, nil)
			nextNonce, err := GetNextNonceOfPushKeyOwner("pk-id", nil, []rest.Client{client1, client2})
			Expect(err).To(BeNil())
			Expect(nextNonce).To(Equal("11"))
		})

		It("should return err when only one remote client and one rpc client are provided and both failed", func() {
			remoteClient.EXPECT().GetPushKeyOwnerNonce("pk-id").Return(nil, fmt.Errorf("error"))
			rpcClientErr := util.ReqErr(400, "100", "field", "error")
			rpcClient.EXPECT().GetPushKeyOwner("pk-id").Return(nil, rpcClientErr)
			_, err := GetNextNonceOfPushKeyOwner("pk-id", rpcClient, []rest.Client{remoteClient})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("Remote API: error"))
		})

		It("should return err when only one rpc client is provided and it failed", func() {
			rpcClientErr := util.ReqErr(400, "100", "field", "error")
			rpcClient.EXPECT().GetPushKeyOwner("pk-id").Return(nil, rpcClientErr)
			_, err := GetNextNonceOfPushKeyOwner("pk-id", rpcClient, []rest.Client{})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("RPC API: field:'field', msg:'error', httpCode:'400', code:'100'"))
		})

		It("should return no err and response when only an rpc client is provided and it succeeded", func() {
			rpcClient.EXPECT().GetPushKeyOwner("pk-id").Return(&types.GetAccountResponse{Account: &state.Account{Nonce: 10}}, nil)
			nextNonce, err := GetNextNonceOfPushKeyOwner("pk-id", rpcClient, []rest.Client{})
			Expect(err).To(BeNil())
			Expect(nextNonce).To(Equal("11"))
		})
	})

	Describe(".GetNextNonceOfAccount", func() {
		It("should return err when two remote clients are provided and both failed", func() {
			client1.EXPECT().GetAccount("address1").Return(nil, fmt.Errorf("error"))
			client2.EXPECT().GetAccount("address1").Return(nil, fmt.Errorf("error"))
			_, err := GetNextNonceOfAccount("address1", nil, []rest.Client{client1, client2})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("Remote API: error"))
		})

		It("should return err when only one remote client and one rpc client are provided and both failed", func() {
			remoteClient.EXPECT().GetAccount("address1").Return(nil, fmt.Errorf("error"))
			rpcClientErr := util.ReqErr(400, "100", "field", "error")
			rpcClient.EXPECT().GetAccount("address1").Return(nil, rpcClientErr)
			_, err := GetNextNonceOfAccount("address1", rpcClient, []rest.Client{remoteClient})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("Remote API: error"))
		})

		It("should return err when only one rpc client is provided and it failed", func() {
			rpcClientErr := util.ReqErr(400, "100", "field", "error")
			rpcClient.EXPECT().GetAccount("address1").Return(nil, rpcClientErr)
			_, err := GetNextNonceOfAccount("address1", rpcClient, []rest.Client{})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("RPC API: field:'field', msg:'error', httpCode:'400', code:'100'"))
		})

		It("should return no err and response when only an rpc client is provided and it succeeded", func() {
			remoteClient.EXPECT().GetAccount("address1").Return(&types.GetAccountResponse{Account: &state.Account{Nonce: 10}}, nil)
			nextNonce, err := GetNextNonceOfAccount("address1", nil, []rest.Client{remoteClient})
			Expect(err).To(BeNil())
			Expect(nextNonce).To(Equal("11"))
		})

		It("should return no err and response when only a remote client is provided and it succeeded", func() {
			rpcClient.EXPECT().GetAccount("address1").Return(&types.GetAccountResponse{Account: &state.Account{Nonce: 10}}, nil)
			nextNonce, err := GetNextNonceOfAccount("address1", rpcClient, []rest.Client{})
			Expect(err).To(BeNil())
			Expect(nextNonce).To(Equal("11"))
		})
	})

	Describe(".CreateRepo", func() {
		It("should return err when two remote clients are provided and both failed", func() {

			args := &types.CreateRepoBody{Name: "repo1"}
			client1.EXPECT().CreateRepo(args).Return(nil, fmt.Errorf("error"))
			client2.EXPECT().CreateRepo(args).Return(nil, fmt.Errorf("error"))
			_, err := CreateRepo(args, nil, []rest.Client{client1, client2})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("Remote API: error"))
		})

		It("should return err when only one remote client and one rpc client are provided and both failed", func() {
			args := &types.CreateRepoBody{Name: "repo1"}
			remoteClient.EXPECT().CreateRepo(args).Return(nil, fmt.Errorf("error"))
			rpcClientErr := util.ReqErr(400, "100", "field", "error")
			rpcClient.EXPECT().CreateRepo(args).Return(nil, rpcClientErr)
			_, err := CreateRepo(args, rpcClient, []rest.Client{remoteClient})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("Remote API: error"))
		})

		It("should return no err and response when only an rpc client is provided and it succeeded", func() {
			args := &types.CreateRepoBody{Name: "repo1"}
			remoteClient.EXPECT().CreateRepo(args).Return(&types.CreateRepoResponse{Hash: "0x123"}, nil)
			hash, err := CreateRepo(args, nil, []rest.Client{remoteClient})
			Expect(err).To(BeNil())
			Expect(hash).To(Equal("0x123"))
		})

		It("should return no err and response when only a remote client is provided and it succeeded", func() {
			args := &types.CreateRepoBody{Name: "repo1"}
			rpcClient.EXPECT().CreateRepo(args).Return(&types.CreateRepoResponse{Hash: "0x123"}, nil)
			hash, err := CreateRepo(args, rpcClient, []rest.Client{})
			Expect(err).To(BeNil())
			Expect(hash).To(Equal("0x123"))
		})
	})

	Describe(".RegisterPushKey()", func() {
		It("should return err when two remote clients are provided and both failed", func() {
			args := &types.RegisterPushKeyBody{PublicKey: key.PubKey().ToPublicKey()}
			client1.EXPECT().RegisterPushKey(args).Return(nil, fmt.Errorf("error"))
			client2.EXPECT().RegisterPushKey(args).Return(nil, fmt.Errorf("error"))
			_, err := RegisterPushKey(args, nil, []rest.Client{client1, client2})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("Remote API: error"))
		})

		It("should return err when only one remote client and one rpc client are provided and both failed", func() {
			args := &types.RegisterPushKeyBody{PublicKey: key.PubKey().ToPublicKey()}
			remoteClient.EXPECT().RegisterPushKey(args).Return(nil, fmt.Errorf("error"))
			rpcClientErr := util.ReqErr(400, "100", "field", "error")
			rpcClient.EXPECT().RegisterPushKey(args).Return(nil, rpcClientErr)
			_, err := RegisterPushKey(args, rpcClient, []rest.Client{remoteClient})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("Remote API: error"))
		})

		It("should return no err and response when only an rpc client is provided and it succeeded", func() {
			args := &types.RegisterPushKeyBody{PublicKey: key.PubKey().ToPublicKey()}
			remoteClient.EXPECT().RegisterPushKey(args).Return(&types.RegisterPushKeyResponse{Hash: "0x123"}, nil)
			hash, err := RegisterPushKey(args, nil, []rest.Client{remoteClient})
			Expect(err).To(BeNil())
			Expect(hash).To(Equal("0x123"))
		})

		It("should return no err and response when only a remote client is provided and it succeeded", func() {
			args := &types.RegisterPushKeyBody{PublicKey: key.PubKey().ToPublicKey()}
			rpcClient.EXPECT().RegisterPushKey(args).Return(&types.RegisterPushKeyResponse{Hash: "0x123"}, nil)
			hash, err := RegisterPushKey(args, rpcClient, []rest.Client{})
			Expect(err).To(BeNil())
			Expect(hash).To(Equal("0x123"))
		})
	})

	Describe(".AddRepoContributors", func() {
		It("should return err when two remote clients are provided and both failed", func() {
			args := &types.AddRepoContribsBody{RepoName: "repo1"}
			client1.EXPECT().AddRepoContributors(args).Return(nil, fmt.Errorf("error"))
			client2.EXPECT().AddRepoContributors(args).Return(nil, fmt.Errorf("error"))
			_, err := AddRepoContributors(args, nil, []rest.Client{client1, client2})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("Remote API: error"))
		})

		It("should return err when only one remote client and one rpc client are provided and both failed", func() {
			args := &types.AddRepoContribsBody{RepoName: "repo1"}
			remoteClient.EXPECT().AddRepoContributors(args).Return(nil, fmt.Errorf("error"))
			rpcClientErr := util.ReqErr(400, "100", "field", "error")
			rpcClient.EXPECT().AddRepoContributors(args).Return(nil, rpcClientErr)
			_, err := AddRepoContributors(args, rpcClient, []rest.Client{remoteClient})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("Remote API: error"))
		})

		It("should return no err and response when only an rpc client is provided and it succeeded", func() {
			args := &types.AddRepoContribsBody{RepoName: "repo1"}
			remoteClient.EXPECT().AddRepoContributors(args).Return(&types.HashResponse{Hash: "0x123"}, nil)
			hash, err := AddRepoContributors(args, nil, []rest.Client{remoteClient})
			Expect(err).To(BeNil())
			Expect(hash).To(Equal("0x123"))
		})

		It("should return no err and response when only a remote client is provided and it succeeded", func() {
			args := &types.AddRepoContribsBody{RepoName: "repo1"}
			rpcClient.EXPECT().AddRepoContributors(args).Return(&types.HashResponse{Hash: "0x123"}, nil)
			hash, err := AddRepoContributors(args, rpcClient, []rest.Client{})
			Expect(err).To(BeNil())
			Expect(hash).To(Equal("0x123"))
		})
	})

	Describe(".SendCoin", func() {
		It("should return err when two remote clients are provided and both failed", func() {
			args := &types.SendCoinBody{Value: 10.20}
			client1.EXPECT().SendCoin(args).Return(nil, fmt.Errorf("error"))
			client2.EXPECT().SendCoin(args).Return(nil, fmt.Errorf("error"))
			_, err := SendCoin(args, nil, []rest.Client{client1, client2})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("Remote API: error"))
		})

		It("should return err when only one remote client and one rpc client are provided and both failed", func() {
			args := &types.SendCoinBody{Value: 10.20}
			remoteClient.EXPECT().SendCoin(args).Return(nil, fmt.Errorf("error"))
			rpcClientErr := util.ReqErr(400, "100", "field", "error")
			rpcClient.EXPECT().SendCoin(args).Return(nil, rpcClientErr)
			_, err := SendCoin(args, rpcClient, []rest.Client{remoteClient})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("Remote API: error"))
		})

		It("should return no err and response when only an rpc client is provided and it succeeded", func() {
			args := &types.SendCoinBody{Value: 10.20}
			remoteClient.EXPECT().SendCoin(args).Return(&types.HashResponse{Hash: "0x123"}, nil)
			hash, err := SendCoin(args, nil, []rest.Client{remoteClient})
			Expect(err).To(BeNil())
			Expect(hash).To(Equal("0x123"))
		})

		It("should return no err and response when only a remote client is provided and it succeeded", func() {
			args := &types.SendCoinBody{Value: 10.20}
			rpcClient.EXPECT().SendCoin(args).Return(&types.HashResponse{Hash: "0x123"}, nil)
			hash, err := SendCoin(args, rpcClient, []rest.Client{})
			Expect(err).To(BeNil())
			Expect(hash).To(Equal("0x123"))
		})
	})

	Describe(".CallClients", func() {
		It("should return error when no caller callbacks were provided", func() {
			err := CallClients(&client.RPCClient{}, []rest.Client{&rest.ClientV1{}}, nil, nil)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("no client caller provided"))
		})
	})
})
