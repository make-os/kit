package mempool

import (
	"os"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/make-os/lobe/types"
	"github.com/make-os/lobe/types/txns"
	tmtypes "github.com/tendermint/tendermint/types"

	abci "github.com/tendermint/tendermint/abci/types"

	tmmem "github.com/tendermint/tendermint/mempool"

	"github.com/make-os/lobe/config"
	"github.com/make-os/lobe/crypto"
	"github.com/make-os/lobe/testutil"

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
				tx := txns.NewCoinTransferTx(0, "recipient_addr", sender, "10", "0.1", time.Now().Unix())
				mempool.pool.Put(tx)
			})

			It("should return error when we try to add a tx", func() {
				tx := txns.NewCoinTransferTx(0, "recipient_addr2", sender, "10", "0.1", time.Now().Unix())
				err := mempool.CheckTxWithInfo(tx.Bytes(), nil, tmmem.TxInfo{})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("mempool is full: number of txs 1 (max: 1)"))
			})
		})

		Context("when the pools total txs size is surpassed", func() {
			BeforeEach(func() {
				cfg.Mempool.Size = 2
				cfg.Mempool.MaxTxsSize = 100
				tx := txns.NewCoinTransferTx(0, "recipient_addr", sender, "10", "0.1", time.Now().Unix())
				mempool.pool.Put(tx)
			})

			It("should return error when we try to add a tx", func() {
				tx := txns.NewCoinTransferTx(0, "recipient_addr2", sender, "10", "0.1", time.Now().Unix())
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
				tx := txns.NewCoinTransferTx(0, "recipient_addr2", sender, "10", "0.1", time.Now().Unix())
				err := mempool.CheckTxWithInfo(tx.Bytes(), nil, tmmem.TxInfo{})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("tx is too large. Max size is 100, but got"))
			})
		})
	})

	Describe(".addTx", func() {
		When("status code is not OK", func() {
			It("should not add tx to pool", func() {
				tx := txns.NewCoinTransferTx(0, "recipient_addr2", sender, "10", "0.1", time.Now().Unix())
				mempool.addTx(tx.Bytes(), &abci.Response{Value: &abci.Response_CheckTx{CheckTx: &abci.ResponseCheckTx{
					Code: 1,
				}}})
				Expect(mempool.Size()).To(BeZero())
			})
		})

		When("status code is OK", func() {
			It("should add tx to pool", func() {
				tx := txns.NewCoinTransferTx(0, "recipient_addr2", sender, "10", "0.1", time.Now().Unix())
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
				tx := txns.NewCoinTransferTx(0, "recipient_addr1", sender, "10", "0.1", time.Now().Unix())
				tx2 := txns.NewCoinTransferTx(1, "recipient_addr2", sender, "10", "0.1", time.Now().Unix())
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
			var tx, tx2, tx3 types.BaseTx
			var res []tmtypes.Tx
			okRes := &abci.Response{Value: &abci.Response_CheckTx{CheckTx: &abci.ResponseCheckTx{
				Code: abci.CodeTypeOK,
			}}}

			BeforeEach(func() {
				tx = txns.NewCoinTransferTx(0, "recipient_addr1", sender, "10", "0.1", time.Now().Unix())
				tx2 = txns.NewTicketPurchaseTx(txns.TxTypeValidatorTicket, 1, sender, "10", "0.1", time.Now().Unix())
				tx3 = txns.NewTicketPurchaseTx(txns.TxTypeValidatorTicket, 2, sender, "10", "0.1", time.Now().Unix())
				mempool.addTx(tx.Bytes(), okRes)
				mempool.addTx(tx2.Bytes(), okRes)
				mempool.addTx(tx3.Bytes(), okRes)
				Expect(mempool.Size()).To(Equal(3))
				res = mempool.ReapMaxBytesMaxGas(1000, 0)
			})

			It("should return 2 txs; 1 tx must remain in the pool and it must be a types.TxTypeValidatorTicket", func() {
				Expect(len(res)).To(Equal(2))
				Expect(mempool.pool.Size()).To(Equal(1))
				Expect(mempool.pool.HasByHash(tx3.GetHash().String())).To(BeTrue())
				actual := mempool.pool.Head()
				Expect(actual.GetType()).To(Equal(txns.TxTypeValidatorTicket))
			})
		})

		When("pool has three transactions; 1 is a coin transfer and 2 proposal transaction with same repo name and proposal ID", func() {
			var tx types.BaseTx
			var tx2, tx3 *txns.TxRepoProposalRegisterPushKey
			var res []tmtypes.Tx
			okRes := &abci.Response{Value: &abci.Response_CheckTx{CheckTx: &abci.ResponseCheckTx{
				Code: abci.CodeTypeOK,
			}}}

			BeforeEach(func() {
				tx = txns.NewCoinTransferTx(0, "recipient_addr1", sender, "10", "0.1", time.Now().Unix())

				tx2 = txns.NewBareRepoProposalRegisterPushKey()
				tx2.RepoName = "repo1"
				tx2.TxProposalCommon.ID = "1"
				tx2.Fee = "1.2"
				tx2.Nonce = 1
				tx2.SenderPubKey = sender.PubKey().ToPublicKey()
				tx2.Timestamp = time.Now().Unix()

				tx3 = txns.NewBareRepoProposalRegisterPushKey()
				tx3.RepoName = "repo1"
				tx3.TxProposalCommon.ID = "1"
				tx3.Fee = "1.5"
				tx3.Nonce = 2
				tx3.SenderPubKey = sender.PubKey().ToPublicKey()
				tx3.Timestamp = time.Now().Unix()

				mempool.addTx(tx.Bytes(), okRes)
				mempool.addTx(tx2.Bytes(), okRes)
				mempool.addTx(tx3.Bytes(), okRes)
				Expect(mempool.Size()).To(Equal(3))

				res = mempool.ReapMaxBytesMaxGas(1000, 0)
			})

			It("should return 2 txs; 1 coin tx and 1 proposal. Must not include multiple proposal tx with matching repo name and nonce", func() {
				Expect(len(res)).To(Equal(2))
				Expect(mempool.pool.Size()).To(Equal(1))
				Expect(mempool.pool.HasByHash(tx3.GetHash().String())).To(BeTrue())
				actual := mempool.pool.Head()
				Expect(actual.GetType()).To(Equal(txns.TxTypeRepoProposalRegisterPushKey))
			})
		})
	})
})
