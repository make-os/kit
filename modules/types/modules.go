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
	ConfigureVM(vm *otto.Otto) []prompt.Suggest

	// GetModules returns modules
	GetModules() *Modules

	// CreateNewModules creates and returns a new Modules instance
	CreateNewModules() *Modules
}

// DefaultCallContext is the default module context
var DefaultModuleContext = &ModulesContext{Env: JS}

// ModulesContext contains common configuration accessible to all modules.
type ModulesContext struct {
	Env Env
}

// Modules contains all supported modules
type Modules struct {
	Tx      TxModule
	Chain   ChainModule
	Pool    PoolModule
	Account AccountModule
	PushKey PushKeyModule
	Util    UtilModule
	Ticket  TicketModule
	Repo    RepoModule
	NS      NamespaceModule
	DHT     DHTModule
	ExtMgr  ExtManager
	RPC     RPCModule
}

// SetContext provides all modules with a function for getting the call context.
func (m *Modules) SetContext(ctx *ModulesContext) {
	for _, f := range structs.Fields(m) {
		mod, ok := f.Value().(Module)
		if !ok {
			continue
		}
		mod.SetContext(ctx)
	}
}

// ConfigureVM applies all modules' VM configurations to the given VM.
func (m *Modules) ConfigureVM(vm *otto.Otto, consoleOnly bool) (sugs []prompt.Suggest) {
	for _, f := range structs.Fields(m) {
		mod, ok := f.Value().(Module)
		if !ok {
			continue
		}
		if !consoleOnly {
			sugs = append(sugs, mod.ConfigureVM(vm)...)
			continue
		}

		if mod.ConsoleOnlyMode() {
			sugs = append(sugs, mod.ConfigureVM(vm)...)
		}
	}
	return
}

type CallContextGetter func() *ModulesContext

type Module interface {
	ConfigureVM(vm *otto.Otto) []prompt.Suggest
	ConsoleOnlyMode() bool
	SetContext(*ModulesContext)
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
	SendCoin(params map[string]interface{}, options ...interface{}) util.Map
	Get(hash string) util.Map
	SendPayload(params map[string]interface{}) util.Map
}

type PoolModule interface {
	Module
	GetSize() util.Map
	GetTop(n int) []util.Map
	GetPushPoolSize() int
}

type AccountModule interface {
	Module
	ListLocalAccounts() []string
	GetKey(address string, passphrase ...string) string
	GetPublicKey(address string, passphrase ...string) string
	GetNonce(address string, height ...uint64) string
	GetAccount(address string, height ...uint64) util.Map
	GetSpendableBalance(address string, height ...uint64) string
	GetStakedBalance(address string, height ...uint64) string
	GetPrivateValidator(includePrivKey ...bool) util.Map
	SetCommission(params map[string]interface{}, options ...interface{}) util.Map
}

type PushKeyModule interface {
	Module
	Register(params map[string]interface{}, options ...interface{}) util.Map
	Get(id string, blockHeight ...uint64) util.Map
	GetByAddress(address string) []string
	GetAccountOfOwner(gpgID string, blockHeight ...uint64) util.Map
}

type UtilModule interface {
	Module
	TreasuryAddress() string
	GenKey(seed ...int64) interface{}
}

type TicketModule interface {
	Module
	BuyValidatorTicket(params map[string]interface{}, options ...interface{}) util.Map
	BuyHostTicket(params map[string]interface{}, options ...interface{}) interface{}
	ListProposerValidatorTickets(proposerPubKey string, queryOpts ...map[string]interface{}) []util.Map
	ListProposerHostTickets(proposerPubKey string, queryOpts ...map[string]interface{}) interface{}
	ListTopValidators(limit ...int) interface{}
	ListTopHosts(limit ...int) interface{}
	TicketStats(proposerPubKey ...string) (result util.Map)
	ListRecent(limit ...int) []util.Map
	UnbondHostTicket(params map[string]interface{}, options ...interface{}) interface{}
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
}
type NamespaceModule interface {
	Module
	Lookup(name string, height ...uint64) interface{}
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
	Local() util.Map
	Connect(host string, port int, https bool, user, pass string) util.Map
}

// Env describes environment a module can be called from.
type Env int

const (

	// JS represents javascript-like environment that cannot handle big integers
	// natively and require external packages to for big integers.
	JS Env = iota

	// NORMAL represents an environment that can handle big numbers
	// and buffers natively without needing external packages.
	NORMAL
)
