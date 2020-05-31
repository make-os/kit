package modules

import (
	"fmt"

	"gitlab.com/makeos/mosdef/mempool"
	"gitlab.com/makeos/mosdef/remote/push/types"
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/types/modules"

	"github.com/c-bata/go-prompt"
	"github.com/robertkrimen/otto"
	"gitlab.com/makeos/mosdef/util"
)

// PoolModule provides access to the transaction pool
type PoolModule struct {
	vm       *otto.Otto
	reactor  *mempool.Reactor
	pushPool types.PushPooler
}

// NewPoolModule creates an instance of PoolModule
func NewPoolModule(vm *otto.Otto, reactor *mempool.Reactor, pushPool types.PushPooler) *PoolModule {
	return &PoolModule{vm: vm, reactor: reactor, pushPool: pushPool}
}

func (m *PoolModule) globals() []*modules.ModuleFunc {
	return []*modules.ModuleFunc{}
}

// funcs exposed by the module
func (m *PoolModule) funcs() []*modules.ModuleFunc {
	return []*modules.ModuleFunc{
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

// Configure configures the JS context and return
// any number of console prompt suggestions
func (m *PoolModule) Configure() []prompt.Suggest {
	var suggestions []prompt.Suggest

	// Register the main namespace
	obj := map[string]interface{}{}
	util.VMSet(m.vm, constants.NamespacePool, obj)

	for _, f := range m.funcs() {
		obj[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", constants.NamespacePool, f.Name)
		suggestions = append(suggestions, prompt.Suggest{Text: funcFullName,
			Description: f.Description})
	}

	// Register global functions
	for _, f := range m.globals() {
		m.vm.Set(f.Name, f.Value)
		suggestions = append(suggestions, prompt.Suggest{Text: f.Name,
			Description: f.Description})
	}

	return suggestions
}

// getSize returns the size of the pool
func (m *PoolModule) GetSize() util.Map {
	return EncodeForJS(m.reactor.GetPoolSize())
}

// getTop returns all the transactions in the pool
func (m *PoolModule) GetTop(n int) []util.Map {
	var res []util.Map
	for _, tx := range m.reactor.GetTop(n) {
		res = append(res, EncodeForJS(tx.ToMap()))
	}
	return res
}

// getPushPoolSize returns the size of the push pool
func (m *PoolModule) GetPushPoolSize() int {
	return m.pushPool.Len()
}
