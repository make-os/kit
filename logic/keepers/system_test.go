package keepers

import (
	"fmt"
	"os"

	"github.com/golang/mock/gomock"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/params"
	"github.com/make-os/kit/storage"
	storagemocks "github.com/make-os/kit/storage/mocks"
	storagetypes "github.com/make-os/kit/storage/types"
	"github.com/make-os/kit/testutil"
	"github.com/make-os/kit/types/state"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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
})
