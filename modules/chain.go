package modules

import (
	"fmt"
	"strconv"

	"github.com/make-os/lobe/modules/types"
	"github.com/make-os/lobe/node/services"
	types2 "github.com/make-os/lobe/rpc/types"
	"github.com/make-os/lobe/types/constants"
	"github.com/make-os/lobe/types/core"
	"github.com/spf13/cast"
	"github.com/tendermint/tendermint/crypto/ed25519"

	"github.com/make-os/lobe/crypto"

	"github.com/make-os/lobe/util"

	"github.com/c-bata/go-prompt"
	"github.com/robertkrimen/otto"
)

// ChainModule provides access to chain information
type ChainModule struct {
	types.ModuleCommon
	service services.Service
	keepers core.Keepers
}

// NewChainModule creates an instance of ChainModule
func NewChainModule(service services.Service, keepers core.Keepers) *ChainModule {
	return &ChainModule{service: service, keepers: keepers}
}

// NewAttachableChainModule creates an instance of ChainModule suitable in attach mode
func NewAttachableChainModule(client types2.Client) *ChainModule {
	return &ChainModule{ModuleCommon: types.ModuleCommon{Client: client}}
}

// globals are functions exposed in the VM's global namespace
func (m *ChainModule) globals() []*types.VMMember {
	return []*types.VMMember{}
}

// methods are functions exposed in the special namespace of this module.
func (m *ChainModule) methods() []*types.VMMember {
	return []*types.VMMember{
		{
			Name:        "getBlock",
			Value:       m.GetBlock,
			Description: "Get full block data at a given height",
		},
		{
			Name:        "getHeight",
			Value:       m.GetHeight,
			Description: "Get the current chain height",
		},
		{
			Name:        "getBlockInfo",
			Value:       m.GetBlockInfo,
			Description: "Get summarized block information at a given height",
		},
		{
			Name:        "getValidators",
			Value:       m.GetValidators,
			Description: "Get validators at a given height",
		},
	}
}

// ConfigureVM configures the JS context and return
// any number of console prompt suggestions
func (m *ChainModule) ConfigureVM(vm *otto.Otto) prompt.Completer {

	// Register the main namespace
	nsMap := map[string]interface{}{}
	util.VMSet(vm, constants.NamespaceChain, nsMap)

	for _, f := range m.methods() {
		nsMap[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", constants.NamespaceChain, f.Name)
		m.Suggestions = append(m.Suggestions, prompt.Suggest{Text: funcFullName, Description: f.Description})
	}

	// Register global functions
	for _, f := range m.globals() {
		vm.Set(f.Name, f.Value)
		m.Suggestions = append(m.Suggestions, prompt.Suggest{Text: f.Name, Description: f.Description})
	}

	return m.Completer
}

// getBlock fetches a block at the given height
func (m *ChainModule) GetBlock(height string) util.Map {

	if m.IsAttached() {
		res, err := m.Client.Chain().GetBlock(cast.ToUint64(height))
		if err != nil {
			panic(err)
		}
		return util.ToMap(res)
	}

	blockHeight, err := strconv.ParseInt(height, 10, 64)
	if err != nil {
		panic(util.ReqErr(400, StatusCodeInvalidParam, "height", "value is invalid"))
	}

	res, err := m.service.GetBlock(blockHeight)
	if err != nil {
		panic(util.ReqErr(500, StatusCodeServerErr, "", err.Error()))
	}

	return util.ToMap(res)
}

// getHeight returns the current block height
func (m *ChainModule) GetHeight() string {

	if m.IsAttached() {
		res, err := m.Client.Chain().GetHeight()
		if err != nil {
			panic(err)
		}
		return cast.ToString(res)
	}

	bi, err := m.keepers.SysKeeper().GetLastBlockInfo()
	if err != nil {
		panic(util.ReqErr(500, StatusCodeServerErr, "", err.Error()))
	}
	return cast.ToString(bi.Height.Int64())
}

// getBlockInfo Get summarized block information at a given height
func (m *ChainModule) GetBlockInfo(height string) util.Map {

	if m.IsAttached() {
		res, err := m.Client.Chain().GetBlockInfo(cast.ToUint64(height))
		if err != nil {
			panic(err)
		}
		return util.ToMap(res)
	}

	blockHeight, err := strconv.ParseInt(height, 10, 64)
	if err != nil {
		panic(util.ReqErr(400, StatusCodeInvalidParam, "height", "value is invalid"))
	}

	res, err := m.keepers.SysKeeper().GetBlockInfo(blockHeight)
	if err != nil {
		panic(util.ReqErr(500, StatusCodeServerErr, "", err.Error()))
	}

	return util.ToBasicMap(res)
}

// getValidators returns validators of a given block
//
//  - height: The target block height
//
// RETURNS res []map
//  - publicKey <string>: The base58 public key of validator
//  - address <string>: The bech32 address of the validator
//  - tmAddr <string>: The tendermint address and the validator
//  - ticketId <string>: The id of the validator ticket
func (m *ChainModule) GetValidators(height string) (res []util.Map) {

	if m.IsAttached() {
		res, err := m.Client.Chain().GetValidators(cast.ToUint64(height))
		if err != nil {
			panic(err)
		}
		return util.StructSliceToMap(res)
	}

	blockHeight, err := strconv.ParseInt(height, 10, 64)
	if err != nil {
		panic(util.ReqErr(400, StatusCodeInvalidParam, "height", "value is invalid"))
	}

	validators, err := m.keepers.ValidatorKeeper().Get(blockHeight)
	if err != nil {
		panic(util.ReqErr(500, StatusCodeServerErr, "", err.Error()))
	}

	var vList []util.Map
	for pubKey, valInfo := range validators {
		var pub32 ed25519.PubKeyEd25519
		copy(pub32[:], pubKey.Bytes())
		pubKey := crypto.MustPubKeyFromBytes(pubKey.Bytes())
		vList = append(vList, map[string]interface{}{
			"pubkey":   pubKey.Base58(),
			"address":  pubKey.Addr(),
			"tmAddr":   pub32.Address().String(),
			"ticketId": valInfo.TicketID.String(),
		})
	}

	return vList
}
