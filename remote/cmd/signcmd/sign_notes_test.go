package signcmd

import (
	"fmt"
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	restclient "gitlab.com/makeos/mosdef/api/rest/client"
	"gitlab.com/makeos/mosdef/api/rpc/client"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/mocks"
	"gitlab.com/makeos/mosdef/testutil"
	"gitlab.com/makeos/mosdef/types/core"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

func testGetNextNonce2(nonce string, err error) func(string, *client.RPCClient, []restclient.RestClient) (string, error) {
	return func(s string, rpcClient *client.RPCClient, clients []restclient.RestClient) (string, error) {
		return nonce, err
	}
}

var _ = Describe("SignNote", func() {
	var err error
	var cfg *config.AppConfig
	var ctrl *gomock.Controller
	var mockRepo *mocks.MockBareRepo
	var key *crypto.Key

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		ctrl = gomock.NewController(GinkgoT())
		mockRepo = mocks.NewMockBareRepo(ctrl)
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

		It("should return error when unable to unlock push key", func() {
			mockRepo.EXPECT().GetConfig("user.signingKey").Return(key.PushAddr().String())
			args := &SignNoteArgs{}
			args.PushKeyUnlocker = testPushKeyUnlocker(nil, fmt.Errorf("error"))
			err := SignNoteCmd(cfg, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to unlock push key: error"))
		})

		It("should return error when note does not already exist", func() {
			mockRepo.EXPECT().GetConfig("user.signingKey").Return(key.PushAddr().String())
			args := &SignNoteArgs{Name: "note1"}
			refname := plumbing.ReferenceName("refs/notes/note1")
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			args.PushKeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
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
			args.PushKeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
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
			args.PushKeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			refname := plumbing.ReferenceName("refs/notes/note1")
			hash := plumbing.NewHash("25560419583cd1eb46e322528597f94404e0b7be")
			mockRepo.EXPECT().Reference(refname, true).Return(plumbing.NewHashReference(refname, hash), nil)
			args.GetNextNonce = testGetNextNonce2("1", nil)
			args.RemoteURLTokenUpdater = func(targetRepo core.BareRepo, targetRemote string, txDetail *core.TxDetail, pushKey core.StoredKey, reset bool) (string, error) {
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
			args.PushKeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			refname := plumbing.ReferenceName("refs/notes/note1")
			hash := plumbing.NewHash("25560419583cd1eb46e322528597f94404e0b7be")
			mockRepo.EXPECT().Reference(refname, true).Return(plumbing.NewHashReference(refname, hash), nil)
			args.GetNextNonce = testGetNextNonce2("1", nil)
			args.RemoteURLTokenUpdater = func(targetRepo core.BareRepo, targetRemote string, txDetail *core.TxDetail, pushKey core.StoredKey, reset bool) (string, error) {
				return "", nil
			}
			err := SignNoteCmd(cfg, mockRepo, args)
			Expect(err).To(BeNil())
		})
	})
})
