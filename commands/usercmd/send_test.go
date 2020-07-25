package contribcmd

import (
	"fmt"
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	restclient "gitlab.com/makeos/lobe/api/remote/client"
	"gitlab.com/makeos/lobe/api/rpc/client"
	"gitlab.com/makeos/lobe/api/types"
	"gitlab.com/makeos/lobe/commands/common"
	"gitlab.com/makeos/lobe/config"
	"gitlab.com/makeos/lobe/crypto"
	kstypes "gitlab.com/makeos/lobe/keystore/types"
	"gitlab.com/makeos/lobe/mocks"
	"gitlab.com/makeos/lobe/testutil"
)

var _ = Describe("UserCmd", func() {
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

	Describe(".SendCmd", func() {
		It("should return error when unable to unlock signing key", func() {
			args := &SendArgs{SigningKey: "sk", SigningKeyPass: "sk_pass"}
			args.KeyUnlocker = func(cfg *config.AppConfig, args2 *common.UnlockKeyArgs) (kstypes.StoredKey, error) {
				Expect(args2.KeyAddrOrIdx).To(Equal(args.SigningKey))
				Expect(args2.Passphrase).To(Equal(args.SigningKeyPass))
				return nil, fmt.Errorf("error")
			}
			err := SendCmd(cfg, args)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to unlock the signing key: error"))
		})

		It("should return error when unable to get signing key next nonce", func() {
			args := &SendArgs{SigningKey: "sk", SigningKeyPass: "sk_pass"}
			mockKey := mocks.NewMockStoredKey(ctrl)
			mockKey.EXPECT().GetAddress().Return(key.Addr().String())
			args.KeyUnlocker = func(cfg *config.AppConfig, args2 *common.UnlockKeyArgs) (kstypes.StoredKey, error) {
				return mockKey, nil
			}
			args.GetNextNonce = func(address string, rpcClient client.Client, remoteClients []restclient.Client) (string, error) {
				Expect(address).To(Equal(key.Addr().String()))
				return "", fmt.Errorf("error")
			}
			err := SendCmd(cfg, args)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to get signer's next nonce: error"))
		})

		It("should return error when unable send transaction", func() {
			args := &SendArgs{SigningKey: "sk", SigningKeyPass: "sk_pass"}
			mockKey := mocks.NewMockStoredKey(ctrl)
			mockKey.EXPECT().GetAddress().Return(key.Addr().String())
			mockKey.EXPECT().GetKey().Return(key)
			args.KeyUnlocker = func(cfg *config.AppConfig, args2 *common.UnlockKeyArgs) (kstypes.StoredKey, error) {
				return mockKey, nil
			}
			args.GetNextNonce = func(address string, rpcClient client.Client, remoteClients []restclient.Client) (string, error) {
				return "10", nil
			}
			args.SendCoin = func(req *types.SendCoinBody, rpcClient client.Client, remoteClients []restclient.Client) (hash string, err error) {
				return "", fmt.Errorf("error")
			}
			err := SendCmd(cfg, args)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to send coins: error"))
		})

		Describe("on success", func() {
			var err error
			args := &SendArgs{SigningKey: "sk", SigningKeyPass: "sk_pass"}
			BeforeEach(func() {
				mockKey := mocks.NewMockStoredKey(ctrl)
				mockKey.EXPECT().GetAddress().Return(key.Addr().String())
				mockKey.EXPECT().GetKey().Return(key)
				args.KeyUnlocker = func(cfg *config.AppConfig, args2 *common.UnlockKeyArgs) (kstypes.StoredKey, error) {
					return mockKey, nil
				}
				args.GetNextNonce = func(address string, rpcClient client.Client, remoteClients []restclient.Client) (string, error) {
					return "10", nil
				}
				args.SendCoin = func(req *types.SendCoinBody, rpcClient client.Client, remoteClients []restclient.Client) (hash string, err error) {
					return "0x123", nil
				}
				err = SendCmd(cfg, args)
			})

			It("should return nil error", func() {
				Expect(err).To(BeNil())
			})
		})
	})
})
