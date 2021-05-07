package node

import (
	"bytes"

	"github.com/k0kubun/pp"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/logic/contracts/mergerequest"
	"github.com/make-os/kit/logic/keepers"
	"github.com/make-os/kit/params"
	"github.com/make-os/kit/pkgs/logger"
	storagetypes "github.com/make-os/kit/storage/types"
	tickettypes "github.com/make-os/kit/ticket/types"
	"github.com/make-os/kit/types"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util"
	fmt2 "github.com/make-os/kit/util/colorfmt"
	"github.com/make-os/kit/util/epoch"
	"github.com/make-os/kit/validation"
	"github.com/pkg/errors"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

type ticketInfo struct {
	Tx    types.BaseTx
	index int
}

type mergeProposalInfo struct {
	repo       string
	proposalID string
}

// blockTx represents a tx in a block
type blockTx struct {
	tx    types.BaseTx
	index int
}

// App implements tendermint ABCI interface to
type App struct {
	db                        storagetypes.Engine
	logic                     core.AtomicLogic
	cfg                       *config.AppConfig
	validateTx                validation.ValidateTxFunc
	curBlock                  *state.BlockInfo
	log                       logger.Logger
	txIndex                   int
	unIdxValidatorTickets     []*ticketInfo
	unIdxHostTickets          []*ticketInfo
	unbondHostReqs            []util.HexBytes
	ticketMgr                 tickettypes.TicketManager
	isCurrentBlockProposer    bool
	unsavedValidators         []*core.Validator
	heightToSaveNewValidators int64
	okTxs                     []blockTx
	repoPropTxs               []*txns.TxRepoProposalVote
	newRepos                  []string
	closedMergeProps          []*mergeProposalInfo
}

// NewApp creates an instance of App
func NewApp(
	cfg *config.AppConfig,
	db storagetypes.Engine,
	logic core.AtomicLogic,
	ticketMgr tickettypes.TicketManager) *App {
	return &App{
		db:         db,
		logic:      logic,
		cfg:        cfg,
		curBlock:   &state.BlockInfo{},
		log:        cfg.G().Log.Module("app"),
		ticketMgr:  ticketMgr,
		validateTx: validation.ValidateTx,
	}
}

// InitChain is called once upon genesis.
func (a *App) InitChain(req abcitypes.RequestInitChain) abcitypes.ResponseInitChain {

	stateTree := a.logic.StateTree()

	a.log.Info("Initializing for the first time")
	a.log.Info("Creating the chain and generating initial state")

	// Apply genesis state
	if err := a.logic.ApplyGenesisState(req.AppStateBytes); err != nil {
		panic(errors.Wrap(err, "failed to write genesis state"))
	}

	// Store genesis validators
	if err := a.logic.Validator().Index(1, req.GetValidators()); err != nil {
		panic(errors.Wrap(err, "failed to index validators"))
	}

	a.log.Info("Initial app state has been loaded",
		"GenesisHash", util.ToHex(stateTree.WorkingHash()),
		"StateVersion", stateTree.Version())

	return abcitypes.ResponseInitChain{}
}

// Info returns information about the application state.
// Used to sync tendermint with the application during a handshake that happens on startup.
// The returned AppVersion will be included in the header of every block.
// Tendermint expects LastBlockAppHash and LastBlockHeight to be updated during commit.
func (a *App) Info(abcitypes.RequestInfo) abcitypes.ResponseInfo {

	var lastBlockAppHash []byte
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
		lastBlockHeight = lastBlock.Height.Int64()
	}

	return abcitypes.ResponseInfo{
		Version:          a.cfg.VersionInfo.BuildVersion,
		AppVersion:       config.GetNetVersion(),
		LastBlockHeight:  lastBlockHeight,
		LastBlockAppHash: lastBlockAppHash,
	}
}

// CheckTx a proposed transaction for admission into the mempool.
// A non-zero response means the transaction is rejected and will not
// be broadcast to other nodes.
func (a *App) CheckTx(req abcitypes.RequestCheckTx) abcitypes.ResponseCheckTx {

	// Decode the transaction in byte form to types.BaseTx
	tx, err := txns.DecodeTx(req.Tx)
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

	return abcitypes.ResponseCheckTx{Code: 0, Data: tx.GetHash()}
}

// BeginBlock indicates the beginning of a new block.
func (a *App) BeginBlock(req abcitypes.RequestBeginBlock) abcitypes.ResponseBeginBlock {
	a.curBlock.Time.Set(req.GetHeader().Time.Unix())
	a.curBlock.Height.Set(req.GetHeader().Height)
	a.curBlock.Hash = req.GetHash()
	a.curBlock.LastAppHash = req.GetHeader().AppHash
	a.curBlock.ProposerAddress = req.GetHeader().ProposerAddress

	if bytes.Equal(a.cfg.G().PrivVal.GetAddress().Bytes(), a.curBlock.ProposerAddress) {
		a.isCurrentBlockProposer = true
	}

	a.log.Info(fmt2.YellowStringf("Processing a new block"),
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
func (a *App) preExecChecks(tx types.BaseTx) *abcitypes.ResponseDeliverTx {

	// Invalidate the transaction if it is a validator ticket acquisition tx and
	// we have reached the maximum per block.
	// TODO: Slash proposer for violating the rule.
	if tx.Is(txns.TxTypeValidatorTicket) &&
		len(a.unIdxValidatorTickets) == params.MaxValTicketsPerBlock {
		return respDeliverTx(types.ErrCodeMaxTxTypeReached,
			"failed to execute tx: validator ticket capacity reached")
	}

	return nil
}

// postExec initiates events based on specific, successfully processed transactions
func (a *App) postExec(tx types.BaseTx, resp *abcitypes.ResponseDeliverTx) *abcitypes.ResponseDeliverTx {
	switch o := tx.(type) {
	case *txns.TxTicketPurchase:
		if o.Is(txns.TxTypeValidatorTicket) {
			a.unIdxValidatorTickets = append(a.unIdxValidatorTickets, &ticketInfo{Tx: tx, index: a.txIndex})
		} else {
			a.unIdxHostTickets = append(a.unIdxHostTickets, &ticketInfo{Tx: tx, index: a.txIndex})
		}

	case *txns.TxTicketUnbond:
		a.unbondHostReqs = append(a.unbondHostReqs, o.TicketHash)

	case *txns.TxRepoCreate:
		a.newRepos = append(a.newRepos, o.Name)

	case *txns.TxRepoProposalVote:
		a.repoPropTxs = append(a.repoPropTxs, o)

	case *txns.TxPush:
		for _, ref := range o.Note.GetPushedReferences() {
			if ref.MergeProposalID == "" {
				continue
			}
			a.closedMergeProps = append(a.closedMergeProps, &mergeProposalInfo{
				repo:       o.Note.GetRepoName(),
				proposalID: mergerequest.MakeMergeRequestProposalID(ref.MergeProposalID),
			})
		}

	case *txns.TxSubmitWork:
		pp.Println("ABC")
		a.logic.SysKeeper().IncrGasMinedInCurEpoch(params.GasReward)
	}

	// Keep reference of successfully processed txs
	a.okTxs = append(a.okTxs, blockTx{tx, a.txIndex})

	return resp
}

// DeliverTx processes transactions included in a proposed block.
// Execute the transaction such that in modifies the blockchain state.
func (a *App) DeliverTx(req abcitypes.RequestDeliverTx) abcitypes.ResponseDeliverTx {

	// Increment the tx index
	a.txIndex++

	// Decode transaction to types.BaseTx
	tx, err := txns.DecodeTx(req.Tx)
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
	resp := a.logic.ExecTx(&core.ExecArgs{
		Tx:          tx,
		ChainHeight: uint64(a.curBlock.Height - 1),
		ValidateTx:  validation.ValidateTx,
	})

	if !resp.IsOK() {
		a.log.Error("Transaction execution failed", "Err", resp.Log)
		return resp
	}

	// Perform post-execution checks
	return *a.postExec(tx, &resp)
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

	if err := a.logic.OnEndBlock(a.curBlock); err != nil {
		panic(errors.Wrap(err, "logic.OnEndBlock"))
	}

	return resp
}

// Commit persist the application state.
// It must return a merkle root hash of the application state.
func (a *App) Commit() abcitypes.ResponseCommit {
	defer a.reset()

	// Construct a new block information object
	bi := &state.BlockInfo{
		Height:          a.curBlock.Height,
		Hash:            a.curBlock.Hash,
		LastAppHash:     a.curBlock.LastAppHash,
		ProposerAddress: a.curBlock.ProposerAddress,
		AppHash:         a.logic.StateTree().WorkingHash(),
		Time:            a.curBlock.Time,
	}

	// Save the block information
	if err := a.logic.SysKeeper().SaveBlockInfo(bi); err != nil {
		a.commitPanic(errors.Wrap(err, "failed to save block information"))
	}

	// Index tickets we have collected so far.
	a.indexTickets()

	// Update the current validators record if the current block
	// height is the height where the last validator update will take effect.
	// Tendermint effects validator updates after 2 blocks; We need to index
	// the validators to the real height when the validators were selected (2 blocks ago)
	if a.curBlock.Height.Int64() == a.heightToSaveNewValidators {
		a.indexValidators()
		a.log.Info("Indexed new validators for the new epoch", "Height", a.curBlock.Height)
	}

	// Index the un-indexed txs
	a.broadcastTx()

	// Index proposal votes
	a.indexProposalVotes()

	// Set the expire height for each host stake unbond request
	a.expireHostTickets()

	// Create new repositories
	_ = a.createGitRepositories()

	// Mark all merge proposals as closed.
	a.markMergeProposalAsClosed()

	// Update difficulty
	a.updateDifficulty(a.curBlock)

	// Commit all state changes
	if err := a.logic.Commit(); err != nil {
		a.commitPanic(errors.Wrap(err, "failed to commit"))
	}

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
	a.unIdxHostTickets = []*ticketInfo{}
	a.unbondHostReqs = []util.HexBytes{}
	a.txIndex = 0
	a.isCurrentBlockProposer = false
	a.okTxs = []blockTx{}
	a.repoPropTxs = []*txns.TxRepoProposalVote{}
	a.newRepos = []string{}
	a.closedMergeProps = []*mergeProposalInfo{}

	// Only reset heightToSaveNewValidators if the current height is
	// same as it to avoid not triggering saving of new validators at the target height.
	if a.curBlock.Height.Int64() == a.heightToSaveNewValidators {
		a.heightToSaveNewValidators = 0
	}

	a.curBlock = &state.BlockInfo{}
}

// Query for data from the application.
func (a *App) Query(abcitypes.RequestQuery) abcitypes.ResponseQuery {
	return abcitypes.ResponseQuery{Code: 0}
}

// updateValidators updates the validators of the chain.
func (a *App) updateValidators(curHeight int64, resp *abcitypes.ResponseEndBlock) error {

	// If it is not time to update validators, do nothing.
	if !epoch.IsBeforeEndOfEpochOfHeight(curHeight) {
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
	var newValidators []*core.Validator           // for validator keeper
	var vIndex = map[string]struct{}{}
	for _, st := range selected {
		pubKey := st.Ticket.ProposerPubKey
		newValUpdates = append(newValUpdates, abcitypes.Ed25519ValidatorUpdate(pubKey.Bytes(), 1))
		newValidators = append(newValidators, &core.Validator{PubKey: pubKey, TicketID: st.Ticket.Hash})
		vIndex[pubKey.HexStr()] = struct{}{}
	}

	// Get current validators
	curValidators, err := a.logic.ValidatorKeeper().Get(0)
	if err != nil {
		return err
	}

	// Set the power of existing validators to zero if they are not
	// part of the new list. It means they have been removed.
	for pubKey := range curValidators {
		if _, ok := vIndex[pubKey.HexStr()]; ok {
			continue
		}
		newValUpdates = append(newValUpdates, abcitypes.Ed25519ValidatorUpdate(pubKey.Bytes(), 0))
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

// indexValidators indexes new validators
func (a *App) indexValidators() {
	if err := a.logic.ValidatorKeeper().Index(a.curBlock.Height.Int64(), a.unsavedValidators); err != nil {
		a.commitPanic(errors.Wrap(err, "failed to update current validators"))
	}
}

// createGitRepositories creates new git repositories.
// If the node is in validator node, no git repository is created.
// If the node is tracking repositories, only tracked repositories will be created.
func (a *App) createGitRepositories() error {

	if len(a.newRepos) == 0 {
		return nil
	}

	if a.cfg.IsValidatorNode() {
		return types.ErrSkipped
	}

	tracked := a.logic.RepoSyncInfoKeeper().Tracked()
	for _, repoName := range a.newRepos {
		if len(tracked) > 0 && tracked[repoName] == nil {
			continue
		}
		if err := a.logic.GetRemoteServer().InitRepository(repoName); err != nil {
			a.commitPanic(errors.Wrap(err, "failed to create repository"))
		}
	}

	return nil
}

// markMergeProposalAsClosed marks a merge proposal as closed.
func (a *App) markMergeProposalAsClosed() {
	for _, info := range a.closedMergeProps {
		if err := a.logic.RepoKeeper().MarkProposalAsClosed(info.repo, info.proposalID); err != nil {
			a.commitPanic(errors.Wrap(err, "failed to mark merge proposal as closed"))
		}
	}
}

// expireHostTickets sets the expiry height of unbonded host tickets
func (a *App) expireHostTickets() {
	for _, ticketHash := range a.unbondHostReqs {
		if err := a.logic.GetTicketManager().UpdateExpireBy(ticketHash, uint64(a.curBlock.Height)); err != nil {
			a.commitPanic(errors.Wrap(err, "failed to expiire host tickets"))
		}
	}
}

// indexProposalVotes indexes a vote for on a proposal
func (a *App) indexProposalVotes() {
	for _, v := range a.repoPropTxs {
		if err := a.logic.RepoKeeper().IndexProposalVote(v.RepoName, v.ProposalID,
			v.GetFrom().String(), v.Vote); err != nil {
			a.commitPanic(errors.Wrap(err, "failed to index repository proposal vote"))
		}
	}
}

// broadcastTx selected transactions that may be need by other app processes
func (a *App) broadcastTx() {
	for _, btx := range a.okTxs {
		if btx.tx.Is(txns.TxTypePush) {
			a.cfg.G().Bus.Emit(core.EvtTxPushProcessed, btx.tx.(*txns.TxPush), a.curBlock.Height.Int64(), btx.index)
		}
	}
}

// indexTickets indexes new validator and host tickets
func (a *App) indexTickets() {
	for _, ticket := range append(a.unIdxValidatorTickets, a.unIdxHostTickets...) {
		if err := a.ticketMgr.Index(ticket.Tx, uint64(a.curBlock.Height),
			ticket.index); err != nil {
			a.commitPanic(errors.Wrap(err, "failed to index ticket"))
		}
	}
}

// updateDifficulty will update the difficulty at the end of the current epoch.
func (a *App) updateDifficulty(block *state.BlockInfo) {
	isEpochEnding := epoch.GetLastHeightInEpochOfHeight(block.Height.Int64()) == block.Height.Int64()
	if !isEpochEnding {
		return
	}

	return
}

func (a *App) ListSnapshots(snapshots abcitypes.RequestListSnapshots) abcitypes.ResponseListSnapshots {
	panic("implement me")
}

func (a *App) OfferSnapshot(snapshot abcitypes.RequestOfferSnapshot) abcitypes.ResponseOfferSnapshot {
	panic("implement me")
}

func (a *App) LoadSnapshotChunk(chunk abcitypes.RequestLoadSnapshotChunk) abcitypes.ResponseLoadSnapshotChunk {
	panic("implement me")
}

func (a *App) ApplySnapshotChunk(chunk abcitypes.RequestApplySnapshotChunk) abcitypes.ResponseApplySnapshotChunk {
	panic("implement me")
}
