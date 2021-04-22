package modules

import (
	"fmt"

	"github.com/c-bata/go-prompt"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/miner"
	modulestypes "github.com/make-os/kit/modules/types"
	types2 "github.com/make-os/kit/rpc/types"
	"github.com/make-os/kit/types/constants"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/errors"
	"github.com/robertkrimen/otto"
)

// MinerModule provides access to the Miner
type MinerModule struct {
	modulestypes.ModuleCommon
	cfg   *config.AppConfig
	miner miner.Miner
}

// NewAttachableMinerModule creates an instance of MinerModule suitable in attach mode
func NewAttachableMinerModule(cfg *config.AppConfig, client types2.Client) *MinerModule {
	return &MinerModule{ModuleCommon: modulestypes.ModuleCommon{Client: client}, cfg: cfg}
}

// NewMinerModule creates an instance of MinerModule
func NewMinerModule(cfg *config.AppConfig, miner miner.Miner) *MinerModule {
	return &MinerModule{cfg: cfg, miner: miner}
}

// methods are functions exposed in the special namespace of this module.
func (m *MinerModule) methods() []*modulestypes.VMMember {
	return []*modulestypes.VMMember{
		{Name: "start", Value: m.Start, Description: "Start the miner"},
		{Name: "stop", Value: m.Stop, Description: "Stop the miner"},
		{Name: "isRunning", Value: m.IsRunning, Description: "Checks whether the miner is running"},
		{Name: "getHashrate", Value: m.GetHashrate, Description: "Get the hashrate of the miner"},
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

func (m *MinerModule) SubmitWork(epoch int64, nonce uint64) error {
	return nil
}
