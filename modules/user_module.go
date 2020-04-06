package modules

import (
	"fmt"

	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/keystore"
	"gitlab.com/makeos/mosdef/node/services"
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/modules"
	"gitlab.com/makeos/mosdef/util"

	"github.com/c-bata/go-prompt"
	"github.com/robertkrimen/otto"
	apptypes "gitlab.com/makeos/mosdef/types"
)

// UserModule provides account management functionalities
// that are accessed through the javascript console environment
type UserModule struct {
	cfg     *config.AppConfig
	acctMgr *keystore.Keystore
	vm      *otto.Otto
	service services.Service
	logic   core.Logic
}

// NewUserModule creates an instance of UserModule
func NewUserModule(
	cfg *config.AppConfig,
	vm *otto.Otto,
	acctmgr *keystore.Keystore,
	service services.Service,
	logic core.Logic) *UserModule {
	return &UserModule{
		cfg:     cfg,
		acctMgr: acctmgr,
		vm:      vm,
		service: service,
		logic:   logic,
	}
}

func (m *UserModule) namespacedFuncs() []*modules.ModuleFunc {
	return []*modules.ModuleFunc{
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
			Value:       m.GetSpendableBalance,
			Description: "Get the spendable coin balance of an account",
		},
		{
			Name:        "getStakedBalance",
			Value:       m.GetStakedBalance,
			Description: "Get the total staked coins of an account",
		},
		{
			Name:        "getPV",
			Value:       m.GetPrivateValidator,
			Description: "Get the private validator information",
		},
		{
			Name:        "setCommission",
			Value:       m.SetCommission,
			Description: "Set the percentage of reward to share with a delegator",
		},
	}
}

func (m *UserModule) globals() []*modules.ModuleFunc {
	return []*modules.ModuleFunc{
		{
			Name:        "accounts",
			Value:       m.ListLocalAccounts(),
			Description: "Get the list of accounts that exist on this node",
		},
	}
}

// Configure configures the JS context and return
// any number of console prompt suggestions
func (m *UserModule) Configure() []prompt.Suggest {
	fMap := map[string]interface{}{}
	var suggestions []prompt.Suggest

	// Set the namespace object
	util.VMSet(m.vm, constants.NamespaceUser, fMap)

	// add namespaced functions
	for _, f := range m.namespacedFuncs() {
		fMap[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", constants.NamespaceUser, f.Name)
		suggestions = append(suggestions, prompt.Suggest{Text: funcFullName,
			Description: f.Description})
	}

	// Register global functions
	for _, f := range m.globals() {
		_ = m.vm.Set(f.Name, f.Value)
		suggestions = append(suggestions, prompt.Suggest{Text: f.Name,
			Description: f.Description})
	}

	return suggestions
}

// listAccounts lists all accounts on this node
func (m *UserModule) ListLocalAccounts() []string {
	accounts, err := m.acctMgr.List()
	if err != nil {
		panic(util.NewStatusError(500, StatusCodeAppErr, "", err.Error()))
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
func (m *UserModule) GetKey(address string, passphrase ...string) string {
	var pass string

	if address == "" {
		panic(util.NewStatusError(400, StatusCodeAddressRequire, "address", "address is required"))
	}

	// Get the address
	acct, err := m.acctMgr.GetByAddress(address)
	if err != nil {
		if err != apptypes.ErrAccountUnknown {
			panic(util.NewStatusError(500, StatusCodeAppErr, "address", err.Error()))
		}
		panic(util.NewStatusError(404, StatusCodeAccountNotFound, "address", err.Error()))
	}

	if acct.IsUnprotected() {
		pass = keystore.DefaultPassphrase
		goto unlock
	}

	// If passphrase is not set, start interactive mode
	if len(passphrase) == 0 {
		pass = m.acctMgr.AskForPasswordOnce()
	} else {
		pass = passphrase[0]
	}

unlock:
	// Unlock the key using the passphrase
	if err := acct.Unlock(pass); err != nil {
		if err == apptypes.ErrInvalidPassphrase {
			panic(util.NewStatusError(401, StatusCodeInvalidPass, "passphrase", err.Error()))
		}
		panic(util.NewStatusError(500, StatusCodeAppErr, "passphrase", err.Error()))
	}

	return acct.GetKey().PrivKey().Base58()
}

// getPublicKey returns the public key of a key.
// The passphrase argument is used to unlock the key.
// If passphrase is not set, an interactive prompt will be started
// to collect the passphrase without revealing it in the terminal.
//
// address: The address corresponding the the local key
// [passphrase]: The passphrase of the local key
func (m *UserModule) GetPublicKey(address string, passphrase ...string) string {
	var pass string

	if address == "" {
		panic(util.NewStatusError(400, StatusCodeAddressRequire, "address", "address is required"))
	}

	// Get the address
	acct, err := m.acctMgr.GetByAddress(address)
	if err != nil {
		if err == apptypes.ErrAccountUnknown {
			panic(util.NewStatusError(404, StatusCodeAccountNotFound, "address", err.Error()))
		}
		panic(util.NewStatusError(500, StatusCodeAppErr, "", err.Error()))
	}

	if acct.IsUnprotected() {
		pass = keystore.DefaultPassphrase
		goto unlock
	}

	// If passphrase is not set, start interactive mode
	if len(passphrase) == 0 {
		pass = m.acctMgr.AskForPasswordOnce()
	} else {
		pass = passphrase[0]
	}

unlock:
	// Unlock the key using the passphrase
	if err := acct.Unlock(pass); err != nil {
		if err == apptypes.ErrInvalidPassphrase {
			panic(util.NewStatusError(401, StatusCodeInvalidPass, "passphrase", err.Error()))
		}
		panic(util.NewStatusError(500, StatusCodeAppErr, "passphrase", err.Error()))
	}

	return acct.GetKey().PubKey().Base58()
}

// GetNonce returns the current nonce of a network account
// address: The address corresponding the account
// [passphrase]: The target block height to query (default: latest)
// [height]: The target block height to query (default: latest)
func (m *UserModule) GetNonce(address string, height ...uint64) string {
	acct := m.logic.AccountKeeper().Get(util.Address(address), height...)
	if acct.IsNil() {
		panic(util.NewStatusError(404, StatusCodeAccountNotFound,
			"address", apptypes.ErrAccountUnknown.Error()))
	}
	return fmt.Sprintf("%d", acct.Nonce)
}

// Get returns the account of the given address
// address: The address corresponding the account
// [height]: The target block height to query (default: latest)
func (m *UserModule) GetAccount(address string, height ...uint64) util.Map {
	acct := m.logic.AccountKeeper().Get(util.Address(address), height...)
	if acct.IsNil() {
		panic(util.NewStatusError(404, StatusCodeAccountNotFound,
			"address", apptypes.ErrAccountUnknown.Error()))
	}
	if len(acct.Stakes) == 0 {
		acct.Stakes = nil
	}
	return EncodeForJS(acct)
}

// GetSpendableBalance returns the spendable balance of an account
// address: The address corresponding the account
// [height]: The target block height to query (default: latest)
func (m *UserModule) GetSpendableBalance(address string, height ...uint64) string {
	acct := m.logic.AccountKeeper().Get(util.Address(address), height...)
	if acct.IsNil() {
		panic(util.NewStatusError(404, StatusCodeAccountNotFound,
			"address", apptypes.ErrAccountUnknown.Error()))
	}

	curBlockInfo, err := m.logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		panic(util.NewStatusError(500, StatusCodeAppErr, "", err.Error()))
	}

	return acct.GetSpendableBalance(uint64(curBlockInfo.Height)).String()
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
		panic(util.NewStatusError(404, StatusCodeAccountNotFound,
			"address", apptypes.ErrAccountUnknown.Error()))
	}

	curBlockInfo, err := m.logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		panic(util.NewStatusError(500, StatusCodeAppErr, "", err.Error()))
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
func (m *UserModule) GetPrivateValidator(includePrivKey ...bool) util.Map {
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

	var tx = core.NewBareTxSetDelegateCommission()
	if err = tx.FromMap(params); err != nil {
		panic(util.NewStatusError(400, StatusCodeInvalidParams, "", err.Error()))
	}

	payloadOnly := finalizeTx(tx, m.logic, options...)
	if payloadOnly {
		return EncodeForJS(tx.ToMap())
	}

	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		panic(util.NewStatusError(400, StatusCodeMempoolAddFail, "", err.Error()))
	}

	return EncodeForJS(map[string]interface{}{
		"hash": hash,
	})
}
