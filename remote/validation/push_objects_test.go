package validation_test

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/themakeos/lobe/config"
	"github.com/themakeos/lobe/crypto"
	"github.com/themakeos/lobe/mocks"
	plumbing2 "github.com/themakeos/lobe/remote/plumbing"
	"github.com/themakeos/lobe/remote/push/types"
	"github.com/themakeos/lobe/remote/validation"
	"github.com/themakeos/lobe/testutil"
	tickettypes "github.com/themakeos/lobe/ticket/types"
	"github.com/themakeos/lobe/types/constants"
	"github.com/themakeos/lobe/types/core"
	"github.com/themakeos/lobe/types/state"
	"github.com/themakeos/lobe/util"
	crypto2 "github.com/themakeos/lobe/util/crypto"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

var _ = Describe("Validation", func() {
	var err error
	var cfg *config.AppConfig
	var privKey *crypto.Key
	var ctrl *gomock.Controller
	var mockLogic *mocks.MockLogic
	var mockTickMgr *mocks.MockTicketManager
	var mockRepoKeeper *mocks.MockRepoKeeper
	var mockNSKeeper *mocks.MockNamespaceKeeper
	var mockPushKeyKeeper *mocks.MockPushKeyKeeper
	var mockAcctKeeper *mocks.MockAccountKeeper
	var mockSysKeeper *mocks.MockSystemKeeper

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"

		privKey = crypto.NewKeyFromIntSeed(1)

		mockObjs := testutil.MockLogic(ctrl)
		mockLogic = mockObjs.Logic
		mockTickMgr = mockObjs.TicketManager
		mockRepoKeeper = mockObjs.RepoKeeper
		mockPushKeyKeeper = mockObjs.PushKeyKeeper
		mockAcctKeeper = mockObjs.AccountKeeper
		mockSysKeeper = mockObjs.SysKeeper
		mockNSKeeper = mockObjs.NamespaceKeeper
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".CheckPushNoteSanity", func() {
		key := crypto.NewKeyFromIntSeed(1)
		nodePubKey := key.PubKey().MustBytes32()
		okNote := &types.Note{RepoName: "repo", PushKeyID: util.RandBytes(20),
			Timestamp: time.Now().Unix(), CreatorPubKey: key.PubKey().MustBytes32()}
		okSig, _ := key.PrivKey().Sign(okNote.Bytes())
		okNote.RemoteNodeSig = okSig
		newHash := util.RandString(40)
		oldHash := util.RandString(40)
		pkID := util.RandBytes(20)
		now := time.Now().Unix()

		var cases = [][]interface{}{
			{&types.Note{}, "field:repo, msg:repo name is required"},
			{&types.Note{RepoName: "repo"}, "field:pusherKeyId, msg:push key id is required"},
			{&types.Note{RepoName: "re*&po"}, "field:repo, msg:repo name is not valid"},
			{&types.Note{RepoName: "repo", Namespace: "*&ns"}, "field:namespace, msg:namespace is not valid"},
			{&types.Note{RepoName: "repo", PushKeyID: []byte("xyz")}, "field:pusherKeyId, msg:push key id is not valid"},
			{&types.Note{RepoName: "repo", PushKeyID: pkID, Timestamp: 0}, "field:timestamp, msg:timestamp is required"},
			{&types.Note{RepoName: "repo", PushKeyID: pkID, Timestamp: 2000000000}, "field:timestamp, msg:timestamp cannot be a future time"},
			{&types.Note{RepoName: "repo", PushKeyID: pkID, Timestamp: now}, "field:accountNonce, msg:account nonce must be greater than zero"},
			{&types.Note{RepoName: "repo", PushKeyID: pkID, Timestamp: now, PusherAcctNonce: 1}, "field:nodePubKey, msg:push node public key is required"},
			{&types.Note{RepoName: "repo", PushKeyID: pkID, Timestamp: now, PusherAcctNonce: 1, CreatorPubKey: nodePubKey}, "field:nodeSig, msg:push node signature is required"},
			{&types.Note{RepoName: "repo", PushKeyID: pkID, Timestamp: now, PusherAcctNonce: 1, CreatorPubKey: nodePubKey, RemoteNodeSig: []byte("invalid signature")}, "field:nodeSig, msg:failed to verify signature"},
			{&types.Note{RepoName: "repo", References: []*types.PushedReference{{}}}, "index:0, field:references.name, msg:name is required"},
			{&types.Note{RepoName: "repo", References: []*types.PushedReference{{Name: "ref1"}}}, "index:0, field:references.oldHash, msg:old hash is required"},
			{&types.Note{RepoName: "repo", References: []*types.PushedReference{{Name: "ref1", OldHash: "invalid"}}}, "index:0, field:references.oldHash, msg:old hash is not valid"},
			{&types.Note{RepoName: "repo", References: []*types.PushedReference{{Name: "ref1", OldHash: oldHash}}}, "index:0, field:references.newHash, msg:new hash is required"},
			{&types.Note{RepoName: "repo", References: []*types.PushedReference{{Name: "ref1", OldHash: oldHash, NewHash: "invalid"}}}, "index:0, field:references.newHash, msg:new hash is not valid"},
			{&types.Note{RepoName: "repo", References: []*types.PushedReference{{Name: "ref1", OldHash: oldHash, NewHash: newHash}}}, "index:0, field:references.nonce, msg:reference nonce must be greater than zero"},
			{&types.Note{RepoName: "repo", References: []*types.PushedReference{{Name: "ref1", OldHash: oldHash, NewHash: newHash, Nonce: 1}}}, "index:0, field:fee, msg:fee is required"},
			{&types.Note{RepoName: "repo", References: []*types.PushedReference{{Name: "ref1", OldHash: oldHash, NewHash: newHash, Nonce: 1, Fee: "ten"}}}, "index:0, field:fee, msg:fee must be numeric"},
			{&types.Note{RepoName: "repo", References: []*types.PushedReference{{Name: "ref1", OldHash: oldHash, NewHash: newHash, Nonce: 1, Fee: "1", Value: "one"}}}, "index:0, field:value, msg:value must be numeric"},
			{&types.Note{RepoName: "repo", References: []*types.PushedReference{{Name: "ref1", OldHash: oldHash, NewHash: newHash, Nonce: 1, Fee: "0", MergeProposalID: "1a"}}}, "index:0, field:mergeID, msg:merge proposal id must be numeric"},
			{&types.Note{RepoName: "repo", References: []*types.PushedReference{{Name: "ref1", OldHash: oldHash, NewHash: newHash, Nonce: 1, Fee: "0", MergeProposalID: "123456789"}}}, "index:0, field:mergeID, msg:merge proposal id exceeded 8 bytes limit"},
			{&types.Note{RepoName: "repo", References: []*types.PushedReference{{Name: "ref1", OldHash: oldHash, NewHash: newHash, Nonce: 1, Fee: "0"}}}, "index:0, field:pushSig, msg:signature is required"},
		}

		It("should check cases", func() {
			for _, c := range cases {
				_c := c
				if _c[1] != nil {
					Expect(validation.CheckPushNoteSanity(_c[0].(*types.Note)).Error()).To(Equal(_c[1]))
				} else {
					Expect(validation.CheckPushNoteSanity(_c[0].(*types.Note))).To(BeNil())
				}
			}
		})
	})

	Describe(".CheckPushedReferenceConsistency", func() {
		var mockRepo *mocks.MockLocalRepo
		var oldHash = fmt.Sprintf("%x", util.RandBytes(20))
		var newHash = fmt.Sprintf("%x", util.RandBytes(20))
		_ = newHash

		BeforeEach(func() {
			mockRepo = mocks.NewMockLocalRepo(ctrl)
		})

		When("old hash is non-zero and pushed reference does not exist", func() {
			BeforeEach(func() {
				refs := &types.PushedReference{Name: "refs/heads/master", OldHash: oldHash}
				repository := &state.Repository{References: map[string]*state.Reference{}}
				err = validation.CheckPushedReferenceConsistency(mockRepo, refs, repository)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:references, msg:reference 'refs/heads/master' is unknown"))
			})
		})

		When("old hash is zero and pushed reference does not exist", func() {
			BeforeEach(func() {
				refs := &types.PushedReference{Name: "refs/heads/master", OldHash: strings.Repeat("0", 40)}
				repository := &state.Repository{References: map[string]*state.Reference{}}
				err = validation.CheckPushedReferenceConsistency(mockRepo, refs, repository)
			})

			It("should not return error about unknown reference", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).ToNot(Equal("field:references, msg:reference 'refs/heads/master' is unknown"))
			})
		})

		When("old hash of reference is different from the local hash of same reference", func() {
			BeforeEach(func() {
				refName := "refs/heads/master"
				ref := &types.PushedReference{Name: refName, OldHash: oldHash}
				repository := &state.Repository{References: map[string]*state.Reference{refName: {Nonce: 0}}}
				mockRepo.EXPECT().Reference(plumbing.ReferenceName(refName), false).
					Return(plumbing.NewReferenceFromStrings("", util.RandString(40)), nil)
				err = validation.CheckPushedReferenceConsistency(mockRepo, ref, repository)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:references, msg:reference 'refs/heads/master' old hash does not match its local version"))
			})
		})

		When("old hash of reference is non-zero and the local equivalent reference is not accessible", func() {
			BeforeEach(func() {
				refName := "refs/heads/master"
				ref := &types.PushedReference{Name: refName, OldHash: oldHash}
				repository := &state.Repository{References: map[string]*state.Reference{refName: {Nonce: 0}}}
				mockRepo.EXPECT().Reference(plumbing.ReferenceName(refName), false).
					Return(nil, plumbing.ErrReferenceNotFound)
				err = validation.CheckPushedReferenceConsistency(mockRepo, ref, repository)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:references, msg:reference 'refs/heads/master' does not exist locally"))
			})
		})

		When("old hash of reference is non-zero and nil repo passed", func() {
			BeforeEach(func() {
				refName := "refs/heads/master"
				ref := &types.PushedReference{Name: refName, OldHash: oldHash}
				repository := &state.Repository{References: map[string]*state.Reference{refName: {Nonce: 0}}}
				err = validation.CheckPushedReferenceConsistency(nil, ref, repository)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).ToNot(Equal("field:references, msg:reference 'refs/heads/master' does not exist locally"))
			})
		})

		When("old hash of reference is non-zero and it is different from the hash of the equivalent local reference", func() {
			BeforeEach(func() {
				refName := "refs/heads/master"
				ref := &types.PushedReference{Name: refName, OldHash: oldHash}
				repository := &state.Repository{References: map[string]*state.Reference{refName: {Nonce: 0}}}
				mockRepo.EXPECT().Reference(plumbing.ReferenceName(refName), false).
					Return(plumbing.NewReferenceFromStrings("", util.RandString(40)), nil)
				err = validation.CheckPushedReferenceConsistency(mockRepo, ref, repository)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:references, msg:reference 'refs/heads/master' old hash does not match its local version"))
			})
		})

		When("pushed reference nonce is unexpected", func() {
			BeforeEach(func() {
				refName := "refs/heads/master"
				ref := &types.PushedReference{OldHash: oldHash, Name: refName, NewHash: newHash, Nonce: 2}
				repository := &state.Repository{References: map[string]*state.Reference{refName: {Nonce: 0}}}
				mockRepo.EXPECT().
					Reference(plumbing.ReferenceName(refName), false).
					Return(plumbing.NewHashReference("", plumbing.NewHash(oldHash)), nil)
				err = validation.CheckPushedReferenceConsistency(mockRepo, ref, repository)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:references, msg:reference 'refs/heads/master' has nonce '2', expecting '1'"))
			})
		})

		When("nonce is unset", func() {
			BeforeEach(func() {
				refName := "refs/heads/master"
				refs := &types.PushedReference{Name: refName, OldHash: oldHash, NewHash: newHash, Nonce: 0}
				repository := &state.Repository{References: map[string]*state.Reference{refName: {Nonce: 0}}}
				mockRepo.EXPECT().
					Reference(plumbing.ReferenceName(refName), false).
					Return(plumbing.NewHashReference("", plumbing.NewHash(oldHash)), nil)
				err = validation.CheckPushedReferenceConsistency(mockRepo, refs, repository)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:references, msg:reference 'refs/heads/master' has nonce '0', expecting '1'"))
			})
		})

		When("pushed reference is a new merge request reference", func() {
			var refName, newHash string
			BeforeEach(func() {
				refName = plumbing2.MakeMergeRequestReference(1)
				_ = refName
				newHash = util.RandString(40)
				_ = newHash
				oldHash = strings.Repeat("0", 40)
			})

			It("should return err when repo does not require a proposal fee but 'Value' is non-zero ", func() {
				refs := &types.PushedReference{Name: refName, OldHash: oldHash, NewHash: newHash, Nonce: 1, Fee: "1", Value: "1"}
				repository := &state.Repository{Config: state.DefaultRepoConfig}
				err = validation.CheckPushedReferenceConsistency(mockRepo, refs, repository)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("field:value, msg:" + constants.ErrProposalFeeNotExpected.Error()))
			})

			It("should return err when repo requires a proposal fee and 'Value' is zero (0)", func() {
				refs := &types.PushedReference{Name: refName, OldHash: oldHash, NewHash: newHash, Nonce: 1, Fee: "1", Value: "0"}
				repository := &state.Repository{Config: state.DefaultRepoConfig}
				repository.Config.Gov.PropFee = 100
				repository.Config.Gov.NoPropFeeForMergeReq = false
				err = validation.CheckPushedReferenceConsistency(mockRepo, refs, repository)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("field:value, msg:" + constants.ErrFullProposalFeeRequired.Error()))
			})

			When("config exempts merge request from paying proposal fee", func() {
				It("should return nil when repo requires a proposal fee and 'Value' is zero (0)", func() {
					refs := &types.PushedReference{Name: refName, OldHash: oldHash, NewHash: newHash, Nonce: 1, Fee: "1", Value: "0"}
					repository := &state.Repository{Config: state.DefaultRepoConfig}
					repository.Config.Gov.PropFee = 100
					repository.Config.Gov.NoPropFeeForMergeReq = true
					err = validation.CheckPushedReferenceConsistency(mockRepo, refs, repository)
					Expect(err).To(BeNil())
				})
			})
		})

		When("no validation error", func() {
			BeforeEach(func() {
				refName := "refs/heads/master"
				newHash := util.RandString(40)
				refs := &types.PushedReference{Name: refName, OldHash: oldHash, NewHash: newHash, Nonce: 1, Fee: "1"}
				repository := &state.Repository{References: map[string]*state.Reference{refName: {Nonce: 0}}}
				err = validation.CheckPushedReferenceConsistency(mockRepo, refs, repository)
			})

			It("should return err", func() {
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".CheckPushNoteConsistency", func() {
		When("no repository with matching name exist", func() {
			BeforeEach(func() {
				tx := &types.Note{RepoName: "unknown"}
				mockRepoKeeper.EXPECT().Get(tx.RepoName).Return(state.BareRepository())
				err = validation.CheckPushNoteConsistency(tx, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:repo, msg:repository named 'unknown' is unknown"))
			})
		})

		When("namespace is set but does not exist", func() {
			BeforeEach(func() {
				tx := &types.Note{Namespace: "ns1"}
				mockRepoKeeper.EXPECT().Get(gomock.Any()).Return(&state.Repository{Balance: "10"})
				mockNSKeeper.EXPECT().Get(crypto2.MakeNamespaceHash(tx.Namespace)).Return(state.BareNamespace())
				err = validation.CheckPushNoteConsistency(tx, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:namespace, msg:namespace 'ns1' is unknown"))
			})
		})

		When("namespace is set but repo not a target of any domain", func() {
			BeforeEach(func() {
				tx := &types.Note{Namespace: "ns1"}
				mockRepoKeeper.EXPECT().Get(gomock.Any()).Return(&state.Repository{Balance: "10"})
				ns := state.BareNamespace()
				ns.Domains["domain1"] = "r/some_repo"
				mockNSKeeper.EXPECT().Get(crypto2.MakeNamespaceHash(tx.Namespace)).Return(ns)
				err = validation.CheckPushNoteConsistency(tx, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:repo, msg:repo not a target in namespace 'ns1'"))
			})
		})

		When("pusher public key id is unknown", func() {
			BeforeEach(func() {
				tx := &types.Note{RepoName: "repo1", PushKeyID: util.RandBytes(20)}
				mockRepoKeeper.EXPECT().Get(tx.RepoName).Return(&state.Repository{Balance: "10"})
				mockPushKeyKeeper.EXPECT().Get(crypto.BytesToPushKeyID(tx.PushKeyID)).Return(state.BarePushKey())
				err = validation.CheckPushNoteConsistency(tx, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("field:pusherKeyId, msg:pusher's public key id '.*' is unknown"))
			})
		})

		When("push owner address not the same as the pusher address", func() {
			BeforeEach(func() {
				tx := &types.Note{
					RepoName:      "repo1",
					PushKeyID:     util.RandBytes(20),
					PusherAddress: "address1",
				}
				mockRepoKeeper.EXPECT().Get(tx.RepoName).Return(&state.Repository{Balance: "10"})

				pushKey := state.BarePushKey()
				pushKey.Address = "address2"
				mockPushKeyKeeper.EXPECT().Get(crypto.BytesToPushKeyID(tx.PushKeyID)).Return(pushKey)
				err = validation.CheckPushNoteConsistency(tx, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:pusherAddr, msg:push key does not belong to pusher"))
			})
		})

		When("unable to find pusher account", func() {
			BeforeEach(func() {
				tx := &types.Note{
					RepoName:      "repo1",
					PushKeyID:     util.RandBytes(20),
					PusherAddress: "address1",
				}
				mockRepoKeeper.EXPECT().Get(tx.RepoName).Return(&state.Repository{Balance: "10"})

				pushKey := state.BarePushKey()
				pushKey.Address = "address1"
				mockPushKeyKeeper.EXPECT().Get(crypto.BytesToPushKeyID(tx.PushKeyID)).Return(pushKey)

				mockAcctKeeper.EXPECT().Get(tx.PusherAddress).Return(state.BareAccount())

				err = validation.CheckPushNoteConsistency(tx, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:pusherAddr, msg:pusher account not found"))
			})
		})

		When("pusher account nonce is not correct", func() {
			BeforeEach(func() {
				tx := &types.Note{RepoName: "repo1", PushKeyID: util.RandBytes(20), PusherAddress: "address1", PusherAcctNonce: 3}
				mockRepoKeeper.EXPECT().Get(tx.RepoName).Return(&state.Repository{Balance: "10"})

				pushKey := state.BarePushKey()
				pushKey.Address = "address1"
				mockPushKeyKeeper.EXPECT().Get(crypto.BytesToPushKeyID(tx.PushKeyID)).Return(pushKey)

				acct := state.BareAccount()
				acct.Nonce = 1
				mockAcctKeeper.EXPECT().Get(tx.PusherAddress).Return(acct)

				err = validation.CheckPushNoteConsistency(tx, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:accountNonce, msg:wrong account nonce '3', expecting '2'"))
			})
		})

		When("reference signature is invalid", func() {
			BeforeEach(func() {
				tx := &types.Note{RepoName: "repo1", PushKeyID: util.RandBytes(20), PusherAddress: "address1", PusherAcctNonce: 2}
				tx.References = append(tx.References, &types.PushedReference{
					Name:    "refs/heads/master",
					Nonce:   1,
					PushSig: util.RandBytes(64),
				})
				mockRepoKeeper.EXPECT().Get(tx.RepoName).Return(&state.Repository{Balance: "10"})

				pushKey := state.BarePushKey()
				pushKey.Address = "address1"
				pushKey.PubKey = privKey.PubKey().ToPublicKey()
				mockPushKeyKeeper.EXPECT().Get(crypto.BytesToPushKeyID(tx.PushKeyID)).Return(pushKey)

				acct := state.BareAccount()
				acct.Nonce = 1
				mockAcctKeeper.EXPECT().Get(tx.PusherAddress).Return(acct)

				err = validation.CheckPushNoteConsistency(tx, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(MatchRegexp("index:0, field:references, msg:reference (.*) signature is not valid"))
			})
		})

		When("pusher account balance not sufficient to pay fee", func() {
			BeforeEach(func() {

				tx := &types.Note{
					RepoName:        "repo1",
					PushKeyID:       util.RandBytes(20),
					PusherAddress:   "address1",
					PusherAcctNonce: 2,
				}

				mockRepoKeeper.EXPECT().Get(tx.RepoName).Return(&state.Repository{Balance: "10"})

				pushKey := state.BarePushKey()
				pushKey.Address = "address1"
				mockPushKeyKeeper.EXPECT().Get(crypto.BytesToPushKeyID(tx.PushKeyID)).Return(pushKey)

				acct := state.BareAccount()
				acct.Nonce = 1
				mockAcctKeeper.EXPECT().Get(tx.PusherAddress).Return(acct)

				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&core.BlockInfo{Height: 1}, nil)
				mockLogic.EXPECT().DrySend(tx.PusherAddress, util.String("0"), tx.GetFee(), uint64(2), uint64(1)).
					Return(fmt.Errorf("insufficient"))

				err = validation.CheckPushNoteConsistency(tx, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("insufficient"))
			})
		})
	})

	Describe(".CheckEndorsementSanity", func() {
		It("should return error when push note id is not set", func() {
			err := validation.CheckEndorsementSanity(&types.PushEndorsement{}, false, -1)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("field:endorsements.noteID, msg:push note ID is required"))
		})

		It("should return error when public key is not valid", func() {
			err := validation.CheckEndorsementSanity(&types.PushEndorsement{
				NoteID:         []byte("id"),
				EndorserPubKey: util.EmptyBytes32,
			}, false, -1)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("field:endorsements.pubKey, msg:endorser's public key is required"))
		})

		When("endorsement is not from a push transaction", func() {
			It("should return error when there are no references in the endorsement at index=0", func() {
				err := validation.CheckEndorsementSanity(&types.PushEndorsement{
					NoteID:         []byte("id"),
					EndorserPubKey: util.BytesToBytes32([]byte("pub_key")),
					References:     []*types.EndorsedReference{},
				}, false, 0)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("index:0, field:endorsements.refs, msg:at least one reference is required"))
			})

			It("should return error when there are no references in the endorsement at index >= 1", func() {
				err := validation.CheckEndorsementSanity(&types.PushEndorsement{
					NoteID:         []byte("id"),
					EndorserPubKey: util.BytesToBytes32([]byte("pub_key")),
					References:     []*types.EndorsedReference{},
				}, false, 1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("index:1, field:endorsements.refs, msg:at least one reference is required"))
			})
		})

		When("endorsement is from a push transaction", func() {
			It("should return error when there are no references in the endorsement at index=0", func() {
				err := validation.CheckEndorsementSanity(&types.PushEndorsement{
					NoteID:         []byte("id"),
					EndorserPubKey: util.BytesToBytes32([]byte("pub_key")),
					References:     []*types.EndorsedReference{},
				}, true, 0)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("index:0, field:endorsements.refs, msg:at least one reference is required"))
			})

			It("should not return expected error when there are no references in the endorsement at index >= 1", func() {
				err := validation.CheckEndorsementSanity(&types.PushEndorsement{
					NoteID:         []byte("id"),
					EndorserPubKey: util.BytesToBytes32([]byte("pub_key")),
					References:     []*types.EndorsedReference{},
				}, true, 1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).ToNot(Equal("index:1, field:endorsements.refs, msg:at least one reference is required"))
			})

			It("should not return when BLS signature is set", func() {
				err := validation.CheckEndorsementSanity(&types.PushEndorsement{
					NoteID:         []byte("id"),
					EndorserPubKey: util.BytesToBytes32([]byte("pub_key")),
					References:     []*types.EndorsedReference{},
					SigBLS:         []byte{1, 2, 3},
				}, true, 1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("index:1, field:endorsements.sigBLS, msg:BLS signature is not expected"))
			})

			It("should not return when endorsement at index > 0 has references set", func() {
				err := validation.CheckEndorsementSanity(&types.PushEndorsement{
					EndorserPubKey: util.BytesToBytes32([]byte("pub_key")),
					References:     []*types.EndorsedReference{{}},
				}, true, 1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("index:1, field:endorsements.refs, msg:references not expected"))
			})
		})

		It("should return error when BLS signature is not set", func() {
			err := validation.CheckEndorsementSanity(&types.PushEndorsement{
				NoteID:         []byte("id"),
				EndorserPubKey: util.BytesToBytes32([]byte("pub_key")),
				References:     []*types.EndorsedReference{{}},
			}, false, -1)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("field:endorsements.sigBLS, msg:endorser's BLS signature is required"))
		})

		It("should return no error when endorsement is valid", func() {
			key := crypto.NewKeyFromIntSeed(1)
			end := &types.PushEndorsement{
				NoteID:         []byte("id"),
				EndorserPubKey: key.PubKey().MustBytes32(),
				References:     []*types.EndorsedReference{{}},
				SigBLS:         util.RandBytes(64),
			}
			err := validation.CheckEndorsementSanity(end, false, -1)
			Expect(err).To(BeNil())
		})
	})

	Describe(".CheckEndorsementConsistency", func() {
		When("unable to fetch top hosts", func() {
			BeforeEach(func() {
				mockTickMgr.EXPECT().GetTopHosts(gomock.Any()).Return(nil, fmt.Errorf("error"))
				err = validation.CheckEndorsementConsistency(&types.PushEndorsement{
					NoteID:         []byte("id"),
					EndorserPubKey: util.EmptyBytes32,
				}, mockLogic, false, -1)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to get top hosts: error"))
			})
		})

		When("sender is not a host", func() {
			BeforeEach(func() {
				key := crypto.NewKeyFromIntSeed(1)
				mockTickMgr.EXPECT().GetTopHosts(gomock.Any()).Return([]*tickettypes.SelectedTicket{}, nil)
				end := &types.PushEndorsement{NoteID: []byte("id"), EndorserPubKey: key.PubKey().MustBytes32()}
				err = validation.CheckEndorsementConsistency(end, mockLogic, false, -1)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:endorsements.senderPubKey, msg:sender public key does not belong to an active host"))
			})
		})

		When("unable to decode host's BLS public key", func() {
			BeforeEach(func() {
				key := crypto.NewKeyFromIntSeed(1)
				ticket := &tickettypes.Ticket{ProposerPubKey: key.PubKey().MustBytes32(), BLSPubKey: util.RandBytes(128)}
				mockTickMgr.EXPECT().GetTopHosts(gomock.Any()).Return([]*tickettypes.SelectedTicket{{Ticket: ticket}}, nil)
				end := &types.PushEndorsement{NoteID: []byte("id"), EndorserPubKey: key.PubKey().MustBytes32()}
				err = validation.CheckEndorsementConsistency(end, mockLogic, false, -1)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("failed to decode bls public key of endorser"))
			})
		})

		When("unable to verify BLS signature", func() {
			BeforeEach(func() {
				key := crypto.NewKeyFromIntSeed(1)
				key2 := crypto.NewKeyFromIntSeed(2)
				ticket := &tickettypes.Ticket{ProposerPubKey: key.PubKey().MustBytes32(), BLSPubKey: key2.PrivKey().BLSKey().Public().Bytes()}
				mockTickMgr.EXPECT().GetTopHosts(gomock.Any()).Return([]*tickettypes.SelectedTicket{{Ticket: ticket}}, nil)
				end := &types.PushEndorsement{NoteID: []byte("id"), EndorserPubKey: key.PubKey().MustBytes32(), SigBLS: util.RandBytes(64)}
				err = validation.CheckEndorsementConsistency(end, mockLogic, false, -1)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("field:endorsements.sig, msg:signature could not be verified"))
			})
		})

		When("noBLSSigCheck is true", func() {
			BeforeEach(func() {
				key := crypto.NewKeyFromIntSeed(1)
				key2 := crypto.NewKeyFromIntSeed(2)
				ticket := &tickettypes.Ticket{ProposerPubKey: key.PubKey().MustBytes32(), BLSPubKey: key2.PrivKey().BLSKey().Public().Bytes()}
				mockTickMgr.EXPECT().GetTopHosts(gomock.Any()).Return([]*tickettypes.SelectedTicket{{Ticket: ticket}}, nil)
				end := &types.PushEndorsement{NoteID: []byte("id"), EndorserPubKey: key.PubKey().MustBytes32()}
				err = validation.CheckEndorsementConsistency(end, mockLogic, true, -1)
			})

			It("should not check signature", func() {
				Expect(err).To(BeNil())
			})
		})
	})
})
