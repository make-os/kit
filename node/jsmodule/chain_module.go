package jsmodule

import (
	"fmt"
	"strconv"

	"github.com/makeos/mosdef/util"

	"github.com/pkg/errors"

	"github.com/c-bata/go-prompt"
	"github.com/makeos/mosdef/types"
	"github.com/robertkrimen/otto"
)

const jsBlockModuleName = "chain"

// ChainModule provides access to chain information
type ChainModule struct {
	vm          *otto.Otto
	nodeService types.Service
	logic       types.Logic
}

// NewChainModule creates an instance of ChainModule
func NewChainModule(vm *otto.Otto, nodeService types.Service, logic types.Logic) *ChainModule {
	return &ChainModule{vm: vm, nodeService: nodeService, logic: logic}
}

func (m *ChainModule) globals() []*types.JSModuleFunc {
	return []*types.JSModuleFunc{}
}

// funcs exposed by the module
func (m *ChainModule) funcs() []*types.JSModuleFunc {
	return []*types.JSModuleFunc{
		&types.JSModuleFunc{
			Name:        "getBlock",
			Value:       m.getBlock,
			Description: "Send the native coin from an account to a destination account",
		},
		&types.JSModuleFunc{
			Name:        "getCurrentHeight",
			Value:       m.getCurrentHeight,
			Description: "Get the current block height",
		},
		&types.JSModuleFunc{
			Name:        "getAccount",
			Value:       m.getAccount,
			Description: "Get the account of a given address",
		},
		&types.JSModuleFunc{
			Name:        "getBalance",
			Value:       m.getBalance,
			Description: "Get the coin balance of an account",
		},
	}
}

// Configure configures the JS context and return
// any number of console prompt suggestions
func (m *ChainModule) Configure() []prompt.Suggest {
	suggestions := []prompt.Suggest{}

	// Add the main tx namespace
	obj := map[string]interface{}{}
	util.VMSet(m.vm, jsBlockModuleName, obj)

	for _, f := range m.funcs() {
		obj[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", jsBlockModuleName, f.Name)
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

// getBlock fetches a block at the given height
func (m *ChainModule) getBlock(height interface{}) interface{} {

	var err error
	var blockHeight int64

	// Convert to the expected type (int64)
	switch v := height.(type) {
	case int64:
		blockHeight = int64(v)
	case string:
		blockHeight, err = strconv.ParseInt(v, 10, 64)
		if err != nil {
			panic(types.ErrArgDecode("Int64", 0))
		}
	default:
		panic(types.ErrArgDecode("integer/string", 0))
	}

	res, err := m.nodeService.GetBlock(blockHeight)
	if err != nil {
		panic(errors.Wrap(err, "failed to get block"))
	}

	return res
}

// getCurrentHeight returns the current block height
func (m *ChainModule) getCurrentHeight() interface{} {
	res, err := m.nodeService.GetCurrentHeight()
	if err != nil {
		panic(errors.Wrap(err, "failed to get current block height"))
	}
	return util.EncodeForJS(map[string]interface{}{
		"height": fmt.Sprintf("%d", res),
	})
}

// getAccount returns the account of the given address
func (m *ChainModule) getAccount(address string, height ...int64) interface{} {
	account := m.logic.AccountKeeper().GetAccount(util.String(address), height...)
	if account.Balance.String() == "0" && account.Nonce == int64(0) {
		return nil
	}
	return util.EncodeForJS(account)
}

// getBalance returns the balance of an account
func (m *ChainModule) getBalance(address string, height ...int64) interface{} {
	account := m.logic.AccountKeeper().GetAccount(util.String(address), height...)
	if account.Balance.String() == "0" && account.Nonce == int64(0) {
		return nil
	}
	return account.Balance.String()
}
