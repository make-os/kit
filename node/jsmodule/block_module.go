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

// BlockModule provides access to block information
type BlockModule struct {
	vm          *otto.Otto
	nodeService types.Service
}

// NewBlockModule creates an instance of BlockModule
func NewBlockModule(vm *otto.Otto, nodeService types.Service) *BlockModule {
	return &BlockModule{vm: vm, nodeService: nodeService}
}

func (m *BlockModule) globals() []*types.JSModuleFunc {
	return []*types.JSModuleFunc{}
}

// funcs are functions accessible using the tx.coin namespace
func (m *BlockModule) funcs() []*types.JSModuleFunc {
	return []*types.JSModuleFunc{
		&types.JSModuleFunc{
			Name:        "getBlock",
			Value:       m.getBlock,
			Description: "Send the native coin from an account to a destination account.",
		},
		&types.JSModuleFunc{
			Name:        "getCurrentHeight",
			Value:       m.getCurrentHeight,
			Description: "Gets the current block height",
		},
	}
}

// Configure configures the JS context and return
// any number of console prompt suggestions
func (m *BlockModule) Configure() []prompt.Suggest {
	suggestions := []prompt.Suggest{}

	obj := map[string]interface{}{}
	for _, f := range m.funcs() {
		obj[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", jsBlockModuleName, f.Name)
		suggestions = append(suggestions, prompt.Suggest{Text: funcFullName,
			Description: f.Description})
	}

	// Add the main tx namespace
	m.vm.Set(jsBlockModuleName, obj)

	// Add global functions
	for _, f := range m.globals() {
		m.vm.Set(f.Name, f.Value)
	}

	return suggestions
}

// getBlock fetches a block at the given height
func (m *BlockModule) getBlock(height interface{}) interface{} {

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
func (m *BlockModule) getCurrentHeight() interface{} {
	res, err := m.nodeService.GetCurrentHeight()
	if err != nil {
		panic(errors.Wrap(err, "failed to get current block height"))
	}
	return util.EncodeForJS(map[string]interface{}{
		"height": fmt.Sprintf("%d", res),
	})
}
