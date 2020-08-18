package modules

import (
	"fmt"
	"strconv"

	"github.com/spf13/cast"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/themakeos/lobe/api/rpc/client"
	"github.com/themakeos/lobe/modules/types"
	"github.com/themakeos/lobe/node/services"
	"github.com/themakeos/lobe/types/constants"
	"github.com/themakeos/lobe/types/core"

	"github.com/themakeos/lobe/crypto"

	"github.com/themakeos/lobe/util"

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
func NewAttachableChainModule(client client.Client) *ChainModule {
	return &ChainModule{ModuleCommon: types.ModuleCommon{AttachedClient: client}}
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
			Description: "Send the native coin from an account to a destination account",
		},
		{
			Name:        "getHeight",
			Value:       m.GetHeight,
			Description: "Get the current chain height",
		},
		{
			Name:        "getBlockInfo",
			Value:       m.GetBlockInfo,
			Description: "Get summary block information of a given height",
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

	var err error
	var blockHeight int64

	blockHeight, err = strconv.ParseInt(height, 10, 64)
	if err != nil {
		panic(util.ReqErr(400, StatusCodeInvalidParam, "height", "value is invalid"))
	}

	res, err := m.service.GetBlock(blockHeight)
	if err != nil {
		panic(util.ReqErr(500, StatusCodeServerErr, "", err.Error()))
	}

	return res
}

// getHeight returns the current block height
func (m *ChainModule) GetHeight() string {
	bi, err := m.keepers.SysKeeper().GetLastBlockInfo()
	if err != nil {
		panic(util.ReqErr(500, StatusCodeServerErr, "", err.Error()))
	}
	return cast.ToString(bi.Height.Int64())
}

// getBlockInfo get summary block information of a given height
func (m *ChainModule) GetBlockInfo(height string) util.Map {

	var err error
	var blockHeight int64

	blockHeight, err = strconv.ParseInt(height, 10, 64)
	if err != nil {
		panic(util.ReqErr(400, StatusCodeInvalidParam, "height", "value is invalid"))
	}

	res, err := m.keepers.SysKeeper().GetBlockInfo(blockHeight)
	if err != nil {
		panic(util.ReqErr(500, StatusCodeServerErr, "", err.Error()))
	}

	return util.ToMap(res)
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
func (m *ChainModule) GetValidators(height string) (res []util.Map) {

	var err error
	var blockHeight int64

	blockHeight, err = strconv.ParseInt(height, 10, 64)
	if err != nil {
		panic(util.ReqErr(400, StatusCodeInvalidParam, "height", "value is invalid"))
	}

	validators, err := m.keepers.ValidatorKeeper().GetByHeight(blockHeight)
	if err != nil {
		panic(util.ReqErr(500, StatusCodeServerErr, "", err.Error()))
	}

	var vList []util.Map
	for pubKey, valInfo := range validators {
		var pub32 ed25519.PubKeyEd25519
		copy(pub32[:], pubKey.Bytes())

		pubKey := crypto.MustPubKeyFromBytes(pubKey.Bytes())
		vList = append(vList, map[string]interface{}{
			"publicKey": pubKey.Base58(),
			"address":   pubKey.Addr(),
			"tmAddress": pub32.Address().String(),
			"ticketId":  valInfo.TicketID.String(),
		})
	}

	return vList
}
