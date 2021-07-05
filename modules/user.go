package modules

import (
	"fmt"

	"github.com/make-os/kit/config"
	"github.com/make-os/kit/crypto/ed25519"
	"github.com/make-os/kit/keystore"
	kstypes "github.com/make-os/kit/keystore/types"
	"github.com/make-os/kit/modules/types"
	"github.com/make-os/kit/node/services"
	types2 "github.com/make-os/kit/rpc/types"
	"github.com/make-os/kit/types/api"
	"github.com/make-os/kit/types/constants"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/errors"
	address2 "github.com/make-os/kit/util/identifier"
	"github.com/spf13/cast"

	"github.com/c-bata/go-prompt"
	at "github.com/make-os/kit/types"
	"github.com/robertkrimen/otto"
)

// UserModule provides account management functionalities
// that are accessed through the JavaScript console environment
type UserModule struct {
	types.ModuleCommon
	cfg      *config.AppConfig
	keystore kstypes.Keystore
	service  services.Service
	logic    core.Logic
}

// NewAttachableUserModule creates an instance of UserModule suitable in attach mode
func NewAttachableUserModule(cfg *config.AppConfig, client types2.Client, ks *keystore.Keystore) *UserModule {
	return &UserModule{ModuleCommon: types.ModuleCommon{Client: client}, cfg: cfg, keystore: ks}
}

// NewUserModule creates an instance of UserModule
func NewUserModule(
	cfg *config.AppConfig,
	keystore kstypes.Keystore,
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
		{Name: "getKeys", Value: m.GetKeys, Description: "Get address of keys on the keystore"},
		{Name: "getPrivKey", Value: m.GetPrivKey, Description: "Get the private key of a key (supports interactive mode)"},
		{Name: "getPubKey", Value: m.GetPublicKey, Description: "Get the public key of an account (supports interactive mode)"},
		{Name: "getNonce", Value: m.GetNonce, Description: "Get the nonce of an account"},
		{Name: "get", Value: m.GetAccount, Description: "Get the account of a given address"},
		{Name: "getBalance", Value: m.GetAvailableBalance, Description: "Get the spendable coin balance of an account"},
		{Name: "getGasBalance", Value: m.GetGasBalance, Description: "Get the gas balance of an account"},
		{Name: "getStakedBalance", Value: m.GetStakedBalance, Description: "Get the total staked coins of an account"},
		{Name: "getValidator", Value: m.GetValidator, Description: "Get the validator information"},
		{Name: "setCommission", Value: m.SetCommission, Description: "Set the percentage of reward to share with a delegator"},
		{Name: "send", Value: m.SendCoin, Description: "Send coins to another user account or a repository"},
		{Name: "gasToCoin", Value: m.BurnGasForCoin, Description: "Burns gas to native coin (Testnet Only)"},
	}
}

// globals are functions exposed in the VM's global namespace
func (m *UserModule) globals() []*types.VMMember {

	defer func() {
		if err := recover(); err != nil {
			m.cfg.G().Log.Error(fmt.Sprint(err))
		}
	}()

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

// GetKeys returns a list of address of keys on the keystore
func (m *UserModule) GetKeys() []string {

	if m.IsAttached() {
		res, err := m.Client.User().GetKeys()
		if err != nil {
			panic(err)
		}
		return res
	}

	accounts, err := m.keystore.List()
	if err != nil {
		panic(errors.ReqErr(500, StatusCodeServerErr, "", err.Error()))
	}

	var resp []string
	for _, a := range accounts {
		resp = append(resp, a.GetUserAddress())
	}

	return resp
}

// getKey returns the private key of an account.
//
// The passphrase argument is used to unlock the account.
// If passphrase is not set, an interactive prompt will be started
// to collect the passphrase without revealing it in the terminal.
//
//  - address: The address corresponding the the local key
//  - [passphrase]: The passphrase of the local key
func (m *UserModule) getKey(address string, passphrase string) *ed25519.Key {

	if address == "" {
		panic(errors.ReqErr(400, StatusCodeAddressRequire, "address", "address is required"))
	}

	// Get the address
	acct, err := m.keystore.GetByIndexOrAddress(address)
	if err != nil {
		if err != at.ErrAccountUnknown {
			panic(errors.ReqErr(500, StatusCodeServerErr, "address", err.Error()))
		}
		panic(errors.ReqErr(404, StatusCodeAccountNotFound, "address", err.Error()))
	}

	if acct.IsUnprotected() {
		passphrase = keystore.DefaultPassphrase
		goto unlock
	}

unlock:
	// Unlock the key using the passphrase
	if err := acct.Unlock(passphrase); err != nil {
		if err == at.ErrInvalidPassphrase {
			panic(errors.ReqErr(401, StatusCodeInvalidPass, "passphrase", err.Error()))
		}
		panic(errors.ReqErr(500, StatusCodeServerErr, "passphrase", err.Error()))
	}

	return acct.GetKey()
}

// GetPrivKey returns the private key of an account.
//
// The passphrase argument is used to unlock the account.
// If passphrase is not set, an interactive prompt will be started
// to collect the passphrase without revealing it in the terminal.
//
//  - address: The address corresponding the the local key
//  - [passphrase]: The passphrase of the local key
func (m *UserModule) GetPrivKey(address string, passphrase ...string) string {

	// If passphrase is not set, start interactive mode
	var pass string
	if len(passphrase) == 0 {
		pass, _ = m.keystore.AskForPasswordOnce()
	} else {
		pass = passphrase[0]
	}

	if m.IsAttached() {
		res, err := m.Client.User().GetPrivateKey(address, pass)
		if err != nil {
			panic(err)
		}
		return res
	}

	return m.getKey(address, pass).PrivKey().Base58()
}

// getPublicKey returns the public key of a key.
//
// The passphrase argument is used to unlock the key.
// If passphrase is not set, an interactive prompt will be started
// to collect the passphrase without revealing it in the terminal.
//
//  - address: The address corresponding the the local key
//  - [passphrase]: The passphrase of the local key
func (m *UserModule) GetPublicKey(address string, passphrase ...string) string {

	// If passphrase is not set, start interactive mode
	var pass string
	if len(passphrase) == 0 {
		pass, _ = m.keystore.AskForPasswordOnce()
	} else {
		pass = passphrase[0]
	}

	if m.IsAttached() {
		res, err := m.Client.User().GetPublicKey(address, pass)
		if err != nil {
			panic(err)
		}
		return res
	}

	return m.getKey(address, pass).PubKey().Base58()
}

// GetNonce returns the current nonce of a network account
//  - address: The address corresponding the account
//  - [passphrase]: The target block height to query (default: latest)
//  - [height]: The target block height to query (default: latest)
func (m *UserModule) GetNonce(address string, height ...uint64) string {

	if m.IsAttached() {
		nonce, err := m.Client.User().GetNonce(address, height...)
		if err != nil {
			panic(err)
		}
		return cast.ToString(nonce)
	}

	acct := m.logic.AccountKeeper().Get(address2.Address(address), height...)
	if acct.IsNil() {
		panic(errors.ReqErr(404, StatusCodeAccountNotFound, "address", at.ErrAccountUnknown.Error()))
	}

	return cast.ToString(acct.Nonce.UInt64())
}

// GetAccount returns the account of the given address.
//  - address: The address corresponding the account
//  - [height]: The target block height to query (default: latest)
func (m *UserModule) GetAccount(address string, height ...uint64) util.Map {

	if m.IsAttached() {
		tx, err := m.Client.User().Get(address, height...)
		if err != nil {
			panic(err)
		}
		return util.ToMap(tx)
	}

	acct := m.logic.AccountKeeper().Get(address2.Address(address), height...)
	if acct.IsNil() {
		panic(errors.ReqErr(404, StatusCodeAccountNotFound, "address", at.ErrAccountUnknown.Error()))
	}

	if len(acct.Stakes) == 0 {
		acct.Stakes = nil
	}

	out := util.ToMap(acct)
	out["gas"] = acct.GetGasBalance()
	return out
}

// GetAvailableBalance returns the spendable balance of an account.
//  - address: The address corresponding the account
//  - [height]: The target block height to query (default: latest)
func (m *UserModule) GetAvailableBalance(address string, height ...uint64) string {

	if m.IsAttached() {
		bal, err := m.Client.User().GetBalance(address, height...)
		if err != nil {
			panic(err)
		}
		return cast.ToString(bal)
	}

	acct := m.logic.AccountKeeper().Get(address2.Address(address), height...)
	if acct.IsNil() {
		panic(errors.ReqErr(404, StatusCodeAccountNotFound, "address", at.ErrAccountUnknown.Error()))
	}

	curBlockInfo, err := m.logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		panic(errors.ReqErr(500, StatusCodeServerErr, "", err.Error()))
	}

	return acct.GetAvailableBalance(uint64(curBlockInfo.Height)).String()
}

// GetStakedBalance getStakedBalance returns the total staked coins of an account
//  - address: The address corresponding the account
//  - [height]: The target block height to query (default: latest)
func (m *UserModule) GetStakedBalance(address string, height ...uint64) string {

	if m.IsAttached() {
		bal, err := m.Client.User().GetStakedBalance(address, height...)
		if err != nil {
			panic(err)
		}
		return cast.ToString(bal)
	}

	acct := m.logic.AccountKeeper().Get(address2.Address(address), height...)
	if acct.IsNil() {
		panic(errors.ReqErr(404, StatusCodeAccountNotFound, "address", at.ErrAccountUnknown.Error()))
	}

	curBlockInfo, err := m.logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		panic(errors.ReqErr(500, StatusCodeServerErr, "", err.Error()))
	}

	return acct.Stakes.TotalStaked(uint64(curBlockInfo.Height)).String()
}

// GetValidator getPrivateValidator returns the address, public and private keys of the validator.
//
//  - includePrivKey: Indicates that the private key of the validator should be included in the result
//
// RETURNS object <map>:
//  - pubkey <string>: The validator base58 public key
//  - address 	<string>: The validator's bech32 address.
//  - tmAddr <string>: The tendermint address
//  - privkey <string>: The validator's base58 public key
func (m *UserModule) GetValidator(includePrivKey ...bool) util.Map {

	inclPrivKey := false
	if len(includePrivKey) > 0 {
		inclPrivKey = includePrivKey[0]
	}

	if m.IsAttached() {
		res, err := m.Client.User().GetValidator(inclPrivKey)
		if err != nil {
			panic(err)
		}
		return util.ToMap(res)
	}

	key, _ := m.cfg.G().PrivVal.GetKey()
	info := map[string]interface{}{
		"pubkey":  key.PubKey().Base58(),
		"address": key.Addr().String(),
		"tmAddr":  m.cfg.G().PrivVal.Key.Address.String(),
	}

	if inclPrivKey {
		info["privkey"] = key.PrivKey().Base58()
	}

	return info
}

// SetCommission setCommission sets the delegator commission for an account
//
// params <map>
//  - nonce <number|string>: The senders next account nonce
//  - fee <number|string>: The transaction fee to pay
//  - commission <number|string>: The network commission value
//  - timestamp <number>: The unix timestamp
//
// options <[]interface{}>
//  - [0] key <string>: The signer's private key
//  - [1] payloadOnly <bool>: When true, returns the payload only, without sending the tx.
//
// RETURNS object <map>:
//  - hash <string>: The transaction hash
func (m *UserModule) SetCommission(params map[string]interface{}, options ...interface{}) util.Map {
	var err error

	var tx = txns.NewBareTxSetDelegateCommission()
	if err = tx.FromMap(params); err != nil {
		panic(errors.ReqErr(400, StatusCodeInvalidParam, "", err.Error()))
	}

	retPayload, signingKey := finalizeTx(tx, m.logic, m.Client, options...)
	if retPayload {
		return tx.ToMap()
	}

	if m.IsAttached() {
		resp, err := m.Client.User().SetCommission(&api.BodySetCommission{
			Commission: tx.Commission.Float(),
			Nonce:      tx.Nonce,
			Fee:        cast.ToFloat64(tx.Fee.String()),
			SigningKey: ed25519.NewKeyFromPrivKey(signingKey),
		})
		if err != nil {
			panic(err)
		}
		return util.ToMap(resp)
	}

	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		panic(errors.ReqErr(400, StatusCodeMempoolAddFail, "", err.Error()))
	}

	return map[string]interface{}{
		"hash": hash,
	}
}

// SendCoin sendCoin sends the native coin from a source account to a destination account.
//
// params <map>
//  - value 		<string>: 			The amount of coin to send
//  - to 			<string>: 			The address of the recipient
//  - nonce 		<number|string>: 	The senders next account nonce
//  - fee 			<number|string>: 	The transaction fee to pay
//  - timestamp 	<number>: 			The unix timestamp
//
// options <[]interface{}>
//  - [0] key <string>: 			The signer's private key
//  - [1] payloadOnly <bool>: 		When true, returns the payload only, without sending the tx.
//
// RETURNS object <map>
//  - hash <string>: 				The transaction hash
func (m *UserModule) SendCoin(params map[string]interface{}, options ...interface{}) util.Map {
	var err error

	var tx = txns.NewBareTxCoinTransfer()
	if err = tx.FromMap(params); err != nil {
		panic(errors.ReqErr(400, StatusCodeInvalidParam, "params", err.Error()))
	}

	retPayload, signingKey := finalizeTx(tx, m.logic, m.Client, options...)
	if retPayload {
		return tx.ToMap()
	}

	if m.IsAttached() {
		resp, err := m.Client.User().Send(&api.BodySendCoin{
			To:         tx.To,
			Nonce:      tx.Nonce,
			Value:      cast.ToFloat64(tx.Value.String()),
			Fee:        cast.ToFloat64(tx.Fee.String()),
			SigningKey: ed25519.NewKeyFromPrivKey(signingKey),
		})
		if err != nil {
			panic(err)
		}
		return util.ToMap(resp)
	}

	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		panic(errors.ReqErr(400, StatusCodeMempoolAddFail, "", err.Error()))
	}

	return map[string]interface{}{
		"hash": hash,
	}
}

// GetGasBalance returns the gas of an account.
//  - address: The address corresponding the account
//  - [height]: The target block height to query (default: latest)
func (m *UserModule) GetGasBalance(address string, height ...uint64) string {
	acct := m.logic.AccountKeeper().Get(address2.Address(address), height...)
	if acct.IsNil() {
		panic(errors.ReqErr(404, StatusCodeAccountNotFound, "address", at.ErrAccountUnknown.Error()))
	}

	return acct.GetGasBalance().String()
}

// BurnGasForCoin creates a tx to burn/converts gas into native coin.
//
// ARGS:
// params <map>
// params.amount <string>:				The amount of gas to burn
// params.nonce <number|string>: 		The senders next account nonce
// params.fee <number|string>: 			The transaction fee to pay
// params.timestamp <number>: 			The unix timestamp
//
// options <[]interface{}>
// options[0] key <string>: 			The signer's private key
// options[1] payloadOnly <bool>: 		When true, returns the payload only, without sending the tx.
//
// RETURNS object <map>
// object.hash <string>: The transaction hash
// TODO: Remove in production build
func (m *UserModule) BurnGasForCoin(params map[string]interface{}, options ...interface{}) util.Map {
	var err error

	var tx = txns.NewBareTxTxBurnGasForCoin()
	if err = tx.FromMap(params); err != nil {
		panic(errors.ReqErr(400, StatusCodeInvalidParam, "params", err.Error()))
	}

	if printPayload, _ := finalizeTx(tx, m.logic, nil, options...); printPayload {
		return tx.ToMap()
	}

	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		panic(errors.ReqErr(400, StatusCodeMempoolAddFail, "", err.Error()))
	}

	return map[string]interface{}{
		"hash": hash,
	}
}
