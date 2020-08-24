package modules

import (
	"fmt"

	"github.com/make-os/lobe/api/rpc/client"
	apitypes "github.com/make-os/lobe/api/types"
	"github.com/make-os/lobe/config"
	"github.com/make-os/lobe/crypto"
	"github.com/make-os/lobe/keystore"
	keystoretypes "github.com/make-os/lobe/keystore/types"
	"github.com/make-os/lobe/modules/types"
	"github.com/make-os/lobe/node/services"
	"github.com/make-os/lobe/types/constants"
	"github.com/make-os/lobe/types/core"
	"github.com/make-os/lobe/types/txns"
	"github.com/make-os/lobe/util"
	address2 "github.com/make-os/lobe/util/identifier"
	"github.com/spf13/cast"

	"github.com/c-bata/go-prompt"
	at "github.com/make-os/lobe/types"
	"github.com/robertkrimen/otto"
)

// UserModule provides account management functionalities
// that are accessed through the JavaScript console environment
type UserModule struct {
	types.ModuleCommon
	cfg      *config.AppConfig
	keystore keystoretypes.Keystore
	service  services.Service
	logic    core.Logic
}

// NewAttachableUserModule creates an instance of UserModule suitable in attach mode
func NewAttachableUserModule(client client.Client, ks *keystore.Keystore) *UserModule {
	return &UserModule{ModuleCommon: types.ModuleCommon{AttachedClient: client}, keystore: ks}
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

// methods are functions exposed in the special namespace of this module.
func (m *UserModule) methods() []*types.VMMember {
	return []*types.VMMember{
		{
			Name:        "getKeys",
			Value:       m.GetKeys,
			Description: "List keys on the keystore",
		},
		{
			Name:        "getKey",
			Value:       m.GetKey,
			Description: "Get the private key of a key (supports interactive mode)",
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
			Name:        "getValidator",
			Value:       m.GetValidatorKey,
			Description: "Get the validator information",
		},
		{
			Name:        "setCommission",
			Value:       m.SetCommission,
			Description: "Set the percentage of reward to share with a delegator",
		},
		{
			Name:        "send",
			Value:       m.SendCoin,
			Description: "Send coins to another user account or a repository",
		},
	}
}

// globals are functions exposed in the VM's global namespace
func (m *UserModule) globals() []*types.VMMember {
	return []*types.VMMember{
		{
			Name:        "accounts",
			Value:       m.GetKeys(),
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

// ListLocalAccounts lists all accounts on this node
func (m *UserModule) GetKeys() []string {
	accounts, err := m.keystore.List()
	if err != nil {
		panic(util.ReqErr(500, StatusCodeServerErr, "", err.Error()))
	}

	var resp []string
	for _, a := range accounts {
		resp = append(resp, a.GetUserAddress())
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
		panic(util.ReqErr(400, StatusCodeAddressRequire, "address", "address is required"))
	}

	// Get the address
	acct, err := m.keystore.GetByIndexOrAddress(address)
	if err != nil {
		if err != at.ErrAccountUnknown {
			panic(util.ReqErr(500, StatusCodeServerErr, "address", err.Error()))
		}
		panic(util.ReqErr(404, StatusCodeAccountNotFound, "address", err.Error()))
	}

	if acct.IsUnprotected() {
		pass = keystore.DefaultPassphrase
		goto unlock
	}

	// If passphrase is not set, start interactive mode
	if len(passphrase) == 0 {
		pass, _ = m.keystore.AskForPasswordOnce()
	} else {
		pass = passphrase[0]
	}

unlock:
	// Unlock the key using the passphrase
	if err := acct.Unlock(pass); err != nil {
		if err == at.ErrInvalidPassphrase {
			panic(util.ReqErr(401, StatusCodeInvalidPass, "passphrase", err.Error()))
		}
		panic(util.ReqErr(500, StatusCodeServerErr, "passphrase", err.Error()))
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
	acct := m.logic.AccountKeeper().Get(address2.Address(address), height...)
	if acct.IsNil() {
		panic(util.ReqErr(404, StatusCodeAccountNotFound, "address", at.ErrAccountUnknown.Error()))
	}
	return fmt.Sprintf("%d", acct.Nonce)
}

// Get returns the account of the given address
// address: The address corresponding the account
// [height]: The target block height to query (default: latest)
func (m *UserModule) GetAccount(address string, height ...uint64) util.Map {

	if m.InAttachMode() {
		tx, err := m.AttachedClient.GetAccount(address, height...)
		if err != nil {
			panic(err)
		}
		return util.ToMap(tx)
	}

	acct := m.logic.AccountKeeper().Get(address2.Address(address), height...)
	if acct.IsNil() {
		panic(util.ReqErr(404, StatusCodeAccountNotFound, "address", at.ErrAccountUnknown.Error()))
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
	acct := m.logic.AccountKeeper().Get(address2.Address(address), height...)
	if acct.IsNil() {
		panic(util.ReqErr(404, StatusCodeAccountNotFound, "address", at.ErrAccountUnknown.Error()))
	}

	curBlockInfo, err := m.logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		panic(util.ReqErr(500, StatusCodeServerErr, "", err.Error()))
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
	acct := m.logic.AccountKeeper().Get(address2.Address(address), height...)
	if acct.IsNil() {
		panic(util.ReqErr(404, StatusCodeAccountNotFound, "address", at.ErrAccountUnknown.Error()))
	}

	curBlockInfo, err := m.logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		panic(util.ReqErr(500, StatusCodeServerErr, "", err.Error()))
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
func (m *UserModule) GetValidatorKey(includePrivKey ...bool) util.Map {
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
		panic(util.ReqErr(400, StatusCodeInvalidParam, "", err.Error()))
	}

	if printPayload, _ := finalizeTx(tx, m.logic, nil, options...); printPayload {
		return tx.ToMap()
	}

	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		panic(util.ReqErr(400, StatusCodeMempoolAddFail, "", err.Error()))
	}

	return map[string]interface{}{
		"hash": hash,
	}
}

// sendCoin sends the native coin from a source account to a destination account.
//
// ARGS:
// params <map>
// params.value 		<string>: 			The amount of coin to send
// params.to 			<string>: 			The address of the recipient
// params.nonce 		<number|string>: 	The senders next account nonce
// params.fee 			<number|string>: 	The transaction fee to pay
// params.timestamp 	<number>: 			The unix timestamp
//
// options <[]interface{}>
// options[0] key <string>: 			The signer's private key
// options[1] payloadOnly <bool>: 		When true, returns the payload only, without sending the tx.
//
// RETURNS object <map>
// object.hash <string>: 				The transaction hash
func (m *UserModule) SendCoin(params map[string]interface{}, options ...interface{}) util.Map {
	var err error

	var tx = txns.NewBareTxCoinTransfer()
	if err = tx.FromMap(params); err != nil {
		panic(util.ReqErr(400, StatusCodeInvalidParam, "params", err.Error()))
	}

	printPayload, signingKey := finalizeTx(tx, m.logic, m.AttachedClient, options...)
	if printPayload {
		return tx.ToMap()
	}

	if m.InAttachMode() {
		resp, err := m.AttachedClient.SendCoin(&apitypes.SendCoinBody{
			To:         tx.To,
			Nonce:      tx.Nonce,
			Value:      cast.ToFloat64(tx.Value.String()),
			Fee:        cast.ToFloat64(tx.Fee.String()),
			SigningKey: crypto.NewKeyFromPrivKey(signingKey),
		})
		if err != nil {
			panic(err)
		}
		return util.ToMap(resp)
	}

	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		panic(util.ReqErr(400, StatusCodeMempoolAddFail, "", err.Error()))
	}

	return map[string]interface{}{
		"hash": hash,
	}
}
