package modules

import (
	"fmt"
	prompt "github.com/c-bata/go-prompt"
	"github.com/pkg/errors"
	"github.com/robertkrimen/otto"
	modtypes "gitlab.com/makeos/mosdef/modules/types"
	servtypes "gitlab.com/makeos/mosdef/services/types"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/util"
)

// RepoModule provides repository functionalities to JS environment
type RepoModule struct {
	vm      *otto.Otto
	keepers core.Keepers
	service servtypes.Service
	repoMgr core.RepoManager
}

// NewRepoModule creates an instance of RepoModule
func NewRepoModule(
	vm *otto.Otto,
	service servtypes.Service,
	repoMgr core.RepoManager,
	keepers core.Keepers) *RepoModule {
	return &RepoModule{vm: vm, service: service, keepers: keepers, repoMgr: repoMgr}
}

// funcs are functions accessible using the `repo` namespace
func (m *RepoModule) funcs() []*modtypes.ModulesAggregatorFunc {
	return []*modtypes.ModulesAggregatorFunc{
		&modtypes.ModulesAggregatorFunc{
			Name:        "create",
			Value:       m.create,
			Description: "Create a git repository on the network",
		},
		&modtypes.ModulesAggregatorFunc{
			Name:        "get",
			Value:       m.get,
			Description: "Find and return a repository",
		},
		&modtypes.ModulesAggregatorFunc{
			Name:        "update",
			Value:       m.update,
			Description: "Update a repository",
		},
		&modtypes.ModulesAggregatorFunc{
			Name:        "prune",
			Value:       m.prune,
			Description: "Delete all dangling and unreachable loose objects from a repository",
		},
		&modtypes.ModulesAggregatorFunc{
			Name:        "upsertOwner",
			Value:       m.upsertOwner,
			Description: "Create a proposal to add or update a repository owner",
		},
		&modtypes.ModulesAggregatorFunc{
			Name:        "vote",
			Value:       m.voteOnProposal,
			Description: "Vote for or against a proposal",
		},
		&modtypes.ModulesAggregatorFunc{
			Name:        "depositFee",
			Value:       m.depositFee,
			Description: "Add fees to a deposit-enabled repository proposal",
		},
		&modtypes.ModulesAggregatorFunc{
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

// create sends a TxTypeRepoCreate transaction to create a git repository
// params {
// 		nonce: number,
//		fee: string,
// 		value: string,
//		name: string
//		timestamp: number
// }
// options[0]: key
// options[1]: payloadOnly - When true, returns the payload only, without sending the tx.
func (m *RepoModule) create(params map[string]interface{}, options ...interface{}) interface{} {
	var err error

	var tx = core.NewBareTxRepoCreate()
	if err = tx.FromMap(params); err != nil {
		panic(err)
	}

	payloadOnly := finalizeTx(tx, m.service, options...)
	if payloadOnly {
		return EncodeForJS(tx.ToMap())
	}

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
// options[0]: key
// options[1]: payloadOnly - When true, returns the payload only, without sending the tx.
func (m *RepoModule) upsertOwner(params map[string]interface{}, options ...interface{}) interface{} {
	var err error

	var tx = core.NewBareRepoProposalUpsertOwner()
	if err = tx.FromMap(params); err != nil {
		panic(err)
	}

	payloadOnly := finalizeTx(tx, m.service, options...)
	if payloadOnly {
		return EncodeForJS(tx.ToMap())
	}

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
// options[0]: key
// options[1]: payloadOnly - When true, returns the payload only, without sending the tx.
func (m *RepoModule) voteOnProposal(params map[string]interface{}, options ...interface{}) interface{} {
	var err error

	var tx = core.NewBareRepoProposalVote()
	if err = tx.FromMap(params); err != nil {
		panic(err)
	}

	payloadOnly := finalizeTx(tx, m.service, options...)
	if payloadOnly {
		return EncodeForJS(tx.ToMap())
	}

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

	var repo *state.Repository
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
// options[0]: key
// options[1]: payloadOnly - When true, returns the payload only, without sending the tx.
func (m *RepoModule) update(params map[string]interface{}, options ...interface{}) interface{} {
	var err error

	var tx = core.NewBareRepoProposalUpdate()
	if err = tx.FromMap(params); err != nil {
		panic(err)
	}

	payloadOnly := finalizeTx(tx, m.service, options...)
	if payloadOnly {
		return EncodeForJS(tx.ToMap())
	}

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
// options[0]: key
// options[1]: payloadOnly - When true, returns the payload only, without sending the tx.
func (m *RepoModule) depositFee(params map[string]interface{}, options ...interface{}) interface{} {
	var err error

	var tx = core.NewBareRepoProposalFeeSend()
	if err = tx.FromMap(params); err != nil {
		panic(err)
	}

	payloadOnly := finalizeTx(tx, m.service, options...)
	if payloadOnly {
		return EncodeForJS(tx.ToMap())
	}

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
// options[0]: key
// options[1]: payloadOnly - When true, returns the payload only, without sending the tx.
func (m *RepoModule) CreateMergeRequest(
	params map[string]interface{},
	options ...interface{}) interface{} {
	var err error

	var tx = core.NewBareRepoProposalMergeRequest()
	if err = tx.FromMap(params); err != nil {
		panic(err)
	}

	payloadOnly := finalizeTx(tx, m.service, options...)
	if payloadOnly {
		return EncodeForJS(tx.ToMap())
	}

	hash, err := m.service.SendTx(tx)
	if err != nil {
		panic(errors.Wrap(err, "failed to send transaction"))
	}

	return EncodeForJS(map[string]interface{}{
		"hash": hash,
	})
}
