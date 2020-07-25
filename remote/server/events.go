package server

import (
	"fmt"

	"github.com/olebedev/emitter"
	"gitlab.com/makeos/lobe/mempool"
	"gitlab.com/makeos/lobe/types"
	"gitlab.com/makeos/lobe/types/txns"
	"gitlab.com/makeos/lobe/util"
)

// subscribe subscribes to various incoming events
func (sv *Server) subscribe() {

	// Removes a push note corresponding to a finalized push transaction from the push pool
	var rmTxFromPushPool = func(evt emitter.Event) error {
		if err := util.CheckEvtArgs(evt.Args); err != nil {
			return err
		}
		tx, ok := evt.Args[1].(types.BaseTx)
		if !ok {
			return fmt.Errorf("unexpected type (types.BaseTx)")
		}
		if tx.Is(txns.TxTypePush) {
			sv.pushPool.Remove(tx.(*txns.TxPush).Note)
		}
		return nil
	}

	// On EvtMempoolTxRemoved:
	// Remove the transaction from the push pool
	go func() {
		for evt := range sv.cfg.G().Bus.On(mempool.EvtMempoolTxRemoved) {
			rmTxFromPushPool(evt)
		}
	}()

	// On EvtMempoolTxRejected:
	// Remove the transaction from the push pool
	go func() {
		for evt := range sv.cfg.G().Bus.On(mempool.EvtMempoolTxRejected) {
			rmTxFromPushPool(evt)
		}
	}()
}
