package txcmd_test

import (
	"fmt"
	"io/ioutil"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	restclient "github.com/themakeos/lobe/api/remote/client"
	"github.com/themakeos/lobe/api/rpc/client"
	"github.com/themakeos/lobe/api/types"
	"github.com/themakeos/lobe/cmd/txcmd"
	"github.com/themakeos/lobe/modules"
	"github.com/themakeos/lobe/util"
)

var _ = Describe("TxCmd", func() {
	var err error
	var ctrl *gomock.Controller

	BeforeEach(func() {
		Expect(err).To(BeNil())
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe(".CreateCmd", func() {
		It("should return error when unable to get transaction", func() {
			args := &txcmd.GetArgs{Hash: "0x123"}
			args.GetTransaction = func(hash string, rpcClient client.Client, remoteClients []restclient.Client) (res *types.GetTxResponse, err error) {
				Expect(hash).To(Equal(args.Hash))
				return nil, fmt.Errorf("error")
			}
			err := txcmd.GetCmd(args)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to get transaction: error"))
		})

		It("should return error='unknown transaction' when error is ReqError and has http status=404", func() {
			args := &txcmd.GetArgs{Hash: "0x123"}
			args.GetTransaction = func(hash string, rpcClient client.Client, remoteClients []restclient.Client) (res *types.GetTxResponse, err error) {
				Expect(hash).To(Equal(args.Hash))
				reqErr := util.ReqErr(404, "not_found", "", "not found")
				return nil, errors.Wrap(reqErr, "error")
			}
			err := txcmd.GetCmd(args)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("unknown transaction"))
		})

		It("should return no error on success", func() {
			args := &txcmd.GetArgs{Hash: "0x123", Stdout: ioutil.Discard}
			args.GetTransaction = func(hash string, rpcClient client.Client, remoteClients []restclient.Client) (res *types.GetTxResponse, err error) {
				Expect(hash).To(Equal(args.Hash))
				return &types.GetTxResponse{
					Data:   map[string]interface{}{"value": "10.2"},
					Status: modules.TxStatusInBlock,
				}, nil
			}
			err := txcmd.GetCmd(args)
			Expect(err).To(BeNil())
		})
	})
})
