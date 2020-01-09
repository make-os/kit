package repo

import (
	"github.com/makeos/mosdef/mempool"
	"github.com/makeos/mosdef/types"
)

func rmPushNoteFromPushPool(pushPool types.PushPool, evtArgs []interface{}) {
	if err := checkEvtArgs(evtArgs); err != nil {
		return
	}
	tx, ok := evtArgs[1].(types.BaseTx)
	if !ok {
		panic("expected types.BaseTx")
	}
	if tx.Is(types.TxTypePush) {
		pushPool.Remove(tx.(*types.TxPush).PushNote)
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

	// On EvtABCICommittedTx: Update repo state
	// Commented because the repo object syncher does this job.
	// Should be removed completely if satisfied with the object syncher
	go func() {
		// for evt := range m.cfg.G().Bus.On(types.EvtABCICommittedTx) {
		// 	if err := checkEvtArgs(evt.Args); err != nil {
		// 		return
		// 	}

		// 	txs, ok := evt.Args[1].([]types.BaseTx)
		// 	if !ok {
		// 		panic("expected []types.BaseTx")
		// 	}

		// 	for _, baseTx := range txs {

		// 		tx, ok := baseTx.(*types.TxPush)
		// 		if !ok || !m.syncher.IsSynced() {
		// 			continue
		// 		}

		// 		pp.Println("BY EVENT")
		// 		if err := m.MergeTxPushToRepo(tx); err != nil {
		// 			continue
		// 		}
		// 	}
		// }
	}()
}
