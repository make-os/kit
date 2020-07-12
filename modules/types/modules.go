package types

import (
	"github.com/c-bata/go-prompt"
	"github.com/fatih/structs"
	"github.com/robertkrimen/otto"
	"gitlab.com/makeos/mosdef/util"
)

// ModuleFunc describes a module function
type ModuleFunc struct {
	Name        string
	Value       interface{}
	Description string
}

// ModulesHub describes a mechanism for aggregating, configuring and
// accessing modules that provide uniform functionalities in JS environment,
// JSON-RPC APIs and REST APIs
type ModulesHub interface {
	// ConfigureVM instructs VM-accessible modules accessible to configure the VM
	ConfigureVM(vm *otto.Otto) []prompt.Completer

	// GetModules returns modules
	GetModules() *Modules
}

// Modules contains all supported modules
type Modules struct {
	Tx      TxModule
	Chain   ChainModule
	Pool    PoolModule
	User    UserModule
	PushKey PushKeyModule
	Util    ConsoleUtilModule
	Ticket  TicketModule
	Repo    RepoModule
	NS      NamespaceModule
	DHT     DHTModule
	ExtMgr  ExtManager
	RPC     RPCModule
}

// ConfigureVM applies all modules' VM configurations to the given VM.
func (m *Modules) ConfigureVM(vm *otto.Otto, consoleOnly bool) (completers []prompt.Completer) {
	for _, f := range structs.Fields(m) {
		mod, ok := f.Value().(Module)
		if !ok {
			continue
		}
		if !consoleOnly {
			completers = append(completers, mod.ConfigureVM(vm))
			continue
		}

		if mod.ConsoleOnlyMode() {
			completers = append(completers, mod.ConfigureVM(vm))
		}
	}
	return
}

type Module interface {
	ConfigureVM(vm *otto.Otto) prompt.Completer
	ConsoleOnlyMode() bool
}

type ChainModule interface {
	Module
	GetBlock(height string) util.Map
	GetCurrentHeight() util.Map
	GetBlockInfo(height string) util.Map
	GetValidators(height string) (res []util.Map)
}

type TxModule interface {
	Module
	Get(hash string) util.Map
	SendPayload(params map[string]interface{}) util.Map
}

type PoolModule interface {
	Module
	GetSize() util.Map
	GetTop(n int) []util.Map
	GetPushPoolSize() int
}

type UserModule interface {
	Module
	ListLocalAccounts() []string
	GetKey(address string, passphrase ...string) string
	GetPublicKey(address string, passphrase ...string) string
	GetNonce(address string, height ...uint64) string
	GetAccount(address string, height ...uint64) util.Map
	GetAvailableBalance(address string, height ...uint64) string
	GetStakedBalance(address string, height ...uint64) string
	GetValidatorInfo(includePrivKey ...bool) util.Map
	SetCommission(params map[string]interface{}, options ...interface{}) util.Map
	SendCoin(params map[string]interface{}, options ...interface{}) util.Map
}

type PushKeyModule interface {
	Module
	Register(params map[string]interface{}, options ...interface{}) util.Map
	Get(id string, blockHeight ...uint64) util.Map
	Unregister(params map[string]interface{}, options ...interface{}) util.Map
	GetByAddress(address string) []string
	GetAccountOfOwner(gpgID string, blockHeight ...uint64) util.Map
}

type ConsoleUtilModule interface {
	Module
	PrettyPrint(values ...interface{})
	Dump(objs ...interface{})
	Diff(a, b interface{})
	Eval(src interface{}) otto.Value
	EvalFile(file string) otto.Value
	ReadFile(filename string) []byte
	ReadTextFile(filename string) string
	TreasuryAddress() string
	GenKey(seed ...int64) util.Map
}

type TicketModule interface {
	Module
	BuyValidatorTicket(params map[string]interface{}, options ...interface{}) util.Map
	BuyHostTicket(params map[string]interface{}, options ...interface{}) util.Map
	ListValidatorTicketsByProposer(proposerPubKey string, queryOpts ...map[string]interface{}) []util.Map
	ListHostTicketsByProposer(proposerPubKey string, queryOpts ...map[string]interface{}) []util.Map
	ListTopValidators(limit ...int) []util.Map
	ListTopHosts(limit ...int) []util.Map
	TicketStats(proposerPubKey ...string) (result util.Map)
	ListRecent(limit ...int) []util.Map
	UnbondHostTicket(params map[string]interface{}, options ...interface{}) util.Map
}

type GetOptions struct {
	Height      interface{}
	NoProposals bool
}

type RepoModule interface {
	Module
	Create(params map[string]interface{}, options ...interface{}) util.Map
	UpsertOwner(params map[string]interface{}, options ...interface{}) util.Map
	VoteOnProposal(params map[string]interface{}, options ...interface{}) util.Map
	Prune(name string, force bool)
	Get(name string, opts ...GetOptions) util.Map
	Update(params map[string]interface{}, options ...interface{}) util.Map
	DepositFee(params map[string]interface{}, options ...interface{}) util.Map
	AddContributor(params map[string]interface{}, options ...interface{}) util.Map
}
type NamespaceModule interface {
	Module
	Lookup(name string, height ...uint64) util.Map
	GetTarget(path string, height ...uint64) string
	Register(params map[string]interface{}, options ...interface{}) util.Map
	UpdateDomain(params map[string]interface{}, options ...interface{}) util.Map
}

type DHTModule interface {
	Module
	Store(key string, val string)
	Lookup(key string) interface{}
	Announce(key string)
	GetRepoObjectProviders(key string) (res []map[string]interface{})
	GetProviders(key string) (res []map[string]interface{})
	GetPeers() []string
}

type ExtManager interface {
	Module
	Exist(name string) bool
	Installed() (extensions []string)
	Load(name string, args ...map[string]string) map[string]interface{}
	Run(name string, args ...map[string]string) map[string]interface{}
	Stop(name string)
	Running() []string
	IsRunning(name string) bool
}

type RPCModule interface {
	Module
	IsRunning() bool
	ConnectLocal() util.Map
}

// ConsoleSuggestions provides functionalities for providing the console with suggestions.
// It is meant to be embedded in a module to allow it handle console suggestion provisioning.
type ConsoleSuggestions struct {
	Suggestions []prompt.Suggest
}

// Completer returns suggestions for console input
func (m *ConsoleSuggestions) Completer(d prompt.Document) []prompt.Suggest {
	if words := d.GetWordBeforeCursor(); len(words) > 1 {
		return prompt.FilterHasPrefix(m.Suggestions, words, true)
	}
	return nil
}
