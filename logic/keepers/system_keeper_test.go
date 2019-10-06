package keepers

import (
	"os"

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

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		c = storage.NewBadger(cfg)
		Expect(c.Init()).To(BeNil())
		sysKeeper = NewSystemKeeper(c)
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
})
