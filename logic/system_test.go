package logic

import (
	"fmt"
	"os"

	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/logic/keepers"
	"github.com/makeos/mosdef/testutil"
	"github.com/makeos/mosdef/util"

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

	Describe(".GetEpoch", func() {
		When("the network is not matured", func() {
			BeforeEach(func() {
				params.NetMaturityHeight = 20
				mockLogic.SysKeeper.EXPECT().GetNetMaturityHeight().Return(uint64(0), fmt.Errorf("error"))
				sysLogic.logic = mockLogic.Logic
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
				mockLogic.SysKeeper.EXPECT().GetNetMaturityHeight().Return(uint64(100), nil)
				sysLogic.logic = mockLogic.Logic
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
				mockLogic.SysKeeper.EXPECT().GetNetMaturityHeight().Return(uint64(100), nil)
				sysLogic.logic = mockLogic.Logic
			})

			It("should curEpoch = 1 and nextEpoch = 1", func() {
				curEpoch, nextEpoch := sysLogic.GetEpoch(199)
				Expect(curEpoch).To(Equal(1))
				Expect(nextEpoch).To(Equal(1))
			})
		})
	})

	Describe(".GetLastEpochSeed", func() {
		When("current epoch is the first (1) epoch", func() {
			It("should return the hash of the genesis file", func() {
				params.NumBlocksPerEpoch = 5
				seed, err := sysLogic.GetLastEpochSeed(3)
				Expect(err).To(BeNil())
				Expect(seed).To(Equal(config.GenesisFileHash()))
			})
		})

		When("unable to get seed block height", func() {
			It("should return err", func() {
				params.NumBlocksPerEpoch = 5
				seed, err := sysLogic.GetLastEpochSeed(7)
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(keepers.ErrBlockInfoNotFound))
				Expect(seed.IsEmpty()).To(BeTrue())
			})
		})

		When("unable to get preceding block of seed block", func() {
			BeforeEach(func() {
				sysLogic.logic.SysKeeper().SaveBlockInfo(&types.BlockInfo{
					Height:          3,
					EpochSeedOutput: util.BytesToBytes32(util.RandBytes(32)),
				})
			})

			It("should return err='..failed to get preceding block of seed block'", func() {
				params.NumBlocksPerEpoch = 5
				seed, err := sysLogic.GetLastEpochSeed(7)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("failed to get preceding block of seed block"))
				Expect(seed.IsEmpty()).To(BeTrue())
			})
		})

		When("successful", func() {
			seed := util.BytesToBytes32(util.RandBytes(32))
			precedingBlockHash := util.RandBytes(32)

			When("seed block and preceding block exist", func() {
				BeforeEach(func() {
					sysLogic.logic.SysKeeper().SaveBlockInfo(&types.BlockInfo{
						Height:          3,
						EpochSeedOutput: seed,
					})
					sysLogic.logic.SysKeeper().SaveBlockInfo(&types.BlockInfo{
						Height: 2,
						Hash:   precedingBlockHash,
					})
				})

				It("should return nil error and seed", func() {
					params.NumBlocksPerEpoch = 5
					seed, err := sysLogic.GetLastEpochSeed(7)
					Expect(err).To(BeNil())
					Expect(seed.IsEmpty()).To(BeFalse())
				})

				Specify("that seed is the hash of precedingBlockHash + seed", func() {
					epochSeed, err := sysLogic.GetLastEpochSeed(7)
					Expect(err).To(BeNil())
					mix := util.Blake2b256(append(precedingBlockHash, seed.Bytes()...))
					Expect(epochSeed.Bytes()).To(Equal(mix))
				})
			})

			When("seed block does not include a seed", func() {
				BeforeEach(func() {
					sysLogic.logic.SysKeeper().SaveBlockInfo(&types.BlockInfo{
						Height:          3,
						EpochSeedOutput: util.EmptyBytes32,
					})
					sysLogic.logic.SysKeeper().SaveBlockInfo(&types.BlockInfo{
						Height: 2,
						Hash:   precedingBlockHash,
					})
				})

				It("should return nil error and seed", func() {
					params.NumBlocksPerEpoch = 5
					seed, err := sysLogic.GetLastEpochSeed(7)
					Expect(err).To(BeNil())
					Expect(seed.IsEmpty()).To(BeFalse())
				})

				Specify("that seed is the preceding block hash", func() {
					epochSeed, err := sysLogic.GetLastEpochSeed(7)
					Expect(err).To(BeNil())
					Expect(epochSeed.Bytes()).To(Equal(precedingBlockHash))
				})
			})
		})
	})

	Describe(".MakeEpochSeedTx", func() {
		When("getting last committed block fails", func() {
			BeforeEach(func() {
				params.NetMaturityHeight = 20
				params.NumBlocksPerEpoch = 100
				mockLogic.SysKeeper.EXPECT().GetLastBlockInfo().Return(nil, fmt.Errorf("error"))
				sysLogic.logic = mockLogic.Logic
			})

			It("should return nil", func() {
				stx, err := sysLogic.MakeEpochSeedTx()
				Expect(err).To(BeNil())
				Expect(stx).To(BeNil())
			})
		})

		When("the next block is not the start of epoch end stage", func() {
			BeforeEach(func() {
				params.NumBlocksPerEpoch = 10
				params.NumBlocksToEffectValChange = 2
				curBlockHeight := int64(6)
				mockLogic.SysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: curBlockHeight}, nil)
				sysLogic.logic = mockLogic.Logic
			})

			It("should return nil", func() {
				stx, err := sysLogic.MakeEpochSeedTx()
				Expect(err).To(BeNil())
				Expect(stx).To(BeNil())
			})
		})

		When("the next block is the start of epoch end stage", func() {
			BeforeEach(func() {
				params.NumBlocksPerEpoch = 10
				params.NumBlocksToEffectValChange = 2
				curBlockHeight := int64(7)
				mockLogic.SysKeeper.EXPECT().GetLastBlockInfo().Return(&types.BlockInfo{Height: curBlockHeight}, nil)

				wpv := crypto.GenerateWrappedPV(util.RandBytes(16))
				g := &config.Globals{PrivVal: wpv}
				cfg := config.EmptyAppConfigWithGlobals(g)
				mockLogic.Logic.EXPECT().Cfg().Return(&cfg)

				sysLogic.logic = mockLogic.Logic
			})

			It("should return seed and no error", func() {
				stx, err := sysLogic.MakeEpochSeedTx()
				Expect(err).To(BeNil())
				Expect(stx).ToNot(BeNil())
				Expect(stx.GetType()).To(Equal(types.TxTypeEpochSeed))
				Expect(stx.(*types.TxEpochSeed).Output).To(HaveLen(32))
				Expect(stx.(*types.TxEpochSeed).Proof).To(HaveLen(96))
			})
		})
	})

	Describe(".MakeSecret", func() {
		When("error is returned while trying to get secret", func() {
			BeforeEach(func() {
				mockLogic.SysKeeper.EXPECT().GetEpochSeeds(gomock.Any(), gomock.Any()).Return(nil, fmt.Errorf("error"))
				sysLogic.logic = mockLogic.Logic
			})

			It("should return err='failed to get secrets: error'", func() {
				_, err := sysLogic.MakeSecret(1)
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(Equal("failed to get secrets: error"))
			})
		})

		When("no secret is found", func() {
			BeforeEach(func() {
				mockLogic.SysKeeper.EXPECT().GetEpochSeeds(gomock.Any(), gomock.Any()).Return(nil, nil)
				sysLogic.logic = mockLogic.Logic
			})

			It("should return err='...no secret found'", func() {
				_, err := sysLogic.MakeSecret(1)
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring(ErrNoSeedFound.Error()))
			})
		})

		When("two secrets (0x06, 0x04) exists", func() {
			var secrets [][]byte = [][]byte{[]byte{0x06}, []byte{0x02}}
			BeforeEach(func() {
				mockLogic.SysKeeper.EXPECT().GetEpochSeeds(gomock.Any(), gomock.Any()).Return(secrets, nil)
				sysLogic.logic = mockLogic.Logic
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
