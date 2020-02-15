package modules

import (
	"fmt"

	prompt "github.com/c-bata/go-prompt"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/robertkrimen/otto"
)

// RepoModule provides repository functionalities to JS environment
type RepoModule struct {
	vm      *otto.Otto
	keepers types.Keepers
	service types.Service
	repoMgr types.RepoManager
}

// NewRepoModule creates an instance of RepoModule
func NewRepoModule(
	vm *otto.Otto,
	service types.Service,
	repoMgr types.RepoManager,
	keepers types.Keepers) *RepoModule {
	return &RepoModule{vm: vm, service: service, keepers: keepers, repoMgr: repoMgr}
}

// funcs are functions accessible using the `repo` namespace
func (m *RepoModule) funcs() []*types.ModulesAggregatorFunc {
	return []*types.ModulesAggregatorFunc{
		&types.ModulesAggregatorFunc{
			Name:        "create",
			Value:       m.create,
			Description: "Create a git repository on the network",
		},
		&types.ModulesAggregatorFunc{
			Name:        "get",
			Value:       m.get,
			Description: "Find and return a repository",
		},
		&types.ModulesAggregatorFunc{
			Name:        "update",
			Value:       m.update,
			Description: "Update a repository",
		},
		&types.ModulesAggregatorFunc{
			Name:        "prune",
			Value:       m.prune,
			Description: "Delete all dangling and unreachable loose objects from a repository",
		},
		&types.ModulesAggregatorFunc{
			Name:        "upsertOwner",
			Value:       m.upsertOwner,
			Description: "Create a proposal to add or update a repository owner",
		},
		&types.ModulesAggregatorFunc{
			Name:        "vote",
			Value:       m.voteOnProposal,
			Description: "Vote for or against a proposal",
		},
		&types.ModulesAggregatorFunc{
			Name:        "depositFee",
			Value:       m.depositFee,
			Description: "Add fees to a deposit-enabled repository proposal",
		},
		&types.ModulesAggregatorFunc{
			Name:        "createMergeRequest",
			Value:       m.CreateMergeRequest,
			Description: "Create a merge request proposal",
		},
	}
}

func (m *RepoModule) globals() []*types.ModulesAggregatorFunc {
	return []*types.ModulesAggregatorFunc{}
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

// create sends a TxTypeRepoCreate transaction to create a git repository
// params {
// 		nonce: number,
//		fee: string,
// 		value: string,
//		name: string
//		timestamp: number
// }
// options: key
func (m *RepoModule) create(params map[string]interface{}, options ...interface{}) interface{} {
	var err error

	// Decode parameters into a transaction object
	var tx = types.NewBareTxRepoCreate()
	mapstructure.Decode(params, tx)
	decodeCommon(tx, params)

	if value, ok := params["value"]; ok {
		defer castPanic("value")
		tx.Value = util.String(value.(string))
	}

	if repoName, ok := params["name"]; ok {
		defer castPanic("name")
		tx.Name = repoName.(string)
	}

	if config, ok := params["config"]; ok {
		defer castPanic("config")
		tx.Config = config.(map[string]interface{})
	}

	finalizeTx(tx, m.service, options...)

	// Process the transaction
	hash, err := m.service.SendTx(tx)
	if err != nil {
		panic(errors.Wrap(err, "failed to send transaction"))
	}

	return EncodeForJS(map[string]interface{}{
		"hash":    hash,
		"address": fmt.Sprintf("r/%s", tx.Name),
	})
}

// upsertOwner creates a proposal to add or update a repository owner
// params {
// 		nonce: number,
//		fee: string,
//		name: string
// 		id: string
// 		address: string
//		timestamp: number
// }
// options: key
func (m *RepoModule) upsertOwner(params map[string]interface{}, options ...interface{}) interface{} {
	var err error

	// Decode parameters into a transaction object
	var tx = types.NewBareRepoProposalUpsertOwner()
	mapstructure.Decode(params, tx)
	decodeCommon(tx, params)

	if repoName, ok := params["name"]; ok {
		defer castPanic("name")
		tx.RepoName = repoName.(string)
	}

	if id, ok := params["id"]; ok {
		defer castPanic("id")
		tx.ProposalID = id.(string)
	}

	if ownerAddrs, ok := params["addresses"]; ok {
		defer castPanic("addresses")
		tx.Addresses = ownerAddrs.(string)
	}

	if value, ok := params["value"]; ok {
		defer castPanic("value")
		tx.Value = util.String(value.(string))
	}

	if veto, ok := params["veto"]; ok {
		defer castPanic("veto")
		tx.Veto = veto.(bool)
	}

	finalizeTx(tx, m.service, options...)

	// Process the transaction
	hash, err := m.service.SendTx(tx)
	if err != nil {
		panic(errors.Wrap(err, "failed to send transaction"))
	}

	return EncodeForJS(map[string]interface{}{
		"hash": hash,
	})
}

// voteOnProposal sends a TxTypeRepoCreate transaction to create a git repository
// params {
// 		nonce: number,
//		fee: string,
//		name: string
// 		id: string
//		yes: bool
//		timestamp: number
// }
// options: key
func (m *RepoModule) voteOnProposal(params map[string]interface{}, options ...interface{}) interface{} {
	var err error

	// Decode parameters into a transaction object
	var tx = types.NewBareRepoProposalVote()
	mapstructure.Decode(params, tx)
	decodeCommon(tx, params)

	if repoName, ok := params["name"]; ok {
		defer castPanic("name")
		tx.RepoName = repoName.(string)
	}

	if id, ok := params["id"]; ok {
		defer castPanic("id")
		tx.ProposalID = id.(string)
	}

	if vote, ok := params["vote"]; ok {
		switch v := vote.(type) {
		case int64:
			tx.Vote = int(v)
		case float64:
			tx.Vote = int(v)
		default:
			panic("unexpected type for 'vote'")
		}
	}

	finalizeTx(tx, m.service, options...)

	// Process the transaction
	hash, err := m.service.SendTx(tx)
	if err != nil {
		panic(errors.Wrap(err, "failed to send transaction"))
	}

	return EncodeForJS(map[string]interface{}{
		"hash": hash,
	})
}

// prune removes dangling or unreachable objects from a repository.
// If force is true, the repository is immediately pruned.
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
// name: The name of the repository
// opts: Optional options
// opts.height: Limit the query to a given block height
// opts.noProps: Hide proposals.
func (m *RepoModule) get(name string, opts ...map[string]interface{}) interface{} {
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

	var repo *types.Repository
	if !noProposals {
		repo = m.keepers.RepoKeeper().GetRepo(name, targetHeight)
	} else {
		repo = m.keepers.RepoKeeper().GetRepoOnly(name, targetHeight)
		repo.Proposals = map[string]interface{}{}
	}

	if repo.IsNil() {
		return otto.NullValue()
	}

	return EncodeForJS(repo)
}

// update sends a TxTypeRepoProposalUpdate transaction to update a repository
// params {
// 		nonce: number,
//		fee: string,
//		name: string,
// 		id: string
//		value: string
//		config: {[key:string]: any}
//		timestamp: number
// }
// options: key
func (m *RepoModule) update(params map[string]interface{}, options ...interface{}) interface{} {
	var err error

	// Decode parameters into a transaction object
	var tx = types.NewBareRepoProposalUpdate()
	mapstructure.Decode(params, tx)
	decodeCommon(tx, params)

	if repoName, ok := params["name"]; ok {
		defer castPanic("name")
		tx.RepoName = repoName.(string)
	}

	if id, ok := params["id"]; ok {
		defer castPanic("id")
		tx.ProposalID = id.(string)
	}

	if value, ok := params["value"]; ok {
		defer castPanic("value")
		tx.Value = util.String(value.(string))
	}

	if config, ok := params["config"]; ok {
		defer castPanic("config")
		tx.Config = config.(map[string]interface{})
	}

	finalizeTx(tx, m.service, options...)

	// Process the transaction
	hash, err := m.service.SendTx(tx)
	if err != nil {
		panic(errors.Wrap(err, "failed to send transaction"))
	}

	return EncodeForJS(map[string]interface{}{
		"hash": hash,
	})
}

// depositFee sends a TxTypeRepoProposalFeeSend transaction to add fees to a
// repo's proposal.
// params {
// 		nonce: number,
//		fee: string,
//		name: string,
// 		id: string
//		value: string
//		timestamp: number
// }
// options: key
func (m *RepoModule) depositFee(params map[string]interface{}, options ...interface{}) interface{} {
	var err error

	// Decode parameters into a transaction object
	var tx = types.NewBareRepoProposalFeeSend()
	mapstructure.Decode(params, tx)
	decodeCommon(tx, params)

	if repoName, ok := params["name"]; ok {
		defer castPanic("name")
		tx.RepoName = repoName.(string)
	}

	if value, ok := params["value"]; ok {
		defer castPanic("value")
		tx.Value = util.String(value.(string))
	}

	if id, ok := params["id"]; ok {
		defer castPanic("id")
		tx.ProposalID = id.(string)
	}

	finalizeTx(tx, m.service, options...)

	// Process the transaction
	hash, err := m.service.SendTx(tx)
	if err != nil {
		panic(errors.Wrap(err, "failed to send transaction"))
	}

	return EncodeForJS(map[string]interface{}{
		"hash": hash,
	})
}

// CreateMergeRequest creates a merge request proposal
// params {
// 		nonce: number,
//		fee: string,
//		name: string,
// 		id: string
//		base: string
//		baseHash: string
// 		target: string
// 		targetHash: string
//		timestamp: number
// }
// options: key
func (m *RepoModule) CreateMergeRequest(
	params map[string]interface{},
	options ...interface{}) interface{} {
	var err error

	// Decode parameters into a transaction object
	var tx = types.NewBareRepoProposalMergeRequest()
	mapstructure.Decode(params, tx)
	decodeCommon(tx, params)

	if repoName, ok := params["name"]; ok {
		defer castPanic("name")
		tx.RepoName = repoName.(string)
	}

	if id, ok := params["id"]; ok {
		defer castPanic("id")
		tx.ProposalID = id.(string)
	}

	if base, ok := params["base"]; ok {
		defer castPanic("base")
		tx.BaseBranch = base.(string)
	}

	if baseHash, ok := params["baseHash"]; ok {
		defer castPanic("baseHash")
		tx.BaseBranchHash = baseHash.(string)
	}

	if target, ok := params["target"]; ok {
		defer castPanic("target")
		tx.TargetBranch = target.(string)
	}

	if targetHash, ok := params["targetHash"]; ok {
		defer castPanic("targetHash")
		tx.TargetBranchHash = targetHash.(string)
	}

	finalizeTx(tx, m.service, options...)

	// Process the transaction
	hash, err := m.service.SendTx(tx)
	if err != nil {
		panic(errors.Wrap(err, "failed to send transaction"))
	}

	return EncodeForJS(map[string]interface{}{
		"hash": hash,
	})
}
