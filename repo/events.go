package repo

import (
	"github.com/makeos/mosdef/mempool"
	"github.com/makeos/mosdef/types"
)

func (m *Manager) subscribe() {

	// On EvtMempoolTxRemoved: Remove the transaction from the push pool
	go func() {
		for evt := range m.cfg.G().Bus.On(mempool.EvtMempoolTxRemoved) {
			if err := checkEvtArgs(evt.Args); err != nil {
				return
			}
			tx, ok := evt.Args[1].(types.BaseTx)
			if !ok {
				panic("expected types.BaseTx")
			}
			if tx.Is(types.TxTypePush) {
				m.pushPool.Remove(tx.(*types.TxPush).PushNote)
			}
		}
	}()

	// On EvtMempoolTxRejected: Remove the transaction from the push pool
	go func() {
		for evt := range m.cfg.G().Bus.On(mempool.EvtMempoolTxRejected) {
			if err := checkEvtArgs(evt.Args); err != nil {
				return
			}
			tx, ok := evt.Args[1].(types.BaseTx)
			if !ok {
				panic("expected types.BaseTx")
			}
			if tx.Is(types.TxTypePush) {
				m.pushPool.Remove(tx.(*types.TxPush).PushNote)
			}
		}
	}()

	go func() {
		for evt := range m.cfg.G().Bus.On(mempool.EvtMempoolTxCommitted) {
			_ = evt
		}
	}()
}
