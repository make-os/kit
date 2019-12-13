package types

import (
	"fmt"
	"strings"

	"github.com/makeos/mosdef/util"
	"github.com/vmihailenco/msgpack"
)

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

// IsEmpty checks whether an account is empty/unset
func (a *Account) IsEmpty() bool {
	return a.Balance.Empty() || a.Balance.Equal("0") &&
		a.Nonce == uint64(0) &&
		len(a.Stakes) == 0 &&
		a.DelegatorCommission == float64(0)
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

// EncodeMsgpack implements msgpack.CustomEncoder
func (a *Account) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.EncodeMulti(a.Balance, a.Nonce, a.Stakes, a.DelegatorCommission)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (a *Account) DecodeMsgpack(dec *msgpack.Decoder) error {
	return dec.DecodeMulti(&a.Balance, &a.Nonce, &a.Stakes, &a.DelegatorCommission)
}

// Bytes return the serialized equivalent of the account
func (a *Account) Bytes() []byte {
	return util.ObjectToBytes(a)
}

// CleanUnbonded removes unbonded stakes.
// Ignores stakes with unbond height set to 0.
// curHeight: The current blockchain height
func (a *Account) CleanUnbonded(curHeight uint64) {
	for name, stake := range a.Stakes {
		if stake.UnbondHeight != 0 && stake.UnbondHeight <= curHeight {
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

// Remove removes a stake entry that matches the given stake type, value and
// unbond height. Returns the key of the stake entry removed
func (s *AccountStakes) Remove(stakeType string, value util.String, unbondHeight uint64) string {
	for k, si := range *s {
		if strings.HasPrefix(k, stakeType) &&
			si.Value.Equal(value) &&
			unbondHeight == si.UnbondHeight {
			delete(*s, k)
			return k
		}
	}
	return ""
}

// UpdateUnbondHeight update the unbond height that matches the given stake
// type, value and unbond height. Returns the key of the stake entry that was updated.
func (s *AccountStakes) UpdateUnbondHeight(
	stakeType string,
	value util.String,
	unbondHeight,
	newUnbondHeight uint64) string {
	for k, si := range *s {
		if strings.HasPrefix(k, stakeType) &&
			si.Value.Equal(value) &&
			unbondHeight == si.UnbondHeight {
			si.UnbondHeight = newUnbondHeight
			return k
		}
	}
	return ""
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
		if si.UnbondHeight == 0 || si.UnbondHeight > curHeight {
			total = total.Add(si.Value.Decimal())
		}
	}
	return util.String(total.String())
}

// NewAccountFromBytes decodes bz to Account
func NewAccountFromBytes(bz []byte) (*Account, error) {
	var a = &Account{}
	return a, util.BytesToObject(bz, a)
}
