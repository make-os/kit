package logic

import (
	types2 "gitlab.com/makeos/mosdef/logic/types"
	"gitlab.com/makeos/mosdef/util"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

// Validator implements types.ValidatorLogic.
// Provides functionalities for managing and deriving validators.
type Validator struct {
	logic types2.Logic
}

// Index indexes the validator set for the given height.
func (v *Validator) Index(height int64, valUpdates []abcitypes.ValidatorUpdate) error {
	var validators = []*types2.Validator{}
	for _, validator := range valUpdates {
		validators = append(validators, &types2.Validator{
			PubKey: util.BytesToBytes32(validator.PubKey.Data),
		})
	}
	return v.logic.ValidatorKeeper().Index(height, validators)
}
