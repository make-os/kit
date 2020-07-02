package keepers

import (
	"fmt"
	"os"

	storagemocks "gitlab.com/makeos/mosdef/storage/mocks"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/txns"

	"github.com/golang/mock/gomock"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/storage"
	"gitlab.com/makeos/mosdef/testutil"
)

var _ = Describe("TxKeeper", func() {
	var appDB storage.Engine
	var err error
	var cfg *config.AppConfig
	var txKeeper *TxKeeper
	var ctrl *gomock.Controller

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		appDB, _ = testutil.GetDB(cfg)
		dbTx := appDB.NewTx(true, true)
		txKeeper = NewTxKeeper(dbTx)
	})

	AfterEach(func() {
		ctrl.Finish()
		Expect(appDB.Close()).To(BeNil())
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".Index", func() {
		When("db operation failed", func() {
			BeforeEach(func() {
				mockDB := storagemocks.NewMockTx(ctrl)
				mockDB.EXPECT().Put(gomock.Any()).Return(fmt.Errorf("error"))
				txKeeper.db = mockDB
			})

			It("should return err='failed to index tx: error'", func() {
				tx := txns.NewBareTxCoinTransfer()
				err := txKeeper.Index(tx)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to index tx: error"))
			})
		})

		When("index is successful", func() {
			tx := txns.NewBareTxCoinTransfer()

			BeforeEach(func() {
				err := txKeeper.Index(tx)
				Expect(err).To(BeNil())
			})

			It("should return nil", func() {
				rec, err := txKeeper.db.Get(MakeTxKey(tx.GetHash()))
				Expect(err).To(BeNil())
				Expect(rec.Value).To(Equal(tx.Bytes()))
			})
		})
	})

	Describe(".GetTx", func() {
		When("db operation failed", func() {
			BeforeEach(func() {
				mockDB := storagemocks.NewMockTx(ctrl)
				mockDB.EXPECT().Get(gomock.Any()).Return(nil, fmt.Errorf("error"))
				txKeeper.db = mockDB
			})

			It("should return err='failed to get tx: error'", func() {
				tx := txns.NewBareTxCoinTransfer()
				_, err := txKeeper.GetTx(tx.GetHash())
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("failed to get tx: error"))
			})
		})

		When("tx is found", func() {
			tx := txns.NewBareTxCoinTransfer()

			BeforeEach(func() {
				err := txKeeper.Index(tx)
				Expect(err).To(BeNil())
			})

			It("should return tx", func() {
				res, err := txKeeper.GetTx(tx.GetHash())
				Expect(err).To(BeNil())
				Expect(res.Bytes()).To(Equal(tx.Bytes()))
			})
		})

		When("tx is not found", func() {
			It("should return tx", func() {
				_, err := txKeeper.GetTx([]byte("unknown"))
				Expect(err).ToNot(BeNil())
				Expect(err).To(Equal(types.ErrTxNotFound))
			})
		})
	})
})
