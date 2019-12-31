package jsmodules

import (
	"fmt"

	"github.com/makeos/mosdef/mempool"

	"github.com/c-bata/go-prompt"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
	"github.com/robertkrimen/otto"
)

// PoolModule provides access to the transaction pool
type PoolModule struct {
	vm       *otto.Otto
	reactor  *mempool.Reactor
	pushPool types.PushPool
}

// NewPoolModule creates an instance of PoolModule
func NewPoolModule(vm *otto.Otto, reactor *mempool.Reactor, pushPool types.PushPool) *PoolModule {
	return &PoolModule{vm: vm, reactor: reactor, pushPool: pushPool}
}

func (m *PoolModule) globals() []*types.JSModuleFunc {
	return []*types.JSModuleFunc{}
}

// funcs exposed by the module
func (m *PoolModule) funcs() []*types.JSModuleFunc {
	return []*types.JSModuleFunc{
		&types.JSModuleFunc{
			Name:        "getSize",
			Value:       m.getSize,
			Description: "Get the current size of the mempool",
		},
		&types.JSModuleFunc{
			Name:        "getTop",
			Value:       m.getTop,
			Description: "Get top transactions from the mempool",
		},
		&types.JSModuleFunc{
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
	return util.EncodeForJS(m.reactor.GetPoolSize())
}

// getTop returns all the transactions in the pool
func (m *PoolModule) getTop(n int) interface{} {
	var res = []interface{}{}
	for _, tx := range m.reactor.GetTop(n) {
		res = append(res, util.EncodeForJS(tx.ToMap()))
	}
	return res
}

// getPushPoolSize returns the size of the push pool
func (m *PoolModule) getPushPoolSize() interface{} {
	return m.pushPool.Len()
}
