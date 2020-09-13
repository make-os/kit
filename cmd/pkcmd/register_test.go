package pkcmd

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/golang/mock/gomock"
	restclient "github.com/make-os/lobe/api/remote/client"
	"github.com/make-os/lobe/api/rpc/client"
	types2 "github.com/make-os/lobe/api/types"
	"github.com/make-os/lobe/cmd/common"
	"github.com/make-os/lobe/config"
	"github.com/make-os/lobe/crypto"
	kstypes "github.com/make-os/lobe/keystore/types"
	"github.com/make-os/lobe/mocks"
	"github.com/make-os/lobe/testutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
			args := &RegisterArgs{Target: key.PubKey().Base58(), SigningKey: "os1abc", SigningKeyPass: "abc"}
			args.KeyUnlocker = func(cfg *config.AppConfig, a *common.UnlockKeyArgs) (kstypes.StoredKey, error) {
				Expect(a.KeyStoreID).To(Equal(args.SigningKey))
				Expect(a.Passphrase).To(Equal(args.SigningKeyPass))
				Expect(a.TargetRepo).To(BeNil())
				return nil, fmt.Errorf("error")
			}
			err := RegisterCmd(cfg, args)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to unlock the signing key: error"))
		})

		It("should return error when target key ID is a local account but unable to unlock the local account", func() {
			args := &RegisterArgs{Target: "os1", SigningKey: "os1abc", SigningKeyPass: "abc"}
			args.KeyUnlocker = func(cfg *config.AppConfig, a *common.UnlockKeyArgs) (kstypes.StoredKey, error) {
				Expect(a.KeyStoreID).To(Equal(args.Target))
				Expect(a.Passphrase).To(Equal(args.TargetPass))
				Expect(a.TargetRepo).To(BeNil())
				return nil, fmt.Errorf("error")
			}
			err := RegisterCmd(cfg, args)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to unlock the local key: error"))
		})

		It("should return error when target key ID is a local account but unable to unlock the signing account", func() {
			args := &RegisterArgs{Target: "os1", SigningKey: "os1abc", SigningKeyPass: "abc"}
			mockLocalKey := mocks.NewMockStoredKey(ctrl)
			mockLocalKey.EXPECT().GetKey().Return(key)
			args.KeyUnlocker = func(cfg *config.AppConfig, a *common.UnlockKeyArgs) (kstypes.StoredKey, error) {
				Expect(a.KeyStoreID).To(Equal(args.Target))
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
			args := &RegisterArgs{Target: key.PubKey().Base58(), SigningKey: "os1abc", SigningKeyPass: "abc"}
			mockSigningKey := mocks.NewMockStoredKey(ctrl)
			mockSigningKey.EXPECT().GetUserAddress().Return("os1abc")
			args.KeyUnlocker = func(cfg *config.AppConfig, a *common.UnlockKeyArgs) (kstypes.StoredKey, error) {
				return mockSigningKey, nil
			}
			args.GetNextNonce = func(address string, rpcClient client.Client, remoteClients []restclient.Client) (string, error) {
				Expect(address).To(Equal("os1abc"))
				return "", fmt.Errorf("error")
			}
			err := RegisterCmd(cfg, args)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to get signer's next nonce: error"))
		})

		It("should return error when unable create registration transaction", func() {
			args := &RegisterArgs{Target: key.PubKey().Base58(), SigningKey: "os1abc", SigningKeyPass: "abc"}
			mockSigningKey := mocks.NewMockStoredKey(ctrl)
			mockSigningKey.EXPECT().GetUserAddress().Return("os1abc")
			mockSigningKey.EXPECT().GetKey().Return(key)
			args.KeyUnlocker = func(cfg *config.AppConfig, a *common.UnlockKeyArgs) (kstypes.StoredKey, error) {
				return mockSigningKey, nil
			}
			args.GetNextNonce = func(address string, rpcClient client.Client, remoteClients []restclient.Client) (string, error) {
				Expect(address).To(Equal("os1abc"))
				return "10", nil
			}
			args.RegisterPushKey = func(req *types2.BodyRegisterPushKey, rpcClient client.Client, remoteClients []restclient.Client) (hash string, err error) {
				return "", fmt.Errorf("error")
			}
			err := RegisterCmd(cfg, args)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to register push key: error"))
		})

		It("should return no error on successful transaction creation", func() {
			args := &RegisterArgs{Target: key.PubKey().Base58(), SigningKey: "os1abc", SigningKeyPass: "abc", Stdout: ioutil.Discard}
			mockSigningKey := mocks.NewMockStoredKey(ctrl)
			mockSigningKey.EXPECT().GetUserAddress().Return("os1abc")
			mockSigningKey.EXPECT().GetKey().Return(key)
			args.KeyUnlocker = func(cfg *config.AppConfig, a *common.UnlockKeyArgs) (kstypes.StoredKey, error) {
				return mockSigningKey, nil
			}
			args.GetNextNonce = func(address string, rpcClient client.Client, remoteClients []restclient.Client) (string, error) {
				Expect(address).To(Equal("os1abc"))
				return "10", nil
			}
			args.RegisterPushKey = func(req *types2.BodyRegisterPushKey, rpcClient client.Client, remoteClients []restclient.Client) (hash string, err error) {
				Expect(req.PublicKey).To(Equal(key.PubKey().ToPublicKey()))
				Expect(req.FeeCap).To(Equal(args.FeeCap))
				Expect(req.Fee).To(Equal(args.Fee))
				Expect(req.Scopes).To(Equal(args.Scopes))
				Expect(req.Nonce).To(Equal(uint64(10)))
				return "0x123", nil
			}
			args.ShowTxStatusTracker = func(stdout io.Writer, hash string, rpcClient client.Client, remoteClients []restclient.Client) error {
				return nil
			}
			err := RegisterCmd(cfg, args)
			Expect(err).To(BeNil())
		})

		It("should return error when tx tracker returns error", func() {
			args := &RegisterArgs{Target: key.PubKey().Base58(), SigningKey: "os1abc", SigningKeyPass: "abc", Stdout: ioutil.Discard}
			mockSigningKey := mocks.NewMockStoredKey(ctrl)
			mockSigningKey.EXPECT().GetUserAddress().Return("os1abc")
			mockSigningKey.EXPECT().GetKey().Return(key)
			args.KeyUnlocker = func(cfg *config.AppConfig, a *common.UnlockKeyArgs) (kstypes.StoredKey, error) {
				return mockSigningKey, nil
			}
			args.GetNextNonce = func(address string, rpcClient client.Client, remoteClients []restclient.Client) (string, error) {
				return "10", nil
			}
			args.RegisterPushKey = func(req *types2.BodyRegisterPushKey, rpcClient client.Client, remoteClients []restclient.Client) (hash string, err error) {
				return "0x123", nil
			}
			args.ShowTxStatusTracker = func(stdout io.Writer, hash string, rpcClient client.Client, remoteClients []restclient.Client) error {
				return fmt.Errorf("error")
			}
			err := RegisterCmd(cfg, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})
	})
})
