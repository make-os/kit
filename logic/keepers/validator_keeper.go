package keepers

import (
	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/params"
	"gitlab.com/makeos/mosdef/storage"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/util"
)

// ValidatorKeeper manages information about validators
type ValidatorKeeper struct {
	db storage.Tx
}

// NewValidatorKeeper creates an instance of ValidatorKeeper
func NewValidatorKeeper(db storage.Tx) *ValidatorKeeper {
	return &ValidatorKeeper{db: db}
}

// GetByHeight gets a list of validators that produced a block.
func (v *ValidatorKeeper) getByHeight(height int64) (core.BlockValidators, error) {

	// Get the height of the last block of the previous epoch
	lastEpochEndBlockHeight := height - (height % int64(params.NumBlocksPerEpoch))

get:
	if lastEpochEndBlockHeight <= 0 {
		lastEpochEndBlockHeight = 1
	}

	// Find the validator set attached to the height.
	res := make(map[util.Bytes32]*core.Validator)
	key := MakeBlockValidatorsKey(lastEpochEndBlockHeight)
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
		nextEveBlock := lastEpochEndBlockHeight - int64(params.NumBlocksPerEpoch)
		if nextEveBlock >= 0 {
			lastEpochEndBlockHeight = nextEveBlock
			goto get
		}
		return res, nil
	}

	rec.Scan(&res)

	return res, nil
}

// GetByHeight gets validators at the given height. If height is <= 0, the
// validator set of the highest height is returned.
func (v *ValidatorKeeper) GetByHeight(height int64) (core.BlockValidators, error) {

	if height > 0 {
		return v.getByHeight(height)
	}

	var err error
	res := make(map[util.Bytes32]*core.Validator)
	key := MakeQueryKeyBlockValidators()
	v.db.Iterate(key, false, func(rec *storage.Record) bool {
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
	rec := storage.NewFromKeyValue(key, util.ToBytes(data))
	if err := v.db.Put(rec); err != nil {
		return errors.Wrap(err, "failed to index validators")
	}

	return nil
}
