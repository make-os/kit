package modules

import (
	"fmt"
	"strconv"

	modtypes "gitlab.com/makeos/mosdef/modules/types"
	"gitlab.com/makeos/mosdef/node/services"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"

	"github.com/tendermint/tendermint/crypto/ed25519"

	"gitlab.com/makeos/mosdef/crypto"

	"gitlab.com/makeos/mosdef/util"

	"github.com/pkg/errors"

	"github.com/c-bata/go-prompt"
	"github.com/robertkrimen/otto"
)

// ChainModule provides access to chain information
type ChainModule struct {
	vm      *otto.Otto
	service services.Service
	keepers core.Keepers
}

// NewChainModule creates an instance of ChainModule
func NewChainModule(vm *otto.Otto, service services.Service, keepers core.Keepers) *ChainModule {
	return &ChainModule{vm: vm, service: service, keepers: keepers}
}

func (m *ChainModule) globals() []*modtypes.ModulesAggregatorFunc {
	return []*modtypes.ModulesAggregatorFunc{}
}

// funcs exposed by the module
func (m *ChainModule) funcs() []*modtypes.ModulesAggregatorFunc {
	return []*modtypes.ModulesAggregatorFunc{
		{
			Name:        "getBlock",
			Value:       m.getBlock,
			Description: "Send the native coin from an account to a destination account",
		},
		{
			Name:        "getCurrentHeight",
			Value:       m.getCurrentHeight,
			Description: "Get the current block height",
		},
		{
			Name:        "getBlockInfo",
			Value:       m.getBlockInfo,
			Description: "Get summary block information of a given height",
		},
		{
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
	util.VMSet(m.vm, types.NamespaceNode, obj)

	for _, f := range m.funcs() {
		obj[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", types.NamespaceNode, f.Name)
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
func (m *ChainModule) getBlock(height string) util.Map {

	var err error
	var blockHeight int64

	blockHeight, err = strconv.ParseInt(height, 10, 64)
	if err != nil {
		panic(types.ErrArgDecode("Int64", 0))
	}

	res, err := m.service.GetBlock(blockHeight)
	if err != nil {
		panic(errors.Wrap(err, "failed to get block"))
	}

	return EncodeForJS(res)
}

// getCurrentHeight returns the current block height
func (m *ChainModule) getCurrentHeight() interface{} {
	bi, err := m.keepers.SysKeeper().GetLastBlockInfo()
	if err != nil {
		panic(errors.Wrap(err, "failed to get current block height"))
	}
	return EncodeForJS(map[string]interface{}{
		"height": fmt.Sprintf("%d", bi.Height),
	})
}

// getBlockInfo get summary block information of a given height
func (m *ChainModule) getBlockInfo(height string) util.Map {

	var err error
	var blockHeight int64

	blockHeight, err = strconv.ParseInt(height, 10, 64)
	if err != nil {
		panic(types.ErrArgDecode("Int64", 0))
	}

	res, err := m.keepers.SysKeeper().GetBlockInfo(blockHeight)
	if err != nil {
		panic(errors.Wrap(err, "failed to get block info"))
	}

	return EncodeForJS(res)
}

// getValidators returns validators of a given block
//
// ARGS:
// height: The target block height
//
// RETURNS res []Map
// res.publicKey <string>: 	The base58 public key of validator
// res.address <string>: 	The bech32 address of the validator
// res.tmAddress <string>: 	The tendermint address and the validator
// res.ticketId <string>: 	The id of the validator ticket
func (m *ChainModule) getValidators(height string) (res []util.Map) {

	var err error
	var blockHeight int64

	blockHeight, err = strconv.ParseInt(height, 10, 64)
	if err != nil {
		panic(types.ErrArgDecode("Int64", 0))
	}

	validators, err := m.keepers.ValidatorKeeper().GetByHeight(blockHeight)
	if err != nil {
		panic(err)
	}

	var vList = []util.Map{}
	for pubKey, valInfo := range validators {

		var pub32 ed25519.PubKeyEd25519
		copy(pub32[:], pubKey.Bytes())

		pubKey := crypto.MustPubKeyFromBytes(pubKey.Bytes())
		vList = append(vList, map[string]interface{}{
			"publicKey": pubKey.Base58(),
			"address":   pubKey.Addr(),
			"tmAddress": pub32.Address().String(),
			"ticketId":  valInfo.TicketID.HexStr(),
		})
	}

	return vList
}
