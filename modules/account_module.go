package modules

import (
	"fmt"

	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/modules/types"
	"gitlab.com/makeos/mosdef/node/services"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/util"

	"github.com/pkg/errors"

	"gitlab.com/makeos/mosdef/accountmgr"

	prompt "github.com/c-bata/go-prompt"
	"github.com/robertkrimen/otto"
	apptypes "gitlab.com/makeos/mosdef/types"
)

// AccountModule provides account management functionalities
// that are accessed through the javascript console environment
type AccountModule struct {
	cfg     *config.AppConfig
	acctMgr *accountmgr.AccountManager
	vm      *otto.Otto
	service services.Service
	logic   core.Logic
}

// NewAccountModule creates an instance of AccountModule
func NewAccountModule(
	cfg *config.AppConfig,
	vm *otto.Otto,
	acctmgr *accountmgr.AccountManager,
	service services.Service,
	logic core.Logic) *AccountModule {
	return &AccountModule{
		cfg:     cfg,
		acctMgr: acctmgr,
		vm:      vm,
		service: service,
		logic:   logic,
	}
}

func (m *AccountModule) namespacedFuncs() []*types.ModulesAggregatorFunc {
	return []*types.ModulesAggregatorFunc{
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

func (m *AccountModule) globals() []*types.ModulesAggregatorFunc {
	return []*types.ModulesAggregatorFunc{
		{
			Name:        "accounts",
			Value:       m.ListLocalAccounts(),
			Description: "Get the list of accounts that exist on this node",
		},
	}
}

// Configure configures the JS context and return
// any number of console prompt suggestions
func (m *AccountModule) Configure() []prompt.Suggest {
	fMap := map[string]interface{}{}
	suggestions := []prompt.Suggest{}

	// Set the namespace object
	util.VMSet(m.vm, apptypes.NamespaceUser, fMap)

	// add namespaced functions
	for _, f := range m.namespacedFuncs() {
		fMap[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", apptypes.NamespaceUser, f.Name)
		suggestions = append(suggestions, prompt.Suggest{Text: funcFullName,
			Description: f.Description})
	}

	// Add global functions
	for _, f := range m.globals() {
		_ = m.vm.Set(f.Name, f.Value)
		suggestions = append(suggestions, prompt.Suggest{Text: f.Name,
			Description: f.Description})
	}

	return suggestions
}

// listAccounts lists all accounts on this node
func (m *AccountModule) ListLocalAccounts() []string {
	accounts, err := m.acctMgr.ListAccounts()
	if err != nil {
		panic(err)
	}

	var resp = []string{}
	for _, a := range accounts {
		resp = append(resp, a.Address)
	}

	return resp
}

// getKey returns the private key of an account.
// The passphrase argument is used to unlock the account.
// If passphrase is not set, an interactive prompt will be started
// to collect the passphrase without revealing it in the terminal.
//
// address: The address corresponding the the local account
// [passphrase]: The passphrase of the local account
func (m *AccountModule) GetKey(address string, passphrase ...string) string {
	var pass string

	if address == "undefined" {
		panic(fmt.Errorf("address is required"))
	}

	// Find the address
	acct, err := m.acctMgr.GetByAddress(address)
	if err != nil {
		panic(err)
	}

	// If passphrase is not set, start interactive mode
	if len(passphrase) == 0 {
		pass, err = m.acctMgr.AskForPasswordOnce()
		if err != nil {
			panic(err)
		}
	} else {
		pass = passphrase[0]
	}

	// Unlock the account using the passphrase
	if err := acct.Unlock(pass); err != nil {
		panic(errors.Wrap(err, "failed to unlock account with the provided passphrase"))
	}

	return acct.GetKey().PrivKey().Base58()
}

// getPublicKey returns the public key of an account.
// The passphrase argument is used to unlock the account.
// If passphrase is not set, an interactive prompt will be started
// to collect the passphrase without revealing it in the terminal.
//
// address: The address corresponding the the local account
// [passphrase]: The passphrase of the local account
func (m *AccountModule) GetPublicKey(address string, passphrase ...string) string {
	var pass string

	if address == "undefined" {
		panic(fmt.Errorf("address is required"))
	}

	// Find the address
	acct, err := m.acctMgr.GetByAddress(address)
	if err != nil {
		panic(err)
	}

	// If passphrase is not set, start interactive mode
	if len(passphrase) == 0 {
		pass, err = m.acctMgr.AskForPasswordOnce()
		if err != nil {
			panic(err)
		}
	} else {
		pass = passphrase[0]
	}

	// Unlock the account using the passphrase
	if err := acct.Unlock(pass); err != nil {
		panic(errors.Wrap(err, "failed to unlock account with the provided passphrase"))
	}

	return acct.GetKey().PubKey().Base58()
}

// GetNonce returns the current nonce of a network account
// address: The address corresponding the account
// [passphrase]: The target block height to query (default: latest)
func (m *AccountModule) GetNonce(address string, height ...uint64) string {
	account := m.logic.AccountKeeper().GetAccount(util.String(address), height...)
	if account.IsNil() {
		panic(apptypes.ErrAccountUnknown)
	}
	return fmt.Sprintf("%d", account.Nonce)
}

// GetAccount returns the account of the given address
// address: The address corresponding the account
// [height]: The target block height to query (default: latest)
func (m *AccountModule) GetAccount(address string, height ...uint64) interface{} {
	account := m.logic.AccountKeeper().GetAccount(util.String(address), height...)
	if account.IsNil() {
		panic(apptypes.ErrAccountUnknown)
	}
	if len(account.Stakes) == 0 {
		account.Stakes = nil
	}
	return EncodeForJS(account)
}

// GetSpendableBalance returns the spendable balance of an account
// address: The address corresponding the account
// [height]: The target block height to query (default: latest)
func (m *AccountModule) GetSpendableBalance(address string, height ...uint64) string {
	account := m.logic.AccountKeeper().GetAccount(util.String(address), height...)
	if account.Balance.String() == "0" && account.Nonce == uint64(0) {
		panic(apptypes.ErrAccountUnknown)
	}

	curBlockInfo, err := m.logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		panic(errors.Wrap(err, "failed to get current block info"))
	}

	return account.GetSpendableBalance(uint64(curBlockInfo.Height)).String()
}

// getStakedBalance returns the total staked coins of an account
//
// ARGS:
// address: The address corresponding the account
// [height]: The target block height to query (default: latest)
func (m *AccountModule) GetStakedBalance(address string, height ...uint64) string {
	account := m.logic.AccountKeeper().GetAccount(util.String(address), height...)
	if account.Balance.String() == "0" && account.Nonce == uint64(0) {
		panic(apptypes.ErrAccountUnknown)
	}

	curBlockInfo, err := m.logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		panic(errors.Wrap(err, "failed to get current block info"))
	}

	return account.Stakes.TotalStaked(uint64(curBlockInfo.Height)).String()
}

// getPrivateValidator returns the address, public and private keys of the validator.
//
// ARGS:
// includePrivKey: Indicates that the private key of the validator should be included in the result
//
// RETURNS object <map>:
// object.publicKey <string> -	The validator base58 public key
// object.address 	<string> -	The validator's bech32 address.
// object.tmAddress <string> -	The tendermint address
func (m *AccountModule) GetPrivateValidator(includePrivKey ...bool) interface{} {
	key, _ := m.cfg.G().PrivVal.GetKey()

	info := map[string]string{
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
func (m *AccountModule) SetCommission(params map[string]interface{},
	options ...interface{}) interface{} {
	var err error

	var tx = core.NewBareTxSetDelegateCommission()
	if err = tx.FromMap(params); err != nil {
		panic(err)
	}

	payloadOnly := finalizeTx(tx, m.logic, options...)
	if payloadOnly {
		return EncodeForJS(tx.ToMap())
	}

	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		panic(errors.Wrap(err, "failed to send transaction"))
	}

	return EncodeForJS(map[string]interface{}{
		"hash": hash,
	})
}
