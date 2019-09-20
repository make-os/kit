package logic

import (
	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/logic/keepers"
	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/storage/tree"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
)

// Logic is the central point for defining and accessing
// and modifying different type of state.
type Logic struct {
	cfg           *config.EngineConfig
	db            storage.Engine
	tx            types.TxLogic
	stateTree     *tree.SafeTree
	systemKeeper  *keepers.SystemKeeper
	accountKeeper *keepers.AccountKeeper
}

// New creates an instance of Logic
func New(db storage.Engine, tree *tree.SafeTree, cfg *config.EngineConfig) *Logic {
	hub := &Logic{db: db, stateTree: tree}
	hub.tx = &Transaction{logic: hub}
	hub.cfg = cfg
	hub.systemKeeper = keepers.NewSystemKeeper(db)
	hub.accountKeeper = keepers.NewAccountKeeper(tree)
	return hub
}

// Tx returns the transaction logic
func (h *Logic) Tx() types.TxLogic {
	return h.tx
}

// DB returns the hubs db reference
func (h *Logic) DB() storage.Engine {
	return h.db
}

// StateTree returns the state tree
func (h *Logic) StateTree() *tree.SafeTree {
	return h.stateTree
}

// SysKeeper returns the system keeper
func (h *Logic) SysKeeper() types.SystemKeeper {
	return h.systemKeeper
}

// AccountKeeper returns the account keeper
func (h *Logic) AccountKeeper() types.AccountKeeper {
	return h.accountKeeper
}

// WriteGenesisState creates initial state objects such as
// genesis accounts and their balances.
func (h *Logic) WriteGenesisState() error {

	// Add all genesis accounts
	for _, ga := range h.cfg.GenesisAccounts {
		h.accountKeeper.Update(util.String(ga.Address), &types.Account{
			Balance: util.String(ga.Balance),
		})
	}

	return nil
}
