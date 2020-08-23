package signcmd

import (
	"encoding/pem"
	"fmt"
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	restclient "github.com/themakeos/lobe/api/remote/client"
	"github.com/themakeos/lobe/api/rpc/client"
	"github.com/themakeos/lobe/cmd/common"
	"github.com/themakeos/lobe/config"
	"github.com/themakeos/lobe/crypto"
	"github.com/themakeos/lobe/keystore/types"
	"github.com/themakeos/lobe/mocks"
	"github.com/themakeos/lobe/remote/server"
	remotetypes "github.com/themakeos/lobe/remote/types"
	"github.com/themakeos/lobe/testutil"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

var testGetNextNonce = func(pushKeyID string, rpcClient client.Client, remoteClients []restclient.Client) (string, error) {
	return "1", nil
}

func testPushKeyUnlocker(key types.StoredKey, err error) common.KeyUnlocker {
	return func(cfg *config.AppConfig, a *common.UnlockKeyArgs) (types.StoredKey, error) {
		return key, err
	}
}

func testSetRemotePushToken(token string, err error) func(targetRepo remotetypes.LocalRepo, args *server.SetRemotePushTokenArgs) (string, error) {
	return func(targetRepo remotetypes.LocalRepo, args *server.SetRemotePushTokenArgs) (string, error) {
		return token, err
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

	Describe(".populateSignCommitArgsFromRepoConfig", func() {
		It("should populate argument from config", func() {
			mockRepo.EXPECT().GetConfig("user.signingKey").Return("xyz")
			mockRepo.EXPECT().GetConfig("user.passphrase").Return("abc")
			mockRepo.EXPECT().GetConfig("user.fee").Return("10.3")
			mockRepo.EXPECT().GetConfig("user.nonce").Return("1")
			mockRepo.EXPECT().GetConfig("user.value").Return("34.5")
			mockRepo.EXPECT().GetConfig("commit.amend").Return("true")
			mockRepo.EXPECT().GetConfig("sign.noUsername").Return("true")
			mockRepo.EXPECT().GetConfig("sign.mergeID").Return("123")
			args := &SignCommitArgs{}
			populateSignCommitArgsFromRepoConfig(mockRepo, args)
			Expect(args.SigningKey).To(Equal("xyz"))
			Expect(args.PushKeyPass).To(Equal("abc"))
			Expect(args.Fee).To(Equal("10.3"))
			Expect(args.Nonce).To(Equal(uint64(1)))
			Expect(args.Value).To(Equal("34.5"))
			Expect(args.AmendCommit).To(Equal(true))
			Expect(args.SetRemotePushTokensOptionOnly).To(BeTrue())
			Expect(args.MergeID).To(Equal("123"))
		})
	})

	Describe(".SignCommitCmd", func() {

		It("should return error when push key ID is not provided and set args.MergeID if set in config", func() {
			mockRepo.EXPECT().GetConfig(gomock.Any()).DoAndReturn(mockGetConfig(map[string]string{
				"sign.mergeID": "123",
			})).AnyTimes()
			args := &SignCommitArgs{}
			err := SignCommitCmd(cfg, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(ErrMissingPushKeyID))
			Expect(args.MergeID).To(Equal("123"))
		})

		It("should return error when unable to find and unlock push key", func() {
			mockRepo.EXPECT().GetConfig(gomock.Any()).DoAndReturn(mockGetConfig(map[string]string{
				"user.signingKey": key.PushAddr().String(),
			})).AnyTimes()

			args := &SignCommitArgs{}
			args.KeyUnlocker = testPushKeyUnlocker(nil, fmt.Errorf("error"))
			err := SignCommitCmd(cfg, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("failed to unlock the signing key: error"))
		})

		It("should attempt to get pusher key if signing key is a user key", func() {
			mockRepo.EXPECT().GetConfig(gomock.Any()).DoAndReturn(mockGetConfig(map[string]string{
				"user.signingKey": key.PushAddr().String(),
			})).AnyTimes()

			args := &SignCommitArgs{}
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			mockStoredKey.EXPECT().GetMeta().Return(types.StoredKeyMeta{})
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.Addr().String())
			args.GetNextNonce = func(address string, rpcClient client.Client, remoteClients []restclient.Client) (string, error) {
				return "", fmt.Errorf("error")
			}
			SignCommitCmd(cfg, mockRepo, args)
		})

		It("should return error when mergeID is set but invalid", func() {
			mockRepo.EXPECT().GetConfig(gomock.Any()).DoAndReturn(mockGetConfig(map[string]string{
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
			mockRepo.EXPECT().GetConfig(gomock.Any()).DoAndReturn(mockGetConfig(map[string]string{
				"user.signingKey": key.PushAddr().String(),
			})).AnyTimes()

			args := &SignCommitArgs{GetNextNonce: func(pushKeyID string, rpcClient client.Client, remoteClients []restclient.Client) (string, error) {
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
			mockRepo.EXPECT().GetConfig(gomock.Any()).DoAndReturn(mockGetConfig(map[string]string{
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

		When("args.Branch is set", func() {
			It("should return error when unable to checkout branch", func() {
				mockRepo.EXPECT().GetConfig(gomock.Any()).DoAndReturn(mockGetConfig(map[string]string{
					"user.signingKey": key.PushAddr().String(),
				})).AnyTimes()
				args := &SignCommitArgs{GetNextNonce: testGetNextNonce, Branch: "refs/heads/dev"}
				mockStoredKey := mocks.NewMockStoredKey(ctrl)
				mockStoredKey.EXPECT().GetMeta().Return(types.StoredKeyMeta{})
				args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
				mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())
				mockRepo.EXPECT().Head().Return("refs/heads/master", nil)
				mockRepo.EXPECT().Checkout("dev", false, args.ForceCheckout).Return(fmt.Errorf("error"))
				err := SignCommitCmd(cfg, mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to checkout branch (refs/heads/dev): error"))
			})

			It("should return error when unable to checkout branch", func() {
				mockRepo.EXPECT().GetConfig(gomock.Any()).DoAndReturn(mockGetConfig(map[string]string{
					"user.signingKey": key.PushAddr().String(),
				})).AnyTimes()
				args := &SignCommitArgs{GetNextNonce: testGetNextNonce, Branch: "refs/heads/dev"}
				mockStoredKey := mocks.NewMockStoredKey(ctrl)
				mockStoredKey.EXPECT().GetMeta().Return(types.StoredKeyMeta{})
				args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
				mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())
				mockRepo.EXPECT().Head().Return("refs/heads/master", nil)
				mockRepo.EXPECT().Checkout("dev", false, args.ForceCheckout).Return(fmt.Errorf("error"))
				err := SignCommitCmd(cfg, mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to checkout branch (refs/heads/dev): error"))
			})
		})

		When("previous commit amendment is not required (AmendCommit=false", func() {
			BeforeEach(func() {
				mockRepo.EXPECT().GetConfig(gomock.Any()).AnyTimes()
			})

			It("should attempt to create a new commit and return error on failure", func() {
				args := &SignCommitArgs{Fee: "1", SigningKey: key.PushAddr().String(), Message: "some message", GetNextNonce: testGetNextNonce, AmendCommit: false}
				args.SetRemotePushToken = testSetRemotePushToken("", nil)
				mockStoredKey := mocks.NewMockStoredKey(ctrl)
				mockStoredKey.EXPECT().GetMeta().Return(types.StoredKeyMeta{})
				args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
				mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())
				mockRepo.EXPECT().Head().Return("refs/heads/master", nil)
				mockRepo.EXPECT().GetName().Return("repo_name")
				mockRepo.EXPECT().HeadObject().Return(nil, nil)
				mockRepo.EXPECT().CreateEmptyCommit(args.Message, args.SigningKey).Return(fmt.Errorf("error"))
				err := SignCommitCmd(cfg, mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("error"))
			})

			It("should attempt to create a new commit and return nil on success", func() {
				args := &SignCommitArgs{Fee: "1", SigningKey: key.PushAddr().String(), Message: "some message", GetNextNonce: testGetNextNonce, AmendCommit: false}
				args.SetRemotePushToken = testSetRemotePushToken("", nil)
				mockStoredKey := mocks.NewMockStoredKey(ctrl)
				mockStoredKey.EXPECT().GetMeta().Return(types.StoredKeyMeta{})
				args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
				mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())
				mockRepo.EXPECT().Head().Return("refs/heads/master", nil)
				mockRepo.EXPECT().GetName().Return("repo_name")
				mockRepo.EXPECT().HeadObject().Return(nil, nil)
				mockRepo.EXPECT().CreateEmptyCommit(args.Message, args.SigningKey).Return(nil)
				mockRepo.EXPECT().GetRecentCommitHash().Return("hash1", nil)
				err := SignCommitCmd(cfg, mockRepo, args)
				Expect(err).To(BeNil())
			})

			When("args.SigningKey is a user address", func() {
				It(`should pass push key to CreateEmptyCommit, TxDetail object and GetNextNonce`, func() {
					mockRepo.EXPECT().GetConfig(gomock.Any()).AnyTimes()
					args := &SignCommitArgs{Fee: "1", SigningKey: key.Addr().String(), Message: "some message", AmendCommit: false}
					args.GetNextNonce = func(address string, rpcClient client.Client, remoteClients []restclient.Client) (string, error) {
						Expect(address).To(Equal(key.PushAddr().String()))
						return "1", nil
					}
					args.SetRemotePushToken = func(r remotetypes.LocalRepo, args *server.SetRemotePushTokenArgs) (string, error) {
						Expect(args.TxDetail.PushKeyID).To(Equal(key.PushAddr().String()))
						return "", nil
					}
					mockRepo.EXPECT().GetName().Return("repo_name")
					mockStoredKey := mocks.NewMockStoredKey(ctrl)
					mockStoredKey.EXPECT().GetMeta().Return(types.StoredKeyMeta{})
					args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
					mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())
					mockRepo.EXPECT().Head().Return("refs/heads/master", nil)
					mockRepo.EXPECT().HeadObject().Return(nil, nil)
					mockRepo.EXPECT().CreateEmptyCommit(args.Message, args.SigningKey).Return(nil)
					mockRepo.EXPECT().GetRecentCommitHash().Return("hash1", nil)
					err := SignCommitCmd(cfg, mockRepo, args)
					Expect(err).To(BeNil())
				})
			})
		})

		When("args.Head is set'", func() {
			Specify("that TxDetail.Reference is set to 'refs/heads/some_branch' when args.Head is 'refs/heads/some_branch'", func() {
				mockRepo.EXPECT().GetConfig(gomock.Any()).AnyTimes()
				args := &SignCommitArgs{Fee: "1", SigningKey: key.PushAddr().String(), Message: "some message", GetNextNonce: testGetNextNonce, AmendCommit: false, Head: "refs/heads/some_branch"}
				args.SetRemotePushToken = func(targetRepo remotetypes.LocalRepo, args *server.SetRemotePushTokenArgs) (string, error) {
					Expect(args.TxDetail.Reference).To(Equal("refs/heads/some_branch"))
					return "", nil
				}
				mockRepo.EXPECT().GetName().Return("repo_name")
				mockStoredKey := mocks.NewMockStoredKey(ctrl)
				args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
				mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())
				mockStoredKey.EXPECT().GetMeta().Return(types.StoredKeyMeta{})
				mockRepo.EXPECT().Head().Return("refs/heads/master", nil)
				mockRepo.EXPECT().HeadObject().Return(nil, nil)
				mockRepo.EXPECT().CreateEmptyCommit(args.Message, args.SigningKey).Return(nil)
				mockRepo.EXPECT().GetRecentCommitHash().Return("hash1", nil)
				err := SignCommitCmd(cfg, mockRepo, args)
				Expect(err).To(BeNil())
			})

			Specify("that TxDetail.Reference is set to 'refs/heads/some_branch' when args.Head is 'some_branch'", func() {
				mockRepo.EXPECT().GetConfig(gomock.Any()).AnyTimes()
				args := &SignCommitArgs{Fee: "1", SigningKey: key.PushAddr().String(), Message: "some message", GetNextNonce: testGetNextNonce, AmendCommit: false, Head: "some_branch"}
				args.SetRemotePushToken = func(targetRepo remotetypes.LocalRepo, args *server.SetRemotePushTokenArgs) (string, error) {
					Expect(args.TxDetail.Reference).To(Equal("refs/heads/some_branch"))
					return "", nil
				}
				mockRepo.EXPECT().GetName().Return("repo_name")
				mockStoredKey := mocks.NewMockStoredKey(ctrl)
				args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
				mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())
				mockStoredKey.EXPECT().GetMeta().Return(types.StoredKeyMeta{})
				mockRepo.EXPECT().Head().Return("refs/heads/master", nil)
				mockRepo.EXPECT().HeadObject().Return(nil, nil)
				mockRepo.EXPECT().CreateEmptyCommit(args.Message, args.SigningKey).Return(nil)
				mockRepo.EXPECT().GetRecentCommitHash().Return("hash1", nil)
				err := SignCommitCmd(cfg, mockRepo, args)
				Expect(err).To(BeNil())
			})
		})

		When("checkout args.Branch", func() {
			It("should return error if args.Branch is set and checkout failed", func() {
				mockRepo.EXPECT().GetConfig(gomock.Any()).AnyTimes()
				args := &SignCommitArgs{Fee: "1", SigningKey: key.PushAddr().String(),
					Message: "some message", GetNextNonce: testGetNextNonce,
					AmendCommit: false, Head: "refs/heads/some_branch",
					Branch: "some_branch"}
				args.SetRemotePushToken = testSetRemotePushToken("", nil)

				mockStoredKey := mocks.NewMockStoredKey(ctrl)
				args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
				mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())
				mockStoredKey.EXPECT().GetMeta().Return(types.StoredKeyMeta{})
				mockRepo.EXPECT().Head().Return("refs/heads/master", nil)
				mockRepo.EXPECT().Checkout("some_branch", false, args.ForceCheckout).Return(fmt.Errorf("error"))
				err := SignCommitCmd(cfg, mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("failed to checkout branch (refs/heads/some_branch): error"))
			})

			It("should return no error and revert checkout to HEAD if args.Branch is set and different from HEAD", func() {
				mockRepo.EXPECT().GetConfig(gomock.Any()).AnyTimes()
				args := &SignCommitArgs{Fee: "1", SigningKey: key.PushAddr().String(),
					Message: "some message", GetNextNonce: testGetNextNonce,
					AmendCommit: false, Head: "refs/heads/some_branch",
					Branch: "some_branch"}
				args.SetRemotePushToken = testSetRemotePushToken("", nil)

				mockRepo.EXPECT().GetName().Return("repo_name")
				mockStoredKey := mocks.NewMockStoredKey(ctrl)
				args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
				mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())
				mockStoredKey.EXPECT().GetMeta().Return(types.StoredKeyMeta{})
				mockRepo.EXPECT().Head().Return("refs/heads/master", nil)
				mockRepo.EXPECT().Checkout("some_branch", false, args.ForceCheckout).Return(nil)
				mockRepo.EXPECT().HeadObject().Return(nil, nil)
				mockRepo.EXPECT().CreateEmptyCommit(args.Message, args.SigningKey).Return(nil)
				mockRepo.EXPECT().GetRecentCommitHash().Return("hash1", nil)
				mockRepo.EXPECT().Checkout("master", false, args.ForceCheckout).Return(nil)
				err := SignCommitCmd(cfg, mockRepo, args)
				Expect(err).To(BeNil())
			})
		})

		When("unable to create and set push token", func() {
			It("should return error", func() {
				mockRepo.EXPECT().GetConfig(gomock.Any()).AnyTimes()
				args := &SignCommitArgs{Fee: "1", SigningKey: key.PushAddr().String(), Message: "some message", GetNextNonce: testGetNextNonce, AmendCommit: false, Head: "refs/heads/some_branch"}
				args.SetRemotePushToken = func(r remotetypes.LocalRepo, args *server.SetRemotePushTokenArgs) (string, error) {
					Expect(args.TxDetail.Reference).To(Equal("refs/heads/some_branch"))
					return "", nil
				}

				mockRepo.EXPECT().GetName().Return("repo_name")
				mockStoredKey := mocks.NewMockStoredKey(ctrl)
				args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
				mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())
				mockStoredKey.EXPECT().GetMeta().Return(types.StoredKeyMeta{})
				mockRepo.EXPECT().Head().Return("refs/heads/master", nil)
				mockRepo.EXPECT().HeadObject().Return(nil, nil)
				mockRepo.EXPECT().CreateEmptyCommit(args.Message, args.SigningKey).Return(nil)
				mockRepo.EXPECT().GetRecentCommitHash().Return("hash1", nil)
				args.SetRemotePushToken = testSetRemotePushToken("", fmt.Errorf("error"))
				err := SignCommitCmd(cfg, mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("error"))
			})
		})

		When("amend of previous commit is required (AmendCommit=true)", func() {
			BeforeEach(func() {
				mockRepo.EXPECT().GetConfig(gomock.Any()).AnyTimes()
			})

			It("should return error when unable to get recent commit due to ErrNoCommits", func() {
				args := &SignCommitArgs{Fee: "1", SigningKey: key.PushAddr().String(), Message: "some message", GetNextNonce: testGetNextNonce, AmendCommit: true}
				args.SetRemotePushToken = testSetRemotePushToken("", nil)
				mockStoredKey := mocks.NewMockStoredKey(ctrl)
				mockStoredKey.EXPECT().GetMeta().Return(types.StoredKeyMeta{})
				args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
				mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())
				mockRepo.EXPECT().GetName().Return("repo_name")
				mockRepo.EXPECT().Head().Return("refs/heads/master", nil)
				mockRepo.EXPECT().HeadObject().Return(nil, nil)
				err := SignCommitCmd(cfg, mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("no commit found; empty branch"))
			})

			It("should use previous commit message if args.Message is not set", func() {
				args := &SignCommitArgs{Fee: "1", SigningKey: key.PushAddr().String(), Message: "", GetNextNonce: testGetNextNonce, AmendCommit: true}
				args.SetRemotePushToken = testSetRemotePushToken("", nil)
				mockStoredKey := mocks.NewMockStoredKey(ctrl)
				mockStoredKey.EXPECT().GetMeta().Return(types.StoredKeyMeta{})
				args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
				mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())
				mockRepo.EXPECT().GetName().Return("repo_name")
				mockRepo.EXPECT().Head().Return("refs/heads/master", nil)
				recentCommit := &object.Commit{Message: "This is a commit"}
				mockRepo.EXPECT().HeadObject().Return(recentCommit, nil)
				mockRepo.EXPECT().AmendRecentCommitWithMsg(recentCommit.Message, args.SigningKey).Return(nil)
				mockRepo.EXPECT().GetRecentCommitHash().Return("000hash", nil)
				err := SignCommitCmd(cfg, mockRepo, args)
				Expect(err).To(BeNil())
				Expect(args.Message).To(Equal(recentCommit.Message))
			})

			It("should return error when unable to update recent commit", func() {
				args := &SignCommitArgs{Fee: "1", SigningKey: key.PushAddr().String(), Message: "", GetNextNonce: testGetNextNonce, AmendCommit: true}
				args.SetRemotePushToken = testSetRemotePushToken("", nil)
				mockStoredKey := mocks.NewMockStoredKey(ctrl)
				mockStoredKey.EXPECT().GetMeta().Return(types.StoredKeyMeta{})
				args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
				mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())
				mockRepo.EXPECT().GetName().Return("repo_name")
				mockRepo.EXPECT().Head().Return("refs/heads/master", nil)
				recentCommit := &object.Commit{Message: "This is a commit"}
				mockRepo.EXPECT().HeadObject().Return(recentCommit, nil)
				mockRepo.EXPECT().AmendRecentCommitWithMsg(recentCommit.Message, args.SigningKey).Return(fmt.Errorf("error"))
				err := SignCommitCmd(cfg, mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("error"))
			})
		})

		When("APPNAME_REPONAME_PASS is not set", func() {
			It("should set it to the value of args.PushKeyPass", func() {
				mockRepo.EXPECT().GetConfig(gomock.Any()).AnyTimes()
				args := &SignCommitArgs{Fee: "1", SigningKey: key.PushAddr().String(), Message: "some message",
					GetNextNonce: testGetNextNonce, AmendCommit: false, PushKeyPass: "password"}
				args.SetRemotePushToken = testSetRemotePushToken("", nil)

				mockRepo.EXPECT().GetName().Return("repo_name")
				mockStoredKey := mocks.NewMockStoredKey(ctrl)
				args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
				mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())
				mockStoredKey.EXPECT().GetMeta().Return(types.StoredKeyMeta{})
				mockRepo.EXPECT().Head().Return("refs/heads/master", nil)
				mockRepo.EXPECT().HeadObject().Return(&object.Commit{Message: "This is a commit"}, nil)
				mockRepo.EXPECT().CreateEmptyCommit(args.Message, args.SigningKey).Return(nil)
				mockRepo.EXPECT().GetRecentCommitHash().Return("hash1", nil)
				err := SignCommitCmd(cfg, mockRepo, args)
				Expect(err).To(BeNil())

				passVar := common.MakeRepoScopedEnvVar(cfg.GetExecName(), "repo_name", "PASS")
				Expect(os.Getenv(passVar)).To(Equal(args.PushKeyPass))
			})
		})

		When(".CreatePushTokenOnly is set to true", func() {
			It("should skip code for reference signing", func() {
				mockRepo.EXPECT().GetConfig(gomock.Any()).AnyTimes()
				args := &SignCommitArgs{Fee: "1", SigningKey: key.PushAddr().String(),
					Message: "", GetNextNonce: testGetNextNonce, CreatePushTokenOnly: true}
				args.SetRemotePushToken = testSetRemotePushToken("", nil)
				mockStoredKey := mocks.NewMockStoredKey(ctrl)
				mockStoredKey.EXPECT().GetMeta().Return(types.StoredKeyMeta{})
				args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
				mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())
				mockRepo.EXPECT().GetName().Return("repo_name")
				mockRepo.EXPECT().Head().Return("refs/heads/master", nil)
				mockRepo.EXPECT().HeadObject().Return(&object.Commit{Message: "This is a commit"}, nil)
				mockRepo.EXPECT().GetRecentCommitHash().Return("000hash", nil)
				err := SignCommitCmd(cfg, mockRepo, args)
				Expect(err).To(BeNil())
			})
		})

		When("recent commit is already signed", func() {
			It("should skip signing if push key in signature header matches signing key", func() {
				mockRepo.EXPECT().GetConfig(gomock.Any()).AnyTimes()
				args := &SignCommitArgs{Fee: "1", SigningKey: key.PushAddr().String(), GetNextNonce: testGetNextNonce}
				args.SetRemotePushToken = testSetRemotePushToken("", nil)
				mockStoredKey := mocks.NewMockStoredKey(ctrl)
				mockStoredKey.EXPECT().GetMeta().Return(types.StoredKeyMeta{})
				args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
				mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())
				mockRepo.EXPECT().GetName().Return("repo_name")
				mockRepo.EXPECT().Head().Return("refs/heads/master", nil)
				pemData := pem.EncodeToMemory(&pem.Block{Headers: map[string]string{"pkID": key.PushAddr().String()}})
				mockRepo.EXPECT().HeadObject().Return(&object.Commit{
					Message:      "This is a commit",
					PGPSignature: string(pemData),
				}, nil)
				mockRepo.EXPECT().GetRecentCommitHash().Return("000hash", nil)
				err := SignCommitCmd(cfg, mockRepo, args)
				Expect(err).To(BeNil())
			})

			When("ForceSign is true", func() {
				It("should not skip signing even if push key in signature header matches signing key", func() {
					mockRepo.EXPECT().GetConfig(gomock.Any()).AnyTimes()
					args := &SignCommitArgs{Fee: "1", SigningKey: key.PushAddr().String(), GetNextNonce: testGetNextNonce, ForceSign: true}
					args.SetRemotePushToken = testSetRemotePushToken("", nil)
					mockStoredKey := mocks.NewMockStoredKey(ctrl)
					mockStoredKey.EXPECT().GetMeta().Return(types.StoredKeyMeta{})
					args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
					mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())
					mockRepo.EXPECT().GetName().Return("repo_name")
					mockRepo.EXPECT().Head().Return("refs/heads/master", nil)
					pemData := pem.EncodeToMemory(&pem.Block{Headers: map[string]string{"pkID": key.PushAddr().String()}})
					mockRepo.EXPECT().HeadObject().Return(&object.Commit{
						Message:      "This is a commit",
						PGPSignature: string(pemData),
					}, nil)
					mockRepo.EXPECT().CreateEmptyCommit(args.Message, args.SigningKey).Return(nil)
					mockRepo.EXPECT().GetRecentCommitHash().Return("000hash", nil)
					err := SignCommitCmd(cfg, mockRepo, args)
					Expect(err).To(BeNil())
				})
			})
		})

		When(".SignRefOnly is set to true", func() {
			It("should skip code for push token creation and signing", func() {
				mockRepo.EXPECT().GetConfig(gomock.Any()).AnyTimes()
				args := &SignCommitArgs{Fee: "1", SigningKey: key.PushAddr().String(),
					Message: "", GetNextNonce: testGetNextNonce, SignRefOnly: true}
				mockStoredKey := mocks.NewMockStoredKey(ctrl)
				mockStoredKey.EXPECT().GetMeta().Return(types.StoredKeyMeta{})
				args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
				mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())
				mockRepo.EXPECT().GetName().Return("repo_name")
				mockRepo.EXPECT().Head().Return("refs/heads/master", nil)
				mockRepo.EXPECT().HeadObject().Return(&object.Commit{Message: "This is a commit"}, nil)
				mockRepo.EXPECT().CreateEmptyCommit(args.Message, args.SigningKey).Return(nil)
				err := SignCommitCmd(cfg, mockRepo, args)
				Expect(err).To(BeNil())
			})
		})
	})
})
