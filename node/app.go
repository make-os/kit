package node

import (
	"bytes"
	"fmt"
	types2 "gitlab.com/makeos/mosdef/logic/types"
	types3 "gitlab.com/makeos/mosdef/ticket/types"
	"gitlab.com/makeos/mosdef/types/msgs"

	"github.com/tendermint/tendermint/state"

	"github.com/fatih/color"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/logic/keepers"
	"gitlab.com/makeos/mosdef/params"
	"gitlab.com/makeos/mosdef/storage"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/util"
	"gitlab.com/makeos/mosdef/util/logger"
	"gitlab.com/makeos/mosdef/validators"
	"github.com/pkg/errors"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

type ticketInfo struct {
	Tx    msgs.BaseTx
	index int
}

type mergeProposalInfo struct {
	repo       string
	proposalID string
}

// App implements tendermint ABCI interface to
type App struct {
	db                        storage.Engine
	logic                     types2.AtomicLogic
	cfg                       *config.AppConfig
	validateTx                validators.ValidateTxFunc
	curWorkingBlock           *types2.BlockInfo
	log                       logger.Logger
	txIndex                   int
	unIdxValidatorTickets     []*ticketInfo
	unIdxStorerTickets        []*ticketInfo
	unbondStorerReqs          []util.Bytes32
	ticketMgr                 types3.TicketManager
	isCurrentBlockProposer    bool
	unsavedValidators         []*types2.Validator
	heightToSaveNewValidators int64
	unIdxTxs                  []msgs.BaseTx
	unIdxRepoPropVotes        []*msgs.TxRepoProposalVote
	newRepos                  []string
	unIdxClosedMergeProposal  []*mergeProposalInfo
}

// NewApp creates an instance of App
func NewApp(
	cfg *config.AppConfig,
	db storage.Engine,
	logic types2.AtomicLogic,
	ticketMgr types3.TicketManager) *App {
	return &App{
		db:              db,
		logic:           logic,
		cfg:             cfg,
		curWorkingBlock: &types2.BlockInfo{},
		log:             cfg.G().Log.Module("app"),
		ticketMgr:       ticketMgr,
		validateTx:      validators.ValidateTx,
	}
}

// InitChain is called once upon genesis.
func (a *App) InitChain(req abcitypes.RequestInitChain) abcitypes.ResponseInitChain {

	stateTree := a.logic.StateTree()

	a.log.Info("Initializing for the first time")
	a.log.Info("Creating the chain and generaring initial state")

	// Write genesis state as long as the state tree is empty
	if stateTree.WorkingHash() == nil {
		if err := a.logic.WriteGenesisState(); err != nil {
			panic(errors.Wrap(err, "failed to write genesis state"))
		}
	} else {
		panic(fmt.Errorf("At init, state must be empty...It is not empty"))
	}

	// Store genesis validators
	if err := a.logic.Validator().Index(1, req.GetValidators()); err != nil {
		panic(errors.Wrap(err, "failed to index validators"))
	}

	a.log.Info("Initial app state has been loaded",
		"GenesisHash", util.BytesToBytes32(stateTree.WorkingHash()).HexStr(),
		"StateVersion", stateTree.Version())

	return abcitypes.ResponseInitChain{}
}

// Info returns information about the application state.
// Used to sync tendermint with the application during a handshake that happens on startup.
// The returned AppVersion will be included in the header of every block.
// Tendermint expects LastBlockAppHash and LastBlockHeight to be updated during commit.
func (a *App) Info(req abcitypes.RequestInfo) abcitypes.ResponseInfo {

	var lastBlockAppHash = []byte{}
	var lastBlockHeight = int64(0)

	// Get the last committed block information
	lastBlock, err := a.logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		if err != keepers.ErrBlockInfoNotFound {
			panic(err)
		}
	}

	if lastBlock != nil {
		lastBlockAppHash = lastBlock.AppHash
		lastBlockHeight = lastBlock.Height
	}

	return abcitypes.ResponseInfo{
		Version:          a.cfg.VersionInfo.BuildVersion,
		AppVersion:       config.GetNetVersion(),
		LastBlockHeight:  lastBlockHeight,
		LastBlockAppHash: lastBlockAppHash,
	}
}

// SetOption set non-consensus critical application specific options.
func (a *App) SetOption(req abcitypes.RequestSetOption) abcitypes.ResponseSetOption {
	return abcitypes.ResponseSetOption{}
}

// CheckTx a proposed transaction for admission into the mempool.
// A non-zero response means the transaction is rejected and will not
// be broadcast to other nodes.
func (a *App) CheckTx(req abcitypes.RequestCheckTx) abcitypes.ResponseCheckTx {

	// Decode the transaction in byte form to types.BaseTx
	tx, err := msgs.DecodeTx(req.Tx)
	if err != nil {
		return abcitypes.ResponseCheckTx{
			Code: types.ErrCodeTxBadEncode,
			Log:  "unable to decode to types.BaseTx",
		}
	}

	// Perform validation
	if err = a.validateTx(tx, -1, a.logic); err != nil {
		return abcitypes.ResponseCheckTx{
			Code: types.ErrCodeTxFailedValidation,
			Log:  err.Error(),
		}
	}

	return abcitypes.ResponseCheckTx{Code: 0, Data: tx.GetHash().Bytes()}
}

// BeginBlock indicates the beginning of a new block.
func (a *App) BeginBlock(req abcitypes.RequestBeginBlock) abcitypes.ResponseBeginBlock {
	a.curWorkingBlock.Height = req.GetHeader().Height
	a.curWorkingBlock.Hash = req.GetHash()
	a.curWorkingBlock.LastAppHash = req.GetHeader().AppHash
	a.curWorkingBlock.ProposerAddress = req.GetHeader().ProposerAddress
	a.curWorkingBlock.Time = req.GetHeader().Time.Unix()

	if bytes.Equal(a.cfg.G().PrivVal.GetAddress().Bytes(), a.curWorkingBlock.ProposerAddress) {
		a.isCurrentBlockProposer = true
	}

	a.log.Info(color.YellowString("Processing a new block"),
		"Height", req.Header.Height, "IsProposer", a.isCurrentBlockProposer)

	return abcitypes.ResponseBeginBlock{}
}

func respDeliverTx(code uint32, log string) *abcitypes.ResponseDeliverTx {
	return &abcitypes.ResponseDeliverTx{
		Code: code,
		Log:  log,
	}
}

// preExecChecks performs some checks that attempt to spot problems with
// specific transaction types before they are validated. These checks are
// against the ABCI block execution session(s).
func (a *App) preExecChecks(tx msgs.BaseTx) *abcitypes.ResponseDeliverTx {

	// Invalidate the transaction if it is a validator ticket acquisition tx and
	// we have reached the maximum per block.
	// TODO: Slash proposer for violating the rule.
	if tx.Is(msgs.TxTypeValidatorTicket) &&
		len(a.unIdxValidatorTickets) == params.MaxValTicketsPerBlock {
		return respDeliverTx(types.ErrCodeMaxTxTypeReached,
			"failed to execute tx: validator ticket capacity reached")
	}

	return nil
}

// postExecChecks performs some checks that reacts to the
// result from executing a transaction.
func (a *App) postExecChecks(
	tx msgs.BaseTx,
	resp *abcitypes.ResponseDeliverTx) *abcitypes.ResponseDeliverTx {

	if !resp.IsOK() {
		return resp
	}

	switch o := tx.(type) {
	case *msgs.TxTicketPurchase:
		if o.Is(msgs.TxTypeValidatorTicket) {
			a.unIdxValidatorTickets = append(a.unIdxValidatorTickets, &ticketInfo{Tx: tx, index: a.txIndex})
		} else {
			a.unIdxStorerTickets = append(a.unIdxStorerTickets, &ticketInfo{Tx: tx, index: a.txIndex})
		}

	case *msgs.TxTicketUnbond:
		a.unbondStorerReqs = append(a.unbondStorerReqs, o.TicketHash)

	case *msgs.TxRepoCreate:
		a.newRepos = append(a.newRepos, o.Name)

	case *msgs.TxRepoProposalVote:
		a.unIdxRepoPropVotes = append(a.unIdxRepoPropVotes, o)

	case *msgs.TxPush:
		for _, ref := range o.PushNote.GetPushedReferences() {
			if ref.MergeProposalID != "" {
				a.unIdxClosedMergeProposal = append(a.unIdxClosedMergeProposal, &mergeProposalInfo{
					repo:       o.PushNote.RepoName,
					proposalID: ref.MergeProposalID,
				})
			}
		}
	}

	// Add the successfully processed tx to the un-indexed tx cache.
	// They will be committed in the COMMIT phase
	a.unIdxTxs = append(a.unIdxTxs, tx)

	return resp
}

// DeliverTx processes transactions included in a proposed block.
// Execute the transaction such that in modifies the blockchain state.
func (a *App) DeliverTx(req abcitypes.RequestDeliverTx) abcitypes.ResponseDeliverTx {

	// Increment the tx index
	a.txIndex++

	// Decode transaction to types.BaseTx
	tx, err := msgs.DecodeTx(req.Tx)
	if err != nil {
		return *respDeliverTx(types.ErrCodeTxBadEncode, "unable to decode to types.BaseTx")
	}

	// Perform validation
	if err = a.validateTx(tx, -1, a.logic); err != nil {
		a.log.Debug("DeliverTX: tx failed validation", "Err", err)
		return *respDeliverTx(types.ErrCodeTxFailedValidation, err.Error())
	}

	// Perform pre-execution checks
	if resp := a.preExecChecks(tx); resp != nil {
		return *resp
	}

	// Execute the transaction (does not commit the state changes yet)
	resp := a.logic.Tx().ExecTx(tx, uint64(a.curWorkingBlock.Height-1))

	// If the transaction returns an ErrCodeReExecBlock code, discard current
	// uncommitted state updates and return immediately because the current
	// block will be re-applied
	if resp.Code == state.ErrCodeReExecBlock {
		a.logic.Discard()
		return resp
	}

	// Perform post-execution checks
	return *a.postExecChecks(tx, &resp)
}

// updateValidators updates the validators of the chain.
func (a *App) updateValidators(curHeight int64, resp *abcitypes.ResponseEndBlock) error {

	// If it is not time to update validators, do nothing.
	if !params.IsBeforeEndOfEpoch(curHeight) {
		return nil
	}

	a.log.Info("Preparing to update validators", "Height", curHeight)

	// Get next set of validators
	selected, err := a.ticketMgr.GetTopValidators(params.MaxValidatorsPerEpoch)
	if err != nil {
		return err
	}

	// Do not update validators if no tickets were selected
	if len(selected) == 0 {
		a.log.Warn("Refused to update current validators since no tickets were selected")
		return nil
	}

	// Create a new validator list.
	// Keep an index of validators public key for faster query.
	var newValUpdates []abcitypes.ValidatorUpdate // for tendermint
	var newValidators []*types2.Validator         // for validator keeper
	var vIndex = map[string]struct{}{}
	for _, st := range selected {
		newValUpdates = append(newValUpdates, abcitypes.ValidatorUpdate{
			PubKey: abcitypes.PubKey{Type: "ed25519", Data: st.Ticket.ProposerPubKey.Bytes()},
			Power:  1,
		})
		newValidators = append(newValidators, &types2.Validator{
			PubKey:   st.Ticket.ProposerPubKey,
			TicketID: st.Ticket.Hash,
		})
		vIndex[st.Ticket.ProposerPubKey.HexStr()] = struct{}{}
	}

	// Get current validators
	curValidators, err := a.logic.ValidatorKeeper().GetByHeight(0)
	if err != nil {
		return err
	}

	// Set the power of existing validators to zero if they are not
	// part of the new list. It means they have been removed.
	for pubKey := range curValidators {
		if _, ok := vIndex[pubKey.HexStr()]; ok {
			continue
		}
		newValUpdates = append(newValUpdates, abcitypes.ValidatorUpdate{
			PubKey: abcitypes.PubKey{Type: "ed25519", Data: pubKey.Bytes()},
			Power:  0,
		})
	}

	// Set the new validators
	resp.ValidatorUpdates = newValUpdates

	// Cache the current validators; it will be persisted in a future block.
	// Note: Tendermint validator updates kicks in after H+2 blocks.
	a.unsavedValidators = newValidators
	a.heightToSaveNewValidators = curHeight + 1

	a.log.Info("Validators have successfully been updated",
		"NumValidators", len(a.unsavedValidators))

	return nil
}

// EndBlock indicates the end of a block.
// Note: Any error from operations in here should panic to stop the block from
// being committed.
func (a *App) EndBlock(req abcitypes.RequestEndBlock) abcitypes.ResponseEndBlock {
	resp := abcitypes.ResponseEndBlock{}

	// Update validators
	if err := a.updateValidators(req.Height, &resp); err != nil {
		panic(errors.Wrap(err, "failed to update validators"))
	}

	if err := a.logic.OnEndBlock(a.curWorkingBlock); err != nil {
		panic(errors.Wrap(err, "logic.OnEndBlock"))
	}

	return resp
}

// Commit persist the application state.
// It must return a merkle root hash of the application state.
func (a *App) Commit() abcitypes.ResponseCommit {
	defer a.reset()

	// Construct a new block information object
	bi := &types2.BlockInfo{
		Height:          a.curWorkingBlock.Height,
		Hash:            a.curWorkingBlock.Hash,
		LastAppHash:     a.curWorkingBlock.LastAppHash,
		ProposerAddress: a.curWorkingBlock.ProposerAddress,
		AppHash:         a.logic.StateTree().WorkingHash(),
		Time:            a.curWorkingBlock.Time,
	}

	// Save the block information
	if err := a.logic.SysKeeper().SaveBlockInfo(bi); err != nil {
		a.commitPanic(errors.Wrap(err, "failed to save block information"))
	}

	// Index tickets we have collected so far.
	for _, ticket := range append(a.unIdxValidatorTickets, a.unIdxStorerTickets...) {
		if err := a.ticketMgr.Index(ticket.Tx, uint64(a.curWorkingBlock.Height),
			ticket.index); err != nil {
			a.commitPanic(errors.Wrap(err, "failed to index ticket"))
		}
	}

	// Update the current validators record if the current block
	// height is the height where the last validator update will take effect.
	// Tendermint effects validator updates after 2 blocks; We need to index
	// the validators to the real height when the validators were selected (2 blocks ago)
	if a.curWorkingBlock.Height == a.heightToSaveNewValidators {
		if err := a.logic.ValidatorKeeper().
			Index(a.curWorkingBlock.Height, a.unsavedValidators); err != nil {
			a.commitPanic(errors.Wrap(err, "failed to update current validators"))
		}
		a.log.Info("Indexed new validators for the new epoch", "Height", a.curWorkingBlock.Height)
	}

	// Index the un-indexed txs
	for _, t := range a.unIdxTxs {
		if err := a.logic.TxKeeper().Index(t); err != nil {
			a.commitPanic(errors.Wrap(err, "failed to index transaction after commit"))
		}
	}

	// Index proposal votes
	for _, v := range a.unIdxRepoPropVotes {
		if err := a.logic.RepoKeeper().IndexProposalVote(v.RepoName, v.ProposalID,
			v.GetFrom().String(), v.Vote); err != nil {
			a.commitPanic(errors.Wrap(err, "failed to index repository proposal vote"))
		}
	}

	// Set the decay height for each storer stake unbond request
	for _, ticketHash := range a.unbondStorerReqs {
		a.logic.GetTicketManager().UpdateDecayBy(ticketHash, uint64(a.curWorkingBlock.Height))
	}

	// Create new repositories
	for _, repoName := range a.newRepos {
		if err := a.logic.GetRepoManager().CreateRepository(repoName); err != nil {
			a.commitPanic(errors.Wrap(err, "failed to create repository"))
		}
	}

	// Mark all merge proposals as closed.
	for _, info := range a.unIdxClosedMergeProposal {
		if err := a.logic.RepoKeeper().MarkProposalAsClosed(info.repo, info.proposalID); err != nil {
			a.commitPanic(errors.Wrap(err, "failed to mark merge proposal as closed"))
		}
	}

	// Commit all state changes
	if err := a.logic.Commit(); err != nil {
		a.commitPanic(errors.Wrap(err, "failed to commit"))
	}

	// Emit events about the committed transactions
	committedTxs := make([]msgs.BaseTx, len(a.unIdxTxs))
	copy(committedTxs, a.unIdxTxs)
	a.cfg.G().Bus.Emit(types.EvtABCICommittedTx, nil, committedTxs)

	return abcitypes.ResponseCommit{
		Data: bi.AppHash,
	}
}

// commitPanic cleans up resources, rollback logic tx and panic
func (a *App) commitPanic(err error) {
	a.logic.Discard()
	panic(err)
}

// reset cached values
func (a *App) reset() {
	a.unIdxValidatorTickets = []*ticketInfo{}
	a.unIdxStorerTickets = []*ticketInfo{}
	a.unbondStorerReqs = []util.Bytes32{}
	a.txIndex = 0
	a.isCurrentBlockProposer = false
	a.unIdxTxs = []msgs.BaseTx{}
	a.unIdxRepoPropVotes = []*msgs.TxRepoProposalVote{}
	a.newRepos = []string{}
	a.unIdxClosedMergeProposal = []*mergeProposalInfo{}

	// Only reset heightToSaveNewValidators if the current height is
	// same as it to avoid not triggering saving of new validators at the target height.
	if a.curWorkingBlock.Height == a.heightToSaveNewValidators {
		a.heightToSaveNewValidators = 0
	}

	a.curWorkingBlock = &types2.BlockInfo{}
}

// Query for data from the application.
func (a *App) Query(req abcitypes.RequestQuery) abcitypes.ResponseQuery {
	return abcitypes.ResponseQuery{Code: 0}
}
