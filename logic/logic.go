package logic

import (
	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/crypto/rand"
	"github.com/makeos/mosdef/logic/keepers"
	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/storage/tree"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
	"github.com/pkg/errors"
)

// Logic is the central point for defining and accessing
// and modifying different type of state.
type Logic struct {
	cfg             *config.EngineConfig
	db              storage.Engine
	tx              types.TxLogic
	sys             types.SysLogic
	stateTree       *tree.SafeTree
	validator       types.ValidatorLogic
	ticketMgr       types.TicketManager
	systemKeeper    *keepers.SystemKeeper
	accountKeeper   *keepers.AccountKeeper
	validatorKeeper *keepers.ValidatorKeeper
	txKeeper        *keepers.TxKeeper
	drand           rand.DRander
}

// New creates an instance of Logic
// PANICS: when drand initialization fails
func New(db storage.Engine, tree *tree.SafeTree, cfg *config.EngineConfig) *Logic {
	l := &Logic{db: db, stateTree: tree, cfg: cfg}
	l.sys = &System{logic: l}
	l.tx = &Transaction{logic: l}
	l.validator = &Validator{logic: l}
	l.systemKeeper = keepers.NewSystemKeeper(db)
	l.txKeeper = keepers.NewTxKeeper(db)
	l.accountKeeper = keepers.NewAccountKeeper(tree)
	l.validatorKeeper = keepers.NewValidatorKeeper(db)

	// Create a drand instance
	l.drand = rand.NewDRand()
	if err := l.drand.Init(); err != nil {
		panic(errors.Wrap(err, "failed to initialize drand"))
	}

	return l
}

// GetDRand returns a drand client
func (h *Logic) GetDRand() rand.DRander {
	return h.drand
}

// SetTicketManager sets the ticket manager
func (h *Logic) SetTicketManager(tm types.TicketManager) {
	h.ticketMgr = tm
}

// GetTicketManager returns the ticket manager
func (h *Logic) GetTicketManager() types.TicketManager {
	return h.ticketMgr
}

// Tx returns the transaction logic
func (h *Logic) Tx() types.TxLogic {
	return h.tx
}

// Sys returns system logic
func (h *Logic) Sys() types.SysLogic {
	return h.sys
}

// DB returns the hubs db reference
func (h *Logic) DB() storage.Engine {
	return h.db
}

// StateTree returns the state tree
func (h *Logic) StateTree() types.Tree {
	return h.stateTree
}

// SysKeeper returns the system keeper
func (h *Logic) SysKeeper() types.SystemKeeper {
	return h.systemKeeper
}

// TxKeeper returns the transaction keeper
func (h *Logic) TxKeeper() types.TxKeeper {
	return h.txKeeper
}

// ValidatorKeeper returns the validator keeper
func (h *Logic) ValidatorKeeper() types.ValidatorKeeper {
	return h.validatorKeeper
}

// AccountKeeper returns the account keeper
func (h *Logic) AccountKeeper() types.AccountKeeper {
	return h.accountKeeper
}

// Validator returns the validator logic
func (h *Logic) Validator() types.ValidatorLogic {
	return h.validator
}

// WriteGenesisState creates initial state objects such as
// genesis accounts and their balances.
func (h *Logic) WriteGenesisState() error {

	// Add all genesis accounts
	for _, ga := range h.cfg.GenesisAccounts {
		newAcct := types.BareAccount()
		newAcct.Balance = util.String(ga.Balance)
		h.accountKeeper.Update(util.String(ga.Address), newAcct)
	}

	return nil
}
