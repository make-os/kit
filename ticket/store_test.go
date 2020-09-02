package ticket

import (
	"os"

	storagetypes "github.com/make-os/lobe/storage/types"
	tickettypes "github.com/make-os/lobe/ticket/types"

	"github.com/make-os/lobe/config"
	"github.com/make-os/lobe/testutil"
	"github.com/make-os/lobe/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Store", func() {
	var err error
	var cfg *config.AppConfig
	var appDB storagetypes.Engine
	var store *Store

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		appDB, _ = testutil.GetDB()
		store = NewStore(appDB.NewTx(true, true))
	})

	AfterEach(func() {
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".Register", func() {
		var err error
		var ticket = &tickettypes.Ticket{Hash: util.StrToHexBytes("hash1"), ExpireBy: 100, MatureBy: 40, ProposerPubKey: util.StrToBytes32("pubkey"), Height: 10, Index: 2}
		var ticket2 = &tickettypes.Ticket{Hash: util.StrToHexBytes("hash2"), ExpireBy: 101, MatureBy: 40, ProposerPubKey: util.StrToBytes32("pubkey"), Height: 11, Index: 4}

		BeforeEach(func() {
			Expect(err).To(BeNil())
			err = store.Add(ticket)
			Expect(err).To(BeNil())
		})

		Context("add 1 record", func() {
			It("should successfully add the ticket", func() {
				key := MakeKey(ticket.Hash, ticket.Height, ticket.Index)
				var t tickettypes.Ticket
				rec, err := store.db.Get(key)
				Expect(err).To(BeNil())
				rec.Scan(&t)
				Expect(t).To(Equal(*ticket))
			})
		})

		Context("add 2 records", func() {
			BeforeEach(func() {
				Expect(err).To(BeNil())
				err = store.Add(ticket, ticket2)
				Expect(err).To(BeNil())
			})

			It("should successfully add the ticket", func() {
				var t, t2 tickettypes.Ticket

				key := MakeKey(ticket.Hash, ticket.Height, ticket.Index)
				rec, err := store.db.Get(key)
				Expect(err).To(BeNil())
				rec.Scan(&t)
				Expect(t).To(Equal(*ticket))

				key = MakeKey(ticket2.Hash, ticket2.Height, ticket2.Index)
				rec, err = store.db.Get(key)
				Expect(err).To(BeNil())
				rec.Scan(&t2)
				Expect(t2).To(Equal(*ticket2))
			})
		})
	})

	Describe(".GetByHash", func() {
		var store *Store
		var err error
		var ticket = &tickettypes.Ticket{Hash: util.StrToHexBytes("hash1"), ExpireBy: 100, MatureBy: 40, ProposerPubKey: util.StrToBytes32("pubkey"), Height: 10, Index: 2}

		BeforeEach(func() {
			store = NewStore(appDB.NewTx(true, true))
			Expect(err).To(BeNil())
			err = store.Add(ticket)
			Expect(err).To(BeNil())
		})

		It("should successfully find the ticket", func() {
			t := store.GetByHash(ticket.Hash)
			Expect(t).To(Equal(ticket))
		})

		When("no ticket with matching hash exist", func() {
			It("should return nil", func() {
				t := store.GetByHash(util.StrToHexBytes("unknown_hash"))
				Expect(err).To(BeNil())
				Expect(t).To(BeNil())
			})
		})
	})

	Describe(".Remove", func() {
		When("an entry with hash='hash1' exist", func() {
			var store *Store
			var err error
			var ticket = &tickettypes.Ticket{Hash: util.StrToHexBytes("hash1"), ExpireBy: 100, MatureBy: 40, ProposerPubKey: util.StrToBytes32("pubkey"), Height: 10, Index: 2}

			BeforeEach(func() {
				store = NewStore(appDB.NewTx(true, true))
				Expect(err).To(BeNil())
				err = store.Add(ticket)
				Expect(err).To(BeNil())
			})

			It("should successfully remove it", func() {
				Expect(store.RemoveByHash(ticket.Hash)).To(BeNil())
				t := store.GetByHash(ticket.Hash)
				Expect(t).To(BeNil())
			})
		})

		When("no entry with hash='hash1' exist", func() {
			var store *Store
			var err error
			var ticket = &tickettypes.Ticket{Hash: util.StrToHexBytes("hash2"), ExpireBy: 100, MatureBy: 40, ProposerPubKey: util.StrToBytes32("pubkey"), Height: 10, Index: 2}

			BeforeEach(func() {
				store = NewStore(appDB.NewTx(true, true))
				Expect(err).To(BeNil())
				err = store.Add(ticket)
				Expect(err).To(BeNil())
			})

			It("should remove nothing", func() {
				Expect(store.RemoveByHash(util.StrToHexBytes("hash1"))).To(BeNil())
				t := store.GetByHash(ticket.Hash)
				Expect(t).ToNot(BeNil())
			})
		})
	})

	Describe(".QueryOne", func() {
		When("an entry with hash='hash1' exist", func() {
			var store *Store
			var err error
			var ticket = &tickettypes.Ticket{Hash: util.StrToHexBytes("hash1"), ExpireBy: 100, MatureBy: 40, ProposerPubKey: util.StrToBytes32("pubkey"), Height: 10, Index: 2}

			BeforeEach(func() {
				store = NewStore(appDB.NewTx(true, true))
				Expect(err).To(BeNil())
				err = store.Add(ticket)
				Expect(err).To(BeNil())
			})

			It("should successfully find the entry with a predicate", func() {
				var entry = store.QueryOne(func(t *tickettypes.Ticket) bool { return t.Hash.Equal(ticket.Hash) })
				Expect(entry).To(Equal(ticket))
			})
		})

		When("an entry with hash='hash1' exist", func() {
			var store *Store
			var err error
			var ticket = &tickettypes.Ticket{Hash: util.StrToHexBytes("hash1"), ExpireBy: 100, MatureBy: 40, ProposerPubKey: util.StrToBytes32("pubkey"), Height: 10, Index: 2}

			BeforeEach(func() {
				store = NewStore(appDB.NewTx(true, true))
				Expect(err).To(BeNil())
				err = store.Add(ticket)
				Expect(err).To(BeNil())
			})

			It("should return nil when predicate fails to return true", func() {
				entry := store.QueryOne(func(t *tickettypes.Ticket) bool { return t.Hash.Equal(util.StrToHexBytes("hash2")) })
				Expect(entry).To(BeNil())
			})
		})
	})

	Describe(".Query", func() {
		When("two entries exist", func() {
			var store *Store
			var err error
			var ticket = &tickettypes.Ticket{Hash: util.StrToHexBytes("hash1"), ExpireBy: 100, MatureBy: 40, ProposerPubKey: util.StrToBytes32("pubkey"), Height: 10, Index: 2}
			var ticket2 = &tickettypes.Ticket{Hash: util.StrToHexBytes("hash2"), ExpireBy: 100, MatureBy: 40, ProposerPubKey: util.StrToBytes32("pubkey"), Height: 11, Index: 2}

			BeforeEach(func() {
				store = NewStore(appDB.NewTx(true, true))
				Expect(err).To(BeNil())
				err = store.Add(ticket, ticket2)
				Expect(err).To(BeNil())
			})

			It("should return two entries when predicate returns only true", func() {
				entries := store.Query(func(t *tickettypes.Ticket) bool { return true })
				Expect(entries).To(HaveLen(2))
			})

			It("should return one entries when predicate returns only true for hash2", func() {
				entries := store.Query(func(t *tickettypes.Ticket) bool { return t.Hash.Equal(util.StrToHexBytes("hash2")) })
				Expect(entries).To(HaveLen(1))
				Expect(entries[0]).To(Equal(ticket2))
			})

			When("limit is set", func() {
				It("should return 1 entry", func() {
					entries := store.Query(func(t *tickettypes.Ticket) bool { return true }, tickettypes.QueryOptions{Limit: 1})
					Expect(entries).To(HaveLen(1))
				})
			})

			When("sorted by height in descending order", func() {
				It("should return entries in the following order => hash2, hash1", func() {
					entries := store.Query(func(t *tickettypes.Ticket) bool { return true }, tickettypes.QueryOptions{SortByHeight: -1})
					Expect(entries).To(HaveLen(2))
					Expect(entries[0].Height).To(Equal(uint64(11)))
					Expect(entries[1].Height).To(Equal(uint64(10)))
				})
			})
		})
	})

	Describe(".Count", func() {
		When("two entries exist", func() {
			var store *Store
			var err error
			var ticket = &tickettypes.Ticket{Hash: util.StrToHexBytes("hash1"), ExpireBy: 100, MatureBy: 40, ProposerPubKey: util.StrToBytes32("pubkey"), Height: 10, Index: 2}
			var ticket2 = &tickettypes.Ticket{Hash: util.StrToHexBytes("hash2"), ExpireBy: 100, MatureBy: 40, ProposerPubKey: util.StrToBytes32("pubkey"), Height: 10, Index: 2}

			BeforeEach(func() {
				store = NewStore(appDB.NewTx(true, true))
				Expect(err).To(BeNil())
				err = store.Add(ticket, ticket2)
				Expect(err).To(BeNil())
			})

			It("should return 2 when predicate returns only true", func() {
				count := store.Count(func(t *tickettypes.Ticket) bool { return true })
				Expect(count).To(Equal(2))
			})

			It("should return 1 when predicate returns only true for hash2", func() {
				count := store.Count(func(t *tickettypes.Ticket) bool { return t.Hash.Equal(util.StrToHexBytes("hash2")) })
				Expect(count).To(Equal(1))
			})
		})
	})

	Describe(".UpdateOne", func() {
		When("one entry exist", func() {
			var store *Store
			var err error
			var ticket = &tickettypes.Ticket{Hash: util.StrToHexBytes("hash1"), ExpireBy: 100, MatureBy: 40, ProposerPubKey: util.StrToBytes32("pubkey"), Height: 10, Index: 2}

			BeforeEach(func() {
				store = NewStore(appDB.NewTx(true, true))
				err = store.Add(ticket)
				Expect(err).To(BeNil())
			})

			It("should update expire height", func() {
				qp := func(t *tickettypes.Ticket) bool {
					return t.Hash.Equal(util.StrToHexBytes("hash1"))
				}
				store.UpdateOne(tickettypes.Ticket{ExpireBy: 200}, qp)
				res := store.QueryOne(qp)
				Expect(res.ExpireBy).To(Equal(uint64(200)))
				Expect(store.Count(qp)).To(Equal(1))
			})

			It("should update nothing if predicate returns false", func() {
				qp := func(t *tickettypes.Ticket) bool { return t.Hash.Equal(util.StrToHexBytes("hash2")) }
				store.UpdateOne(tickettypes.Ticket{ExpireBy: 200}, qp)
				res := store.QueryOne(qp)
				Expect(res).To(BeNil())
			})
		})
	})
})
