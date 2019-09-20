package node

import (
	"encoding/hex"
	"fmt"

	"github.com/makeos/mosdef/util"

	"github.com/makeos/mosdef/util/logger"

	"github.com/makeos/mosdef/logic/keepers"

	"github.com/pkg/errors"

	"github.com/makeos/mosdef/config"

	"github.com/makeos/mosdef/node/validators"
	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/types"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

// App implements tendermint ABCI interface to
type App struct {
	db           storage.Engine
	logic        types.Logic
	cfg          *config.EngineConfig
	workingBlock *types.BlockInfo
	log          logger.Logger
}

// NewApp creates an instance of App
func NewApp(cfg *config.EngineConfig, db storage.Engine, logic types.Logic) *App {
	return &App{
		db:           db,
		logic:        logic,
		cfg:          cfg,
		workingBlock: &types.BlockInfo{},
		log:          cfg.G().Log.Module("App"),
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

	// We expect the transaction to be hex encoded,
	// now we will attempt to decode it obtain the
	// byte representation of the tx.
	bs, err := hex.DecodeString(string(req.Tx))
	if err != nil {
		return abcitypes.ResponseCheckTx{
			Code: types.ErrCodeTxBadEncode,
			Log:  "unable to decode tx bytes; expected hex encoded value",
		}
	}

	// Decode the transaction in byte form to types.Transaction
	tx, err := types.NewTxFromBytes(bs)
	if err != nil {
		return abcitypes.ResponseCheckTx{
			Code: types.ErrCodeTxBadEncode,
			Log:  "unable to decode to types.Transaction",
		}
	}

	// Perform syntactic validation
	if err = validators.ValidateTxSyntax(tx, -1); err != nil {
		return abcitypes.ResponseCheckTx{
			Code: types.ErrCodeTxFailedValidation,
			Log:  err.Error(),
		}
	}

	return abcitypes.ResponseCheckTx{Code: 0, Data: tx.Hash.Bytes()}
}

// BeginBlock indicates the beginning of a new block.
func (app *App) BeginBlock(req abcitypes.RequestBeginBlock) abcitypes.ResponseBeginBlock {
	app.workingBlock.Height = req.GetHeader().Height
	app.workingBlock.Hash = req.GetHash()
	app.workingBlock.LastAppHash = req.GetHeader().AppHash
	return abcitypes.ResponseBeginBlock{}
}

// DeliverTx processes transactions included in a proposed block.
// Execute the transaction such that in modifies the blockchain state.
func (app *App) DeliverTx(req abcitypes.RequestDeliverTx) abcitypes.ResponseDeliverTx {
	return app.logic.Tx().PrepareExec(req)
}

// EndBlock indicates the end of a block
func (app *App) EndBlock(req abcitypes.RequestEndBlock) abcitypes.ResponseEndBlock {
	return abcitypes.ResponseEndBlock{}
}

// Commit persist the application state.
// It must return a merkle root hash of the application state.
func (app *App) Commit() abcitypes.ResponseCommit {

	appHash, _, err := app.logic.StateTree().SaveVersion()
	if err != nil {
		panic(errors.Wrap(err, "failed to commit: could not save new tree version"))
	}

	// Store the committed block
	if err := app.logic.SysKeeper().SaveBlockInfo(&types.BlockInfo{
		Height:      app.workingBlock.Height,
		Hash:        app.workingBlock.Hash,
		LastAppHash: app.workingBlock.LastAppHash,
		AppHash:     appHash,
	}); err != nil {
		panic(err)
	}

	// Reset the working block
	app.workingBlock = &types.BlockInfo{}

	return abcitypes.ResponseCommit{
		Data: appHash,
	}
}

// Query for data from the application.
func (app *App) Query(req abcitypes.RequestQuery) abcitypes.ResponseQuery {
	return abcitypes.ResponseQuery{Code: 0}
}
