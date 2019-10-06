package ticket

import (
	"os"

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

	Describe(".GetTicketByProposerPubKey", func() {
		var store *SQLStore
		var err error
		var ticket = &types.Ticket{Hash: "hash1", DecayBy: 100, MatureBy: 40, ProposerPubKey: "pubkey", Height: 10, Index: 2}
		var ticket2 = &types.Ticket{Hash: "hash2", DecayBy: 100, MatureBy: 40, ProposerPubKey: "pubkey", Height: 1, Index: 2, ChildOf: "hash"}

		BeforeEach(func() {
			store, err = NewSQLStore(cfg.GetTicketDBDir())
			Expect(err).To(BeNil())
			err = store.Add(ticket)
			Expect(err).To(BeNil())
			err = store.Add(ticket2)
			Expect(err).To(BeNil())
		})

		Context("without query options", func() {
			It("should return 2 tickets", func() {
				tickets, err := store.GetTicketByProposerPubKey("pubkey")
				Expect(err).To(BeNil())
				Expect(tickets).To(HaveLen(2))
				Expect(tickets[0]).To(Equal(ticket))
				Expect(tickets[1]).To(Equal(ticket2))
			})
		})

		Context("with limit set to 1", func() {
			It("should return 1 ticket (the first added ticket)", func() {
				tickets, err := store.GetTicketByProposerPubKey("pubkey", types.QueryOptions{
					Limit: 1,
				})
				Expect(err).To(BeNil())
				Expect(tickets).To(HaveLen(1))
				Expect(tickets[0]).To(Equal(ticket))
			})
		})

		Context("with noChild set to true", func() {
			It("should return 1 ticket (the one with am empty childOf field)", func() {
				tickets, err := store.GetTicketByProposerPubKey("pubkey", types.QueryOptions{
					NoChild: true,
				})
				Expect(err).To(BeNil())
				Expect(tickets).To(HaveLen(1))
				Expect(tickets[0]).To(Equal(ticket))
				Expect(tickets[0].ChildOf).To(BeEmpty())
			})
		})

		Context("with limit set to 1 and offset set to 1", func() {
			It("should return 1 tickets (the second ticket)", func() {
				tickets, err := store.GetTicketByProposerPubKey("pubkey", types.QueryOptions{
					Limit:  1,
					Offset: 1,
				})
				Expect(err).To(BeNil())
				Expect(tickets).To(HaveLen(1))
				Expect(tickets[0]).To(Equal(ticket2))
			})
		})

		Context("with order set to 'height asc'", func() {
			It("should return the ticket with the lowest height as the first", func() {
				tickets, err := store.GetTicketByProposerPubKey("pubkey", types.QueryOptions{
					Order: "height asc",
				})
				Expect(err).To(BeNil())
				Expect(tickets).To(HaveLen(2))
				Expect(tickets[0]).To(Equal(ticket2))
			})
		})

		Context("with order set to 'height desc'", func() {
			It("should return the ticket with the largest height as the first", func() {
				tickets, err := store.GetTicketByProposerPubKey("pubkey", types.QueryOptions{
					Order: "height desc",
				})
				Expect(err).To(BeNil())
				Expect(tickets).To(HaveLen(2))
				Expect(tickets[0]).To(Equal(ticket))
			})
		})
	})

	Describe(".GetLive", func() {
		var store *SQLStore
		var err error
		var ticket = &types.Ticket{Hash: "hash1", DecayBy: 100, MatureBy: 40, ProposerPubKey: "pubkey", Height: 10, Index: 2}
		var ticket2 = &types.Ticket{Hash: "hash2", DecayBy: 100, MatureBy: 70, ProposerPubKey: "pubkey", Height: 1, Index: 2, ChildOf: "hash"}

		BeforeEach(func() {
			store, err = NewSQLStore(cfg.GetTicketDBDir())
			Expect(err).To(BeNil())
			err = store.Add(ticket)
			Expect(err).To(BeNil())
			err = store.Add(ticket2)
			Expect(err).To(BeNil())
		})

		When("current block height is 50", func() {
			It("should return 1 ticket with MatureBy = 40 and DecayBy = 100", func() {
				tickets, err := store.GetLive(50)
				Expect(err).To(BeNil())
				Expect(tickets).To(HaveLen(1))
				Expect(tickets[0].MatureBy).To(Equal(uint64(40)))
				Expect(tickets[0].DecayBy).To(Equal(uint64(100)))
			})
		})

		When("current block height is 100", func() {
			It("should return 0 tickets", func() {
				tickets, err := store.GetLive(100)
				Expect(err).To(BeNil())
				Expect(tickets).To(HaveLen(0))
			})
		})

		When("current block height is 80", func() {
			It("should return 2 tickets", func() {
				tickets, err := store.GetLive(80)
				Expect(err).To(BeNil())
				Expect(tickets).To(HaveLen(2))
			})
		})
	})

	Describe(".CountLive", func() {
		var store *SQLStore
		var err error
		var ticket = &types.Ticket{Hash: "hash1", DecayBy: 100, MatureBy: 40, ProposerPubKey: "pubkey", Height: 10, Index: 2}
		var ticket2 = &types.Ticket{Hash: "hash2", DecayBy: 100, MatureBy: 70, ProposerPubKey: "pubkey", Height: 1, Index: 2, ChildOf: "hash"}

		BeforeEach(func() {
			store, err = NewSQLStore(cfg.GetTicketDBDir())
			Expect(err).To(BeNil())
			err = store.Add(ticket)
			Expect(err).To(BeNil())
			err = store.Add(ticket2)
			Expect(err).To(BeNil())
		})

		When("current block height is 50", func() {
			It("should return 1 ticket", func() {
				count, err := store.CountLive(50)
				Expect(err).To(BeNil())
				Expect(count).To(Equal(1))
			})
		})

		When("current block height is 100", func() {
			It("should return 0 tickets", func() {
				count, err := store.CountLive(100)
				Expect(err).To(BeNil())
				Expect(count).To(Equal(0))
			})
		})

		When("current block height is 80", func() {
			It("should return 2 tickets", func() {
				count, err := store.CountLive(80)
				Expect(err).To(BeNil())
				Expect(count).To(Equal(2))
			})
		})
	})

	Describe(".Count", func() {
		var store *SQLStore
		var err error
		var ticket = &types.Ticket{Hash: "hash1", DecayBy: 100, MatureBy: 40, ProposerPubKey: "pubkey", Height: 10, Index: 2}
		var ticket2 = &types.Ticket{Hash: "hash2", DecayBy: 100, MatureBy: 40, ProposerPubKey: "pubkey", Height: 1, Index: 2, ChildOf: "hash"}

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

		Context("count tx without a childOf value", func() {
			var count int
			var err error

			BeforeEach(func() {
				count, err = store.Count(types.Ticket{}, types.QueryOptions{NoChild: true})
				Expect(err).To(BeNil())
			})

			It("should return 1", func() {
				Expect(count).To(Equal(1))
			})
		})
	})
})
