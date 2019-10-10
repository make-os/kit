package keepers

import (
	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
	"github.com/pkg/errors"
)

// ValidatorKeeper manages information about validators
type ValidatorKeeper struct {
	db storage.Engine
}

// NewValidatorKeeper creates an instance of ValidatorKeeper
func NewValidatorKeeper(db storage.Engine) *ValidatorKeeper {
	return &ValidatorKeeper{db: db}
}

// GetByHeight gets validators at the given height
func (v *ValidatorKeeper) getByHeight(height int64) (types.BlockValidators, error) {

	res := make(map[string]int64)
	key := MakeBlockValidatorsKey(height)
	rec, err := v.db.Get(key)
	if err != nil {
		if err == storage.ErrRecordNotFound {
			return res, nil
		}
		return nil, err
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
	res := make(map[string]int64)
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
	var data = make(map[string]int64)
	for _, v := range validators {
		data[v.PubKey.String()] = v.Power
	}

	key := MakeBlockValidatorsKey(height)
	rec := storage.NewFromKeyValue(key, util.ObjectToBytes(data))
	if err := v.db.Put(rec); err != nil {
		return errors.Wrap(err, "failed to index validators")
	}

	return nil
}
