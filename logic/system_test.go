package logic

import (
	"os"

	"github.com/makeos/mosdef/types"

	"github.com/makeos/mosdef/params"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/storage/tree"
	"github.com/makeos/mosdef/testutil"
)

var _ = Describe("System", func() {
	var c storage.Engine
	var err error
	var cfg *config.EngineConfig
	var state *tree.SafeTree
	var logic *Logic
	var sysLogic *System

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		c = storage.NewBadger(cfg)
		Expect(c.Init()).To(BeNil())
		db := storage.NewTMDBAdapter(c.F(true, true))
		state = tree.NewSafeTree(db, 128)
		logic = New(c, state, cfg)
		sysLogic = &System{logic: logic}
	})

	AfterEach(func() {
		Expect(c.Close()).To(BeNil())
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".GetCurTicketPrice", func() {
		When("initial ticket price = 10, blocks per price window=100, percent increase=20, cur. height = 2", func() {
			BeforeEach(func() {
				params.InitialTicketPrice = 10
				params.NumBlocksPerPriceWindow = 100
				params.PricePercentIncrease = 0.2
				err := logic.SysKeeper().SaveBlockInfo(&types.BlockInfo{AppHash: []byte("stuff"), Height: 2})
				Expect(err).To(BeNil())
			})

			It("should return price = 10", func() {
				price := sysLogic.GetCurTicketPrice()
				Expect(price).To(Equal(float64(10)))
			})
		})

		When("initial ticket price = 10, blocks per price window=100, percent increase=20, cur. height = 200", func() {
			BeforeEach(func() {
				params.InitialTicketPrice = 10
				params.NumBlocksPerPriceWindow = 100
				params.PricePercentIncrease = 0.2
				err := logic.SysKeeper().SaveBlockInfo(&types.BlockInfo{AppHash: []byte("stuff"), Height: 200})
				Expect(err).To(BeNil())
			})

			It("should return price = 12", func() {
				price := sysLogic.GetCurTicketPrice()
				Expect(price).To(Equal(float64(12)))
			})
		})
	})
})
