package modules

import (
	"github.com/c-bata/go-prompt"
	"github.com/robertkrimen/otto"
	"gitlab.com/makeos/mosdef/util"
)

// ModuleFunc describes a module function
type ModuleFunc struct {
	Name        string
	Value       interface{}
	Description string
}

// ModuleHub describes a mechanism for aggregating, configuring and
// accessing modules that provide uniform functionalities in JS environment,
// JSON-RPC APIs and REST APIs
type ModuleHub interface {
	// ConfigureVM instructs VM-accessible modules accessible to configure the VM
	ConfigureVM(vm *otto.Otto) []prompt.Suggest

	// GetModules returns all modules
	GetModules() *Modules
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

type Module interface {
	ConfigureVM(vm *otto.Otto) []prompt.Suggest
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
	Buy(params map[string]interface{}, options ...interface{}) interface{}
	HostBuy(params map[string]interface{}, options ...interface{}) interface{}
	ListValidatorTicketsOfProposer(proposerPubKey string, queryOpts ...map[string]interface{}) []util.Map
	ListHostTicketsOfProposer(proposerPubKey string, queryOpts ...map[string]interface{}) interface{}
	ListTopValidators(limit ...int) interface{}
	ListTopHosts(limit ...int) interface{}
	TicketStats(proposerPubKey ...string) (result util.Map)
	ListRecent(limit ...int) []util.Map
	UnbondHostTicket(params map[string]interface{}, options ...interface{}) interface{}
}

type RepoModule interface {
	Module
	Create(params map[string]interface{}, options ...interface{}) interface{}
	UpsertOwner(params map[string]interface{}, options ...interface{}) util.Map
	VoteOnProposal(params map[string]interface{}, options ...interface{}) util.Map
	Prune(name string, force bool)
	Get(name string, opts ...map[string]interface{}) util.Map
	Update(params map[string]interface{}, options ...interface{}) util.Map
	DepositFee(params map[string]interface{}, options ...interface{}) util.Map
}
type NamespaceModule interface {
	Module
	Lookup(name string, height ...uint64) interface{}
	GetTarget(path string, height ...uint64) string
	Register(params map[string]interface{}, options ...interface{}) interface{}
	UpdateDomain(params map[string]interface{}, options ...interface{}) interface{}
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
