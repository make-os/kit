package signcmd

import (
	"fmt"
	"os"

	"github.com/golang/mock/gomock"
	restclient "github.com/make-os/lobe/api/remote/client"
	"github.com/make-os/lobe/api/rpc/client"
	"github.com/make-os/lobe/config"
	"github.com/make-os/lobe/crypto"
	types2 "github.com/make-os/lobe/keystore/types"
	"github.com/make-os/lobe/mocks"
	"github.com/make-os/lobe/remote/server"
	remotetypes "github.com/make-os/lobe/remote/types"
	"github.com/make-os/lobe/testutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

func testGetNextNonce2(nonce string, err error) func(string, client.Client, []restclient.Client) (string, error) {
	return func(s string, rpcClient client.Client, clients []restclient.Client) (string, error) {
		return nonce, err
	}
}

var _ = Describe("SignNote", func() {
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

	Describe(".SignNoteCmd", func() {
		It("should return error when push key ID is not provided", func() {
			mockRepo.EXPECT().GetConfig(gomock.Any()).AnyTimes()
			args := &SignNoteArgs{}
			err := SignNoteCmd(cfg, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(ErrMissingPushKeyID))
		})

		It("should return error when failed to unlock the signing key", func() {
			mockRepo.EXPECT().GetConfig(gomock.Any()).Return(key.PushAddr().String()).AnyTimes()
			args := &SignNoteArgs{}
			args.KeyUnlocker = testPushKeyUnlocker(nil, fmt.Errorf("error"))
			err := SignNoteCmd(cfg, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to unlock push key: error"))
		})

		It("should attempt to get pusher key if signing key is a user address", func() {
			mockRepo.EXPECT().GetConfig(gomock.Any()).Return(key.Addr().String()).AnyTimes()
			args := &SignNoteArgs{Name: "note1"}
			refname := plumbing.ReferenceName("refs/notes/note1")
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.Addr().String())
			mockStoredKey.EXPECT().GetMeta().Return(types2.StoredKeyMeta{})
			mockRepo.EXPECT().Reference(refname, true).Return(nil, fmt.Errorf("error"))
			SignNoteCmd(cfg, mockRepo, args)
		})

		It("should return error when note does not already exist", func() {
			mockRepo.EXPECT().GetConfig(gomock.Any()).Return(key.Addr().String()).AnyTimes()
			args := &SignNoteArgs{Name: "note1"}
			refname := plumbing.ReferenceName("refs/notes/note1")
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			mockStoredKey.EXPECT().GetMeta().Return(types2.StoredKeyMeta{})
			mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())
			mockRepo.EXPECT().Reference(refname, true).Return(nil, fmt.Errorf("error"))
			err := SignNoteCmd(cfg, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to get note reference: error"))
		})

		It("should return error when unable to get next nonce of pusher", func() {
			mockRepo.EXPECT().GetConfig(gomock.Any()).Return(key.Addr().String()).AnyTimes()
			args := &SignNoteArgs{Name: "note1"}
			refname := plumbing.ReferenceName("refs/notes/note1")
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			mockStoredKey.EXPECT().GetMeta().Return(types2.StoredKeyMeta{})
			mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())
			mockRepo.EXPECT().Reference(refname, true).Return(&plumbing.Reference{}, nil)
			args.GetNextNonce = testGetNextNonce2("", fmt.Errorf("error"))
			err := SignNoteCmd(cfg, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to get next nonce: error"))
		})

		It("should return error when unable to update remote URL with push token", func() {
			mockRepo.EXPECT().GetConfig(gomock.Any()).Return(key.Addr().String()).AnyTimes()
			args := &SignNoteArgs{Name: "note1"}
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			mockStoredKey.EXPECT().GetMeta().Return(types2.StoredKeyMeta{})
			mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())
			refname := plumbing.ReferenceName("refs/notes/note1")
			hash := plumbing.NewHash("25560419583cd1eb46e322528597f94404e0b7be")
			mockRepo.EXPECT().Reference(refname, true).Return(plumbing.NewHashReference(refname, hash), nil)
			args.GetNextNonce = testGetNextNonce2("1", nil)
			args.SetRemotePushToken = func(targetRepo remotetypes.LocalRepo, args *server.SetRemotePushTokenArgs) (string, error) {
				return "", fmt.Errorf("error")
			}
			err := SignNoteCmd(cfg, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		It("should return no error when successful", func() {
			mockRepo.EXPECT().GetConfig(gomock.Any()).Return(key.Addr().String()).AnyTimes()
			args := &SignNoteArgs{Name: "note1"}
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			mockStoredKey.EXPECT().GetMeta().Return(types2.StoredKeyMeta{})
			mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())
			refname := plumbing.ReferenceName("refs/notes/note1")
			hash := plumbing.NewHash("25560419583cd1eb46e322528597f94404e0b7be")
			mockRepo.EXPECT().Reference(refname, true).Return(plumbing.NewHashReference(refname, hash), nil)
			args.GetNextNonce = testGetNextNonce2("1", nil)
			args.SetRemotePushToken = func(targetRepo remotetypes.LocalRepo, args *server.SetRemotePushTokenArgs) (string, error) {
				return "", nil
			}
			err := SignNoteCmd(cfg, mockRepo, args)
			Expect(err).To(BeNil())
		})

		When("args.SigningKey is a user address", func() {
			It("should pass push key id to TxDetail object and GetNextNonce", func() {
				mockRepo.EXPECT().GetConfig(gomock.Any()).Return(key.Addr().String()).AnyTimes()
				args := &SignNoteArgs{Name: "note1", SigningKey: key.Addr().String()}
				mockStoredKey := mocks.NewMockStoredKey(ctrl)
				args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
				mockStoredKey.EXPECT().GetMeta().Return(types2.StoredKeyMeta{})
				mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())
				refname := plumbing.ReferenceName("refs/notes/note1")
				hash := plumbing.NewHash("25560419583cd1eb46e322528597f94404e0b7be")
				mockRepo.EXPECT().Reference(refname, true).Return(plumbing.NewHashReference(refname, hash), nil)
				args.GetNextNonce = func(address string, rpcClient client.Client, remoteClients []restclient.Client) (string, error) {
					Expect(address).To(Equal(key.PushAddr().String()))
					return "", nil
				}
				args.SetRemotePushToken = func(targetRepo remotetypes.LocalRepo, args *server.SetRemotePushTokenArgs) (string, error) {
					Expect(args.TxDetail.PushKeyID).To(Equal(key.PushAddr().String()))
					return "", nil
				}
				err := SignNoteCmd(cfg, mockRepo, args)
				Expect(err).To(BeNil())
			})
		})
	})
})
