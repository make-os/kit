package ticket

import (
	"os"

	"github.com/golang/mock/gomock"

	"github.com/makeos/mosdef/crypto"
	l "github.com/makeos/mosdef/logic"
	"github.com/makeos/mosdef/params"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/types/mocks"
	"github.com/makeos/mosdef/util"

	"github.com/makeos/mosdef/storage"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/testutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Manager", func() {
	var err error
	var appDB, stateTreeDB storage.Engine
	var cfg *config.AppConfig
	var mgr *Manager
	var logic *l.Logic
	var ctrl *gomock.Controller
	var key = crypto.NewKeyFromIntSeed(1)
	var mockSysKeeper *mocks.MockSystemKeeper
	var mockLogic *mocks.MockLogic

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		appDB, stateTreeDB = testutil.GetDB(cfg)
		logic = l.New(appDB, stateTreeDB, cfg)
		mgr = NewManager(appDB.NewTx(true, true), cfg, logic)
		mockObjects := testutil.MockLogic(ctrl)
		mockLogic = mockObjects.Logic
		mockSysKeeper = mockObjects.SysKeeper
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".GetByProposer", func() {
		When("matching ticket exist", func() {
			ticket := &types.Ticket{ProposerPubKey: util.StrToBytes32("pub_key"), Type: types.TxTypeValidatorTicket}
			BeforeEach(func() {
				err := mgr.s.Add(ticket)
				Expect(err).To(BeNil())
			})

			It("should return 1 ticket", func() {
				tickets, err := mgr.GetByProposer(types.TxTypeValidatorTicket, util.StrToBytes32("pub_key"))
				Expect(err).To(BeNil())
				Expect(tickets).To(HaveLen(1))
				Expect(tickets[0]).To(Equal(ticket))
			})
		})

		When("matching ticket does not exist", func() {
			ticket := &types.Ticket{ProposerPubKey: util.StrToBytes32("pub_key"), Type: types.TxTypeValidatorTicket}
			BeforeEach(func() {
				err := mgr.s.Add(ticket)
				Expect(err).To(BeNil())
			})

			It("should return 0 ticket", func() {
				tickets, err := mgr.GetByProposer(types.TxTypeCoinTransfer, util.StrToBytes32("pub_key"))
				Expect(err).To(BeNil())
				Expect(tickets).To(HaveLen(0))
			})
		})
	})

	Describe(".CountActiveValidatorTickets", func() {
		ticket := &types.Ticket{Hash: util.StrToBytes32("h1"), Type: types.TxTypeValidatorTicket, ProposerPubKey: util.StrToBytes32("pub_key"), MatureBy: 100, DecayBy: 200}
		ticket2 := &types.Ticket{Hash: util.StrToBytes32("h2"), Type: types.TxTypeValidatorTicket, ProposerPubKey: util.StrToBytes32("pub_key"), MatureBy: 100, DecayBy: 150}

		When("only one live ticket exist", func() {
			BeforeEach(func() {
				err := mgr.s.Add(ticket, ticket2)
				Expect(err).To(BeNil())
				mgr.logic = mockLogic
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: 160}, nil)
			})

			It("should return 1", func() {
				count, err := mgr.CountActiveValidatorTickets()
				Expect(err).To(BeNil())
				Expect(count).To(Equal(1))
			})
		})

		When("no live ticket exist", func() {
			BeforeEach(func() {
				err := mgr.s.Add(ticket, ticket2)
				Expect(err).To(BeNil())
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: 300}, nil)
				mgr.logic = mockLogic
			})

			It("should return ticket1", func() {
				count, err := mgr.CountActiveValidatorTickets()
				Expect(err).To(BeNil())
				Expect(count).To(Equal(0))
			})
		})
	})

	Describe(".Index", func() {
		When("no ticket currently exist", func() {
			When("a ticket is indexed", func() {
				BeforeEach(func() {
					params.MinTicketMatDur = 60
					params.MaxTicketActiveDur = 40
					tx := types.NewBaseTx(types.TxTypeValidatorTicket, 1, "", key, "10", "1", 0)
					err = mgr.Index(tx, 100, 1)
					Expect(err).To(BeNil())
				})

				Specify("that a ticket was indexed", func() {
					tickets := mgr.s.Query(func(*types.Ticket) bool { return true })
					Expect(err).To(BeNil())
					Expect(tickets).To(HaveLen(1))
				})

				Specify("that the ticket's matureBy=160 and decayBy=200", func() {
					tickets := mgr.s.Query(func(*types.Ticket) bool { return true })
					Expect(err).To(BeNil())
					Expect(tickets).To(HaveLen(1))
					Expect(tickets[0].MatureBy).To(Equal(uint64(160)))
					Expect(tickets[0].DecayBy).To(Equal(uint64(200)))
				})
			})

			When("a storer ticket is indexed", func() {
				BeforeEach(func() {
					params.MinTicketMatDur = 60
					params.MaxTicketActiveDur = 40
					tx := types.NewBaseTx(types.TxTypeStorerTicket, 1, "", key, "10", "1", 0)
					err = mgr.Index(tx, 100, 1)
					Expect(err).To(BeNil())
				})

				Specify("that a ticket was indexed", func() {
					tickets := mgr.s.Query(func(*types.Ticket) bool { return true })
					Expect(err).To(BeNil())
					Expect(tickets).To(HaveLen(1))
				})

				Specify("that the ticket's matureBy=160 and decayBy=0", func() {
					tickets := mgr.s.Query(func(*types.Ticket) bool { return true })
					Expect(err).To(BeNil())
					Expect(tickets).To(HaveLen(1))
					Expect(tickets[0].MatureBy).To(Equal(uint64(160)))
					Expect(tickets[0].DecayBy).To(Equal(uint64(0)))
				})
			})
		})

		When("tx.Delegate is set  - delegated ticket", func() {
			var tickets []*types.Ticket
			var tx types.BaseTx
			var proposer = crypto.NewKeyFromIntSeed(2)
			var delegator = crypto.NewKeyFromIntSeed(3)

			BeforeEach(func() {
				txn := types.NewBareTxTicketPurchase(types.TxTypeValidatorTicket)
				txn.Value = util.String("35")
				txn.SenderPubKey = util.BytesToBytes32(delegator.PubKey().MustBytes())
				txn.Delegate = proposer.PubKey().MustBytes32()
				tx = txn
				err = mgr.Index(tx, 100, 1)
				Expect(err).To(BeNil())

				tickets = mgr.s.Query(func(*types.Ticket) bool { return true })
			})

			Specify("only 1 ticket was created", func() {
				Expect(tickets).To(HaveLen(1))
			})

			Specify("that delegator is set to the address of the sender", func() {
				Expect(tickets[0].Delegator).To(Equal(delegator.Addr().String()))
			})

			Specify("that proposer public key is set to the value of tx.Delegate", func() {
				Expect(tickets[0].ProposerPubKey).To(Equal(tx.(*types.TxTicketPurchase).Delegate))
			})
		})

		When("tx.Delegate is set and the proposer's commission rate is 50", func() {
			var tickets []*types.Ticket
			var tx types.BaseTx
			var proposer = crypto.NewKeyFromIntSeed(2)
			var delegator = crypto.NewKeyFromIntSeed(3)

			BeforeEach(func() {
				logic.AccountKeeper().Update(proposer.Addr(), &types.Account{
					Balance:             util.String("1000"),
					Stakes:              types.BareAccountStakes(),
					DelegatorCommission: 50,
				})
			})

			BeforeEach(func() {
				txn := types.NewBareTxTicketPurchase(types.TxTypeValidatorTicket)
				txn.Value = util.String("35")
				txn.SenderPubKey = util.BytesToBytes32(delegator.PubKey().MustBytes())
				txn.Delegate = proposer.PubKey().MustBytes32()
				tx = txn
				err = mgr.Index(tx, 100, 1)
				Expect(err).To(BeNil())
				tickets = mgr.s.Query(func(*types.Ticket) bool { return true })
			})

			Specify("that the ticket has a commission rate of 50", func() {
				Expect(tickets[0].CommissionRate).To(Equal(float64(50)))
			})
		})
	})

	Describe(".Remove", func() {
		var tickets []*types.Ticket
		var tx types.BaseTx

		When("one ticket exist", func() {
			BeforeEach(func() {
				txn := types.NewBareTxTicketPurchase(types.TxTypeValidatorTicket)
				txn.Value = util.String("35")
				txn.SenderPubKey = util.StrToBytes32("pub_key")
				tx = txn
				err = mgr.Index(tx, 100, 1)
				Expect(err).To(BeNil())
			})

			It("should remove ticket", func() {
				tickets = mgr.s.Query(func(*types.Ticket) bool { return true })
				Expect(tickets).To(HaveLen(1))
				err = mgr.Remove(tickets[0].Hash)
				Expect(err).To(BeNil())
				tickets = mgr.s.Query(func(*types.Ticket) bool { return true })
				Expect(tickets).To(HaveLen(0))
			})
		})
	})

	Describe(".UpdateDecayBy", func() {
		var tx types.BaseTx
		var tickets []*types.Ticket

		When("one ticket exist", func() {
			BeforeEach(func() {
				params.MinTicketMatDur = 60
				params.MaxTicketActiveDur = 40
				txn := types.NewBareTxTicketPurchase(types.TxTypeValidatorTicket)
				txn.Value = util.String("35")
				txn.SenderPubKey = util.StrToBytes32("pub_key")
				tx = txn
				err = mgr.Index(tx, 100, 1)
				Expect(err).To(BeNil())
				tickets = mgr.s.Query(func(*types.Ticket) bool { return true })
				Expect(tickets).To(HaveLen(1))
			})

			It("should update decayBy to 260", func() {
				Expect(mgr.UpdateDecayBy(tickets[0].Hash, 260)).To(BeNil())
				tickets = mgr.s.Query(func(*types.Ticket) bool { return true })
				Expect(tickets).To(HaveLen(1))
				Expect(tickets[0].DecayBy).To(Equal(uint64(260)))
			})
		})
	})

	Describe(".GetOrderedLiveValidatorTickets", func() {
		Context("vector 1 - highest value ordered in descending order", func() {
			var tickets []*types.Ticket

			BeforeEach(func() {
				ticket := &types.Ticket{Hash: util.StrToBytes32("h1"), Type: types.TxTypeValidatorTicket, ProposerPubKey: util.StrToBytes32("pub_key1"), Height: 3, Index: 2, MatureBy: 10, DecayBy: 100, Value: "3"}
				ticket2 := &types.Ticket{Hash: util.StrToBytes32("h2"), Type: types.TxTypeValidatorTicket, ProposerPubKey: util.StrToBytes32("pub_key2"), Height: 3, Index: 1, MatureBy: 10, DecayBy: 100, Value: "4"}
				ticket3 := &types.Ticket{Hash: util.StrToBytes32("h3"), Type: types.TxTypeValidatorTicket, ProposerPubKey: util.StrToBytes32("pub_key3"), Height: 1, Index: 1, MatureBy: 10, DecayBy: 100, Value: "1"}
				Expect(mgr.s.Add(ticket, ticket2, ticket3)).To(BeNil())
				tickets = mgr.GetOrderedLiveValidatorTickets(11, 0)
			})

			Specify("that ticket order should be ", func() {
				Expect(tickets).To(HaveLen(3))
				Expect(tickets[0].Hash).To(Equal(util.StrToBytes32("h2")))
				Expect(tickets[1].Hash).To(Equal(util.StrToBytes32("h1")))
				Expect(tickets[2].Hash).To(Equal(util.StrToBytes32("h3")))
			})
		})

		Context("vector 2 - height ordered in ascending order", func() {
			var tickets []*types.Ticket

			BeforeEach(func() {
				ticket := &types.Ticket{Hash: util.StrToBytes32("h1"), Type: types.TxTypeValidatorTicket, ProposerPubKey: util.StrToBytes32("pub_key1"), Height: 2, Index: 2, MatureBy: 10, DecayBy: 100, Value: "3"}
				ticket2 := &types.Ticket{Hash: util.StrToBytes32("h2"), Type: types.TxTypeValidatorTicket, ProposerPubKey: util.StrToBytes32("pub_key2"), Height: 4, Index: 1, MatureBy: 10, DecayBy: 100, Value: "3"}
				ticket3 := &types.Ticket{Hash: util.StrToBytes32("h3"), Type: types.TxTypeValidatorTicket, ProposerPubKey: util.StrToBytes32("pub_key3"), Height: 1, Index: 1, MatureBy: 10, DecayBy: 100, Value: "1"}
				Expect(mgr.s.Add(ticket, ticket2, ticket3)).To(BeNil())
				tickets = mgr.GetOrderedLiveValidatorTickets(11, 0)
			})

			Specify("that ticket order should be ", func() {
				Expect(tickets).To(HaveLen(3))
				Expect(tickets[0].Hash).To(Equal(util.StrToBytes32("h1")))
				Expect(tickets[1].Hash).To(Equal(util.StrToBytes32("h2")))
				Expect(tickets[2].Hash).To(Equal(util.StrToBytes32("h3")))
			})
		})

		Context("vector 3 - index ordered in ascending order", func() {
			var tickets []*types.Ticket

			BeforeEach(func() {
				ticket := &types.Ticket{Hash: util.StrToBytes32("h1"), Type: types.TxTypeValidatorTicket, ProposerPubKey: util.StrToBytes32("pub_key1"), Height: 2, Index: 2, MatureBy: 10, DecayBy: 100, Value: "3"}
				ticket2 := &types.Ticket{Hash: util.StrToBytes32("h2"), Type: types.TxTypeValidatorTicket, ProposerPubKey: util.StrToBytes32("pub_key2"), Height: 2, Index: 1, MatureBy: 10, DecayBy: 100, Value: "3"}
				ticket3 := &types.Ticket{Hash: util.StrToBytes32("h3"), Type: types.TxTypeValidatorTicket, ProposerPubKey: util.StrToBytes32("pub_key3"), Height: 1, Index: 1, MatureBy: 10, DecayBy: 100, Value: "1"}
				Expect(mgr.s.Add(ticket, ticket2, ticket3)).To(BeNil())
				tickets = mgr.GetOrderedLiveValidatorTickets(11, 0)
			})

			Specify("that ticket order should be ", func() {
				Expect(tickets).To(HaveLen(3))
				Expect(tickets[0].Hash).To(Equal(util.StrToBytes32("h2")))
				Expect(tickets[1].Hash).To(Equal(util.StrToBytes32("h1")))
				Expect(tickets[2].Hash).To(Equal(util.StrToBytes32("h3")))
			})
		})

		Context("with limit", func() {
			var tickets []*types.Ticket

			BeforeEach(func() {
				ticket := &types.Ticket{Hash: util.StrToBytes32("h1"), Type: types.TxTypeValidatorTicket, ProposerPubKey: util.StrToBytes32("pub_key1"), Height: 2, Index: 2, MatureBy: 10, DecayBy: 100, Value: "3"}
				ticket2 := &types.Ticket{Hash: util.StrToBytes32("h2"), Type: types.TxTypeValidatorTicket, ProposerPubKey: util.StrToBytes32("pub_key2"), Height: 2, Index: 1, MatureBy: 10, DecayBy: 100, Value: "3"}
				ticket3 := &types.Ticket{Hash: util.StrToBytes32("h3"), Type: types.TxTypeValidatorTicket, ProposerPubKey: util.StrToBytes32("pub_key3"), Height: 1, Index: 1, MatureBy: 10, DecayBy: 100, Value: "1"}
				Expect(mgr.s.Add(ticket, ticket2, ticket3)).To(BeNil())
				tickets = mgr.GetOrderedLiveValidatorTickets(11, 1)
			})

			Specify("that ticket order should be ", func() {
				Expect(tickets).To(HaveLen(1))
			})
		})
	})

	Describe(".GetByHash", func() {
		When("one ticket exist", func() {
			ticket := &types.Ticket{Hash: util.StrToBytes32("h1"), Type: types.TxTypeValidatorTicket, ProposerPubKey: util.StrToBytes32("pub_key1"), Height: 2, Index: 2, MatureBy: 10, DecayBy: 100, Value: "3"}
			BeforeEach(func() {
				err := mgr.s.Add(ticket)
				Expect(err).To(BeNil())
			})

			It("should find ticket by hash", func() {
				t := mgr.GetByHash(ticket.Hash)
				Expect(t).ToNot(BeNil())
				Expect(t.Hash).To(Equal(ticket.Hash))
			})
		})

		When("no ticket exist", func() {
			It("should find no ticket by hash", func() {
				t := mgr.GetByHash(util.StrToBytes32("h1"))
				Expect(t).To(BeNil())
			})
		})
	})

	Describe(".GetTopStorers", func() {
		When("proposer (pub_key1) has 1 self-owned ticket (value=3) and 1 delegated ticket (value=1) and proposer (pub_key2) has 1 delegated ticket (value=10)", func() {
			ticket := &types.Ticket{Hash: util.StrToBytes32("h1"), Type: types.TxTypeStorerTicket, ProposerPubKey: util.StrToBytes32("pub_key1"), Height: 2, Index: 2, MatureBy: 10, DecayBy: 100, Value: "3"}
			ticket2 := &types.Ticket{Hash: util.StrToBytes32("h2"), Type: types.TxTypeStorerTicket, Delegator: "addr", ProposerPubKey: util.StrToBytes32("pub_key1"), Height: 1, Index: 1, MatureBy: 10, DecayBy: 100, Value: "1"}
			ticket3 := &types.Ticket{Hash: util.StrToBytes32("h3"), Type: types.TxTypeStorerTicket, Delegator: "addr", ProposerPubKey: util.StrToBytes32("pub_key2"), Height: 1, Index: 1, MatureBy: 10, DecayBy: 100, Value: "10"}

			BeforeEach(func() {
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: 11}, nil)
				mgr.logic = mockLogic
				err := mgr.s.Add(ticket, ticket2, ticket3)
				Expect(err).To(BeNil())
			})

			It("should return two tickets in the order; pub_key2 (value=10) and pub_key1 (value=4)", func() {
				res, err := mgr.GetTopStorers(0)
				Expect(err).To(BeNil())
				Expect(res).To(HaveLen(2))
				Expect(res[0].Ticket.ProposerPubKey).To(Equal(util.StrToBytes32("pub_key2")))
				Expect(res[0].TotalValue.String()).To(Equal("10"))
				Expect(res[1].Ticket.ProposerPubKey).To(Equal(util.StrToBytes32("pub_key1")))
				Expect(res[1].TotalValue.String()).To(Equal("4"))
				Expect(res.Has(util.StrToBytes32("pub_key2"))).To(BeTrue())
				Expect(res.Has(util.StrToBytes32("pub_key1"))).To(BeTrue())
			})

			When("limit is 1", func() {
				It("should return 1 ticket in the order; pub_key2 (value=10)", func() {
					res, err := mgr.GetTopStorers(1)
					Expect(err).To(BeNil())
					Expect(res).To(HaveLen(1))
					Expect(res[0].Ticket.ProposerPubKey).To(Equal(util.StrToBytes32("pub_key2")))
					Expect(res.Has(util.StrToBytes32("pub_key2"))).To(BeTrue())
					Expect(res.Has(util.StrToBytes32("pub_key1"))).To(BeFalse())
					Expect(res[0].TotalValue.String()).To(Equal("10"))
				})
			})
		})
	})

	Describe(".GetActiveTicketsByProposer", func() {
		ticket := &types.Ticket{Hash: util.StrToBytes32("h1"), Type: types.TxTypeValidatorTicket, ProposerPubKey: util.StrToBytes32("pub_key1"), Height: 2, Index: 2, MatureBy: 10, DecayBy: 100, Value: "3"}
		ticket2 := &types.Ticket{Hash: util.StrToBytes32("h2"), Type: types.TxTypeValidatorTicket, ProposerPubKey: util.StrToBytes32("pub_key2"), Height: 2, Index: 1, MatureBy: 10, DecayBy: 100, Value: "4"}
		ticket3 := &types.Ticket{Hash: util.StrToBytes32("h3"), Type: types.TxTypeValidatorTicket, ProposerPubKey: util.StrToBytes32("pub_key3"), Height: 1, Index: 1, MatureBy: 10, DecayBy: 100, Value: "1"}
		ticket3_2 := &types.Ticket{Hash: util.StrToBytes32("h3_2"), Type: types.TxTypeValidatorTicket, ProposerPubKey: util.StrToBytes32("pub_key3"), Height: 1, Index: 1, MatureBy: 10, DecayBy: 100, Value: "1"}
		ticket3_3 := &types.Ticket{Hash: util.StrToBytes32("h3_3"), Type: types.TxTypeStorerTicket, ProposerPubKey: util.StrToBytes32("pub_key3"), Height: 1, Index: 1, MatureBy: 10, DecayBy: 100, Value: "1"}
		ticket3_4 := &types.Ticket{Hash: util.StrToBytes32("h3_4"), Type: types.TxTypeStorerTicket, Delegator: "addr", ProposerPubKey: util.StrToBytes32("pub_key3"), Height: 1, Index: 1, MatureBy: 10, DecayBy: 100, Value: "1"}
		ticket4 := &types.Ticket{Hash: util.StrToBytes32("h3_4"), Type: types.TxTypeStorerTicket, Delegator: "addr", ProposerPubKey: util.StrToBytes32("pub_key4"), Height: 1, Index: 1, MatureBy: 10, DecayBy: 0, Value: "1"}

		When("proposer='pub_key3', type=TxTypeValidatorTicket, addDelegated=false", func() {
			BeforeEach(func() {
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: 11}, nil)
				mgr.logic = mockLogic

				err := mgr.s.Add(ticket, ticket2, ticket3, ticket3_2, ticket3_4)
				Expect(err).To(BeNil())
			})

			It("should return 2 tickets", func() {
				res, err := mgr.GetActiveTicketsByProposer(util.StrToBytes32("pub_key3"), types.TxTypeValidatorTicket, false)
				Expect(err).To(BeNil())
				Expect(res).To(HaveLen(2))
			})
		})

		When("proposer='pub_key3', type=TxTypeStorerTicket, addDelegated=false", func() {
			BeforeEach(func() {
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: 11}, nil)
				mgr.logic = mockLogic

				err := mgr.s.Add(ticket, ticket2, ticket3, ticket3_2, ticket3_3)
				Expect(err).To(BeNil())
			})

			It("should return 1 tickets", func() {
				res, err := mgr.GetActiveTicketsByProposer(util.StrToBytes32("pub_key3"), types.TxTypeStorerTicket, false)
				Expect(err).To(BeNil())
				Expect(res).To(HaveLen(1))
			})
		})

		When("proposer='pub_key3', type=TxTypeStorerTicket, addDelegated=true", func() {
			BeforeEach(func() {
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: 11}, nil)
				mgr.logic = mockLogic

				err := mgr.s.Add(ticket, ticket2, ticket3, ticket3_2, ticket3_3, ticket3_4)
				Expect(err).To(BeNil())
			})

			It("should return 2 tickets", func() {
				res, err := mgr.GetActiveTicketsByProposer(util.StrToBytes32("pub_key3"), types.TxTypeStorerTicket, true)
				Expect(err).To(BeNil())
				Expect(res).To(HaveLen(2))
			})
		})

		When("ticket decay height = 0", func() {
			When("args are proposer='pub_key4', type=TxTypeStorerTicket, addDelegated=true", func() {
				BeforeEach(func() {
					mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: 11}, nil)
					mgr.logic = mockLogic

					err := mgr.s.Add(ticket, ticket2, ticket3, ticket3_2, ticket3_3, ticket4)
					Expect(err).To(BeNil())
				})

				It("should return 2 tickets", func() {
					res, err := mgr.GetActiveTicketsByProposer(util.StrToBytes32("pub_key4"), types.TxTypeStorerTicket, true)
					Expect(err).To(BeNil())
					Expect(res).To(HaveLen(1))
				})
			})
		})
	})

	Describe(".SelectRandomValidatorTickets", func() {

		ticket := &types.Ticket{Type: types.TxTypeValidatorTicket, ProposerPubKey: util.StrToBytes32("pub_key1"), Height: 2, Index: 2, MatureBy: 10, DecayBy: 100, Value: "3"}
		ticket2 := &types.Ticket{Type: types.TxTypeValidatorTicket, ProposerPubKey: util.StrToBytes32("pub_key2"), Height: 2, Index: 1, MatureBy: 10, DecayBy: 100, Value: "4"}
		ticket3 := &types.Ticket{Type: types.TxTypeValidatorTicket, ProposerPubKey: util.StrToBytes32("pub_key3"), Height: 1, Index: 1, MatureBy: 10, DecayBy: 100, Value: "1"}
		BeforeEach(func() {
			err := mgr.s.Add(ticket, ticket2, ticket3)
			Expect(err).To(BeNil())
		})

		When("seed=[]byte('seed') and limit = 1", func() {
			It("should return 1 ticket", func() {
				seed := []byte("seed")
				tickets, err := mgr.SelectRandomValidatorTickets(11, seed, 1)
				Expect(err).To(BeNil())
				Expect(tickets).To(HaveLen(1))
				Expect(tickets[0].ProposerPubKey).To(Equal(ticket.ProposerPubKey))
			})
		})

		When("seed=[]byte('seed_123_abc') and limit = 1", func() {
			It("should return 1 ticket", func() {
				seed := []byte("seed_123_abc")
				tickets, err := mgr.SelectRandomValidatorTickets(11, seed, 1)
				Expect(err).To(BeNil())
				Expect(tickets).To(HaveLen(1))
				Expect(tickets[0].ProposerPubKey).To(Equal(ticket2.ProposerPubKey))
			})
		})

		When("seed=[]byte('seed_123_abc') and limit = 10", func() {
			It("should return 3 ticket", func() {
				seed := []byte("seed_123_abc")
				tickets, err := mgr.SelectRandomValidatorTickets(11, seed, 10)
				Expect(err).To(BeNil())
				Expect(tickets).To(HaveLen(3))
			})
		})

		When("multiple tickets of same proposer public key are pre-selected (before random selection)", func() {
			ticket := &types.Ticket{Type: types.TxTypeValidatorTicket, ProposerPubKey: util.StrToBytes32("pub_key1"), Height: 2, Index: 2, MatureBy: 10, DecayBy: 100, Value: "3"}
			ticket2 := &types.Ticket{Type: types.TxTypeValidatorTicket, ProposerPubKey: util.StrToBytes32("pub_key1"), Height: 2, Index: 1, MatureBy: 10, DecayBy: 100, Value: "4"}
			ticket3 := &types.Ticket{Type: types.TxTypeValidatorTicket, ProposerPubKey: util.StrToBytes32("pub_key3"), Height: 1, Index: 1, MatureBy: 10, DecayBy: 100, Value: "1"}

			BeforeEach(func() {
				err := mgr.s.Add(ticket, ticket2, ticket3)
				Expect(err).To(BeNil())
			})

			When("seed=[]byte('seed') and limit = 10", func() {
				It("should return 2 ticket with different proposer pub key", func() {
					seed := []byte("seed")
					tickets, err := mgr.SelectRandomValidatorTickets(11, seed, 10)
					Expect(err).To(BeNil())
					Expect(tickets).To(HaveLen(2))
					Expect(tickets[0].ProposerPubKey).To(Equal(util.StrToBytes32("pub_key1")))
					Expect(tickets[1].ProposerPubKey).To(Equal(util.StrToBytes32("pub_key3")))
				})
			})
		})
	})
})
