package gitcmd

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/golang/mock/gomock"
	"github.com/make-os/lobe/cmd/common"
	"github.com/make-os/lobe/config"
	"github.com/make-os/lobe/crypto"
	"github.com/make-os/lobe/keystore/types"
	"github.com/make-os/lobe/mocks"
	remotetypes "github.com/make-os/lobe/remote/types"
	"github.com/make-os/lobe/testutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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

		It("should successfully create and output a PEM encoded signature with expected headers", func() {
			out := bytes.NewBuffer(nil)
			args := &GitSignArgs{Args: []string{"", "", "", key.PushAddr().String()}, PushKeyID: key.PushAddr().String(), StdOut: out, StdErr: out}
			args.RepoGetter = func(path string) (remotetypes.LocalRepo, error) { return mockRepo, nil }
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			args.PushKeyUnlocker = func(cfg *config.AppConfig, a *common.UnlockKeyArgs) (types.StoredKey, error) {
				return mockStoredKey, nil
			}

			mockStoredKey.EXPECT().GetKey().Return(key)
			mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())

			config.AppName = "MY_TEST_APP"
			err := GitSignCmd(cfg, strings.NewReader("data"), args)
			Expect(err).To(BeNil())
			lines := strings.Split(out.String(), "\n")
			Expect(lines).To(ContainElement("-----BEGIN PGP SIGNATURE-----"))
			Expect(lines).To(ContainElement("pkID: pk1dmqxfznwyhmkcgcfthlvvt88vajyhnxq7w8nsw"))
			Expect(lines).To(ContainElement("-----END PGP SIGNATURE-----"))
		})
	})
})
