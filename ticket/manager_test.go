package ticket

import (
	"os"

	tickettypes "github.com/make-os/lobe/ticket/types"
	"github.com/make-os/lobe/types"
	"github.com/make-os/lobe/types/core"
	"github.com/make-os/lobe/types/state"
	"github.com/make-os/lobe/types/txns"

	"github.com/golang/mock/gomock"

	"github.com/make-os/lobe/crypto"
	l "github.com/make-os/lobe/logic"
	"github.com/make-os/lobe/mocks"
	"github.com/make-os/lobe/params"
	"github.com/make-os/lobe/util"

	"github.com/make-os/lobe/storage"

	"github.com/make-os/lobe/config"
	"github.com/make-os/lobe/testutil"
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
	var key2 = crypto.NewKeyFromIntSeed(2)
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
		When("ticket of matching type exist", func() {
			ticket := &tickettypes.Ticket{ProposerPubKey: crypto.StrToPublicKey("pub_key").ToBytes32(), Type: txns.TxTypeValidatorTicket}
			BeforeEach(func() {
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&core.BlockInfo{Height: 1}, nil)
				mgr.logic = mockLogic
				err := mgr.s.Add(ticket)
				Expect(err).To(BeNil())
			})

			It("should return 1 ticket", func() {
				tickets, err := mgr.GetByProposer(txns.TxTypeValidatorTicket, crypto.StrToPublicKey("pub_key").ToBytes32())
				Expect(err).To(BeNil())
				Expect(tickets).To(HaveLen(1))
				Expect(tickets[0]).To(Equal(ticket))
			})
		})

		When("matching unable to find ticket with matching type", func() {
			ticket := &tickettypes.Ticket{ProposerPubKey: crypto.StrToPublicKey("pub_key").ToBytes32(), Type: txns.TxTypeValidatorTicket}
			BeforeEach(func() {
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&core.BlockInfo{Height: 1}, nil)
				mgr.logic = mockLogic
				err := mgr.s.Add(ticket)
				Expect(err).To(BeNil())
			})

			It("should return 0 ticket", func() {
				tickets, err := mgr.GetByProposer(1000, crypto.StrToPublicKey("pub_key").ToBytes32())
				Expect(err).To(BeNil())
				Expect(tickets).To(HaveLen(0))
			})
		})

		When("with query options", func() {
			ticket := &tickettypes.Ticket{
				ProposerPubKey: crypto.StrToPublicKey("pub_key").ToBytes32(),
				Hash:           util.HexBytes(util.RandBytes(32)),
				Type:           txns.TxTypeValidatorTicket,
				MatureBy:       50,
				ExpireBy:       1000,
			}
			ticketB := &tickettypes.Ticket{
				ProposerPubKey: crypto.StrToPublicKey("pub_key").ToBytes32(),
				Hash:           util.HexBytes(util.RandBytes(32)),
				Type:           txns.TxTypeValidatorTicket,
				MatureBy:       101,
				ExpireBy:       1000,
			}
			ticketC := &tickettypes.Ticket{
				ProposerPubKey: crypto.StrToPublicKey("pub_key").ToBytes32(),
				Hash:           util.HexBytes(util.RandBytes(32)),
				Type:           txns.TxTypeValidatorTicket,
				MatureBy:       50,
				ExpireBy:       1000,
			}
			ticketD := &tickettypes.Ticket{
				ProposerPubKey: crypto.StrToPublicKey("pub_key").ToBytes32(),
				Hash:           util.HexBytes(util.RandBytes(32)),
				Type:           txns.TxTypeValidatorTicket,
				MatureBy:       101,
				ExpireBy:       10,
			}

			When("immature=true", func() {
				BeforeEach(func() {
					mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&core.BlockInfo{Height: 100}, nil)
					mgr.logic = mockLogic
					err := mgr.s.Add(ticket, ticketB)
					Expect(err).To(BeNil())
				})

				Specify("that only immature tickets are returned", func() {
					tickets, err := mgr.GetByProposer(txns.TxTypeValidatorTicket, crypto.StrToPublicKey("pub_key").ToBytes32(), tickettypes.QueryOptions{
						Immature: true,
					})
					Expect(err).To(BeNil())
					Expect(tickets).To(HaveLen(1))
					Expect(tickets[0]).To(Equal(ticketB))
				})
			})

			When("immature=false", func() {
				BeforeEach(func() {
					mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&core.BlockInfo{Height: 100}, nil)
					mgr.logic = mockLogic
					err := mgr.s.Add(ticket, ticketB)
					Expect(err).To(BeNil())
				})

				Specify("that mature and immature tickets are returned", func() {
					tickets, err := mgr.GetByProposer(txns.TxTypeValidatorTicket, util.StrToBytes32("pub_key"), tickettypes.QueryOptions{
						Immature: false,
					})
					Expect(err).To(BeNil())
					Expect(tickets).To(HaveLen(2))
				})
			})

			When("mature=true", func() {
				BeforeEach(func() {
					mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&core.BlockInfo{Height: 100}, nil)
					mgr.logic = mockLogic
					err := mgr.s.Add(ticket, ticketB)
					Expect(err).To(BeNil())
				})

				Specify("that only mature tickets are returned", func() {
					tickets, err := mgr.GetByProposer(txns.TxTypeValidatorTicket, util.StrToBytes32("pub_key"), tickettypes.QueryOptions{
						Matured: true,
					})
					Expect(err).To(BeNil())
					Expect(tickets).To(HaveLen(1))
					Expect(tickets[0]).To(Equal(ticket))
				})
			})

			When("mature=false", func() {
				BeforeEach(func() {
					mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&core.BlockInfo{Height: 100}, nil)
					mgr.logic = mockLogic
					err := mgr.s.Add(ticket, ticketB)
					Expect(err).To(BeNil())
				})

				Specify("that mature and immature tickets are returned", func() {
					tickets, err := mgr.GetByProposer(txns.TxTypeValidatorTicket, util.StrToBytes32("pub_key"), tickettypes.QueryOptions{
						Matured: false,
					})
					Expect(err).To(BeNil())
					Expect(tickets).To(HaveLen(2))
				})
			})

			When("expired=true", func() {
				BeforeEach(func() {
					mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&core.BlockInfo{Height: 100}, nil)
					mgr.logic = mockLogic
					err := mgr.s.Add(ticketC, ticketD)
					Expect(err).To(BeNil())
				})

				Specify("that only expired tickets are returned", func() {
					tickets, err := mgr.GetByProposer(txns.TxTypeValidatorTicket, util.StrToBytes32("pub_key"), tickettypes.QueryOptions{
						Expired: true,
					})
					Expect(err).To(BeNil())
					Expect(tickets).To(HaveLen(1))
					Expect(tickets[0]).To(Equal(ticketD))
				})
			})

			When("expired=false", func() {
				BeforeEach(func() {
					mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&core.BlockInfo{Height: 100}, nil)
					mgr.logic = mockLogic
					err := mgr.s.Add(ticketC, ticketD)
					Expect(err).To(BeNil())
				})

				Specify("that expired and unexpired tickets are returned", func() {
					tickets, err := mgr.GetByProposer(txns.TxTypeValidatorTicket, util.StrToBytes32("pub_key"), tickettypes.QueryOptions{
						Expired: false,
					})
					Expect(err).To(BeNil())
					Expect(tickets).To(HaveLen(2))
				})
			})

			When("active=true", func() {
				BeforeEach(func() {
					mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&core.BlockInfo{Height: 100}, nil)
					mgr.logic = mockLogic
					err := mgr.s.Add(ticketC, ticketD)
					Expect(err).To(BeNil())
				})

				Specify("that only unexpired tickets are returned", func() {
					tickets, err := mgr.GetByProposer(txns.TxTypeValidatorTicket, util.StrToBytes32("pub_key"), tickettypes.QueryOptions{
						Active: true,
					})
					Expect(err).To(BeNil())
					Expect(tickets).To(HaveLen(1))
					Expect(tickets[0]).To(Equal(ticketC))
				})
			})

			When("active=false", func() {
				BeforeEach(func() {
					mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&core.BlockInfo{Height: 100}, nil)
					mgr.logic = mockLogic
					err := mgr.s.Add(ticketC, ticketD)
					Expect(err).To(BeNil())
				})

				Specify("that expired and unexpired tickets are returned", func() {
					tickets, err := mgr.GetByProposer(txns.TxTypeValidatorTicket, util.StrToBytes32("pub_key"), tickettypes.QueryOptions{
						Active: false,
					})
					Expect(err).To(BeNil())
					Expect(tickets).To(HaveLen(2))
				})
			})
		})

	})

	Describe(".CountActiveValidatorTickets", func() {
		ticket := &tickettypes.Ticket{Hash: util.StrToHexBytes("h1"), Type: txns.TxTypeValidatorTicket, ProposerPubKey: crypto.StrToPublicKey("pub_key").ToBytes32(), MatureBy: 100, ExpireBy: 200}
		ticket2 := &tickettypes.Ticket{Hash: util.StrToHexBytes("h2"), Type: txns.TxTypeValidatorTicket, ProposerPubKey: crypto.StrToPublicKey("pub_key").ToBytes32(), MatureBy: 100, ExpireBy: 150}

		When("only one live ticket exist", func() {
			BeforeEach(func() {
				err := mgr.s.Add(ticket, ticket2)
				Expect(err).To(BeNil())
				mgr.logic = mockLogic
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&core.BlockInfo{Height: 160}, nil)
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
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&core.BlockInfo{Height: 300}, nil)
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
					tx := txns.NewBareTxTicketPurchase(txns.TxTypeValidatorTicket)
					err = mgr.Index(tx, 100, 1)
					Expect(err).To(BeNil())
				})

				Specify("that a ticket was indexed", func() {
					tickets := mgr.s.Query(func(*tickettypes.Ticket) bool { return true })
					Expect(err).To(BeNil())
					Expect(tickets).To(HaveLen(1))
				})

				Specify("that the ticket's matureBy=160 and expireBy=200", func() {
					tickets := mgr.s.Query(func(*tickettypes.Ticket) bool { return true })
					Expect(err).To(BeNil())
					Expect(tickets).To(HaveLen(1))
					Expect(tickets[0].MatureBy).To(Equal(uint64(160)))
					Expect(tickets[0].ExpireBy).To(Equal(uint64(200)))
				})
			})

			When("a host ticket is indexed", func() {
				BeforeEach(func() {
					params.MinTicketMatDur = 60
					params.MaxTicketActiveDur = 40
					tx := txns.NewBareTxTicketPurchase(txns.TxTypeHostTicket)
					err = mgr.Index(tx, 100, 1)
					Expect(err).To(BeNil())
				})

				Specify("that a ticket was indexed", func() {
					tickets := mgr.s.Query(func(*tickettypes.Ticket) bool { return true })
					Expect(err).To(BeNil())
					Expect(tickets).To(HaveLen(1))
				})

				Specify("that the ticket's matureBy=160 and expireBy=0", func() {
					tickets := mgr.s.Query(func(*tickettypes.Ticket) bool { return true })
					Expect(err).To(BeNil())
					Expect(tickets).To(HaveLen(1))
					Expect(tickets[0].MatureBy).To(Equal(uint64(160)))
					Expect(tickets[0].ExpireBy).To(Equal(uint64(0)))
				})
			})
		})

		When("tx.Delegate is set  - delegated ticket", func() {
			var tickets []*tickettypes.Ticket
			var tx types.BaseTx
			var proposer = crypto.NewKeyFromIntSeed(2)
			var delegator = crypto.NewKeyFromIntSeed(3)

			BeforeEach(func() {
				txn := txns.NewBareTxTicketPurchase(txns.TxTypeValidatorTicket)
				txn.Value = "35"
				txn.SenderPubKey = crypto.BytesToPublicKey(delegator.PubKey().MustBytes())
				txn.Delegate = crypto.BytesToPublicKey(proposer.PubKey().MustBytes())
				tx = txn
				err = mgr.Index(tx, 100, 1)
				Expect(err).To(BeNil())

				tickets = mgr.s.Query(func(*tickettypes.Ticket) bool { return true })
			})

			Specify("only 1 ticket was created", func() {
				Expect(tickets).To(HaveLen(1))
			})

			Specify("that delegator is set to the address of the sender", func() {
				Expect(tickets[0].Delegator).To(Equal(delegator.Addr().String()))
			})

			Specify("that proposer public key is set to the value of tx.Delegate", func() {
				Expect(tickets[0].ProposerPubKey).To(Equal(tx.(*txns.TxTicketPurchase).Delegate.ToBytes32()))
			})
		})

		When("tx.Delegate is set and the proposer's commission rate is 50", func() {
			var tickets []*tickettypes.Ticket
			var tx types.BaseTx
			var proposer = crypto.NewKeyFromIntSeed(2)
			var delegator = crypto.NewKeyFromIntSeed(3)

			BeforeEach(func() {
				logic.AccountKeeper().Update(proposer.Addr(), &state.Account{
					Balance:             "1000",
					Stakes:              state.BareAccountStakes(),
					DelegatorCommission: 50,
				})
			})

			BeforeEach(func() {
				txn := txns.NewBareTxTicketPurchase(txns.TxTypeValidatorTicket)
				txn.Value = "35"
				txn.SenderPubKey = crypto.BytesToPublicKey(delegator.PubKey().MustBytes())
				txn.Delegate = crypto.BytesToPublicKey(proposer.PubKey().MustBytes())
				tx = txn
				err = mgr.Index(tx, 100, 1)
				Expect(err).To(BeNil())
				tickets = mgr.s.Query(func(*tickettypes.Ticket) bool { return true })
			})

			Specify("that the ticket has a commission rate of 50", func() {
				Expect(tickets[0].CommissionRate).To(Equal(float64(50)))
			})
		})
	})

	Describe(".Remove", func() {
		var tickets []*tickettypes.Ticket
		var tx types.BaseTx

		When("one ticket exist", func() {
			BeforeEach(func() {
				txn := txns.NewBareTxTicketPurchase(txns.TxTypeValidatorTicket)
				txn.Value = "35"
				txn.SenderPubKey = crypto.StrToPublicKey("pub_key")
				tx = txn
				err = mgr.Index(tx, 100, 1)
				Expect(err).To(BeNil())
			})

			It("should remove ticket", func() {
				tickets = mgr.s.Query(func(*tickettypes.Ticket) bool { return true })
				Expect(tickets).To(HaveLen(1))
				err = mgr.Remove(tickets[0].Hash)
				Expect(err).To(BeNil())
				tickets = mgr.s.Query(func(*tickettypes.Ticket) bool { return true })
				Expect(tickets).To(HaveLen(0))
			})
		})
	})

	Describe(".UpdateExpireBy", func() {
		var tx types.BaseTx
		var tickets []*tickettypes.Ticket

		When("one ticket exist", func() {
			BeforeEach(func() {
				params.MinTicketMatDur = 60
				params.MaxTicketActiveDur = 40
				txn := txns.NewBareTxTicketPurchase(txns.TxTypeValidatorTicket)
				txn.Value = "35"
				txn.SenderPubKey = crypto.StrToPublicKey("pub_key")
				tx = txn
				err = mgr.Index(tx, 100, 1)
				Expect(err).To(BeNil())
				tickets = mgr.s.Query(func(*tickettypes.Ticket) bool { return true })
				Expect(tickets).To(HaveLen(1))
			})

			It("should update expireBy to 260", func() {
				Expect(mgr.UpdateExpireBy(tickets[0].Hash, 260)).To(BeNil())
				tickets = mgr.s.Query(func(*tickettypes.Ticket) bool { return true })
				Expect(tickets).To(HaveLen(1))
				Expect(tickets[0].ExpireBy).To(Equal(uint64(260)))
			})
		})
	})

	Describe(".GetByHash", func() {
		When("one ticket exist", func() {
			ticket := &tickettypes.Ticket{Hash: util.StrToHexBytes("h1"), Type: txns.TxTypeValidatorTicket, ProposerPubKey: util.StrToBytes32("pub_key1"), Height: 2, Index: 2, MatureBy: 10, ExpireBy: 100, Value: "3"}
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
				t := mgr.GetByHash(util.StrToHexBytes("h1"))
				Expect(t).To(BeNil())
			})
		})
	})

	Describe(".getTopTickets", func() {
		When("proposer (pub_key1) has 1 self-owned ticket (value=3) and 1 delegated ticket (value=1) and proposer (pub_key2) has 1 delegated ticket (value=10)", func() {
			ticket := &tickettypes.Ticket{Hash: util.StrToHexBytes("h1"), Type: txns.TxTypeHostTicket, ProposerPubKey: util.StrToBytes32("pub_key1"), Height: 2, Index: 2, MatureBy: 10, ExpireBy: 100, Value: "3"}
			ticket2 := &tickettypes.Ticket{Hash: util.StrToHexBytes("h2"), Type: txns.TxTypeHostTicket, Delegator: "addr", ProposerPubKey: util.StrToBytes32("pub_key1"), Height: 1, Index: 1, MatureBy: 10, ExpireBy: 100, Value: "1"}
			ticket3 := &tickettypes.Ticket{Hash: util.StrToHexBytes("h3"), Type: txns.TxTypeHostTicket, Delegator: "addr", ProposerPubKey: util.StrToBytes32("pub_key2"), Height: 1, Index: 1, MatureBy: 10, ExpireBy: 100, Value: "10"}

			BeforeEach(func() {
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&core.BlockInfo{Height: 11}, nil)
				mgr.logic = mockLogic
				err := mgr.s.Add(ticket, ticket2, ticket3)
				Expect(err).To(BeNil())
			})

			It("should return two tickets in the order; pub_key2 (value=10) and pub_key1 (value=4)", func() {
				res, err := mgr.getTopTickets(txns.TxTypeHostTicket, 0)
				Expect(err).To(BeNil())
				Expect(res).To(HaveLen(2))
				Expect(res[0].Ticket.ProposerPubKey).To(Equal(util.StrToBytes32("pub_key2")))
				Expect(res[0].Power.String()).To(Equal("10"))
				Expect(res[1].Ticket.ProposerPubKey).To(Equal(util.StrToBytes32("pub_key1")))
				Expect(res[1].Power.String()).To(Equal("4"))
				Expect(res.Has(util.StrToBytes32("pub_key2"))).To(BeTrue())
				Expect(res.Has(util.StrToBytes32("pub_key1"))).To(BeTrue())
			})

			When("limit is 1", func() {
				It("should return 1 ticket in the order; pub_key2 (value=10)", func() {
					res, err := mgr.getTopTickets(txns.TxTypeHostTicket, 1)
					Expect(err).To(BeNil())
					Expect(res).To(HaveLen(1))
					Expect(res[0].Ticket.ProposerPubKey).To(Equal(util.StrToBytes32("pub_key2")))
					Expect(res.Has(util.StrToBytes32("pub_key2"))).To(BeTrue())
					Expect(res.Has(util.StrToBytes32("pub_key1"))).To(BeFalse())
					Expect(res[0].Power.String()).To(Equal("10"))
				})
			})
		})
	})

	Describe(".GetNonDelegatedTickets", func() {
		ticket := &tickettypes.Ticket{Hash: util.StrToHexBytes("h1"), Type: txns.TxTypeValidatorTicket, ProposerPubKey: util.StrToBytes32("pub_key1"), Height: 2, Index: 2, MatureBy: 10, ExpireBy: 100, Value: "3"}
		ticket2 := &tickettypes.Ticket{Hash: util.StrToHexBytes("h2"), Type: txns.TxTypeValidatorTicket, ProposerPubKey: util.StrToBytes32("pub_key2"), Height: 2, Index: 1, MatureBy: 10, ExpireBy: 100, Value: "4"}
		ticket3 := &tickettypes.Ticket{Hash: util.StrToHexBytes("h3"), Type: txns.TxTypeValidatorTicket, ProposerPubKey: util.StrToBytes32("pub_key3"), Height: 1, Index: 1, MatureBy: 10, ExpireBy: 100, Value: "1"}
		ticket3_2 := &tickettypes.Ticket{Hash: util.StrToHexBytes("h3_2"), Type: txns.TxTypeValidatorTicket, ProposerPubKey: util.StrToBytes32("pub_key3"), Height: 1, Index: 1, MatureBy: 10, ExpireBy: 100, Value: "1"}
		ticket3_3 := &tickettypes.Ticket{Hash: util.StrToHexBytes("h3_3"), Type: txns.TxTypeHostTicket, ProposerPubKey: util.StrToBytes32("pub_key3"), Height: 1, Index: 1, MatureBy: 10, ExpireBy: 100, Value: "1"}
		ticket3_4 := &tickettypes.Ticket{Hash: util.StrToHexBytes("h3_4"), Type: txns.TxTypeHostTicket, Delegator: "addr", ProposerPubKey: util.StrToBytes32("pub_key3"), Height: 1, Index: 1, MatureBy: 10, ExpireBy: 100, Value: "1"}

		When("proposer='pub_key3', type=TxTypeValidatorTicket, addDelegated=false", func() {
			BeforeEach(func() {
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&core.BlockInfo{Height: 11}, nil)
				mgr.logic = mockLogic

				err := mgr.s.Add(ticket, ticket2, ticket3, ticket3_2, ticket3_4)
				Expect(err).To(BeNil())
			})

			It("should return 2 tickets", func() {
				res, err := mgr.GetNonDelegatedTickets(util.StrToBytes32("pub_key3"), txns.TxTypeValidatorTicket)
				Expect(err).To(BeNil())
				Expect(res).To(HaveLen(2))
			})
		})

		When("proposer='pub_key3', type=TxTypeHostTicket, addDelegated=false", func() {
			BeforeEach(func() {
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&core.BlockInfo{Height: 11}, nil)
				mgr.logic = mockLogic

				err := mgr.s.Add(ticket, ticket2, ticket3, ticket3_2, ticket3_3)
				Expect(err).To(BeNil())
			})

			It("should return 1 tickets", func() {
				res, err := mgr.GetNonDelegatedTickets(util.StrToBytes32("pub_key3"), txns.TxTypeHostTicket)
				Expect(err).To(BeNil())
				Expect(res).To(HaveLen(1))
			})
		})
	})

	Describe(".ValueOfTickets", func() {
		When("pubkey is proposer of a ticket with value=3 and delegator of a ticket with value=4", func() {
			ticket := &tickettypes.Ticket{Hash: util.StrToHexBytes("h1"), Type: txns.TxTypeValidatorTicket, ProposerPubKey: key.PubKey().MustBytes32(), Height: 2, Index: 2, MatureBy: 10, ExpireBy: 100, Value: "3"}
			ticket2 := &tickettypes.Ticket{Hash: util.StrToHexBytes("h2"), Type: txns.TxTypeHostTicket, ProposerPubKey: key2.PubKey().MustBytes32(), Delegator: key.Addr().String(), Height: 2, Index: 1, MatureBy: 10, ExpireBy: 100, Value: "4"}
			BeforeEach(func() {
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&core.BlockInfo{Height: 11}, nil)
				mgr.logic = mockLogic
				err := mgr.s.Add(ticket, ticket2)
				Expect(err).To(BeNil())
			})

			It("should return sum=7", func() {
				val, err := mgr.ValueOfTickets(key.PubKey().MustBytes32(), 0)
				Expect(err).To(BeNil())
				Expect(val).To(Equal(float64(7)))
			})
		})

		When("pubkey is proposer of a ticket with value=3", func() {
			ticket := &tickettypes.Ticket{Hash: util.StrToHexBytes("h1"), Type: txns.TxTypeValidatorTicket, ProposerPubKey: key.PubKey().MustBytes32(), Height: 2, Index: 2, MatureBy: 10, ExpireBy: 100, Value: "3"}
			ticket2 := &tickettypes.Ticket{Hash: util.StrToHexBytes("h2"), Type: txns.TxTypeHostTicket, ProposerPubKey: key2.PubKey().MustBytes32(), Height: 2, Index: 1, MatureBy: 10, ExpireBy: 100, Value: "4"}
			BeforeEach(func() {
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&core.BlockInfo{Height: 11}, nil)
				mgr.logic = mockLogic
				err := mgr.s.Add(ticket, ticket2)
				Expect(err).To(BeNil())
			})

			It("should return sum=3", func() {
				val, err := mgr.ValueOfTickets(key.PubKey().MustBytes32(), 0)
				Expect(err).To(BeNil())
				Expect(val).To(Equal(float64(3)))
			})
		})

		When("maturity height is 5", func() {
			When("pubkey is proposer of a ticket with value=3", func() {
				ticket := &tickettypes.Ticket{Hash: util.StrToHexBytes("h1"), Type: txns.TxTypeValidatorTicket, ProposerPubKey: key.PubKey().MustBytes32(), Height: 2, Index: 2, MatureBy: 10, ExpireBy: 100, Value: "3"}
				ticket2 := &tickettypes.Ticket{Hash: util.StrToHexBytes("h2"), Type: txns.TxTypeHostTicket, ProposerPubKey: key2.PubKey().MustBytes32(), Height: 2, Index: 1, MatureBy: 10, ExpireBy: 100, Value: "4"}
				BeforeEach(func() {
					mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&core.BlockInfo{Height: 11}, nil)
					mgr.logic = mockLogic
					err := mgr.s.Add(ticket, ticket2)
					Expect(err).To(BeNil())
				})

				It("should return 0", func() {
					val, err := mgr.ValueOfTickets(key.PubKey().MustBytes32(), 5)
					Expect(err).To(BeNil())
					Expect(val).To(Equal(float64(0)))
				})
			})
		})
	})

	Describe(".ValueOfAllTickets", func() {
		When("there are two tickets of value 3 and 4 respectively", func() {
			ticket := &tickettypes.Ticket{Hash: util.StrToHexBytes("h1"), Type: txns.TxTypeValidatorTicket, ProposerPubKey: key.PubKey().MustBytes32(), Height: 2, Index: 2, MatureBy: 10, ExpireBy: 100, Value: "3"}
			ticket2 := &tickettypes.Ticket{Hash: util.StrToHexBytes("h2"), Type: txns.TxTypeHostTicket, ProposerPubKey: key2.PubKey().MustBytes32(), Delegator: key.Addr().String(), Height: 2, Index: 1, MatureBy: 10, ExpireBy: 100, Value: "4"}
			BeforeEach(func() {
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&core.BlockInfo{Height: 11}, nil)
				mgr.logic = mockLogic
				err := mgr.s.Add(ticket, ticket2)
				Expect(err).To(BeNil())
			})

			It("should return sum=7", func() {
				val, err := mgr.ValueOfAllTickets(0)
				Expect(err).To(BeNil())
				Expect(val).To(Equal(float64(7)))
			})
		})

		When("maturity height is 5", func() {
			When("there are two tickets of value 3 and 4 respectively", func() {
				ticket := &tickettypes.Ticket{Hash: util.StrToHexBytes("h1"), Type: txns.TxTypeValidatorTicket, ProposerPubKey: key.PubKey().MustBytes32(), Height: 2, Index: 2, MatureBy: 10, ExpireBy: 100, Value: "3"}
				ticket2 := &tickettypes.Ticket{Hash: util.StrToHexBytes("h2"), Type: txns.TxTypeHostTicket, ProposerPubKey: key2.PubKey().MustBytes32(), Height: 2, Index: 1, MatureBy: 10, ExpireBy: 100, Value: "4"}
				BeforeEach(func() {
					mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&core.BlockInfo{Height: 11}, nil)
					mgr.logic = mockLogic
					err := mgr.s.Add(ticket, ticket2)
					Expect(err).To(BeNil())
				})

				It("should return 0", func() {
					val, err := mgr.ValueOfAllTickets(5)
					Expect(err).To(BeNil())
					Expect(val).To(Equal(float64(0)))
				})
			})
		})
	})

	Describe(".ValueOfNonDelegatedTickets", func() {
		When("pubkey is proposer of a ticket with value=3 and delegator of a ticket with value=4", func() {
			ticket := &tickettypes.Ticket{Hash: util.StrToHexBytes("h1"), Type: txns.TxTypeValidatorTicket, ProposerPubKey: key.PubKey().MustBytes32(), Height: 2, Index: 2, MatureBy: 10, ExpireBy: 100, Value: "3"}
			ticket2 := &tickettypes.Ticket{Hash: util.StrToHexBytes("h2"), Type: txns.TxTypeHostTicket, ProposerPubKey: key2.PubKey().MustBytes32(), Delegator: key.Addr().String(), Height: 2, Index: 1, MatureBy: 10, ExpireBy: 100, Value: "4"}
			BeforeEach(func() {
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&core.BlockInfo{Height: 11}, nil)
				mgr.logic = mockLogic
				err := mgr.s.Add(ticket, ticket2)
				Expect(err).To(BeNil())
			})

			It("should return sum=3", func() {
				val, err := mgr.ValueOfNonDelegatedTickets(key.PubKey().MustBytes32(), 0)
				Expect(err).To(BeNil())
				Expect(val).To(Equal(float64(3)))
			})
		})

		When("pubkey is proposer of non-delegated tickets with values=3,4", func() {
			ticket := &tickettypes.Ticket{Hash: util.StrToHexBytes("h1"), Type: txns.TxTypeValidatorTicket, ProposerPubKey: key.PubKey().MustBytes32(), Height: 2, Index: 2, MatureBy: 10, ExpireBy: 100, Value: "3"}
			ticket2 := &tickettypes.Ticket{Hash: util.StrToHexBytes("h2"), Type: txns.TxTypeHostTicket, ProposerPubKey: key.PubKey().MustBytes32(), Height: 2, Index: 1, MatureBy: 10, ExpireBy: 100, Value: "4"}
			BeforeEach(func() {
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&core.BlockInfo{Height: 11}, nil)
				mgr.logic = mockLogic
				err := mgr.s.Add(ticket, ticket2)
				Expect(err).To(BeNil())
			})

			It("should return sum=7", func() {
				val, err := mgr.ValueOfNonDelegatedTickets(key.PubKey().MustBytes32(), 0)
				Expect(err).To(BeNil())
				Expect(val).To(Equal(float64(7)))
			})
		})

		When("maturity height is 5", func() {
			When("pubkey is proposer of non-delegated tickets with values=3,4", func() {
				ticket := &tickettypes.Ticket{Hash: util.StrToHexBytes("h1"), Type: txns.TxTypeValidatorTicket, ProposerPubKey: key.PubKey().MustBytes32(), Height: 2, Index: 2, MatureBy: 10, ExpireBy: 100, Value: "3"}
				ticket2 := &tickettypes.Ticket{Hash: util.StrToHexBytes("h2"), Type: txns.TxTypeHostTicket, ProposerPubKey: key.PubKey().MustBytes32(), Height: 2, Index: 1, MatureBy: 10, ExpireBy: 100, Value: "4"}
				BeforeEach(func() {
					mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&core.BlockInfo{Height: 11}, nil)
					mgr.logic = mockLogic
					err := mgr.s.Add(ticket, ticket2)
					Expect(err).To(BeNil())
				})

				It("should return 0", func() {
					val, err := mgr.ValueOfNonDelegatedTickets(key.PubKey().MustBytes32(), 5)
					Expect(err).To(BeNil())
					Expect(val).To(Equal(float64(0)))
				})
			})
		})
	})

	Describe(".ValueOfDelegatedTickets", func() {
		When("pubkey is proposer of a tickets A with value=3 and B with value 4; B is delegated", func() {
			ticketA := &tickettypes.Ticket{Hash: util.StrToHexBytes("h1"), Type: txns.TxTypeValidatorTicket, ProposerPubKey: key.PubKey().MustBytes32(), Height: 2, Index: 2, MatureBy: 10, ExpireBy: 100, Value: "3"}
			ticketB := &tickettypes.Ticket{Hash: util.StrToHexBytes("h2"), Type: txns.TxTypeHostTicket, ProposerPubKey: key.PubKey().MustBytes32(), Delegator: key2.Addr().String(), Height: 2, Index: 1, MatureBy: 10, ExpireBy: 100, Value: "4"}
			BeforeEach(func() {
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&core.BlockInfo{Height: 11}, nil)
				mgr.logic = mockLogic
				err := mgr.s.Add(ticketA, ticketB)
				Expect(err).To(BeNil())
			})

			It("should return sum=3", func() {
				val, err := mgr.ValueOfDelegatedTickets(key.PubKey().MustBytes32(), 0)
				Expect(err).To(BeNil())
				Expect(val).To(Equal(float64(4)))
			})
		})

		When("pubkey is proposer of a tickets A with value=3 and B with value 4; A and B are delegated", func() {
			ticket := &tickettypes.Ticket{Hash: util.StrToHexBytes("h1"), Type: txns.TxTypeValidatorTicket, ProposerPubKey: key.PubKey().MustBytes32(), Delegator: key2.Addr().String(), Height: 2, Index: 2, MatureBy: 10, ExpireBy: 100, Value: "3"}
			ticket2 := &tickettypes.Ticket{Hash: util.StrToHexBytes("h2"), Type: txns.TxTypeHostTicket, ProposerPubKey: key.PubKey().MustBytes32(), Delegator: key2.Addr().String(), Height: 2, Index: 1, MatureBy: 10, ExpireBy: 100, Value: "4"}
			BeforeEach(func() {
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&core.BlockInfo{Height: 11}, nil)
				mgr.logic = mockLogic
				err := mgr.s.Add(ticket, ticket2)
				Expect(err).To(BeNil())
			})

			It("should return sum=7", func() {
				val, err := mgr.ValueOfDelegatedTickets(key.PubKey().MustBytes32(), 0)
				Expect(err).To(BeNil())
				Expect(val).To(Equal(float64(7)))
			})
		})

		When("maturity height is 5", func() {
			When("pubkey is proposer of a tickets A with value=3 and B with value 4; A and B are delegated", func() {
				ticket := &tickettypes.Ticket{Hash: util.StrToHexBytes("h1"), Type: txns.TxTypeValidatorTicket, ProposerPubKey: key.PubKey().MustBytes32(), Delegator: key2.Addr().String(), Height: 2, Index: 2, MatureBy: 10, ExpireBy: 100, Value: "3"}
				ticket2 := &tickettypes.Ticket{Hash: util.StrToHexBytes("h2"), Type: txns.TxTypeHostTicket, ProposerPubKey: key.PubKey().MustBytes32(), Delegator: key2.Addr().String(), Height: 2, Index: 1, MatureBy: 10, ExpireBy: 100, Value: "4"}
				BeforeEach(func() {
					mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&core.BlockInfo{Height: 11}, nil)
					mgr.logic = mockLogic
					err := mgr.s.Add(ticket, ticket2)
					Expect(err).To(BeNil())
				})

				It("should return 0", func() {
					val, err := mgr.ValueOfDelegatedTickets(key.PubKey().MustBytes32(), 5)
					Expect(err).To(BeNil())
					Expect(val).To(Equal(float64(0)))
				})
			})
		})
	})

	Describe(".GetUnExpiredTickets", func() {
		When("pubkey is proposer of a tickets A with value=3 and B with value 4; B is expired", func() {
			ticketA := &tickettypes.Ticket{Hash: util.StrToHexBytes("h1"), Type: txns.TxTypeValidatorTicket, ProposerPubKey: key.PubKey().MustBytes32(), Height: 2, Index: 2, MatureBy: 10, ExpireBy: 100, Value: "3"}
			ticketB := &tickettypes.Ticket{Hash: util.StrToHexBytes("h2"), Type: txns.TxTypeHostTicket, ProposerPubKey: key.PubKey().MustBytes32(), Delegator: key2.Addr().String(), Height: 2, Index: 1, MatureBy: 10, ExpireBy: 1, Value: "4"}
			BeforeEach(func() {
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&core.BlockInfo{Height: 11}, nil)
				mgr.logic = mockLogic
				err := mgr.s.Add(ticketA, ticketB)
				Expect(err).To(BeNil())
			})

			It("should return length = 1", func() {
				res, err := mgr.GetUnExpiredTickets(key.PubKey().MustBytes32(), 0)
				Expect(err).To(BeNil())
				Expect(len(res)).To(Equal(1))
			})
		})

		When("pubkey is proposer of a tickets A with value=3 and B with value 4; B is delegated", func() {
			ticket := &tickettypes.Ticket{Hash: util.StrToHexBytes("h1"), Type: txns.TxTypeValidatorTicket, ProposerPubKey: key.PubKey().MustBytes32(), Delegator: key.Addr().String(), Height: 2, Index: 2, MatureBy: 10, ExpireBy: 100, Value: "3"}
			ticket2 := &tickettypes.Ticket{Hash: util.StrToHexBytes("h2"), Type: txns.TxTypeHostTicket, ProposerPubKey: key.PubKey().MustBytes32(), Delegator: key2.Addr().String(), Height: 2, Index: 1, MatureBy: 10, ExpireBy: 100, Value: "4"}
			BeforeEach(func() {
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&core.BlockInfo{Height: 11}, nil)
				mgr.logic = mockLogic
				err := mgr.s.Add(ticket, ticket2)
				Expect(err).To(BeNil())
			})

			It("should return length = 2", func() {
				res, err := mgr.GetUnExpiredTickets(key.PubKey().MustBytes32(), 0)
				Expect(err).To(BeNil())
				Expect(len(res)).To(Equal(2))
			})
		})

		When("maturityHeight is set to non-zero", func() {
			When("pubkey is proposer of a tickets A with value=3 and B with value 4; B is delegated", func() {
				ticket := &tickettypes.Ticket{Hash: util.StrToHexBytes("h1"), Type: txns.TxTypeValidatorTicket, ProposerPubKey: key.PubKey().MustBytes32(), Delegator: key.Addr().String(), Height: 2, Index: 2, MatureBy: 10, ExpireBy: 100, Value: "3"}
				ticket2 := &tickettypes.Ticket{Hash: util.StrToHexBytes("h2"), Type: txns.TxTypeHostTicket, ProposerPubKey: key.PubKey().MustBytes32(), Delegator: key2.Addr().String(), Height: 2, Index: 1, MatureBy: 10, ExpireBy: 100, Value: "4"}
				BeforeEach(func() {
					mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&core.BlockInfo{Height: 11}, nil)
					mgr.logic = mockLogic
					err := mgr.s.Add(ticket, ticket2)
					Expect(err).To(BeNil())
				})

				It("should return length = 0", func() {
					res, err := mgr.GetUnExpiredTickets(key.PubKey().MustBytes32(), 5)
					Expect(err).To(BeNil())
					Expect(len(res)).To(Equal(0))
				})
			})
		})
	})
})
