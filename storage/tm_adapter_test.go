package storage_test

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/storage"
	"gitlab.com/makeos/mosdef/testutil"
)

var _ = Describe("TMDBAdapter", func() {
	var c storage.Engine
	var tx storage.Tx
	var err error
	var cfg *config.AppConfig
	var adapter *storage.TMDBAdapter

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		c = storage.NewBadger()
		Expect(c.Init(cfg.GetAppDBDir()))
		tx = c.NewTx(true, true)
		adapter = storage.NewTMDBAdapter(tx)
	})

	AfterEach(func() {
		Expect(c.Close()).To(BeNil())
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".Set", func() {
		k := []byte("a")
		v := []byte("b")

		BeforeEach(func() {
			adapter.Set(k, v)
		})

		It("should set key successfully", func() {
			rec, err := tx.Get(k)
			Expect(err).To(BeNil())
			Expect(rec.GetKey()).To(Equal(k))
			Expect(rec.Value).To(Equal(v))
		})
	})

	Describe(".Get", func() {
		k := []byte("a")
		v := []byte("b")

		BeforeEach(func() {
			adapter.Set(k, v)
		})

		It("should return value successfully", func() {
			res := adapter.Get(k)
			Expect(res).To(Equal(v))
		})

		It("should return nil when not found", func() {
			res := adapter.Get([]byte("xyz"))
			Expect(res).To(BeNil())
		})
	})

	Describe(".Has", func() {
		k := []byte("a")
		v := []byte("b")

		It("should return true when key exist", func() {
			adapter.Set(k, v)
			Expect(adapter.Has(k)).To(BeTrue())
		})

		It("should return false when key does not exist", func() {
			Expect(adapter.Has(k)).To(BeFalse())
		})
	})

	Describe(".Delete", func() {
		k := []byte("a")
		v := []byte("b")

		It("should successfully delete key", func() {
			adapter.Set(k, v)
			Expect(adapter.Has(k)).To(BeTrue())
			adapter.Delete(k)
			Expect(adapter.Has(k)).To(BeFalse())
		})
	})

	Describe(".Iterator", func() {
		k := []byte("3")
		k2 := []byte("1")
		k3 := []byte("c_")
		k4 := []byte("4")

		BeforeEach(func() {
			adapter.Set(k, []byte{})
			adapter.Set(k2, []byte{})
			adapter.Set(k3, []byte{})
			adapter.Set(k4, []byte{})
		})

		When("start = k, end = k3", func() {
			It("should return 2 keys in the expected ascending order", func() {
				expected := [][]byte{}
				it := adapter.Iterator(k, k3)
				for ; it.Valid(); it.Next() {
					expected = append(expected, it.Key())
				}
				Expect(expected).To(HaveLen(2))
				Expect(expected[0]).To(Equal(k))
				Expect(expected[1]).To(Equal(k4))
			})
		})
	})

	Describe(".ReverseIterator", func() {
		k := []byte("3")
		k2 := []byte("1")
		k3 := []byte("c_")
		k4 := []byte("4")

		BeforeEach(func() {
			adapter.Set(k, []byte{})
			adapter.Set(k2, []byte{})
			adapter.Set(k3, []byte{})
			adapter.Set(k4, []byte{})
		})

		When("start = k, end = k3", func() {
			It("should return 2 keys in the expected ascending order", func() {
				expected := [][]byte{}
				it := adapter.ReverseIterator(k, k3)
				defer it.Close()
				for ; it.Valid(); it.Next() {
					expected = append(expected, it.Key())
				}
				Expect(expected).To(HaveLen(2))
				Expect(expected[0]).To(Equal(k4))
				Expect(expected[1]).To(Equal(k))
			})
		})
	})

	Describe(".NewBatch", func() {
		It("should add batch key/value pairs", func() {
			k := []byte("x")
			v := []byte("stuff")
			b2 := adapter.NewBatch()
			b2.Set(k, v)
			b2.Write()
			Expect(adapter.Get(k)).To(Equal(v))
		})
	})
})
