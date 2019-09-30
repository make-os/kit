package ticket

import (
	"os"

	"github.com/makeos/mosdef/util"

	"github.com/makeos/mosdef/params"
	"github.com/makeos/mosdef/types"

	l "github.com/makeos/mosdef/logic"

	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/storage/tree"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/testutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Manager", func() {
	var err error
	var c storage.Engine
	var cfg *config.EngineConfig
	var mgr *Manager
	var state *tree.SafeTree
	var logic *l.Logic

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		c = storage.NewBadger(cfg)
		Expect(c.Init()).To(BeNil())
		db := storage.NewTMDBAdapter(c.F(true, true))
		state = tree.NewSafeTree(db, 128)
		logic = l.New(c, state, cfg)
		mgr, err = NewManager(cfg, logic)
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".Index", func() {
		When("ticket purchase value=10, current ticket price=10, ticket block height = 100, params.MinTicketMatDur=60, params.MaxTicketActiveDur=40", func() {
			BeforeEach(func() {
				params.InitialTicketPrice = 10
				params.NumBlocksPerPriceWindow = 100
				params.PricePercentIncrease = 0.2
				params.MinTicketMatDur = 60
				params.MaxTicketActiveDur = 40
				err := logic.SysKeeper().SaveBlockInfo(&types.BlockInfo{Height: 2})
				Expect(err).To(BeNil())
				Expect(logic.Sys().GetCurTicketPrice()).To(Equal(float64(10)))
				tx := &types.Transaction{Hash: util.StrToHash("hash"), Value: util.String("10")}
				err = mgr.Index(tx, "validator_addr", 100, 1)
				Expect(err).To(BeNil())
			})

			Specify("that only one non-child ticket is indexed", func() {
				tickets, err := mgr.store.Query(types.Ticket{})
				Expect(err).To(BeNil())
				Expect(tickets).To(HaveLen(1))
				Expect(tickets[0].ChildOf).To(BeEmpty())
			})

			Specify("that MaturedBy is 160, DecayBy is 140", func() {
				tickets, err := mgr.store.Query(types.Ticket{})
				Expect(err).To(BeNil())
				Expect(tickets).To(HaveLen(1))
				Expect(tickets[0].DecayBy).To(Equal(uint64(200)))
			})
		})

		When("ticket purchase value=25, current ticket price=10, ticket block height = 100", func() {
			BeforeEach(func() {
				params.InitialTicketPrice = 10
				params.NumBlocksPerPriceWindow = 100
				params.PricePercentIncrease = 0.2
				err := logic.SysKeeper().SaveBlockInfo(&types.BlockInfo{Height: 2})
				Expect(err).To(BeNil())
				Expect(logic.Sys().GetCurTicketPrice()).To(Equal(float64(10)))
				tx := &types.Transaction{Hash: util.StrToHash("hash"), Value: util.String("25")}
				err = mgr.Index(tx, "validator_addr", 100, 1)
				Expect(err).To(BeNil())
			})

			Specify("that one non-child and one child tickets are indexed", func() {
				tickets, err := mgr.store.Query(types.Ticket{})
				Expect(err).To(BeNil())
				Expect(tickets).To(HaveLen(2))
				Expect(tickets[0].ChildOf).To(BeEmpty())
				Expect(tickets[1].ChildOf).To(Equal(tickets[0].Hash))
			})
		})
	})
})
