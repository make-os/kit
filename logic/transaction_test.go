package logic

import (
	"encoding/hex"
	"os"

	abcitypes "github.com/tendermint/tendermint/abci/types"

	"github.com/makeos/mosdef/crypto"

	"github.com/makeos/mosdef/util"

	"github.com/makeos/mosdef/types"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/storage/tree"
	"github.com/makeos/mosdef/testutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Transaction", func() {
	var c storage.Engine
	var err error
	var cfg *config.EngineConfig
	var state *tree.SafeTree
	var logic *Logic
	var txLogic *Transaction

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		c = storage.NewBadger(cfg)
		Expect(c.Init()).To(BeNil())
		db := storage.NewTMDBAdapter(c.F(true, true))
		state = tree.NewSafeTree(db, 128)
		logic = New(c, state, cfg)
		txLogic = &Transaction{logic: logic}
	})

	AfterEach(func() {
		Expect(c.Close()).To(BeNil())
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".PrepareExec", func() {
		Context("when tx is not hex decodeable", func() {
			It("should return err='failed to decode transaction from hex to bytes'", func() {
				req := abcitypes.RequestDeliverTx(abcitypes.RequestDeliverTx{
					Tx: []byte("invalid_hex"),
				})
				resp := txLogic.PrepareExec(req)
				Expect(resp.Code).To(Equal(ErrCodeFailedDecode))
				Expect(resp.Log).To(Equal("failed to decode transaction from hex to bytes"))
			})
		})

		Context("when tx bytes are not decodeable to types.Transaction", func() {
			It("should return err='failed to decode transaction from hex to bytes'", func() {
				req := abcitypes.RequestDeliverTx(abcitypes.RequestDeliverTx{
					Tx: []byte(hex.EncodeToString([]byte("invalid_hex"))),
				})
				resp := txLogic.PrepareExec(req)
				Expect(resp.Code).To(Equal(ErrCodeFailedDecode))
				Expect(resp.Log).To(Equal("failed to decode transaction from bytes"))
			})
		})

		Context("when tx is invalid", func() {
			It("should return err='tx failed validation...'", func() {
				tx := &types.Transaction{Sig: []byte("sig")}
				txHex := hex.EncodeToString(tx.Bytes())
				req := abcitypes.RequestDeliverTx(abcitypes.RequestDeliverTx{
					Tx: []byte(txHex),
				})
				resp := txLogic.PrepareExec(req)
				Expect(resp.Code).To(Equal(ErrCodeFailedDecode))
				Expect(resp.Log).To(ContainSubstring("tx failed validation"))
			})
		})
	})

	Describe(".Exec", func() {
		Context("with unknown transaction type", func() {
			It("should return err", func() {
				tx := &types.Transaction{Type: 100}
				err := logic.Tx().Exec(tx)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("unknown transaction type"))
			})
		})
	})

	Describe(".transferTo", func() {
		var senderKey = crypto.NewKeyFromIntSeed(1)
		var recipientKey = crypto.NewKeyFromIntSeed(2)

		Context("when sender public key is not valid", func() {
			It("should return err='invalid sender public key...'", func() {
				err := txLogic.transferTo(util.String("invalid"), util.String(""), util.String("100"), util.String("0"))
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("invalid sender public key"))
			})
		})

		Context("when recipient public key is not valid", func() {
			It("should return err='invalid recipient address...'", func() {
				senderPubKey := util.String(senderKey.PubKey().Base58())
				err := txLogic.transferTo(senderPubKey, util.String("invalid"), util.String("100"), util.String("0"))
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("invalid recipient address"))
			})
		})

		Context("when sender account has insufficient balance", func() {
			BeforeEach(func() {
				logic.AccountKeeper().Update(senderKey.Addr(), &types.Account{
					Balance: util.String("10"),
				})
			})

			It("should return err='sender's account balance is insufficient'", func() {
				senderPubKey := util.String(senderKey.PubKey().Base58())
				err := txLogic.transferTo(senderPubKey, recipientKey.Addr(), util.String("11"), util.String("1"))
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("sender's account balance is insufficient"))
			})
		})

		Context("when sender has bal=100, recipient has bal=10", func() {
			BeforeEach(func() {
				logic.AccountKeeper().Update(senderKey.Addr(), &types.Account{
					Balance: util.String("100"),
				})
				logic.AccountKeeper().Update(recipientKey.Addr(), &types.Account{
					Balance: util.String("10"),
				})
			})

			Context("sender creates a tx with value=10, fee=1", func() {
				BeforeEach(func() {
					senderPubKey := util.String(senderKey.PubKey().Base58())
					err := txLogic.transferTo(senderPubKey, recipientKey.Addr(), util.String("10"), util.String("1"))
					Expect(err).To(BeNil())
				})

				Specify("that sender balance is equal to 89 and nonce=1", func() {
					senderAcct := logic.AccountKeeper().GetAccount(senderKey.Addr())
					Expect(senderAcct.Balance).To(Equal(util.String("89")))
					Expect(senderAcct.Nonce).To(Equal(int64(1)))
				})

				Specify("that recipient balance is equal to 20 and nonce=0", func() {
					recipientAcct := logic.AccountKeeper().GetAccount(recipientKey.Addr())
					Expect(recipientAcct.Balance).To(Equal(util.String("20")))
					Expect(recipientAcct.Nonce).To(Equal(int64(0)))
				})
			})
		})
	})
})
