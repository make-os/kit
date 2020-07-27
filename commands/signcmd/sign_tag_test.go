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
	types2 "github.com/themakeos/lobe/keystore/types"
	"github.com/themakeos/lobe/mocks"
	"github.com/themakeos/lobe/remote/types"
	"github.com/themakeos/lobe/testutil"
)

var _ = Describe("SignTag", func() {
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
			mockRepo.EXPECT().GetConfig("user.signingKey").Return("")
			err := SignTagCmd(cfg, []string{}, mockRepo, &SignTagArgs{})
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(ErrMissingPushKeyID))
		})

		It("should return error when failed to unlock the signing key", func() {
			mockRepo.EXPECT().GetConfig("user.signingKey").Return(key.PushAddr().String())
			args := &SignTagArgs{}
			args.KeyUnlocker = testPushKeyUnlocker(nil, fmt.Errorf("error"))
			err := SignTagCmd(cfg, []string{}, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to unlock push key: error"))
		})

		It("should attempt to get pusher key if signing key is a user address", func() {
			mockRepo.EXPECT().GetConfig("user.signingKey").Return(key.Addr().String())
			args := &SignTagArgs{}
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			mockStoredKey.EXPECT().GetMeta().Return(types2.StoredKeyMeta{})
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			mockStoredKey.EXPECT().GetUserAddress().Return(key.Addr().String())
			mockStoredKey.EXPECT().GetKey().Return(key)
			args.GetNextNonce = testGetNextNonce2("", fmt.Errorf("error"))
			SignTagCmd(cfg, []string{}, mockRepo, args)
		})

		It("should return error when unable to get next nonce of pusher account", func() {
			mockRepo.EXPECT().GetConfig("user.signingKey").Return(key.PushAddr().String())
			args := &SignTagArgs{}
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			mockStoredKey.EXPECT().GetMeta().Return(types2.StoredKeyMeta{})
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			mockStoredKey.EXPECT().GetUserAddress().Return(key.PushAddr().String())
			args.GetNextNonce = testGetNextNonce2("", fmt.Errorf("error"))
			err := SignTagCmd(cfg, []string{}, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		It("should return error when unable to set push token on remote URL", func() {
			mockRepo.EXPECT().GetConfig("user.signingKey").Return(key.PushAddr().String())
			args := &SignTagArgs{}
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			mockStoredKey.EXPECT().GetMeta().Return(types2.StoredKeyMeta{})
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			mockStoredKey.EXPECT().GetUserAddress().Return(key.PushAddr().String())
			args.GetNextNonce = testGetNextNonce2("1", nil)
			args.RemoteURLTokenUpdater = testRemoteURLTokenUpdater("", fmt.Errorf("error"))
			err := SignTagCmd(cfg, []string{}, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		It("should return error when unable to create tag", func() {
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			mockStoredKey.EXPECT().GetMeta().Return(types2.StoredKeyMeta{})
			args := &SignTagArgs{SigningKey: key.PushAddr().String()}
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			mockStoredKey.EXPECT().GetUserAddress().Return(key.PushAddr().String())
			args.GetNextNonce = testGetNextNonce2("1", nil)
			args.RemoteURLTokenUpdater = testRemoteURLTokenUpdater("", nil)
			mockRepo.EXPECT().GetName().Return("repo_name")
			mockRepo.EXPECT().CreateTagWithMsg(gomock.Any(), args.Message, args.SigningKey).Return(fmt.Errorf("error"))
			err := SignTagCmd(cfg, []string{}, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		It("should return no error when tag is created", func() {
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			mockStoredKey.EXPECT().GetMeta().Return(types2.StoredKeyMeta{})
			args := &SignTagArgs{SigningKey: key.PushAddr().String()}
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			mockStoredKey.EXPECT().GetUserAddress().Return(key.PushAddr().String())
			args.GetNextNonce = testGetNextNonce2("1", nil)
			args.RemoteURLTokenUpdater = testRemoteURLTokenUpdater("", nil)
			mockRepo.EXPECT().GetName().Return("repo_name")
			mockRepo.EXPECT().CreateTagWithMsg(gomock.Any(), args.Message, args.SigningKey).Return(nil)
			err := SignTagCmd(cfg, []string{}, mockRepo, args)
			Expect(err).To(BeNil())
		})

		When("args.SigningKey is a user address", func() {
			It("should pass user address to CreateTagWithMsg. Pass push key id to TxDetail object and GetNextNonce", func() {
				mockStoredKey := mocks.NewMockStoredKey(ctrl)
				mockStoredKey.EXPECT().GetMeta().Return(types2.StoredKeyMeta{})
				args := &SignTagArgs{SigningKey: key.Addr().String()}
				args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
				mockStoredKey.EXPECT().GetUserAddress().Return(key.Addr().String())
				mockStoredKey.EXPECT().GetKey().Return(key)
				args.GetNextNonce = func(address string, rpcClient client.Client, remoteClients []restclient.Client) (string, error) {
					Expect(address).To(Equal(key.PushAddr().String()))
					return "", nil
				}
				args.RemoteURLTokenUpdater = func(targetRepo types.LocalRepo, targetRemote string, txDetail *types.TxDetail, pushKey types2.StoredKey, reset bool) (string, error) {
					Expect(txDetail.PushKeyID).To(Equal(key.PushAddr().String()))
					return "", nil
				}
				mockRepo.EXPECT().GetName().Return("repo_name")
				mockRepo.EXPECT().CreateTagWithMsg(gomock.Any(), args.Message, args.SigningKey).Return(nil)
				err := SignTagCmd(cfg, []string{}, mockRepo, args)
				Expect(err).To(BeNil())
			})
		})

		It("should set args.PushKeyID to value of git flag --local-user", func() {
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			mockStoredKey.EXPECT().GetMeta().Return(types2.StoredKeyMeta{})
			args := &SignTagArgs{}
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			mockStoredKey.EXPECT().GetUserAddress().Return(key.PushAddr().String())
			args.GetNextNonce = testGetNextNonce2("1", nil)
			args.RemoteURLTokenUpdater = testRemoteURLTokenUpdater("", nil)
			mockRepo.EXPECT().GetName().Return("repo_name")
			mockRepo.EXPECT().CreateTagWithMsg(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			err := SignTagCmd(cfg, []string{"--local-user", key.PushAddr().String()}, mockRepo, args)
			Expect(err).To(BeNil())
			Expect(args.SigningKey).To(Equal(key.PushAddr().String()))
		})

		It("should set args.PushKeyID to value of git flag --message", func() {
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			mockStoredKey.EXPECT().GetMeta().Return(types2.StoredKeyMeta{})
			args := &SignTagArgs{SigningKey: key.PushAddr().String()}
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			mockStoredKey.EXPECT().GetUserAddress().Return(key.PushAddr().String())
			args.GetNextNonce = testGetNextNonce2("1", nil)
			args.RemoteURLTokenUpdater = testRemoteURLTokenUpdater("", nil)
			mockRepo.EXPECT().GetName().Return("repo_name")
			mockRepo.EXPECT().CreateTagWithMsg(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			msg := "A git message"
			err := SignTagCmd(cfg, []string{"--message", msg}, mockRepo, args)
			Expect(err).To(BeNil())
			Expect(args.Message).To(Equal(msg))
		})
	})
})
