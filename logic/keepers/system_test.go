package keepers_test

import (
	"os"

	"github.com/makeos/mosdef/types"

	"github.com/makeos/mosdef/logic/keepers"

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
	var sysKeeper *keepers.SystemKeeper

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		c = storage.NewBadger(cfg)
		Expect(c.Init()).To(BeNil())
		sysKeeper = keepers.NewSystemKeeper(c)
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
			rec, err := c.Get(keepers.MakeKeyBlockInfo(info.Height))
			Expect(err).To(BeNil())
			var actual types.BlockInfo
			err = rec.Scan(&actual)
			Expect(err).To(BeNil())
			Expect(info).To(BeEquivalentTo(&actual))
		})
	})

	Describe(".GetLastBlockInfo", func() {
		When("no last block info", func() {
			It("should return ErrLastBlockInfoNotFound", func() {
				_, err := sysKeeper.GetLastBlockInfo()
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(keepers.ErrBlockInfoNotFound))
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
})
