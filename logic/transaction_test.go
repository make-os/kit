package logic

import (
	"os"

	"github.com/makeos/mosdef/crypto"

	"github.com/makeos/mosdef/util"

	"github.com/makeos/mosdef/types"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/testutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type unknownTxType struct {
	*types.TxCoinTransfer
}

var _ = Describe("Transaction", func() {
	var appDB, stateTreeDB storage.Engine
	var err error
	var cfg *config.AppConfig
	var logic *Logic
	var txLogic *Transaction

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		appDB, stateTreeDB = testutil.GetDB(cfg)
		logic = New(appDB, stateTreeDB, cfg)
		txLogic = &Transaction{logic: logic}
	})

	BeforeEach(func() {
		err := logic.SysKeeper().SaveBlockInfo(&types.BlockInfo{Height: 1})
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		Expect(appDB.Close()).To(BeNil())
		Expect(stateTreeDB.Close()).To(BeNil())
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".ExecTx", func() {

		Context("when tx is invalid", func() {
			It("should return err='tx failed validation...'", func() {
				tx := types.NewBareTxCoinTransfer()
				tx.Sig = []byte("sig")
				resp := txLogic.ExecTx(tx, 1)
				Expect(resp.Code).To(Equal(types.ErrCodeFailedDecode))
				Expect(resp.Log).To(ContainSubstring("tx failed validation"))
			})
		})

		Context("with unknown transaction type", func() {
			It("should return err", func() {
				tx := &unknownTxType{TxCoinTransfer: types.NewBareTxCoinTransfer()}
				resp := logic.Tx().ExecTx(tx, 1)
				Expect(resp.GetCode()).ToNot(BeZero())
				Expect(resp.GetLog()).To(Equal("tx failed validation: field:type, error:unsupported transaction type"))
			})
		})

		Context("with unknown ticket purchase tx type", func() {
			It("should return err", func() {
				tx := types.NewBareTxTicketPurchase(1000)
				resp := logic.Tx().ExecTx(tx, 1)
				Expect(resp.GetCode()).ToNot(BeZero())
				Expect(resp.Log).To(Equal("tx failed validation: field:type, error:type is invalid"))
			})
		})
	})

	Describe("CanExecCoinTransfer", func() {
		var sender = crypto.NewKeyFromIntSeed(1)

		Context("when sender account has insufficient spendable balance", func() {
			It("should not return err='sender's spendable account balance is insufficient'", func() {
				err := txLogic.CanExecCoinTransfer(sender.PubKey(), util.String("100"), util.String("0"), 1, 1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:value, error:sender's spendable account balance is insufficient"))
			})

			When("value=0 and fee is non-zero", func() {
				It("should not return err='sender's spendable account balance is insufficient' with field=fee", func() {
					err := txLogic.CanExecCoinTransfer(sender.PubKey(), util.String("0"), util.String("10"), 1, 1)
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(Equal("field:fee, error:sender's spendable account balance is insufficient"))
				})
			})
		})

		Context("when nonce is invalid", func() {
			It("should return no error", func() {
				err := txLogic.CanExecCoinTransfer(sender.PubKey(), util.String("100"), util.String("0"), 3, 1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:value, error:tx has invalid nonce (3), expected (1)"))
			})
		})

		Context("when sender account has sufficient spendable balance", func() {
			BeforeEach(func() {
				logic.AccountKeeper().Update(sender.Addr(), &types.Account{
					Balance: util.String("1000"),
					Stakes:  types.BareAccountStakes(),
				})
			})

			It("should return no error", func() {
				err := txLogic.CanExecCoinTransfer(sender.PubKey(), util.String("100"), util.String("0"), 1, 0)
				Expect(err).To(BeNil())
			})
		})
	})
})
