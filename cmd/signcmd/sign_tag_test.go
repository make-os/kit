package signcmd

import (
	"fmt"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/cmd/signcmd/types"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/crypto/ed25519"
	"github.com/make-os/kit/mocks"
	"github.com/make-os/kit/remote/server"
	remotetypes "github.com/make-os/kit/remote/types"
	"github.com/make-os/kit/testutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SignTag", func() {
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
		_ = mockRepo
		_ = key
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".SignTagCmd", func() {
		It("should return error when unable to get push key", func() {
			mockRepo.EXPECT().GetGitConfigOption(gomock.Any()).AnyTimes()

			err := SignTagCmd(cfg, []string{}, mockRepo, &types.SignTagArgs{})
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(ErrMissingPushKeyID))
		})

		It("should return error when failed to unlock the signing key", func() {
			mockRepo.EXPECT().GetGitConfigOption(gomock.Any()).DoAndReturn(mockGetConfig(map[string]string{
				"user.signingKey": key.PushAddr().String(),
			})).AnyTimes()

			args := &types.SignTagArgs{}
			args.KeyUnlocker = testPushKeyUnlocker(nil, fmt.Errorf("error"))

			err := SignTagCmd(cfg, []string{}, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to unlock push key: error"))
		})

		It("should return error when unable to get tag", func() {
			mockRepo.EXPECT().GetGitConfigOption(gomock.Any()).DoAndReturn(mockGetConfig(map[string]string{
				"user.signingKey": key.PushAddr().String(),
			})).AnyTimes()

			args := &types.SignTagArgs{}
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())
			mockRepo.EXPECT().Tag("tag1").Return(nil, git.ErrTagNotFound)

			err := SignTagCmd(cfg, []string{"tag1"}, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(git.ErrTagNotFound))
		})

		When("nonce is not set", func() {
			It("should attempt to get nonce and return error if it failed", func() {
				mockRepo.EXPECT().GetGitConfigOption(gomock.Any()).DoAndReturn(mockGetConfig(map[string]string{
					"user.signingKey": key.PushAddr().String(),
				})).AnyTimes()
				args := &types.SignTagArgs{}
				mockStoredKey := mocks.NewMockStoredKey(ctrl)
				args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
				mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())
				refName := plumbing.ReferenceName("refs/tags/tag1")
				ref := plumbing.NewHashReference(refName, plumbing.NewHash("5cb1af69935120f4944a8cd515f008e12290de52"))
				mockRepo.EXPECT().Tag("tag1").Return(ref, nil)
				args.GetNextNonce = testGetNextNonce2("", fmt.Errorf("error"))
				err := SignTagCmd(cfg, []string{"tag1"}, mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to get next nonce: error"))
			})
		})

		It("should return error when unable to create and sign a push token", func() {
			mockRepo.EXPECT().GetGitConfigOption(gomock.Any()).DoAndReturn(mockGetConfig(map[string]string{
				"user.signingKey": key.PushAddr().String(),
			})).AnyTimes()
			args := &types.SignTagArgs{Nonce: 1}
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())
			refName := plumbing.ReferenceName("refs/tags/tag1")
			ref := plumbing.NewHashReference(refName, plumbing.NewHash("5cb1af69935120f4944a8cd515f008e12290de52"))
			mockRepo.EXPECT().Tag("tag1").Return(ref, nil)
			args.CreateApplyPushTokenToRemote = func(targetRepo remotetypes.LocalRepo, args *server.MakeAndApplyPushTokenToRemoteArgs) error {
				return fmt.Errorf("error")
			}
			err := SignTagCmd(cfg, []string{"tag1"}, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		It("should return nil when able to create and sign a push token", func() {
			mockRepo.EXPECT().GetGitConfigOption(gomock.Any()).DoAndReturn(mockGetConfig(map[string]string{
				"user.signingKey": key.PushAddr().String(),
			})).AnyTimes()
			args := &types.SignTagArgs{Nonce: 1}
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())
			refName := plumbing.ReferenceName("refs/tags/tag1")
			ref := plumbing.NewHashReference(refName, plumbing.NewHash("5cb1af69935120f4944a8cd515f008e12290de52"))
			mockRepo.EXPECT().Tag("tag1").Return(ref, nil)
			args.CreateApplyPushTokenToRemote = func(targetRepo remotetypes.LocalRepo, args *server.MakeAndApplyPushTokenToRemoteArgs) error {
				return nil
			}
			err := SignTagCmd(cfg, []string{"tag1"}, mockRepo, args)
			Expect(err).To(BeNil())
		})
	})
})
