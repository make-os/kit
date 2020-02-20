package repo

import (
	"gitlab.com/makeos/mosdef/mempool"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"
)

func rmPushNoteFromPushPool(pushPool core.PushPool, evtArgs []interface{}) {
	if err := checkEvtArgs(evtArgs); err != nil {
		return
	}
	tx, ok := evtArgs[1].(types.BaseTx)
	if !ok {
		panic("expected types.BaseTx")
	}
	if tx.Is(core.TxTypePush) {
		pushPool.Remove(tx.(*core.TxPush).PushNote)
	}
}

// subscribe subscribes to various incoming events
func (m *Manager) subscribe() {

	// On EvtMempoolTxRemoved: Remove the transaction from the push pool
	go func() {
		for evt := range m.cfg.G().Bus.On(mempool.EvtMempoolTxRemoved) {
			rmPushNoteFromPushPool(m.pushPool, evt.Args)
		}
	}()

	// On EvtMempoolTxRejected: Remove the transaction from the push pool
	go func() {
		for evt := range m.cfg.G().Bus.On(mempool.EvtMempoolTxRejected) {
			rmPushNoteFromPushPool(m.pushPool, evt.Args)
		}
	}()
}
