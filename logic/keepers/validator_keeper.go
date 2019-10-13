package keepers

import (
	"github.com/makeos/mosdef/params"
	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
	"github.com/pkg/errors"
)

// ValidatorKeeper manages information about validators
type ValidatorKeeper struct {
	db storage.Functions
}

// NewValidatorKeeper creates an instance of ValidatorKeeper
func NewValidatorKeeper(db storage.Functions) *ValidatorKeeper {
	return &ValidatorKeeper{db: db}
}

// GetByHeight gets a list of validators that produced a block.
func (v *ValidatorKeeper) getByHeight(height int64) (types.BlockValidators, error) {

	// Get the height of the block before the first block of the epoch this target
	// block belongs to - This is known as the epoch eve block
	// e.g [epoch 1: [eveBlock 9]] [epoch 2: [block 10]]
	epochEveBlockHeight := height - (height % int64(params.NumBlocksPerEpoch))

get:
	// The lowest eve block is 1 (genesis block)
	if epochEveBlockHeight <= 0 {
		epochEveBlockHeight = 1
	}

	// Find the validator set attached to the the eve block.
	res := make(map[string]*types.Validator)
	key := MakeBlockValidatorsKey(epochEveBlockHeight)
	rec, err := v.db.Get(key)
	if err != nil {
		if err != storage.ErrRecordNotFound {
			return nil, err
		}
	}

	// At this point, the eve block has no validators.
	// In this case, an older epoch validator must have produced it, therefore we
	// need to find the most recent eve block with an associated validator set.
	if err == storage.ErrRecordNotFound {
		nextEveBlock := epochEveBlockHeight - int64(params.NumBlocksPerEpoch)
		if nextEveBlock >= 0 {
			epochEveBlockHeight = nextEveBlock
			goto get
		}
		return res, nil
	}

	rec.Scan(&res)

	return res, nil
}

// GetByHeight gets validators at the given height. If height is <= 0, the
// validator set of the highest height is returned.
func (v *ValidatorKeeper) GetByHeight(height int64) (types.BlockValidators, error) {

	if height > 0 {
		return v.getByHeight(height)
	}

	var err error
	res := make(map[string]*types.Validator)
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
func (v *ValidatorKeeper) Index(height int64, validators []*types.Validator) error {

	// Convert the slice of validators to a map structure
	var data = make(map[string]*types.Validator)
	for _, v := range validators {
		data[v.PubKey.String()] = v
		v.PubKey = []byte{} // save space since key has this data
	}

	key := MakeBlockValidatorsKey(height)
	rec := storage.NewFromKeyValue(key, util.ObjectToBytes(data))
	if err := v.db.Put(rec); err != nil {
		return errors.Wrap(err, "failed to index validators")
	}

	return nil
}
