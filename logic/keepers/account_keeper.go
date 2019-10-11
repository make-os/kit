package keepers

import (
	"github.com/makeos/mosdef/storage/tree"
	"github.com/pkg/errors"

	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
)

// AccountKeeper represents manages accounts.
type AccountKeeper struct {
	state *tree.SafeTree
}

// NewAccountKeeper creates an instance of AccountKeeper
func NewAccountKeeper(state *tree.SafeTree) *AccountKeeper {
	return &AccountKeeper{state: state}
}

// GetAccount returns an account by address.
// It returns an empty Account if no account is found.
// If block number is specified and greater than 0,
// the account  state at the given block number is
// returned, otherwise the latest is returned
func (a *AccountKeeper) GetAccount(address util.String, blockNum ...int64) *types.Account {

	// Get version is provided
	var version int64
	if len(blockNum) > 0 && blockNum[0] > 0 {
		version = blockNum[0]
	}

	// Query the account by key. If version is provided,
	// we do a versioned query, otherwise we query the latest.
	key := MakeAccountKey(address.String())
	var bs []byte
	if version != 0 {
		_, bs = a.state.GetVersioned(key, version)
	} else {
		_, bs = a.state.Get(key)
	}

	// If we don't find the account, we return an empty account.
	if bs == nil {
		return types.BareAccount()
	}

	// Otherwise, we decode the account bytes to types.Account
	acct, err := types.NewAccountFromBytes(bs)
	if err != nil {
		panic(errors.Wrap(err, "failed to decode account byte slice"))
	}

	return acct
}

// Update resets an account to a new value
func (a *AccountKeeper) Update(address util.String, upd *types.Account) {
	a.state.Set(MakeAccountKey(address.String()), upd.Bytes())
}
