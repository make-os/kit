package repo

import (
	"gitlab.com/makeos/mosdef/mempool"
	"gitlab.com/makeos/mosdef/repo/types/core"
	"gitlab.com/makeos/mosdef/types/msgs"
)

func rmPushNoteFromPushPool(pushPool core.PushPool, evtArgs []interface{}) {
	if err := checkEvtArgs(evtArgs); err != nil {
		return
	}
	tx, ok := evtArgs[1].(msgs.BaseTx)
	if !ok {
		panic("expected types.BaseTx")
	}
	if tx.Is(msgs.TxTypePush) {
		pushPool.Remove(tx.(*msgs.TxPush).PushNote)
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
