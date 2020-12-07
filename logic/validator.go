package logic

import (
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/util"
	abcitypes "github.com/tendermint/tendermint/abci/types"
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
