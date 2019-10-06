package ticket

import (
	"os"

	"github.com/golang/mock/gomock"
	"github.com/makeos/mosdef/types/mocks"

	"github.com/makeos/mosdef/util"

	"github.com/makeos/mosdef/params"
	"github.com/makeos/mosdef/types"

	l "github.com/makeos/mosdef/logic"

	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/storage/tree"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/testutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Manager", func() {
	var err error
	var c storage.Engine
	var cfg *config.EngineConfig
	var mgr *Manager
	var state *tree.SafeTree
	var logic *l.Logic
	var ctrl *gomock.Controller

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		c = storage.NewBadger(cfg)
		Expect(c.Init()).To(BeNil())
		db := storage.NewTMDBAdapter(c.F(true, true))
		state = tree.NewSafeTree(db, 128)
		logic = l.New(c, state, cfg)
		mgr, err = NewManager(cfg, logic)
		Expect(err).To(BeNil())
	})

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	AfterEach(func() {
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".GetByProposer", func() {
		ticket := &types.Ticket{ProposerPubKey: "pub_key"}
		BeforeEach(func() {
			err := mgr.store.Add(ticket)
			Expect(err).To(BeNil())
		})

		It("should return 1 ticket", func() {
			tickets, err := mgr.GetByProposer("pub_key", types.EmptyQueryOptions)
			Expect(err).To(BeNil())
			Expect(tickets).To(HaveLen(1))
			Expect(tickets[0]).To(Equal(ticket))
		})
	})

	Describe(".CountLiveTickets", func() {
		ticket := &types.Ticket{ProposerPubKey: "pub_key", MatureBy: 100, DecayBy: 200}
		ticket2 := &types.Ticket{ProposerPubKey: "pub_key", MatureBy: 100, DecayBy: 150}

		When("only one live ticket exist", func() {
			BeforeEach(func() {
				err := mgr.store.Add(ticket, ticket2)
				Expect(err).To(BeNil())
				mockSysKeeper := mocks.NewMockSystemKeeper(ctrl)
				mockLogic := mocks.NewMockLogic(ctrl)
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: 160}, nil)
				mockLogic.EXPECT().SysKeeper().Return(mockSysKeeper)
				mgr.logic = mockLogic
			})

			It("should return 1", func() {
				count, err := mgr.CountLiveTickets(types.EmptyQueryOptions)
				Expect(err).To(BeNil())
				Expect(count).To(Equal(1))
			})
		})

		When("no live ticket exist", func() {
			BeforeEach(func() {
				err := mgr.store.Add(ticket, ticket2)
				Expect(err).To(BeNil())
				mockSysKeeper := mocks.NewMockSystemKeeper(ctrl)
				mockLogic := mocks.NewMockLogic(ctrl)
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: 300}, nil)
				mockLogic.EXPECT().SysKeeper().Return(mockSysKeeper)
				mgr.logic = mockLogic
			})

			It("should return ticket1", func() {
				count, err := mgr.CountLiveTickets(types.EmptyQueryOptions)
				Expect(err).To(BeNil())
				Expect(count).To(Equal(0))
			})
		})
	})

	Describe(".Index", func() {
		When("ticket purchase value=10, current ticket price=10, ticket block height = 100, "+
			"params.MinTicketMatDur=60, params.MaxTicketActiveDur=40", func() {
			BeforeEach(func() {
				params.InitialTicketPrice = 10
				params.NumBlocksPerPriceWindow = 100
				params.PricePercentIncrease = 0.2
				params.MinTicketMatDur = 60
				params.MaxTicketActiveDur = 40
				err := logic.SysKeeper().SaveBlockInfo(&types.BlockInfo{Height: 2})
				Expect(err).To(BeNil())
				Expect(logic.Sys().GetCurValidatorTicketPrice()).To(Equal(float64(10)))
				tx := &types.Transaction{Hash: util.StrToHash("hash"), Value: util.String("10")}
				err = mgr.Index(tx, "validator_addr", 100, 1)
				Expect(err).To(BeNil())
			})

			Specify("that only one non-child ticket is indexed", func() {
				tickets, err := mgr.store.Query(types.Ticket{})
				Expect(err).To(BeNil())
				Expect(tickets).To(HaveLen(1))
				Expect(tickets[0].ChildOf).To(BeEmpty())
			})

			Specify("that MaturedBy is 160, DecayBy is 140", func() {
				tickets, err := mgr.store.Query(types.Ticket{})
				Expect(err).To(BeNil())
				Expect(tickets).To(HaveLen(1))
				Expect(tickets[0].DecayBy).To(Equal(uint64(200)))
			})
		})

		When("ticket purchase value=35, current ticket price=10, ticket block height = 100", func() {
			var tickets []*types.Ticket

			BeforeEach(func() {
				params.InitialTicketPrice = 10
				params.NumBlocksPerPriceWindow = 100
				params.PricePercentIncrease = 0.2
				err := logic.SysKeeper().SaveBlockInfo(&types.BlockInfo{Height: 2})
				Expect(err).To(BeNil())
				Expect(logic.Sys().GetCurValidatorTicketPrice()).To(Equal(float64(10)))
				tx := &types.Transaction{Hash: util.StrToHash("hash"), Value: util.String("35")}
				err = mgr.Index(tx, "validator_addr", 100, 1)
				Expect(err).To(BeNil())

				tickets, err = mgr.store.Query(types.Ticket{})
				Expect(err).To(BeNil())
			})

			Specify("that there are 3 tickets created", func() {
				Expect(tickets).To(HaveLen(3))
			})

			Specify("that one non-child and two child tickets are indexed", func() {
				Expect(tickets[0].ChildOf).To(BeEmpty())
				Expect(tickets[1].ChildOf).To(Equal(tickets[0].Hash))
				Expect(tickets[2].ChildOf).To(Equal(tickets[0].Hash))
			})

			Specify("that child tickets have increasing index", func() {
				Expect(tickets[1].Index).To(Equal(0))
				Expect(tickets[2].Index).To(Equal(1))
			})
		})
	})
})
