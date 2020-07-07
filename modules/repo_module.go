package modules

import (
	"fmt"

	"github.com/c-bata/go-prompt"
	"github.com/pkg/errors"
	"github.com/robertkrimen/otto"
	"github.com/spf13/cast"
	"gitlab.com/makeos/mosdef/console"
	modulestypes "gitlab.com/makeos/mosdef/modules/types"
	"gitlab.com/makeos/mosdef/node/services"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/types/txns"
	"gitlab.com/makeos/mosdef/util"
	"gopkg.in/src-d/go-git.v4"
)

// RepoModule provides repository functionalities to JS environment
type RepoModule struct {
	console.ConsoleSuggestions
	logic   core.Logic
	service services.Service
	repoSrv core.RemoteServer
}

// NewRepoModule creates an instance of RepoModule
func NewRepoModule(service services.Service, repoSrv core.RemoteServer, logic core.Logic) *RepoModule {
	return &RepoModule{service: service, logic: logic, repoSrv: repoSrv}
}

// ConsoleOnlyMode indicates that this module can be used on console-only mode
func (m *RepoModule) ConsoleOnlyMode() bool {
	return false
}

// methods are functions exposed in the special namespace of this module.
func (m *RepoModule) methods() []*modulestypes.ModuleFunc {
	return []*modulestypes.ModuleFunc{
		{
			Name:        "create",
			Value:       m.Create,
			Description: "Create a git repository on the network",
		},
		{
			Name:        "get",
			Value:       m.Get,
			Description: "Get and return a repository",
		},
		{
			Name:        "update",
			Value:       m.Update,
			Description: "Update a repository",
		},
		{
			Name:        "prune",
			Value:       m.Prune,
			Description: "Delete all dangling and unreachable loose objects from a repository",
		},
		{
			Name:        "upsertOwner",
			Value:       m.UpsertOwner,
			Description: "Create a proposal to add or update a repository owner",
		},
		{
			Name:        "vote",
			Value:       m.VoteOnProposal,
			Description: "Vote for or against a proposal",
		},
		{
			Name:        "depositFee",
			Value:       m.DepositFee,
			Description: "Deposit fees into a proposal",
		},
		{
			Name:        "addContributor",
			Value:       m.Register,
			Description: "Register one or more contributors",
		},
		{
			Name:        "announce",
			Value:       m.AnnounceObjects,
			Description: "Announce commit and tag objects of a repository",
		},
	}
}

// globals are functions exposed in the VM's global namespace
func (m *RepoModule) globals() []*modulestypes.ModuleFunc {
	return []*modulestypes.ModuleFunc{}
}

// ConfigureVM configures the JS context and return
// any number of console prompt suggestions
func (m *RepoModule) ConfigureVM(vm *otto.Otto) prompt.Completer {

	// Register the main namespace
	obj := map[string]interface{}{}
	util.VMSet(vm, constants.NamespaceRepo, obj)

	for _, f := range m.methods() {
		obj[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", constants.NamespaceRepo, f.Name)
		m.Suggestions = append(m.Suggestions, prompt.Suggest{Text: funcFullName,
			Description: f.Description})
	}

	// Register global functions
	for _, f := range m.globals() {
		vm.Set(f.Name, f.Value)
		m.Suggestions = append(m.Suggestions, prompt.Suggest{Text: f.Name,
			Description: f.Description})
	}

	return m.Completer
}

// create registers a git repository on the network
//
// ARGS:
// params <map>
// params.name <string>:					The name of the namespace
// params.value <string>:					The amount to pay for initial resources
// params.nonce <number|string>: 			The senders next account nonce
// params.fee <number|string>: 				The transaction fee to pay
// params.timestamp <number>: 				The unix timestamp
// params.config <map|hex-msgpack-string>  	The repo configuration
// params.sig <hex-string>					The transaction signature
//
// options <[]interface{}>
// options[0] key <string>: 				The signer's private key
// options[1] payloadOnly <bool>: 			When true, returns the payload only, without sending the tx.
//
// RETURNS object <map>
// object.hash <string>: 					The transaction hash
// object.address <string: 					The address of the repository
func (m *RepoModule) Create(params map[string]interface{}, options ...interface{}) util.Map {
	var err error

	var tx = txns.NewBareTxRepoCreate()
	if err = tx.FromMap(params); err != nil {
		panic(se(400, StatusCodeInvalidParam, "params", err.Error()))
	}

	if finalizeTx(tx, m.logic, options...) {
		return tx.ToMap()
	}

	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		panic(se(400, StatusCodeMempoolAddFail, "", err.Error()))
	}

	return map[string]interface{}{
		"hash":    hash,
		"address": fmt.Sprintf("r/%s", tx.Name),
	}
}

// upsertOwner creates a proposal to add or update a repository owner
//
// ARGS:
// params <map>
// params.id 		<string>: 					A unique proposal id
// params.addresses <string>: 					A comma separated list of addresses
// params.veto 		<bool>: 					The senders next account nonce
// params.fee 		<number|string>: 			The transaction fee to pay
// params.timestamp <number>: 					The unix timestamp
//
// options <[]interface{}>
// options[0] key <string>: 					The signer's private key
// options[1] payloadOnly <bool>: 				When true, returns the payload only, without sending the tx.
//
// RETURNS <map>: 								When payloadOnly is false
// hash <string>: 								The transaction hash
//
// RETURNS <core.TxRepoProposalUpsertOwner>: 	When payloadOnly is true, returns signed transaction object
func (m *RepoModule) UpsertOwner(params map[string]interface{}, options ...interface{}) util.Map {
	var err error

	var tx = txns.NewBareRepoProposalUpsertOwner()
	if err = tx.FromMap(params); err != nil {
		panic(se(400, StatusCodeInvalidParam, "params", err.Error()))
	}

	if finalizeTx(tx, m.logic, options...) {
		return tx.ToMap()
	}

	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		panic(se(400, StatusCodeMempoolAddFail, "", err.Error()))
	}

	return map[string]interface{}{
		"hash": hash,
	}
}

// voteOnProposal sends a TxTypeRepoCreate transaction to create a git repository
//
// ARGS:
// params <map>
// params.id 		<string>: 			The proposal ID to vote on
// params.name 		<string>: 			The name of the repository
// params.vote 		<uint>: 			The vote choice (1) yes (0) no (-1) vote no with veto (-2) abstain
// params.nonce 	<number|string>: 	The senders next account nonce
// params.fee 		<number|string>: 	The transaction fee to pay
// params.timestamp <number>: 			The unix timestamp
//
// options <[]interface{}>
// options[0] key <string>: 			The signer's private key
// options[1] payloadOnly <bool>: 		When true, returns the payload only, without sending the tx.
//
// RETURNS object <map>
// object.hash <string>: 				The transaction hash
func (m *RepoModule) VoteOnProposal(params map[string]interface{}, options ...interface{}) util.Map {
	var err error

	var tx = txns.NewBareRepoProposalVote()
	if err = tx.FromMap(params); err != nil {
		panic(se(400, StatusCodeInvalidParam, "params", err.Error()))
	}

	if finalizeTx(tx, m.logic, options...) {
		return tx.ToMap()
	}

	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		panic(se(400, StatusCodeMempoolAddFail, "", err.Error()))
	}

	return map[string]interface{}{
		"hash": hash,
	}
}

// prune removes dangling or unreachable objects from a repository.
//
// ARGS:
// name: The name of the repository
// force: When true, forcefully prunes the target repository
func (m *RepoModule) Prune(name string, force bool) {
	if force {
		if err := m.repoSrv.GetPruner().Prune(name, true); err != nil {
			panic(util.ReqErr(500, StatusCodeServerErr, "", err.Error()))
		}
		return
	}
	m.repoSrv.GetPruner().Schedule(name)
}

// Get finds and returns a repository.
//
// ARGS:
// name: The name of the repository
//
// opts <map>: fetch options
// opts.height: Query a specific block
// opts.noProps: When true, the result will not include proposals
//
// RETURNS <map>
func (m *RepoModule) Get(name string, opts ...modulestypes.GetOptions) util.Map {
	var blockHeight uint64
	var noProposals bool
	var err error

	if len(opts) > 0 {
		opt := opts[0]
		noProposals = opt.NoProposals
		if opt.Height != nil {
			blockHeight, err = cast.ToUint64E(opt.Height)
			if err != nil {
				panic(se(400, StatusCodeInvalidParam, "opts.height", "unexpected type"))
			}
		}
	}

	var repo *state.Repository
	if !noProposals {
		repo = m.logic.RepoKeeper().Get(name, blockHeight)
	} else {
		repo = m.logic.RepoKeeper().GetNoPopulate(name, blockHeight)
		repo.Proposals = state.RepoProposals{}
	}

	if repo.IsNil() {
		panic(se(404, StatusCodeRepoNotFound, "name", types.ErrRepoNotFound.Error()))
	}

	return util.ToMap(repo)
}

// Update creates a proposal to update a repository
//
// ARGS:
// params <map>
// params.name 		<string>: 				The name of the repository
// params.id 		<string>: 				A unique proposal ID
// params.value 	<string|number>:		The proposal fee
// params.config 	<map[string]string>: 	The updated repository config
// params.nonce 	<number|string>: 		The senders next account nonce
// params.fee 		<number|string>: 		The transaction fee to pay
// params.timestamp <number>: 				The unix timestamp
//
// options <[]interface{}>
// options[0] key <string>: 				The signer's private key
// options[1] payloadOnly <bool>: 			When true, returns the payload only, without sending the tx.
//
// RETURNS object <map>
// object.hash <string>: 					The transaction hash
func (m *RepoModule) Update(params map[string]interface{}, options ...interface{}) util.Map {
	var err error

	var tx = txns.NewBareRepoProposalUpdate()
	if err = tx.FromMap(params); err != nil {
		panic(se(400, StatusCodeInvalidParam, "params", err.Error()))
	}

	if finalizeTx(tx, m.logic, options...) {
		return tx.ToMap()
	}

	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		panic(se(400, StatusCodeMempoolAddFail, "", err.Error()))
	}

	return map[string]interface{}{
		"hash": hash,
	}
}

// DepositFee creates a transaction to deposit a fee to a proposal
//
// ARGS:
// params <map>
// params.name 		<string>: 				The name of the repository
// params.id 		<string>: 				A unique proposal ID
// params.value 	<string|number>:		The amount to add
// params.nonce 	<number|string>: 		The senders next account nonce
// params.fee 		<number|string>: 		The transaction fee to pay
// params.timestamp <number>: 				The unix timestamp
//
// options <[]interface{}>
// options[0] key <string>: 				The signer's private key
// options[1] payloadOnly <bool>: 			When true, returns the payload only, without sending the tx.
//
// RETURNS object <map>
// object.hash <string>: 					The transaction hash
func (m *RepoModule) DepositFee(params map[string]interface{}, options ...interface{}) util.Map {
	var err error

	var tx = txns.NewBareRepoProposalFeeSend()
	if err = tx.FromMap(params); err != nil {
		panic(se(400, StatusCodeInvalidParam, "params", err.Error()))
	}

	if finalizeTx(tx, m.logic, options...) {
		return tx.ToMap()
	}

	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		panic(se(400, StatusCodeMempoolAddFail, "", err.Error()))
	}

	return map[string]interface{}{
		"hash": hash,
	}
}

// Register creates a proposal to register one or more push keys
//
// ARGS:
// params <map>
// params.name 					<string>: 						The name of the repository
// params.id 					<string>: 						A unique proposal ID
// params.ids 					<string|[]string>: 				A list or comma separated list of push key IDs to add
// params.policies 				<[]map[string]interface{}>: 	A list of policies
//	- sub 						<string>:						The policy's subject
//	- obj 						<string>:						The policy's object
//	- act 						<string>:						The policy's action
// params.value 				<string|number>:				The proposal fee to pay
// params.nonce 				<number|string>: 				The senders next account nonce
// params.fee 					<number|string>: 				The transaction fee to pay
// params.timestamp 			<number>: 						The unix timestamp
// params.namespace 			<string>: 						A namespace to also register the key to
// params.namespaceOnly 		<string>: 						Like namespace but key will not be registered to the repo.
//
// options 			<[]interface{}>
// options[0] 		key <string>: 					The signer's private key
// options[1] 		payloadOnly <bool>: 			When true, returns the payload only, without sending the tx.
//
// RETURNS object <map>
// object.hash <string>: 							The transaction hash
func (m *RepoModule) Register(params map[string]interface{}, options ...interface{}) util.Map {
	var err error

	var tx = txns.NewBareRepoProposalRegister()
	if err = tx.FromMap(params); err != nil {
		panic(se(400, StatusCodeInvalidParam, "params", err.Error()))
	}

	if finalizeTx(tx, m.logic, options...) {
		return tx.ToMap()
	}

	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		panic(se(400, StatusCodeMempoolAddFail, "", err.Error()))
	}

	return map[string]interface{}{
		"hash": hash,
	}
}

// AnnounceObjects announces commits and tags of a repository
//
// ARGS:
// repoName: The name of the target repository
func (m *RepoModule) AnnounceObjects(repoName string) {
	err := m.logic.GetRemoteServer().AnnounceRepoObjects(repoName)
	if err != nil {
		if errors.Cause(err) == git.ErrRepositoryNotExists {
			panic(se(404, StatusCodeRepoNotFound, "repoName", err.Error()))
		}
		panic(se(500, StatusCodeServerErr, "", err.Error()))
	}
}
