package logic

import (
	"os"

	"github.com/makeos/mosdef/testutil"

	"github.com/golang/mock/gomock"
	"github.com/makeos/mosdef/types"

	"github.com/makeos/mosdef/params"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/storage"
)

var _ = Describe("System", func() {
	var appDB, stateTreeDB storage.Engine
	var err error
	var cfg *config.AppConfig
	var logic *Logic
	var sysLogic *System
	var ctrl *gomock.Controller

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())

		appDB, stateTreeDB = testutil.GetDB(cfg)
		logic = New(appDB, stateTreeDB, cfg)

		sysLogic = &System{logic: logic}
	})

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	AfterEach(func() {
		Expect(appDB.Close()).To(BeNil())
		Expect(stateTreeDB.Close()).To(BeNil())
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".GetCurValidatorTicketPrice", func() {
		When("initial ticket price = 10, blocks per price window=100, percent increase=20, cur. height = 2", func() {
			BeforeEach(func() {
				params.MinValidatorsTicketPrice = 10
				err := logic.SysKeeper().SaveBlockInfo(&types.BlockInfo{AppHash: []byte("stuff"), Height: 2})
				Expect(err).To(BeNil())
			})

			It("should return price = 10", func() {
				price := sysLogic.GetCurValidatorTicketPrice()
				Expect(price).To(Equal(float64(10)))
			})
		})
	})

})
