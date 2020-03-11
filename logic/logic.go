package logic

import (
	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/logic/keepers"
	"gitlab.com/makeos/mosdef/pkgs/tree"
	"gitlab.com/makeos/mosdef/storage"
	types2 "gitlab.com/makeos/mosdef/ticket/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/util"
)

// Logic is the central point for defining and accessing
// and modifying different type of state.
type Logic struct {
	// cfg is the application's config
	cfg *config.AppConfig

	// _db is the db handle for instantly committed database operations.
	// Use this to store records that should be be run in a transaction.
	_db storage.Engine

	// db is the db handle for transaction-centric operations.
	// Use this to store records that should run a transaction managed by ABCI app.
	db storage.Tx

	// stateTree is the chain's state tree
	stateTree *tree.SafeTree

	// tx is the transaction logic for handling transactions of all kinds
	tx core.TxLogic

	// sys provides functionalities for handling and accessing system information
	sys core.SysLogic

	// validator provides functionalities for managing validator information
	validator core.ValidatorLogic

	// ticketMgr provides functionalities for managing tickets
	ticketMgr types2.TicketManager

	// systemKeeper provides functionalities for managing system data
	systemKeeper *keepers.SystemKeeper

	// accountKeeper provides functionalities for managing account data
	accountKeeper *keepers.AccountKeeper

	// repoKeeper provides functionalities for managing repository data
	repoKeeper *keepers.RepoKeeper

	// nsKeeper provides functionalities for managing namespace data
	nsKeeper *keepers.NamespaceKeeper

	// validatorKeeper provides operations for managing validator data
	validatorKeeper *keepers.ValidatorKeeper

	// txKeeper provides operations for managing transaction data
	txKeeper *keepers.TxKeeper

	// gpgPubKeyKeeper provides functionalities for managing gpg public keys
	gpgPubKeyKeeper *keepers.GPGPubKeyKeeper

	// repoMgr provides access to the git repository manager
	repoMgr core.RepoManager

	// mempoolReactor provides access to mempool operations
	mempoolReactor core.MempoolReactor
}

// New creates an instance of Logic
// PANICS: If unable to load state tree
func New(db storage.Engine, stateTreeDB storage.Engine, cfg *config.AppConfig) *Logic {
	dbTx := db.NewTx(true, true)
	l := newLogicWithTx(dbTx, stateTreeDB.NewTx(true, true), cfg)
	l._db = db
	return l
}

// NewAtomic creates an instance of Logic that supports atomic operations across
// all sub-logic providers and keepers.
// PANICS: If unable to load state tree
func NewAtomic(db storage.Engine, stateTreeDB storage.Engine, cfg *config.AppConfig) *Logic {
	l := newLogicWithTx(db.NewTx(false, false), stateTreeDB.NewTx(true, true), cfg)
	l._db = db
	return l
}

func newLogicWithTx(dbTx, stateTreeDBTx storage.Tx, cfg *config.AppConfig) *Logic {

	// Load the state tree
	dbAdapter := storage.NewTMDBAdapter(stateTreeDBTx)
	tree := tree.NewSafeTree(dbAdapter, 5000)
	if _, err := tree.Load(); err != nil {
		panic(errors.Wrap(err, "failed to load state tree"))
	}

	// Create the logic instances
	l := &Logic{stateTree: tree, cfg: cfg, db: dbTx}
	l.sys = &System{logic: l}
	l.tx = &Transaction{logic: l}
	l.validator = &Validator{logic: l}

	// Create the keepers
	l.systemKeeper = keepers.NewSystemKeeper(dbTx)
	l.txKeeper = keepers.NewTxKeeper(dbTx)
	l.accountKeeper = keepers.NewAccountKeeper(tree)
	l.validatorKeeper = keepers.NewValidatorKeeper(dbTx)
	l.repoKeeper = keepers.NewRepoKeeper(tree, dbTx)
	l.gpgPubKeyKeeper = keepers.NewGPGPubKeyKeeper(tree, dbTx)
	l.nsKeeper = keepers.NewNamespaceKeeper(tree)

	return l
}

// ManagedSysKeeper returns a SystemKeeper initialized with a managed database
func (l *Logic) ManagedSysKeeper() core.SystemKeeper {
	return keepers.NewSystemKeeper(l._db.NewTx(true, true))
}

// SetMempoolReactor sets the mempool reactor
func (l *Logic) SetMempoolReactor(mr core.MempoolReactor) {
	l.mempoolReactor = mr
}

// GetMempoolReactor returns the mempool reactor
func (l *Logic) GetMempoolReactor() core.MempoolReactor {
	return l.mempoolReactor
}

// SetRepoManager sets the repository manager
func (l *Logic) SetRepoManager(m core.RepoManager) {
	l.repoMgr = m
}

// GetRepoManager returns the repository manager
func (l *Logic) GetRepoManager() core.RepoManager {
	return l.repoMgr
}

// GetDBTx returns the db transaction used by the logic providers and keepers
func (l *Logic) GetDBTx() storage.Tx {
	return l.db
}

// Commit the state tree, database transaction and other
// processes that needs to be finalized after a new tree
// version is saved.
// NOTE: The operations are not all atomic.
func (l *Logic) Commit() error {

	// Save the state tree.
	_, _, err := l.stateTree.SaveVersion()
	if err != nil {
		return errors.Wrap(err, "failed to save tree")
	}

	// Commit the database transaction.
	if err := l.db.Commit(); err != nil {
		return errors.Wrap(err, "failed to commit transaction")
	}

	// Renew the database transaction
	l.db.RenewTx()

	return nil
}

// Cfg returns the application config
func (l *Logic) Cfg() *config.AppConfig {
	return l.cfg
}

// Discard the underlying transaction and renew it.
// Also rollback any uncommitted tree modifications.
func (l *Logic) Discard() {
	l.db.Discard()
	l.stateTree.Rollback()
	l.db.RenewTx()
}

// SetTicketManager sets the ticket manager
func (l *Logic) SetTicketManager(tm types2.TicketManager) {
	l.ticketMgr = tm
}

// GetTicketManager returns the ticket manager
func (l *Logic) GetTicketManager() types2.TicketManager {
	return l.ticketMgr
}

// Tx returns the transaction logic
func (l *Logic) Tx() core.TxLogic {
	return l.tx
}

// Sys returns system logic
func (l *Logic) Sys() core.SysLogic {
	return l.sys
}

// DB returns the hubs db reference
func (l *Logic) DB() storage.Engine {
	return l._db
}

// StateTree returns the state tree
func (l *Logic) StateTree() tree.Tree {
	return l.stateTree
}

// SysKeeper returns the system keeper
func (l *Logic) SysKeeper() core.SystemKeeper {
	return l.systemKeeper
}

// NamespaceKeeper returns the namespace keeper
func (l *Logic) NamespaceKeeper() core.NamespaceKeeper {
	return l.nsKeeper
}

// TxKeeper returns the transaction keeper
func (l *Logic) TxKeeper() core.TxKeeper {
	return l.txKeeper
}

// ValidatorKeeper returns the validator keeper
func (l *Logic) ValidatorKeeper() core.ValidatorKeeper {
	return l.validatorKeeper
}

// AccountKeeper returns the account keeper
func (l *Logic) AccountKeeper() core.AccountKeeper {
	return l.accountKeeper
}

// RepoKeeper returns the repo keeper
func (l *Logic) RepoKeeper() core.RepoKeeper {
	return l.repoKeeper
}

// GPGPubKeyKeeper returns the gpg public key keeper
func (l *Logic) GPGPubKeyKeeper() core.GPGPubKeyKeeper {
	return l.gpgPubKeyKeeper
}

// Validator returns the validator logic
func (l *Logic) Validator() core.ValidatorLogic {
	return l.validator
}

// WriteGenesisState creates initial state objects from the genesis file
func (l *Logic) WriteGenesisState() error {

	genesisData := l.cfg.GenesisFileEntries
	if len(genesisData) == 0 {
		genesisData = config.GenesisData()
	}

	// Register all genesis data entries to the state
	for _, ga := range genesisData {

		// Create account
		if ga.Type == config.GenDataTypeAccount {
			newAcct := state.BareAccount()
			newAcct.Balance = util.String(ga.Balance)
			l.accountKeeper.Update(util.Address(ga.Address), newAcct)
		}

		// Create repository
		if ga.Type == config.GenDataTypeRepo {
			newRepo := state.BareRepository()
			for address, owner := range ga.Owners {
				newRepo.AddOwner(address, &state.RepoOwner{
					Creator:  owner.Creator,
					JoinedAt: owner.JoinedAt,
					Veto:     owner.Veto,
				})
			}
			newRepo.Config = state.MakeDefaultRepoConfig()
			newRepo.Config.MergeMap(ga.Config)
			l.RepoKeeper().Update(ga.Name, newRepo)
			if ga.Helm {
				if err := l.SysKeeper().SetHelmRepo(ga.Name); err != nil {
					return errors.Wrap(err, "failed to set helm repo")
				}
			}
		}
	}

	return nil
}

// OnEndBlock is called within the ABCI EndBlock method;
// Do things that need to happen after each block transactions are processed;
// Note: The ABCI will panic if an error is returned.
func (l *Logic) OnEndBlock(block *core.BlockInfo) error {
	if err := maybeApplyEndedProposals(l, uint64(block.Height)); err != nil {
		return err
	}
	return nil
}
