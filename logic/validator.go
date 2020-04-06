package logic

import (
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/util"
)

// Validator implements types.ValidatorLogic.
// Provides functionalities for managing and deriving validators.
type Validator struct {
	logic core.Logic
}

// Index indexes the validator set for the given height.
func (v *Validator) Index(height int64, valUpdates []abcitypes.ValidatorUpdate) error {
	var validators []*core.Validator
	for _, validator := range valUpdates {
		validators = append(validators, &core.Validator{
			PubKey: util.BytesToBytes32(validator.PubKey.Data),
		})
	}
	return v.logic.ValidatorKeeper().Index(height, validators)
}
