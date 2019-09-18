package node

import (
	"encoding/hex"

	"github.com/makeos/mosdef/node/validators"
	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/types"
	abcitypes "github.com/tendermint/tendermint/abci/types"
)

// App implements tendermint ABCI interface to
type App struct {
	db    storage.Engine
	logic types.Logic
}

// NewApp creates an instance of App
func NewApp(db storage.Engine, logic types.Logic) *App {
	return &App{db: db, logic: logic}
}

// InitChain is called once upon genesis.
func (app *App) InitChain(req abcitypes.RequestInitChain) abcitypes.ResponseInitChain {
	return abcitypes.ResponseInitChain{}
}

// Info returns information about the application state.
// Used to sync tendermine with the application during a handshake that happens on startup.
// The returned AppVersion will be included in the header of every block.
// Tendermint expects LastBlockAppHash and LastBlockHeight to be updated during commit.
func (app *App) Info(req abcitypes.RequestInfo) abcitypes.ResponseInfo {
	return abcitypes.ResponseInfo{}
}

// SetOption set non-consensus critical application specific options.
func (app *App) SetOption(req abcitypes.RequestSetOption) abcitypes.ResponseSetOption {
	return abcitypes.ResponseSetOption{}
}

// DeliverTx processes transactions included in a proposed block.
// Execute the transaction such that in modifies the blockchain state.
func (app *App) DeliverTx(req abcitypes.RequestDeliverTx) abcitypes.ResponseDeliverTx {
	return app.logic.Tx().PrepareExec(req)
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

// Commit persist the application state.
// It must return a merkle root hash of the application state.
func (app *App) Commit() abcitypes.ResponseCommit {
	return abcitypes.ResponseCommit{}
}

// Query for data from the application.
func (app *App) Query(req abcitypes.RequestQuery) abcitypes.ResponseQuery {
	return abcitypes.ResponseQuery{Code: 0}
}

// BeginBlock indicates the beginning of a new block.
func (app *App) BeginBlock(req abcitypes.RequestBeginBlock) abcitypes.ResponseBeginBlock {
	return abcitypes.ResponseBeginBlock{}
}

// EndBlock indicates the end of a block
func (app *App) EndBlock(req abcitypes.RequestEndBlock) abcitypes.ResponseEndBlock {
	return abcitypes.ResponseEndBlock{}
}
