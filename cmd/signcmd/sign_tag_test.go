package signcmd

import (
	"encoding/pem"
	"fmt"
	"os"

	"github.com/golang/mock/gomock"
	"github.com/make-os/lobe/config"
	"github.com/make-os/lobe/crypto"
	types2 "github.com/make-os/lobe/keystore/types"
	"github.com/make-os/lobe/mocks"
	"github.com/make-os/lobe/remote/server"
	"github.com/make-os/lobe/remote/types"
	types3 "github.com/make-os/lobe/rpc/types"
	"github.com/make-os/lobe/testutil"
	"github.com/make-os/lobe/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
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
			mockRepo.EXPECT().GetConfig(gomock.Any()).AnyTimes()

			err := SignTagCmd(cfg, []string{}, mockRepo, &SignTagArgs{})
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(ErrMissingPushKeyID))
		})

		It("should return error when failed to unlock the signing key", func() {
			mockRepo.EXPECT().GetConfig(gomock.Any()).DoAndReturn(mockGetConfig(map[string]string{
				"user.signingKey": key.PushAddr().String(),
			})).AnyTimes()

			args := &SignTagArgs{Force: true}
			args.KeyUnlocker = testPushKeyUnlocker(nil, fmt.Errorf("error"))

			err := SignTagCmd(cfg, []string{}, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to unlock push key: error"))
		})

		It("should return error when unable to get tag", func() {
			mockRepo.EXPECT().GetConfig(gomock.Any()).DoAndReturn(mockGetConfig(map[string]string{
				"user.signingKey": key.PushAddr().String(),
			})).AnyTimes()

			args := &SignTagArgs{}
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			mockStoredKey.EXPECT().GetMeta().Return(types2.StoredKeyMeta{})
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())
			mockRepo.EXPECT().Tag("tag1").Return(nil, fmt.Errorf("error"))

			err := SignTagCmd(cfg, []string{"tag1"}, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		It("should return error when unable to get existing tag object", func() {
			mockRepo.EXPECT().GetConfig(gomock.Any()).DoAndReturn(mockGetConfig(map[string]string{
				"user.signingKey": key.PushAddr().String(),
			})).AnyTimes()

			args := &SignTagArgs{}
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			mockStoredKey.EXPECT().GetMeta().Return(types2.StoredKeyMeta{})
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())

			ref := plumbing.NewReferenceFromStrings("", util.RandString(40))
			mockRepo.EXPECT().Tag("tag1").Return(ref, nil)
			mockRepo.EXPECT().TagObject(ref.Hash()).Return(nil, fmt.Errorf("error"))

			err := SignTagCmd(cfg, []string{"tag1"}, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		It("should use existing tag's message if none was provided and return error if unable to get nonce", func() {
			mockRepo.EXPECT().GetConfig(gomock.Any()).DoAndReturn(mockGetConfig(map[string]string{
				"user.signingKey": key.PushAddr().String(),
			})).AnyTimes()

			args := &SignTagArgs{}
			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			mockStoredKey.EXPECT().GetMeta().Return(types2.StoredKeyMeta{})
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())

			ref := plumbing.NewReferenceFromStrings("", util.RandString(40))
			mockRepo.EXPECT().Tag("tag1").Return(ref, nil)
			mockRepo.EXPECT().TagObject(ref.Hash()).Return(&object.Tag{Message: "tag1 message"}, nil)

			args.GetNextNonce = testGetNextNonce2("1", fmt.Errorf("error getting nonce"))

			err := SignTagCmd(cfg, []string{"tag1"}, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to get next nonce: error getting nonce"))
			Expect(args.Message).To(Equal("tag1 message"))
		})

		It("should return error when unable to create tag", func() {
			mockRepo.EXPECT().GetConfig(gomock.Any()).DoAndReturn(mockGetConfig(map[string]string{
				"user.signingKey": key.PushAddr().String(),
			})).AnyTimes()

			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			mockStoredKey.EXPECT().GetMeta().Return(types2.StoredKeyMeta{})
			args := &SignTagArgs{SigningKey: key.PushAddr().String()}
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())

			ref := plumbing.NewReferenceFromStrings("", util.RandString(40))
			mockRepo.EXPECT().Tag("tag1").Return(ref, nil)
			tag := &object.Tag{Message: "tag1 message"}
			mockRepo.EXPECT().TagObject(ref.Hash()).Return(tag, nil)
			mockRepo.EXPECT().GetName().Return("repo1")

			args.GetNextNonce = testGetNextNonce2("1", nil)
			args.SetRemotePushToken = testSetRemotePushToken("", nil)
			mockRepo.EXPECT().CreateTagWithMsg(gomock.Any(), tag.Message, args.SigningKey).Return(fmt.Errorf("error"))

			err := SignTagCmd(cfg, []string{"tag1"}, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		It("should return no error when tag is created", func() {
			mockRepo.EXPECT().GetConfig(gomock.Any()).DoAndReturn(mockGetConfig(map[string]string{
				"user.signingKey": key.PushAddr().String(),
			})).AnyTimes()

			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			mockStoredKey.EXPECT().GetMeta().Return(types2.StoredKeyMeta{})
			args := &SignTagArgs{SigningKey: key.PushAddr().String()}
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())

			ref := plumbing.NewReferenceFromStrings("", util.RandString(40))
			mockRepo.EXPECT().Tag("tag1").Return(ref, nil)
			tag := &object.Tag{Message: "tag1 message"}
			mockRepo.EXPECT().TagObject(ref.Hash()).Return(tag, nil)

			mockRepo.EXPECT().GetName().Return("repo_name")
			mockRepo.EXPECT().CreateTagWithMsg(gomock.Any(), tag.Message, args.SigningKey).Return(nil)
			mockRepo.EXPECT().Tag("tag1").Return(ref, nil)
			args.GetNextNonce = testGetNextNonce2("1", nil)
			args.SetRemotePushToken = testSetRemotePushToken("", nil)

			err := SignTagCmd(cfg, []string{"tag1"}, mockRepo, args)
			Expect(err).To(BeNil())
		})

		It("should return error when unable to create and set push token", func() {
			mockRepo.EXPECT().GetConfig(gomock.Any()).DoAndReturn(mockGetConfig(map[string]string{
				"user.signingKey": key.PushAddr().String(),
			})).AnyTimes()

			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			mockStoredKey.EXPECT().GetMeta().Return(types2.StoredKeyMeta{})
			args := &SignTagArgs{SigningKey: key.PushAddr().String()}
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())

			ref := plumbing.NewReferenceFromStrings("", util.RandString(40))
			mockRepo.EXPECT().Tag("tag1").Return(ref, nil)
			tag := &object.Tag{Message: "tag1 message"}
			mockRepo.EXPECT().TagObject(ref.Hash()).Return(tag, nil)

			mockRepo.EXPECT().GetName().Return("repo_name")
			mockRepo.EXPECT().CreateTagWithMsg(gomock.Any(), tag.Message, args.SigningKey).Return(nil)
			mockRepo.EXPECT().Tag("tag1").Return(ref, nil)
			args.GetNextNonce = testGetNextNonce2("1", nil)
			args.SetRemotePushToken = testSetRemotePushToken("", fmt.Errorf("error"))

			err := SignTagCmd(cfg, []string{"tag1"}, mockRepo, args)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		When("args.SigningKey is a user address", func() {
			It("should pass push key id to CreateTagWithMsg, TxDetail object and GetNextNonce", func() {
				mockRepo.EXPECT().GetConfig(gomock.Any()).DoAndReturn(mockGetConfig(map[string]string{
					"user.signingKey": key.PushAddr().String(),
				})).AnyTimes()

				mockStoredKey := mocks.NewMockStoredKey(ctrl)
				mockStoredKey.EXPECT().GetMeta().Return(types2.StoredKeyMeta{})
				args := &SignTagArgs{SigningKey: key.Addr().String()}
				args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
				mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())

				ref := plumbing.NewReferenceFromStrings("", util.RandString(40))
				mockRepo.EXPECT().Tag("tag1").Return(ref, nil)
				tag := &object.Tag{Message: "tag1 message"}
				mockRepo.EXPECT().TagObject(ref.Hash()).Return(tag, nil)

				args.GetNextNonce = func(address string, rpcClient types3.Client) (string, error) {
					Expect(address).To(Equal(key.PushAddr().String()))
					return "", nil
				}
				args.SetRemotePushToken = func(targetRepo types.LocalRepo, args *server.GenSetPushTokenArgs) (string, error) {
					Expect(args.TxDetail.PushKeyID).To(Equal(key.PushAddr().String()))
					return "", nil
				}
				mockRepo.EXPECT().GetName().Return("repo_name")
				mockRepo.EXPECT().Tag("tag1").Return(ref, nil)
				mockRepo.EXPECT().CreateTagWithMsg(gomock.Any(), tag.Message, key.PushAddr().String()).Return(nil)

				err := SignTagCmd(cfg, []string{"tag1"}, mockRepo, args)
				Expect(err).To(BeNil())
			})
		})

		It("should set args.PushKeyID to value of git flag --local-user", func() {
			mockRepo.EXPECT().GetConfig(gomock.Any()).DoAndReturn(mockGetConfig(map[string]string{
				"user.signingKey": key.PushAddr().String(),
			})).AnyTimes()

			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			mockStoredKey.EXPECT().GetMeta().Return(types2.StoredKeyMeta{})
			args := &SignTagArgs{}
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())

			ref := plumbing.NewReferenceFromStrings("", util.RandString(40))
			mockRepo.EXPECT().Tag("tag1").Return(ref, nil)
			tag := &object.Tag{Message: "tag1 message"}
			mockRepo.EXPECT().TagObject(ref.Hash()).Return(tag, nil)

			args.GetNextNonce = testGetNextNonce2("1", nil)
			args.SetRemotePushToken = testSetRemotePushToken("", nil)
			mockRepo.EXPECT().GetName().Return("repo_name")
			mockRepo.EXPECT().Tag("tag1").Return(ref, nil)
			mockRepo.EXPECT().CreateTagWithMsg(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

			err := SignTagCmd(cfg, []string{"tag1", "--local-user", key.PushAddr().String()}, mockRepo, args)
			Expect(err).To(BeNil())
			Expect(args.SigningKey).To(Equal(key.PushAddr().String()))
		})

		It("should set args.Message to value of git flag --message", func() {
			mockRepo.EXPECT().GetConfig(gomock.Any()).DoAndReturn(mockGetConfig(map[string]string{
				"user.signingKey": key.PushAddr().String(),
			})).AnyTimes()

			mockStoredKey := mocks.NewMockStoredKey(ctrl)
			mockStoredKey.EXPECT().GetMeta().Return(types2.StoredKeyMeta{})
			args := &SignTagArgs{SigningKey: key.PushAddr().String()}
			args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
			mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())

			ref := plumbing.NewReferenceFromStrings("", util.RandString(40))
			mockRepo.EXPECT().Tag("tag1").Return(ref, nil)
			tag := &object.Tag{Message: "tag1 message"}
			mockRepo.EXPECT().TagObject(ref.Hash()).Return(tag, nil)

			args.GetNextNonce = testGetNextNonce2("1", nil)
			args.SetRemotePushToken = testSetRemotePushToken("", nil)
			mockRepo.EXPECT().GetName().Return("repo_name")
			mockRepo.EXPECT().Tag("tag1").Return(ref, nil)
			mockRepo.EXPECT().CreateTagWithMsg(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			msg := "A git message"

			err := SignTagCmd(cfg, []string{"tag1", "--message", msg}, mockRepo, args)
			Expect(err).To(BeNil())
			Expect(args.Message).To(Equal(msg))
		})

		When(".SignRefOnly is set to true", func() {
			It("should skip code for push token creation and signing", func() {
				mockRepo.EXPECT().GetConfig(gomock.Any()).DoAndReturn(mockGetConfig(map[string]string{
					"user.signingKey": key.PushAddr().String(),
				})).AnyTimes()

				mockStoredKey := mocks.NewMockStoredKey(ctrl)
				mockStoredKey.EXPECT().GetMeta().Return(types2.StoredKeyMeta{})
				args := &SignTagArgs{SigningKey: key.PushAddr().String(), SignRefOnly: true}
				args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
				mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())

				ref := plumbing.NewReferenceFromStrings("", util.RandString(40))
				mockRepo.EXPECT().Tag("tag1").Return(ref, nil)
				tag := &object.Tag{Message: "tag1 message"}
				mockRepo.EXPECT().TagObject(ref.Hash()).Return(tag, nil)

				mockRepo.EXPECT().GetName().Return("repo_name")
				mockRepo.EXPECT().CreateTagWithMsg(gomock.Any(), tag.Message, args.SigningKey).Return(nil)
				args.GetNextNonce = testGetNextNonce2("1", nil)

				err := SignTagCmd(cfg, []string{"tag1"}, mockRepo, args)
				Expect(err).To(BeNil())
			})
		})

		When(".CreatePushTokenOnly is set to true", func() {
			It("should skip code for reference signing", func() {
				mockRepo.EXPECT().GetConfig(gomock.Any()).DoAndReturn(mockGetConfig(map[string]string{
					"user.signingKey": key.PushAddr().String(),
				})).AnyTimes()

				mockStoredKey := mocks.NewMockStoredKey(ctrl)
				mockStoredKey.EXPECT().GetMeta().Return(types2.StoredKeyMeta{})
				args := &SignTagArgs{SigningKey: key.PushAddr().String(), CreatePushTokenOnly: true}
				args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
				mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())

				ref := plumbing.NewReferenceFromStrings("", util.RandString(40))
				mockRepo.EXPECT().Tag("tag1").Return(ref, nil)
				tag := &object.Tag{Message: "tag1 message"}
				mockRepo.EXPECT().TagObject(ref.Hash()).Return(tag, nil)

				mockRepo.EXPECT().GetName().Return("repo_name")
				mockRepo.EXPECT().Tag("tag1").Return(ref, nil)
				args.GetNextNonce = testGetNextNonce2("1", nil)
				args.SetRemotePushToken = testSetRemotePushToken("", nil)

				err := SignTagCmd(cfg, []string{"tag1"}, mockRepo, args)
				Expect(err).To(BeNil())
			})
		})

		When("tag already exist and it is signed", func() {
			BeforeEach(func() {
				mockRepo.EXPECT().GetConfig(gomock.Any()).DoAndReturn(mockGetConfig(map[string]string{
					"user.signingKey": key.PushAddr().String(),
				})).AnyTimes()
			})

			It("should skip signing tag", func() {

				mockStoredKey := mocks.NewMockStoredKey(ctrl)
				mockStoredKey.EXPECT().GetMeta().Return(types2.StoredKeyMeta{})
				args := &SignTagArgs{SigningKey: key.PushAddr().String(), CreatePushTokenOnly: true}
				args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
				mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())

				ref := plumbing.NewReferenceFromStrings("", util.RandString(40))
				mockRepo.EXPECT().Tag("tag1").Return(ref, nil)
				pemData := pem.EncodeToMemory(&pem.Block{Headers: map[string]string{"pkID": key.PushAddr().String()}})
				tag := &object.Tag{Message: "tag1 message", PGPSignature: string(pemData)}
				mockRepo.EXPECT().TagObject(ref.Hash()).Return(tag, nil)

				mockRepo.EXPECT().GetName().Return("repo_name")
				mockRepo.EXPECT().Tag("tag1").Return(ref, nil)
				args.GetNextNonce = testGetNextNonce2("1", nil)
				args.SetRemotePushToken = testSetRemotePushToken("", nil)

				err := SignTagCmd(cfg, []string{"tag1"}, mockRepo, args)
				Expect(err).To(BeNil())
			})

			When("ForceSign is true", func() {
				It("should not skip signing the tag object", func() {
					mockStoredKey := mocks.NewMockStoredKey(ctrl)
					mockStoredKey.EXPECT().GetMeta().Return(types2.StoredKeyMeta{})
					args := &SignTagArgs{SigningKey: key.PushAddr().String(), ForceSign: true}
					args.KeyUnlocker = testPushKeyUnlocker(mockStoredKey, nil)
					mockStoredKey.EXPECT().GetPushKeyAddress().Return(key.PushAddr().String())

					ref := plumbing.NewReferenceFromStrings("", util.RandString(40))
					mockRepo.EXPECT().Tag("tag1").Return(ref, nil)
					pemData := pem.EncodeToMemory(&pem.Block{Headers: map[string]string{"pkID": key.PushAddr().String()}})
					tag := &object.Tag{Message: "tag1 message", PGPSignature: string(pemData)}
					mockRepo.EXPECT().TagObject(ref.Hash()).Return(tag, nil)
					mockRepo.EXPECT().GetName().Return("repo_name")

					mockRepo.EXPECT().CreateTagWithMsg(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

					mockRepo.EXPECT().Tag("tag1").Return(ref, nil)
					args.GetNextNonce = testGetNextNonce2("1", nil)
					args.SetRemotePushToken = testSetRemotePushToken("", nil)

					err := SignTagCmd(cfg, []string{"tag1"}, mockRepo, args)
					Expect(err).To(BeNil())
				})
			})
		})
	})
})
