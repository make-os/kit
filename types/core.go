package types

import (
	"fmt"

	"github.com/c-bata/go-prompt"
	"github.com/makeos/mosdef/util"
	"github.com/mitchellh/mapstructure"
	"github.com/robertkrimen/otto"
)

// Various names for staking categories
const (
	StakeTypeValidator = "v" // Validators
)

// JSModuleFunc describes a JS module function
type JSModuleFunc struct {
	Name        string
	Value       interface{}
	Description string
}

// JSModule describes a mechanism for providing functionalities
// accessible in the JS console environment.
type JSModule interface {
	Configure(vm *otto.Otto) []prompt.Suggest
}

// Service provides an interface for exposing functionalities.
// It is meant to be used by packages that offer operations
// that other packages or processes might need
type Service interface {
	SendTx(tx *Transaction) (util.Hash, error)
	GetBlock(height int64) (map[string]interface{}, error)
	GetCurrentHeight() (int64, error)
	GetNonce(address util.String) (uint64, error)
}

// BareAccount returns an empty account
func BareAccount() *Account {
	return &Account{
		Balance: util.String("0"),
		Nonce:   0,
		Stakes:  AccountStakes(map[string]*StakeInfo{}),
	}
}

// Account represents a user's identity and includes
// balance and other information.
type Account struct {
	Balance             util.String   `json:"balance"`
	Nonce               uint64        `json:"nonce"`
	Stakes              AccountStakes `json:"stakes"`
	DelegatorCommission float64       `json:"delegatorCommission"`
}

// GetBalance returns the account balance
func (a *Account) GetBalance() util.String {
	return a.Balance
}

// GetSpendableBalance returns the spendable balance of the account.
// Formula: balance - total staked.
// curHeight: The current blockchain height; Used to determine which stakes are unbonded.
func (a *Account) GetSpendableBalance(curHeight uint64) util.String {
	return util.String(a.Balance.Decimal().
		Sub(a.Stakes.TotalStaked(curHeight).Decimal()).String())
}

// Bytes return the bytes equivalent of the account
func (a *Account) Bytes() []byte {
	return util.ObjectToBytes([]interface{}{
		a.Balance,
		a.Nonce,
		a.Stakes,
		a.DelegatorCommission,
	})
}

// CleanUnbonded removes unbonded stakes.
// curHeight: The current blockchain height. Unbond
func (a *Account) CleanUnbonded(curHeight uint64) {
	for name, stake := range a.Stakes {
		if stake.UnbondHeight <= curHeight {
			delete(a.Stakes, name)
		}
	}
}

// BareAccountStakes returns an empty AccountStakes
func BareAccountStakes() AccountStakes {
	return AccountStakes(map[string]*StakeInfo{})
}

// BareStakeInfo returns an empty StakeInfo
func BareStakeInfo() *StakeInfo {
	return &StakeInfo{Value: util.String("0")}
}

// StakeInfo represents properties about a stake.
type StakeInfo struct {
	// Value is the amount staked
	Value util.String `json:"value"`

	// UnbondHeight is the height at which the stake is unbonded.
	// A value of 0 means the stake is bonded forever
	UnbondHeight uint64 `json:"unbondHeight"`
}

// AccountStakes holds staked balances
type AccountStakes map[string]*StakeInfo

// Add adds a staked balance
// stakeType: The unique stake identifier
// value: The value staked
// unbondHeight: The height where the stake is unbonded
// Returns the full stake name
func (s *AccountStakes) Add(stakeType string, value util.String, unbondHeight uint64) string {
	var key string
	i := len(*s)
	for {
		key = fmt.Sprintf("%s%d", stakeType, i)
		if _, ok := (*s)[key]; ok {
			i++
			continue
		}
		(*s)[key] = &StakeInfo{
			Value:        value,
			UnbondHeight: unbondHeight,
		}
		break
	}
	return key
}

// Has checks whether a staked balance with the given name exist
func (s *AccountStakes) Has(name string) bool {
	_, ok := (*s)[name]
	return ok
}

// Get information about of a stake category.
// Returns zero if not found
// name: The name of the staking category
func (s *AccountStakes) Get(name string) *StakeInfo {
	if !s.Has(name) {
		return BareStakeInfo()
	}
	return (*s)[name]
}

// TotalStaked returns the sum of all staked balances
// curHeight: The current blockchain height; Used to determine which stakes are unbonded.
func (s *AccountStakes) TotalStaked(curHeight uint64) util.String {
	total := util.String("0").Decimal()
	for _, si := range *s {
		if si.UnbondHeight > curHeight {
			total = total.Add(si.Value.Decimal())
		}
	}
	return util.String(total.String())
}

// NewAccountFromBytes decodes bz to Account
func NewAccountFromBytes(bz []byte) (*Account, error) {
	var values []interface{}
	if err := util.BytesToObject(bz, &values); err != nil {
		return nil, err
	}

	var stakes = AccountStakes(map[string]*StakeInfo{})
	for k, v := range values[2].(map[string]interface{}) {
		var si StakeInfo
		mapstructure.Decode(v.(map[string]interface{}), &si)
		stakes[k] = &StakeInfo{
			Value:        si.Value,
			UnbondHeight: si.UnbondHeight,
		}
	}

	return &Account{
		Balance:             util.String(values[0].(string)),
		Nonce:               values[1].(uint64),
		Stakes:              stakes,
		DelegatorCommission: values[3].(float64),
	}, nil
}
