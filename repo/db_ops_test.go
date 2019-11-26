package repo

import (
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/testutil"
	"github.com/makeos/mosdef/util"
)

var _ = Describe("DBCache", func() {
	var err error
	var cfg *config.EngineConfig
	var cache *DBCache
	var dbOps *DBOps
	var repoName string

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		dbCacheCleanerInt = 200 * time.Millisecond

		repoName = util.RandString(5)
		err = os.MkdirAll(filepath.Join(cfg.GetRepoRoot(), repoName), 0700)
		Expect(err).To(BeNil())

		cache, err = NewDBCache(10, cfg.GetRepoRoot(), 1*time.Hour)
		Expect(err).To(BeNil())
		dbOps = NewDBOps(cache, repoName)
		_ = dbOps
	})

	AfterEach(func() {
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

})
