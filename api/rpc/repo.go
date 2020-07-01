package rpc

import (
	"github.com/stretchr/objx"
	modulestypes "gitlab.com/makeos/mosdef/modules/types"
	"gitlab.com/makeos/mosdef/rpc"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/constants"
)

// RepoAPI provides RPC methods for various repo related functionalities.
type RepoAPI struct {
	mods *modulestypes.Modules
}

// NewRepoAPI creates an instance of RepoAPI
func NewRepoAPI(mods *modulestypes.Modules) *RepoAPI {
	return &RepoAPI{mods: mods}
}

// createRepo creates a new repository
//
// ARGS:
// params 		<map>: TxCreateRepo transaction fields
// Response 	<map>: RPC:repo_create response
func (a *RepoAPI) createRepo(params interface{}) (resp *rpc.Response) {
	p, ok := params.(map[string]interface{})
	if !ok {
		return rpc.Error(types.RPCErrCodeInvalidParamType, "param must be a map", "")
	}
	return rpc.Success(a.mods.Repo.Create(p))
}

// getRepo finds and returns a repository
func (a *RepoAPI) getRepo(params interface{}) (resp *rpc.Response) {
	m, ok := params.(map[string]interface{})
	if !ok {
		return rpc.Error(types.RPCErrCodeInvalidParamType, "param must be a map", "")
	}

	obj := objx.New(m)
	name := obj.Get("name").Str()
	opts := modulestypes.GetOptions{}

	if !obj.Get("height").IsNil() {
		opts.Height = obj.Get("height").Inter(0)
	}

	if !obj.Get("noProposals").IsNil() {
		opts.NoProposals = obj.Get("noProposals").Bool()
	}

	return rpc.Success(a.mods.Repo.Get(name, opts))
}

// APIs returns all API handlers
func (a *RepoAPI) APIs() rpc.APISet {
	return map[string]rpc.APIInfo{
		"create": {Namespace: constants.NamespaceRepo, Func: a.createRepo, Description: "Create a repository"},
		"get":    {Namespace: constants.NamespaceRepo, Func: a.getRepo, Description: "Get a repository"},
	}
}
