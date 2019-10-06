package node

import (
	"fmt"

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
	db                storage.Engine
	logic             types.Logic
	cfg               *config.EngineConfig
	validateTx        validators.ValidateTxFunc
	workingBlock      *types.BlockInfo
	log               logger.Logger
	txIndex           int
	ticketPurchaseTxs []*tickPurchaseTx
	ticketMgr         types.TicketManager
	epochSecretTx     *types.Transaction
}

// NewApp creates an instance of App
func NewApp(
	cfg *config.EngineConfig,
	db storage.Engine,
	logic types.Logic,
	ticketMgr types.TicketManager) *App {
	return &App{
		db:           db,
		logic:        logic,
		cfg:          cfg,
		workingBlock: &types.BlockInfo{},
		log:          cfg.G().Log.Module("App"),
		ticketMgr:    ticketMgr,
		validateTx:   validators.ValidateTx,
	}
}

// InitChain is called once upon genesis.
func (app *App) InitChain(req abcitypes.RequestInitChain) abcitypes.ResponseInitChain {

	app.log.Info("Initializing chain...")

	// State must be empty
	if app.logic.StateTree().WorkingHash() != nil {
		panic(fmt.Errorf("At init, state must be empty...It is not empty"))
	}

	// Write genesis state (e.g root accounts)
	if err := app.logic.WriteGenesisState(); err != nil {
		panic(errors.Wrap(err, "failed to write genesis state"))
	}

	workingHash := app.logic.StateTree().WorkingHash()
	app.log.Info("Chain initialization was successful",
		"GenesisHash", util.BytesToHash(workingHash).HexStr())

	return abcitypes.ResponseInitChain{}
}

// Info returns information about the application state.
// Used to sync tendermint with the application during a handshake that happens on startup.
// The returned AppVersion will be included in the header of every block.
// Tendermint expects LastBlockAppHash and LastBlockHeight to be updated during commit.
func (app *App) Info(req abcitypes.RequestInfo) abcitypes.ResponseInfo {

	var lastBlockAppHash = []byte{}
	var lastBlockHeight = int64(0)

	// Get the last committed block information
	lastBlock, err := app.logic.SysKeeper().GetLastBlockInfo()
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
		Version:          app.cfg.VersionInfo.BuildVersion,
		AppVersion:       config.GetNetVersion(),
		LastBlockHeight:  lastBlockHeight,
		LastBlockAppHash: lastBlockAppHash,
	}
}

// SetOption set non-consensus critical application specific options.
func (app *App) SetOption(req abcitypes.RequestSetOption) abcitypes.ResponseSetOption {
	return abcitypes.ResponseSetOption{}
}

// CheckTx a proposed transaction for admission into the mempool.
// A non-zero response means the transaction is rejected and will not
// be broadcast to other nodes.
func (app *App) CheckTx(req abcitypes.RequestCheckTx) abcitypes.ResponseCheckTx {

	// Decode the transaction in byte form to types.Transaction
	tx, err := types.NewTxFromBytes(req.Tx)
	if err != nil {
		return abcitypes.ResponseCheckTx{
			Code: types.ErrCodeTxBadEncode,
			Log:  "unable to decode to types.Transaction",
		}
	}

	// Perform syntactic validation
	if err = app.validateTx(tx, -1, app.logic); err != nil {
		return abcitypes.ResponseCheckTx{
			Code: types.ErrCodeTxFailedValidation,
			Log:  err.Error(),
		}
	}

	return abcitypes.ResponseCheckTx{Code: 0, Data: tx.GetHash().Bytes()}
}

// BeginBlock indicates the beginning of a new block.
func (app *App) BeginBlock(req abcitypes.RequestBeginBlock) abcitypes.ResponseBeginBlock {

	curHeight := req.GetHeader().Height
	app.workingBlock.Height = req.GetHeader().Height
	app.workingBlock.Hash = req.GetHash()
	app.workingBlock.LastAppHash = req.GetHeader().AppHash
	app.workingBlock.ProposerAddress = common.HexBytes(req.GetHeader().ProposerAddress).String()

	// If the network is still immature, return immediately
	if err := app.logic.Sys().CheckSetNetMaturity(); err != nil {
		app.log.Debug("Network is currently immature", "Err",
			err, "CurHeight", curHeight)
		return abcitypes.ResponseBeginBlock{}
	}

	// Determine current epoch
	curEpoch, nextEpoch := app.logic.Sys().GetEpoch(uint64(curHeight))
	app.log.Info("Epoch is known", "Epoch", curEpoch, "Next", nextEpoch)

	return abcitypes.ResponseBeginBlock{}
}

// preExecChecks performs some checks that attempt to spot problems
// with specific transaction types and possible activate slashing
// conditions and invalidate the transaction before it ever gets
// executed.
func (app *App) preExecChecks(tx *types.Transaction) *abcitypes.ResponseDeliverTx {

	txType := tx.GetType()

	// Invalidate the transaction if it is a validator ticket
	// purchasing tx and we have reached the max per block.
	// TODO: Slash proposer for violating the rule.
	if txType == types.TxTypeTicketValidator &&
		len(app.ticketPurchaseTxs) == params.MaxValTicketsPerBlock {
		return &abcitypes.ResponseDeliverTx{
			Code: types.ErrCodeMaxTxTypeReached,
			Log:  "failed to execute tx: validator ticket capacity reached",
		}
	}

	if txType == types.TxTypeEpochSecret {
		// Invalidate the epoch secret tx if the current block is not the
		// last block in the current epoch.
		// TODO: Slash the proposer for violating this rule.
		if (app.workingBlock.Height)%int64(params.NumBlocksPerEpoch) != 0 {
			return &abcitypes.ResponseDeliverTx{
				Code: types.ErrCodeTxTypeUnexpected,
				Log:  "failed to execute tx: epoch secret not expected",
			}
		}

		// Invalidate the epoch secret tx if we have already seen one in this block.
		// TODO: Slash proposer for violating this rule.
		if app.epochSecretTx != nil {
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
func (app *App) postExecChecks(
	tx *types.Transaction,
	resp abcitypes.ResponseDeliverTx) *abcitypes.ResponseDeliverTx {

	txType := tx.GetType()

	if txType == types.TxTypeEpochSecret {

		// Cache the epoch secret tx for use in the COMMIT stage
		app.epochSecretTx = tx

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
	if resp.Code == 0 && txType == types.TxTypeTicketValidator {
		app.ticketPurchaseTxs = append(app.ticketPurchaseTxs, &tickPurchaseTx{
			Tx:    tx,
			index: app.txIndex,
		})
	}

	return &resp
}

// DeliverTx processes transactions included in a proposed block.
// Execute the transaction such that in modifies the blockchain state.
func (app *App) DeliverTx(req abcitypes.RequestDeliverTx) abcitypes.ResponseDeliverTx {

	// Increment the tx index
	app.txIndex++

	// Decode transaction to types.Transaction
	tx, err := types.NewTxFromBytes(req.Tx)
	if err != nil {
		return abcitypes.ResponseDeliverTx{
			Code: types.ErrCodeTxBadEncode,
			Log:  "unable to decode to types.Transaction",
		}
	}

	// Perform pre execution checks
	if resp := app.preExecChecks(tx); resp != nil {
		return *resp
	}

	// Execute the transaction (does not commit the state changes yet)
	resp := app.logic.Tx().PrepareExec(req)

	// Perform post execution operations
	return *app.postExecChecks(tx, resp)
}

// EndBlock indicates the end of a block
func (app *App) EndBlock(req abcitypes.RequestEndBlock) abcitypes.ResponseEndBlock {
	return abcitypes.ResponseEndBlock{}
}

// Commit persist the application state.
// It must return a merkle root hash of the application state.
func (app *App) Commit() abcitypes.ResponseCommit {
	defer app.reset()

	// Construct a new block information object
	bi := &types.BlockInfo{
		Height:      app.workingBlock.Height,
		Hash:        app.workingBlock.Hash,
		LastAppHash: app.workingBlock.LastAppHash,
	}

	// Add epoch secret data to the block info object and
	// update the highest known drand round so we are able
	// to determine in the future what round is superior
	if estx := app.epochSecretTx; estx != nil {
		bi.EpochSecret = estx.Secret
		bi.EpochPreviousSecret = estx.PreviousSecret
		bi.EpochRound = estx.SecretRound
		bi.InvalidEpochSecret = estx.IsInvalidated()
		if err := app.logic.SysKeeper().SetHighestDrandRound(estx.SecretRound); err != nil {
			panic(errors.Wrap(err, "failed to save highest drand round"))
		}
	} else {
		// Ok, so no epoch secret tx in this block. We need
		// to ensure this block is not the last block of this epoch.
		// If it is, we need to TODO: Slash the proposer for not adding a secret.
		if bi.Height%int64(params.NumBlocksPerEpoch) == 0 {

		}
	}

	// Save the uncommitted changes to the tree
	appHash, _, err := app.logic.StateTree().SaveVersion()
	if err != nil {
		panic(errors.Wrap(err, "failed to commit: could not save new tree version"))
	}

	// Save the block information
	bi.AppHash = appHash
	if err := app.logic.SysKeeper().SaveBlockInfo(bi); err != nil {
		panic(err)
	}

	// Index any purchased ticket we have collected so far.
	for _, ptx := range app.ticketPurchaseTxs {
		app.ticketMgr.Index(ptx.Tx, ptx.Tx.SenderPubKey.String(),
			uint64(app.workingBlock.Height), ptx.index)
	}

	return abcitypes.ResponseCommit{
		Data: appHash,
	}
}

// reset cached values
func (app *App) reset() {
	app.workingBlock = &types.BlockInfo{}
	app.ticketPurchaseTxs = []*tickPurchaseTx{}
	app.txIndex = 0
	app.epochSecretTx = nil
}

// Query for data from the application.
func (app *App) Query(req abcitypes.RequestQuery) abcitypes.ResponseQuery {
	return abcitypes.ResponseQuery{Code: 0}
}
