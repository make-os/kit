package tree_test

import (
	"os"

	. "github.com/make-os/kit/pkgs/tree"
	storagetypes "github.com/make-os/kit/storage/types"
	tmdb "github.com/tendermint/tm-db"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/make-os/kit/config"
	"github.com/make-os/kit/testutil"
)

var _ = Describe("TMDBAdapter", func() {
	var appDB storagetypes.Engine
	var stateDB tmdb.DB
	var err error
	var cfg *config.AppConfig
	var tree *SafeTree

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		appDB, stateDB = testutil.GetDB()
		tree, err = NewSafeTree(stateDB, 128)
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		Expect(appDB.Close()).To(BeNil())
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".Set", func() {
		key := []byte("key")

		When("key didn't previously existed", func() {
			It("should return false when successful added", func() {
				res := tree.Set(key, []byte("value"))
				Expect(res).To(BeFalse())
			})
		})

		When("key previously existed", func() {
			BeforeEach(func() {
				res := tree.Set(key, []byte("value_a"))
				Expect(res).To(BeFalse())
			})

			It("should return true when successful added", func() {
				res := tree.Set(key, []byte("value"))
				Expect(res).To(BeTrue())
			})
		})

		When("value is nil", func() {
			It("should panic", func() {
				Expect(func() {
					tree.Set(key, nil)
				}).To(Panic())
			})
		})
	})

	Describe(".Get", func() {
		key := []byte("key")

		When("key does not exist", func() {
			It("should return nil", func() {
				idx, val := tree.Get(key)
				Expect(val).To(BeNil())
				Expect(idx).To(BeZero())
			})
		})

		When("key exists", func() {
			BeforeEach(func() {
				res := tree.Set(key, []byte("val"))
				Expect(res).To(BeFalse())
			})

			It("should return index 0 and val=[]byte('val')", func() {
				idx, val := tree.Get(key)
				Expect(val).ToNot(BeNil())
				Expect(val).To(Equal([]byte("val")))
				Expect(idx).To(Equal(int64(0)))
			})
		})
	})

	Describe(".Remove", func() {
		key := []byte("key")
		When("key does not exist", func() {
			It("should return false", func() {
				removed := tree.Remove(key)
				Expect(removed).To(BeFalse())
			})
		})

		When("key exists", func() {
			It("should return true", func() {
				res := tree.Set(key, []byte("val"))
				Expect(res).To(BeFalse())
				removed := tree.Remove(key)
				Expect(removed).To(BeTrue())
			})
		})
	})

	Describe(".Version", func() {
		key := []byte("key")

		When("tree is empty and unsaved", func() {
			It("should return 0 as version", func() {
				v := tree.Version()
				Expect(v).To(Equal(int64(0)))
			})
		})

		When("tree has unsaved modifications", func() {
			It("should return 0 as version", func() {
				tree.Set(key, []byte("val"))
				v := tree.Version()
				Expect(v).To(Equal(int64(0)))
			})
		})

		When("tree is saved", func() {
			It("should return 1 as version", func() {
				tree.Set(key, []byte("val"))
				v := tree.Version()
				Expect(v).To(Equal(int64(0)))
				tree.SaveVersion()
				Expect(tree.Version()).To(Equal(int64(1)))
			})
		})
	})

	Describe(".SaveVersion", func() {
		BeforeEach(func() {
			Expect(tree.Version()).To(Equal(int64(0)))
		})

		It("should increment version", func() {
			tree.SaveVersion()
			Expect(tree.Version()).To(Equal(int64(1)))
		})
	})

	Describe(".Load", func() {
		key := []byte("key")
		var tree2 *SafeTree

		BeforeEach(func() {
			tree.Set(key, []byte("val"))
			_, _, err := tree.SaveVersion()
			Expect(err).To(BeNil())
			tree2, _ = NewSafeTree(stateDB, 128)
			v, err := tree2.Load()
			Expect(err).To(BeNil())
			Expect(v).To(Equal(int64(1)))
		})

		It("should return value of key", func() {
			_, res := tree2.Get(key)
			Expect(res).To(Equal([]byte("val")))
		})
	})

	Describe(".WorkingHash", func() {
		key := []byte("key")

		Specify("that working hash is updated after each set", func() {
			tree.Set(key, []byte("val"))
			wh := tree.WorkingHash()
			tree.Set(key, []byte("val2"))
			wh2 := tree.WorkingHash()
			Expect(wh).To(Not(BeEmpty()))
			Expect(wh2).To(Not(BeEmpty()))
			Expect(wh).To(Not(Equal(wh2)))
		})
	})

	Describe(".Hash", func() {
		key := []byte("key")
		When("has a saved version", func() {
			Specify("that working hash changed and version is incremented after save", func() {
				wh := tree.Hash()
				tree.Set(key, []byte("val"))
				_, _, err := tree.SaveVersion()
				Expect(err).To(BeNil())
				Expect(tree.Version()).To(Equal(int64(1)))
				wh2 := tree.Hash()
				Expect(wh).To(Not(BeEmpty()))
				Expect(wh2).To(Not(BeEmpty()))
				Expect(wh).To(Not(Equal(wh2)))
			})
		})
	})

	Describe(".Rollback", func() {
		key := []byte("key")

		When("key has not been saved", func() {
			BeforeEach(func() {
				tree.Set(key, []byte("val"))
			})

			Specify("that a rollback removes the key", func() {
				_, v := tree.Get(key)
				Expect(v).To(Equal([]byte("val")))
				tree.Rollback()
				_, v = tree.Get(key)
				Expect(v).To(BeNil())
			})
		})
	})

	Describe(".GetVersioned", func() {
		key := []byte("key")

		BeforeEach(func() {
			tree.Set(key, []byte("val"))
			_, _, err := tree.SaveVersion()
			Expect(err).To(BeNil())
			tree.Set(key, []byte("val2"))
			_, _, err = tree.SaveVersion()
			Expect(err).To(BeNil())
			Expect(tree.Version()).To(Equal(int64(2)))
		})

		Specify("that at version=1, value is byte(val)", func() {
			_, v := tree.GetVersioned(key, 1)
			Expect(v).To(Equal([]byte("val")))
		})

		Specify("that at version=2, value is byte(val2)", func() {
			_, v := tree.GetVersioned(key, 2)
			Expect(v).To(Equal([]byte("val2")))
		})
	})
})
