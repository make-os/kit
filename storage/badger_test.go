package storage

import (
	"os"

	"github.com/dgraph-io/badger"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/testutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CouchDB", func() {
	var c Engine
	var err error
	var cfg *config.EngineConfig

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		c = NewBadger(cfg)
	})

	AfterEach(func() {
		Expect(c.Close()).To(BeNil())
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".Init", func() {
		It("should return no error", func() {
			Expect(c.Init()).To(BeNil())
		})
	})

	Describe("Test default operations", func() {
		BeforeEach(func() {
			Expect(c.Init()).To(BeNil())
		})

		Describe(".Put", func() {
			var beforeTx *badger.Txn

			BeforeEach(func() {
				beforeTx = c.(*Badger).BadgerFunctions.tx
			})

			AfterEach(func() {
				curTx := c.(*Badger).BadgerFunctions.tx
				Expect(curTx).ToNot(Equal(beforeTx))
			})

			It("should successfully put a record", func() {
				key := []byte("key")
				value := []byte("value")
				expected := make([]byte, len(value))
				err := c.Put(NewRecord(key, value))
				Expect(err).To(BeNil())
				c.(*Badger).db.View(func(txn *badger.Txn) error {
					item, err := txn.Get(key)
					Expect(err).To(BeNil())
					Expect(item.ValueSize()).To(Equal(int64(len(value))))
					item.ValueCopy(expected)
					Expect(expected).To(Equal(value))
					return nil
				})
			})
		})

		Describe(".Get", func() {
			key := []byte("key")
			value := []byte("value")
			var kv *Record
			var beforeTx *badger.Txn

			BeforeEach(func() {
				beforeTx = c.(*Badger).BadgerFunctions.tx
				kv = NewFromKeyValue(key, value)
				err := c.Put(NewRecord(key, value))
				Expect(err).To(BeNil())
			})

			AfterEach(func() {
				curTx := c.(*Badger).BadgerFunctions.tx
				Expect(curTx).ToNot(Equal(beforeTx))
			})

			It("should successfully get a record", func() {
				rec, err := c.Get(key)
				Expect(err).To(BeNil())
				Expect(rec).To(Equal(kv))
			})
		})

		Describe(".Del", func() {
			key := []byte("key")
			value := []byte("value")
			var beforeTx *badger.Txn

			BeforeEach(func() {
				beforeTx = c.(*Badger).BadgerFunctions.tx
				err := c.Put(NewRecord(key, value))
				Expect(err).To(BeNil())
				Expect(c.Del(key)).To(BeNil())
			})

			AfterEach(func() {
				curTx := c.(*Badger).BadgerFunctions.tx
				Expect(curTx).ToNot(Equal(beforeTx))
			})

			It("should fail find the record", func() {
				rec, err := c.Get(key)
				Expect(err).To(Equal(ErrRecordNotFound))
				Expect(rec).To(BeNil())
			})
		})

		Describe(".Iterate", func() {
			k1 := NewRecord([]byte("a"), []byte("val"))
			k2 := NewRecord([]byte("b"), []byte("val2"))
			var beforeTx *badger.Txn

			BeforeEach(func() {
				beforeTx = c.(*Badger).BadgerFunctions.tx
				Expect(c.Put(k1)).To(BeNil())
				Expect(c.Put(k2)).To(BeNil())
			})

			AfterEach(func() {
				curTx := c.(*Badger).BadgerFunctions.tx
				Expect(curTx).ToNot(Equal(beforeTx))
			})

			Context("iterating from the first record", func() {
				It("should successfully return the records in the correct order", func() {
					var recs = []*Record{}
					c.Iterate(nil, true, func(rec *Record) bool {
						recs = append(recs, rec)
						return false
					})
					Expect(recs[0].Equal(k1)).To(BeTrue())
					Expect(recs[1].Equal(k2)).To(BeTrue())
				})
			})

			Context("iterating from the last record", func() {
				It("should successfully return the records in the correct order", func() {
					var recs = []*Record{}
					c.Iterate(nil, false, func(rec *Record) bool {
						recs = append(recs, rec)
						return false
					})
					Expect(recs[1].Equal(k1)).To(BeTrue())
					Expect(recs[0].Equal(k2)).To(BeTrue())
				})
			})

			Context("iterating from the first record and end after 1 iteration", func() {
				It("should successfully return the records in the correct order", func() {
					var recs = []*Record{}
					c.Iterate(nil, true, func(rec *Record) bool {
						recs = append(recs, rec)
						return true
					})
					Expect(recs).To(HaveLen(1))
					Expect(recs[0].Equal(k1)).To(BeTrue())
				})
			})
		})
	})
})
