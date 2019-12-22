package mempool

import (
	"os"
	"time"

	tmtypes "github.com/tendermint/tendermint/types"

	abci "github.com/tendermint/tendermint/abci/types"

	tmmem "github.com/tendermint/tendermint/mempool"

	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/types"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/testutil"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Mempool", func() {
	var err error
	var cfg *config.AppConfig
	var mempool *Mempool
	var sender = crypto.NewKeyFromIntSeed(1)

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".CheckTxWithInfo", func() {
		Context("when the pool capacity is full", func() {
			BeforeEach(func() {
				cfg.Mempool.Size = 1
				cfg.Mempool.MaxTxsSize = 200
				mempool = NewMempool(cfg)
				tx := types.NewTx(types.TxTypeCoinTransfer, 0, "recipient_addr", sender, "10", "0.1", time.Now().Unix())
				mempool.pool.Put(tx)
			})

			It("should return error when we try to add a tx", func() {
				tx := types.NewTx(types.TxTypeCoinTransfer, 0, "recipient_addr2", sender, "10", "0.1", time.Now().Unix())
				err := mempool.CheckTxWithInfo(tx.Bytes(), nil, tmmem.TxInfo{})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("mempool is full: number of txs 1 (max: 1)"))
			})
		})

		Context("when the pools total txs size is surpassed", func() {
			BeforeEach(func() {
				cfg.Mempool.Size = 2
				cfg.Mempool.MaxTxsSize = 100
				mempool = NewMempool(cfg)
				tx := types.NewTx(types.TxTypeCoinTransfer, 0, "recipient_addr", sender, "10", "0.1", time.Now().Unix())
				mempool.pool.Put(tx)
			})

			It("should return error when we try to add a tx", func() {
				tx := types.NewTx(types.TxTypeCoinTransfer, 0, "recipient_addr2", sender, "10", "0.1", time.Now().Unix())
				err := mempool.CheckTxWithInfo(tx.Bytes(), nil, tmmem.TxInfo{})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("mempool is full: number of txs 1 (max: 2)"))
			})
		})

		Context("when a tx is too large", func() {
			BeforeEach(func() {
				cfg.Mempool.Size = 2
				cfg.Mempool.MaxTxSize = 100
				mempool = NewMempool(cfg)
			})

			It("should return error when we try to add a tx", func() {
				tx := types.NewTx(types.TxTypeCoinTransfer, 0, "recipient_addr2", sender, "10", "0.1", time.Now().Unix())
				err := mempool.CheckTxWithInfo(tx.Bytes(), nil, tmmem.TxInfo{})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("Tx too large. Max size is 100, but got"))
			})
		})
	})

	Describe(".addTx", func() {
		BeforeEach(func() {
			mempool = NewMempool(cfg)
		})

		When("status code is not OK", func() {
			It("should not add tx to pool", func() {
				tx := types.NewTx(types.TxTypeCoinTransfer, 0, "recipient_addr2", sender, "10", "0.1", time.Now().Unix())
				mempool.addTx(tx.Bytes(), &abci.Response{Value: &abci.Response_CheckTx{CheckTx: &abci.ResponseCheckTx{
					Code: 1,
				}}})
				Expect(mempool.Size()).To(BeZero())
			})
		})

		When("status code is OK", func() {
			It("should add tx to pool", func() {
				tx := types.NewTx(types.TxTypeCoinTransfer, 0, "recipient_addr2", sender, "10", "0.1", time.Now().Unix())
				mempool.addTx(tx.Bytes(), &abci.Response{Value: &abci.Response_CheckTx{CheckTx: &abci.ResponseCheckTx{
					Code: abci.CodeTypeOK,
				}}})
				Expect(mempool.Size()).To(Equal(1))
			})
		})
	})

	Describe(".ReapMaxBytesMaxGas", func() {
		When("pool is empty", func() {
			BeforeEach(func() {
				mempool = NewMempool(cfg)
			})

			It("should return empty result", func() {
				res := mempool.ReapMaxBytesMaxGas(0, 0)
				Expect(res).To(BeEmpty())
			})
		})

		When("pool has two transactions with total size = 370 bytes", func() {
			okRes := &abci.Response{Value: &abci.Response_CheckTx{CheckTx: &abci.ResponseCheckTx{
				Code: abci.CodeTypeOK,
			}}}

			BeforeEach(func() {
				mempool = NewMempool(cfg)
				tx := types.NewTx(types.TxTypeCoinTransfer, 0, "recipient_addr1", sender, "10", "0.1", time.Now().Unix())
				tx2 := types.NewTx(types.TxTypeCoinTransfer, 1, "recipient_addr2", sender, "10", "0.1", time.Now().Unix())
				mempool.addTx(tx.Bytes(), okRes)
				mempool.addTx(tx2.Bytes(), okRes)
				Expect(mempool.Size()).To(Equal(2))
				Expect(mempool.TxsBytes()).To(Equal(tx.GetSize() + tx2.GetSize()))
			})

			It("should return 1 tx if max bytes is 185", func() {
				res := mempool.ReapMaxBytesMaxGas(185, 0)
				Expect(len(res)).To(Equal(1))
			})

			It("should return 2 tx if max bytes is 370", func() {
				res := mempool.ReapMaxBytesMaxGas(370, 0)
				Expect(len(res)).To(Equal(2))
			})
		})

		When("pool has three transactions; 1 is a coin transfer and 2 are validator ticket purchase txs", func() {
			var tx, tx2, tx3 *types.Transaction
			var res []tmtypes.Tx
			okRes := &abci.Response{Value: &abci.Response_CheckTx{CheckTx: &abci.ResponseCheckTx{
				Code: abci.CodeTypeOK,
			}}}

			BeforeEach(func() {
				mempool = NewMempool(cfg)
				tx = types.NewTx(types.TxTypeCoinTransfer, 0, "recipient_addr1", sender, "10", "0.1", time.Now().Unix())
				tx2 = types.NewTx(types.TxTypeValidatorTicket, 1, "recipient_addr2", sender, "10", "0.1", time.Now().Unix())
				tx3 = types.NewTx(types.TxTypeValidatorTicket, 2, "recipient_addr3", sender, "10", "0.1", time.Now().Unix())
				mempool.addTx(tx.Bytes(), okRes)
				mempool.addTx(tx2.Bytes(), okRes)
				mempool.addTx(tx3.Bytes(), okRes)
				Expect(mempool.Size()).To(Equal(3))
				res = mempool.ReapMaxBytesMaxGas(1000, 0)
			})

			It("should return 2 txs; 1 tx must remain in the pool and it must be a types.TxTypeValidatorTicket", func() {
				Expect(len(res)).To(Equal(2))
				Expect(mempool.pool.Size()).To(Equal(int64(1)))
				Expect(mempool.pool.HasByHash(tx3.GetHash().HexStr())).To(BeTrue())
				actual := mempool.pool.Head()
				Expect(actual.GetType()).To(Equal(types.TxTypeValidatorTicket))
			})
		})

		Context("epochSecretGetter is set and returns a tx", func() {
			var tx = types.NewTx(types.TxTypeCoinTransfer, 0, "recipient_addr1", sender, "10", "0.1", time.Now().Unix())
			BeforeEach(func() {
				mempool = NewMempool(cfg)
				Expect(mempool.Size()).To(Equal(0))
				mempool.SetEpochSecretGetter(func() (types.Tx, error) {
					return tx, nil
				})
			})

			It("should return 1 tx", func() {
				res := mempool.ReapMaxBytesMaxGas(1000, 0)
				Expect(res).ToNot(BeEmpty())
				Expect(res).To(HaveLen(1))
			})

			When("maxBytes is set to 185 (max size per tx)", func() {
				var tx2 = types.NewTx(types.TxTypeCoinTransfer, 0, "recipient_addr1", sender, "20", "0.1", time.Now().Unix())
				It("should return one tx; the tx return by epochSecretGetter", func() {
					mempool.pool.Put(tx2)
					res := mempool.ReapMaxBytesMaxGas(185, 0)
					Expect(res).ToNot(BeEmpty())
					Expect(res).To(HaveLen(1))
					actual, _ := types.NewTxFromBytes(res[0])
					Expect(tx).To(Equal(actual))
				})
			})
		})
	})
})
