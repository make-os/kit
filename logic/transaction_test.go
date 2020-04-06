package logic

import (
	"os"

	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"

	"gitlab.com/makeos/mosdef/crypto"

	"gitlab.com/makeos/mosdef/types"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/storage"
	"gitlab.com/makeos/mosdef/testutil"
)

type unknownTxType struct {
	*core.TxCoinTransfer
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
		err := logic.SysKeeper().SaveBlockInfo(&core.BlockInfo{Height: 1})
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
				tx := core.NewBareTxCoinTransfer()
				tx.Sig = []byte("sig")
				resp := txLogic.ExecTx(tx, 1)
				Expect(resp.Code).To(Equal(types.ErrCodeFailedDecode))
				Expect(resp.Log).To(ContainSubstring("tx failed validation"))
			})
		})

		Context("with unknown transaction type", func() {
			It("should return err", func() {
				tx := &unknownTxType{TxCoinTransfer: core.NewBareTxCoinTransfer()}
				resp := logic.Tx().ExecTx(tx, 1)
				Expect(resp.GetCode()).ToNot(BeZero())
				Expect(resp.GetLog()).To(Equal("tx failed validation: field:type, msg:unsupported transaction type"))
			})
		})

		Context("with unknown ticket purchase tx type", func() {
			It("should return err", func() {
				tx := core.NewBareTxTicketPurchase(1000)
				resp := logic.Tx().ExecTx(tx, 1)
				Expect(resp.GetCode()).ToNot(BeZero())
				Expect(resp.Log).To(Equal("tx failed validation: field:type, msg:type is invalid"))
			})
		})
	})

	Describe("CanExecCoinTransfer", func() {
		var sender = crypto.NewKeyFromIntSeed(1)

		Context("when sender account has insufficient spendable balance", func() {
			It("should not return err='sender's spendable account balance is insufficient'", func() {
				err := txLogic.CanExecCoinTransfer(sender.PubKey(), "100", "0", 1, 1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:value, msg:sender's spendable account balance is insufficient"))
			})

			When("value=0 and fee is non-zero", func() {
				It("should not return err='sender's spendable account balance is insufficient' with field=fee", func() {
					err := txLogic.CanExecCoinTransfer(sender.PubKey(), "0", "10", 1, 1)
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(Equal("field:fee, msg:sender's spendable account balance is insufficient"))
				})
			})
		})

		Context("when nonce is invalid", func() {
			It("should return no error", func() {
				err := txLogic.CanExecCoinTransfer(sender.PubKey(), "100", "0", 3, 1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:value, msg:tx has invalid nonce (3), expected (1)"))
			})
		})

		Context("when sender account has sufficient spendable balance", func() {
			BeforeEach(func() {
				logic.AccountKeeper().Update(sender.Addr(), &state.Account{
					Balance: "1000",
					Stakes:  state.BareAccountStakes(),
				})
			})

			It("should return no error", func() {
				err := txLogic.CanExecCoinTransfer(sender.PubKey(), "100", "0", 1, 0)
				Expect(err).To(BeNil())
			})
		})
	})
})
