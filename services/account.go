package services

import (
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
)

// GetNonce returns the current nonce of an account.
// Returned types.ErrAccountUnknown if account is unknown.
func (s *Service) GetNonce(address util.String) (uint64, error) {
	acct := s.logic.AccountKeeper().GetAccount(address, 0)
	if acct.Nonce == 0 && acct.Balance == "0" {
		return 0, types.ErrAccountUnknown
	}
	return acct.Nonce, nil
}
