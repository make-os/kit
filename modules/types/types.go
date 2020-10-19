package types

import (
	"github.com/c-bata/go-prompt"
	"github.com/fatih/structs"
	"github.com/make-os/lobe/rpc/types"
	"github.com/make-os/lobe/util"
	"github.com/robertkrimen/otto"
)

// VMMember describes a member function or variable of a VM
type VMMember struct {

	// Name of the member
	Name string

	// Value is the value of the member
	Value interface{}

	// Description is the brief human description of the member
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
	Chain   NodeModule
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
	Dev     DevModule
}

// ConfigureVM applies all modules' VM configurations to the given VM.
func (m *Modules) ConfigureVM(vm *otto.Otto) (completers []prompt.Completer) {
	for _, f := range structs.Fields(m) {
		mod, ok := f.Value().(Module)
		if !ok {
			continue
		}
		completers = append(completers, mod.ConfigureVM(vm))
	}
	return
}

type Module interface {
	ConfigureVM(vm *otto.Otto) prompt.Completer
}

type NodeModule interface {
	Module
	GetBlock(height string) util.Map
	GetHeight() string
	GetBlockInfo(height string) util.Map
	GetValidators(height string) (res []util.Map)
	IsSyncing() bool
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
	GetKeys() []string
	GetPrivKey(address string, passphrase ...string) string
	GetPublicKey(address string, passphrase ...string) string
	GetNonce(address string, height ...uint64) string
	GetAccount(address string, height ...uint64) util.Map
	GetAvailableBalance(address string, height ...uint64) string
	GetStakedBalance(address string, height ...uint64) string
	GetValidator(includePrivKey ...bool) util.Map
	SetCommission(params map[string]interface{}, options ...interface{}) util.Map
	SendCoin(params map[string]interface{}, options ...interface{}) util.Map
}

type PushKeyModule interface {
	Module
	Register(params map[string]interface{}, options ...interface{}) util.Map
	Update(params map[string]interface{}, options ...interface{}) util.Map
	Find(id string, blockHeight ...uint64) util.Map
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
	GetValidatorTicketsByProposer(proposerPubKey string, queryOpts ...util.Map) []util.Map
	GetHostTicketsByProposer(proposerPubKey string, queryOpts ...util.Map) []util.Map
	GetTopValidators(limit ...int) []util.Map
	GetTopHosts(limit ...int) []util.Map
	GetStats(proposerPubKey ...string) (result util.Map)
	GetAll(limit ...int) []util.Map
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
	Vote(params map[string]interface{}, options ...interface{}) util.Map
	Get(name string, opts ...GetOptions) util.Map
	Update(params map[string]interface{}, options ...interface{}) util.Map
	DepositProposalFee(params map[string]interface{}, options ...interface{}) util.Map
	AddContributor(params map[string]interface{}, options ...interface{}) util.Map
	Track(names string, height ...uint64)
	UnTrack(names string)
	GetTracked() util.Map
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
	Lookup(key string) string
	Announce(key string)
	GetRepoObjectProviders(key string) (res []util.Map)
	GetProviders(key string) (res []util.Map)
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
}

type DevModule interface {
	Module
	GetDevUserAccountKey() string
	GetDevUserAddress() string
}

// ModuleCommon provides common module fields and methods.
type ModuleCommon struct {
	// Suggestions contains console suggestions
	Suggestions []prompt.Suggest

	// Client is an RPC client
	Client types.Client
}

// IsAttached checks whether the module is in attach mode.
func (m *ModuleCommon) IsAttached() bool {
	return m.Client != nil
}

// Completer returns suggestions for console input
func (m *ModuleCommon) Completer(d prompt.Document) []prompt.Suggest {
	if words := d.GetWordBeforeCursor(); len(words) > 1 {
		return prompt.FilterHasPrefix(m.Suggestions, words, true)
	}
	return nil
}

const (
	TxStatusInMempool  = "in_mempool"
	TxStatusInPushpool = "in_pushpool"
	TxStatusInBlock    = "in_block"
)
