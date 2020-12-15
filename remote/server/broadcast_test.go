package server

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/crypto/ed25519"
	"github.com/make-os/kit/mocks"
	"github.com/make-os/kit/net/dht/announcer"
	"github.com/make-os/kit/remote/push/types"
	"github.com/make-os/kit/remote/repo"
	testutil2 "github.com/make-os/kit/remote/testutil"
	"github.com/make-os/kit/testutil"
	tickettypes "github.com/make-os/kit/ticket/types"
	"github.com/make-os/kit/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Reactor", func() {
	var err error
	var cfg *config.AppConfig
	var svr *Server
	var ctrl *gomock.Controller
	var repoName, path string
	var mockTickMgr *mocks.MockTicketManager

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		cfg.Node.GitBinPath = "/usr/bin/git"
		ctrl = gomock.NewController(GinkgoT())

		repoName = util.RandString(5)
		path = filepath.Join(cfg.GetRepoRoot(), repoName)
		testutil2.ExecGit(cfg.GetRepoRoot(), "init", repoName)
		_, err := repo.GetWithLiteGit(cfg.Node.GitBinPath, path)
		Expect(err).To(BeNil())

		mockObjects := testutil.Mocks(ctrl)
		mockTickMgr = mockObjects.TicketManager

		mockDHT := mocks.NewMockDHT(ctrl)
		mockDHT.EXPECT().RegisterChecker(announcer.ObjTypeRepoName, gomock.Any())
		mockDHT.EXPECT().RegisterChecker(announcer.ObjTypeGit, gomock.Any())

		svr = New(cfg, ":9000", mockObjects.Logic, mockDHT, mocks.NewMockMempool(ctrl), mockObjects.Service, mocks.NewMockBlockGetter(ctrl))

	})

	AfterEach(func() {
		svr.Stop()
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".BroadcastNoteAndEndorsement", func() {
		It("should return error when unable to get top tickets", func() {
			svr.noteBroadcaster = func(pushNote types.PushNote) {}
			mockTickMgr.EXPECT().GetTopHosts(gomock.Any()).Return(nil, fmt.Errorf("error"))
			err := svr.BroadcastNoteAndEndorsement(&types.Note{})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to get top hosts: error"))
		})

		It("should return nil when no top selected tickets", func() {
			svr.noteBroadcaster = func(pushNote types.PushNote) {}
			tickets := tickettypes.SelectedTickets{}
			mockTickMgr.EXPECT().GetTopHosts(gomock.Any()).Return(tickets, nil)
			err := svr.BroadcastNoteAndEndorsement(&types.Note{})
			Expect(err).To(BeNil())
		})

		It("should return error when unable to create endorsement", func() {
			svr.noteBroadcaster = func(pushNote types.PushNote) {}
			ticket := &tickettypes.SelectedTicket{Ticket: &tickettypes.Ticket{
				ProposerPubKey: svr.validatorKey.PubKey().MustBytes32(),
			}}
			tickets := tickettypes.SelectedTickets{ticket}
			svr.endorsementCreator = func(validatorKey *ed25519.Key, note types.PushNote) (*types.PushEndorsement, error) {
				return nil, fmt.Errorf("error")
			}
			mockTickMgr.EXPECT().GetTopHosts(gomock.Any()).Return(tickets, nil)
			err := svr.BroadcastNoteAndEndorsement(&types.Note{})
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		When("endorsement is created successfully", func() {
			var endorsementBroadcast bool
			var madePushTx bool
			var note = &types.Note{RepoName: "repo1"}
			var end = &types.PushEndorsement{NoteID: []byte{1, 2, 3}}

			BeforeEach(func() {
				svr.noteBroadcaster = func(pushNote types.PushNote) {}
				ticket := &tickettypes.SelectedTicket{Ticket: &tickettypes.Ticket{
					ProposerPubKey: svr.validatorKey.PubKey().MustBytes32(),
				}}
				tickets := tickettypes.SelectedTickets{ticket}

				svr.endorsementCreator = func(validatorKey *ed25519.Key, note types.PushNote) (*types.PushEndorsement, error) {
					return end, nil
				}

				svr.endorsementBroadcaster = func(endorsement types.Endorsement) {
					endorsementBroadcast = true
				}

				svr.makePushTx = func(noteID string) error {
					madePushTx = true
					return nil
				}
				mockTickMgr.EXPECT().GetTopHosts(gomock.Any()).Return(tickets, nil)
				err := svr.BroadcastNoteAndEndorsement(note)
				Expect(err).To(BeNil())
			})

			It("should broadcast the endorsement", func() {
				Expect(endorsementBroadcast).To(BeTrue())
			})

			It("should make push transaction", func() {
				Expect(madePushTx).To(BeTrue())
			})

			It("should register endorsement to the push note", func() {
				noteEnds := svr.endorsements.Get(note.ID().String())
				Expect(noteEnds).To(HaveLen(1))
				Expect(noteEnds).To(HaveKey(end.ID().String()))
			})
		})
	})
})
