package validation_test

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/crypto"
	dhttypes "gitlab.com/makeos/mosdef/dht/types"
	"gitlab.com/makeos/mosdef/mocks"
	plumbing2 "gitlab.com/makeos/mosdef/remote/plumbing"
	"gitlab.com/makeos/mosdef/remote/push/types"
	"gitlab.com/makeos/mosdef/remote/validation"
	"gitlab.com/makeos/mosdef/testutil"
	tickettypes "gitlab.com/makeos/mosdef/ticket/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/util"
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

	Describe(".validation.CheckPushNoteSyntax", func() {
		key := crypto.NewKeyFromIntSeed(1)
		okTx := &types.PushNote{RepoName: "repo", PushKeyID: util.RandBytes(20), Timestamp: time.Now().Unix(), NodePubKey: key.PubKey().MustBytes32()}
		bz, _ := key.PrivKey().Sign(okTx.Bytes())
		okTx.NodeSig = bz

		var cases = [][]interface{}{
			{&types.PushNote{}, "field:repo, msg:repo name is required"},
			{&types.PushNote{RepoName: "repo"}, "field:pusherKeyId, msg:push key id is required"},
			{&types.PushNote{RepoName: "re*&po"}, "field:repo, msg:repo name is not valid"},
			{&types.PushNote{RepoName: "repo", Namespace: "*&ns"}, "field:namespace, msg:namespace is not valid"},
			{&types.PushNote{RepoName: "repo", PushKeyID: []byte("xyz")}, "field:pusherKeyId, msg:push key id is not valid"},
			{&types.PushNote{RepoName: "repo", PushKeyID: util.RandBytes(20), Timestamp: 0}, "field:timestamp, msg:timestamp is required"},
			{&types.PushNote{RepoName: "repo", PushKeyID: util.RandBytes(20), Timestamp: 2000000000}, "field:timestamp, msg:timestamp cannot be a future time"},
			{&types.PushNote{RepoName: "repo", PushKeyID: util.RandBytes(20), Timestamp: time.Now().Unix()}, "field:accountNonce, msg:account nonce must be greater than zero"},
			{&types.PushNote{RepoName: "repo", PushKeyID: util.RandBytes(20), Timestamp: time.Now().Unix(), PusherAcctNonce: 1}, "field:nodePubKey, msg:push node public key is required"},
			{&types.PushNote{RepoName: "repo", PushKeyID: util.RandBytes(20), Timestamp: time.Now().Unix(), PusherAcctNonce: 1, NodePubKey: key.PubKey().MustBytes32()}, "field:nodeSig, msg:push node signature is required"},
			{&types.PushNote{RepoName: "repo", PushKeyID: util.RandBytes(20), Timestamp: time.Now().Unix(), PusherAcctNonce: 1, NodePubKey: key.PubKey().MustBytes32(), NodeSig: []byte("invalid signature")}, "field:nodeSig, msg:failed to verify signature"},
			{&types.PushNote{RepoName: "repo", References: []*types.PushedReference{{}}}, "index:0, field:references.name, msg:name is required"},
			{&types.PushNote{RepoName: "repo", References: []*types.PushedReference{{Name: "ref1"}}}, "index:0, field:references.oldHash, msg:old hash is required"},
			{&types.PushNote{RepoName: "repo", References: []*types.PushedReference{{Name: "ref1", OldHash: "invalid"}}}, "index:0, field:references.oldHash, msg:old hash is not valid"},
			{&types.PushNote{RepoName: "repo", References: []*types.PushedReference{{Name: "ref1", OldHash: util.RandString(40)}}}, "index:0, field:references.newHash, msg:new hash is required"},
			{&types.PushNote{RepoName: "repo", References: []*types.PushedReference{{Name: "ref1", OldHash: util.RandString(40), NewHash: "invalid"}}}, "index:0, field:references.newHash, msg:new hash is not valid"},
			{&types.PushNote{RepoName: "repo", References: []*types.PushedReference{{Name: "ref1", OldHash: util.RandString(40), NewHash: util.RandString(40)}}}, "index:0, field:references.nonce, msg:reference nonce must be greater than zero"},
			{&types.PushNote{RepoName: "repo", References: []*types.PushedReference{{Name: "ref1", OldHash: util.RandString(40), NewHash: util.RandString(40), Nonce: 1, Objects: []string{"invalid object"}}}}, "index:0, field:references.objects.0, msg:object hash is not valid"},
			{&types.PushNote{RepoName: "repo", References: []*types.PushedReference{{Name: "ref1", OldHash: util.RandString(40), NewHash: util.RandString(40), Nonce: 1}}}, "index:0, field:fee, msg:fee is required"},
			{&types.PushNote{RepoName: "repo", References: []*types.PushedReference{{Name: "ref1", OldHash: util.RandString(40), NewHash: util.RandString(40), Nonce: 1, Fee: "ten"}}}, "index:0, field:fee, msg:fee must be numeric"},
			{&types.PushNote{RepoName: "repo", References: []*types.PushedReference{{Name: "ref1", OldHash: util.RandString(40), NewHash: util.RandString(40), Nonce: 1, Fee: "0", MergeProposalID: "1a"}}}, "index:0, field:mergeID, msg:merge proposal id must be numeric"},
			{&types.PushNote{RepoName: "repo", References: []*types.PushedReference{{Name: "ref1", OldHash: util.RandString(40), NewHash: util.RandString(40), Nonce: 1, Fee: "0", MergeProposalID: "123456789"}}}, "index:0, field:mergeID, msg:merge proposal id exceeded 8 bytes limit"},
			{&types.PushNote{RepoName: "repo", References: []*types.PushedReference{{Name: "ref1", OldHash: util.RandString(40), NewHash: util.RandString(40), Nonce: 1, Fee: "0"}}}, "index:0, field:pushSig, msg:signature is required"},
		}

		It("should check cases", func() {
			for _, c := range cases {
				_c := c
				if _c[1] != nil {
					Expect(validation.CheckPushNoteSyntax(_c[0].(*types.PushNote)).Error()).To(Equal(_c[1]))
				} else {
					Expect(validation.CheckPushNoteSyntax(_c[0].(*types.PushNote))).To(BeNil())
				}
			}
		})
	})

	Describe(".CheckPushedReferenceConsistency", func() {
		var mockRepo *mocks.MockLocalRepo
		var oldHash = fmt.Sprintf("%x", util.RandBytes(20))
		var newHash = fmt.Sprintf("%x", util.RandBytes(20))

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
				ref := &types.PushedReference{OldHash: oldHash, Name: refName, NewHash: newHash, Objects: []string{newHash}, Nonce: 2}
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
				refs := &types.PushedReference{Name: refName, OldHash: oldHash, NewHash: newHash, Objects: []string{newHash}, Nonce: 0}
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

		When("no validation error", func() {
			BeforeEach(func() {
				refName := "refs/heads/master"
				newHash := util.RandString(40)
				refs := &types.PushedReference{Name: refName, OldHash: oldHash, NewHash: newHash, Objects: []string{newHash}, Nonce: 1, Fee: "1"}
				repository := &state.Repository{References: map[string]*state.Reference{refName: {Nonce: 0}}}
				mockRepo.EXPECT().
					Reference(plumbing.ReferenceName(refName), false).
					Return(plumbing.NewHashReference("", plumbing.NewHash(oldHash)), nil)

				err = validation.CheckPushedReferenceConsistency(mockRepo, refs, repository)
			})

			It("should return err", func() {
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".validation.CheckPushNoteConsistency", func() {
		When("no repository with matching name exist", func() {
			BeforeEach(func() {
				tx := &types.PushNote{RepoName: "unknown"}
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
				tx := &types.PushNote{Namespace: "ns1"}
				mockRepoKeeper.EXPECT().Get(gomock.Any()).Return(&state.Repository{Balance: "10"})
				mockNSKeeper.EXPECT().Get(util.HashNamespace(tx.Namespace)).Return(state.BareNamespace())
				err = validation.CheckPushNoteConsistency(tx, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:namespace, msg:namespace 'ns1' is unknown"))
			})
		})

		When("namespace is set but repo not a target of any domain", func() {
			BeforeEach(func() {
				tx := &types.PushNote{Namespace: "ns1"}
				mockRepoKeeper.EXPECT().Get(gomock.Any()).Return(&state.Repository{Balance: "10"})
				ns := state.BareNamespace()
				ns.Domains["domain1"] = "r/some_repo"
				mockNSKeeper.EXPECT().Get(util.HashNamespace(tx.Namespace)).Return(ns)
				err = validation.CheckPushNoteConsistency(tx, mockLogic)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:repo, msg:repo not a target in namespace 'ns1'"))
			})
		})

		When("pusher public key id is unknown", func() {
			BeforeEach(func() {
				tx := &types.PushNote{RepoName: "repo1", PushKeyID: util.RandBytes(20)}
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
				tx := &types.PushNote{
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
				tx := &types.PushNote{
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
				tx := &types.PushNote{RepoName: "repo1", PushKeyID: util.RandBytes(20), PusherAddress: "address1", PusherAcctNonce: 3}
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
				tx := &types.PushNote{RepoName: "repo1", PushKeyID: util.RandBytes(20), PusherAddress: "address1", PusherAcctNonce: 2}
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

				tx := &types.PushNote{
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

	Describe(".CheckPushEndorsement", func() {
		It("should return error when push note id is not set", func() {
			err := validation.CheckPushEndorsement(&types.PushEndorsement{}, -1)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("field:endorsements.pushNoteID, msg:push note id is required"))
		})

		It("should return error when public key is not valid", func() {
			err := validation.CheckPushEndorsement(&types.PushEndorsement{
				NoteID:         util.StrToBytes32("id"),
				EndorserPubKey: util.EmptyBytes32,
			}, -1)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("field:endorsements.senderPubKey, msg:sender public key is required"))
		})
	})

	Describe(".validation.CheckPushEndConsistency", func() {
		When("unable to fetch top hosts", func() {
			BeforeEach(func() {
				mockTickMgr.EXPECT().GetTopHosts(gomock.Any()).Return(nil, fmt.Errorf("error"))
				err = validation.CheckPushEndConsistency(&types.PushEndorsement{
					NoteID:         util.StrToBytes32("id"),
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
				err = validation.CheckPushEndConsistency(&types.PushEndorsement{
					NoteID:         util.StrToBytes32("id"),
					EndorserPubKey: key.PubKey().MustBytes32(),
				}, mockLogic, false, -1)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:endorsements.senderPubKey, msg:sender public key does not belong to an active host"))
			})
		})

		When("unable to decode host's BLS public key", func() {
			BeforeEach(func() {
				key := crypto.NewKeyFromIntSeed(1)
				mockTickMgr.EXPECT().GetTopHosts(gomock.Any()).Return([]*tickettypes.SelectedTicket{
					{
						Ticket: &tickettypes.Ticket{
							ProposerPubKey: key.PubKey().MustBytes32(),
							BLSPubKey:      util.RandBytes(128),
						},
					},
				}, nil)
				err = validation.CheckPushEndConsistency(&types.PushEndorsement{
					NoteID:         util.StrToBytes32("id"),
					EndorserPubKey: key.PubKey().MustBytes32(),
				}, mockLogic, false, -1)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("failed to decode bls public key of endorser"))
			})
		})

		When("unable to verify signature", func() {
			BeforeEach(func() {
				key := crypto.NewKeyFromIntSeed(1)
				key2 := crypto.NewKeyFromIntSeed(2)
				mockTickMgr.EXPECT().GetTopHosts(gomock.Any()).Return([]*tickettypes.SelectedTicket{
					{
						Ticket: &tickettypes.Ticket{
							ProposerPubKey: key.PubKey().MustBytes32(),
							BLSPubKey:      key2.PrivKey().BLSKey().Public().Bytes(),
						},
					},
				}, nil)
				err = validation.CheckPushEndConsistency(&types.PushEndorsement{
					NoteID:         util.StrToBytes32("id"),
					EndorserPubKey: key.PubKey().MustBytes32(),
				}, mockLogic, false, -1)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("field:endorsements.sig, msg:signature could not be verified"))
			})
		})

		When("noSigCheck is true", func() {
			BeforeEach(func() {
				key := crypto.NewKeyFromIntSeed(1)
				key2 := crypto.NewKeyFromIntSeed(2)
				mockTickMgr.EXPECT().GetTopHosts(gomock.Any()).Return([]*tickettypes.SelectedTicket{
					{
						Ticket: &tickettypes.Ticket{
							ProposerPubKey: key.PubKey().MustBytes32(),
							BLSPubKey:      key2.PrivKey().BLSKey().Public().Bytes(),
						},
					},
				}, nil)
				err = validation.CheckPushEndConsistency(&types.PushEndorsement{
					NoteID:         util.StrToBytes32("id"),
					EndorserPubKey: key.PubKey().MustBytes32(),
				}, mockLogic, true, -1)
			})

			It("should not check signature", func() {
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".FetchAndCheckReferenceObjects", func() {
		When("object does not exist in the dht", func() {
			BeforeEach(func() {
				objHash := "obj_hash"

				tx := &types.PushNote{RepoName: "repo1", References: []*types.PushedReference{
					{Name: "refs/heads/master", Objects: []string{objHash}},
				}}

				mockRepo := mocks.NewMockLocalRepo(ctrl)
				mockRepo.EXPECT().GetObjectSize(objHash).Return(int64(0), fmt.Errorf("object not found"))
				tx.TargetRepo = mockRepo

				mockDHT := mocks.NewMockDHTNode(ctrl)
				dhtKey := plumbing2.MakeRepoObjectDHTKey(tx.GetRepoName(), objHash)
				mockDHT.EXPECT().GetObject(gomock.Any(), &dhttypes.DHTObjectQuery{
					Module:    core.RepoObjectModule,
					ObjectKey: []byte(dhtKey),
				}).Return(nil, fmt.Errorf("object not found"))

				err = validation.FetchAndCheckReferenceObjects(tx, mockDHT)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to fetch object 'obj_hash': object not found"))
			})
		})

		When("object exist in the dht but failed to write to repository", func() {
			BeforeEach(func() {
				objHash := "obj_hash"

				tx := &types.PushNote{RepoName: "repo1", References: []*types.PushedReference{
					{Name: "refs/heads/master", Objects: []string{objHash}},
				}}

				mockRepo := mocks.NewMockLocalRepo(ctrl)
				mockRepo.EXPECT().GetObjectSize(objHash).Return(int64(0), fmt.Errorf("object not found"))
				tx.TargetRepo = mockRepo

				mockDHT := mocks.NewMockDHTNode(ctrl)
				dhtKey := plumbing2.MakeRepoObjectDHTKey(tx.GetRepoName(), objHash)
				content := []byte("content")
				mockDHT.EXPECT().GetObject(gomock.Any(), &dhttypes.DHTObjectQuery{
					Module:    core.RepoObjectModule,
					ObjectKey: []byte(dhtKey),
				}).Return(content, nil)

				mockRepo.EXPECT().WriteObjectToFile(objHash, content).Return(fmt.Errorf("something bad"))

				err = validation.FetchAndCheckReferenceObjects(tx, mockDHT)
			})

			It("should return err", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to write fetched object 'obj_hash' to disk: something bad"))
			})
		})

		When("object exist in the dht and successfully written to disk", func() {
			BeforeEach(func() {
				objHash := "obj_hash"

				tx := &types.PushNote{RepoName: "repo1", References: []*types.PushedReference{
					{Name: "refs/heads/master", Objects: []string{objHash}},
				}, Size: 7}

				mockRepo := mocks.NewMockLocalRepo(ctrl)
				mockRepo.EXPECT().GetObjectSize(objHash).Return(int64(0), fmt.Errorf("object not found"))
				tx.TargetRepo = mockRepo

				mockDHT := mocks.NewMockDHTNode(ctrl)
				dhtKey := plumbing2.MakeRepoObjectDHTKey(tx.GetRepoName(), objHash)
				content := []byte("content")
				mockDHT.EXPECT().GetObject(gomock.Any(), &dhttypes.DHTObjectQuery{
					Module:    core.RepoObjectModule,
					ObjectKey: []byte(dhtKey),
				}).Return(content, nil)

				mockRepo.EXPECT().WriteObjectToFile(objHash, content).Return(nil)
				mockRepo.EXPECT().GetObjectSize(objHash).Return(int64(len(content)), nil)

				err = validation.FetchAndCheckReferenceObjects(tx, mockDHT)
			})

			It("should return no error", func() {
				Expect(err).To(BeNil())
			})
		})

		When("object exist in the dht, successfully written to disk and object size is different from actual size", func() {
			BeforeEach(func() {
				objHash := "obj_hash"

				tx := &types.PushNote{RepoName: "repo1", References: []*types.PushedReference{
					{Name: "refs/heads/master", Objects: []string{objHash}},
				}, Size: 10}

				mockRepo := mocks.NewMockLocalRepo(ctrl)
				mockRepo.EXPECT().GetObjectSize(objHash).Return(int64(0), fmt.Errorf("object not found"))
				tx.TargetRepo = mockRepo

				mockDHT := mocks.NewMockDHTNode(ctrl)
				dhtKey := plumbing2.MakeRepoObjectDHTKey(tx.GetRepoName(), objHash)
				content := []byte("content")
				mockDHT.EXPECT().GetObject(gomock.Any(), &dhttypes.DHTObjectQuery{
					Module:    core.RepoObjectModule,
					ObjectKey: []byte(dhtKey),
				}).Return(content, nil)

				mockRepo.EXPECT().WriteObjectToFile(objHash, content).Return(nil)
				mockRepo.EXPECT().GetObjectSize(objHash).Return(int64(len(content)), nil)

				err = validation.FetchAndCheckReferenceObjects(tx, mockDHT)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:size, msg:invalid size (10 bytes). actual object size (7 bytes) is different"))
			})
		})
	})

})
