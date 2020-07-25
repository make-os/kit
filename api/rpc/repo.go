package rpc

import (
	"github.com/spf13/cast"
	"github.com/stretchr/objx"
	modulestypes "gitlab.com/makeos/lobe/modules/types"
	"gitlab.com/makeos/lobe/rpc"
	"gitlab.com/makeos/lobe/types"
	"gitlab.com/makeos/lobe/types/constants"
)

// RepoAPI provides RPC methods for various repo related functionalities.
type RepoAPI struct {
	mods *modulestypes.Modules
}

// NewRepoAPI creates an instance of RepoAPI
func NewRepoAPI(mods *modulestypes.Modules) *RepoAPI {
	return &RepoAPI{mods: mods}
}

// createRepo creates a transaction to create a repository
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

// addContributors creates a transaction to add one or more push keys as contributors
func (a *RepoAPI) addContributors(params interface{}) (resp *rpc.Response) {
	p, ok := params.(map[string]interface{})
	if !ok {
		return rpc.Error(types.RPCErrCodeInvalidParamType, "param must be a map", "")
	}
	return rpc.Success(a.mods.Repo.AddContributor(p))
}

// vote creates a transaction to vote for/against a repo proposal
func (a *RepoAPI) vote(params interface{}) (resp *rpc.Response) {
	p, ok := params.(map[string]interface{})
	if !ok {
		return rpc.Error(types.RPCErrCodeInvalidParamType, "param must be a map", "")
	}
	return rpc.Success(a.mods.Repo.Vote(p))
}

// APIs returns all API handlers
func (a *RepoAPI) APIs() rpc.APISet {
	ns := constants.NamespaceRepo
	return []rpc.APIInfo{
		{Name: "create", Namespace: ns, Func: a.createRepo, Description: "Create a repository"},
		{Name: "get", Namespace: ns, Func: a.getRepo, Description: "Get a repository"},
		{Name: "addContributors", Namespace: ns, Func: a.addContributors, Description: "Create a repository"},
		{Name: "vote", Namespace: ns, Func: a.vote, Description: "Cast a vote on a repository's proposal"},
	}
}
