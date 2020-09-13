package utils

import (
	"fmt"

	"github.com/golang/mock/gomock"
	remote "github.com/make-os/lobe/api/remote/client"
	"github.com/make-os/lobe/api/rpc/client"
	"github.com/make-os/lobe/api/types"
	"github.com/make-os/lobe/crypto"
	"github.com/make-os/lobe/mocks"
	remoteclientmocks "github.com/make-os/lobe/mocks/remote-client"
	mocks2 "github.com/make-os/lobe/mocks/rpc"
	rpcclientmocks "github.com/make-os/lobe/mocks/rpc-client"
	"github.com/make-os/lobe/modules"
	"github.com/make-os/lobe/types/state"
	"github.com/make-os/lobe/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ClientUtils", func() {
	var ctrl *gomock.Controller
	var key = crypto.NewKeyFromIntSeed(1)
	var remClient1 *mocks.MockClient
	var remClient2 *mocks.MockClient
	var rpcClient *mocks2.MockClient
	var rpcUserClient *rpcclientmocks.MockUser
	var remoteUserClient *remoteclientmocks.MockUser
	var rpcPkClient *rpcclientmocks.MockPushKey
	var remotePkClient *remoteclientmocks.MockPushKey
	var rpcRepoClient *rpcclientmocks.MockRepo
	var remoteRepoClient *remoteclientmocks.MockRepo
	var rpcTxClient *rpcclientmocks.MockTx
	var remoteTxClient *remoteclientmocks.MockTx

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		remClient1 = mocks.NewMockClient(ctrl)
		remClient2 = mocks.NewMockClient(ctrl)
		rpcClient = mocks2.NewMockClient(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".GetNextNonceOfPushKeyOwner", func() {
		BeforeEach(func() {
			rpcPkClient = rpcclientmocks.NewMockPushKey(ctrl)
			rpcClient.EXPECT().PushKey().Return(rpcPkClient).AnyTimes()
			remotePkClient = remoteclientmocks.NewMockPushKey(ctrl)
			remClient2.EXPECT().PushKey().Return(remotePkClient).AnyTimes()
			remClient1.EXPECT().PushKey().Return(remotePkClient).AnyTimes()
		})

		It("should return error when no rpc client and remote API clients were provided", func() {
			_, err := GetNextNonceOfPushKeyOwner("pk-id", nil, nil)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("no remote client or rpc client provided"))
		})

		It("should return err when only one remote API client was provided and it failed", func() {
			remotePkClient.EXPECT().GetOwnerNonce("pk-id").Return(nil, fmt.Errorf("error"))
			_, err := GetNextNonceOfPushKeyOwner("pk-id", nil, []remote.Client{remClient2})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("Remote API: error"))
		})

		It("should return err when two remote API clients were provided and both failed", func() {
			remotePkClient.EXPECT().GetOwnerNonce("pk-id").Return(nil, fmt.Errorf("error")).Times(2)
			_, err := GetNextNonceOfPushKeyOwner("pk-id", nil, []remote.Client{remClient2, remClient1})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("Remote API: error"))
		})

		It("should return no err when two remote API clients were provided and one succeeded", func() {
			remotePkClient.EXPECT().GetOwnerNonce("pk-id").Return(nil, fmt.Errorf("error"))
			remotePkClient.EXPECT().GetOwnerNonce("pk-id").Return(&types.ResultAccountNonce{Nonce: "10"}, nil)
			nextNonce, err := GetNextNonceOfPushKeyOwner("pk-id", nil, []remote.Client{remClient2, remClient1})
			Expect(err).To(BeNil())
			Expect(nextNonce).To(Equal("11"))
		})

		It("should return err when only one remote API and one rpc client were provided and both failed", func() {
			rpcPkClient.EXPECT().GetOwner("pk-id").Return(nil, fmt.Errorf("error"))
			remotePkClient.EXPECT().GetOwnerNonce("pk-id").Return(nil, fmt.Errorf("error"))
			_, err := GetNextNonceOfPushKeyOwner("pk-id", rpcClient, []remote.Client{remClient1})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("Remote API: error"))
		})

		It("should return err when only one rpc client was provided and it failed", func() {
			rpcClientErr := util.ReqErr(400, "100", "field", "error")
			rpcPkClient.EXPECT().GetOwner("pk-id").Return(nil, rpcClientErr)
			_, err := GetNextNonceOfPushKeyOwner("pk-id", rpcClient, []remote.Client{})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("RPC API: field:'field', msg:'error', httpCode:'400', code:'100'"))
		})

		It("should return no err when only a rpc client was provided and it succeeded", func() {
			rpcPkClient.EXPECT().GetOwner("pk-id").Return(&types.ResultAccount{Account: &state.Account{Nonce: 10}}, nil)
			nextNonce, err := GetNextNonceOfPushKeyOwner("pk-id", rpcClient, []remote.Client{})
			Expect(err).To(BeNil())
			Expect(nextNonce).To(Equal("11"))
		})
	})

	Describe(".GetNextNonceOfAccount", func() {
		BeforeEach(func() {
			rpcUserClient = rpcclientmocks.NewMockUser(ctrl)
			rpcClient.EXPECT().User().Return(rpcUserClient).AnyTimes()
			remoteUserClient = remoteclientmocks.NewMockUser(ctrl)
			remClient1.EXPECT().User().Return(remoteUserClient).AnyTimes()
			remClient2.EXPECT().User().Return(remoteUserClient).AnyTimes()
		})

		It("should return err when two remote API clients were provided and both failed", func() {
			remoteUserClient.EXPECT().Get("address1").Return(nil, fmt.Errorf("error")).Times(2)
			_, err := GetNextNonceOfAccount("address1", nil, []remote.Client{remClient2, remClient1})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("Remote API: error"))
		})

		It("should return err when only one remote API and one rpc client were provided and both failed", func() {
			rpcUserClient.EXPECT().Get("address1").Return(nil, fmt.Errorf("error"))
			remoteUserClient.EXPECT().Get("address1").Return(nil, fmt.Errorf("error"))
			_, err := GetNextNonceOfAccount("address1", rpcClient, []remote.Client{remClient1})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("Remote API: error"))
		})

		It("should return err when only one rpc client was provided and it failed", func() {
			rpcClientErr := util.ReqErr(400, "100", "field", "error")
			rpcUserClient.EXPECT().Get("address1").Return(nil, rpcClientErr)
			_, err := GetNextNonceOfAccount("address1", rpcClient, []remote.Client{})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("RPC API: field:'field', msg:'error', httpCode:'400', code:'100'"))
		})

		It("should return no err when only a remote API client was provided and it succeeded", func() {
			remoteUserClient.EXPECT().Get("address1").Return(&types.ResultAccount{Account: &state.Account{Nonce: 10}}, nil)
			nextNonce, err := GetNextNonceOfAccount("address1", nil, []remote.Client{remClient1})
			Expect(err).To(BeNil())
			Expect(nextNonce).To(Equal("11"))
		})

		It("should return no err when only a rpc client was provided and it succeeded", func() {
			rpcUserClient.EXPECT().Get("address1").Return(&types.ResultAccount{Account: &state.Account{Nonce: 10}}, nil)
			nextNonce, err := GetNextNonceOfAccount("address1", rpcClient, []remote.Client{})
			Expect(err).To(BeNil())
			Expect(nextNonce).To(Equal("11"))
		})
	})

	Describe(".CreateRepo", func() {
		BeforeEach(func() {
			rpcRepoClient = rpcclientmocks.NewMockRepo(ctrl)
			rpcClient.EXPECT().Repo().Return(rpcRepoClient).AnyTimes()
			remoteRepoClient = remoteclientmocks.NewMockRepo(ctrl)
			remClient1.EXPECT().Repo().Return(remoteRepoClient).AnyTimes()
			remClient2.EXPECT().Repo().Return(remoteRepoClient).AnyTimes()
		})

		It("should return err when two remote API clients were provided and both failed", func() {
			args := &types.BodyCreateRepo{Name: "repo1"}
			remoteRepoClient.EXPECT().Create(args).Return(nil, fmt.Errorf("error")).Times(2)
			_, err := CreateRepo(args, nil, []remote.Client{remClient1, remClient2})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("Remote API: error"))
		})

		It("should return err when only one remote API and one rpc client were provided and both failed", func() {
			args := &types.BodyCreateRepo{Name: "repo1"}
			rpcRepoClient.EXPECT().Create(args).Return(nil, fmt.Errorf("error"))
			remoteRepoClient.EXPECT().Create(args).Return(nil, fmt.Errorf("error"))
			_, err := CreateRepo(args, rpcClient, []remote.Client{remClient1})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("Remote API: error"))
		})

		It("should return no err when only a remote API client was provided and it succeeded", func() {
			args := &types.BodyCreateRepo{Name: "repo1"}
			remoteRepoClient.EXPECT().Create(args).Return(&types.ResultCreateRepo{Hash: "0x123"}, nil)
			hash, err := CreateRepo(args, nil, []remote.Client{remClient1})
			Expect(err).To(BeNil())
			Expect(hash).To(Equal("0x123"))
		})

		It("should return no err when only a rpc client was provided and it succeeded", func() {
			args := &types.BodyCreateRepo{Name: "repo1"}
			rpcRepoClient.EXPECT().Create(args).Return(&types.ResultCreateRepo{Hash: "0x123"}, nil)
			hash, err := CreateRepo(args, rpcClient, []remote.Client{})
			Expect(err).To(BeNil())
			Expect(hash).To(Equal("0x123"))
		})
	})

	Describe(".VoteRepoProposal", func() {
		BeforeEach(func() {
			rpcRepoClient = rpcclientmocks.NewMockRepo(ctrl)
			rpcClient.EXPECT().Repo().Return(rpcRepoClient).AnyTimes()
			remoteRepoClient = remoteclientmocks.NewMockRepo(ctrl)
			remClient1.EXPECT().Repo().Return(remoteRepoClient).AnyTimes()
			remClient2.EXPECT().Repo().Return(remoteRepoClient).AnyTimes()
		})

		It("should return err when two remote API clients were provided and both failed", func() {
			args := &types.BodyRepoVote{RepoName: "repo1"}
			remoteRepoClient.EXPECT().VoteProposal(args).Return(nil, fmt.Errorf("error")).Times(2)
			_, err := VoteRepoProposal(args, nil, []remote.Client{remClient1, remClient2})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("Remote API: error"))
		})

		It("should return err when only one remote API and one rpc client were provided and both failed", func() {
			args := &types.BodyRepoVote{RepoName: "repo1"}
			rpcRepoClient.EXPECT().VoteProposal(args).Return(nil, fmt.Errorf("error"))
			remoteRepoClient.EXPECT().VoteProposal(args).Return(nil, fmt.Errorf("error"))
			_, err := VoteRepoProposal(args, rpcClient, []remote.Client{remClient1})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("Remote API: error"))
		})

		It("should return no err when only a remote API client was provided and it succeeded", func() {
			args := &types.BodyRepoVote{RepoName: "repo1"}
			remoteRepoClient.EXPECT().VoteProposal(args).Return(&types.ResultHash{Hash: "0x123"}, nil)
			hash, err := VoteRepoProposal(args, nil, []remote.Client{remClient1})
			Expect(err).To(BeNil())
			Expect(hash).To(Equal("0x123"))
		})

		It("should return no err when only a rpc client was provided and it succeeded", func() {
			args := &types.BodyRepoVote{RepoName: "repo1"}
			rpcRepoClient.EXPECT().VoteProposal(args).Return(&types.ResultHash{Hash: "0x123"}, nil)
			hash, err := VoteRepoProposal(args, rpcClient, []remote.Client{})
			Expect(err).To(BeNil())
			Expect(hash).To(Equal("0x123"))
		})
	})

	Describe(".RegisterPushKey", func() {
		BeforeEach(func() {
			rpcPkClient = rpcclientmocks.NewMockPushKey(ctrl)
			rpcClient.EXPECT().PushKey().Return(rpcPkClient).AnyTimes()
			remotePkClient = remoteclientmocks.NewMockPushKey(ctrl)
			remClient1.EXPECT().PushKey().Return(remotePkClient).AnyTimes()
			remClient2.EXPECT().PushKey().Return(remotePkClient).AnyTimes()
		})

		It("should return err when two remote API clients were provided and both failed", func() {
			args := &types.BodyRegisterPushKey{PublicKey: key.PubKey().ToPublicKey()}
			remotePkClient.EXPECT().Register(args).Return(nil, fmt.Errorf("error")).Times(2)
			_, err := RegisterPushKey(args, nil, []remote.Client{remClient2, remClient1})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("Remote API: error"))
		})

		It("should return err when only one remote API and one rpc client were provided and both failed", func() {
			args := &types.BodyRegisterPushKey{PublicKey: key.PubKey().ToPublicKey()}
			rpcPkClient.EXPECT().Register(args).Return(nil, fmt.Errorf("error"))
			remotePkClient.EXPECT().Register(args).Return(nil, fmt.Errorf("error"))
			_, err := RegisterPushKey(args, rpcClient, []remote.Client{remClient1})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("Remote API: error"))
		})

		It("should return no err when only an remote API client was provided and it succeeded", func() {
			args := &types.BodyRegisterPushKey{PublicKey: key.PubKey().ToPublicKey()}
			remotePkClient.EXPECT().Register(args).Return(&types.ResultRegisterPushKey{Hash: "0x123"}, nil)
			hash, err := RegisterPushKey(args, nil, []remote.Client{remClient1})
			Expect(err).To(BeNil())
			Expect(hash).To(Equal("0x123"))
		})

		It("should return no err when only a rpc client was provided and it succeeded", func() {
			args := &types.BodyRegisterPushKey{PublicKey: key.PubKey().ToPublicKey()}
			rpcPkClient.EXPECT().Register(args).Return(&types.ResultRegisterPushKey{Hash: "0x123"}, nil)
			hash, err := RegisterPushKey(args, rpcClient, []remote.Client{})
			Expect(err).To(BeNil())
			Expect(hash).To(Equal("0x123"))
		})
	})

	Describe(".AddRepoContributors", func() {
		BeforeEach(func() {
			rpcRepoClient = rpcclientmocks.NewMockRepo(ctrl)
			rpcClient.EXPECT().Repo().Return(rpcRepoClient).AnyTimes()
			remoteRepoClient = remoteclientmocks.NewMockRepo(ctrl)
			remClient1.EXPECT().Repo().Return(remoteRepoClient).AnyTimes()
			remClient2.EXPECT().Repo().Return(remoteRepoClient).AnyTimes()
		})

		It("should return err when two remote API clients were provided and both failed", func() {
			args := &types.BodyAddRepoContribs{RepoName: "repo1"}
			remoteRepoClient.EXPECT().AddContributors(args).Return(nil, fmt.Errorf("error")).Times(2)
			_, err := AddRepoContributors(args, nil, []remote.Client{remClient2, remClient1})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("Remote API: error"))
		})

		It("should return err when only one remote API and one rpc client were provided and both failed", func() {
			args := &types.BodyAddRepoContribs{RepoName: "repo1"}
			rpcRepoClient.EXPECT().AddContributors(args).Return(nil, fmt.Errorf("error"))
			remoteRepoClient.EXPECT().AddContributors(args).Return(nil, fmt.Errorf("error"))
			_, err := AddRepoContributors(args, rpcClient, []remote.Client{remClient1})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("Remote API: error"))
		})

		It("should return no err when only an remote API client was provided and it succeeded", func() {
			args := &types.BodyAddRepoContribs{RepoName: "repo1"}
			remoteRepoClient.EXPECT().AddContributors(args).Return(&types.ResultHash{Hash: "0x123"}, nil)
			hash, err := AddRepoContributors(args, nil, []remote.Client{remClient1})
			Expect(err).To(BeNil())
			Expect(hash).To(Equal("0x123"))
		})

		It("should return no err when only a rpc client was provided and it succeeded", func() {
			args := &types.BodyAddRepoContribs{RepoName: "repo1"}
			rpcRepoClient.EXPECT().AddContributors(args).Return(&types.ResultHash{Hash: "0x123"}, nil)
			hash, err := AddRepoContributors(args, rpcClient, []remote.Client{})
			Expect(err).To(BeNil())
			Expect(hash).To(Equal("0x123"))
		})
	})

	Describe(".SendCoin", func() {
		BeforeEach(func() {
			rpcUserClient = rpcclientmocks.NewMockUser(ctrl)
			rpcClient.EXPECT().User().Return(rpcUserClient).AnyTimes()
			remoteUserClient = remoteclientmocks.NewMockUser(ctrl)
			remClient1.EXPECT().User().Return(remoteUserClient).AnyTimes()
			remClient2.EXPECT().User().Return(remoteUserClient).AnyTimes()
		})

		It("should return err when two remote API clients were provided and both failed", func() {
			args := &types.BodySendCoin{Value: 10.20}
			remoteUserClient.EXPECT().Send(args).Return(nil, fmt.Errorf("error")).Times(2)
			_, err := SendCoin(args, nil, []remote.Client{remClient2, remClient1})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("Remote API: error"))
		})

		It("should return err when only one remote API and one rpc client were provided and both failed", func() {
			args := &types.BodySendCoin{Value: 10.20}
			rpcUserClient.EXPECT().Send(args).Return(nil, fmt.Errorf("error"))
			remoteUserClient.EXPECT().Send(args).Return(nil, fmt.Errorf("error"))
			_, err := SendCoin(args, rpcClient, []remote.Client{remClient1})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("Remote API: error"))
		})

		It("should return no err when only a remote client was provided and it succeeded", func() {
			args := &types.BodySendCoin{Value: 10.20}
			remoteUserClient.EXPECT().Send(args).Return(&types.ResultHash{Hash: "0x123"}, nil)
			hash, err := SendCoin(args, nil, []remote.Client{remClient1})
			Expect(err).To(BeNil())
			Expect(hash).To(Equal("0x123"))
		})

		It("should return no err when only a rpc client was provided and it succeeded", func() {
			args := &types.BodySendCoin{Value: 10.20}
			rpcUserClient.EXPECT().Send(args).Return(&types.ResultHash{Hash: "0x123"}, nil)
			hash, err := SendCoin(args, rpcClient, []remote.Client{})
			Expect(err).To(BeNil())
			Expect(hash).To(Equal("0x123"))
		})
	})

	Describe(".GetTransaction", func() {
		BeforeEach(func() {
			rpcTxClient = rpcclientmocks.NewMockTx(ctrl)
			rpcClient.EXPECT().Tx().Return(rpcTxClient).AnyTimes()
			remoteTxClient = remoteclientmocks.NewMockTx(ctrl)
			remClient1.EXPECT().Tx().Return(remoteTxClient).AnyTimes()
			remClient2.EXPECT().Tx().Return(remoteTxClient).AnyTimes()
		})

		It("should return err when two remote API clients were provided and both failed", func() {
			hash := "0x123"
			remoteTxClient.EXPECT().Get(hash).Return(nil, fmt.Errorf("error")).Times(2)
			_, err := GetTransaction(hash, nil, []remote.Client{remClient2, remClient1})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("Remote API: error"))
		})

		It("should return err when only one remote API and one rpc client were provided and both failed", func() {
			hash := "0x123"
			rpcTxClient.EXPECT().Get(hash).Return(nil, fmt.Errorf("error"))
			remoteTxClient.EXPECT().Get(hash).Return(nil, fmt.Errorf("error"))
			_, err := GetTransaction(hash, rpcClient, []remote.Client{remClient1})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("Remote API: error"))
		})

		It("should return no err when only a remote API client was provided and it succeeded", func() {
			hash := "0x123"
			expectedRes := &types.ResultTx{
				Data:   map[string]interface{}{"type": "1", "value": "10.2"},
				Status: modules.TxStatusInBlock,
			}
			remoteTxClient.EXPECT().Get(hash).Return(expectedRes, nil)
			res, err := GetTransaction(hash, nil, []remote.Client{remClient1})
			Expect(err).To(BeNil())
			Expect(res).To(Equal(expectedRes))
		})

		It("should return no err when only a rpc API client was provided and it succeeded", func() {
			hash := "0x123"
			expectedRes := &types.ResultTx{
				Data:   map[string]interface{}{"type": "1", "value": "10.2"},
				Status: modules.TxStatusInBlock,
			}
			rpcTxClient.EXPECT().Get(hash).Return(expectedRes, nil)
			res, err := GetTransaction(hash, rpcClient, []remote.Client{})
			Expect(err).To(BeNil())
			Expect(res).To(Equal(expectedRes))
		})
	})

	Describe(".CallClients", func() {
		It("should return error when no caller callbacks were provided", func() {
			err := CallClients(&client.RPCClient{}, []remote.Client{&remote.RemoteClient{}}, nil, nil)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("no client caller provided"))
		})
	})
})
