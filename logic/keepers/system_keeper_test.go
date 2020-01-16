package keepers

import (
	"fmt"
	"os"

	"github.com/makeos/mosdef/params"
	storagemocks "github.com/makeos/mosdef/storage/mocks"

	"github.com/golang/mock/gomock"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/testutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SystemKeeper", func() {
	var appDB storage.Engine
	var err error
	var cfg *config.AppConfig
	var sysKeeper *SystemKeeper
	var ctrl *gomock.Controller

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		appDB, _ = testutil.GetDB(cfg)
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
		var info = &types.BlockInfo{AppHash: []byte("stuff"), Height: 1}

		BeforeEach(func() {
			err := sysKeeper.SaveBlockInfo(info)
			Expect(err).To(BeNil())
		})

		It("should store last block info", func() {
			rec, err := appDB.Get(MakeKeyBlockInfo(info.Height))
			Expect(err).To(BeNil())
			var actual types.BlockInfo
			err = rec.Scan(&actual)
			Expect(err).To(BeNil())
			Expect(info).To(BeEquivalentTo(&actual))
		})
	})

	Describe(".GetLastBlockInfo", func() {
		When("no last block info", func() {
			It("should return ErrBlockInfoNotFound", func() {
				_, err := sysKeeper.GetLastBlockInfo()
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(ErrBlockInfoNotFound))
			})
		})

		When("there are 2 blocks info stored", func() {
			var info2 = &types.BlockInfo{AppHash: []byte("stuff 2"), Height: 2}
			var info1 = &types.BlockInfo{AppHash: []byte("stuff 1"), Height: 1}

			BeforeEach(func() {
				err := sysKeeper.SaveBlockInfo(info1)
				Expect(err).To(BeNil())
				err = sysKeeper.SaveBlockInfo(info2)
				Expect(err).To(BeNil())
			})

			It("should return the info of the block with the highest height", func() {
				info, err := sysKeeper.GetLastBlockInfo()
				Expect(err).To(BeNil())
				Expect(info).To(BeEquivalentTo(info2))
				Expect(sysKeeper.lastSaved).To(Equal(info2))
			})
		})
	})

	Describe(".GetBlockInfo", func() {
		When("no block info was not found", func() {
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

		When("there are 2 block info stored", func() {
			var info2 = &types.BlockInfo{AppHash: []byte("stuff 2"), Height: 2}
			var info1 = &types.BlockInfo{AppHash: []byte("stuff 1"), Height: 1}
			BeforeEach(func() {
				err := sysKeeper.SaveBlockInfo(info2)
				Expect(err).To(BeNil())
				err = sysKeeper.SaveBlockInfo(info1)
				Expect(err).To(BeNil())
			})

			It("should find and return block with height=2", func() {
				info, err := sysKeeper.GetBlockInfo(2)
				Expect(err).To(BeNil())
				Expect(info).To(BeEquivalentTo(info2))
			})

			It("should find and return block with height=1", func() {
				info, err := sysKeeper.GetBlockInfo(1)
				Expect(err).To(BeNil())
				Expect(info).To(BeEquivalentTo(info1))
			})
		})
	})

	Describe(".MarkAsMatured", func() {
		It("should successfully add net maturity mark", func() {
			err := sysKeeper.MarkAsMatured(100)
			Expect(err).To(BeNil())
			r, err := sysKeeper.db.Get(MakeNetMaturityKey())
			Expect(err).To(BeNil())
			Expect(r.Value).To(Equal(util.EncodeNumber(100)))
		})
	})

	Describe(".IsMarkedAsMature", func() {
		When("maturity height has been set/marked", func() {
			BeforeEach(func() {
				r, err := sysKeeper.db.Get(MakeNetMaturityKey())
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(storage.ErrRecordNotFound))
				Expect(r).To(BeNil())
				err = sysKeeper.MarkAsMatured(89)
				Expect(err).To(BeNil())
			})

			It("should return true if maturity mark is set", func() {
				isMarked, err := sysKeeper.IsMarkedAsMature()
				Expect(err).To(BeNil())
				Expect(isMarked).To(BeTrue())
			})
		})

		When("maturity height has not been set/marked", func() {
			It("should return false", func() {
				isMarked, err := sysKeeper.IsMarkedAsMature()
				Expect(err).To(BeNil())
				Expect(isMarked).To(BeFalse())
			})
		})
	})

	Describe(".GetNetMaturityHeight", func() {
		When("when matured height is set to 8900", func() {
			BeforeEach(func() {
				err = sysKeeper.MarkAsMatured(8900)
				Expect(err).To(BeNil())
			})

			It("should return expected height=8900", func() {
				h, err := sysKeeper.GetNetMaturityHeight()
				Expect(err).To(BeNil())
				Expect(h).To(Equal(uint64(8900)))
			})
		})

		When("when matured height is not set", func() {
			It("should return err=types.ErrImmatureNetwork", func() {
				_, err := sysKeeper.GetNetMaturityHeight()
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(types.ErrImmatureNetwork))
			})
		})
	})

	Describe(".GetEpochSeeds", func() {

		When("no block information exist", func() {
			It("should return nil and empty result", func() {
				res, err := sysKeeper.GetEpochSeeds(10, 0)
				Expect(err).To(BeNil())
				Expect(res).To(BeEmpty())
			})
		})

		When("an error occurred when attempting to fetch block info", func() {
			var err error
			var returnedErr = fmt.Errorf("error")
			BeforeEach(func() {
				db := storagemocks.NewMockTx(ctrl)
				db.EXPECT().Get(gomock.Any()).Return(nil, returnedErr)
				sysKeeper.db = db
				_, err = sysKeeper.GetEpochSeeds(10, 0)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(returnedErr))
			})
		})

		When("an error=ErrBlockInfoNotFound occurred when attempting to fetch block info", func() {
			var err error
			BeforeEach(func() {
				db := storagemocks.NewMockTx(ctrl)
				db.EXPECT().Get(gomock.Any()).Return(nil, ErrBlockInfoNotFound).AnyTimes()
				sysKeeper.db = db
				_, err = sysKeeper.GetEpochSeeds(10, 0)
			})

			It("should return nil", func() {
				Expect(err).To(BeNil())
			})
		})

		When("seed block does not include a seed", func() {
			var err error
			var res [][]byte
			hash7 := util.RandBytes(32)
			hash2 := util.RandBytes(32)

			BeforeEach(func() {
				params.NumBlocksPerEpoch = 5
				params.NumBlocksToEffectValChange = 2

				db := storagemocks.NewMockTx(ctrl)
				rec8 := storage.NewRecord(nil, util.ObjectToBytes(types.BlockInfo{}))
				rec7 := storage.NewRecord(nil, util.ObjectToBytes(types.BlockInfo{Hash: hash7}))
				db.EXPECT().Get(MakeKeyBlockInfo(8)).Return(rec8, nil)
				db.EXPECT().Get(MakeKeyBlockInfo(7)).Return(rec7, nil)

				rec3 := storage.NewRecord(nil, util.ObjectToBytes(types.BlockInfo{}))
				rec2 := storage.NewRecord(nil, util.ObjectToBytes(types.BlockInfo{Hash: hash2}))
				db.EXPECT().Get(MakeKeyBlockInfo(3)).Return(rec3, nil)
				db.EXPECT().Get(MakeKeyBlockInfo(2)).Return(rec2, nil)

				sysKeeper.db = db
				res, err = sysKeeper.GetEpochSeeds(10, 0)
			})

			Specify("that the hash of the preceding block is returned", func() {
				Expect(err).To(BeNil())
				Expect(res).To(HaveLen(2))
				Expect(res[0]).To(Equal(hash7))
				Expect(res[1]).To(Equal(hash2))
			})

			When("seed bloc includes a secret", func() {
				var err error
				var res [][]byte
				hash8Seed := util.BytesToBytes32(util.RandBytes(32))
				hash7 := util.RandBytes(32)
				hash3Seed := util.BytesToBytes32(util.RandBytes(32))
				hash2 := util.RandBytes(32)

				BeforeEach(func() {
					db := storagemocks.NewMockTx(ctrl)

					rec8 := storage.NewRecord(nil, util.ObjectToBytes(types.BlockInfo{EpochSeedOutput: hash8Seed}))
					rec7 := storage.NewRecord(nil, util.ObjectToBytes(types.BlockInfo{Hash: hash7}))
					db.EXPECT().Get(MakeKeyBlockInfo(8)).Return(rec8, nil).AnyTimes()
					db.EXPECT().Get(MakeKeyBlockInfo(7)).Return(rec7, nil).AnyTimes()

					rec3 := storage.NewRecord(nil, util.ObjectToBytes(types.BlockInfo{EpochSeedOutput: hash3Seed}))
					rec2 := storage.NewRecord(nil, util.ObjectToBytes(types.BlockInfo{Hash: hash2}))
					db.EXPECT().Get(MakeKeyBlockInfo(3)).Return(rec3, nil).AnyTimes()
					db.EXPECT().Get(MakeKeyBlockInfo(2)).Return(rec2, nil).AnyTimes()

					sysKeeper.db = db
					res, err = sysKeeper.GetEpochSeeds(10, 0)
				})

				Specify("that the secrets are a mix of their preceding block hash and the seed", func() {
					Expect(err).To(BeNil())
					Expect(res).To(HaveLen(2))
					Expect(res[0]).To(Equal(util.Blake2b256(append(hash7, hash8Seed.Bytes()...))))
					Expect(res[1]).To(Equal(util.Blake2b256(append(hash2, hash3Seed.Bytes()...))))
				})
			})

			When("limit is 1", func() {
				var err error
				var res [][]byte
				hash8Seed := util.BytesToBytes32(util.RandBytes(32))
				hash7 := util.RandBytes(32)
				hash3Seed := util.BytesToBytes32(util.RandBytes(32))
				hash2 := util.RandBytes(32)

				BeforeEach(func() {
					db := storagemocks.NewMockTx(ctrl)

					rec8 := storage.NewRecord(nil, util.ObjectToBytes(types.BlockInfo{EpochSeedOutput: hash8Seed}))
					rec7 := storage.NewRecord(nil, util.ObjectToBytes(types.BlockInfo{Hash: hash7}))
					db.EXPECT().Get(MakeKeyBlockInfo(8)).Return(rec8, nil).AnyTimes()
					db.EXPECT().Get(MakeKeyBlockInfo(7)).Return(rec7, nil).AnyTimes()

					rec3 := storage.NewRecord(nil, util.ObjectToBytes(types.BlockInfo{EpochSeedOutput: hash3Seed}))
					rec2 := storage.NewRecord(nil, util.ObjectToBytes(types.BlockInfo{Hash: hash2}))
					db.EXPECT().Get(MakeKeyBlockInfo(3)).Return(rec3, nil).AnyTimes()
					db.EXPECT().Get(MakeKeyBlockInfo(2)).Return(rec2, nil).AnyTimes()

					sysKeeper.db = db
					res, err = sysKeeper.GetEpochSeeds(10, 1)
				})

				Specify("that 1 secret is returned", func() {
					Expect(err).To(BeNil())
					Expect(res).To(HaveLen(1))
					Expect(res[0]).To(Equal(util.Blake2b256(append(hash7, hash8Seed.Bytes()...))))
				})
			})
		})

		Describe(".SetLastRepoObjectsSyncHeight", func() {
			var height = uint64(100)
			BeforeEach(func() {
				err = sysKeeper.SetLastRepoObjectsSyncHeight(height)
				Expect(err).To(BeNil())
			})

			It("should set the key and value", func() {
				key := MakeKeyRepoSyncherHeight()
				rec, err := appDB.Get(key)
				Expect(err).To(BeNil())
				var res uint64
				rec.Scan(&res)
				Expect(res).To(Equal(height))
			})
		})

		Describe(".SetLastRepoObjectsSyncHeight", func() {
			var height = uint64(100)
			BeforeEach(func() {
				err = sysKeeper.SetLastRepoObjectsSyncHeight(height)
				Expect(err).To(BeNil())
			})

			It("should return expected height", func() {
				result, err := sysKeeper.GetLastRepoObjectsSyncHeight()
				Expect(err).To(BeNil())
				Expect(height).To(Equal(result))
			})
		})
	})
})
