package modules

import (
	"fmt"

	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/console"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/keystore"
	keystoretypes "gitlab.com/makeos/mosdef/keystore/types"
	"gitlab.com/makeos/mosdef/modules/types"
	"gitlab.com/makeos/mosdef/node/services"
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/txns"
	"gitlab.com/makeos/mosdef/util"

	"github.com/c-bata/go-prompt"
	"github.com/robertkrimen/otto"
	at "gitlab.com/makeos/mosdef/types"
)

// UserModule provides account management functionalities
// that are accessed through the javascript console environment
type UserModule struct {
	console.ConsoleSuggestions
	cfg      *config.AppConfig
	keystore keystoretypes.Keystore
	service  services.Service
	logic    core.Logic
}

// NewUserModule creates an instance of UserModule
func NewUserModule(
	cfg *config.AppConfig,
	keystore keystoretypes.Keystore,
	service services.Service,
	logic core.Logic) *UserModule {
	return &UserModule{
		cfg:      cfg,
		keystore: keystore,
		service:  service,
		logic:    logic,
	}
}

// ConsoleOnlyMode indicates that this module can be used on console-only mode
func (m *UserModule) ConsoleOnlyMode() bool {
	return false
}

// methods are functions exposed in the special namespace of this module.
func (m *UserModule) methods() []*types.ModuleFunc {
	return []*types.ModuleFunc{
		{
			Name:        "listAccounts",
			Value:       m.ListLocalAccounts,
			Description: "List local accounts on this node",
		},
		{
			Name:        "getKey",
			Value:       m.GetKey,
			Description: "Get the private key of an account (supports interactive mode)",
		},
		{
			Name:        "getPublicKey",
			Value:       m.GetPublicKey,
			Description: "Get the public key of an account (supports interactive mode)",
		},
		{
			Name:        "getNonce",
			Value:       m.GetNonce,
			Description: "Get the nonce of an account",
		},
		{
			Name:        "get",
			Value:       m.GetAccount,
			Description: "Get the account of a given address",
		},
		{
			Name:        "getBalance",
			Value:       m.GetAvailableBalance,
			Description: "Get the spendable coin balance of an account",
		},
		{
			Name:        "getStakedBalance",
			Value:       m.GetStakedBalance,
			Description: "Get the total staked coins of an account",
		},
		{
			Name:        "getValidatorInfo",
			Value:       m.GetValidatorInfo,
			Description: "Get the validator information",
		},
		{
			Name:        "setCommission",
			Value:       m.SetCommission,
			Description: "Set the percentage of reward to share with a delegator",
		},
	}
}

// globals are functions exposed in the VM's global namespace
func (m *UserModule) globals() []*types.ModuleFunc {
	return []*types.ModuleFunc{
		{
			Name:        "accounts",
			Value:       m.ListLocalAccounts(),
			Description: "Get the list of accounts that exist on this node",
		},
	}
}

// ConfigureVM configures the JS context and return
// any number of console prompt suggestions
func (m *UserModule) ConfigureVM(vm *otto.Otto) prompt.Completer {

	// Set the namespace object
	nsMap := map[string]interface{}{}
	util.VMSet(vm, constants.NamespaceUser, nsMap)

	// add methods functions
	for _, f := range m.methods() {
		nsMap[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", constants.NamespaceUser, f.Name)
		m.Suggestions = append(m.Suggestions, prompt.Suggest{Text: funcFullName, Description: f.Description})
	}

	// Register global functions
	for _, f := range m.globals() {
		_ = vm.Set(f.Name, f.Value)
		m.Suggestions = append(m.Suggestions, prompt.Suggest{Text: f.Name, Description: f.Description})
	}

	return m.Completer
}

// listAccounts lists all accounts on this node
func (m *UserModule) ListLocalAccounts() []string {
	accounts, err := m.keystore.List()
	if err != nil {
		panic(util.StatusErr(500, StatusCodeServerErr, "", err.Error()))
	}

	var resp []string
	for _, a := range accounts {
		resp = append(resp, a.GetAddress())
	}

	return resp
}

// getKey returns the private key of an account.
// The passphrase argument is used to unlock the account.
// If passphrase is not set, an interactive prompt will be started
// to collect the passphrase without revealing it in the terminal.
//
// address: The address corresponding the the local key
// [passphrase]: The passphrase of the local key
func (m *UserModule) getKey(address string, passphrase ...string) *crypto.Key {

	var pass string
	if address == "" {
		panic(util.StatusErr(400, StatusCodeAddressRequire, "address", "address is required"))
	}

	// Get the address
	acct, err := m.keystore.GetByAddress(address)
	if err != nil {
		if err != at.ErrAccountUnknown {
			panic(util.StatusErr(500, StatusCodeServerErr, "address", err.Error()))
		}
		panic(util.StatusErr(404, StatusCodeAccountNotFound, "address", err.Error()))
	}

	if acct.IsUnprotected() {
		pass = keystore.DefaultPassphrase
		goto unlock
	}

	// If passphrase is not set, start interactive mode
	if len(passphrase) == 0 {
		pass = m.keystore.AskForPasswordOnce()
	} else {
		pass = passphrase[0]
	}

unlock:
	// Unlock the key using the passphrase
	if err := acct.Unlock(pass); err != nil {
		if err == at.ErrInvalidPassphrase {
			panic(util.StatusErr(401, StatusCodeInvalidPass, "passphrase", err.Error()))
		}
		panic(util.StatusErr(500, StatusCodeServerErr, "passphrase", err.Error()))
	}

	return acct.GetKey()
}

// GetKey returns the private key of an account.
// The passphrase argument is used to unlock the account.
// If passphrase is not set, an interactive prompt will be started
// to collect the passphrase without revealing it in the terminal.
//
// address: The address corresponding the the local key
// [passphrase]: The passphrase of the local key
func (m *UserModule) GetKey(address string, passphrase ...string) string {
	return m.getKey(address, passphrase...).PrivKey().Base58()
}

// getPublicKey returns the public key of a key.
// The passphrase argument is used to unlock the key.
// If passphrase is not set, an interactive prompt will be started
// to collect the passphrase without revealing it in the terminal.
//
// address: The address corresponding the the local key
// [passphrase]: The passphrase of the local key
func (m *UserModule) GetPublicKey(address string, passphrase ...string) string {
	return m.getKey(address, passphrase...).PubKey().Base58()
}

// GetNonce returns the current nonce of a network account
// address: The address corresponding the account
// [passphrase]: The target block height to query (default: latest)
// [height]: The target block height to query (default: latest)
func (m *UserModule) GetNonce(address string, height ...uint64) string {
	acct := m.logic.AccountKeeper().Get(util.Address(address), height...)
	if acct.IsNil() {
		panic(util.StatusErr(404, StatusCodeAccountNotFound, "address", at.ErrAccountUnknown.Error()))
	}
	return fmt.Sprintf("%d", acct.Nonce)
}

// Get returns the account of the given address
// address: The address corresponding the account
// [height]: The target block height to query (default: latest)
func (m *UserModule) GetAccount(address string, height ...uint64) util.Map {
	acct := m.logic.AccountKeeper().Get(util.Address(address), height...)
	if acct.IsNil() {
		panic(util.StatusErr(404, StatusCodeAccountNotFound, "address", at.ErrAccountUnknown.Error()))
	}

	if len(acct.Stakes) == 0 {
		acct.Stakes = nil
	}

	return util.ToMap(acct)
}

// GetAvailableBalance returns the spendable balance of an account.
// address: The address corresponding the account
// [height]: The target block height to query (default: latest)
func (m *UserModule) GetAvailableBalance(address string, height ...uint64) string {
	acct := m.logic.AccountKeeper().Get(util.Address(address), height...)
	if acct.IsNil() {
		panic(util.StatusErr(404, StatusCodeAccountNotFound, "address", at.ErrAccountUnknown.Error()))
	}

	curBlockInfo, err := m.logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		panic(util.StatusErr(500, StatusCodeServerErr, "", err.Error()))
	}

	return acct.GetAvailableBalance(uint64(curBlockInfo.Height)).String()
}

// getStakedBalance returns the total staked coins of an account
//
// ARGS:
// address: The address corresponding the account
// [height]: The target block height to query (default: latest)
//
// RETURNS <string>: numeric value
func (m *UserModule) GetStakedBalance(address string, height ...uint64) string {
	acct := m.logic.AccountKeeper().Get(util.Address(address), height...)
	if acct.IsNil() {
		panic(util.StatusErr(404, StatusCodeAccountNotFound, "address", at.ErrAccountUnknown.Error()))
	}

	curBlockInfo, err := m.logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		panic(util.StatusErr(500, StatusCodeServerErr, "", err.Error()))
	}

	return acct.Stakes.TotalStaked(uint64(curBlockInfo.Height)).String()
}

// getPrivateValidator returns the address, public and private keys of the validator.
//
// ARGS:
// includePrivKey: Indicates that the private key of the validator should be included in the result
//
// RETURNS object <map>:
// publicKey <string>:	The validator base58 public key
// address 	<string>:	The validator's bech32 address.
// tmAddress <string>:	The tendermint address
func (m *UserModule) GetValidatorInfo(includePrivKey ...bool) util.Map {
	key, _ := m.cfg.G().PrivVal.GetKey()

	info := map[string]interface{}{
		"publicKey": key.PubKey().Base58(),
		"address":   key.Addr().String(),
		"tmAddress": m.cfg.G().PrivVal.Key.Address.String(),
	}

	if len(includePrivKey) > 0 && includePrivKey[0] {
		info["privateKey"] = key.PrivKey().Base58()
	}

	return info
}

// setCommission sets the delegator commission for an account
//
// ARGS:
// params <map>
// params.nonce <number|string>: 		The senders next account nonce
// params.fee <number|string>: 			The transaction fee to pay
// params.commission <number|string>:	The network commission value
// params.timestamp <number>: 			The unix timestamp
//
// options <[]interface{}>
// options[0] key <string>: 			The signer's private key
// options[1] payloadOnly <bool>: 		When true, returns the payload only, without sending the tx.
//
// RETURNS object <map>:
// object.hash <string>: The transaction hash
func (m *UserModule) SetCommission(params map[string]interface{}, options ...interface{}) util.Map {
	var err error

	var tx = txns.NewBareTxSetDelegateCommission()
	if err = tx.FromMap(params); err != nil {
		panic(util.StatusErr(400, StatusCodeInvalidParam, "", err.Error()))
	}

	if finalizeTx(tx, m.logic, options...) {
		return tx.ToMap()
	}

	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		panic(util.StatusErr(400, StatusCodeMempoolAddFail, "", err.Error()))
	}

	return map[string]interface{}{
		"hash": hash,
	}
}
