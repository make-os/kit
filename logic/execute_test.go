package logic

import (
	"fmt"
	"os"

	storagetypes "github.com/make-os/kit/storage/types"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/validation"
	tmdb "github.com/tendermint/tm-db"

	"github.com/make-os/kit/types"

	"github.com/make-os/kit/config"
	"github.com/make-os/kit/testutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type unknownTxType struct {
	*txns.TxCoinTransfer
}

type testSystemContract struct {
	canExec bool
	execErr error
}

func (t testSystemContract) Init(_ core.Keepers, _ types.BaseTx, _ uint64) core.SystemContract {
	return t
}

func (t testSystemContract) CanExec(_ types.TxCode) bool {
	return t.canExec
}

func (t testSystemContract) Exec() error {
	return t.execErr
}

var _ = Describe("Transaction", func() {
	var appDB storagetypes.Engine
	var stateTreeDB tmdb.DB
	var err error
	var cfg *config.AppConfig
	var logic *Logic

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		appDB, stateTreeDB = testutil.GetDB()
		logic = New(appDB, stateTreeDB, cfg)
	})

	BeforeEach(func() {
		err := logic.SysKeeper().SaveBlockInfo(&state.BlockInfo{Height: 1})
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		Expect(appDB.Close()).To(BeNil())
		Expect(stateTreeDB.Close()).To(BeNil())
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".ExecTx", func() {

		Context("when tx failed validation", func() {
			It("should return err", func() {
				tx := txns.NewBareTxCoinTransfer()
				tx.Sig = []byte("sig")
				resp := logic.ExecTx(&core.ExecArgs{
					Tx:          tx,
					ChainHeight: 1,
					ValidateTx:  validation.ValidateTx,
				})
				Expect(resp.Code).To(Equal(types.ErrCodeFailedDecode))
				Expect(resp.Log).To(ContainSubstring("tx failed validation"))
			})
		})

		Context("when tx has no contract to execute it", func() {
			It("should return error", func() {
				tx := txns.NewBareTxCoinTransfer()
				resp := logic.ExecTx(&core.ExecArgs{
					Tx:             tx,
					ChainHeight:    1,
					SystemContract: []core.SystemContract{&testSystemContract{canExec: false}},
					ValidateTx:     func(tx types.BaseTx, i int, logic core.Logic) error { return nil },
				})
				Expect(resp.Code).To(Equal(types.ErrCodeExecFailure))
				Expect(resp.Log).To(ContainSubstring("failed to execute tx: no executor found"))
			})
		})

		Context("with unknown transaction type", func() {
			It("should return err", func() {
				tx := &unknownTxType{TxCoinTransfer: txns.NewBareTxCoinTransfer()}
				resp := logic.ExecTx(&core.ExecArgs{
					Tx:          tx,
					ChainHeight: 1,
					ValidateTx:  validation.ValidateTx,
				})
				Expect(resp.GetCode()).ToNot(BeZero())
				Expect(resp.GetLog()).To(Equal(`tx failed validation: "field":"type","msg":"unsupported transaction type"`))
			})
		})

		Context("with unknown ticket purchase tx type", func() {
			It("should return err", func() {
				tx := txns.NewBareTxTicketPurchase(1000)
				resp := logic.ExecTx(&core.ExecArgs{
					Tx:          tx,
					ChainHeight: 1,
					ValidateTx:  validation.ValidateTx,
				})
				Expect(resp.GetCode()).ToNot(BeZero())
				Expect(resp.Log).To(Equal(`tx failed validation: "field":"type","msg":"type is invalid"`))
			})
		})

		Context("when tx execution failed", func() {
			It("should return error", func() {
				tx := txns.NewBareTxCoinTransfer()
				resp := logic.ExecTx(&core.ExecArgs{
					Tx:             tx,
					ChainHeight:    1,
					SystemContract: []core.SystemContract{&testSystemContract{canExec: true, execErr: fmt.Errorf("error")}},
					ValidateTx:     func(tx types.BaseTx, i int, logic core.Logic) error { return nil },
				})
				Expect(resp.Code).To(Equal(types.ErrCodeExecFailure))
				Expect(resp.Log).To(ContainSubstring("failed to execute tx: error"))
			})
		})
	})
})
