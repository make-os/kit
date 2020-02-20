package modules

import (
	"fmt"
	"gitlab.com/makeos/mosdef/mempool"
	modtypes "gitlab.com/makeos/mosdef/modules/types"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"

	"github.com/c-bata/go-prompt"
	"github.com/robertkrimen/otto"
	"gitlab.com/makeos/mosdef/util"
)

// PoolModule provides access to the transaction pool
type PoolModule struct {
	vm       *otto.Otto
	reactor  *mempool.Reactor
	pushPool core.PushPool
}

// NewPoolModule creates an instance of PoolModule
func NewPoolModule(vm *otto.Otto, reactor *mempool.Reactor, pushPool core.PushPool) *PoolModule {
	return &PoolModule{vm: vm, reactor: reactor, pushPool: pushPool}
}

func (m *PoolModule) globals() []*modtypes.ModulesAggregatorFunc {
	return []*modtypes.ModulesAggregatorFunc{}
}

// funcs exposed by the module
func (m *PoolModule) funcs() []*modtypes.ModulesAggregatorFunc {
	return []*modtypes.ModulesAggregatorFunc{
		&modtypes.ModulesAggregatorFunc{
			Name:        "getSize",
			Value:       m.getSize,
			Description: "Get the current size of the mempool",
		},
		&modtypes.ModulesAggregatorFunc{
			Name:        "getTop",
			Value:       m.getTop,
			Description: "Get top transactions from the mempool",
		},
		&modtypes.ModulesAggregatorFunc{
			Name:        "getPushPoolSize",
			Value:       m.getPushPoolSize,
			Description: "Get the current size of the push pool",
		},
	}
}

// Configure configures the JS context and return
// any number of console prompt suggestions
func (m *PoolModule) Configure() []prompt.Suggest {
	suggestions := []prompt.Suggest{}

	// Add the main namespace
	obj := map[string]interface{}{}
	util.VMSet(m.vm, types.NamespacePool, obj)

	for _, f := range m.funcs() {
		obj[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", types.NamespacePool, f.Name)
		suggestions = append(suggestions, prompt.Suggest{Text: funcFullName,
			Description: f.Description})
	}

	// Add global functions
	for _, f := range m.globals() {
		m.vm.Set(f.Name, f.Value)
		suggestions = append(suggestions, prompt.Suggest{Text: f.Name,
			Description: f.Description})
	}

	return suggestions
}

// getSize returns the size of the pool
func (m *PoolModule) getSize() interface{} {
	return EncodeForJS(m.reactor.GetPoolSize())
}

// getTop returns all the transactions in the pool
func (m *PoolModule) getTop(n int) interface{} {
	var res = []interface{}{}
	for _, tx := range m.reactor.GetTop(n) {
		res = append(res, EncodeForJS(tx.ToMap()))
	}
	return res
}

// getPushPoolSize returns the size of the push pool
func (m *PoolModule) getPushPoolSize() interface{} {
	return m.pushPool.Len()
}
