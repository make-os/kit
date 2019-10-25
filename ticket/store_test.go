package ticket

import (
	"os"

	"github.com/jinzhu/gorm"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/testutil"
	"github.com/makeos/mosdef/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SQLStore", func() {
	var err error
	var cfg *config.EngineConfig

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".NewSQLStore", func() {
		It("should return error if db could not be openned", func() {
			_, err := NewSQLStore("/unknown/path")
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("unable to open database file"))
		})

		It("should return no error if db openned successfully", func() {
			_, err := NewSQLStore(cfg.GetTicketDBDir())
			Expect(err).To(BeNil())
		})
	})

	Describe(".getQueryOptions", func() {
		It("should return empty types.QueryOptions if none is passed in arg", func() {
			expected := types.QueryOptions{}
			Expect(getQueryOptions()).To(Equal(expected))
		})

		It("should return exact options passed to it", func() {
			expected := types.QueryOptions{Limit: 1}
			Expect(getQueryOptions(expected)).To(Equal(expected))
		})
	})

	Describe(".Add", func() {
		var store *SQLStore
		var err error
		var ticket = &types.Ticket{
			Hash:           "hash1",
			DecayBy:        100,
			MatureBy:       40,
			ProposerPubKey: "pubkey",
			Height:         10,
			Index:          2,
		}

		BeforeEach(func() {
			store, err = NewSQLStore(cfg.GetTicketDBDir())
			Expect(err).To(BeNil())
			err = store.Add(ticket)
			Expect(err).To(BeNil())
		})

		It("should successfully add the ticket", func() {
			var t types.Ticket
			err := store.db.First(&t).Error
			Expect(err).To(BeNil())
			Expect(t).To(Equal(*ticket))
		})
	})

	Describe(".Remove", func() {
		var store *SQLStore
		var err error
		var ticket = &types.Ticket{
			Hash:           "hash1",
			DecayBy:        100,
			MatureBy:       40,
			ProposerPubKey: "pubkey",
			Height:         10,
			Index:          2,
		}

		BeforeEach(func() {
			store, err = NewSQLStore(cfg.GetTicketDBDir())
			Expect(err).To(BeNil())
			err = store.Add(ticket)
			Expect(err).To(BeNil())
			err = store.Remove(ticket.Hash)
			Expect(err).To(BeNil())
		})

		It("should successfully add the ticket", func() {
			var t types.Ticket
			err := store.db.First(&t).Error
			Expect(err).ToNot(BeNil())
			Expect(err).To(Equal(gorm.ErrRecordNotFound))
		})
	})

	Describe(".GetLiveValidatorTickets", func() {
		var store *SQLStore
		var err error
		var ticket = &types.Ticket{Type: types.TxTypeValidatorTicket, Hash: "hash1", DecayBy: 100, MatureBy: 40, ProposerPubKey: "pubkey", Height: 10, Index: 2}
		var ticket2 = &types.Ticket{Type: types.TxTypeValidatorTicket, Hash: "hash2", DecayBy: 100, MatureBy: 70, ProposerPubKey: "pubkey", Height: 1, Index: 2}
		var ticket3 = &types.Ticket{Type: types.TxTypeStorerTicket, Hash: "hash3", DecayBy: 100, MatureBy: 70, ProposerPubKey: "pubkey", Height: 1, Index: 2}

		BeforeEach(func() {
			store, err = NewSQLStore(cfg.GetTicketDBDir())
			Expect(err).To(BeNil())
			err = store.Add(ticket, ticket2, ticket3)
			Expect(err).To(BeNil())
		})

		When("current block height is 50", func() {
			It("should return 1 ticket with MatureBy = 40 and DecayBy = 100", func() {
				tickets, err := store.GetLiveValidatorTickets(50)
				Expect(err).To(BeNil())
				Expect(tickets).To(HaveLen(1))
				Expect(tickets[0].MatureBy).To(Equal(uint64(40)))
				Expect(tickets[0].DecayBy).To(Equal(uint64(100)))
			})
		})

		When("current block height is 100", func() {
			It("should return 0 tickets", func() {
				tickets, err := store.GetLiveValidatorTickets(100)
				Expect(err).To(BeNil())
				Expect(tickets).To(HaveLen(0))
			})
		})

		When("current block height is 80", func() {
			It("should return 2 tickets", func() {
				tickets, err := store.GetLiveValidatorTickets(80)
				Expect(err).To(BeNil())
				Expect(tickets).To(HaveLen(2))
			})
		})
	})

	Describe(".CountLiveValidators", func() {
		var store *SQLStore
		var err error
		var ticket = &types.Ticket{Type: types.TxTypeValidatorTicket, Hash: "hash1", DecayBy: 100, MatureBy: 40, ProposerPubKey: "pubkey", Height: 10, Index: 2}
		var ticket2 = &types.Ticket{Type: types.TxTypeValidatorTicket, Hash: "hash2", DecayBy: 100, MatureBy: 70, ProposerPubKey: "pubkey", Height: 1, Index: 2}
		var ticket3 = &types.Ticket{Type: types.TxTypeStorerTicket, Hash: "hash3", DecayBy: 100, MatureBy: 70, ProposerPubKey: "pubkey", Height: 1, Index: 2}

		BeforeEach(func() {
			store, err = NewSQLStore(cfg.GetTicketDBDir())
			Expect(err).To(BeNil())
			err = store.Add(ticket, ticket2, ticket3)
			Expect(err).To(BeNil())
		})

		When("current block height is 50", func() {
			It("should return 1 ticket", func() {
				count, err := store.CountLiveValidators(50)
				Expect(err).To(BeNil())
				Expect(count).To(Equal(1))
			})
		})

		When("current block height is 100", func() {
			It("should return 0 tickets", func() {
				count, err := store.CountLiveValidators(100)
				Expect(err).To(BeNil())
				Expect(count).To(Equal(0))
			})
		})

		When("current block height is 80", func() {
			It("should return 2 tickets", func() {
				count, err := store.CountLiveValidators(80)
				Expect(err).To(BeNil())
				Expect(count).To(Equal(2))
			})
		})
	})

	Describe(".Count", func() {
		var store *SQLStore
		var err error
		var ticket = &types.Ticket{Hash: "hash1", DecayBy: 100, MatureBy: 40, ProposerPubKey: "pubkey", Height: 10, Index: 2}
		var ticket2 = &types.Ticket{Hash: "hash2", DecayBy: 100, MatureBy: 40, ProposerPubKey: "pubkey", Height: 1, Index: 2}

		BeforeEach(func() {
			store, err = NewSQLStore(cfg.GetTicketDBDir())
			Expect(err).To(BeNil())
			err = store.Add(ticket)
			Expect(err).To(BeNil())
			err = store.Add(ticket2)
			Expect(err).To(BeNil())
		})

		Context("a call with empty query", func() {
			var count int
			var err error

			BeforeEach(func() {
				count, err = store.Count(types.Ticket{})
				Expect(err).To(BeNil())
			})

			It("should return 2", func() {
				Expect(count).To(Equal(2))
			})
		})
	})

	Describe(".UpdateOne", func() {
		var store *SQLStore
		var err error
		var ticket = &types.Ticket{Hash: "hash1", DecayBy: 100, MatureBy: 40, ProposerPubKey: "pubkey", Height: 10, Index: 2}
		var ticket2 = &types.Ticket{Hash: "hash2", DecayBy: 100, MatureBy: 40, ProposerPubKey: "pubkey", Height: 1, Index: 2}

		BeforeEach(func() {
			store, err = NewSQLStore(cfg.GetTicketDBDir())
			Expect(err).To(BeNil())
			err = store.Add(ticket, ticket2)
			Expect(err).To(BeNil())
		})

		Context("update ticket1 'DecayBy' field", func() {

			BeforeEach(func() {
				err := store.UpdateOne(types.Ticket{Hash: ticket.Hash}, types.Ticket{DecayBy: 5000})
				Expect(err).To(BeNil())
			})

			Specify("that the ticket was updated", func() {
				updTicket, err := store.QueryOne(types.Ticket{Hash: ticket.Hash})
				Expect(err).To(BeNil())
				Expect(updTicket.Hash).To(Equal(ticket.Hash))
				Expect(updTicket.DecayBy).To(Equal(uint64(5000)))
			})
		})
	})

	Describe(".Query", func() {
		var store *SQLStore
		var err error

		When("no ticket was not found", func() {
			BeforeEach(func() {
				store, err = NewSQLStore(cfg.GetTicketDBDir())
				Expect(err).To(BeNil())
			})

			It("should return no result and nil", func() {
				res, err := store.Query(types.Ticket{})
				Expect(err).To(BeNil())
				Expect(res).To(BeEmpty())
			})
		})

		When("a ticket was not found", func() {
			var ticket = &types.Ticket{Hash: "hash1", DecayBy: 100, MatureBy: 40, ProposerPubKey: "pubkey", Height: 10, Index: 2}

			BeforeEach(func() {
				store, err = NewSQLStore(cfg.GetTicketDBDir())
				Expect(err).To(BeNil())
				err = store.Add(ticket)
				Expect(err).To(BeNil())
			})

			It("should return no result and nil", func() {
				res, err := store.Query(types.Ticket{})
				Expect(err).To(BeNil())
				Expect(res).To(HaveLen(1))
			})
		})
	})

	Describe(".QueryOne", func() {
		var store *SQLStore
		var err error

		When("no ticket was not found", func() {
			BeforeEach(func() {
				store, err = NewSQLStore(cfg.GetTicketDBDir())
				Expect(err).To(BeNil())
			})

			It("should return no result and nil", func() {
				res, err := store.QueryOne(types.Ticket{})
				Expect(err).To(BeNil())
				Expect(res).To(BeNil())
			})
		})

		When("a ticket was not found", func() {
			var ticket = &types.Ticket{Hash: "hash1", DecayBy: 100, MatureBy: 40, ProposerPubKey: "pubkey", Height: 10, Index: 2}

			BeforeEach(func() {
				store, err = NewSQLStore(cfg.GetTicketDBDir())
				Expect(err).To(BeNil())
				err = store.Add(ticket)
				Expect(err).To(BeNil())
			})

			It("should return no result and nil", func() {
				res, err := store.QueryOne(types.Ticket{})
				Expect(err).To(BeNil())
				Expect(res).ToNot(BeNil())
			})
		})
	})
})
