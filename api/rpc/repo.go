package rpc

import (
	"github.com/spf13/cast"
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
	opts.Height = cast.ToUint64(obj.Get("height").Inter())
	opts.NoProposals = obj.Get("noProposals").Bool()

	return rpc.Success(a.mods.Repo.Get(name, opts))
}

// APIs returns all API handlers
func (a *RepoAPI) APIs() rpc.APISet {
	return []rpc.APIInfo{
		{Name: "create", Namespace: constants.NamespaceRepo, Func: a.createRepo, Description: "Create a repository"},
		{Name: "get", Namespace: constants.NamespaceRepo, Func: a.getRepo, Description: "Get a repository"},
	}
}
