package repo

import (
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/testutil"
	"github.com/makeos/mosdef/util"
)

var _ = Describe("DBCache", func() {
	var err error
	var cfg *config.AppConfig
	var cache *DBCache

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		dbCacheCleanerInt = 200 * time.Millisecond
		cache, err = NewDBCache(10, cfg.GetRepoRoot(), 1*time.Hour)
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".NewDBCache", func() {
		It("should return an instance of DBCache", func() {
			Expect(cache).To(BeAssignableToTypeOf(&DBCache{}))
		})
	})

	Describe(".Get", func() {
		When("target repo directory is not found", func() {
			It("should return ErrRepoNotFound", func() {
				_, err := cache.Get("some_repo")
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(ErrRepoNotFound))
			})
		})

		When("target repo directory exist", func() {
			var repoName = "my_repo"
			var err error
			var db storage.Engine

			BeforeEach(func() {
				err = os.Mkdir(filepath.Join(cfg.GetRepoRoot(), repoName), 0700)
				Expect(err).To(BeNil())
				db, err = cache.Get(repoName)
			})

			It("should return a db and cache size should be 1", func() {
				Expect(err).To(BeNil())
				Expect(db).ToNot(BeNil())
				Expect(cache.cache.Len()).To(Equal(1))
			})
		})

		When("db already exist in cache", func() {
			var repoName string
			var err error
			var db storage.Engine
			var db2 storage.Engine

			BeforeEach(func() {
				repoName = util.RandString(5)
				err = os.Mkdir(filepath.Join(cfg.GetRepoRoot(), repoName), 0700)
				Expect(err).To(BeNil())
				db, err = cache.Get(repoName)
				Expect(err).To(BeNil())
				Expect(db).ToNot(BeNil())
				db2, err = cache.Get(repoName)
			})

			It("should return existing db and cache size should be 1", func() {
				Expect(err).To(BeNil())
				Expect(db).To(Equal(db2))
				Expect(cache.cache.Len()).To(Equal(1))
			})
		})
	})

	Describe("when entry has expired", func() {
		var repoName string
		var err error

		BeforeEach(func() {
			cache, err = NewDBCache(10, cfg.GetRepoRoot(), 20*time.Millisecond)
			Expect(err).To(BeNil())

			repoName = util.RandString(5)
			err = os.Mkdir(filepath.Join(cfg.GetRepoRoot(), repoName), 0700)
			Expect(err).To(BeNil())
			_, err = cache.Get(repoName)
			Expect(err).To(BeNil())
			Expect(cache.cache.Len()).To(Equal(1))
			time.Sleep(dbCacheCleanerInt)
		})

		It("should return 0 as cache len", func() {
			Expect(cache.cache.Len()).To(Equal(0))
		})
	})

	Describe(".Clear", func() {
		var repoName string
		var err error
		var db storage.Engine

		BeforeEach(func() {
			repoName = util.RandString(5)
			err = os.Mkdir(filepath.Join(cfg.GetRepoRoot(), repoName), 0700)
			Expect(err).To(BeNil())
			db, err = cache.Get(repoName)
			Expect(err).To(BeNil())
			Expect(db).ToNot(BeNil())
			cache.Clear()
		})

		It("should empty the cache", func() {
			Expect(cache.cache.Len()).To(Equal(0))
		})
	})
})
