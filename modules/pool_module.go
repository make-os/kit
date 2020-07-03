package modules

import (
	"fmt"

	"github.com/c-bata/go-prompt"
	"github.com/robertkrimen/otto"
	"gitlab.com/makeos/mosdef/console"
	"gitlab.com/makeos/mosdef/mempool"
	modulestypes "gitlab.com/makeos/mosdef/modules/types"
	"gitlab.com/makeos/mosdef/remote/push/types"
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/util"
)

// PoolModule provides access to the transaction pool
type PoolModule struct {
	console.ConsoleSuggestions
	reactor  *mempool.Reactor
	pushPool types.PushPool
}

// NewPoolModule creates an instance of PoolModule
func NewPoolModule(reactor *mempool.Reactor, pushPool types.PushPool) *PoolModule {
	return &PoolModule{reactor: reactor, pushPool: pushPool}
}

// ConsoleOnlyMode indicates that this module can be used on console-only mode
func (m *PoolModule) ConsoleOnlyMode() bool {
	return false
}

// globals are functions exposed in the VM's global namespace
func (m *PoolModule) globals() []*modulestypes.ModuleFunc {
	return []*modulestypes.ModuleFunc{}
}

// methods are functions exposed in the special namespace of this module.
func (m *PoolModule) methods() []*modulestypes.ModuleFunc {
	return []*modulestypes.ModuleFunc{
		{
			Name:        "getSize",
			Value:       m.GetSize,
			Description: "Get the current size of the mempool",
		},
		{
			Name:        "getTop",
			Value:       m.GetTop,
			Description: "Get top transactions from the mempool",
		},
		{
			Name:        "getPushPoolSize",
			Value:       m.GetPushPoolSize,
			Description: "Get the current size of the push pool",
		},
	}
}

// ConfigureVM configures the JS context and return
// any number of console prompt suggestions
func (m *PoolModule) ConfigureVM(vm *otto.Otto) prompt.Completer {

	// Register the main namespace
	obj := map[string]interface{}{}
	util.VMSet(vm, constants.NamespacePool, obj)

	for _, f := range m.methods() {
		obj[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", constants.NamespacePool, f.Name)
		m.Suggestions = append(m.Suggestions, prompt.Suggest{Text: funcFullName,
			Description: f.Description})
	}

	// Register global functions
	for _, f := range m.globals() {
		vm.Set(f.Name, f.Value)
		m.Suggestions = append(m.Suggestions, prompt.Suggest{Text: f.Name,
			Description: f.Description})
	}

	return m.Completer
}

// getSize returns the size of the pool
func (m *PoolModule) GetSize() util.Map {
	return util.StructToMap(m.reactor.GetPoolSize())
}

// getTop returns all the transactions in the pool
func (m *PoolModule) GetTop(n int) []util.Map {
	var res = []util.Map{}
	for _, tx := range m.reactor.GetTop(n) {
		res = append(res, tx.ToMap())
	}
	return res
}

// getPushPoolSize returns the size of the push pool
func (m *PoolModule) GetPushPoolSize() int {
	return m.pushPool.Len()
}
