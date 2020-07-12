package pkcmd

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
	"gitlab.com/makeos/mosdef/commands/common"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/crypto"
	kstypes "gitlab.com/makeos/mosdef/keystore/types"
	"gitlab.com/makeos/mosdef/mocks"
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

	Describe(".RegisterCmd", func() {
		It("should return error when target key ID is a public key but signing key failed to be unlocked", func() {
			args := &RegisterArgs{Target: key.PubKey().Base58(), SigningKey: "maker1abc", SigningKeyPass: "abc"}
			args.KeyUnlocker = func(cfg *config.AppConfig, a *common.UnlockKeyArgs) (kstypes.StoredKey, error) {
				Expect(a.KeyAddrOrIdx).To(Equal(args.SigningKey))
				Expect(a.Passphrase).To(Equal(args.SigningKeyPass))
				Expect(a.TargetRepo).To(BeNil())
				return nil, fmt.Errorf("error")
			}
			err := RegisterCmd(cfg, args)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to unlock the signing key: error"))
		})

		It("should return error when target key ID is a local account but unable to unlock the local account", func() {
			args := &RegisterArgs{Target: "maker1", SigningKey: "maker1abc", SigningKeyPass: "abc"}
			args.KeyUnlocker = func(cfg *config.AppConfig, a *common.UnlockKeyArgs) (kstypes.StoredKey, error) {
				Expect(a.KeyAddrOrIdx).To(Equal(args.Target))
				Expect(a.Passphrase).To(Equal(args.TargetPass))
				Expect(a.TargetRepo).To(BeNil())
				return nil, fmt.Errorf("error")
			}
			err := RegisterCmd(cfg, args)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to unlock the local key: error"))
		})

		It("should return error when target key ID is a local account but unable to unlock the signing account", func() {
			args := &RegisterArgs{Target: "maker1", SigningKey: "maker1abc", SigningKeyPass: "abc"}
			mockLocalKey := mocks.NewMockStoredKey(ctrl)
			mockLocalKey.EXPECT().GetKey().Return(key)
			args.KeyUnlocker = func(cfg *config.AppConfig, a *common.UnlockKeyArgs) (kstypes.StoredKey, error) {
				Expect(a.KeyAddrOrIdx).To(Equal(args.Target))
				Expect(a.Passphrase).To(Equal(args.TargetPass))
				Expect(a.TargetRepo).To(BeNil())
				args.KeyUnlocker = func(cfg *config.AppConfig, a *common.UnlockKeyArgs) (kstypes.StoredKey, error) {
					return nil, fmt.Errorf("error")
				}
				return mockLocalKey, nil
			}
			err := RegisterCmd(cfg, args)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to unlock the signing key: error"))
		})

		It("should return error when unable get next nonce of signer", func() {
			args := &RegisterArgs{Target: key.PubKey().Base58(), SigningKey: "maker1abc", SigningKeyPass: "abc"}
			mockSigningKey := mocks.NewMockStoredKey(ctrl)
			mockSigningKey.EXPECT().GetAddress().Return("maker1abc")
			args.KeyUnlocker = func(cfg *config.AppConfig, a *common.UnlockKeyArgs) (kstypes.StoredKey, error) {
				return mockSigningKey, nil
			}
			args.GetNextNonce = func(address string, rpcClient client.Client, remoteClients []restclient.Client) (string, error) {
				Expect(address).To(Equal("maker1abc"))
				return "", fmt.Errorf("error")
			}
			err := RegisterCmd(cfg, args)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to get signer's next nonce: error"))
		})

		It("should return error when unable create registration transaction", func() {
			args := &RegisterArgs{Target: key.PubKey().Base58(), SigningKey: "maker1abc", SigningKeyPass: "abc"}
			mockSigningKey := mocks.NewMockStoredKey(ctrl)
			mockSigningKey.EXPECT().GetAddress().Return("maker1abc")
			mockSigningKey.EXPECT().GetKey().Return(key)
			args.KeyUnlocker = func(cfg *config.AppConfig, a *common.UnlockKeyArgs) (kstypes.StoredKey, error) {
				return mockSigningKey, nil
			}
			args.GetNextNonce = func(address string, rpcClient client.Client, remoteClients []restclient.Client) (string, error) {
				Expect(address).To(Equal("maker1abc"))
				return "10", nil
			}
			args.RegisterPushKey = func(req *types2.RegisterPushKeyBody, rpcClient client.Client, remoteClients []restclient.Client) (hash string, err error) {
				return "", fmt.Errorf("error")
			}
			err := RegisterCmd(cfg, args)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to register push key: error"))
		})

		It("should return no error on successful transaction creation", func() {
			args := &RegisterArgs{Target: key.PubKey().Base58(), SigningKey: "maker1abc", SigningKeyPass: "abc", Stdout: ioutil.Discard}
			mockSigningKey := mocks.NewMockStoredKey(ctrl)
			mockSigningKey.EXPECT().GetAddress().Return("maker1abc")
			mockSigningKey.EXPECT().GetKey().Return(key)
			args.KeyUnlocker = func(cfg *config.AppConfig, a *common.UnlockKeyArgs) (kstypes.StoredKey, error) {
				return mockSigningKey, nil
			}
			args.GetNextNonce = func(address string, rpcClient client.Client, remoteClients []restclient.Client) (string, error) {
				Expect(address).To(Equal("maker1abc"))
				return "10", nil
			}
			args.RegisterPushKey = func(req *types2.RegisterPushKeyBody, rpcClient client.Client, remoteClients []restclient.Client) (hash string, err error) {
				Expect(req.PublicKey).To(Equal(key.PubKey().ToPublicKey()))
				Expect(req.FeeCap).To(Equal(args.FeeCap))
				Expect(req.Fee).To(Equal(args.Fee))
				Expect(req.Scopes).To(Equal(args.Scopes))
				Expect(req.Nonce).To(Equal(uint64(10)))
				return "0x123", nil
			}
			err := RegisterCmd(cfg, args)
			Expect(err).To(BeNil())
		})
	})
})
