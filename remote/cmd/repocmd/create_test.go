package repocmd

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	restclient "gitlab.com/makeos/mosdef/api/remote/client"
	"gitlab.com/makeos/mosdef/api/rpc/client"
	types2 "gitlab.com/makeos/mosdef/api/types"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/crypto"
	kstypes "gitlab.com/makeos/mosdef/keystore/types"
	"gitlab.com/makeos/mosdef/mocks"
	"gitlab.com/makeos/mosdef/remote/types"
	"gitlab.com/makeos/mosdef/testutil"
)

var _ = Describe("SignCommit", func() {
	var err error
	var cfg *config.AppConfig
	var ctrl *gomock.Controller
	var key = crypto.NewKeyFromIntSeed(1)

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".CreateCmd", func() {
		It("should return error when config is a path to an unknown file", func() {
			args := &CreateArgs{Config: "path/to/unknown/file"}
			err := CreateCmd(cfg, args)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to read config file"))
		})

		It("should return error when config file contains malformed json", func() {
			f, err := ioutil.TempFile("", "")
			Expect(err).To(BeNil())
			f.WriteString("{ malformed }")
			f.Close()
			args := &CreateArgs{Config: f.Name()}
			err = CreateCmd(cfg, args)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed parse configuration"))
		})

		It("should return error when failed to unlock account", func() {
			args := &CreateArgs{Account: "1", AccountPass: "pass"}
			args.KeyUnlocker = func(cfg *config.AppConfig, keyAddrOrIdx, defaultPassphrase string, targetRepo types.LocalRepo) (kstypes.StoredKey, error) {
				Expect(keyAddrOrIdx).To(Equal(args.Account))
				Expect(defaultPassphrase).To(Equal(args.AccountPass))
				return nil, fmt.Errorf("error")
			}
			err := CreateCmd(cfg, args)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to unlock the signing key: error"))
		})

		It("should return error when nonce is 0 and it failed to fetch next nonce", func() {
			args := &CreateArgs{Account: "1", AccountPass: "pass"}
			mockKey := mocks.NewMockStoredKey(ctrl)
			mockKey.EXPECT().GetAddress().Return(key.Addr().String())
			args.KeyUnlocker = func(cfg *config.AppConfig, keyAddrOrIdx, defaultPassphrase string, targetRepo types.LocalRepo) (kstypes.StoredKey, error) {
				return mockKey, nil
			}
			args.GetNextNonce = func(address string, rpcClient client.Client, remoteClients []restclient.Client) (string, error) {
				Expect(address).To(Equal(key.Addr().String()))
				return "", fmt.Errorf("error")
			}
			err := CreateCmd(cfg, args)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to get signer's next nonce: error"))
		})

		It("should return error when to create repo", func() {
			args := &CreateArgs{Name: "repo1", Value: "12.2", Fee: "1.2", Account: "1", AccountPass: "pass", Config: `{"governance": {"propFee": "100"}}`}
			mockKey := mocks.NewMockStoredKey(ctrl)
			mockKey.EXPECT().GetAddress().Return(key.Addr().String())
			mockKey.EXPECT().GetKey().Return(key)
			args.KeyUnlocker = func(cfg *config.AppConfig, keyAddrOrIdx, defaultPassphrase string, targetRepo types.LocalRepo) (kstypes.StoredKey, error) {
				return mockKey, nil
			}
			args.GetNextNonce = func(address string, rpcClient client.Client, remoteClients []restclient.Client) (string, error) {
				Expect(address).To(Equal(key.Addr().String()))
				return "2", nil
			}
			args.CreateRepo = func(req *types2.CreateRepoBody, rpcClient client.Client, remoteClients []restclient.Client) (hash string, err error) {
				Expect(req.Name).To(Equal(args.Name))
				Expect(req.Config.Governance.ProposalFee).To(Equal(float64(100)))
				Expect(req.Value).To(Equal("12.2"))
				Expect(req.Nonce).To(Equal(uint64(2)))
				Expect(req.Fee).To(Equal("1.2"))
				return "", fmt.Errorf("error")
			}
			err := CreateCmd(cfg, args)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to create repo: error"))
		})

		It("should return nil on success", func() {
			args := &CreateArgs{Name: "repo1", Value: "12.2", Fee: "1.2", Account: "1", AccountPass: "pass", Config: `{"governance": {"propFee": "100"}}`}
			mockKey := mocks.NewMockStoredKey(ctrl)
			mockKey.EXPECT().GetAddress().Return(key.Addr().String())
			mockKey.EXPECT().GetKey().Return(key)
			args.KeyUnlocker = func(cfg *config.AppConfig, keyAddrOrIdx, defaultPassphrase string, targetRepo types.LocalRepo) (kstypes.StoredKey, error) {
				return mockKey, nil
			}
			args.GetNextNonce = func(address string, rpcClient client.Client, remoteClients []restclient.Client) (string, error) {
				Expect(address).To(Equal(key.Addr().String()))
				return "2", nil
			}
			args.CreateRepo = func(req *types2.CreateRepoBody, rpcClient client.Client, remoteClients []restclient.Client) (hash string, err error) {
				Expect(req.Name).To(Equal(args.Name))
				Expect(req.Config.Governance.ProposalFee).To(Equal(float64(100)))
				Expect(req.Value).To(Equal("12.2"))
				Expect(req.Nonce).To(Equal(uint64(2)))
				Expect(req.Fee).To(Equal("1.2"))
				return "0x123", nil
			}
			err := CreateCmd(cfg, args)
			Expect(err).To(BeNil())
		})
	})
})
