package types

import (
	"github.com/c-bata/go-prompt"
	"github.com/makeos/mosdef/util"
	"github.com/robertkrimen/otto"
)

// Various names for staking categories
const (
	StakeNameValidator = "v" // Validators
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
		Stakes:  AccountStakes(map[string]util.String{}),
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
// Formula: balance - total staked
func (a *Account) GetSpendableBalance() util.String {
	return util.String(a.Balance.Decimal().Sub(a.Stakes.TotalStaked().Decimal()).String())
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

// BareAccountStakes returns an empty AccountStakes
func BareAccountStakes() AccountStakes {
	return AccountStakes(map[string]util.String{})
}

// AccountStakes holds staked balances
type AccountStakes map[string]util.String

// Add adds a staked balance
func (s *AccountStakes) Add(name string, value util.String) {
	(*s)[name] = value
}

// Has checks whether a staked balance with the given name exist
func (s *AccountStakes) Has(name string) bool {
	_, ok := (*s)[name]
	return ok
}

// Get the balance of a stake stored with the given name.
// Returns zero if not found
func (s *AccountStakes) Get(name string) util.String {
	if !s.Has(name) {
		return util.String("0")
	}
	return (*s)[name]
}

// TotalStaked returns the sum of all staked balances
func (s *AccountStakes) TotalStaked() util.String {
	total := util.String("0").Decimal()
	for _, v := range *s {
		total = total.Add(v.Decimal())
	}
	return util.String(total.String())
}

// NewAccountFromBytes decodes bz to Account
func NewAccountFromBytes(bz []byte) (*Account, error) {
	var values []interface{}
	if err := util.BytesToObject(bz, &values); err != nil {
		return nil, err
	}

	var stakes = AccountStakes(map[string]util.String{})
	for k, v := range values[2].(map[string]interface{}) {
		stakes.Add(k, util.String(v.(string)))
	}

	return &Account{
		Balance:             util.String(values[0].(string)),
		Nonce:               values[1].(uint64),
		Stakes:              stakes,
		DelegatorCommission: values[3].(float64),
	}, nil
}
