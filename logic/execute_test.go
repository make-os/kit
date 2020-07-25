package logic

import (
	"fmt"
	"os"

	"github.com/themakeos/lobe/types/core"
	"github.com/themakeos/lobe/types/txns"
	"github.com/themakeos/lobe/validation"

	"github.com/themakeos/lobe/types"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/themakeos/lobe/config"
	"github.com/themakeos/lobe/storage"
	"github.com/themakeos/lobe/testutil"
)

type unknownTxType struct {
	*txns.TxCoinTransfer
}

type testSystemContract struct {
	canExec bool
	execErr error
}

func (t testSystemContract) Init(logic core.Logic, tx types.BaseTx, curChainHeight uint64) core.SystemContract {
	return t
}

func (t testSystemContract) CanExec(tx types.TxCode) bool {
	return t.canExec
}

func (t testSystemContract) Exec() error {
	return t.execErr
}

var _ = Describe("Transaction", func() {
	var appDB, stateTreeDB storage.Engine
	var err error
	var cfg *config.AppConfig
	var logic *Logic

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		appDB, stateTreeDB = testutil.GetDB(cfg)
		logic = New(appDB, stateTreeDB, cfg)
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
				Expect(resp.GetLog()).To(Equal("tx failed validation: field:type, msg:unsupported transaction type"))
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
				Expect(resp.Log).To(Equal("tx failed validation: field:type, msg:type is invalid"))
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
