package keepers

import (
	"fmt"
	"math/big"
	"os"

	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/params"
	"github.com/make-os/kit/storage"
	"github.com/make-os/kit/storage/common"
	storagemocks "github.com/make-os/kit/storage/mocks"
	storagetypes "github.com/make-os/kit/storage/types"
	"github.com/make-os/kit/testutil"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/shopspring/decimal"
	"github.com/spf13/cast"
)

var _ = Describe("SystemKeeper", func() {
	var appDB storagetypes.Engine
	var err error
	var cfg *config.AppConfig
	var sysKeeper *SystemKeeper
	var ctrl *gomock.Controller

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		appDB, _ = testutil.GetDB()
		dbTx := appDB.NewTx(true, true)
		sysKeeper = NewSystemKeeper(dbTx)
	})

	AfterEach(func() {
		ctrl.Finish()
		Expect(appDB.Close()).To(BeNil())
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".SaveBlockInfo", func() {
		var block = &state.BlockInfo{AppHash: []byte("stuff"), Height: 1}

		BeforeEach(func() {
			err := sysKeeper.SaveBlockInfo(block)
			Expect(err).To(BeNil())
		})

		It("should store last block block", func() {
			rec, err := appDB.Get(MakeKeyBlockInfo(block.Height.Int64()))
			Expect(err).To(BeNil())
			var actual state.BlockInfo
			err = rec.Scan(&actual)
			Expect(err).To(BeNil())
			Expect(block).To(BeEquivalentTo(&actual))
		})
	})

	Describe(".GetLastBlockInfo", func() {
		When("no last block block", func() {
			It("should return ErrBlockInfoNotFound", func() {
				_, err := sysKeeper.GetLastBlockInfo()
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(ErrBlockInfoNotFound))
			})
		})

		When("there are 2 blocks block stored", func() {
			var block2 = &state.BlockInfo{AppHash: []byte("stuff 2"), Height: 2}
			var block1 = &state.BlockInfo{AppHash: []byte("stuff 1"), Height: 1}

			BeforeEach(func() {
				err := sysKeeper.SaveBlockInfo(block1)
				Expect(err).To(BeNil())
				err = sysKeeper.SaveBlockInfo(block2)
				Expect(err).To(BeNil())
			})

			It("should return the block of the block with the highest height", func() {
				block, err := sysKeeper.GetLastBlockInfo()
				Expect(err).To(BeNil())
				Expect(block).To(BeEquivalentTo(block2))
				Expect(sysKeeper.lastSaved).To(Equal(block2))
			})
		})
	})

	Describe(".GetBlockInfo", func() {
		When("no block block was not found", func() {
			It("should return ErrBlockInfoNotFound", func() {
				_, err := sysKeeper.GetBlockInfo(2)
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(ErrBlockInfoNotFound))
			})
		})

		When("error is returned", func() {
			err := fmt.Errorf("bad error")
			BeforeEach(func() {
				db := storagemocks.NewMockTx(ctrl)
				db.EXPECT().Get(gomock.Any()).Return(nil, err)
				sysKeeper.db = db
			})

			It("should return ErrBlockInfoNotFound", func() {
				_, err := sysKeeper.GetBlockInfo(2)
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(err))
			})
		})

		When("there are 2 block block stored", func() {
			var block2 = &state.BlockInfo{AppHash: []byte("stuff 2"), Height: 2}
			var block1 = &state.BlockInfo{AppHash: []byte("stuff 1"), Height: 1}
			BeforeEach(func() {
				err := sysKeeper.SaveBlockInfo(block2)
				Expect(err).To(BeNil())
				err = sysKeeper.SaveBlockInfo(block1)
				Expect(err).To(BeNil())
			})

			It("should find and return block with height=2", func() {
				block, err := sysKeeper.GetBlockInfo(2)
				Expect(err).To(BeNil())
				Expect(block).To(BeEquivalentTo(block2))
			})

			It("should find and return block with height=1", func() {
				block, err := sysKeeper.GetBlockInfo(1)
				Expect(err).To(BeNil())
				Expect(block).To(BeEquivalentTo(block1))
			})
		})
	})

	Describe(".SetHelmRepo & GetHelmRepo", func() {
		It("should set and get repo name", func() {
			Expect(sysKeeper.SetHelmRepo("repo1")).To(BeNil())
			repo, err := sysKeeper.GetHelmRepo()
			Expect(err).To(BeNil())
			Expect(repo).To(Equal("repo1"))
		})
	})

	Describe(".GetCurrentEpoch", func() {
		var block1 = &state.BlockInfo{AppHash: []byte("stuff 2"), Height: 200}
		BeforeEach(func() {
			params.NumBlocksPerEpoch = 10
			err = sysKeeper.SaveBlockInfo(block1)
			Expect(err).To(BeNil())
		})

		It("should return expected epoch=20", func() {
			epoch, err := sysKeeper.GetCurrentEpoch()
			Expect(err).To(BeNil())
			Expect(epoch).To(Equal(int64(20)))
		})
	})

	Describe(".GetEpochAt", func() {
		It("should get correct epoch", func() {
			params.NumBlocksPerEpoch = 10
			Expect(sysKeeper.GetEpochAt(10)).To(Equal(int64(1)))
			Expect(sysKeeper.GetEpochAt(11)).To(Equal(int64(2)))
		})
	})

	Describe(".GetCurrentEpochStartBlock", func() {
		It("should return expected block info", func() {
			params.NumBlocksPerEpoch = 10
			var block10 = &state.BlockInfo{AppHash: []byte("hash1"), Height: 10}
			var block11 = &state.BlockInfo{AppHash: []byte("hash2"), Height: 11}
			var block12 = &state.BlockInfo{AppHash: []byte("hash2"), Height: 12}
			sysKeeper.SaveBlockInfo(block10)
			sysKeeper.SaveBlockInfo(block11)
			sysKeeper.SaveBlockInfo(block12)
			sb, err := sysKeeper.GetCurrentEpochStartBlock()
			Expect(err).To(BeNil())
			Expect(sb.Height.Int64()).To(Equal(int64(11)))
		})

		It("should return error if unable to get start block", func() {
			params.NumBlocksPerEpoch = 10
			var block12 = &state.BlockInfo{AppHash: []byte("hash2"), Height: 12}
			sysKeeper.SaveBlockInfo(block12)
			_, err := sysKeeper.GetCurrentEpochStartBlock()
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("failed to get first block info: block info not found"))
		})
	})

	Describe(".RegisterWorkNonce & .IsWorkNonceRegistered", func() {
		It("should register nonce for epoch and delete lower epoch", func() {
			sysKeeper.RegisterWorkNonce(1, 90)
			sysKeeper.RegisterWorkNonce(2, 100)
			sysKeeper.RegisterWorkNonce(2, 101)
			Expect(sysKeeper.IsWorkNonceRegistered(1, 90)).To(Equal(storage.ErrRecordNotFound))
			Expect(sysKeeper.IsWorkNonceRegistered(2, 100)).To(BeNil())
			Expect(sysKeeper.IsWorkNonceRegistered(2, 101)).To(BeNil())
		})
	})

	Describe(".IndexNodeWork & .GetNodeWorks", func() {
		It("should index work and return expected result", func() {
			err := sysKeeper.IndexNodeWork(1, 10)
			Expect(err).To(BeNil())
			sysKeeper.IndexNodeWork(2, 10)
			sysKeeper.IndexNodeWork(3, 10)
			res, err := sysKeeper.GetNodeWorks()
			Expect(err).To(BeNil())
			Expect(res).To(HaveLen(3))
			Expect(res[0].Epoch).To(Equal(int64(1)))
			Expect(res[1].Epoch).To(Equal(int64(2)))
			Expect(res[2].Epoch).To(Equal(int64(3)))
		})

		When("number indexed equal limit", func() {
			It("should slice earliest records and maintain limit", func() {
				NodeWorkIndexLimit = 2
				err := sysKeeper.IndexNodeWork(1, 10)
				Expect(err).To(BeNil())
				sysKeeper.IndexNodeWork(2, 10)
				sysKeeper.IndexNodeWork(3, 10)
				res, err := sysKeeper.GetNodeWorks()
				Expect(err).To(BeNil())
				Expect(res).To(HaveLen(2))
				Expect(res[0].Epoch).To(Equal(int64(2)))
				Expect(res[1].Epoch).To(Equal(int64(3)))
			})
		})
	})

	Describe(".IncrGasMinedInCurEpoch & .GetTotalGasMinedInEpoch", func() {
		It("should update balance correctly", func() {
			params.NumBlocksPerEpoch = 1
			var block1 = &state.BlockInfo{AppHash: []byte("hash1"), Height: 1}
			sysKeeper.SaveBlockInfo(block1)

			bal, err := sysKeeper.GetTotalGasMinedInEpoch(1)
			Expect(err).To(BeNil())
			Expect(bal.String()).To(Equal("0"))

			sysKeeper.IncrGasMinedInCurEpoch("100")
			bal, err = sysKeeper.GetTotalGasMinedInEpoch(1)
			Expect(err).To(BeNil())
			Expect(bal.String()).To(Equal("100"))

			sysKeeper.IncrGasMinedInCurEpoch("100")
			bal, err = sysKeeper.GetTotalGasMinedInEpoch(1)
			Expect(err).To(BeNil())
			Expect(bal.String()).To(Equal("200"))

			// Epoch 2
			var block2 = &state.BlockInfo{AppHash: []byte("hash2"), Height: 2}
			sysKeeper.SaveBlockInfo(block2)

			sysKeeper.IncrGasMinedInCurEpoch("100")
			bal, err = sysKeeper.GetTotalGasMinedInEpoch(2)
			Expect(err).To(BeNil())
			Expect(bal.String()).To(Equal("100"))
		})
	})

	Describe(".GetCurrentDifficulty", func() {
		It("should return minimum difficulty if no difficulty has been recorded before", func() {
			diff, err := sysKeeper.GetCurrentDifficulty()
			Expect(err).To(BeNil())
			Expect(diff.Int64()).To(Equal(params.MinDifficulty.Int64()))
		})

		It("should return expected difficulty if previously set", func() {
			diff := new(big.Int).SetInt64(200000)
			record := common.NewFromKeyValue(MakeDifficultyKey(), util.EncodeNumber(diff.Uint64()))
			err := sysKeeper.db.Put(record)
			Expect(err).To(BeNil())

			curDiff, err := sysKeeper.GetCurrentDifficulty()
			Expect(err).To(BeNil())
			Expect(curDiff.Int64()).To(Equal(diff.Int64()))
		})
	})

	Describe(".AvgGasMinedLastEpochs", func() {
		When("blocks per epoch = 1; num. previous epoch = 12; cur. epoch = 24", func() {
			It("should return expected avg. gas mined", func() {
				params.NumBlocksPerEpoch = 1

				// Epoch 12 (10,000 gas mined)
				sysKeeper.SaveBlockInfo(&state.BlockInfo{AppHash: []byte("hash1"), Height: 12})
				sysKeeper.IncrGasMinedInCurEpoch("10000")

				// Epoch 13 (5,000 gas mined)
				sysKeeper.SaveBlockInfo(&state.BlockInfo{AppHash: []byte("hash1"), Height: 13})
				sysKeeper.IncrGasMinedInCurEpoch("5000")

				// Epoch 14 (3,000 gas mined)
				sysKeeper.SaveBlockInfo(&state.BlockInfo{AppHash: []byte("hash1"), Height: 14})
				sysKeeper.IncrGasMinedInCurEpoch("3000")

				// Epoch 20 (2,000 gas mined)
				sysKeeper.SaveBlockInfo(&state.BlockInfo{AppHash: []byte("hash1"), Height: 20})
				sysKeeper.IncrGasMinedInCurEpoch("2000")

				// Epoch 23 (1,000 gas mined)
				sysKeeper.SaveBlockInfo(&state.BlockInfo{AppHash: []byte("hash1"), Height: 23})
				sysKeeper.IncrGasMinedInCurEpoch("1000")

				// Epoch 24 (1,500 gas mined)
				sysKeeper.SaveBlockInfo(&state.BlockInfo{AppHash: []byte("hash1"), Height: 24})
				sysKeeper.IncrGasMinedInCurEpoch("1500")

				nPrevEpoch := 12.0

				// Calculate expected average (exclude epoch 24 - current epoch)
				expectedAvg := float64(10000+5000+3000+2000+1000) / nPrevEpoch
				res, err := sysKeeper.AvgGasMinedLastEpochs(int64(nPrevEpoch))
				Expect(err).To(BeNil())
				Expect(res.String()).To(Equal(cast.ToString(expectedAvg)))
			})
		})

		When("blocks per epoch = 1; num. previous epoch = 20; cur. epoch = 15", func() {
			It("should return expected avg. gas mined", func() {
				params.NumBlocksPerEpoch = 1

				// Epoch 12 (10,000 gas mined)
				sysKeeper.SaveBlockInfo(&state.BlockInfo{AppHash: []byte("hash1"), Height: 12})
				sysKeeper.IncrGasMinedInCurEpoch("10000")

				// Epoch 13 (5,000 gas mined)
				sysKeeper.SaveBlockInfo(&state.BlockInfo{AppHash: []byte("hash1"), Height: 13})
				sysKeeper.IncrGasMinedInCurEpoch("5000")

				// Epoch 14 (3,000 gas mined)
				sysKeeper.SaveBlockInfo(&state.BlockInfo{AppHash: []byte("hash1"), Height: 14})
				sysKeeper.IncrGasMinedInCurEpoch("3000")

				// Epoch 15 (1,500 gas mined)
				sysKeeper.SaveBlockInfo(&state.BlockInfo{AppHash: []byte("hash1"), Height: 15})
				sysKeeper.IncrGasMinedInCurEpoch("1500")

				nPrevEpoch := 20.0
				nEpochInWindow := 14.0

				// Calculate expected average (exclude epoch 24 - current epoch)
				expectedAvg := decimal.NewFromFloat(float64(10000+5000+3000) / nEpochInWindow)
				res, err := sysKeeper.AvgGasMinedLastEpochs(int64(nPrevEpoch))
				Expect(err).To(BeNil())
				Expect(res.StringFixed(2)).To(Equal(expectedAvg.StringFixed(2)))
			})
		})

		When("blocks per epoch = 1; num. previous epoch = 2; cur. epoch = 15", func() {
			It("should return expected avg. gas mined", func() {
				params.NumBlocksPerEpoch = 1

				// Epoch 12 (10,000 gas mined)
				sysKeeper.SaveBlockInfo(&state.BlockInfo{AppHash: []byte("hash1"), Height: 12})
				sysKeeper.IncrGasMinedInCurEpoch("10000")

				// Epoch 13 (5,000 gas mined)
				sysKeeper.SaveBlockInfo(&state.BlockInfo{AppHash: []byte("hash1"), Height: 13})
				sysKeeper.IncrGasMinedInCurEpoch("5000")

				// Epoch 14 (3,000 gas mined)
				sysKeeper.SaveBlockInfo(&state.BlockInfo{AppHash: []byte("hash1"), Height: 14})
				sysKeeper.IncrGasMinedInCurEpoch("3000")

				// Epoch 15 (1,500 gas mined)
				sysKeeper.SaveBlockInfo(&state.BlockInfo{AppHash: []byte("hash1"), Height: 15})
				sysKeeper.IncrGasMinedInCurEpoch("1500")

				nPrevEpoch := 2.0
				nEpochInWindow := 2.0

				// Calculate expected average (exclude epoch 24 - current epoch)
				expectedAvg := decimal.NewFromFloat(float64(5000+3000) / nEpochInWindow)
				res, err := sysKeeper.AvgGasMinedLastEpochs(int64(nPrevEpoch))
				Expect(err).To(BeNil())
				Expect(res.StringFixed(2)).To(Equal(expectedAvg.StringFixed(2)))
			})
		})
	})

	Describe(".ComputeDifficulty", func() {
		It("should increment difficulty if average total gas mined in last epoch window"+
			" is greater or equal to minimum expected gas mined in an epoch", func() {
			params.NumBlocksPerEpoch = 1
			params.GasReward = "10000"
			params.MinDifficulty = new(big.Int).SetInt64(1000000)
			params.MinTotalGasRewardPerEpoch = "10000"

			// Epoch 1 (10000 gas mined)
			sysKeeper.SaveBlockInfo(&state.BlockInfo{AppHash: []byte("hash1"), Height: 1})
			sysKeeper.IncrGasMinedInCurEpoch("10000")

			// Epoch 2 (10000 gas mined)
			sysKeeper.SaveBlockInfo(&state.BlockInfo{AppHash: []byte("hash1"), Height: 2})
			sysKeeper.IncrGasMinedInCurEpoch("10000")

			// Epoch 3 (10000 gas mined)
			sysKeeper.SaveBlockInfo(&state.BlockInfo{AppHash: []byte("hash1"), Height: 3})
			sysKeeper.IncrGasMinedInCurEpoch("10000")

			// Epoch 4 (10000 gas mined)
			sysKeeper.SaveBlockInfo(&state.BlockInfo{AppHash: []byte("hash1"), Height: 4})
			sysKeeper.IncrGasMinedInCurEpoch("10000")

			// Set initial difficulty
			initialDiff := new(big.Int).SetInt64(2000000)
			record := common.NewFromKeyValue(MakeDifficultyKey(), util.EncodeNumber(initialDiff.Uint64()))
			err = sysKeeper.db.Put(record)
			Expect(err).To(BeNil())

			err := sysKeeper.ComputeDifficulty()
			Expect(err).To(BeNil())

			curDiff, err := sysKeeper.GetCurrentDifficulty()
			Expect(err).To(BeNil())

			points := decimal.NewFromBigInt(initialDiff, 0).Mul(decimal.NewFromFloat(params.DifficultyChangePct))
			expected := decimal.NewFromBigInt(initialDiff, 0).Add(points)
			Expect(curDiff.Int64()).To(Equal(expected.IntPart()))
		})

		It("should decrement difficulty if average total gas mined is less than gas reward", func() {
			params.NumBlocksPerEpoch = 1
			params.GasReward = "10000"
			params.MinDifficulty = new(big.Int).SetInt64(1000000)
			params.MinTotalGasRewardPerEpoch = "10000"

			// Epoch 1 (10,000 gas mined)
			sysKeeper.SaveBlockInfo(&state.BlockInfo{AppHash: []byte("hash1"), Height: 1})
			sysKeeper.IncrGasMinedInCurEpoch("5000")

			// Epoch 2 (5,000 gas mined)
			sysKeeper.SaveBlockInfo(&state.BlockInfo{AppHash: []byte("hash1"), Height: 2})
			sysKeeper.IncrGasMinedInCurEpoch("10000")

			// Epoch 3 (3,000 gas mined)
			sysKeeper.SaveBlockInfo(&state.BlockInfo{AppHash: []byte("hash1"), Height: 3})
			sysKeeper.IncrGasMinedInCurEpoch("5000")

			// Epoch 4 (1,500 gas mined)
			sysKeeper.SaveBlockInfo(&state.BlockInfo{AppHash: []byte("hash1"), Height: 4})
			sysKeeper.IncrGasMinedInCurEpoch("10000")

			// Set initial difficulty
			initialDiff := new(big.Int).SetInt64(2000000)
			record := common.NewFromKeyValue(MakeDifficultyKey(), util.EncodeNumber(initialDiff.Uint64()))
			err = sysKeeper.db.Put(record)
			Expect(err).To(BeNil())

			err := sysKeeper.ComputeDifficulty()
			Expect(err).To(BeNil())

			curDiff, err := sysKeeper.GetCurrentDifficulty()
			Expect(err).To(BeNil())

			points := decimal.NewFromBigInt(initialDiff, 0).Mul(decimal.NewFromFloat(params.DifficultyChangePct))
			expected := decimal.NewFromBigInt(initialDiff, 0).Sub(points)
			Expect(curDiff.Int64()).To(Equal(expected.IntPart()))
		})

		When("difficulty is to be reduced below minimum difficulty", func() {
			It("should reset difficulty to minimum difficulty", func() {
				params.NumBlocksPerEpoch = 1
				params.GasReward = "10000"
				params.MinDifficulty = new(big.Int).SetInt64(1000000)

				// Epoch 1 (10,000 gas mined)
				sysKeeper.SaveBlockInfo(&state.BlockInfo{AppHash: []byte("hash1"), Height: 1})
				sysKeeper.IncrGasMinedInCurEpoch("5000")

				// Epoch 2 (5,000 gas mined)
				sysKeeper.SaveBlockInfo(&state.BlockInfo{AppHash: []byte("hash1"), Height: 2})
				sysKeeper.IncrGasMinedInCurEpoch("10000")

				// Epoch 3 (3,000 gas mined)
				sysKeeper.SaveBlockInfo(&state.BlockInfo{AppHash: []byte("hash1"), Height: 3})
				sysKeeper.IncrGasMinedInCurEpoch("5000")

				// Epoch 4 (1,500 gas mined)
				sysKeeper.SaveBlockInfo(&state.BlockInfo{AppHash: []byte("hash1"), Height: 4})
				sysKeeper.IncrGasMinedInCurEpoch("10000")

				// Set initial difficulty
				initialDiff := new(big.Int).SetInt64(1000000)
				record := common.NewFromKeyValue(MakeDifficultyKey(), util.EncodeNumber(initialDiff.Uint64()))
				err = sysKeeper.db.Put(record)
				Expect(err).To(BeNil())

				err := sysKeeper.ComputeDifficulty()
				Expect(err).To(BeNil())

				curDiff, err := sysKeeper.GetCurrentDifficulty()
				Expect(err).To(BeNil())

				Expect(curDiff.Int64()).To(Equal(params.MinDifficulty.Int64()))
			})
		})
	})
})
