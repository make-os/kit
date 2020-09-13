package txcmd_test

import (
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/make-os/lobe/cmd/txcmd"
	types2 "github.com/make-os/lobe/modules/types"
	"github.com/make-os/lobe/rpc/types"
	"github.com/make-os/lobe/types/api"
	"github.com/make-os/lobe/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
)

func TestTxCmd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RepoCmd Suite")
}

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
			args.GetTransaction = func(hash string, rpcClient types.Client) (res *api.ResultTx, err error) {
				Expect(hash).To(Equal(args.Hash))
				return nil, fmt.Errorf("error")
			}
			err := txcmd.GetCmd(args)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to get transaction: error"))
		})

		It("should return error='unknown transaction' when error is ReqError and has http status=404", func() {
			args := &txcmd.GetArgs{Hash: "0x123"}
			args.GetTransaction = func(hash string, rpcClient types.Client) (res *api.ResultTx, err error) {
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
			args.GetTransaction = func(hash string, rpcClient types.Client) (res *api.ResultTx, err error) {
				Expect(hash).To(Equal(args.Hash))
				return &api.ResultTx{
					Data:   map[string]interface{}{"value": "10.2"},
					Status: types2.TxStatusInBlock,
				}, nil
			}
			err := txcmd.GetCmd(args)
			Expect(err).To(BeNil())
		})
	})
})
