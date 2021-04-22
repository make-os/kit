package mempool

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	types2 "github.com/make-os/kit/mempool/types"
	"github.com/make-os/kit/mocks"
	"github.com/make-os/kit/types"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/types/txns"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	tmtypes "github.com/tendermint/tendermint/types"

	"github.com/make-os/kit/config"
	"github.com/make-os/kit/crypto/ed25519"
	"github.com/make-os/kit/testutil"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestMempool(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Mempool Suite")
}

var _ = Describe("Mempool", func() {
	var err error
	var cfg *config.AppConfig
	var mempool *Mempool
	var sender = ed25519.NewKeyFromIntSeed(1)
	var ctrl *gomock.Controller
	var mockKeeper *mocks.MockLogic
	var mockAcctKeeper *mocks.MockAccountKeeper

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())

		ctrl = gomock.NewController(GinkgoT())
		mockKeeper = mocks.NewMockLogic(ctrl)
		mockAcctKeeper = mocks.NewMockAccountKeeper(ctrl)
		mockAcctKeeper.EXPECT().Get(gomock.Any()).Return(state.NewBareAccount()).AnyTimes()
		mockKeeper.EXPECT().AccountKeeper().Return(mockAcctKeeper).AnyTimes()

		mempool = NewMempool(cfg, mockKeeper)
		mempool.validateTx = func(_ types.BaseTx, _ int, _ core.Logic) error { return nil }
	})

	AfterEach(func() {
		ctrl.Finish()
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".Add", func() {
		It("should return error when tx will cause mempool to exceed capacity", func() {
			cfg.Mempool.Size = -10
			tx := txns.NewCoinTransferTx(1, "recipient_addr1", sender, "10", "0.1", time.Now().Unix())
			_, err := mempool.Add(tx)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("mempool is full"))
		})

		It("should return error when tx failed validation", func() {
			tx := txns.NewCoinTransferTx(1, "recipient_addr1", sender, "10", "0.1", time.Now().Unix())
			mempool.validateTx = func(_ types.BaseTx, _ int, _ core.Logic) error { return fmt.Errorf("error") }
			_, err := mempool.Add(tx)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("error"))
		})

		It("should successfully add tx to pool and added TxMetaKeyAllowNonceGap meta key to tx meta", func() {
			tx := txns.NewCoinTransferTx(1, "recipient_addr1", sender, "10", "0.1", time.Now().Unix())
			_, err := mempool.Add(tx)
			Expect(err).To(BeNil())
			Expect(tx.GetMeta()).To(HaveKey(types.TxMetaKeyAllowNonceGap))
		})

		It("should return error when tx already exist in pool", func() {
			tx := txns.NewCoinTransferTx(1, "recipient_addr1", sender, "10", "0.1", time.Now().Unix())
			_, err := mempool.Add(tx)
			Expect(err).To(BeNil())
			_, err = mempool.Add(tx)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("exact transaction already in the pool"))
		})

		It("should emit EvtMempoolTxRejected when tx already exist in pool", func() {
			tx := txns.NewCoinTransferTx(1, "recipient_addr1", sender, "10", "0.1", time.Now().Unix())
			_, err := mempool.Add(tx)
			Expect(err).To(BeNil())

			go mempool.Add(tx)

			evt := <-cfg.G().Bus.On(types2.EvtMempoolTxRejected)
			Expect(evt.Args).To(HaveLen(2))
			Expect(evt.Args[0]).ToNot(BeNil())
			Expect(evt.Args[1].(types.BaseTx).GetID()).To(Equal(tx.GetID()))
		})

		It("should emit EvtMempoolTxRejected when tx failed validation check", func() {
			mempool.validateTx = func(_ types.BaseTx, _ int, _ core.Logic) error { return fmt.Errorf("error") }
			tx := txns.NewCoinTransferTx(1, "recipient_addr1", sender, "10", "0.1", time.Now().Unix())
			go mempool.Add(tx)

			evt := <-cfg.G().Bus.On(types2.EvtMempoolTxRejected)
			Expect(evt.Args).To(HaveLen(2))
			Expect(evt.Args[0]).ToNot(BeNil())
			Expect(evt.Args[1].(types.BaseTx).GetID()).To(Equal(tx.GetID()))
		})
	})

	Describe(".checkCapacity", func() {
		It("should return error if mempool size has exceeded max. capacity", func() {
			cfg.Mempool.Size = -10
			tx := txns.NewCoinTransferTx(1, "recipient_addr1", sender, "10", "0.1", time.Now().Unix())
			err := mempool.checkCapacity(tx)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("mempool is full"))
		})

		It("should return error if tx size has exceeded max. tx size", func() {
			cfg.Mempool.MaxTxSize = 1
			tx := txns.NewCoinTransferTx(1, "recipient_addr1", sender, "10", "0.1", time.Now().Unix())
			err := mempool.checkCapacity(tx)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("transaction is too large"))
		})

		It("should return nil on success", func() {
			cfg.Mempool.MaxTxSize = 100000
			tx := txns.NewCoinTransferTx(1, "recipient_addr1", sender, "10", "0.1", time.Now().Unix())
			err := mempool.checkCapacity(tx)
			Expect(err).To(BeNil())
		})
	})

	Describe(".recheckTxs", func() {
		It("should remove invalid transactions from the pool", func() {
			tx := txns.NewCoinTransferTx(1, "recipient_addr1", sender, "10", "0.1", time.Now().Unix())
			_, err := mempool.Add(tx)
			Expect(err).To(BeNil())
			Expect(mempool.Size()).To(Equal(1))

			mempool.validateTx = func(_ types.BaseTx, _ int, _ core.Logic) error { return fmt.Errorf("error") }
			mempool.recheckTxs()
			Expect(mempool.Size()).To(Equal(0))
		})
	})

	Describe(".notifyTxsAvailable", func() {
		It("should panic if mempool is empty", func() {
			Expect(mempool.notifiedTxsAvailable).To(BeFalse())
			Expect(func() {
				mempool.notifyTxsAvailable()
			}).To(Panic())
		})

		It("should set notifiedTxsAvailable to true", func() {
			Expect(mempool.notifiedTxsAvailable).To(BeFalse())
			tx := txns.NewCoinTransferTx(1, "recipient_addr1", sender, "10", "0.1", time.Now().Unix())
			_, err := mempool.Add(tx)
			Expect(err).To(BeNil())
			mempool.EnableTxsAvailable()
			mempool.notifyTxsAvailable()
			Expect(mempool.notifiedTxsAvailable).To(BeTrue())
		})
	})

	Describe(".Update", func() {
		It("should return error when unable to decode transaction", func() {
			txs := tmtypes.Txs{[]byte("bad tx")}
			err := mempool.Update(1, txs, nil, nil, nil)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("unsupported tx type"))
		})

		It("should return remove tx from pool", func() {
			tx := txns.NewCoinTransferTx(1, "recipient_addr1", sender, "10", "0.1", time.Now().Unix())
			_, err := mempool.Add(tx)
			Expect(err).To(BeNil())

			txs := tmtypes.Txs{tx.Bytes()}
			err = mempool.Update(1, txs, nil, nil, nil)
			Expect(err).To(BeNil())
			Expect(mempool.Size()).To(BeZero())
		})

		It("should emit event on success response per tx", func() {
			tx := txns.NewCoinTransferTx(1, "recipient_addr1", sender, "10", "0.1", time.Now().Unix())
			_, err := mempool.Add(tx)
			Expect(err).To(BeNil())

			txs := tmtypes.Txs{tx.Bytes()}
			go mempool.Update(1, txs, []*abcitypes.ResponseDeliverTx{{Code: abcitypes.CodeTypeOK}}, nil, nil)
			evt := <-cfg.G().Bus.On(types2.EvtMempoolTxCommitted)
			Expect(evt.Args).To(HaveLen(2))
			Expect(evt.Args[0]).To(BeNil())
			Expect(evt.Args[1].(types.BaseTx).GetID()).To(Equal(tx.GetID()))
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
			BeforeEach(func() {

				tx := txns.NewCoinTransferTx(1, "recipient_addr1", sender, "10", "0.1", time.Now().Unix())
				tx2 := txns.NewCoinTransferTx(2, "recipient_addr2", sender, "10", "0.1", time.Now().Unix())
				_, err := mempool.Add(tx)
				Expect(err).To(BeNil())
				_, err = mempool.Add(tx2)
				Expect(err).To(BeNil())

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
			BeforeEach(func() {

				tx = txns.NewCoinTransferTx(1, "recipient_addr1", sender, "10", "0.1", time.Now().Unix())
				tx2 = txns.NewTicketPurchaseTx(txns.TxTypeValidatorTicket, 2, sender, "10", "0.1", time.Now().Unix())
				tx3 = txns.NewTicketPurchaseTx(txns.TxTypeValidatorTicket, 3, sender, "10", "0.1", time.Now().Unix())
				_, err := mempool.Add(tx)
				Expect(err).To(BeNil())
				_, err = mempool.Add(tx2)
				Expect(err).To(BeNil())
				_, err = mempool.Add(tx3)
				Expect(err).To(BeNil())

				Expect(mempool.Size()).To(Equal(3))
				res = mempool.ReapMaxBytesMaxGas(1000, 0)
			})

			It("should return 2 txs; 1 TxTypeValidatorTicket tx must remain in the cache", func() {
				Expect(len(res)).To(Equal(2))
				Expect(mempool.pool.Size()).To(Equal(0))
				Expect(mempool.pool.CacheSize()).To(Equal(1))
				Expect(mempool.pool.GetFromCache().GetHash()).To(Equal(tx3.GetHash()))
			})
		})

		When("pool has three transactions; 1 is a coin transfer and 2 proposal transaction with same repo name and proposal ID", func() {
			var tx types.BaseTx
			var tx2, tx3 *txns.TxRepoProposalRegisterPushKey
			var res []tmtypes.Tx

			BeforeEach(func() {

				tx = txns.NewCoinTransferTx(1, "recipient_addr1", sender, "10", "0.1", time.Now().Unix())
				_, err := mempool.Add(tx)
				Expect(err).To(BeNil())

				tx2 = txns.NewBareRepoProposalRegisterPushKey()
				tx2.RepoName = "repo1"
				tx2.TxProposalCommon.ID = "1"
				tx2.Fee = "1.2"
				tx2.Nonce = 2
				tx2.SenderPubKey = sender.PubKey().ToPublicKey()
				tx2.Timestamp = time.Now().Unix()
				_, err = mempool.Add(tx2)
				Expect(err).To(BeNil())

				tx3 = txns.NewBareRepoProposalRegisterPushKey()
				tx3.RepoName = "repo1"
				tx3.TxProposalCommon.ID = "1"
				tx3.Fee = "1.5"
				tx3.Nonce = 3
				tx3.SenderPubKey = sender.PubKey().ToPublicKey()
				tx3.Timestamp = time.Now().Unix()
				_, err = mempool.Add(tx3)
				Expect(err).To(BeNil())

				Expect(mempool.Size()).To(Equal(3))
				res = mempool.ReapMaxBytesMaxGas(1000, 0)
			})

			It("should return 2 txs; 1 coin tx and 1 proposal. Other proposal must be added to the cache", func() {
				Expect(len(res)).To(Equal(2))
				Expect(mempool.pool.Size()).To(Equal(0))
				Expect(mempool.pool.CacheSize()).To(Equal(1))
				Expect(mempool.pool.GetFromCache().GetHash()).To(Equal(tx3.GetHash()))
			})
		})
	})
})
