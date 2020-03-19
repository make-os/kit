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
	ConfigureVM(vm *otto.Otto) []prompt.Suggest
	GetModules() *Modules
}

// Modules contains all supported modules
type Modules struct {
	Tx      TxModule
	Chain   ChainModule
	Pool    PoolModule
	Account AccountModule
	PushKey GPGModule
	Util    UtilModule
	Ticket  TicketModule
	Repo    RepoModule
	NS      NamespaceModule
	DHT     DHTModule
	ExtMgr  ExtManager
	RPC     RPCModule
}

type ChainModule interface {
	Configure() []prompt.Suggest
	GetBlock(height string) util.Map
	GetCurrentHeight() util.Map
	GetBlockInfo(height string) util.Map
	GetValidators(height string) (res []util.Map)
}

type TxModule interface {
	Configure() []prompt.Suggest
	SendCoin(params map[string]interface{}, options ...interface{}) util.Map
	Get(hash string) util.Map
	SendPayload(params map[string]interface{}) util.Map
}

type PoolModule interface {
	Configure() []prompt.Suggest
	GetSize() util.Map
	GetTop(n int) []util.Map
	GetPushPoolSize() int
}

type AccountModule interface {
	Configure() []prompt.Suggest
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

type GPGModule interface {
	Configure() []prompt.Suggest
	Register(params map[string]interface{}, options ...interface{}) util.Map
	Get(id string, blockHeight ...uint64) util.Map
	GetByAddress(address string) []string
	GetAccountOfOwner(gpgID string, blockHeight ...uint64) util.Map
}

type UtilModule interface {
	Configure() []prompt.Suggest
	TreasuryAddress() string
	GenKey(seed ...int64) interface{}
}

type TicketModule interface {
	Configure() []prompt.Suggest
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
	Configure() []prompt.Suggest
	Create(params map[string]interface{}, options ...interface{}) interface{}
	UpsertOwner(params map[string]interface{}, options ...interface{}) util.Map
	VoteOnProposal(params map[string]interface{}, options ...interface{}) util.Map
	Prune(name string, force bool)
	Get(name string, opts ...map[string]interface{}) util.Map
	Update(params map[string]interface{}, options ...interface{}) util.Map
	DepositFee(params map[string]interface{}, options ...interface{}) util.Map
	CreateMergeRequest(params map[string]interface{}, options ...interface{}) interface{}
}
type NamespaceModule interface {
	Configure() []prompt.Suggest
	Lookup(name string, height ...uint64) interface{}
	GetTarget(path string, height ...uint64) string
	Register(params map[string]interface{}, options ...interface{}) interface{}
	UpdateDomain(params map[string]interface{}, options ...interface{}) interface{}
}

type DHTModule interface {
	Configure() []prompt.Suggest
	Store(key string, val string)
	Lookup(key string) interface{}
	Announce(key string)
	GetProviders(key string) (res []map[string]interface{})
	GetRepoObject(objURI string) []byte
	GetPeers() []string
}

type ExtManager interface {
	Configure() []prompt.Suggest
	SetVM(vm *otto.Otto) ExtManager
	Exist(name string) bool
	Installed() (extensions []string)
	Load(name string, args ...map[string]string) map[string]interface{}
	Run(name string, args ...map[string]string) map[string]interface{}
	Stop(name string)
	Running() []string
	IsRunning(name string) bool
}

type RPCModule interface {
	Configure() []prompt.Suggest
	IsRunning() bool
	Local() util.Map
	Connect(host string, port int, https bool, user, pass string) util.Map
}
