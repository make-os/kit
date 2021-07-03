package modules

import (
	context2 "context"
	"fmt"
	"strconv"

	"github.com/make-os/kit/modules/types"
	"github.com/make-os/kit/node/services"
	types2 "github.com/make-os/kit/rpc/types"
	"github.com/make-os/kit/types/constants"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/util/epoch"
	"github.com/make-os/kit/util/errors"
	"github.com/spf13/cast"
	tmEd25519 "github.com/tendermint/tendermint/crypto/ed25519"

	"github.com/make-os/kit/crypto/ed25519"

	"github.com/make-os/kit/util"

	"github.com/c-bata/go-prompt"
	"github.com/robertkrimen/otto"
)

// NodeModule provides access to chain information
type NodeModule struct {
	types.ModuleCommon
	service services.Service
	keepers core.Keepers
}

// NewChainModule creates an instance of NodeModule
func NewChainModule(service services.Service, keepers core.Keepers) *NodeModule {
	return &NodeModule{service: service, keepers: keepers}
}

// NewAttachableChainModule creates an instance of NodeModule suitable in attach mode
func NewAttachableChainModule(client types2.Client) *NodeModule {
	return &NodeModule{ModuleCommon: types.ModuleCommon{Client: client}}
}

// globals are functions exposed in the VM's global namespace
func (m *NodeModule) globals() []*types.VMMember {
	return []*types.VMMember{}
}

// methods are functions exposed in the special namespace of this module.
func (m *NodeModule) methods() []*types.VMMember {
	return []*types.VMMember{
		{Name: "getBlock", Value: m.GetBlock, Description: "Get full block data at a given height"},
		{Name: "getCurHeight", Value: m.GetCurHeight, Description: "Get the current chain height"},
		{Name: "getBlockInfo", Value: m.GetBlockInfo, Description: "Get summarized block information at a given height"},
		{Name: "getValidators", Value: m.GetValidators, Description: "Get validators at a given height"},
		{Name: "isSyncing", Value: m.IsSyncing, Description: "Check if the node is synchronizing with peers"},
		{Name: "getCurEpoch", Value: m.GetCurrentEpoch, Description: "Get the current epoch"},
		{Name: "getEpoch", Value: m.GetEpoch, Description: "Get the epoch of a block height"},
		{Name: "getGasMinedInCurEpoch", Value: m.GetTotalGasMinedInCurEpoch, Description: "Get the amount of gas mined in the current epoch"},
		{Name: "getGasMinedInEpoch", Value: m.GetTotalGasMinedInEpoch, Description: "Get the amount of gas mined in an epoch"},
		{Name: "getCurDifficulty", Value: m.GetDifficulty, Description: "Get the current difficulty"},
	}
}

// ConfigureVM configures the JS context and return
// any number of console prompt suggestions
func (m *NodeModule) ConfigureVM(vm *otto.Otto) prompt.Completer {

	// Register the main namespace
	nsMap := map[string]interface{}{}
	util.VMSet(vm, constants.NamespaceNode, nsMap)

	for _, f := range m.methods() {
		nsMap[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", constants.NamespaceNode, f.Name)
		m.Suggestions = append(m.Suggestions, prompt.Suggest{Text: funcFullName, Description: f.Description})
	}

	// Register global functions
	for _, f := range m.globals() {
		vm.Set(f.Name, f.Value)
		m.Suggestions = append(m.Suggestions, prompt.Suggest{Text: f.Name, Description: f.Description})
	}

	return m.Completer
}

// GetBlock fetches a block at the given height
func (m *NodeModule) GetBlock(height string) util.Map {

	if m.IsAttached() {
		res, err := m.Client.Node().GetBlock(cast.ToUint64(height))
		if err != nil {
			panic(err)
		}
		return util.ToMap(res)
	}

	blockHeight, err := strconv.ParseInt(height, 10, 64)
	if err != nil {
		panic(errors.ReqErr(400, StatusCodeInvalidParam, "height", "value is invalid"))
	}

	res, err := m.service.GetBlock(context2.Background(), &blockHeight)
	if err != nil {
		panic(errors.ReqErr(500, StatusCodeServerErr, "", err.Error()))
	}

	return util.ToMap(res)
}

// GetHeight returns the current block height
func (m *NodeModule) GetCurHeight() string {

	if m.IsAttached() {
		res, err := m.Client.Node().GetHeight()
		if err != nil {
			panic(err)
		}
		return cast.ToString(res)
	}

	bi, err := m.keepers.SysKeeper().GetLastBlockInfo()
	if err != nil {
		panic(errors.ReqErr(500, StatusCodeServerErr, "", err.Error()))
	}
	return cast.ToString(bi.Height.Int64())
}

// GetBlockInfo Get summarized block information at a given height
func (m *NodeModule) GetBlockInfo(height string) util.Map {

	if m.IsAttached() {
		res, err := m.Client.Node().GetBlockInfo(cast.ToUint64(height))
		if err != nil {
			panic(err)
		}
		return util.ToMap(res)
	}

	blockHeight, err := strconv.ParseInt(height, 10, 64)
	if err != nil {
		panic(errors.ReqErr(400, StatusCodeInvalidParam, "height", "value is invalid"))
	}

	res, err := m.keepers.SysKeeper().GetBlockInfo(blockHeight)
	if err != nil {
		panic(errors.ReqErr(500, StatusCodeServerErr, "", err.Error()))
	}

	return util.ToJSONMap(res)
}

// GetValidators returns validators of a given block
//
//  - height: The target block height
//
// RETURNS res []map
//  - publicKey <string>: The base58 public key of validator
//  - address <string>: The bech32 address of the validator
//  - tmAddr <string>: The tendermint address and the validator
//  - ticketId <string>: The id of the validator ticket
func (m *NodeModule) GetValidators(height string) (res []util.Map) {

	if m.IsAttached() {
		res, err := m.Client.Node().GetValidators(cast.ToUint64(height))
		if err != nil {
			panic(err)
		}
		return util.StructSliceToMap(res)
	}

	blockHeight, err := strconv.ParseInt(height, 10, 64)
	if err != nil {
		panic(errors.ReqErr(400, StatusCodeInvalidParam, "height", "value is invalid"))
	}

	validators, err := m.keepers.ValidatorKeeper().Get(blockHeight)
	if err != nil {
		panic(errors.ReqErr(500, StatusCodeServerErr, "", err.Error()))
	}

	var vList []util.Map
	for valPubKey, valInfo := range validators {
		pubKey := ed25519.MustPubKeyFromBytes(valPubKey.Bytes())
		vList = append(vList, map[string]interface{}{
			"pubkey":   pubKey.Base58(),
			"address":  pubKey.Addr(),
			"tmAddr":   tmEd25519.PubKey(valPubKey.Bytes()).Address().String(),
			"ticketId": valInfo.TicketID.String(),
		})
	}

	return vList
}

// IsSyncing checks whether the node is synchronizing with peers
func (m *NodeModule) IsSyncing() bool {

	if m.IsAttached() {
		syncing, err := m.Client.Node().IsSyncing()
		if err != nil {
			panic(err)
		}
		return syncing
	}

	syncing, err := m.service.IsSyncing(context2.Background())
	if err != nil {
		panic(errors.ReqErr(500, StatusCodeServerErr, "", err.Error()))
	}

	return syncing
}

// GetCurrentEpoch returns the current epoch
func (m *NodeModule) GetCurrentEpoch() string {
	curEpoch, err := m.keepers.SysKeeper().GetCurrentEpoch()
	if err != nil {
		panic(errors.ReqErr(500, StatusCodeServerErr, "", err.Error()))
	}
	return cast.ToString(curEpoch)
}

// GetEpoch returns the epoch of a block height
func (m *NodeModule) GetEpoch(height int64) string {
	return cast.ToString(epoch.GetEpochAt(height))
}

// GetTotalGasMinedInCurEpoch returns the total gas mined in the current epoch
func (m *NodeModule) GetTotalGasMinedInCurEpoch() string {

	curEpoch, err := m.keepers.SysKeeper().GetCurrentEpoch()
	if err != nil {
		panic(errors.ReqErr(500, StatusCodeServerErr, "", err.Error()))
	}

	bal, err := m.keepers.SysKeeper().GetTotalGasMinedInEpoch(curEpoch)
	if err != nil {
		panic(errors.ReqErr(500, StatusCodeServerErr, "", err.Error()))
	}
	return bal.String()
}

// GetTotalGasMinedInEpoch returns the total gas mined in the given epoch
func (m *NodeModule) GetTotalGasMinedInEpoch(epoch int64) string {
	bal, err := m.keepers.SysKeeper().GetTotalGasMinedInEpoch(epoch)
	if err != nil {
		panic(errors.ReqErr(500, StatusCodeServerErr, "", err.Error()))
	}
	return bal.String()
}

// GetDifficulty returns the current difficulty
func (m *NodeModule) GetDifficulty() string {
	diff, err := m.keepers.SysKeeper().GetCurrentDifficulty()
	if err != nil {
		panic(errors.ReqErr(500, StatusCodeServerErr, "", err.Error()))
	}
	return diff.String()
}
