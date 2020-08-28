package logic

import (
	"encoding/json"
	"fmt"

	"github.com/make-os/lobe/config"
	"github.com/make-os/lobe/logic/contracts"
	"github.com/make-os/lobe/logic/contracts/transfercoin"
	"github.com/make-os/lobe/logic/keepers"
	"github.com/make-os/lobe/logic/proposals"
	"github.com/make-os/lobe/pkgs/tree"
	"github.com/make-os/lobe/storage"
	tickettypes "github.com/make-os/lobe/ticket/types"
	"github.com/make-os/lobe/types/core"
	"github.com/make-os/lobe/types/state"
	"github.com/make-os/lobe/types/txns"
	"github.com/make-os/lobe/util"
	"github.com/make-os/lobe/util/identifier"
	"github.com/pkg/errors"
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

	// validator provides functionalities for managing validator information
	validator core.ValidatorLogic

	// ticketMgr provides functionalities for managing tickets
	ticketMgr tickettypes.TicketManager

	// systemKeeper provides functionalities for managing system data
	systemKeeper *keepers.SystemKeeper

	// accountKeeper provides functionalities for managing network accounts
	accountKeeper *keepers.AccountKeeper

	// repoKeeper provides functionalities for managing repository data
	repoKeeper *keepers.RepoKeeper

	// nsKeeper provides functionalities for managing namespace data
	nsKeeper *keepers.NamespaceKeeper

	// validatorKeeper provides operations for managing validator data
	validatorKeeper *keepers.ValidatorKeeper

	// txKeeper provides operations for managing transaction data
	txKeeper *keepers.TxKeeper

	// trackedRepoKeeper provides functionalities for managing tracked repositories
	trackedRepoKeeper *keepers.TrackedRepoKeeper

	// pushKeyKeeper provides functionalities for managing push public keys
	pushKeyKeeper *keepers.PushKeyKeeper

	// repoMgr provides access to the git repository manager
	repoMgr core.RemoteServer

	// mempoolReactor provides access to mempool operations
	mempoolReactor core.MempoolReactor
}

// New creates an instance of Logic
// PANICS: If unable to load state tree
func New(db storage.Engine, stateTreeDB storage.Engine, cfg *config.AppConfig) *Logic {
	dbTx := db.NewTx(true, true)
	l := newLogicWithTx(dbTx, stateTreeDB.NewTx(true, true), cfg)
	l.trackedRepoKeeper = keepers.NewTrackedRepoKeeper(dbTx, l.stateTree)
	l._db = db
	return l
}

// NewAtomic creates an instance of Logic that supports atomic database
// operations across all keepers and logic providers.
func NewAtomic(db storage.Engine, stateTreeDB storage.Engine, cfg *config.AppConfig) *Logic {
	l := newLogicWithTx(db.NewTx(false, false), stateTreeDB.NewTx(true, true), cfg)
	l._db = db

	// Tracked repo keeper uses a managed transaction since it is not used
	// during transaction execution and will not need its state rollback.
	l.trackedRepoKeeper = keepers.NewTrackedRepoKeeper(l._db.NewTx(true, true), l.stateTree)

	return l
}

// newLogicWithTx creates a Logic instance using an externally provided DB transaction.
// All keepers will use the transactions allowing for atomic state operations across them.
func newLogicWithTx(dbTx, stateTreeDBTx storage.Tx, cfg *config.AppConfig) *Logic {

	// Load the state tree
	dbAdapter := storage.NewTMDBAdapter(stateTreeDBTx)
	safeTree := tree.NewSafeTree(dbAdapter, 5000)
	if _, err := safeTree.Load(); err != nil {
		panic(errors.Wrap(err, "failed to load state tree"))
	}

	// Create the logic instances
	l := &Logic{stateTree: safeTree, cfg: cfg, db: dbTx}
	l.validator = &Validator{logic: l}

	// Create the keepers
	l.systemKeeper = keepers.NewSystemKeeper(dbTx)
	l.txKeeper = keepers.NewTxKeeper(dbTx)
	l.accountKeeper = keepers.NewAccountKeeper(safeTree)
	l.validatorKeeper = keepers.NewValidatorKeeper(dbTx)
	l.repoKeeper = keepers.NewRepoKeeper(safeTree, dbTx)
	l.pushKeyKeeper = keepers.NewPushKeyKeeper(safeTree, dbTx)
	l.nsKeeper = keepers.NewNamespaceKeeper(safeTree)

	return l
}

// SetMempoolReactor sets the mempool reactor
func (l *Logic) SetMempoolReactor(mr core.MempoolReactor) {
	l.mempoolReactor = mr
}

// GetMempoolReactor returns the mempool reactor
func (l *Logic) GetMempoolReactor() core.MempoolReactor {
	return l.mempoolReactor
}

// SetRemoteServer sets the repository manager
func (l *Logic) SetRemoteServer(m core.RemoteServer) {
	l.repoMgr = m
}

// GetRemoteServer returns the repository manager
func (l *Logic) GetRemoteServer() core.RemoteServer {
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
func (l *Logic) Config() *config.AppConfig {
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
func (l *Logic) SetTicketManager(tm tickettypes.TicketManager) {
	l.ticketMgr = tm
}

// GetTicketManager returns the ticket manager
func (l *Logic) GetTicketManager() tickettypes.TicketManager {
	return l.ticketMgr
}

// DrySend checks whether the given sender can execute the transaction
func (l *Logic) DrySend(sender interface{}, value, fee util.String, nonce, chainHeight uint64) error {
	tx := &txns.TxCoinTransfer{TxValue: &txns.TxValue{Value: value}, TxCommon: &txns.TxCommon{Fee: fee, Nonce: nonce}}
	ct := transfercoin.NewContract()
	ct.Init(l, tx, chainHeight)
	return ct.DryExec(sender)
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

// TrackedRepoKeeper returns the track list keeper
func (l *Logic) TrackedRepoKeeper() core.TrackedRepoKeeper {
	return l.trackedRepoKeeper
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

// PushKeyKeeper returns the push key keeper
func (l *Logic) PushKeyKeeper() core.PushKeyKeeper {
	return l.pushKeyKeeper
}

// Validator returns the validator logic
func (l *Logic) Validator() core.ValidatorLogic {
	return l.validator
}

// WriteGenesisState creates initial state objects from the genesis file
func (l *Logic) ApplyGenesisState(genState json.RawMessage) error {

	// Get genesis state from config. If not set, then use the state passed in.
	genesisData := l.cfg.GenesisFileEntries
	if len(genesisData) == 0 {
		genesisData = config.RawStateToGenesisData(genState)
	}

	// Register all genesis data entries to the state
	for _, ga := range genesisData {

		// Create account
		if ga.Type == config.GenDataTypeAccount {
			newAcct := state.BareAccount()
			newAcct.Balance = util.String(ga.Balance)
			l.accountKeeper.Update(identifier.Address(ga.Address), newAcct)
		}

		// Create repository
		if ga.Type == config.GenDataTypeRepo {
			newRepo := state.BareRepository()
			for address, owner := range ga.Owners {
				newRepo.AddOwner(address, &state.RepoOwner{
					Creator:  owner.Creator,
					JoinedAt: util.UInt64(owner.JoinedAt),
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

	repoKeeper := l.RepoKeeper()
	nextChainHeight := uint64(block.Height)

	// Get proposals ending at the given height
	endingProps := repoKeeper.GetProposalsEndingAt(nextChainHeight)

	// Attempt to apply and close the proposal
	for _, ep := range endingProps {
		repo := repoKeeper.Get(ep.RepoName)
		if repo.IsNil() {
			return fmt.Errorf("repo not found") // should never happen
		}
		_, err := proposals.MaybeApplyProposal(&proposals.ApplyProposalArgs{
			Keepers:     l,
			Proposal:    repo.Proposals.Get(ep.ProposalID),
			Repo:        repo,
			ChainHeight: nextChainHeight - 1,
			Contracts:   contracts.SystemContracts,
		})
		if err != nil {
			return err
		}
		repoKeeper.Update(ep.RepoName, repo)
	}

	return nil
}
