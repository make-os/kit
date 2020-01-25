package logic

import (
	"fmt"
	"os"

	"github.com/makeos/mosdef/logic/keepers"
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
	var mockLogic *testutil.MockObjects

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())

		appDB, stateTreeDB = testutil.GetDB(cfg)
		logic = New(appDB, stateTreeDB, cfg)

		sysLogic = &System{logic: logic}
	})

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockLogic = testutil.MockLogic(ctrl)
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
				params.InitialTicketPrice = 10
				params.NumBlocksPerPriceWindow = 100
				params.PricePercentIncrease = 0.2
				err := logic.SysKeeper().SaveBlockInfo(&types.BlockInfo{AppHash: []byte("stuff"), Height: 2})
				Expect(err).To(BeNil())
			})

			It("should return price = 10", func() {
				price := sysLogic.GetCurValidatorTicketPrice()
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
				price := sysLogic.GetCurValidatorTicketPrice()
				Expect(price).To(Equal(float64(12)))
			})
		})
	})

	Describe(".CheckSetNetMaturity", func() {
		When("unable to determine network maturity status", func() {
			BeforeEach(func() {
				mockLogic.SysKeeper.EXPECT().IsMarkedAsMature().Return(false, fmt.Errorf("bad error"))
				sysLogic.logic = mockLogic.Logic
			})

			It("should return err='failed to determine network maturity status'", func() {
				err := sysLogic.CheckSetNetMaturity()
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(Equal("failed to determine network maturity status: bad error"))
			})
		})

		When("network has been marked as matured", func() {
			BeforeEach(func() {
				mockLogic.SysKeeper.EXPECT().IsMarkedAsMature().Return(true, nil)
				sysLogic.logic = mockLogic.Logic
			})

			It("should return nil", func() {
				err := sysLogic.CheckSetNetMaturity()
				Expect(err).To(BeNil())
			})
		})

		When("network has not been marked as matured and no recent committed block", func() {
			BeforeEach(func() {
				mockLogic.SysKeeper.EXPECT().IsMarkedAsMature().Return(false, nil)
				mockLogic.SysKeeper.EXPECT().GetLastBlockInfo().Return(nil, keepers.ErrBlockInfoNotFound)
				sysLogic.logic = mockLogic.Logic
			})

			It("should return err='no committed block yet'", func() {
				err := sysLogic.CheckSetNetMaturity()
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("no committed block yet"))
			})
		})

		When("network has not been marked as matured and an error occurred when fetching last committed block", func() {
			BeforeEach(func() {
				mockLogic.SysKeeper.EXPECT().IsMarkedAsMature().Return(false, nil)
				mockLogic.SysKeeper.EXPECT().GetLastBlockInfo().Return(nil, fmt.Errorf("bad error"))
				sysLogic.logic = mockLogic.Logic
			})

			It("should return err='no committed block yet'", func() {
				err := sysLogic.CheckSetNetMaturity()
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("bad error"))
			})
		})

		Context("network has not been marked as matured", func() {
			When("last committed block height = 10, params.NetMaturityHeight = 20", func() {
				BeforeEach(func() {
					params.NetMaturityHeight = 20
					mockLogic.SysKeeper.EXPECT().IsMarkedAsMature().Return(false, nil)
					mockLogic.SysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: 10}, nil)
					sysLogic.logic = mockLogic.Logic
				})

				It("should return err='network maturity period has not been reached...'", func() {
					err := sysLogic.CheckSetNetMaturity()
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(ContainSubstring("network maturity period has not been reached"))
				})
			})

			When("last committed block height = 10, params.NetMaturityHeight = 10", func() {
				When("failure to get live ticket count", func() {
					BeforeEach(func() {
						params.NetMaturityHeight = 10
						params.MinBootstrapLiveTickets = 2
						mockLogic.SysKeeper.EXPECT().IsMarkedAsMature().Return(false, nil)
						mockLogic.SysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: 10}, nil)
						mockLogic.TicketManager.EXPECT().CountActiveValidatorTickets().Return(0, fmt.Errorf("bad error"))
						sysLogic.logic = mockLogic.Logic
					})

					It("should return err='insufficient live bootstrap tickets'", func() {
						err := sysLogic.CheckSetNetMaturity()
						Expect(err).ToNot(BeNil())
						Expect(err.Error()).To(ContainSubstring("failed to count live tickets: bad error"))
					})
				})
			})

			When("last committed block height = 10, params.NetMaturityHeight = 10", func() {
				When("live tickets 1 and params.MinBootstrapLiveTickets = 2", func() {
					BeforeEach(func() {
						params.NetMaturityHeight = 10
						params.MinBootstrapLiveTickets = 2
						mockLogic.SysKeeper.EXPECT().IsMarkedAsMature().Return(false, nil)
						mockLogic.SysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: 10}, nil)
						mockLogic.TicketManager.EXPECT().CountActiveValidatorTickets().Return(1, nil)
						sysLogic.logic = mockLogic.Logic
					})

					It("should return err='insufficient live bootstrap tickets'", func() {
						err := sysLogic.CheckSetNetMaturity()
						Expect(err).ToNot(BeNil())
						Expect(err.Error()).To(ContainSubstring("insufficient live bootstrap tickets"))
					})
				})
			})

			When("last committed block height = 10, params.NetMaturityHeight = 10", func() {
				When("live tickets 1 and params.MinBootstrapLiveTickets = 1 and failed to mark as mature", func() {
					BeforeEach(func() {
						params.NetMaturityHeight = 10
						params.MinBootstrapLiveTickets = 1
						mockLogic.SysKeeper.EXPECT().IsMarkedAsMature().Return(false, nil)
						mockLogic.SysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: 10}, nil)
						mockLogic.SysKeeper.EXPECT().MarkAsMatured(uint64(10)).Return(fmt.Errorf("bad error"))
						mockLogic.TicketManager.EXPECT().CountActiveValidatorTickets().Return(1, nil)
						sysLogic.logic = mockLogic.Logic
					})

					It("should return err='insufficient live bootstrap tickets'", func() {
						err := sysLogic.CheckSetNetMaturity()
						Expect(err).ToNot(BeNil())
						Expect(err.Error()).To(ContainSubstring("failed to set " +
							"network maturity flag: bad error"))
					})
				})
			})
		})

	})
})
