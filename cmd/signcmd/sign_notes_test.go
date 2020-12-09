package signcmd

import (
	"fmt"
	"os"

	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/crypto/ed25519"
	types2 "github.com/make-os/kit/keystore/types"
	"github.com/make-os/kit/mocks"
	"github.com/make-os/kit/remote/server"
	remotetypes "github.com/make-os/kit/remote/types"
	"github.com/make-os/kit/rpc/types"
	"github.com/make-os/kit/testutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

func testGetNextNonce2(nonce string, err error) func(string, types.Client) (string, error) {
	return func(s string, rpcClient types.Client) (string, error) {
		return nonce, err
	}
}

var _ = Describe("SignNote", func() {
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

	Describe(".SignNoteCmd", func() {
		_ = key

		It("should return error when push key ID is not provided", func() {
			mockRepo.EXPECT().GetGitConfigOption(gomock.Any()).AnyTimes()
			args := &SignNoteArgs{}
			err := SignNoteCmd(cfg, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(ErrMissingPushKeyID))
		})

		It("should return error when failed to unlock the signing key", func() {
			mockRepo.EXPECT().GetGitConfigOption(gomock.Any()).Return(key.PushAddr().String()).AnyTimes()
			args := &SignNoteArgs{}
			args.KeyUnlocker = testPushKeyUnlocker(nil, fmt.Errorf("error"))
			err := SignNoteCmd(cfg, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to unlock push key: error"))
		})

		It("should attempt to get pusher key if signing key is a user address", func() {
			mockRepo.EXPECT().GetGitConfigOption(gomock.Any()).Return(key.Addr().String()).AnyTimes()
			args := &SignNoteArgs{Name: "note1"}
			refName := plumbing.ReferenceName("refs/notes/note1")
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.Addr().String())
			mockStoredKey.EXPECT().GetMeta().Return(types2.StoredKeyMeta{})
			mockRepo.EXPECT().Reference(refName, false).Return(nil, fmt.Errorf("error"))
			_ = SignNoteCmd(cfg, mockRepo, args)
		})

		It("should return error when unable to get note reference", func() {
			mockRepo.EXPECT().GetGitConfigOption(gomock.Any()).Return(key.Addr().String()).AnyTimes()
			args := &SignNoteArgs{Name: "note1"}
			refName := plumbing.ReferenceName("refs/notes/note1")
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			mockStoredKey.EXPECT().GetMeta().Return(types2.StoredKeyMeta{})
			mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())
			mockRepo.EXPECT().Reference(refName, false).Return(nil, fmt.Errorf("error"))
			err := SignNoteCmd(cfg, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		When("nonce is not set", func() {
			It("should attempt to get nonce and return error of failure", func() {
				mockRepo.EXPECT().GetGitConfigOption(gomock.Any()).Return(key.Addr().String()).AnyTimes()
				args := &SignNoteArgs{Name: "note1"}
				args.GetNextNonce = testGetNextNonce2("", fmt.Errorf("error"))
				refName := plumbing.ReferenceName("refs/notes/note1")
				mockStoredKey := mocks.NewMockStoredKey(ctrl)
				args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
				mockStoredKey.EXPECT().GetMeta().Return(types2.StoredKeyMeta{})
				mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())
				ref := plumbing.NewHashReference(refName, plumbing.NewHash("5cb1af69935120f4944a8cd515f008e12290de52"))
				mockRepo.EXPECT().Reference(refName, false).Return(ref, nil)
				err := SignNoteCmd(cfg, mockRepo, args)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to get next nonce: error"))
			})
		})

		It("should return error when unable to create and sign a push token", func() {
			mockRepo.EXPECT().GetGitConfigOption(gomock.Any()).Return(key.Addr().String()).AnyTimes()
			args := &SignNoteArgs{Name: "note1", Nonce: 1}
			refName := plumbing.ReferenceName("refs/notes/note1")
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			mockStoredKey.EXPECT().GetMeta().Return(types2.StoredKeyMeta{})
			mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())
			ref := plumbing.NewHashReference(refName, plumbing.NewHash("5cb1af69935120f4944a8cd515f008e12290de52"))
			mockRepo.EXPECT().Reference(refName, false).Return(ref, nil)
			args.CreateApplyPushTokenToRemote = func(targetRepo remotetypes.LocalRepo, args *server.MakeAndApplyPushTokenToRemoteArgs) error {
				return fmt.Errorf("error")
			}
			err := SignNoteCmd(cfg, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		It("should return nil on successful creation and signing of a push token", func() {
			mockRepo.EXPECT().GetGitConfigOption(gomock.Any()).Return(key.Addr().String()).AnyTimes()
			args := &SignNoteArgs{Name: "note1", Nonce: 1}
			refName := plumbing.ReferenceName("refs/notes/note1")
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			mockStoredKey.EXPECT().GetMeta().Return(types2.StoredKeyMeta{})
			mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())
			ref := plumbing.NewHashReference(refName, plumbing.NewHash("5cb1af69935120f4944a8cd515f008e12290de52"))
			mockRepo.EXPECT().Reference(refName, false).Return(ref, nil)
			args.CreateApplyPushTokenToRemote = func(targetRepo remotetypes.LocalRepo, args *server.MakeAndApplyPushTokenToRemoteArgs) error {
				return nil
			}
			err := SignNoteCmd(cfg, mockRepo, args)
			Expect(err).To(BeNil())
		})
	})
})
