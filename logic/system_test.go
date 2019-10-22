package logic

import (
	"fmt"
	"os"

	"github.com/makeos/mosdef/crypto/rand"
	randmocks "github.com/makeos/mosdef/crypto/rand/mocks"
	"github.com/makeos/mosdef/logic/keepers"
	"github.com/makeos/mosdef/types/mocks"

	"github.com/golang/mock/gomock"
	"github.com/makeos/mosdef/types"

	"github.com/makeos/mosdef/params"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/testutil"
)

var _ = Describe("System", func() {
	var c storage.Engine
	var err error
	var cfg *config.EngineConfig
	var logic *Logic
	var sysLogic *System
	var ctrl *gomock.Controller

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		c = storage.NewBadger(cfg)
		Expect(c.Init()).To(BeNil())
		logic = New(c, cfg)
		sysLogic = &System{logic: logic}
	})

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	AfterEach(func() {
		Expect(c.Close()).To(BeNil())
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
				mockSysKeeper := mocks.NewMockSystemKeeper(ctrl)
				mockSysKeeper.EXPECT().IsMarkedAsMature().Return(false, fmt.Errorf("bad error"))
				mockLogic := mocks.NewMockLogic(ctrl)
				mockLogic.EXPECT().SysKeeper().Return(mockSysKeeper)
				sysLogic.logic = mockLogic
			})

			It("should return err='failed to determine network maturity status'", func() {
				err := sysLogic.CheckSetNetMaturity()
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(Equal("failed to determine network maturity status: bad error"))
			})
		})

		When("network has been marked as matured", func() {
			BeforeEach(func() {
				mockSysKeeper := mocks.NewMockSystemKeeper(ctrl)
				mockSysKeeper.EXPECT().IsMarkedAsMature().Return(true, nil)
				mockLogic := mocks.NewMockLogic(ctrl)
				mockLogic.EXPECT().SysKeeper().Return(mockSysKeeper)
				sysLogic.logic = mockLogic
			})

			It("should return nil", func() {
				err := sysLogic.CheckSetNetMaturity()
				Expect(err).To(BeNil())
			})
		})

		When("network has not been marked as matured and no recent committed block", func() {
			BeforeEach(func() {
				mockSysKeeper := mocks.NewMockSystemKeeper(ctrl)
				mockSysKeeper.EXPECT().IsMarkedAsMature().Return(false, nil)
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(nil, keepers.ErrBlockInfoNotFound)
				mockLogic := mocks.NewMockLogic(ctrl)
				mockLogic.EXPECT().SysKeeper().Return(mockSysKeeper).Times(2)
				sysLogic.logic = mockLogic
			})

			It("should return err='no committed block yet'", func() {
				err := sysLogic.CheckSetNetMaturity()
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("no committed block yet"))
			})
		})

		When("network has not been marked as matured and an error occurred when fetching last committed block", func() {
			BeforeEach(func() {
				mockSysKeeper := mocks.NewMockSystemKeeper(ctrl)
				mockSysKeeper.EXPECT().IsMarkedAsMature().Return(false, nil)
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(nil, fmt.Errorf("bad error"))
				mockLogic := mocks.NewMockLogic(ctrl)
				mockLogic.EXPECT().SysKeeper().Return(mockSysKeeper).Times(2)
				sysLogic.logic = mockLogic
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
					mockSysKeeper := mocks.NewMockSystemKeeper(ctrl)
					mockSysKeeper.EXPECT().IsMarkedAsMature().Return(false, nil)
					mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: 10}, nil)
					mockLogic := mocks.NewMockLogic(ctrl)
					mockLogic.EXPECT().SysKeeper().Return(mockSysKeeper).Times(2)
					sysLogic.logic = mockLogic
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
						mockSysKeeper := mocks.NewMockSystemKeeper(ctrl)
						mockSysKeeper.EXPECT().IsMarkedAsMature().Return(false, nil)
						mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: 10}, nil)
						mockTicketMgr := mocks.NewMockTicketManager(ctrl)
						mockTicketMgr.EXPECT().CountLiveValidatorsValidatorTickets().Return(0, fmt.Errorf("bad error"))
						mockLogic := mocks.NewMockLogic(ctrl)
						mockLogic.EXPECT().SysKeeper().Return(mockSysKeeper).Times(2)
						mockLogic.EXPECT().GetTicketManager().Return(mockTicketMgr)
						sysLogic.logic = mockLogic
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
						mockSysKeeper := mocks.NewMockSystemKeeper(ctrl)
						mockSysKeeper.EXPECT().IsMarkedAsMature().Return(false, nil)
						mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: 10}, nil)
						mockTicketMgr := mocks.NewMockTicketManager(ctrl)
						mockTicketMgr.EXPECT().CountLiveValidatorsValidatorTickets().Return(1, nil)
						mockLogic := mocks.NewMockLogic(ctrl)
						mockLogic.EXPECT().SysKeeper().Return(mockSysKeeper).Times(2)
						mockLogic.EXPECT().GetTicketManager().Return(mockTicketMgr)
						sysLogic.logic = mockLogic
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
						mockSysKeeper := mocks.NewMockSystemKeeper(ctrl)
						mockSysKeeper.EXPECT().IsMarkedAsMature().Return(false, nil)
						mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: 10}, nil)
						mockSysKeeper.EXPECT().MarkAsMatured(uint64(10)).Return(fmt.Errorf("bad error"))
						mockTicketMgr := mocks.NewMockTicketManager(ctrl)
						mockTicketMgr.EXPECT().CountLiveValidatorsValidatorTickets().Return(1, nil)
						mockLogic := mocks.NewMockLogic(ctrl)
						mockLogic.EXPECT().SysKeeper().Return(mockSysKeeper).Times(3)
						mockLogic.EXPECT().GetTicketManager().Return(mockTicketMgr)
						sysLogic.logic = mockLogic
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

	Describe(".GetEpoch", func() {
		When("the network is not matured", func() {
			BeforeEach(func() {
				params.NetMaturityHeight = 20
				mockSysKeeper := mocks.NewMockSystemKeeper(ctrl)
				mockSysKeeper.EXPECT().GetNetMaturityHeight().Return(uint64(0), fmt.Errorf("error"))
				mockLogic := mocks.NewMockLogic(ctrl)
				mockLogic.EXPECT().SysKeeper().Return(mockSysKeeper).Times(1)
				sysLogic.logic = mockLogic
			})

			It("should panic", func() {
				Expect(func() {
					sysLogic.GetEpoch(1)
				}).To(Panic())
			})
		})

		When("current height = 200, epoch start height = 100, block per epoch = 100", func() {
			BeforeEach(func() {
				params.NetMaturityHeight = 20
				params.NumBlocksPerEpoch = 100
				mockSysKeeper := mocks.NewMockSystemKeeper(ctrl)
				mockSysKeeper.EXPECT().GetNetMaturityHeight().Return(uint64(100), nil)
				mockLogic := mocks.NewMockLogic(ctrl)
				mockLogic.EXPECT().SysKeeper().Return(mockSysKeeper).Times(1)
				sysLogic.logic = mockLogic
			})

			It("should curEpoch = 1 and nextEpoch = 2", func() {
				curEpoch, nextEpoch := sysLogic.GetEpoch(200)
				Expect(curEpoch).To(Equal(1))
				Expect(nextEpoch).To(Equal(2))
			})
		})

		When("current height = 199, epoch start height = 100, block per epoch = 100", func() {
			BeforeEach(func() {
				params.NetMaturityHeight = 20
				params.NumBlocksPerEpoch = 100
				mockSysKeeper := mocks.NewMockSystemKeeper(ctrl)
				mockSysKeeper.EXPECT().GetNetMaturityHeight().Return(uint64(100), nil)
				mockLogic := mocks.NewMockLogic(ctrl)
				mockLogic.EXPECT().SysKeeper().Return(mockSysKeeper).Times(1)
				sysLogic.logic = mockLogic
			})

			It("should curEpoch = 1 and nextEpoch = 1", func() {
				curEpoch, nextEpoch := sysLogic.GetEpoch(199)
				Expect(curEpoch).To(Equal(1))
				Expect(nextEpoch).To(Equal(1))
			})
		})
	})

	Describe(".GetCurretEpochSecretTx", func() {
		When("getting last committed block fails", func() {
			BeforeEach(func() {
				params.NetMaturityHeight = 20
				params.NumBlocksPerEpoch = 100
				mockSysKeeper := mocks.NewMockSystemKeeper(ctrl)
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(nil, fmt.Errorf("error"))
				mockLogic := mocks.NewMockLogic(ctrl)
				mockLogic.EXPECT().SysKeeper().Return(mockSysKeeper).Times(1)
				sysLogic.logic = mockLogic
			})

			It("should return nil", func() {
				stx, err := sysLogic.GetCurretEpochSecretTx()
				Expect(err).To(BeNil())
				Expect(stx).To(BeNil())
			})
		})

		When("last committed block height is 8; params.NumBlocksPerEpoch = 10", func() {
			BeforeEach(func() {
				params.NumBlocksPerEpoch = 10
				mockSysKeeper := mocks.NewMockSystemKeeper(ctrl)
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: 8}, nil)
				mockLogic := mocks.NewMockLogic(ctrl)
				mockLogic.EXPECT().SysKeeper().Return(mockSysKeeper).Times(1)
				sysLogic.logic = mockLogic
			})

			It("should return nil", func() {
				stx, err := sysLogic.GetCurretEpochSecretTx()
				Expect(err).To(BeNil())
				Expect(stx).To(BeNil())
			})
		})

		When("last committed block height is 9; params.NumBlocksPerEpoch = 10", func() {
			expected := &rand.DrandRandData{
				Previous: []byte("prev"),
				Round:    1,
				Randomness: &rand.DRandRandomness{
					Gid:   21,
					Point: []byte("secret"),
				},
			}
			BeforeEach(func() {
				params.NumBlocksPerEpoch = 10
				mockSysKeeper := mocks.NewMockSystemKeeper(ctrl)
				mockSysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: 9}, nil)
				mockDrand := randmocks.NewMockDRander(ctrl)
				mockDrand.EXPECT().Get(0).Return(expected)
				mockLogic := mocks.NewMockLogic(ctrl)
				mockLogic.EXPECT().GetDRand().Return(mockDrand)
				mockLogic.EXPECT().SysKeeper().Return(mockSysKeeper).Times(1)
				sysLogic.logic = mockLogic
			})

			It("should return nil", func() {
				stx, err := sysLogic.GetCurretEpochSecretTx()
				Expect(err).To(BeNil())
				Expect(stx).ToNot(BeNil())
				Expect(stx.GetType()).To(Equal(types.TxTypeEpochSecret))
				Expect(stx.GetSecret()).To(Equal([]byte(expected.Randomness.Point)))
				Expect(stx.GetPreviousSecret()).To(Equal([]byte(expected.Previous)))
				Expect(stx.GetSecretRound()).To(Equal(expected.Round))
			})
		})
	})

	Describe(".MakeSecret", func() {
		When("error is returned while trying to get secret", func() {
			BeforeEach(func() {
				mockSysKeeper := mocks.NewMockSystemKeeper(ctrl)
				mockSysKeeper.EXPECT().GetSecrets(gomock.Any(), gomock.Any(),
					gomock.Any()).Return(nil, fmt.Errorf("error"))
				mockLogic := mocks.NewMockLogic(ctrl)
				mockLogic.EXPECT().SysKeeper().Return(mockSysKeeper).AnyTimes()
				sysLogic.logic = mockLogic
			})

			It("should return err='failed to get secrets: error'", func() {
				_, err := sysLogic.MakeSecret(1)
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(Equal("failed to get secrets: error"))
			})
		})

		When("no secret is found", func() {
			BeforeEach(func() {
				mockSysKeeper := mocks.NewMockSystemKeeper(ctrl)
				mockSysKeeper.EXPECT().GetSecrets(gomock.Any(), gomock.Any(),
					gomock.Any()).Return(nil, nil)
				mockLogic := mocks.NewMockLogic(ctrl)
				mockLogic.EXPECT().SysKeeper().Return(mockSysKeeper).AnyTimes()
				sysLogic.logic = mockLogic
			})

			It("should return err='...no secret found'", func() {
				_, err := sysLogic.MakeSecret(1)
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring(ErrNoSecretFound.Error()))
			})
		})

		When("two secrets (0x06, 0x04) exists", func() {
			var secrets [][]byte = [][]byte{[]byte{0x06}, []byte{0x02}}
			BeforeEach(func() {
				mockSysKeeper := mocks.NewMockSystemKeeper(ctrl)
				mockSysKeeper.EXPECT().GetSecrets(gomock.Any(), gomock.Any(),
					gomock.Any()).Return(secrets, nil)
				mockLogic := mocks.NewMockLogic(ctrl)
				mockLogic.EXPECT().SysKeeper().Return(mockSysKeeper).AnyTimes()
				sysLogic.logic = mockLogic
			})

			It("should return 0x04 and no error", func() {
				res, err := sysLogic.MakeSecret(1)
				Expect(err).To(BeNil())
				Expect(res).To(Equal([]uint8{
					0x04,
				}))
			})
		})
	})
})
