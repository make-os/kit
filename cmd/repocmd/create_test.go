package repocmd

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/make-os/lobe/cmd/common"
	"github.com/make-os/lobe/config"
	"github.com/make-os/lobe/crypto"
	kstypes "github.com/make-os/lobe/keystore/types"
	"github.com/make-os/lobe/mocks"
	"github.com/make-os/lobe/rpc/types"
	"github.com/make-os/lobe/testutil"
	"github.com/make-os/lobe/types/api"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/objx"
)

func TestRepocmd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RepoCmd Suite")
}

var _ = Describe("CreateCmd", func() {
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
			args := &CreateArgs{SigningKey: "1", SigningKeyPass: "pass"}
			args.KeyUnlocker = func(cfg *config.AppConfig, a *common.UnlockKeyArgs) (kstypes.StoredKey, error) {
				Expect(a.KeyStoreID).To(Equal(args.SigningKey))
				Expect(a.Passphrase).To(Equal(args.SigningKeyPass))
				return nil, fmt.Errorf("error")
			}
			err := CreateCmd(cfg, args)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to unlock the signing key: error"))
		})

		It("should return error when nonce is 0 and it failed to fetch next nonce", func() {
			args := &CreateArgs{SigningKey: "1", SigningKeyPass: "pass"}
			mockKey := mocks.NewMockStoredKey(ctrl)
			mockKey.EXPECT().GetUserAddress().Return(key.Addr().String())
			args.KeyUnlocker = func(cfg *config.AppConfig, a *common.UnlockKeyArgs) (kstypes.StoredKey, error) {
				return mockKey, nil
			}
			args.GetNextNonce = func(address string, rpcClient types.Client) (string, error) {
				Expect(address).To(Equal(key.Addr().String()))
				return "", fmt.Errorf("error")
			}
			err := CreateCmd(cfg, args)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to get signer's next nonce: error"))
		})

		It("should return error when to create repo", func() {
			args := &CreateArgs{Name: "repo1", Value: 12.2, Fee: 1.2, SigningKey: "1", SigningKeyPass: "pass", Config: `{"governance": {"propFee": "100"}}`}
			mockKey := mocks.NewMockStoredKey(ctrl)
			mockKey.EXPECT().GetUserAddress().Return(key.Addr().String())
			mockKey.EXPECT().GetKey().Return(key)
			args.KeyUnlocker = func(cfg *config.AppConfig, a *common.UnlockKeyArgs) (kstypes.StoredKey, error) {
				return mockKey, nil
			}
			args.GetNextNonce = func(address string, rpcClient types.Client) (string, error) {
				Expect(address).To(Equal(key.Addr().String()))
				return "2", nil
			}
			args.CreateRepo = func(req *api.BodyCreateRepo, rpcClient types.Client) (hash string, err error) {
				Expect(req.Name).To(Equal(args.Name))
				Expect(objx.New(req.Config).Get("governance.propFee").String()).To(Equal("100"))
				Expect(req.Value).To(Equal(12.2))
				Expect(req.Nonce).To(Equal(uint64(2)))
				Expect(req.Fee).To(Equal(1.2))
				return "", fmt.Errorf("error")
			}
			err := CreateCmd(cfg, args)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("failed to create repo: error"))
		})

		It("should return nil on success", func() {
			args := &CreateArgs{Name: "repo1", Value: 12.2, Fee: 1.2, SigningKey: "1", SigningKeyPass: "pass", Config: `{"governance": {"propFee": "100"}}`, Stdout: ioutil.Discard}
			mockKey := mocks.NewMockStoredKey(ctrl)
			mockKey.EXPECT().GetUserAddress().Return(key.Addr().String())
			mockKey.EXPECT().GetKey().Return(key)
			args.KeyUnlocker = func(cfg *config.AppConfig, a *common.UnlockKeyArgs) (kstypes.StoredKey, error) {
				return mockKey, nil
			}
			args.GetNextNonce = func(address string, rpcClient types.Client) (string, error) {
				Expect(address).To(Equal(key.Addr().String()))
				return "2", nil
			}
			args.CreateRepo = func(req *api.BodyCreateRepo, rpcClient types.Client) (hash string, err error) {
				Expect(req.Name).To(Equal(args.Name))
				Expect(objx.New(req.Config).Get("governance.propFee").String()).To(Equal("100"))
				Expect(req.Value).To(Equal(12.2))
				Expect(req.Nonce).To(Equal(uint64(2)))
				Expect(req.Fee).To(Equal(1.2))
				return "0x123", nil
			}
			args.ShowTxStatusTracker = func(stdout io.Writer, hash string, rpcClient types.Client) error {
				return nil
			}
			err := CreateCmd(cfg, args)
			Expect(err).To(BeNil())
		})

		It("should return error when tx tracker returns error", func() {
			args := &CreateArgs{Name: "repo1", Value: 12.2, Fee: 1.2, SigningKey: "1", SigningKeyPass: "pass", Config: `{"governance": {"propFee": "100"}}`, Stdout: ioutil.Discard}
			mockKey := mocks.NewMockStoredKey(ctrl)
			mockKey.EXPECT().GetUserAddress().Return(key.Addr().String())
			mockKey.EXPECT().GetKey().Return(key)
			args.KeyUnlocker = func(cfg *config.AppConfig, a *common.UnlockKeyArgs) (kstypes.StoredKey, error) {
				return mockKey, nil
			}
			args.GetNextNonce = func(address string, rpcClient types.Client) (string, error) {
				return "2", nil
			}
			args.CreateRepo = func(req *api.BodyCreateRepo, rpcClient types.Client) (hash string, err error) {
				return "0x123", nil
			}
			args.ShowTxStatusTracker = func(stdout io.Writer, hash string, rpcClient types.Client) error {
				return fmt.Errorf("error")
			}
			err := CreateCmd(cfg, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})
	})
})
