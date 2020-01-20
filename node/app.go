package node

import (
	"bytes"
	"fmt"
	"github.com/tendermint/tendermint/state"

	"github.com/fatih/color"
	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/crypto/vrf"
	"github.com/makeos/mosdef/logic/keepers"
	"github.com/makeos/mosdef/params"
	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
	"github.com/makeos/mosdef/util/logger"
	"github.com/makeos/mosdef/validators"
	"github.com/pkg/errors"
	abcitypes "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto/ed25519"
)

type ticketInfo struct {
	Tx    types.BaseTx
	index int
}

// App implements tendermint ABCI interface to
type App struct {
	db                        storage.Engine
	logic                     types.AtomicLogic
	cfg                       *config.AppConfig
	validateTx                validators.ValidateTxFunc
	curWorkingBlock           *types.BlockInfo
	log                       logger.Logger
	txIndex                   int
	validatorTickets          []*ticketInfo
	storerTickets             []*ticketInfo
	unbondStorerReqs          []util.Bytes32
	ticketMgr                 types.TicketManager
	epochSeedTx               types.BaseTx
	isCurrentBlockProposer    bool
	mature                    bool
	unsavedValidators         []*types.Validator
	heightToSaveNewValidators int64
	unIndexedTxs              []types.BaseTx
	newRepos                  []string
}

// NewApp creates an instance of App
func NewApp(
	cfg *config.AppConfig,
	db storage.Engine,
	logic types.AtomicLogic,
	ticketMgr types.TicketManager) *App {
	return &App{
		db:              db,
		logic:           logic,
		cfg:             cfg,
		curWorkingBlock: &types.BlockInfo{},
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
	tx, err := types.DecodeTx(req.Tx)
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
	curHeight := req.GetHeader().Height
	a.curWorkingBlock.Height = req.GetHeader().Height
	a.curWorkingBlock.Hash = req.GetHash()
	a.curWorkingBlock.LastAppHash = req.GetHeader().AppHash
	a.curWorkingBlock.ProposerAddress = req.GetHeader().ProposerAddress

	if bytes.Equal(a.cfg.G().PrivVal.GetAddress().Bytes(), a.curWorkingBlock.ProposerAddress) {
		a.isCurrentBlockProposer = true
	}

	a.log.Info(color.YellowString("Processing a new block"),
		"Height", req.Header.Height, "IsProposer", a.isCurrentBlockProposer)

	// If the network is still immature, return immediately.
	// The network is matured if it has reached a specific block height
	// and has accumulated a specific number of active tickets.
	if err := a.logic.Sys().CheckSetNetMaturity(); err != nil {
		a.log.Debug("The network has not reached maturity", "Reason", err, "Height", curHeight)
		a.mature = false
		return abcitypes.ResponseBeginBlock{}
	}

	// At this point, the network is mature
	a.mature = true

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
	if tx.Is(types.TxTypeValidatorTicket) &&
		len(a.validatorTickets) == params.MaxValTicketsPerBlock {
		return respDeliverTx(types.ErrCodeMaxTxTypeReached,
			"failed to execute tx: validator ticket capacity reached")
	}

	if tx.Is(types.TxTypeEpochSeed) {

		// Invalidate the epoch seed tx if the current block is not the first
		// block in the current epoch's end phase.
		// TODO: Slash the proposer for violating this rule.
		if !params.IsStartOfEndOfEpochOfHeight(a.curWorkingBlock.Height) {
			return respDeliverTx(types.ErrCodeEpochSeedNotExpected,
				"failed to execute tx: epoch seed not expected")
		}

		// Invalidate the epoch seed tx if we have already seen one in this block.
		// TODO: Slash proposer for violating this rule.
		if a.epochSeedTx != nil {
			return respDeliverTx(types.ErrCodeEpochSecretExcess,
				"failed to execute tx: epoch seed capacity reached")
		}

		// Perform seed verification
		if err := a.verifyBlockProposerSeedTx(tx.(*types.TxEpochSeed)); err != nil {
			return respDeliverTx(types.ErrCodeTxFailedSeedVerification,
				errors.Wrap(err, "failed to execute epoch seed tx").Error())
		}
	}

	return nil
}

// postExecChecks performs some checks that reacts to the result from executing a transaction.
func (a *App) postExecChecks(
	tx types.BaseTx,
	resp abcitypes.ResponseDeliverTx) *abcitypes.ResponseDeliverTx {

	txType := tx.GetType()
	switch txType {
	case types.TxTypeEpochSeed:
		a.epochSeedTx = tx

	case types.TxTypeValidatorTicket:
		a.validatorTickets = append(a.validatorTickets, &ticketInfo{Tx: tx, index: a.txIndex})

	case types.TxTypeStorerTicket:
		a.storerTickets = append(a.storerTickets, &ticketInfo{Tx: tx, index: a.txIndex})

	case types.TxTypeUnbondStorerTicket:
		a.unbondStorerReqs = append(a.unbondStorerReqs, tx.(*types.TxTicketUnbond).TicketHash)

	case types.TxTypeRepoCreate:
		a.newRepos = append(a.newRepos, tx.(*types.TxRepoCreate).Name)
	}

	// Add the successfully processed tx to the un-indexed tx cache.
	// They will be committed in the COMMIT phase
	a.unIndexedTxs = append(a.unIndexedTxs, tx)

	return &resp
}

// verifyBlockProposerSeedTx verifies an epoch seed transaction that was found
// the current working block. It attempts to find the proposer's VRF public key
// and uses it to check the vrf output and proof.
func (a *App) verifyBlockProposerSeedTx(seedTx *types.TxEpochSeed) error {

	// Get the validators of the current epoch
	validators, err := a.logic.ValidatorKeeper().GetByHeight(a.curWorkingBlock.Height - 1)
	if err != nil {
		return errors.Wrap(err, "failed to get current validators")
	}

	// Since the current block proposer is a member of the current epoch
	// validators, find her ticket so we can learn about her VRF public key
	var ticket *types.Ticket
	for pubKey, valInfo := range validators {

		var pub32 ed25519.PubKeyEd25519
		copy(pub32[:], pubKey.Bytes())
		if !bytes.Equal(pub32.Address().Bytes(), a.curWorkingBlock.ProposerAddress) {
			continue
		}

		ticket = a.ticketMgr.GetByHash(valInfo.TicketID)
		if ticket == nil {
			return errors.Wrap(err, "failed to find an active ticket")
		}

		break
	}

	if ticket == nil {
		return fmt.Errorf("ticket not found")
	}

	// At this point, we have found the ticket. Use the VRF public key to it to
	// verify the VRF proof
	vrfPubKey := vrf.PublicKey(ticket.VRFPubKey.Bytes())
	lastEpochSeed, err := a.logic.Sys().GetLastEpochSeed(a.curWorkingBlock.Height - 1)
	if err != nil {
		return errors.Wrap(err, "failed to get last epoch seed")
	}
	if !vrfPubKey.Verify(lastEpochSeed.Bytes(), seedTx.Output.Bytes(), seedTx.Proof) {
		return fmt.Errorf("failed to verify vrf proof")
	}

	return nil
}

// DeliverTx processes transactions included in a proposed block.
// Execute the transaction such that in modifies the blockchain state.
func (a *App) DeliverTx(req abcitypes.RequestDeliverTx) abcitypes.ResponseDeliverTx {

	// Increment the tx index
	a.txIndex++

	// Decode transaction to types.BaseTx
	tx, err := types.DecodeTx(req.Tx)
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
	return *a.postExecChecks(tx, resp)
}

// updateValidators updates the validators of the chain.
func (a *App) updateValidators(curHeight int64, resp *abcitypes.ResponseEndBlock) error {

	// If it is not time to update validators, do nothing.
	if !params.IsBeforeEndOfEpoch(curHeight) {
		return nil
	}

	a.log.Info("Preparing to update validators", "Height", curHeight)

	// Get secret computed from past epoch
	secret, err := a.logic.Sys().MakeSecret(curHeight - 1)
	if err != nil {
		return err
	}

	// Get next set of validators randomly
	tickets, err := a.ticketMgr.SelectRandomValidatorTickets(curHeight-1, secret,
		params.MaxValidatorsPerEpoch)
	if err != nil {
		return err
	}

	// Do not update validators if no tickets were selected
	if len(tickets) == 0 {
		a.log.Warn("Refused to update current validators since no tickets were selected")
		return nil
	}

	// Create a new validator list. Keep an index of validators
	// public key for fast query
	var newValUpdates []abcitypes.ValidatorUpdate // for tendermint
	var newValidators []*types.Validator          // for validator keeper
	var vIndex = map[string]struct{}{}
	for _, ticket := range tickets {
		newValUpdates = append(newValUpdates, abcitypes.ValidatorUpdate{
			PubKey: abcitypes.PubKey{Type: "ed25519", Data: ticket.ProposerPubKey.Bytes()},
			Power:  1,
		})
		newValidators = append(newValidators, &types.Validator{
			PubKey:   ticket.ProposerPubKey,
			TicketID: ticket.Hash,
		})
		vIndex[ticket.ProposerPubKey.HexStr()] = struct{}{}
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

// EndBlock indicates the end of a block
func (a *App) EndBlock(req abcitypes.RequestEndBlock) abcitypes.ResponseEndBlock {
	resp := abcitypes.ResponseEndBlock{}

	// Update validators only if network is mature
	if a.mature {
		if err := a.updateValidators(req.Height, &resp); err != nil {
			panic(errors.Wrap(err, "failed to update validators"))
		}
	}

	return resp
}

// Commit persist the application state.
// It must return a merkle root hash of the application state.
func (a *App) Commit() abcitypes.ResponseCommit {
	defer a.reset()

	// Construct a new block information object
	bi := &types.BlockInfo{
		Height:          a.curWorkingBlock.Height,
		Hash:            a.curWorkingBlock.Hash,
		LastAppHash:     a.curWorkingBlock.LastAppHash,
		ProposerAddress: a.curWorkingBlock.ProposerAddress,
		AppHash:         a.logic.StateTree().WorkingHash(),
	}

	// If we found an epoch seed in this block, store the seed information in
	// the block information object
	if seedTx := a.epochSeedTx; seedTx != nil {
		bi.EpochSeedOutput = seedTx.(*types.TxEpochSeed).Output
		bi.EpochSeedProof = seedTx.(*types.TxEpochSeed).Proof
	} else {
		// Ok, so no epoch seed tx in this block. If the block is the first
		// block in the current epoch end phase. We need to
		// TODO: Slash the proposer for not adding a secret.
		if params.IsStartOfEndOfEpochOfHeight(bi.Height) {

		}
	}

	// Save the block information
	if err := a.logic.SysKeeper().SaveBlockInfo(bi); err != nil {
		a.commitPanic(errors.Wrap(err, "failed to save block information"))
	}

	// Index tickets we have collected so far.
	for _, ptx := range append(a.validatorTickets, a.storerTickets...) {
		if err := a.ticketMgr.Index(ptx.Tx, uint64(a.curWorkingBlock.Height), ptx.index); err != nil {
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
	for _, t := range a.unIndexedTxs {
		if err := a.logic.TxKeeper().Index(t); err != nil {
			a.commitPanic(errors.Wrap(err, "failed to index transaction after commit"))
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

	// Commit all state changes
	if err := a.logic.Commit(); err != nil {
		a.commitPanic(errors.Wrap(err, "failed to commit"))
	}

	// Emit events about the committed transactions
	committedTxs := make([]types.BaseTx, len(a.unIndexedTxs))
	copy(committedTxs, a.unIndexedTxs)
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
	a.validatorTickets = []*ticketInfo{}
	a.storerTickets = []*ticketInfo{}
	a.unbondStorerReqs = []util.Bytes32{}
	a.txIndex = 0
	a.epochSeedTx = nil
	a.mature = false
	a.isCurrentBlockProposer = false
	a.unIndexedTxs = []types.BaseTx{}
	a.newRepos = []string{}

	// Only reset heightToSaveNewValidators if the current height is
	// same as it to avoid not triggering saving of new validators at the target height.
	if a.curWorkingBlock.Height == a.heightToSaveNewValidators {
		a.heightToSaveNewValidators = 0
	}

	a.curWorkingBlock = &types.BlockInfo{}
}

// Query for data from the application.
func (a *App) Query(req abcitypes.RequestQuery) abcitypes.ResponseQuery {
	return abcitypes.ResponseQuery{Code: 0}
}
