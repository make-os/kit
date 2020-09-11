package modules

import (
	"fmt"

	"github.com/c-bata/go-prompt"
	"github.com/make-os/lobe/api/rpc/client"
	modulestypes "github.com/make-os/lobe/modules/types"
	"github.com/make-os/lobe/remote/push/types"
	"github.com/make-os/lobe/types/constants"
	"github.com/make-os/lobe/types/core"
	"github.com/make-os/lobe/util"
	"github.com/robertkrimen/otto"
)

// PoolModule provides access to the transaction pool
type PoolModule struct {
	modulestypes.ModuleCommon
	mempoolReactor core.MempoolReactor
	pushPool       types.PushPool
}

// NewAttachablePoolModule creates an instance of PoolModule suitable in attach mode
func NewAttachablePoolModule(client client.Client) *PoolModule {
	return &PoolModule{ModuleCommon: modulestypes.ModuleCommon{AttachedClient: client}}
}

// NewPoolModule creates an instance of PoolModule
func NewPoolModule(reactor core.MempoolReactor, pushPool types.PushPool) *PoolModule {
	return &PoolModule{mempoolReactor: reactor, pushPool: pushPool}
}

// globals are functions exposed in the VM's global namespace
func (m *PoolModule) globals() []*modulestypes.VMMember {
	return []*modulestypes.VMMember{}
}

// methods are functions exposed in the special namespace of this module.
func (m *PoolModule) methods() []*modulestypes.VMMember {
	return []*modulestypes.VMMember{
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
		m.Suggestions = append(m.Suggestions, prompt.Suggest{Text: funcFullName, Description: f.Description})
	}

	// Register global functions
	for _, f := range m.globals() {
		vm.Set(f.Name, f.Value)
		m.Suggestions = append(m.Suggestions, prompt.Suggest{Text: f.Name, Description: f.Description})
	}

	return m.Completer
}

// getSize returns the size of the pool
func (m *PoolModule) GetSize() util.Map {
	return util.ToMap(m.mempoolReactor.GetPoolSize())
}

// getTop returns all the transactions in the pool
func (m *PoolModule) GetTop(n int) []util.Map {
	var res = []util.Map{}
	for _, tx := range m.mempoolReactor.GetTop(n) {
		res = append(res, tx.ToMap())
	}
	return res
}

// getPushPoolSize returns the size of the push pool
func (m *PoolModule) GetPushPoolSize() int {
	return m.pushPool.Len()
}
