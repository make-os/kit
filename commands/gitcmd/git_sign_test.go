package gitcmd

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/themakeos/lobe/commands/common"
	"github.com/themakeos/lobe/config"
	"github.com/themakeos/lobe/crypto"
	"github.com/themakeos/lobe/keystore/types"
	"github.com/themakeos/lobe/mocks"
	"github.com/themakeos/lobe/remote/server"
	remotetypes "github.com/themakeos/lobe/remote/types"
	"github.com/themakeos/lobe/testutil"
)

var _ = Describe("GitSign", func() {
	var err error
	var cfg *config.AppConfig
	var ctrl *gomock.Controller
	var mockRepo *mocks.MockLocalRepo
	var key *crypto.Key

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		ctrl = gomock.NewController(GinkgoT())
		mockRepo = mocks.NewMockLocalRepo(ctrl)
		key = crypto.NewKeyFromIntSeed(1)
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".GitSignCmd", func() {
		It("should return error when unable to get repo at current working directory", func() {
			args := &GitSignArgs{Args: []string{"", "", "", key.PushAddr().String()}}
			args.RepoGetter = func(path string) (remotetypes.LocalRepo, error) {
				return nil, fmt.Errorf("error")
			}
			err := GitSignCmd(cfg, strings.NewReader("data"), args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to get repo: error"))
		})

		It("should return error when unable to get and unlock the push key", func() {
			args := &GitSignArgs{Args: []string{"", "", "", key.PushAddr().String()}}
			args.RepoGetter = func(path string) (remotetypes.LocalRepo, error) { return mockRepo, nil }
			args.PushKeyUnlocker = func(cfg *config.AppConfig, a *common.UnlockKeyArgs) (types.StoredKey, error) {
				return nil, fmt.Errorf("error")
			}
			err := GitSignCmd(cfg, strings.NewReader("data"), args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to get push key: error"))
		})

		It("should return error when unable to get push token from environment variable", func() {
			args := &GitSignArgs{Args: []string{"", "", "", key.PushAddr().String()}}
			args.RepoGetter = func(path string) (remotetypes.LocalRepo, error) { return mockRepo, nil }
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			args.PushKeyUnlocker = func(cfg *config.AppConfig, a *common.UnlockKeyArgs) (types.StoredKey, error) {
				return mockStoredKey, nil
			}
			config.AppName = "MY_APP"
			err := GitSignCmd(cfg, strings.NewReader("data"), args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("push request token not set"))
		})

		It("should return error when push token could not be decoded", func() {
			args := &GitSignArgs{Args: []string{"", "", "", key.PushAddr().String()}}
			args.RepoGetter = func(path string) (remotetypes.LocalRepo, error) { return mockRepo, nil }
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			args.PushKeyUnlocker = func(cfg *config.AppConfig, a *common.UnlockKeyArgs) (types.StoredKey, error) {
				return mockStoredKey, nil
			}
			config.AppName = "MY_TEST_APP"
			os.Setenv(fmt.Sprintf("%s_LAST_PUSH_TOKEN", config.AppName), "token")
			err := GitSignCmd(cfg, strings.NewReader("data"), args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to decode token: malformed token"))
		})

		It("should successfully create and output a PEM encoded signature with expected headers", func() {
			out := bytes.NewBuffer(nil)
			args := &GitSignArgs{Args: []string{"", "", "", key.PushAddr().String()}, StdOut: out, StdErr: out}
			args.RepoGetter = func(path string) (remotetypes.LocalRepo, error) { return mockRepo, nil }
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			args.PushKeyUnlocker = func(cfg *config.AppConfig, a *common.UnlockKeyArgs) (types.StoredKey, error) {
				return mockStoredKey, nil
			}

			mockStoredKey.EXPECT().GetKey().Return(key).Times(2)
			txDetail := &remotetypes.TxDetail{RepoName: "repo1", RepoNamespace: "namespace",
				Fee: "1.2", PushKeyID: key.PushAddr().String(), Reference: "refs/heads/master", Nonce: 1}
			token := server.MakePushToken(mockStoredKey, txDetail)

			config.AppName = "MY_TEST_APP"
			os.Setenv(fmt.Sprintf("%s_LAST_PUSH_TOKEN", config.AppName), token)
			err := GitSignCmd(cfg, strings.NewReader("data"), args)
			Expect(err).To(BeNil())
			lines := strings.Split(out.String(), "\n")
			Expect(lines).To(ContainElement("-----BEGIN PGP SIGNATURE-----"))
			Expect(lines).To(ContainElement("fee: 1.2"))
			Expect(lines).To(ContainElement("namespace: namespace"))
			Expect(lines).To(ContainElement("nonce: 1"))
			Expect(lines).To(ContainElement("pkID: pk1dmqxfznwyhmkcgcfthlvvt88vajyhnxq7w8nsw"))
			Expect(lines).To(ContainElement("reference: refs/heads/master"))
			Expect(lines).To(ContainElement("repo: repo1"))
			Expect(lines).To(ContainElement("-----END PGP SIGNATURE-----"))
		})
	})
})
