package logic

import (
	"os"

	abcitypes "github.com/tendermint/tendermint/abci/types"

	"github.com/makeos/mosdef/crypto"

	"github.com/makeos/mosdef/util"

	"github.com/makeos/mosdef/types"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/testutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Transaction", func() {
	var appDB, stateTreeDB storage.Engine
	var err error
	var cfg *config.EngineConfig
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

	Describe(".PrepareExec", func() {

		Context("when tx bytes are not decodeable to types.Transaction", func() {
			It("should return err='failed to decode transaction from hex to bytes'", func() {
				req := abcitypes.RequestDeliverTx(abcitypes.RequestDeliverTx{
					Tx: []byte([]byte("invalid_hex")),
				})
				resp := txLogic.PrepareExec(req, 1)
				Expect(resp.Code).To(Equal(types.ErrCodeFailedDecode))
				Expect(resp.Log).To(Equal("failed to decode transaction from bytes"))
			})
		})

		Context("when tx is invalid", func() {
			It("should return err='tx failed validation...'", func() {
				tx := types.NewBareTx(1)
				tx.Sig = []byte("sig")
				req := abcitypes.RequestDeliverTx(abcitypes.RequestDeliverTx{
					Tx: tx.Bytes(),
				})
				resp := txLogic.PrepareExec(req, 1)
				Expect(resp.Code).To(Equal(types.ErrCodeFailedDecode))
				Expect(resp.Log).To(ContainSubstring("tx failed validation"))
			})
		})
	})

	Describe(".Exec", func() {
		Context("with unknown transaction type", func() {
			It("should return err", func() {
				tx := &types.Transaction{Type: 100}
				err := logic.Tx().Exec(tx, 1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("unknown transaction type"))
			})
		})
	})

	Describe("CanExecCoinTransfer", func() {
		var sender = crypto.NewKeyFromIntSeed(1)

		Context("when sender account has insufficient spendable balance", func() {
			It("should not return err='sender's spendable account balance is insufficient'", func() {
				err := txLogic.CanExecCoinTransfer(types.TxTypeValidatorTicket, sender.PubKey(), util.String("100"), util.String("0"), 1, 1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:value, error:sender's spendable account balance is insufficient"))
			})
		})

		Context("when nonce is invalid", func() {
			It("should return no error", func() {
				err := txLogic.CanExecCoinTransfer(types.TxTypeValidatorTicket, sender.PubKey(), util.String("100"), util.String("0"), 3, 1)
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
				err := txLogic.CanExecCoinTransfer(types.TxTypeValidatorTicket, sender.PubKey(), util.String("100"), util.String("0"), 1, 0)
				Expect(err).To(BeNil())
			})
		})
	})
})
