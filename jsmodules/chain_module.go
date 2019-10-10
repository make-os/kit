package jsmodules

import (
	"fmt"
	"strconv"

	"github.com/tendermint/tendermint/crypto/ed25519"

	"github.com/makeos/mosdef/crypto"

	"github.com/makeos/mosdef/util"

	"github.com/pkg/errors"

	"github.com/c-bata/go-prompt"
	"github.com/makeos/mosdef/types"
	"github.com/robertkrimen/otto"
)

// ChainModule provides access to chain information
type ChainModule struct {
	vm      *otto.Otto
	service types.Service
	logic   types.Logic
}

// NewChainModule creates an instance of ChainModule
func NewChainModule(vm *otto.Otto, service types.Service, logic types.Logic) *ChainModule {
	return &ChainModule{vm: vm, service: service, logic: logic}
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
			Name:        "getBlockInfo",
			Value:       m.getBlockInfo,
			Description: "Get summary block information of a given height",
		},
		&types.JSModuleFunc{
			Name:        "getValidators",
			Value:       m.getValidators,
			Description: "Get validators at a given height",
		},
	}
}

// Configure configures the JS context and return
// any number of console prompt suggestions
func (m *ChainModule) Configure() []prompt.Suggest {
	suggestions := []prompt.Suggest{}

	// Add the main namespace
	obj := map[string]interface{}{}
	util.VMSet(m.vm, types.NamespaceChain, obj)

	for _, f := range m.funcs() {
		obj[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", types.NamespaceChain, f.Name)
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

	res, err := m.service.GetBlock(blockHeight)
	if err != nil {
		panic(errors.Wrap(err, "failed to get block"))
	}

	return res
}

// getCurrentHeight returns the current block height
func (m *ChainModule) getCurrentHeight() interface{} {
	res, err := m.service.GetCurrentHeight()
	if err != nil {
		panic(errors.Wrap(err, "failed to get current block height"))
	}
	return util.EncodeForJS(map[string]interface{}{
		"height": fmt.Sprintf("%d", res),
	})
}

// getBlockInfo get summary block information of a given height
func (m *ChainModule) getBlockInfo(height interface{}) interface{} {

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

	res, err := m.logic.SysKeeper().GetBlockInfo(blockHeight)
	if err != nil {
		panic(errors.Wrap(err, "failed to get block info"))
	}

	return res
}

// getValidators returns the current validators
func (m *ChainModule) getValidators(height interface{}) interface{} {

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

	validators, err := m.logic.ValidatorKeeper().GetByHeight(blockHeight)
	if err != nil {
		panic(err)
	}

	var vList = []map[string]interface{}{}
	for hexPubKey, power := range validators {

		hexBz := types.HexBytesFromHex(hexPubKey)
		var pub32 ed25519.PubKeyEd25519
		copy(pub32[:], hexBz)

		pubKey, _ := crypto.PubKeyFromBytes(hexBz)
		vList = append(vList, map[string]interface{}{
			"power":     power,
			"publicKey": pubKey.Base58(),
			"address":   pubKey.Addr(),
			"tmAddress": pub32.Address().String(),
		})
	}

	return vList
}
