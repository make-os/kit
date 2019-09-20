package logic_test

// import (
// 	"os"

// 	"github.com/makeos/mosdef/config"
// 	"github.com/makeos/mosdef/storage"
// 	"github.com/makeos/mosdef/storage/tree"
// 	"github.com/makeos/mosdef/testutil"
// 	. "github.com/onsi/ginkgo"
// 	. "github.com/onsi/gomega"

// 	l "github.com/makeos/mosdef/logic"
// )

// var _ = Describe("TxContract", func() {
// 	var c storage.Engine
// 	var err error
// 	var cfg *config.EngineConfig
// 	var state *tree.SafeTree
// 	var logic *l.Logic

// 	BeforeEach(func() {
// 		cfg, err = testutil.SetTestCfg()
// 		Expect(err).To(BeNil())
// 		c = storage.NewBadger(cfg)
// 		Expect(c.Init()).To(BeNil())
// 		db := storage.NewTMDBAdapter(c.F(true, true))
// 		state = tree.NewSafeTree(db, 128)
// 		logic = l.New(c, state, cfg)
// 		_ = logic
// 	})

// 	AfterEach(func() {
// 		Expect(c.Close()).To(BeNil())
// 		err = os.RemoveAll(cfg.DataDir())
// 		Expect(err).To(BeNil())
// 	})

// 	Describe(".Exec", func() {
// 		Context("with types.Transaction argument", func() {

// 		})
// 	})
// })
