package mempool

import (
	"gitlab.com/makeos/mosdef/types/msgs"
	"os"
	"time"

	"github.com/golang/mock/gomock"
	tmtypes "github.com/tendermint/tendermint/types"

	abci "github.com/tendermint/tendermint/abci/types"

	tmmem "github.com/tendermint/tendermint/mempool"

	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/testutil"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Mempool", func() {
	var err error
	var cfg *config.AppConfig
	var mempool *Mempool
	var sender = crypto.NewKeyFromIntSeed(1)
	var ctrl *gomock.Controller

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		mempool = NewMempool(cfg)
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".CheckTxWithInfo", func() {
		Context("when the pool capacity is full", func() {
			BeforeEach(func() {
				cfg.Mempool.Size = 1
				cfg.Mempool.MaxTxsSize = 200
				tx := msgs.NewBaseTx(msgs.TxTypeCoinTransfer, 0, "recipient_addr", sender, "10", "0.1", time.Now().Unix())
				mempool.pool.Put(tx)
			})

			It("should return error when we try to add a tx", func() {
				tx := msgs.NewBaseTx(msgs.TxTypeCoinTransfer, 0, "recipient_addr2", sender, "10", "0.1", time.Now().Unix())
				err := mempool.CheckTxWithInfo(tx.Bytes(), nil, tmmem.TxInfo{})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("mempool is full: number of txs 1 (max: 1)"))
			})
		})

		Context("when the pools total txs size is surpassed", func() {
			BeforeEach(func() {
				cfg.Mempool.Size = 2
				cfg.Mempool.MaxTxsSize = 100
				tx := msgs.NewBaseTx(msgs.TxTypeCoinTransfer, 0, "recipient_addr", sender, "10", "0.1", time.Now().Unix())
				mempool.pool.Put(tx)
			})

			It("should return error when we try to add a tx", func() {
				tx := msgs.NewBaseTx(msgs.TxTypeCoinTransfer, 0, "recipient_addr2", sender, "10", "0.1", time.Now().Unix())
				err := mempool.CheckTxWithInfo(tx.Bytes(), nil, tmmem.TxInfo{})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("mempool is full: number of txs 1 (max: 2)"))
			})
		})

		Context("when a tx is too large", func() {
			BeforeEach(func() {
				cfg.Mempool.Size = 2
				cfg.Mempool.MaxTxSize = 100
			})

			It("should return error when we try to add a tx", func() {
				tx := msgs.NewBaseTx(msgs.TxTypeCoinTransfer, 0, "recipient_addr2", sender, "10", "0.1", time.Now().Unix())
				err := mempool.CheckTxWithInfo(tx.Bytes(), nil, tmmem.TxInfo{})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("Tx too large. Max size is 100, but got"))
			})
		})
	})

	Describe(".addTx", func() {
		When("status code is not OK", func() {
			It("should not add tx to pool", func() {
				tx := msgs.NewBaseTx(msgs.TxTypeCoinTransfer, 0, "recipient_addr2", sender, "10", "0.1", time.Now().Unix())
				mempool.addTx(tx.Bytes(), &abci.Response{Value: &abci.Response_CheckTx{CheckTx: &abci.ResponseCheckTx{
					Code: 1,
				}}})
				Expect(mempool.Size()).To(BeZero())
			})
		})

		When("status code is OK", func() {
			It("should add tx to pool", func() {
				tx := msgs.NewBaseTx(msgs.TxTypeCoinTransfer, 0, "recipient_addr2", sender, "10", "0.1", time.Now().Unix())
				mempool.addTx(tx.Bytes(), &abci.Response{Value: &abci.Response_CheckTx{CheckTx: &abci.ResponseCheckTx{
					Code: abci.CodeTypeOK,
				}}})
				Expect(mempool.Size()).To(Equal(1))
			})
		})
	})

	Describe(".ReapMaxBytesMaxGas", func() {
		When("pool is empty", func() {
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
				tx := msgs.NewBaseTx(msgs.TxTypeCoinTransfer, 0, "recipient_addr1", sender, "10", "0.1", time.Now().Unix())
				tx2 := msgs.NewBaseTx(msgs.TxTypeCoinTransfer, 1, "recipient_addr2", sender, "10", "0.1", time.Now().Unix())
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
			var tx, tx2, tx3 msgs.BaseTx
			var res []tmtypes.Tx
			okRes := &abci.Response{Value: &abci.Response_CheckTx{CheckTx: &abci.ResponseCheckTx{
				Code: abci.CodeTypeOK,
			}}}

			BeforeEach(func() {
				tx = msgs.NewBaseTx(msgs.TxTypeCoinTransfer, 0, "recipient_addr1", sender, "10", "0.1", time.Now().Unix())
				tx2 = msgs.NewBaseTx(msgs.TxTypeValidatorTicket, 1, "recipient_addr2", sender, "10", "0.1", time.Now().Unix())
				tx3 = msgs.NewBaseTx(msgs.TxTypeValidatorTicket, 2, "recipient_addr3", sender, "10", "0.1", time.Now().Unix())
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
				Expect(actual.GetType()).To(Equal(msgs.TxTypeValidatorTicket))
			})
		})

	})
})
