package txcmd

import (
	"fmt"
	"io/ioutil"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	restclient "gitlab.com/makeos/mosdef/api/remote/client"
	"gitlab.com/makeos/mosdef/api/rpc/client"
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
			args := &GetArgs{Hash: "0x123"}
			args.GetTransaction = func(hash string, rpcClient client.Client, remoteClients []restclient.Client) (res map[string]interface{}, err error) {
				Expect(hash).To(Equal(args.Hash))
				return nil, fmt.Errorf("error")
			}
			err := GetCmd(args)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to get transaction: error"))
		})

		It("should return no error on success", func() {
			args := &GetArgs{Hash: "0x123", Stdout: ioutil.Discard}
			args.GetTransaction = func(hash string, rpcClient client.Client, remoteClients []restclient.Client) (res map[string]interface{}, err error) {
				Expect(hash).To(Equal(args.Hash))
				return map[string]interface{}{"value": "10.2"}, nil
			}
			err := GetCmd(args)
			Expect(err).To(BeNil())
		})
	})
})
