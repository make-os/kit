package api

import (
	"fmt"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rest "gitlab.com/makeos/mosdef/api/rest/client"
	"gitlab.com/makeos/mosdef/api/rpc/client"
	"gitlab.com/makeos/mosdef/api/types"
	"gitlab.com/makeos/mosdef/mocks"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/util"
)

var _ = Describe("", func() {
	var ctrl *gomock.Controller

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
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
			client := mocks.NewMockClient(ctrl)
			client.EXPECT().GetPushKeyOwnerNonce("pk-id").Return(nil, fmt.Errorf("error"))
			_, err := GetNextNonceOfPushKeyOwner("pk-id", nil, []rest.Client{client})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("Remote API: error"))
		})

		It("should return err when two remote clients are provided and both failed", func() {
			client := mocks.NewMockClient(ctrl)
			client2 := mocks.NewMockClient(ctrl)
			client.EXPECT().GetPushKeyOwnerNonce("pk-id").Return(nil, fmt.Errorf("error"))
			client2.EXPECT().GetPushKeyOwnerNonce("pk-id").Return(nil, fmt.Errorf("error"))
			_, err := GetNextNonceOfPushKeyOwner("pk-id", nil, []rest.Client{client, client2})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("Remote API: error"))
		})

		It("should return no err and response when two remote clients are provided and one succeeds", func() {
			client := mocks.NewMockClient(ctrl)
			client2 := mocks.NewMockClient(ctrl)
			client.EXPECT().GetPushKeyOwnerNonce("pk-id").Return(nil, fmt.Errorf("error"))
			client2.EXPECT().GetPushKeyOwnerNonce("pk-id").Return(&types.GetAccountNonceResponse{Nonce: "10"}, nil)
			nextNonce, err := GetNextNonceOfPushKeyOwner("pk-id", nil, []rest.Client{client, client2})
			Expect(err).To(BeNil())
			Expect(nextNonce).To(Equal("11"))
		})

		It("should return err when only one remote client and one rpc client are provided and both failed", func() {
			remoteClient := mocks.NewMockClient(ctrl)
			remoteClient.EXPECT().GetPushKeyOwnerNonce("pk-id").Return(nil, fmt.Errorf("error"))
			rpcClient := client.NewMockClient(ctrl)
			rpcClientErr := util.NewStatusError(400, "100", "field", "error")
			rpcClient.EXPECT().GetPushKeyOwnerAccount("pk-id").Return(nil, rpcClientErr)
			_, err := GetNextNonceOfPushKeyOwner("pk-id", rpcClient, []rest.Client{remoteClient})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("Remote API: error"))
		})

		It("should return err when only one rpc client is provided and it failed", func() {
			rpcClient := client.NewMockClient(ctrl)
			rpcClientErr := util.NewStatusError(400, "100", "field", "error")
			rpcClient.EXPECT().GetPushKeyOwnerAccount("pk-id").Return(nil, rpcClientErr)
			_, err := GetNextNonceOfPushKeyOwner("pk-id", rpcClient, []rest.Client{})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("RPC API: field:'field', msg:'error', httpCode:'400', code:'100'"))
		})

		It("should return no err and response when only an rpc client is provided and it succeeds", func() {
			rpcClient := client.NewMockClient(ctrl)
			rpcClient.EXPECT().GetPushKeyOwnerAccount("pk-id").Return(&state.Account{Nonce: 10}, nil)
			nextNonce, err := GetNextNonceOfPushKeyOwner("pk-id", rpcClient, []rest.Client{})
			Expect(err).To(BeNil())
			Expect(nextNonce).To(Equal("11"))
		})
	})

	Describe(".ClientCaller", func() {
		It("should return error when no caller callbacks were provided", func() {
			err := ClientCaller(&client.RPCClient{}, []rest.Client{&rest.ClientV1{}}, nil, nil)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("no client caller provided"))
		})
	})
})
