package keepers

import (
	"github.com/make-os/kit/params"
	"github.com/make-os/kit/storage"
	"github.com/make-os/kit/storage/common"
	storagetypes "github.com/make-os/kit/storage/types"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/epoch"
	"github.com/pkg/errors"
)

// ValidatorKeeper manages information about validators
type ValidatorKeeper struct {
	db storagetypes.Tx
}

// NewValidatorKeeper creates an instance of ValidatorKeeper
func NewValidatorKeeper(db storagetypes.Tx) *ValidatorKeeper {
	return &ValidatorKeeper{db: db}
}

// GetByHeight gets a list of validators that produced a block.
func (v *ValidatorKeeper) getByHeight(height int64) (core.BlockValidators, error) {

	// Get the height of the last block of the previous epoch
	epochHeight := epoch.GetLastHeightInEpochOfHeight(height)

get:
	if epochHeight <= 0 {
		epochHeight = 1
	}

	// Get the validator set attached to the height.
	res := make(map[util.Bytes32]*core.Validator)
	key := MakeBlockValidatorsKey(epochHeight)
	rec, err := v.db.Get(key)
	if err != nil {
		if err != storage.ErrRecordNotFound {
			return nil, err
		}
	}

	// At this point, the height has no validators.
	// In this case, an older epoch validator set must have produced it, therefore we
	// need to find the most recent epoch end block with an associated validator set.
	if err == storage.ErrRecordNotFound {
		nextEveBlock := epochHeight - int64(params.NumBlocksPerEpoch)
		if nextEveBlock >= 0 {
			epochHeight = nextEveBlock
			goto get
		}
		return res, nil
	}

	return res, rec.Scan(&res)
}

// Get gets validators at the given height. If height is <= 0, the
// validator set of the highest height is returned.
func (v *ValidatorKeeper) Get(height int64) (core.BlockValidators, error) {

	if height > 0 {
		return v.getByHeight(height)
	}

	var err error
	res := make(map[util.Bytes32]*core.Validator)
	key := MakeQueryKeyBlockValidators()
	v.db.NewTx(true, true).Iterate(key, false, func(rec *common.Record) bool {
		err = rec.Scan(&res)
		return true
	})

	if err != nil {
		return nil, err
	}

	return res, nil
}

// Index adds a set of validators associated to the given height
func (v *ValidatorKeeper) Index(height int64, validators []*core.Validator) error {

	// Convert the slice of validators to a map structure
	var data = make(map[util.Bytes32]*core.Validator)
	for _, v := range validators {
		data[v.PubKey] = v
		v.PubKey = util.EmptyBytes32
	}

	key := MakeBlockValidatorsKey(height)
	rec := common.NewFromKeyValue(key, util.ToBytes(data))
	if err := v.db.Put(rec); err != nil {
		return errors.Wrap(err, "failed to index validators")
	}

	return nil
}
