package keepers

import (
	"fmt"
	"os"

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
	var c storage.Engine
	var err error
	var cfg *config.EngineConfig
	var sysKeeper *SystemKeeper
	var ctrl *gomock.Controller

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		c = storage.NewBadger(cfg)
		Expect(c.Init()).To(BeNil())
		sysKeeper = NewSystemKeeper(c)
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

	Describe(".SaveBlockInfo", func() {
		var info = &types.BlockInfo{AppHash: []byte("stuff"), Height: 1}

		BeforeEach(func() {
			err := sysKeeper.SaveBlockInfo(info)
			Expect(err).To(BeNil())
		})

		It("should store last block info", func() {
			rec, err := c.Get(MakeKeyBlockInfo(info.Height))
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

		When("there are 2 block info stored", func() {
			var info2 = &types.BlockInfo{AppHash: []byte("stuff 2"), Height: 2}
			var info1 = &types.BlockInfo{AppHash: []byte("stuff 1"), Height: 1}
			BeforeEach(func() {
				err := sysKeeper.SaveBlockInfo(info2)
				Expect(err).To(BeNil())
				err = sysKeeper.SaveBlockInfo(info1)
				Expect(err).To(BeNil())
			})

			It("should return the info of the block with the highest height", func() {
				info, err := sysKeeper.GetLastBlockInfo()
				Expect(err).To(BeNil())
				Expect(info).To(BeEquivalentTo(info2))
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
				db := storagemocks.NewMockEngine(ctrl)
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

	Describe(".GetHighestDrandRound", func() {
		It("should return 0 when no round has been saved", func() {
			res, err := sysKeeper.GetHighestDrandRound()
			Expect(err).To(BeNil())
			Expect(res).To(Equal(uint64(0)))
		})

		When("height is set", func() {
			var height = uint64(200)
			BeforeEach(func() {
				rec := storage.NewRecord(MakeHighestDrandRoundKey(), util.EncodeNumber(height))
				err := sysKeeper.db.Put(rec)
				Expect(err).To(BeNil())
			})

			It("should return expected height", func() {
				res, err := sysKeeper.GetHighestDrandRound()
				Expect(err).To(BeNil())
				Expect(res).To(Equal(height))
			})
		})
	})

	Describe(".SetHighestDrandRound", func() {
		var height = uint64(200)

		When("no height has been set", func() {
			BeforeEach(func() {
				err := sysKeeper.SetHighestDrandRound(height)
				Expect(err).To(BeNil())
			})

			It("should set the height", func() {
				rec, err := sysKeeper.db.Get(MakeHighestDrandRoundKey())
				Expect(err).To(BeNil())
				Expect(rec.Value).To(Equal(util.EncodeNumber(height)))
			})
		})

		When("new height is not higher than the existing height", func() {
			var newHeight = uint64(122)
			BeforeEach(func() {
				err := sysKeeper.SetHighestDrandRound(height)
				Expect(err).To(BeNil())
				err = sysKeeper.SetHighestDrandRound(newHeight)
				Expect(err).To(BeNil())
			})

			It("should not set to the new height", func() {
				rec, err := sysKeeper.db.Get(MakeHighestDrandRoundKey())
				Expect(err).To(BeNil())
				Expect(rec.Value).To(Equal(util.EncodeNumber(height)))
			})
		})

		When("new height is higher than the existing height", func() {
			var newHeight = uint64(300)
			BeforeEach(func() {
				err := sysKeeper.SetHighestDrandRound(height)
				Expect(err).To(BeNil())
				err = sysKeeper.SetHighestDrandRound(newHeight)
				Expect(err).To(BeNil())
			})

			It("should not set to the new height", func() {
				rec, err := sysKeeper.db.Get(MakeHighestDrandRoundKey())
				Expect(err).To(BeNil())
				Expect(rec.Value).ToNot(Equal(util.EncodeNumber(height)))
				Expect(rec.Value).To(Equal(util.EncodeNumber(newHeight)))
			})
		})
	})

	Describe(".GetSecrets", func() {

		When("no block information exist", func() {
			It("should return nil and empty result", func() {
				res, err := sysKeeper.GetSecrets(10, 0, 0)
				Expect(err).To(BeNil())
				Expect(res).To(BeEmpty())
			})
		})

		When("an error occurred when attempting to fetch block info", func() {
			var err error
			var returnedErr = fmt.Errorf("error")
			BeforeEach(func() {
				db := storagemocks.NewMockEngine(ctrl)
				db.EXPECT().Get(gomock.Any()).Return(nil, returnedErr)
				sysKeeper.db = db
				_, err = sysKeeper.GetSecrets(10, 0, 0)
			})

			It("should return error", func() {
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(returnedErr))
			})
		})

		When("an error=ErrBlockInfoNotFound occurred when attempting to fetch block info", func() {
			var err error
			BeforeEach(func() {
				db := storagemocks.NewMockEngine(ctrl)
				db.EXPECT().Get(gomock.Any()).Return(nil, ErrBlockInfoNotFound).AnyTimes()
				sysKeeper.db = db
				_, err = sysKeeper.GetSecrets(10, 0, 0)
			})

			It("should return nil", func() {
				Expect(err).To(BeNil())
			})
		})

		When("a block info is includes invalid secret", func() {
			var err error
			var res [][]byte
			BeforeEach(func() {
				rec := storage.NewRecord([]byte("key"), util.ObjectToBytes(types.BlockInfo{
					InvalidEpochSecret: true,
				}))
				db := storagemocks.NewMockEngine(ctrl)
				db.EXPECT().Get(gomock.Any()).Return(rec, nil).AnyTimes()
				sysKeeper.db = db
				res, err = sysKeeper.GetSecrets(10, 0, 0)
			})

			It("should return nil err and empty result", func() {
				Expect(err).To(BeNil())
				Expect(res).To(BeEmpty())
			})
		})

		When("a block info is includes no secret", func() {
			var err error
			var res [][]byte
			BeforeEach(func() {
				rec := storage.NewRecord([]byte("key"), util.ObjectToBytes(types.BlockInfo{}))
				db := storagemocks.NewMockEngine(ctrl)
				db.EXPECT().Get(gomock.Any()).Return(rec, nil).AnyTimes()
				sysKeeper.db = db
				res, err = sysKeeper.GetSecrets(10, 0, 0)
			})

			It("should return nil err and empty result", func() {
				Expect(err).To(BeNil())
				Expect(res).To(BeEmpty())
			})
		})

		When("a block info includes a secret at the starting height=10, skip=1", func() {
			var err error
			var res [][]byte
			BeforeEach(func() {
				rec := storage.NewRecord([]byte("key"), util.ObjectToBytes(types.BlockInfo{
					Height:      10,
					EpochSecret: util.RandBytes(64),
				}))
				db := storagemocks.NewMockEngine(ctrl)
				db.EXPECT().Get(MakeKeyBlockInfo(10)).Return(rec, nil).AnyTimes()
				db.EXPECT().Get(MakeKeyBlockInfo(9)).Return(nil, ErrBlockInfoNotFound).AnyTimes()
				db.EXPECT().Get(MakeKeyBlockInfo(8)).Return(nil, ErrBlockInfoNotFound).AnyTimes()
				db.EXPECT().Get(MakeKeyBlockInfo(7)).Return(nil, ErrBlockInfoNotFound).AnyTimes()
				db.EXPECT().Get(MakeKeyBlockInfo(6)).Return(nil, ErrBlockInfoNotFound).AnyTimes()
				db.EXPECT().Get(MakeKeyBlockInfo(5)).Return(nil, ErrBlockInfoNotFound).AnyTimes()
				db.EXPECT().Get(MakeKeyBlockInfo(4)).Return(nil, ErrBlockInfoNotFound).AnyTimes()
				db.EXPECT().Get(MakeKeyBlockInfo(3)).Return(nil, ErrBlockInfoNotFound).AnyTimes()
				db.EXPECT().Get(MakeKeyBlockInfo(2)).Return(nil, ErrBlockInfoNotFound).AnyTimes()
				db.EXPECT().Get(MakeKeyBlockInfo(1)).Return(nil, ErrBlockInfoNotFound).AnyTimes()
				sysKeeper.db = db
				res, err = sysKeeper.GetSecrets(10, 0, 0)
			})

			It("should return nil err and 1 result", func() {
				Expect(err).To(BeNil())
				Expect(res).ToNot(BeEmpty())
				Expect(res).To(HaveLen(1))
			})
		})

		When("there are two secrets at the height=10 and 5 and skip=5", func() {
			var err error
			var res [][]byte
			BeforeEach(func() {
				rec := storage.NewRecord([]byte("key"), util.ObjectToBytes(types.BlockInfo{
					Height:      10,
					EpochSecret: []byte("a"),
				}))
				rec2 := storage.NewRecord([]byte("key"), util.ObjectToBytes(types.BlockInfo{
					Height:      5,
					EpochSecret: []byte("b"),
				}))
				db := storagemocks.NewMockEngine(ctrl)
				db.EXPECT().Get(MakeKeyBlockInfo(10)).Return(rec, nil).AnyTimes()
				db.EXPECT().Get(MakeKeyBlockInfo(5)).Return(rec2, nil).AnyTimes()
				sysKeeper.db = db
				res, err = sysKeeper.GetSecrets(10, 0, 5)
			})

			It("should return nil err and 2 expected result", func() {
				Expect(err).To(BeNil())
				Expect(res).ToNot(BeEmpty())
				Expect(res).To(HaveLen(2))
				Expect(res[0]).To(Equal([]byte("a")))
				Expect(res[1]).To(Equal([]byte("b")))
			})
		})

		When("there are two secrets at the height=10 and 5 and skip=5 and limit=1", func() {
			var err error
			var res [][]byte
			BeforeEach(func() {
				rec := storage.NewRecord([]byte("key"), util.ObjectToBytes(types.BlockInfo{
					Height:      10,
					EpochSecret: []byte("a"),
				}))
				rec2 := storage.NewRecord([]byte("key"), util.ObjectToBytes(types.BlockInfo{
					Height:      5,
					EpochSecret: []byte("b"),
				}))
				db := storagemocks.NewMockEngine(ctrl)
				db.EXPECT().Get(MakeKeyBlockInfo(10)).Return(rec, nil).AnyTimes()
				db.EXPECT().Get(MakeKeyBlockInfo(5)).Return(rec2, nil).AnyTimes()
				sysKeeper.db = db
				res, err = sysKeeper.GetSecrets(10, 1, 5)
			})

			It("should return nil err and 1 expected result", func() {
				Expect(err).To(BeNil())
				Expect(res).ToNot(BeEmpty())
				Expect(res).To(HaveLen(1))
				Expect(res[0]).To(Equal([]byte("a")))
			})
		})
	})
})
