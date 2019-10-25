package jsmodules

import (
	"fmt"
	"time"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/crypto"

	"github.com/makeos/mosdef/util"

	"github.com/pkg/errors"

	"github.com/makeos/mosdef/accountmgr"

	prompt "github.com/c-bata/go-prompt"
	"github.com/makeos/mosdef/types"
	"github.com/robertkrimen/otto"
)

// AccountModule provides account management functionalities
// that are accessed through the javascript console environment
type AccountModule struct {
	cfg     *config.EngineConfig
	acctMgr *accountmgr.AccountManager
	vm      *otto.Otto
	service types.Service
	logic   types.Logic
}

// NewAccountModule creates an instance of AccountModule
func NewAccountModule(
	cfg *config.EngineConfig,
	vm *otto.Otto,
	acctmgr *accountmgr.AccountManager,
	service types.Service,
	logic types.Logic) *AccountModule {
	return &AccountModule{
		cfg:     cfg,
		acctMgr: acctmgr,
		vm:      vm,
		service: service,
		logic:   logic,
	}
}

func (m *AccountModule) namespacedFuncs() []*types.JSModuleFunc {
	return []*types.JSModuleFunc{
		&types.JSModuleFunc{
			Name:        "listAccounts",
			Value:       m.listAccounts,
			Description: "Fetch all accounts that exist on this node",
		},
		&types.JSModuleFunc{
			Name:        "getKey",
			Value:       m.getKey,
			Description: "Get the private key of an account (supports interactive mode)",
		},
		&types.JSModuleFunc{
			Name:        "getNonce",
			Value:       m.getNonce,
			Description: "Get the nonce of an account",
		},
		&types.JSModuleFunc{
			Name:        "get",
			Value:       m.getAccount,
			Description: "Get the account of a given address",
		},
		&types.JSModuleFunc{
			Name:        "getBalance",
			Value:       m.getSpendableBalance,
			Description: "Get the spendable coin balance of an account",
		},
		&types.JSModuleFunc{
			Name:        "getStakedBalance",
			Value:       m.getStakedBalance,
			Description: "Get the total staked coins of an account",
		},
		&types.JSModuleFunc{
			Name:        "getPV",
			Value:       m.getPrivateValidator,
			Description: "Get the private validator information",
		},
		&types.JSModuleFunc{
			Name:        "execSetDelegatorCommissionRate",
			Value:       m.execSetDelegatorCommissionRate,
			Description: "Set the percentage of reward to share with a delegator",
		},
	}
}

func (m *AccountModule) globals() []*types.JSModuleFunc {
	return []*types.JSModuleFunc{
		&types.JSModuleFunc{
			Name:        "accounts",
			Value:       m.listAccounts(),
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
	util.VMSet(m.vm, types.NamespaceAccount, fMap)

	// add namespaced functions
	for _, f := range m.namespacedFuncs() {
		fMap[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", types.NamespaceAccount, f.Name)
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

// listAccounts lists all accounts on this node
func (m *AccountModule) listAccounts() []string {
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

// getKey returns the private key of a given key.
// The passphrase argument is used to unlock the address.
// If passphrase is not set, an interactive prompt will be started
// to collect the passphrase without revealing it in the terminal.
func (m *AccountModule) getKey(address string, passphrase ...string) string {

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

	// Decrypt the account using the passphrase
	if err := acct.Decrypt(pass); err != nil {
		panic(errors.Wrap(err, "failed to unlock account with the provided passphrase"))
	}

	return acct.GetKey().PrivKey().Base58()
}

// getNonce returns the current nonce of an account
func (m *AccountModule) getNonce(address string) string {
	nonce, err := m.service.GetNonce(util.String(address))
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%d", nonce)
}

// getAccount returns the account of the given address
func (m *AccountModule) getAccount(address string, height ...int64) interface{} {
	account := m.logic.AccountKeeper().GetAccount(util.String(address), height...)
	if account.Balance.String() == "0" && account.Nonce == uint64(0) {
		panic(types.ErrAccountUnknown)
	}
	return util.EncodeForJS(account)
}

// getSpendableBalance returns the spendable balance of an account
func (m *AccountModule) getSpendableBalance(address string, height ...int64) interface{} {
	account := m.logic.AccountKeeper().GetAccount(util.String(address), height...)
	if account.Balance.String() == "0" && account.Nonce == uint64(0) {
		panic(types.ErrAccountUnknown)
	}

	curBlockInfo, err := m.logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		panic(errors.Wrap(err, "failed to get current block info"))
	}

	return account.GetSpendableBalance(uint64(curBlockInfo.Height)).String()
}

// getStakedBalance returns the total staked coins of an account
func (m *AccountModule) getStakedBalance(address string, height ...int64) interface{} {
	account := m.logic.AccountKeeper().GetAccount(util.String(address), height...)
	if account.Balance.String() == "0" && account.Nonce == uint64(0) {
		panic(types.ErrAccountUnknown)
	}

	curBlockInfo, err := m.logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		panic(errors.Wrap(err, "failed to get current block info"))
	}

	return account.Stakes.TotalStaked(uint64(curBlockInfo.Height)).String()
}

// getPrivateValidator returns the address, public and private keys of the validator.
// If includePrivKey is true, the private key of the validator
// will be included in the result.
func (m *AccountModule) getPrivateValidator(includePrivKey ...bool) interface{} {
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

// execSetDelegatorCommissionRate sets the delegator commission for an account
func (m *AccountModule) execSetDelegatorCommissionRate(txObj interface{}, options ...interface{}) interface{} {
	var err error
	tx, key := processTxArgs(txObj, options...)
	tx.Type = types.TxTypeSetDelegatorCommission

	// Set tx public key
	pk, _ := crypto.PrivKeyFromBase58(key)
	tx.SetSenderPubKey(util.String(crypto.NewKeyFromPrivKey(pk).PubKey().Base58()))

	// Set timestamp if not already set
	if tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}

	// Set nonce if nonce is not provided
	if tx.Nonce == 0 {
		nonce, err := m.service.GetNonce(tx.GetFrom())
		if err != nil {
			panic("failed to get sender's nonce")
		}
		tx.Nonce = nonce + 1
	}

	// Sign the tx
	tx.Sig, err = tx.Sign(key)
	if err != nil {
		panic(errors.Wrap(err, "failed to sign transaction"))
	}

	// Process the transaction
	hash, err := m.service.SendTx(tx)
	if err != nil {
		panic(errors.Wrap(err, "failed to send transaction"))
	}

	return util.EncodeForJS(map[string]interface{}{
		"hash": hash,
	})
}
