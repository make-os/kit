package modules

import (
	"fmt"

	prompt "github.com/c-bata/go-prompt"
	"github.com/pkg/errors"
	"github.com/robertkrimen/otto"
	modtypes "gitlab.com/makeos/mosdef/modules/types"
	"gitlab.com/makeos/mosdef/node/services"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/util"
)

// RepoModule provides repository functionalities to JS environment
type RepoModule struct {
	vm      *otto.Otto
	logic   core.Logic
	service services.Service
	repoMgr core.RepoManager
}

// NewRepoModule creates an instance of RepoModule
func NewRepoModule(
	vm *otto.Otto,
	service services.Service,
	repoMgr core.RepoManager,
	logic core.Logic) *RepoModule {
	return &RepoModule{vm: vm, service: service, logic: logic, repoMgr: repoMgr}
}

// funcs are functions accessible using the `repo` namespace
func (m *RepoModule) funcs() []*modtypes.ModulesAggregatorFunc {
	return []*modtypes.ModulesAggregatorFunc{
		{
			Name:        "create",
			Value:       m.create,
			Description: "Create a git repository on the network",
		},
		{
			Name:        "get",
			Value:       m.get,
			Description: "Find and return a repository",
		},
		{
			Name:        "update",
			Value:       m.update,
			Description: "Update a repository",
		},
		{
			Name:        "prune",
			Value:       m.prune,
			Description: "Delete all dangling and unreachable loose objects from a repository",
		},
		{
			Name:        "upsertOwner",
			Value:       m.upsertOwner,
			Description: "Create a proposal to add or update a repository owner",
		},
		{
			Name:        "vote",
			Value:       m.voteOnProposal,
			Description: "Vote for or against a proposal",
		},
		{
			Name:        "depositFee",
			Value:       m.depositFee,
			Description: "Add fees to a deposit-enabled repository proposal",
		},
		{
			Name:        "createMergeRequest",
			Value:       m.CreateMergeRequest,
			Description: "Create a merge request proposal",
		},
	}
}

func (m *RepoModule) globals() []*modtypes.ModulesAggregatorFunc {
	return []*modtypes.ModulesAggregatorFunc{}
}

// Configure configures the JS context and return
// any number of console prompt suggestions
func (m *RepoModule) Configure() []prompt.Suggest {
	suggestions := []prompt.Suggest{}

	// Add the main namespace
	obj := map[string]interface{}{}
	util.VMSet(m.vm, types.NamespaceRepo, obj)

	for _, f := range m.funcs() {
		obj[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", types.NamespaceRepo, f.Name)
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
// object.hash <string>: 	The transaction hash
// object.address <string: 	The address of the repository
func (m *RepoModule) create(params map[string]interface{}, options ...interface{}) interface{} {
	var err error

	var tx = core.NewBareTxRepoCreate()
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
		"hash":    hash,
		"address": fmt.Sprintf("r/%s", tx.Name),
	})
}

// upsertOwner creates a proposal to add or update a repository owner
//
// ARGS:
// params <map>
// params.id 		<string>: 			A unique proposal id
// params.addresses <string>: 			A comma separated list of addresses
// params.veto 		<bool>: 			Whether to grant/revoke veto right
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
func (m *RepoModule) upsertOwner(params map[string]interface{}, options ...interface{}) util.Map {
	var err error

	var tx = core.NewBareRepoProposalUpsertOwner()
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
func (m *RepoModule) voteOnProposal(params map[string]interface{}, options ...interface{}) util.Map {
	var err error

	var tx = core.NewBareRepoProposalVote()
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

// prune removes dangling or unreachable objects from a repository.
//
// ARGS:
// name: The name of the repository
// force: When true, forcefully prunes the target repository
func (m *RepoModule) prune(name string, force bool) {
	if force {
		if err := m.repoMgr.GetPruner().Prune(name, true); err != nil {
			panic(err)
		}
		return
	}
	m.repoMgr.GetPruner().Schedule(name)
}

// get finds and returns a repository
//
// ARGS:
// name: The name of the repository
//
// opts <map>: fetch options
// opts.height: Query a specific block
// opts.noProps: When true, the result will not include proposals
//
// RETURNS <map|nil>
func (m *RepoModule) get(name string, opts ...map[string]interface{}) util.Map {
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
		repo = m.logic.RepoKeeper().GetRepo(name, targetHeight)
	} else {
		repo = m.logic.RepoKeeper().GetRepoOnly(name, targetHeight)
		repo.Proposals = map[string]interface{}{}
	}

	if repo.IsNil() {
		return nil
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
// options[0] key <string>: 			The signer's private key
// options[1] payloadOnly <bool>: 		When true, returns the payload only, without sending the tx.
//
// RETURNS object <map>
// object.hash <string>: 				The transaction hash
func (m *RepoModule) update(params map[string]interface{}, options ...interface{}) util.Map {
	var err error

	var tx = core.NewBareRepoProposalUpdate()
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
// options[0] key <string>: 			The signer's private key
// options[1] payloadOnly <bool>: 		When true, returns the payload only, without sending the tx.
//
// RETURNS object <map>
// object.hash <string>: 				The transaction hash
func (m *RepoModule) depositFee(params map[string]interface{}, options ...interface{}) util.Map {
	var err error

	var tx = core.NewBareRepoProposalFeeSend()
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

// CreateMergeRequest creates a merge request proposal
//
// ARGS:
// params <map>
// params.name 			<string>: 				The name of the repository
// params.id 			<string>: 				A unique proposal ID
// params.base 			<string>:				The base branch name
// params.baseHash 		<string>:				The base branch pre-merge hash
// params.target 		<string>:				The target branch name
// params.targetHash	<string>:				The target branch hash
// params.nonce 		<number|string>: 		The senders next account nonce
// params.fee 			<number|string>: 		The transaction fee to pay
// params.timestamp 	<number>: 				The unix timestamp
//
// options <[]interface{}>
// options[0] key <string>: 			The signer's private key
// options[1] payloadOnly <bool>: 		When true, returns the payload only, without sending the tx.
//
// RETURNS object <map>
// object.hash <string>: 				The transaction hash
func (m *RepoModule) CreateMergeRequest(
	params map[string]interface{},
	options ...interface{}) interface{} {
	var err error

	var tx = core.NewBareRepoProposalMergeRequest()
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
