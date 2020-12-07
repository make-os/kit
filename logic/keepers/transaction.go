package keepers

import (
	"github.com/make-os/kit/storage"
	"github.com/make-os/kit/storage/common"
	storagetypes "github.com/make-os/kit/storage/types"
	"github.com/make-os/kit/types"
	"github.com/make-os/kit/types/txns"
	"github.com/pkg/errors"
)

// TxKeeper manages transaction data
type TxKeeper struct {
	db storagetypes.Tx
}

// NewTxKeeper creates an instance of TxKeeper
func NewTxKeeper(db storagetypes.Tx) *TxKeeper {
	return &TxKeeper{db: db}
}

// Index takes a transaction and stores it.
// It uses the tx hash as the index key
func (tk *TxKeeper) Index(tx types.BaseTx) error {
	rec := common.NewFromKeyValue(MakeTxKey(tx.GetHash()), tx.Bytes())
	if err := tk.db.Put(rec); err != nil {
		return errors.Wrap(err, "failed to index tx")
	}
	return nil
}

// GetTx gets a transaction by its hash
func (tk *TxKeeper) GetTx(hash []byte) (types.BaseTx, error) {
	rec, err := tk.db.Get(MakeTxKey(hash))
	if err != nil {
		if err != storage.ErrRecordNotFound {
			return nil, errors.Wrap(err, "failed to get tx")
		}
		return nil, types.ErrTxNotFound
	}
	return txns.DecodeTx(rec.Value)
}
