package signcmd

import (
	"fmt"
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	restclient "github.com/themakeos/lobe/api/remote/client"
	"github.com/themakeos/lobe/api/rpc/client"
	"github.com/themakeos/lobe/config"
	"github.com/themakeos/lobe/crypto"
	"github.com/themakeos/lobe/keystore/types"
	"github.com/themakeos/lobe/mocks"
	remotetypes "github.com/themakeos/lobe/remote/types"
	"github.com/themakeos/lobe/testutil"
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
			mockRepo.EXPECT().GetConfig("user.signingKey").Return("")
			args := &SignNoteArgs{}
			err := SignNoteCmd(cfg, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(ErrMissingPushKeyID))
		})

		It("should return error when failed to unlock the signing key", func() {
			mockRepo.EXPECT().GetConfig("user.signingKey").Return(key.PushAddr().String())
			args := &SignNoteArgs{}
			args.KeyUnlocker = testPushKeyUnlocker(nil, fmt.Errorf("error"))
			err := SignNoteCmd(cfg, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to unlock push key: error"))
		})

		It("should attempt to get pusher key if signing key is a user address", func() {
			mockRepo.EXPECT().GetConfig("user.signingKey").Return(key.Addr().String())
			args := &SignNoteArgs{Name: "note1"}
			refname := plumbing.ReferenceName("refs/notes/note1")
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			mockStoredKey.EXPECT().GetAddress().Return(key.Addr().String())
			mockStoredKey.EXPECT().GetKey().Return(key)
			mockRepo.EXPECT().Reference(refname, true).Return(nil, fmt.Errorf("error"))
			SignNoteCmd(cfg, mockRepo, args)
		})

		It("should return error when note does not already exist", func() {
			mockRepo.EXPECT().GetConfig("user.signingKey").Return(key.PushAddr().String())
			args := &SignNoteArgs{Name: "note1"}
			refname := plumbing.ReferenceName("refs/notes/note1")
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			mockStoredKey.EXPECT().GetAddress().Return(key.PushAddr().String())
			mockRepo.EXPECT().Reference(refname, true).Return(nil, fmt.Errorf("error"))
			err := SignNoteCmd(cfg, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to get note reference: error"))
		})

		It("should return error when unable to get next nonce of pusher", func() {
			mockRepo.EXPECT().GetConfig("user.signingKey").Return(key.PushAddr().String())
			args := &SignNoteArgs{Name: "note1"}
			refname := plumbing.ReferenceName("refs/notes/note1")
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			mockStoredKey.EXPECT().GetAddress().Return(key.PushAddr().String())
			mockRepo.EXPECT().Reference(refname, true).Return(&plumbing.Reference{}, nil)
			args.GetNextNonce = testGetNextNonce2("", fmt.Errorf("error"))
			err := SignNoteCmd(cfg, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		It("should return error when unable to update remote URL with push token", func() {
			mockRepo.EXPECT().GetConfig("user.signingKey").Return(key.PushAddr().String())
			args := &SignNoteArgs{Name: "note1"}
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			mockStoredKey.EXPECT().GetAddress().Return(key.PushAddr().String())
			refname := plumbing.ReferenceName("refs/notes/note1")
			hash := plumbing.NewHash("25560419583cd1eb46e322528597f94404e0b7be")
			mockRepo.EXPECT().Reference(refname, true).Return(plumbing.NewHashReference(refname, hash), nil)
			args.GetNextNonce = testGetNextNonce2("1", nil)
			args.RemoteURLTokenUpdater = func(targetRepo remotetypes.LocalRepo, targetRemote string, txDetail *remotetypes.TxDetail, pushKey types.StoredKey, reset bool) (string, error) {
				return "", fmt.Errorf("error")
			}
			err := SignNoteCmd(cfg, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		It("should return no error when successful", func() {
			mockRepo.EXPECT().GetConfig("user.signingKey").Return(key.PushAddr().String())
			args := &SignNoteArgs{Name: "note1"}
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			mockStoredKey.EXPECT().GetAddress().Return(key.PushAddr().String())
			refname := plumbing.ReferenceName("refs/notes/note1")
			hash := plumbing.NewHash("25560419583cd1eb46e322528597f94404e0b7be")
			mockRepo.EXPECT().Reference(refname, true).Return(plumbing.NewHashReference(refname, hash), nil)
			args.GetNextNonce = testGetNextNonce2("1", nil)
			args.RemoteURLTokenUpdater = func(targetRepo remotetypes.LocalRepo, targetRemote string, txDetail *remotetypes.TxDetail, pushKey types.StoredKey, reset bool) (string, error) {
				return "", nil
			}
			err := SignNoteCmd(cfg, mockRepo, args)
			Expect(err).To(BeNil())
		})

		When("args.SigningKey is a user address", func() {
			It("should pass push key id to TxDetail object and GetNextNonce", func() {
				mockRepo.EXPECT().GetConfig("user.signingKey").Return(key.Addr().String())
				args := &SignNoteArgs{Name: "note1"}
				mockStoredKey := mocks.NewMockStoredKey(ctrl)
				args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
				mockStoredKey.EXPECT().GetAddress().Return(key.Addr().String())
				mockStoredKey.EXPECT().GetKey().Return(key)
				refname := plumbing.ReferenceName("refs/notes/note1")
				hash := plumbing.NewHash("25560419583cd1eb46e322528597f94404e0b7be")
				mockRepo.EXPECT().Reference(refname, true).Return(plumbing.NewHashReference(refname, hash), nil)
				args.GetNextNonce = func(address string, rpcClient client.Client, remoteClients []restclient.Client) (string, error) {
					Expect(address).To(Equal(key.PushAddr().String()))
					return "", nil
				}
				args.RemoteURLTokenUpdater = func(targetRepo remotetypes.LocalRepo, targetRemote string, txDetail *remotetypes.TxDetail, pushKey types.StoredKey, reset bool) (string, error) {
					Expect(txDetail.PushKeyID).To(Equal(key.PushAddr().String()))
					return "", nil
				}
				err := SignNoteCmd(cfg, mockRepo, args)
				Expect(err).To(BeNil())
			})
		})
	})
})
