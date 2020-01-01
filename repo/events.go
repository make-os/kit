package repo

import (
	"github.com/makeos/mosdef/mempool"
	"github.com/makeos/mosdef/types"
)

func removePushNote(pushPool types.PushPool, args []interface{}) {
	if err := checkEvtArgs(args); err != nil {
		return
	}
	tx, ok := args[1].(types.BaseTx)
	if !ok {
		panic("expected types.BaseTx")
	}
	if tx.Is(types.TxTypePush) {
		pushPool.Remove(tx.(*types.TxPush).PushNote)
	}
}

func (m *Manager) subscribe() {

	// On EvtMempoolTxRemoved: Remove the transaction from the push pool
	go func() {
		for evt := range m.cfg.G().Bus.On(mempool.EvtMempoolTxRemoved) {
			removePushNote(m.pushPool, evt.Args)
		}
	}()

	// On EvtMempoolTxRejected: Remove the transaction from the push pool
	go func() {
		for evt := range m.cfg.G().Bus.On(mempool.EvtMempoolTxRejected) {
			removePushNote(m.pushPool, evt.Args)
		}
	}()

	// On EvtMempoolTxCommitted: Update repository permanently
	go func() {
		for evt := range m.cfg.G().Bus.On(mempool.EvtMempoolTxCommitted) {
			if err := checkEvtArgs(evt.Args); err != nil {
				return
			}
			tx, ok := evt.Args[1].(types.BaseTx)
			if !ok {
				panic("expected types.BaseTx")
			}
			if tx.Is(types.TxTypePush) {
				if err := m.onCommittedTxPush(tx.(*types.TxPush)); err != nil {
					m.Log().Error("failed to process committed push transaction", "Err", err)
				}
			}
		}
	}()
}
