package node

import (
	"fmt"

	"github.com/fatih/color"

	"github.com/makeos/mosdef/crypto"

	"github.com/makeos/mosdef/params"

	"github.com/tendermint/tendermint/libs/common"

	"github.com/makeos/mosdef/util"

	"github.com/makeos/mosdef/util/logger"

	"github.com/makeos/mosdef/logic/keepers"

	"github.com/pkg/errors"

	"github.com/makeos/mosdef/config"

	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/validators"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

type tickPurchaseTx struct {
	Tx    *types.Transaction
	index int
}

// App implements tendermint ABCI interface to
type App struct {
	db                        storage.Engine
	node                      types.CommonNode
	logic                     types.AtomicLogic
	cfg                       *config.EngineConfig
	validateTx                validators.ValidateTxFunc
	wBlock                    *types.BlockInfo
	log                       logger.Logger
	txIndex                   int
	ticketPurchaseTxs         []*tickPurchaseTx
	ticketMgr                 types.TicketManager
	epochSecretTx             *types.Transaction
	isCurrentBlockProposer    bool
	mature                    bool
	latestUnsavedValidators   []*types.Validator
	heightToSaveNewValidators int64
	unIndexedTxs              []*types.Transaction
}

// NewApp creates an instance of App
func NewApp(
	cfg *config.EngineConfig,
	db storage.Engine,
	logic types.AtomicLogic,
	ticketMgr types.TicketManager) *App {
	return &App{
		db:         db,
		logic:      logic,
		cfg:        cfg,
		wBlock:     &types.BlockInfo{},
		log:        cfg.G().Log.Module("App"),
		ticketMgr:  ticketMgr,
		validateTx: validators.ValidateTx,
	}
}

// InitChain is called once upon genesis.
func (a *App) InitChain(req abcitypes.RequestInitChain) abcitypes.ResponseInitChain {

	stateTree := a.logic.StateTree()

	a.log.Info("Initializing for the first time")
	a.log.Info("Creating the chain and populating initial state...")

	// State must be empty
	if stateTree.WorkingHash() != nil {
		panic(fmt.Errorf("At init, state must be empty...It is not empty"))
	}

	// Write genesis state (e.g root accounts)
	if err := a.logic.WriteGenesisState(); err != nil {
		panic(errors.Wrap(err, "failed to write genesis state"))
	}

	// Store genesis validators
	if err := a.logic.Validator().Index(1, req.GetValidators()); err != nil {
		panic(errors.Wrap(err, "failed to index validators"))
	}

	// Commit all data
	if err := a.logic.Commit(); err != nil {
		panic(errors.Wrap(err, "failed to commit"))
	}

	a.log.Info("Node initialization has completed",
		"GenesisHash", util.BytesToHash(stateTree.WorkingHash()).HexStr(),
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

	// Decode the transaction in byte form to types.Transaction
	tx, err := types.NewTxFromBytes(req.Tx)
	if err != nil {
		return abcitypes.ResponseCheckTx{
			Code: types.ErrCodeTxBadEncode,
			Log:  "unable to decode to types.Transaction",
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
	a.wBlock.Height = req.GetHeader().Height
	a.wBlock.Hash = req.GetHash()
	a.wBlock.LastAppHash = req.GetHeader().AppHash
	a.wBlock.ProposerAddress = common.HexBytes(req.GetHeader().ProposerAddress).String()

	if a.cfg.G().PrivVal.GetAddress().String() == a.wBlock.ProposerAddress {
		a.isCurrentBlockProposer = true
	}

	a.log.Info(color.YellowString("🔨 Processing a new block"),
		"Height", req.Header.Height, "IsProposer", a.isCurrentBlockProposer)

	// If the network is still immature, return immediately.
	// The network is matured if it has reached a specific block height
	// and has accumulated a specific number of alive tickets.
	if err := a.logic.Sys().CheckSetNetMaturity(); err != nil {
		a.log.Debug("The network has not reached maturity :(", "Reason", err, "Height", curHeight)
		a.mature = false
		return abcitypes.ResponseBeginBlock{}
	}

	// At this point, the network is mature
	a.mature = true
	return abcitypes.ResponseBeginBlock{}
}

// preExecChecks performs some checks that attempt to spot problems with
// specific transaction types and possible activate slashing conditions
// and invalidate the transaction before it ever gets executed.
func (a *App) preExecChecks(tx *types.Transaction) *abcitypes.ResponseDeliverTx {

	txType := tx.GetType()

	// Invalidate the transaction if it is a validator ticket
	// purchasing tx and we have reached the max per block.
	// TODO: Slash proposer for violating the rule.
	if txType == types.TxTypeGetValidatorTicket &&
		len(a.ticketPurchaseTxs) == params.MaxValTicketsPerBlock {
		return &abcitypes.ResponseDeliverTx{
			Code: types.ErrCodeMaxTxTypeReached,
			Log:  "failed to execute tx: validator ticket capacity reached",
		}
	}

	if txType == types.TxTypeEpochSecret {
		// Invalidate the epoch secret tx if the current block is not the
		// last block in the current epoch.
		// TODO: Slash the proposer for violating this rule.
		if (a.wBlock.Height)%int64(params.NumBlocksPerEpoch) != 0 {
			return &abcitypes.ResponseDeliverTx{
				Code: types.ErrCodeTxTypeUnexpected,
				Log:  "failed to execute tx: epoch secret not expected",
			}
		}

		// Invalidate the epoch secret tx if we have already seen one in this block.
		// TODO: Slash proposer for violating this rule.
		if a.epochSecretTx != nil {
			return &abcitypes.ResponseDeliverTx{
				Code: types.ErrCodeMaxTxTypeReached,
				Log:  "failed to execute tx: epoch secret capacity reached",
			}
		}
	}

	return nil
}

// postExecChecks performs some checks that reacts to the result
// from executing a transaction. In here we can activate slashing
// conditions and invalidate the transaction
func (a *App) postExecChecks(
	tx *types.Transaction,
	resp abcitypes.ResponseDeliverTx) *abcitypes.ResponseDeliverTx {

	txType := tx.GetType()

	if txType == types.TxTypeEpochSecret {

		// Cache the epoch secret tx for use in the COMMIT stage
		a.epochSecretTx = tx

		// At the point, the proposer proposed an epoch secret tx whose round is earlier
		// or same as the last epoch secret round. We respond by invalidating the tx
		// object so that the COMMIT phase can flag it.
		// Also, TODO: Slash the proposer for doing this.
		if resp.Code != 0 && types.IsStaleSecretRoundErr(fmt.Errorf(resp.Log)) {
			tx.Invalidate()
			return &abcitypes.ResponseDeliverTx{
				Code: types.ErrCodeTxInvalidValue,
				Log:  "failed to execute tx: " + resp.Log,
			}
		}

		// Here, the proposer proposed an epoch secret tx whose round was
		// produced at a time earlier that the expected time a new drand
		// will be produced. We respond by invalidating the tx object so
		// that the COMMIT phase can flag it. Also, TODO: Slash the proposer for doing this.
		if resp.Code != 0 && types.IsEarlySecretRoundErr(fmt.Errorf(resp.Log)) {
			tx.Invalidate()
			return &abcitypes.ResponseDeliverTx{
				Code: types.ErrCodeTxInvalidValue,
				Log:  "failed to execute tx: " + resp.Log,
			}
		}
	}

	// Cache ticket purchase transaction; They will be indexed in the COMMIT stage.
	if resp.Code == 0 && txType == types.TxTypeGetValidatorTicket {
		a.ticketPurchaseTxs = append(a.ticketPurchaseTxs, &tickPurchaseTx{
			Tx:    tx,
			index: a.txIndex,
		})
	}

	// Add the successfully processed tx to the un-indexed tx cache.
	// They will be committed in the COMMIT phase
	if resp.Code == 0 {
		a.unIndexedTxs = append(a.unIndexedTxs, tx)
	}

	return &resp
}

// DeliverTx processes transactions included in a proposed block.
// Execute the transaction such that in modifies the blockchain state.
func (a *App) DeliverTx(req abcitypes.RequestDeliverTx) abcitypes.ResponseDeliverTx {

	// Increment the tx index
	a.txIndex++

	// Decode transaction to types.Transaction
	tx, err := types.NewTxFromBytes(req.Tx)
	if err != nil {
		return abcitypes.ResponseDeliverTx{
			Code: types.ErrCodeTxBadEncode,
			Log:  "unable to decode to types.Transaction",
		}
	}

	// Perform validation
	if err = a.validateTx(tx, -1, a.logic); err != nil {
		return abcitypes.ResponseDeliverTx{
			Code: types.ErrCodeTxFailedValidation,
			Log:  err.Error(),
		}
	}

	// Perform pre execution checks
	if resp := a.preExecChecks(tx); resp != nil {
		return *resp
	}

	// Execute the transaction (does not commit the state changes yet)
	resp := a.logic.Tx().PrepareExec(req, uint64(a.wBlock.Height-1))

	// Perform post execution operations
	return *a.postExecChecks(tx, resp)
}

// updateValidators updates the validators of the chain.
func (a *App) updateValidators(curHeight int64, resp *abcitypes.ResponseEndBlock) error {

	// If it is not time to update validators, do nothing.
	if curHeight%int64(params.NumBlocksPerEpoch) != 1 {
		return nil
	}

	// Get secret computed from past epoch
	secret, err := a.logic.Sys().MakeSecret(curHeight - 1)
	if err != nil {
		return err
	}

	// Get next validators; We made use of the
	// secret seed to randomize the op
	tickets, err := a.ticketMgr.SelectRandom(curHeight-1, secret, params.MaxValidatorsPerEpoch)
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
		pubKey, _ := crypto.PubKeyFromBase58(ticket.ProposerPubKey)
		pkBz := pubKey.MustBytes()
		newValUpdates = append(newValUpdates, abcitypes.ValidatorUpdate{
			PubKey: abcitypes.PubKey{Type: "ed25519", Data: pkBz},
			Power:  1,
		})
		newValidators = append(newValidators, &types.Validator{
			PubKey:   pkBz,
			Power:    1,
			TicketID: ticket.Hash,
		})
		pkHex := types.HexBytes(pkBz)
		vIndex[pkHex.String()] = struct{}{}
	}

	// Get current validators
	curValidators, err := a.logic.ValidatorKeeper().GetByHeight(0)
	if err != nil {
		return err
	}

	// Set the power of existing validators to zero if they are not
	// part of the new list. It means they have been removed.
	for pkHexStr := range curValidators {
		pkHex := types.HexBytesFromHex(pkHexStr)
		if _, ok := vIndex[pkHexStr]; ok {
			continue
		}
		newValUpdates = append(newValUpdates, abcitypes.ValidatorUpdate{
			PubKey: abcitypes.PubKey{Type: "ed25519", Data: pkHex},
			Power:  0,
		})
	}

	// Set the new validators
	resp.ValidatorUpdates = newValUpdates

	// Cache the current validators; it will be persisted in a future blocks.
	// Note: Tendermint validator updates kicks in after H+2 block.
	a.latestUnsavedValidators = newValidators
	a.heightToSaveNewValidators = curHeight + 1

	a.log.Info("Validators have successfully been updated",
		"NumValidators", len(a.latestUnsavedValidators))

	return nil
}

// EndBlock indicates the end of a block
func (a *App) EndBlock(req abcitypes.RequestEndBlock) abcitypes.ResponseEndBlock {
	resp := abcitypes.ResponseEndBlock{}

	// Update validators if network is mature
	if a.mature {
		if err := a.updateValidators(req.Height, &resp); err != nil {
			panic(errors.Wrap(err, "failed to update validators"))
		}
	}

	return resp
}

func isEndOfEpoch(height int64) bool {
	return height%int64(params.NumBlocksPerEpoch) == 0
}

// commitPanic cleans up resources or data that should not exit
// due to commit panicking
func (a *App) commitPanic(err error) {

	// Delete any already indexed ticket purchased in this block
	for _, t := range a.ticketPurchaseTxs {
		a.ticketMgr.Remove(t.Tx.GetID())
	}

	panic(err)
}

// Commit persist the application state.
// It must return a merkle root hash of the application state.
func (a *App) Commit() abcitypes.ResponseCommit {
	defer a.reset()

	// Construct a new block information object
	bi := &types.BlockInfo{
		Height:          a.wBlock.Height,
		Hash:            a.wBlock.Hash,
		LastAppHash:     a.wBlock.LastAppHash,
		ProposerAddress: a.wBlock.ProposerAddress,
		AppHash:         a.logic.StateTree().WorkingHash(),
	}

	// Add epoch secret data to the block info object and
	// update the highest known drand round so we are able
	// to determine in the future what round is superior
	if estx := a.epochSecretTx; estx != nil {
		bi.EpochSecret = estx.Secret
		bi.EpochPreviousSecret = estx.PreviousSecret
		bi.EpochRound = estx.SecretRound
		bi.InvalidEpochSecret = estx.IsInvalidated()
		if err := a.logic.SysKeeper().SetHighestDrandRound(estx.SecretRound); err != nil {
			a.commitPanic(errors.Wrap(err, "failed to save highest drand round"))
		}
	} else {
		// Ok, so no epoch secret tx in this block. We need
		// to ensure this block is not the last block of this epoch.
		// If it is, we need to TODO: Slash the proposer for not adding a secret.
		if isEndOfEpoch(bi.Height) {

		}
	}

	// Save the block information
	if err := a.logic.SysKeeper().SaveBlockInfo(bi); err != nil {
		a.commitPanic(errors.Wrap(err, "failed to save block information"))
	}

	// Index any purchased ticket we have collected so far.
	for _, ptx := range a.ticketPurchaseTxs {
		if err := a.ticketMgr.Index(ptx.Tx, uint64(a.wBlock.Height), ptx.index); err != nil {
			a.commitPanic(errors.Wrap(err, "failed to index ticket"))
		}
	}

	// Update the current validators record if the current block
	// height is the height where the last validator update will take effect.
	// Tendermint effects validator updates after 2 blocks; We need to index
	// the validators to the real height when the validators were selected (2 blocks ago)
	if a.wBlock.Height == a.heightToSaveNewValidators {
		if err := a.logic.ValidatorKeeper().
			Index(a.wBlock.Height-2, a.latestUnsavedValidators); err != nil {
			a.commitPanic(errors.Wrap(err, "failed to update current validators"))
		}
	}

	// Save the uncommitted changes to the tree
	appHash, _, err := a.logic.StateTree().SaveVersion()
	if err != nil {
		a.commitPanic(errors.Wrap(err, "failed to commit: could not save new tree version"))
	}

	// Index the un-indexed txs
	for _, t := range a.unIndexedTxs {
		if err := a.logic.TxKeeper().Index(t); err != nil {
			a.commitPanic(errors.Wrap(err, "failed to index transaction after commit"))
		}
	}

	// Commit all state changes
	if err := a.logic.Commit(); err != nil {
		panic(errors.Wrap(err, "failed to commit"))
	}

	return abcitypes.ResponseCommit{
		Data: appHash,
	}
}

// reset cached values
func (a *App) reset() {
	a.ticketPurchaseTxs = []*tickPurchaseTx{}
	a.txIndex = 0
	a.epochSecretTx = nil
	a.mature = false
	a.isCurrentBlockProposer = false
	a.unIndexedTxs = []*types.Transaction{}

	// Only reset heightToSaveNewValidators if the current height is
	// same as it to avoid not triggering saving of new validators at the target height.
	if a.wBlock.Height == a.heightToSaveNewValidators {
		a.heightToSaveNewValidators = 0
	}

	a.wBlock = &types.BlockInfo{}
}

// Query for data from the application.
func (a *App) Query(req abcitypes.RequestQuery) abcitypes.ResponseQuery {
	return abcitypes.ResponseQuery{Code: 0}
}
