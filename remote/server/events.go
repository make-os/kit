package server

import (
	"fmt"

	types2 "github.com/make-os/kit/mempool/types"
	"github.com/make-os/kit/types"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util"
	"github.com/olebedev/emitter"
)

// handleFailedPushTxEvt responds to a failed push transaction
// event by removing the corresponding push note from the push pool
func handleFailedPushTxEvt(sv *Server, evt emitter.Event) error {
	util.CheckEvtArgs(evt.Args)

	tx, ok := evt.Args[1].(types.BaseTx)
	if !ok {
		return fmt.Errorf("unexpected type (types.BaseTx)")
	}

	if tx.Is(txns.TxTypePush) {
		sv.pushPool.Remove(tx.(*txns.TxPush).Note)
	}

	return nil
}

// subscribe subscribes to various incoming events
func (sv *Server) subscribe() {

	// On EvtMempoolTxRemoved:
	// Remove the transaction from the push pool
	go func() {
		for evt := range sv.cfg.G().Bus.On(types2.EvtMempoolTxRemoved) {
			handleFailedPushTxEvt(sv, evt)
		}
	}()

	// On EvtMempoolTxRejected:
	// Remove the transaction from the push pool
	go func() {
		for evt := range sv.cfg.G().Bus.On(types2.EvtMempoolTxRejected) {
			handleFailedPushTxEvt(sv, evt)
		}
	}()
}
