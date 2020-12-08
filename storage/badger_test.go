package storage_test

import (
	"os"
	"testing"

	"github.com/make-os/kit/storage/common"
	"github.com/make-os/kit/storage/types"
	"github.com/make-os/kit/testutil"

	"github.com/make-os/kit/storage"

	"github.com/dgraph-io/badger/v2"

	"github.com/make-os/kit/config"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestStorage(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Storage Suite")
}

var _ = Describe("BadgerStore", func() {
	var c types.Engine
	var err error
	var cfg *config.AppConfig

	BeforeEach(func() {
		Expect(err).To(BeNil())
		cfg, _ = testutil.SetTestCfg()
		c, err = storage.NewBadger("")
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		Expect(c.Close()).To(BeNil())
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe("Test basic operations", func() {
		Describe(".Put", func() {
			var beforeTx *badger.Txn

			BeforeEach(func() {
				beforeTx = c.(*storage.BadgerStore).Tx.GetTx()
			})

			AfterEach(func() {
				curTx := c.(*storage.BadgerStore).Tx.GetTx()
				Expect(curTx).ToNot(Equal(beforeTx))
			})

			It("should successfully put a record", func() {
				key := []byte("key")
				value := []byte("value")
				expected := make([]byte, len(value))
				err := c.Put(common.NewRecord(key, value))
				Expect(err).To(BeNil())
				c.(*storage.BadgerStore).GetDB().View(func(txn *badger.Txn) error {
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
			var kv *common.Record
			var beforeTx *badger.Txn

			BeforeEach(func() {
				beforeTx = c.(*storage.BadgerStore).Tx.GetTx()
				kv = common.NewFromKeyValue(key, value)
				err := c.Put(common.NewRecord(key, value))
				Expect(err).To(BeNil())
			})

			AfterEach(func() {
				curTx := c.(*storage.BadgerStore).Tx.GetTx()
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
				beforeTx = c.(*storage.BadgerStore).Tx.GetTx()
				err := c.Put(common.NewRecord(key, value))
				Expect(err).To(BeNil())
				Expect(c.Del(key)).To(BeNil())
			})

			AfterEach(func() {
				curTx := c.(*storage.BadgerStore).Tx.GetTx()
				Expect(curTx).ToNot(Equal(beforeTx))
			})

			It("should fail find the record", func() {
				rec, err := c.Get(key)
				Expect(err).To(Equal(storage.ErrRecordNotFound))
				Expect(rec).To(BeNil())
			})
		})

		Describe(".Iterate", func() {
			k1 := common.NewRecord([]byte("a"), []byte("val"))
			k2 := common.NewRecord([]byte("b"), []byte("val2"))
			var beforeTx *badger.Txn

			BeforeEach(func() {
				beforeTx = c.(*storage.BadgerStore).Tx.GetTx()
				Expect(c.Put(k1)).To(BeNil())
				Expect(c.Put(k2)).To(BeNil())
			})

			AfterEach(func() {
				curTx := c.(*storage.BadgerStore).Tx.GetTx()
				Expect(curTx).ToNot(Equal(beforeTx))
			})

			Context("iterating from the first record", func() {
				It("should successfully return the records in the correct order", func() {
					var recs []*common.Record
					c.Iterate(nil, true, func(rec *common.Record) bool {
						recs = append(recs, rec)
						return false
					})
					Expect(recs[0].Equal(k1)).To(BeTrue())
					Expect(recs[1].Equal(k2)).To(BeTrue())
				})
			})

			Context("iterating from the last record", func() {
				It("should successfully return the records in the correct order", func() {
					var recs []*common.Record
					c.Iterate(nil, false, func(rec *common.Record) bool {
						recs = append(recs, rec)
						return false
					})
					Expect(recs[1].Equal(k1)).To(BeTrue())
					Expect(recs[0].Equal(k2)).To(BeTrue())
				})
			})

			Context("iterating from the first record and end after 1 iteration", func() {
				It("should successfully return the records in the correct order", func() {
					var recs []*common.Record
					c.Iterate(nil, true, func(rec *common.Record) bool {
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
