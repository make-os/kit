package modules

import (
	"fmt"

	"github.com/c-bata/go-prompt"
	"github.com/pkg/errors"
	"github.com/robertkrimen/otto"
	"gitlab.com/makeos/mosdef/node/services"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/modules"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/types/txns"
	"gitlab.com/makeos/mosdef/util"
	"gopkg.in/src-d/go-git.v4"
)

// RepoModule provides repository functionalities to JS environment
type RepoModule struct {
	logic   core.Logic
	service services.Service
	repoMgr core.RemoteServer
}

// NewRepoModule creates an instance of RepoModule
func NewRepoModule(service services.Service, repoMgr core.RemoteServer, logic core.Logic) *RepoModule {
	return &RepoModule{service: service, logic: logic, repoMgr: repoMgr}
}

// ConsoleOnlyMode indicates that this module can be used on console-only mode
func (m *RepoModule) ConsoleOnlyMode() bool {
	return false
}

// methods are functions exposed in the special namespace of this module.
func (m *RepoModule) methods() []*modules.ModuleFunc {
	return []*modules.ModuleFunc{
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
			Description: "Register fees to a deposit-enabled repository proposal",
		},
		{
			Name:        "addContributor",
			Value:       m.RegisterPushKey,
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
func (m *RepoModule) globals() []*modules.ModuleFunc {
	return []*modules.ModuleFunc{}
}

// ConfigureVM configures the JS context and return
// any number of console prompt suggestions
func (m *RepoModule) ConfigureVM(vm *otto.Otto) []prompt.Suggest {
	var suggestions []prompt.Suggest

	// Register the main namespace
	obj := map[string]interface{}{}
	util.VMSet(vm, constants.NamespaceRepo, obj)

	for _, f := range m.methods() {
		obj[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", constants.NamespaceRepo, f.Name)
		suggestions = append(suggestions, prompt.Suggest{Text: funcFullName,
			Description: f.Description})
	}

	// Register global functions
	for _, f := range m.globals() {
		vm.Set(f.Name, f.Value)
		suggestions = append(suggestions, prompt.Suggest{Text: f.Name,
			Description: f.Description})
	}

	return suggestions
}

// create registers a git repository on the network
//
// ARGS:
// params <map>
// params.name <string>:				The name of the namespace
// params.value <string>:				The amount to pay for initial resources
// params.nonce <number|string>: 		The senders next account nonce
// params.fee <number|string>: 			The transaction fee to pay
// params.timestamp <number>: 			The unix timestamp
//
// options <[]interface{}>
// options[0] key <string>: 			The signer's private key
// options[1] payloadOnly <bool>: 		When true, returns the payload only, without sending the tx.
//
// RETURNS object <map>
// object.hash <string>: 				The transaction hash
// object.address <string: 				The address of the repository
func (m *RepoModule) Create(params map[string]interface{}, options ...interface{}) interface{} {
	var err error

	var tx = txns.NewBareTxRepoCreate()
	if err = tx.FromMap(params); err != nil {
		panic(util.NewStatusError(400, StatusCodeInvalidParam, "params", err.Error()))
	}

	payloadOnly := finalizeTx(tx, m.logic, options...)
	if payloadOnly {
		return EncodeForJS(tx.ToMap())
	}

	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		panic(util.NewStatusError(400, StatusCodeMempoolAddFail, "", err.Error()))
	}

	return EncodeForJS(map[string]interface{}{
		"hash":    hash,
		"address": fmt.Sprintf("r/%s", tx.Name),
	})
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
		panic(util.NewStatusError(400, StatusCodeInvalidParam, "params", err.Error()))
	}

	payloadOnly := finalizeTx(tx, m.logic, options...)
	if payloadOnly {
		return EncodeForJS(tx.ToMap())
	}

	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		panic(util.NewStatusError(400, StatusCodeMempoolAddFail, "", err.Error()))
	}

	return EncodeForJS(map[string]interface{}{
		"hash": hash,
	})
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
		panic(util.NewStatusError(400, StatusCodeInvalidParam, "params", err.Error()))
	}

	payloadOnly := finalizeTx(tx, m.logic, options...)
	if payloadOnly {
		return EncodeForJS(tx.ToMap())
	}

	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		panic(util.NewStatusError(400, StatusCodeMempoolAddFail, "", err.Error()))
	}

	return EncodeForJS(map[string]interface{}{
		"hash": hash,
	})
}

// prune removes dangling or unreachable objects from a repository.
//
// ARGS:
// name: The name of the repository
// force: When true, forcefully prunes the target repository
func (m *RepoModule) Prune(name string, force bool) {
	if force {
		if err := m.repoMgr.GetPruner().Prune(name, true); err != nil {
			panic(err)
		}
		return
	}
	m.repoMgr.GetPruner().Schedule(name)
}

// get finds and returns a repository. Panic if not found.
//
// ARGS:
// name: The name of the repository
//
// opts <map>: fetch options
// opts.height: Query a specific block
// opts.noProps: When true, the result will not include proposals
//
// RETURNS <map>
func (m *RepoModule) Get(name string, opts ...map[string]interface{}) util.Map {
	var targetHeight uint64
	var noProposals bool

	if len(opts) > 0 {
		opt := opts[0]
		if height, ok := opt["height"].(int64); ok {
			targetHeight = uint64(height)
		}
		if noProps, ok := opt["noProps"].(bool); ok {
			noProposals = noProps
		}
	}

	var repo *state.Repository
	if !noProposals {
		repo = m.logic.RepoKeeper().Get(name, targetHeight)
	} else {
		repo = m.logic.RepoKeeper().GetNoPopulate(name, targetHeight)
		repo.Proposals = state.RepoProposals{}
	}

	if repo.IsNil() {
		panic(util.NewStatusError(404, StatusCodeRepoNotFound, "name", types.ErrRepoNotFound.Error()))
	}

	return EncodeForJS(repo)
}

// update creates a proposal to update a repository
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
		panic(util.NewStatusError(400, StatusCodeInvalidParam, "params", err.Error()))
	}

	payloadOnly := finalizeTx(tx, m.logic, options...)
	if payloadOnly {
		return EncodeForJS(tx.ToMap())
	}

	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		panic(util.NewStatusError(400, StatusCodeMempoolAddFail, "", err.Error()))
	}

	return EncodeForJS(map[string]interface{}{
		"hash": hash,
	})
}

// depositFee creates a transaction to deposit a fee to a proposal
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
		panic(util.NewStatusError(400, StatusCodeInvalidParam, "params", err.Error()))
	}

	payloadOnly := finalizeTx(tx, m.logic, options...)
	if payloadOnly {
		return EncodeForJS(tx.ToMap())
	}

	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		panic(util.NewStatusError(400, StatusCodeMempoolAddFail, "", err.Error()))
	}

	return EncodeForJS(map[string]interface{}{
		"hash": hash,
	})
}

// RegisterPushKey creates a proposal to register one or more push keys
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
func (m *RepoModule) RegisterPushKey(
	params map[string]interface{},
	options ...interface{}) interface{} {
	var err error

	var tx = txns.NewBareRepoProposalRegisterPushKey()
	if err = tx.FromMap(params); err != nil {
		panic(util.NewStatusError(400, StatusCodeInvalidParam, "params", err.Error()))
	}

	payloadOnly := finalizeTx(tx, m.logic, options...)
	if payloadOnly {
		return EncodeForJS(tx.ToMap())
	}

	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		panic(util.NewStatusError(400, StatusCodeMempoolAddFail, "", err.Error()))
	}

	return EncodeForJS(map[string]interface{}{
		"hash": hash,
	})
}

// AnnounceObjects announces commits and tags of a repository
//
// ARGS:
// repoName: The name of the target repository
func (m *RepoModule) AnnounceObjects(repoName string) {
	err := m.logic.GetRemoteServer().AnnounceRepoObjects(repoName)
	if err != nil {
		if errors.Cause(err) == git.ErrRepositoryNotExists {
			panic(util.NewStatusError(404, StatusCodeRepoNotFound, "repoName", err.Error()))
		}
		panic(util.NewStatusError(500, StatusCodeAppErr, "", err.Error()))
	}
}
