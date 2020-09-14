package keepers

import (
	"fmt"
	"os"

	"github.com/golang/mock/gomock"
	"github.com/make-os/lobe/config"
	"github.com/make-os/lobe/params"
	"github.com/make-os/lobe/storage/common"
	storagemocks "github.com/make-os/lobe/storage/mocks"
	storagetypes "github.com/make-os/lobe/storage/types"
	"github.com/make-os/lobe/testutil"
	"github.com/make-os/lobe/types/core"
	"github.com/make-os/lobe/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SystemKeeper", func() {
	var appDB storagetypes.Engine
	var err error
	var cfg *config.AppConfig
	var valKeeper *ValidatorKeeper
	var ctrl *gomock.Controller
	var pubKey = util.StrToBytes32("pubkey")

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		appDB, _ = testutil.GetDB()
		valKeeper = NewValidatorKeeper(appDB.NewTx(true, true))
	})

	AfterEach(func() {
		ctrl.Finish()
		Expect(appDB.Close()).To(BeNil())
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".getByHeight", func() {
		When("no result is found", func() {
			It("should return err=nil and empty map", func() {
				res, err := valKeeper.getByHeight(1)
				Expect(err).To(BeNil())
				Expect(res).To(BeEmpty())
			})
		})

		When("db error occurred", func() {
			BeforeEach(func() {
				mockDB := storagemocks.NewMockTx(ctrl)
				mockDB.EXPECT().Get(gomock.Any()).Return(nil, fmt.Errorf("error"))
				valKeeper.db = mockDB
			})

			It("should return err='error' and nil result", func() {
				res, err := valKeeper.getByHeight(1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("error"))
				Expect(res).To(BeNil())
			})
		})

		When("record exist", func() {
			height := int64(1)
			rec := map[util.Bytes32]*core.Validator{util.StrToBytes32("pubkey"): {
				PubKey: util.StrToBytes32("ticket1"),
			}}
			BeforeEach(func() {
				key := MakeBlockValidatorsKey(height)
				err := valKeeper.db.Put(common.NewFromKeyValue(key, util.ToBytes(rec)))
				Expect(err).To(BeNil())
			})

			It("should return err=nil and expected result", func() {
				res, err := valKeeper.getByHeight(height)
				Expect(err).To(BeNil())
				Expect(res).To(Equal(core.BlockValidators(rec)))
			})
		})
	})

	Describe(".Get", func() {
		When("one validator is stored at height=1, search height = 1", func() {
			rec := map[util.Bytes32]*core.Validator{util.StrToBytes32("pubkey"): {PubKey: util.StrToBytes32("ticket1")}}
			BeforeEach(func() {
				key := MakeBlockValidatorsKey(1)
				err := valKeeper.db.Put(common.NewFromKeyValue(key, util.ToBytes(rec)))
				Expect(err).To(BeNil())
			})

			It("should return err=nil and one validator", func() {
				res, err := valKeeper.Get(1)
				Expect(err).To(BeNil())
				Expect(res).To(Equal(core.BlockValidators(rec)))
			})
		})

		When("two two validators exist; valset1 at height 1, valset2 at height 2; argument height = 0", func() {
			valset := map[util.Bytes32]*core.Validator{util.StrToBytes32("pubkey"): {PubKey: util.StrToBytes32("ticket1")}}
			valset2 := map[util.Bytes32]*core.Validator{util.StrToBytes32("pubkey"): {PubKey: util.StrToBytes32("ticket2")}}
			BeforeEach(func() {
				err := valKeeper.db.Put(common.NewFromKeyValue(MakeBlockValidatorsKey(1), util.ToBytes(valset)))
				Expect(err).To(BeNil())
				err = valKeeper.db.Put(common.NewFromKeyValue(MakeBlockValidatorsKey(2), util.ToBytes(valset2)))
				Expect(err).To(BeNil())
			})

			It("should return valset2 since it is the most recent", func() {
				res, err := valKeeper.Get(0)
				Expect(err).To(BeNil())
				Expect(res).To(Equal(core.BlockValidators(valset2)))
			})
		})

		When("two validators exist; valset1 at height 2, valset2 at height 4; argument height = 9; blocks per epoch = 2", func() {
			valset := map[util.Bytes32]*core.Validator{util.StrToBytes32("pubkey"): {PubKey: util.StrToBytes32("ticket1")}}
			valset2 := map[util.Bytes32]*core.Validator{util.StrToBytes32("pubkey"): {PubKey: util.StrToBytes32("ticket2")}}
			BeforeEach(func() {
				params.NumBlocksPerEpoch = 2
				err := valKeeper.db.Put(common.NewFromKeyValue(MakeBlockValidatorsKey(2), util.ToBytes(valset)))
				Expect(err).To(BeNil())
				err = valKeeper.db.Put(common.NewFromKeyValue(MakeBlockValidatorsKey(4), util.ToBytes(valset2)))
				Expect(err).To(BeNil())
			})

			It("should return valset2 since it is the most recent set", func() {
				res, err := valKeeper.Get(9)
				Expect(err).To(BeNil())
				Expect(res).To(Equal(core.BlockValidators(valset2)))
			})
		})

		When("two validators exist; valset1 at height 2, valset2 at height 4; argument height = 10; blocks per epoch = 2", func() {
			valset := map[util.Bytes32]*core.Validator{util.StrToBytes32("pubkey"): {PubKey: util.StrToBytes32("ticket1")}}
			valset2 := map[util.Bytes32]*core.Validator{util.StrToBytes32("pubkey"): {PubKey: util.StrToBytes32("ticket2")}}
			BeforeEach(func() {
				params.NumBlocksPerEpoch = 2
				err := valKeeper.db.Put(common.NewFromKeyValue(MakeBlockValidatorsKey(2), util.ToBytes(valset)))
				Expect(err).To(BeNil())
				err = valKeeper.db.Put(common.NewFromKeyValue(MakeBlockValidatorsKey(4), util.ToBytes(valset2)))
				Expect(err).To(BeNil())
			})

			It("should return valset2 since it is the most recent set", func() {
				res, err := valKeeper.Get(10)
				Expect(err).To(BeNil())
				Expect(res).To(Equal(core.BlockValidators(valset2)))
			})
		})
	})

	Describe(".Index", func() {
		var err error
		When("no issues with database", func() {
			BeforeEach(func() {
				vals := []*core.Validator{{PubKey: pubKey}}
				err = valKeeper.Index(1, vals)
			})

			It("should successfully index validators", func() {
				Expect(err).To(BeNil())
			})

			Specify("that key exist in db", func() {
				rec, err := valKeeper.db.Get(MakeBlockValidatorsKey(1))
				Expect(err).To(BeNil())
				Expect(rec).ToNot(BeNil())
			})
		})

		When("db.Put returns an error", func() {
			BeforeEach(func() {
				mockDB := storagemocks.NewMockTx(ctrl)
				mockDB.EXPECT().Put(gomock.Any()).Return(fmt.Errorf("error"))
				valKeeper.db = mockDB
			})

			BeforeEach(func() {
				vals := []*core.Validator{{PubKey: pubKey}}
				err = valKeeper.Index(1, vals)
			})

			It("should successfully index validators", func() {
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to index validators: error"))
			})
		})
	})
})
