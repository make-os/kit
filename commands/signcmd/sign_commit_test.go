package signcmd

import (
	"fmt"
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	restclient "gitlab.com/makeos/lobe/api/remote/client"
	"gitlab.com/makeos/lobe/api/rpc/client"
	"gitlab.com/makeos/lobe/commands/common"
	"gitlab.com/makeos/lobe/config"
	"gitlab.com/makeos/lobe/crypto"
	"gitlab.com/makeos/lobe/keystore/types"
	"gitlab.com/makeos/lobe/mocks"
	plumbing2 "gitlab.com/makeos/lobe/remote/plumbing"
	remotetypes "gitlab.com/makeos/lobe/remote/types"
	"gitlab.com/makeos/lobe/testutil"
	"gopkg.in/src-d/go-git.v4/plumbing"
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

func testRemoteURLTokenUpdater(token string, err error) func(targetRepo remotetypes.LocalRepo, targetRemote string,
	txDetail *remotetypes.TxDetail, pushKey types.StoredKey, reset bool) (string, error) {
	return func(targetRepo remotetypes.LocalRepo, targetRemote string, txDetail *remotetypes.TxDetail, pushKey types.StoredKey, reset bool) (string, error) {
		return token, err
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

	Describe(".SignCommitCmd", func() {
		It("should return error when push key ID is not provided", func() {
			mockRepo.EXPECT().GetConfig("user.signingKey").Return("")
			args := &SignCommitArgs{}
			err := SignCommitCmd(cfg, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(ErrMissingPushKeyID))
		})

		It("should return error when unable to find and unlock push key", func() {
			mockRepo.EXPECT().GetConfig("user.signingKey").Return(key.PushAddr().String())
			args := &SignCommitArgs{}
			args.KeyUnlocker = testPushKeyUnlocker(nil, fmt.Errorf("error"))
			err := SignCommitCmd(cfg, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("failed to unlock the signing key: error"))
		})

		It("should return error when mergeID is set but invalid", func() {
			mockRepo.EXPECT().GetConfig("user.signingKey").Return(key.PushAddr().String())
			args := &SignCommitArgs{MergeID: "abc123_invalid"}
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			err := SignCommitCmd(cfg, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("merge id must be numeric"))
			args.MergeID = "12345678910"
			err = SignCommitCmd(cfg, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("merge proposal id exceeded 8 bytes limit"))
		})

		It("should return error when unable to get next nonce", func() {
			mockRepo.EXPECT().GetConfig("user.signingKey").Return(key.PushAddr().String())
			args := &SignCommitArgs{GetNextNonce: func(pushKeyID string, rpcClient client.Client, remoteClients []restclient.Client) (string, error) {
				return "", fmt.Errorf("error")
			},
			}
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			err := SignCommitCmd(cfg, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("error"))
		})

		It("should return error when unable to get local repo HEAD", func() {
			mockRepo.EXPECT().GetConfig("user.signingKey").Return(key.PushAddr().String())
			args := &SignCommitArgs{GetNextNonce: testGetNextNonce}
			mockRepo.EXPECT().Head().Return("", fmt.Errorf("error"))
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			err := SignCommitCmd(cfg, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("failed to get HEAD"))
		})

		When("args.Branch is set", func() {
			It("should return error when unable to checkout branch", func() {
				mockRepo.EXPECT().GetConfig("user.signingKey").Return(key.PushAddr().String())
				args := &SignCommitArgs{GetNextNonce: testGetNextNonce, Branch: "refs/heads/dev"}
				mockStoredKey := mocks.NewMockStoredKey(ctrl)
				args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
				mockRepo.EXPECT().Head().Return("refs/heads/master", nil)
				mockRepo.EXPECT().Checkout("dev", false, args.ForceCheckout).Return(fmt.Errorf("error"))
				err := SignCommitCmd(cfg, mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to checkout branch (refs/heads/dev): error"))
			})

			It("should return error when unable to checkout branch", func() {
				mockRepo.EXPECT().GetConfig("user.signingKey").Return(key.PushAddr().String())
				args := &SignCommitArgs{GetNextNonce: testGetNextNonce, Branch: "refs/heads/dev"}
				mockStoredKey := mocks.NewMockStoredKey(ctrl)
				args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
				mockRepo.EXPECT().Head().Return("refs/heads/master", nil)
				mockRepo.EXPECT().Checkout("dev", false, args.ForceCheckout).Return(fmt.Errorf("error"))
				err := SignCommitCmd(cfg, mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to checkout branch (refs/heads/dev): error"))
			})
		})

		It("should return error when unable to create and set push tokens to remote URLs", func() {
			args := &SignCommitArgs{Fee: "1", PushKeyID: key.PushAddr().String(), GetNextNonce: testGetNextNonce}
			args.RemoteURLTokenUpdater = testRemoteURLTokenUpdater("", fmt.Errorf("error"))
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			mockRepo.EXPECT().Head().Return("refs/heads/master", nil)
			err := SignCommitCmd(cfg, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("error"))
		})

		When("previous commit amendment is not required (AmendCommit=false", func() {
			It("should attempt to create a new commit and return error on failure", func() {
				args := &SignCommitArgs{Fee: "1", PushKeyID: key.PushAddr().String(), Message: "some message", GetNextNonce: testGetNextNonce, AmendCommit: false}
				args.RemoteURLTokenUpdater = testRemoteURLTokenUpdater("", nil)
				mockRepo.EXPECT().GetName().Return("repo_name")
				mockStoredKey := mocks.NewMockStoredKey(ctrl)
				args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
				mockRepo.EXPECT().Head().Return("refs/heads/master", nil)
				mockRepo.EXPECT().CreateSignedEmptyCommit(args.Message, args.PushKeyID).Return(fmt.Errorf("error"))
				err := SignCommitCmd(cfg, mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("error"))
			})

			It("should attempt to create a new commit and return nil on success", func() {
				args := &SignCommitArgs{Fee: "1", PushKeyID: key.PushAddr().String(), Message: "some message", GetNextNonce: testGetNextNonce, AmendCommit: false}
				args.RemoteURLTokenUpdater = testRemoteURLTokenUpdater("", nil)
				mockRepo.EXPECT().GetName().Return("repo_name")
				mockStoredKey := mocks.NewMockStoredKey(ctrl)
				args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
				mockRepo.EXPECT().Head().Return("refs/heads/master", nil)
				mockRepo.EXPECT().CreateSignedEmptyCommit(args.Message, args.PushKeyID).Return(nil)
				err := SignCommitCmd(cfg, mockRepo, args)
				Expect(err).To(BeNil())
			})
		})

		When("args.Head is set to 'refs/heads/some_branch'", func() {
			It("the generated tx detail should set Reference to 'refs/heads/some_branch'", func() {
				args := &SignCommitArgs{Fee: "1", PushKeyID: key.PushAddr().String(), Message: "some message", GetNextNonce: testGetNextNonce, AmendCommit: false, Head: "refs/heads/some_branch"}
				args.RemoteURLTokenUpdater = func(targetRepo remotetypes.LocalRepo, targetRemote string, txDetail *remotetypes.TxDetail, pushKey types.StoredKey, reset bool) (string, error) {
					Expect(txDetail.Reference).To(Equal("refs/heads/some_branch"))
					return "", nil
				}

				mockRepo.EXPECT().GetName().Return("repo_name")
				mockStoredKey := mocks.NewMockStoredKey(ctrl)
				args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
				mockRepo.EXPECT().Head().Return("refs/heads/master", nil)
				mockRepo.EXPECT().CreateSignedEmptyCommit(args.Message, args.PushKeyID).Return(nil)
				err := SignCommitCmd(cfg, mockRepo, args)
				Expect(err).To(BeNil())
			})
		})

		When("amend of previous commit is required (AmendCommit=true)", func() {
			It("should return error when unable to get recent commit hash", func() {
				args := &SignCommitArgs{Fee: "1", PushKeyID: key.PushAddr().String(), Message: "some message", GetNextNonce: testGetNextNonce, AmendCommit: true}
				args.RemoteURLTokenUpdater = testRemoteURLTokenUpdater("", nil)

				mockRepo.EXPECT().GetName().Return("repo_name")
				mockStoredKey := mocks.NewMockStoredKey(ctrl)
				args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
				mockRepo.EXPECT().Head().Return("refs/heads/master", nil)
				mockRepo.EXPECT().GetRecentCommitHash().Return("", fmt.Errorf("error"))
				err := SignCommitCmd(cfg, mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("error"))
			})

			It("should return error when unable to get recent commit hash", func() {
				args := &SignCommitArgs{Fee: "1", PushKeyID: key.PushAddr().String(), Message: "some message", GetNextNonce: testGetNextNonce, AmendCommit: true}
				args.RemoteURLTokenUpdater = testRemoteURLTokenUpdater("", nil)
				mockStoredKey := mocks.NewMockStoredKey(ctrl)
				args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
				mockRepo.EXPECT().GetName().Return("repo_name")
				mockRepo.EXPECT().Head().Return("refs/heads/master", nil)
				mockRepo.EXPECT().GetRecentCommitHash().Return("", fmt.Errorf("error"))
				err := SignCommitCmd(cfg, mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("error"))
			})

			It("should return error when unable to get recent commit due to ErrNoCommits", func() {
				args := &SignCommitArgs{Fee: "1", PushKeyID: key.PushAddr().String(), Message: "some message", GetNextNonce: testGetNextNonce, AmendCommit: true}
				args.RemoteURLTokenUpdater = testRemoteURLTokenUpdater("", nil)
				mockStoredKey := mocks.NewMockStoredKey(ctrl)
				args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
				mockRepo.EXPECT().GetName().Return("repo_name")
				mockRepo.EXPECT().Head().Return("refs/heads/master", nil)
				mockRepo.EXPECT().GetRecentCommitHash().Return("", plumbing2.ErrNoCommits)
				err := SignCommitCmd(cfg, mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("no commits have been created yet"))
			})

			It("should use previous commit message if args.Message is not set", func() {
				args := &SignCommitArgs{Fee: "1", PushKeyID: key.PushAddr().String(), Message: "", GetNextNonce: testGetNextNonce, AmendCommit: true}
				args.RemoteURLTokenUpdater = testRemoteURLTokenUpdater("", nil)
				mockStoredKey := mocks.NewMockStoredKey(ctrl)
				args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
				mockRepo.EXPECT().GetName().Return("repo_name")
				mockRepo.EXPECT().Head().Return("refs/heads/master", nil)
				mockRepo.EXPECT().GetRecentCommitHash().Return("8975220bda0eb48354c5868f8a1b310758eb4591", nil)
				recentCommit := &object.Commit{Message: "This is a commit"}
				mockRepo.EXPECT().CommitObject(plumbing.NewHash("8975220bda0eb48354c5868f8a1b310758eb4591")).Return(recentCommit, nil)
				mockRepo.EXPECT().UpdateRecentCommitMsg(recentCommit.Message, args.PushKeyID).Return(nil)
				err := SignCommitCmd(cfg, mockRepo, args)
				Expect(err).To(BeNil())
				Expect(args.Message).To(Equal(recentCommit.Message))
			})

			It("should return error when unable to get recent commit object", func() {
				args := &SignCommitArgs{Fee: "1", PushKeyID: key.PushAddr().String(), Message: "", GetNextNonce: testGetNextNonce, AmendCommit: true}
				args.RemoteURLTokenUpdater = testRemoteURLTokenUpdater("", nil)
				mockStoredKey := mocks.NewMockStoredKey(ctrl)
				args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
				mockRepo.EXPECT().GetName().Return("repo_name")
				mockRepo.EXPECT().Head().Return("refs/heads/master", nil)
				mockRepo.EXPECT().GetRecentCommitHash().Return("8975220bda0eb48354c5868f8a1b310758eb4591", nil)
				mockRepo.EXPECT().CommitObject(plumbing.NewHash("8975220bda0eb48354c5868f8a1b310758eb4591")).Return(nil, fmt.Errorf("error getting commit"))
				err := SignCommitCmd(cfg, mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("error getting commit"))
			})

			It("should return error when unable to update recent commit", func() {
				args := &SignCommitArgs{Fee: "1", PushKeyID: key.PushAddr().String(), Message: "", GetNextNonce: testGetNextNonce, AmendCommit: true}
				args.RemoteURLTokenUpdater = testRemoteURLTokenUpdater("", nil)
				mockStoredKey := mocks.NewMockStoredKey(ctrl)
				args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
				mockRepo.EXPECT().GetName().Return("repo_name")
				mockRepo.EXPECT().Head().Return("refs/heads/master", nil)
				mockRepo.EXPECT().GetRecentCommitHash().Return("8975220bda0eb48354c5868f8a1b310758eb4591", nil)
				recentCommit := &object.Commit{Message: "This is a commit"}
				mockRepo.EXPECT().CommitObject(plumbing.NewHash("8975220bda0eb48354c5868f8a1b310758eb4591")).Return(recentCommit, nil)
				mockRepo.EXPECT().UpdateRecentCommitMsg(recentCommit.Message, args.PushKeyID).Return(fmt.Errorf("error"))
				err := SignCommitCmd(cfg, mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("error"))
			})
		})
	})
})
