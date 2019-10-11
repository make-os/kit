package ticket

import (
	"fmt"
	"os"

	"github.com/makeos/mosdef/crypto"

	"github.com/golang/mock/gomock"
	ticketsmock "github.com/makeos/mosdef/ticket/mocks"
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
				tx := &types.Transaction{Value: util.String("10"), SenderPubKey: "pub_key"}
				err = mgr.Index(tx, 100, 1)
				Expect(err).To(BeNil())
			})

			Specify("that power is 0", func() {
				tickets, err := mgr.store.Query(types.Ticket{})
				Expect(err).To(BeNil())
				Expect(tickets).To(HaveLen(1))
				Expect(tickets[0].Power).To(Equal(int64(1)))
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
				tx := &types.Transaction{Value: util.String("35"), SenderPubKey: "pub_key"}
				err = mgr.Index(tx, 100, 1)
				Expect(err).To(BeNil())

				tickets, err = mgr.store.Query(types.Ticket{})
				Expect(err).To(BeNil())
			})

			Specify("only 1 ticket was created", func() {
				Expect(tickets).To(HaveLen(1))
			})

			Specify("that power is 3", func() {
				Expect(tickets[0].Power).To(Equal(int64(3)))
			})
		})

		When("tx.To is set", func() {
			var tickets []*types.Ticket
			var tx *types.Transaction
			var proposer = crypto.NewKeyFromIntSeed(2)
			var delegator = crypto.NewKeyFromIntSeed(3)

			BeforeEach(func() {
				params.InitialTicketPrice = 10
				params.NumBlocksPerPriceWindow = 100
				params.PricePercentIncrease = 0.2
				err := logic.SysKeeper().SaveBlockInfo(&types.BlockInfo{Height: 2})
				Expect(err).To(BeNil())
				Expect(logic.Sys().GetCurValidatorTicketPrice()).To(Equal(float64(10)))
				tx = &types.Transaction{
					Value:        util.String("35"),
					SenderPubKey: util.String(delegator.PubKey().Base58()),
					To:           util.String(proposer.PubKey().Base58()),
				}
				err = mgr.Index(tx, 100, 1)
				Expect(err).To(BeNil())

				tickets, err = mgr.store.Query(types.Ticket{})
				Expect(err).To(BeNil())
			})

			Specify("only 1 ticket was created", func() {
				Expect(tickets).To(HaveLen(1))
			})

			Specify("that delegator is set to the address of the sender", func() {
				Expect(tickets[0].Delegator).To(Equal(delegator.Addr().String()))
			})

			Specify("that proposer public key is set to the value of tx.To", func() {
				Expect(tickets[0].ProposerPubKey).To(Equal(tx.To.String()))
			})
		})
	})

	Describe(".SelectRandom", func() {

		When("err occurred when fetching live tickets", func() {
			BeforeEach(func() {
				mockStore := ticketsmock.NewMockStore(ctrl)
				mockStore.EXPECT().GetLive(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("bad error"))
				mgr.store = mockStore
			})

			It("should return error", func() {
				seed := []byte("seed")
				_, err := mgr.SelectRandom(11, seed, 1)
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(Equal("bad error"))
			})
		})

		When("no error occurred", func() {
			ticket := &types.Ticket{ProposerPubKey: "pub_key1", Height: 2, Index: 2, MatureBy: 10, DecayBy: 100, Power: 3}
			ticket2 := &types.Ticket{ProposerPubKey: "pub_key2", Height: 2, Index: 1, MatureBy: 10, DecayBy: 100, Power: 4}
			ticket3 := &types.Ticket{ProposerPubKey: "pub_key3", Height: 1, Index: 1, MatureBy: 10, DecayBy: 100, Power: 1}
			BeforeEach(func() {
				err := mgr.store.Add(ticket, ticket2, ticket3)
				Expect(err).To(BeNil())
			})

			When("seed=[]byte('seed') and limit = 1", func() {
				It("should return 1 ticket", func() {
					seed := []byte("seed")
					tickets, err := mgr.SelectRandom(11, seed, 1)
					Expect(err).To(BeNil())
					Expect(tickets).To(HaveLen(1))
					Expect(tickets[0].ProposerPubKey).To(Equal(ticket.ProposerPubKey))
				})
			})

			When("seed=[]byte('seed_123_abc') and limit = 1", func() {
				It("should return 1 ticket", func() {
					seed := []byte("seed_123_abc")
					tickets, err := mgr.SelectRandom(11, seed, 1)
					Expect(err).To(BeNil())
					Expect(tickets).To(HaveLen(1))
					Expect(tickets[0].ProposerPubKey).To(Equal(ticket2.ProposerPubKey))
				})
			})

			When("seed=[]byte('seed_123_abc') and limit = 10", func() {
				It("should return 3 ticket", func() {
					seed := []byte("seed_123_abc")
					tickets, err := mgr.SelectRandom(11, seed, 10)
					Expect(err).To(BeNil())
					Expect(tickets).To(HaveLen(3))
				})
			})
		})

		When("multiple tickets of same proposer public key are pre-selected (before random selection)", func() {
			ticket := &types.Ticket{ProposerPubKey: "pub_key1", Height: 2, Index: 2, MatureBy: 10, DecayBy: 100, Power: 3}
			ticket2 := &types.Ticket{ProposerPubKey: "pub_key1", Height: 2, Index: 1, MatureBy: 10, DecayBy: 100, Power: 4}
			ticket3 := &types.Ticket{ProposerPubKey: "pub_key3", Height: 1, Index: 1, MatureBy: 10, DecayBy: 100, Power: 1}

			BeforeEach(func() {
				err := mgr.store.Add(ticket, ticket2, ticket3)
				Expect(err).To(BeNil())
			})

			When("seed=[]byte('seed') and limit = 10", func() {
				It("should return 2 ticket with different proposer pub key", func() {
					seed := []byte("seed")
					tickets, err := mgr.SelectRandom(11, seed, 10)
					Expect(err).To(BeNil())
					Expect(tickets).To(HaveLen(2))
					Expect(tickets[0].ProposerPubKey).To(Equal("pub_key1"))
					Expect(tickets[1].ProposerPubKey).To(Equal("pub_key3"))
				})
			})
		})
	})
})
