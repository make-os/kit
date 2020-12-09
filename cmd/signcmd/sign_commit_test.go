package signcmd

import (
	"fmt"
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/cmd/common"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/crypto/ed25519"
	"github.com/make-os/kit/keystore/types"
	"github.com/make-os/kit/mocks"
	"github.com/make-os/kit/remote/server"
	remotetypes "github.com/make-os/kit/remote/types"
	types2 "github.com/make-os/kit/rpc/types"
	"github.com/make-os/kit/testutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

func TestSignCmd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SignCmd Suite")
}

var testGetNextNonce = func(pushKeyID string, rpcClient types2.Client) (string, error) {
	return "1", nil
}

func testPushKeyUnlocker(key types.StoredKey, err error) common.KeyUnlocker {
	return func(cfg *config.AppConfig, a *common.UnlockKeyArgs) (types.StoredKey, error) {
		return key, err
	}
}

func testSetRemotePushToken(token string, err error) func(targetRepo remotetypes.LocalRepo, args *server.MakeAndApplyPushTokenToRemoteArgs) error {
	return func(targetRepo remotetypes.LocalRepo, args *server.MakeAndApplyPushTokenToRemoteArgs) error {
		return err
	}
}

func mockGetConfig(kv map[string]string) func(path string) string {
	return func(path string) string {
		return kv[path]
	}
}

var _ = Describe("SignCommit", func() {
	var err error
	var cfg *config.AppConfig
	var ctrl *gomock.Controller
	var mockRepo *mocks.MockLocalRepo
	var key *ed25519.Key

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		ctrl = gomock.NewController(GinkgoT())
		mockRepo = mocks.NewMockLocalRepo(ctrl)
		key = ed25519.NewKeyFromIntSeed(1)
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".populateSignCommitArgsFromRepoConfig", func() {
		It("should populate argument from config", func() {
			mockRepo.EXPECT().GetGitConfigOption("user.signingKey").Return("xyz")
			mockRepo.EXPECT().GetGitConfigOption("user.passphrase").Return("abc")
			mockRepo.EXPECT().GetGitConfigOption("user.fee").Return("10.3")
			mockRepo.EXPECT().GetGitConfigOption("user.nonce").Return("1")
			mockRepo.EXPECT().GetGitConfigOption("user.value").Return("34.5")
			mockRepo.EXPECT().GetGitConfigOption("sign.mergeID").Return("123")
			args := &SignCommitArgs{}
			populateSignCommitArgsFromRepoConfig(mockRepo, args)
			Expect(args.SigningKey).To(Equal("xyz"))
			Expect(args.PushKeyPass).To(Equal("abc"))
			Expect(args.Fee).To(Equal("10.3"))
			Expect(args.Nonce).To(Equal(uint64(1)))
			Expect(args.Value).To(Equal("34.5"))
			Expect(args.MergeID).To(Equal("123"))
		})
	})

	Describe(".SignCommitCmd", func() {

		It("should return error when push key ID is not provided and set args.MergeID if set in config", func() {
			mockRepo.EXPECT().GetGitConfigOption(gomock.Any()).DoAndReturn(mockGetConfig(map[string]string{
				"sign.mergeID": "123",
			})).AnyTimes()
			args := &SignCommitArgs{}
			err := SignCommitCmd(cfg, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(ErrMissingPushKeyID))
			Expect(args.MergeID).To(Equal("123"))
		})

		It("should return error when unable to find and unlock push key", func() {
			mockRepo.EXPECT().GetGitConfigOption(gomock.Any()).DoAndReturn(mockGetConfig(map[string]string{
				"user.signingKey": key.PushAddr().String(),
			})).AnyTimes()

			args := &SignCommitArgs{}
			args.KeyUnlocker = testPushKeyUnlocker(nil, fmt.Errorf("error"))
			err := SignCommitCmd(cfg, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("failed to unlock the signing key: error"))
		})

		It("should attempt to get pusher key if signing key is a user key", func() {
			mockRepo.EXPECT().GetGitConfigOption(gomock.Any()).DoAndReturn(mockGetConfig(map[string]string{
				"user.signingKey": key.PushAddr().String(),
			})).AnyTimes()

			args := &SignCommitArgs{}
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			mockStoredKey.EXPECT().GetMeta().Return(types.StoredKeyMeta{})
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.Addr().String())
			args.GetNextNonce = func(address string, rpcClient types2.Client) (string, error) {
				return "", fmt.Errorf("error")
			}
			_ = SignCommitCmd(cfg, mockRepo, args)
		})

		It("should return error when mergeID is set but invalid", func() {
			mockRepo.EXPECT().GetGitConfigOption(gomock.Any()).DoAndReturn(mockGetConfig(map[string]string{
				"user.signingKey": key.PushAddr().String(),
			})).AnyTimes()

			args := &SignCommitArgs{MergeID: "abc123_invalid"}
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			mockStoredKey.EXPECT().GetMeta().Return(types.StoredKeyMeta{})
			mockStoredKey.EXPECT().GetMeta().Return(types.StoredKeyMeta{})
			mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String()).Times(2)
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			err := SignCommitCmd(cfg, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("merge proposal id must be numeric"))
			args.MergeID = "12345678910"
			err = SignCommitCmd(cfg, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("merge proposal id exceeded 8 bytes limit"))
		})

		It("should return error when unable to get next nonce", func() {
			mockRepo.EXPECT().GetGitConfigOption(gomock.Any()).DoAndReturn(mockGetConfig(map[string]string{
				"user.signingKey": key.PushAddr().String(),
			})).AnyTimes()

			args := &SignCommitArgs{GetNextNonce: func(pushKeyID string, rpcClient types2.Client) (string, error) {
				return "", fmt.Errorf("error")
			}}
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			mockStoredKey.EXPECT().GetMeta().Return(types.StoredKeyMeta{})
			mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			err := SignCommitCmd(cfg, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("failed to get next nonce: error"))
		})

		It("should return error when unable to get local repo HEAD", func() {
			mockRepo.EXPECT().GetGitConfigOption(gomock.Any()).DoAndReturn(mockGetConfig(map[string]string{
				"user.signingKey": key.PushAddr().String(),
			})).AnyTimes()
			args := &SignCommitArgs{GetNextNonce: testGetNextNonce}
			mockRepo.EXPECT().Head().Return("", fmt.Errorf("error"))
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			mockStoredKey.EXPECT().GetMeta().Return(types.StoredKeyMeta{})
			mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			err := SignCommitCmd(cfg, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("failed to get HEAD"))
		})

		When("HEAD is current reference branch and it was not found", func() {
			It("should return error", func() {
				ref := plumbing.ReferenceName("refs/heads/master")
				mockRepo.EXPECT().GetGitConfigOption(gomock.Any()).AnyTimes()
				args := &SignCommitArgs{Fee: "1", SigningKey: key.PushAddr().String(), Message: "some message", GetNextNonce: testGetNextNonce}
				mockStoredKey := mocks.NewMockStoredKey(ctrl)
				args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
				mockStoredKey.EXPECT().GetMeta().Return(types.StoredKeyMeta{})
				mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())
				mockRepo.EXPECT().Head().Return(ref.String(), nil)
				mockRepo.EXPECT().Reference(ref, false).Return(nil, fmt.Errorf("error"))
				err := SignCommitCmd(cfg, mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to find reference: refs/heads/master: error"))
			})
		})

		When("HEAD is args.HEAD and it was not found", func() {
			It("should return error", func() {
				ref := plumbing.ReferenceName("refs/heads/master")
				ref2 := plumbing.ReferenceName("refs/heads/dev")
				mockRepo.EXPECT().GetGitConfigOption(gomock.Any()).AnyTimes()
				args := &SignCommitArgs{Fee: "1", SigningKey: key.PushAddr().String(), Message: "some message",
					GetNextNonce: testGetNextNonce, Head: ref2.String()}
				mockStoredKey := mocks.NewMockStoredKey(ctrl)
				args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
				mockStoredKey.EXPECT().GetMeta().Return(types.StoredKeyMeta{})
				mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())
				mockRepo.EXPECT().Head().Return(ref.String(), nil)
				mockRepo.EXPECT().Reference(ref2, false).Return(nil, fmt.Errorf("error"))
				err := SignCommitCmd(cfg, mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to find reference: refs/heads/dev: error"))
			})
		})

		When("HEAD is args.HEAD is not a full reference name and", func() {
			It("should be expanded", func() {
				ref := plumbing.ReferenceName("refs/heads/master")
				ref2 := "dev"
				expected := plumbing.ReferenceName("refs/heads/dev")
				mockRepo.EXPECT().GetGitConfigOption(gomock.Any()).AnyTimes()
				args := &SignCommitArgs{Fee: "1", SigningKey: key.PushAddr().String(), Message: "some message",
					GetNextNonce: testGetNextNonce, Head: ref2}
				mockStoredKey := mocks.NewMockStoredKey(ctrl)
				args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
				mockStoredKey.EXPECT().GetMeta().Return(types.StoredKeyMeta{})
				mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())
				mockRepo.EXPECT().Head().Return(ref.String(), nil)
				mockRepo.EXPECT().Reference(expected, false).Return(nil, fmt.Errorf("error"))
				_ = SignCommitCmd(cfg, mockRepo, args)
			})
		})

		It("should return error when unable to create apply push token", func() {
			refName := plumbing.ReferenceName("refs/heads/master")
			ref := plumbing.NewHashReference(refName, plumbing.NewHash("5cb1af69935120f4944a8cd515f008e12290de52"))
			mockRepo.EXPECT().GetGitConfigOption(gomock.Any()).AnyTimes()
			args := &SignCommitArgs{Fee: "1", SigningKey: key.PushAddr().String(), Message: "some message", GetNextNonce: testGetNextNonce}
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			mockStoredKey.EXPECT().GetMeta().Return(types.StoredKeyMeta{})
			mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())
			mockRepo.EXPECT().Head().Return(refName.String(), nil)
			mockRepo.EXPECT().Reference(refName, false).Return(ref, nil)
			args.CreateApplyPushTokenToRemote = func(targetRepo remotetypes.LocalRepo, args *server.MakeAndApplyPushTokenToRemoteArgs) error {
				return fmt.Errorf("error")
			}
			err = SignCommitCmd(cfg, mockRepo, args)
			Expect(err).To(MatchError("error"))
		})

		It("should return nil when able to create apply push token", func() {
			refName := plumbing.ReferenceName("refs/heads/master")
			ref := plumbing.NewHashReference(refName, plumbing.NewHash("5cb1af69935120f4944a8cd515f008e12290de52"))
			mockRepo.EXPECT().GetGitConfigOption(gomock.Any()).AnyTimes()
			args := &SignCommitArgs{Fee: "1", SigningKey: key.PushAddr().String(), Message: "some message", GetNextNonce: testGetNextNonce}
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			mockStoredKey.EXPECT().GetMeta().Return(types.StoredKeyMeta{})
			mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())
			mockRepo.EXPECT().Head().Return(refName.String(), nil)
			mockRepo.EXPECT().Reference(refName, false).Return(ref, nil)
			args.CreateApplyPushTokenToRemote = func(targetRepo remotetypes.LocalRepo, args *server.MakeAndApplyPushTokenToRemoteArgs) error {
				return nil
			}
			err = SignCommitCmd(cfg, mockRepo, args)
			Expect(err).To(BeNil())
		})
	})
})
