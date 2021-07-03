package modules

import (
	"fmt"

	"github.com/c-bata/go-prompt"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/miner"
	modulestypes "github.com/make-os/kit/modules/types"
	types2 "github.com/make-os/kit/rpc/types"
	"github.com/make-os/kit/types/constants"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/errors"
	"github.com/robertkrimen/otto"
)

// MinerModule provides access to the Miner
type MinerModule struct {
	modulestypes.ModuleCommon
	cfg   *config.AppConfig
	logic core.Logic
	miner miner.Miner
}

// NewAttachableMinerModule creates an instance of MinerModule suitable in attach mode
func NewAttachableMinerModule(cfg *config.AppConfig, client types2.Client, logic core.Logic) *MinerModule {
	return &MinerModule{ModuleCommon: modulestypes.ModuleCommon{Client: client}, cfg: cfg, logic: logic}
}

// NewMinerModule creates an instance of MinerModule
func NewMinerModule(cfg *config.AppConfig, logic core.Logic, miner miner.Miner) *MinerModule {
	return &MinerModule{cfg: cfg, miner: miner, logic: logic}
}

// methods are functions exposed in the special namespace of this module.
func (m *MinerModule) methods() []*modulestypes.VMMember {
	return []*modulestypes.VMMember{
		{Name: "start", Value: m.Start, Description: "Start the miner"},
		{Name: "stop", Value: m.Stop, Description: "Stop the miner"},
		{Name: "isRunning", Value: m.IsRunning, Description: "Checks whether the miner is running"},
		{Name: "getHashrate", Value: m.GetHashrate, Description: "Get the hashrate of the miner"},
		{Name: "submitWork", Value: m.SubmitWork, Description: "Submit a proof of work nonce to the network"},
		{Name: "getPrevWork", Value: m.GetPreviousWork, Description: "Get the previous nonces found by the node"},
	}
}

// globals are functions exposed in the VM's global namespace
func (m *MinerModule) globals() []*modulestypes.VMMember {
	return []*modulestypes.VMMember{}
}

// ConfigureVM configures the JS context and return
// any number of console prompt suggestions
func (m *MinerModule) ConfigureVM(vm *otto.Otto) prompt.Completer {

	// Set the namespace object
	nsMap := map[string]interface{}{}
	util.VMSet(vm, constants.NamespaceMiner, nsMap)

	// add methods functions
	for _, f := range m.methods() {
		nsMap[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", constants.NamespaceMiner, f.Name)
		m.Suggestions = append(m.Suggestions, prompt.Suggest{Text: funcFullName, Description: f.Description})
	}

	// Register global functions
	for _, f := range m.globals() {
		vm.Set(f.Name, f.Value)
		m.Suggestions = append(m.Suggestions, prompt.Suggest{Text: f.Name, Description: f.Description})
	}

	return m.Completer
}

// Start starts the miner
func (m *MinerModule) Start(scheduleStart ...bool) error {
	var ss bool
	if len(scheduleStart) > 0 {
		ss = scheduleStart[0]
	}
	err := m.miner.Start(ss)
	if err != nil {
		panic(errors.ReqErr(500, StatusCodeServerErr, "", err.Error()))
	}
	return nil
}

// Stop stops the miner
func (m *MinerModule) Stop() {
	m.miner.Stop()
}

// IsRunning checks whether the miner is running
func (m *MinerModule) IsRunning() bool {
	return m.miner.IsMining()
}

// GetHashrate returns the hashrate
func (m *MinerModule) GetHashrate() float64 {
	return m.miner.GetHashrate()
}

// SubmitWork submits a proof of work nonce
//
// params <map>
//  - epoch <number|string>: The epoch in which the work was done
//  - wnonce <number|string>: The nonce discovered during the work
//  - nonce <number|string>: The senders next account nonce
//  - fee <number|string>: The transaction fee to pay (optional)
//  - timestamp <number>: The unix timestamp
//  - sig <String>: The transaction signature
//
// options <[]interface{}>
//  - [0] key <string>: The signer's private key
//  - [1] payloadOnly <bool>: When true, returns the payload only, without sending the tx.
//
// RETURN object <map>
//  - hash <string>: The transaction hash
//  - address <string: The address of the repository
func (m *MinerModule) SubmitWork(params map[string]interface{}, options ...interface{}) util.Map {

	var tx = txns.NewBareTxSubmitWork()
	if err := tx.FromMap(params); err != nil {
		panic(se(400, StatusCodeInvalidParam, "params", err.Error()))
	}

	if tx.Fee.Empty() {
		tx.Fee = "0"
	}

	retPayload, _ := finalizeTx(tx, m.logic, m.Client, options...)
	if retPayload {
		return tx.ToMap()
	}

	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		panic(se(400, StatusCodeMempoolAddFail, "", err.Error()))
	}

	return map[string]interface{}{
		"hash": hash,
	}
}

// GetPreviousWork returns the previous nonces found by the node
func (m *MinerModule) GetPreviousWork() []util.Map {
	res, err := m.logic.SysKeeper().GetNodeWorks()
	if err != nil {
		panic(se(400, StatusCodeServerErr, "", err.Error()))
	}
	return util.StructSliceToMap(res)
}
