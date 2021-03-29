package api

import (
	modulestypes "github.com/make-os/kit/modules/types"
	"github.com/make-os/kit/rpc"
	"github.com/make-os/kit/types/constants"
	"github.com/spf13/cast"
	"github.com/stretchr/objx"
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
	return rpc.Success(a.mods.Repo.Create(cast.ToStringMap(params)))
}

// getRepo finds and returns a repository
func (a *RepoAPI) getRepo(params interface{}) (resp *rpc.Response) {
	obj := objx.New(cast.ToStringMap(params))
	name := obj.Get("name").Str()
	opts := modulestypes.GetOptions{}
	opts.Height = cast.ToUint64(obj.Get("height").Inter())
	opts.Select = cast.ToStringSlice(obj.Get("select").InterSlice())
	return rpc.Success(a.mods.Repo.Get(name, opts))
}

// addContributor creates a transaction to add one or more push keys as contributors
func (a *RepoAPI) addContributor(params interface{}) (resp *rpc.Response) {
	return rpc.Success(a.mods.Repo.AddContributor(cast.ToStringMap(params)))
}

// vote creates a transaction to vote for/against a repo proposal
func (a *RepoAPI) vote(params interface{}) (resp *rpc.Response) {
	return rpc.Success(a.mods.Repo.Vote(cast.ToStringMap(params)))
}

// update updates a repository
func (a *RepoAPI) update(params interface{}) (resp *rpc.Response) {
	return rpc.Success(a.mods.Repo.Update(cast.ToStringMap(params)))
}

// upsertOwner adds or updates one or more owners
func (a *RepoAPI) upsertOwner(params interface{}) (resp *rpc.Response) {
	return rpc.Success(a.mods.Repo.UpsertOwner(cast.ToStringMap(params)))
}

// depositPropFee deposit fees into a proposal
func (a *RepoAPI) depositPropFee(params interface{}) (resp *rpc.Response) {
	return rpc.Success(a.mods.Repo.DepositProposalFee(cast.ToStringMap(params)))
}

// track adds one or more repositories to the repo track list
func (a *RepoAPI) track(params interface{}) (resp *rpc.Response) {
	m := objx.New(cast.ToStringMap(params))
	a.mods.Repo.Track(m.Get("names").Str(), cast.ToUint64(m.Get("height").Inter()))
	return rpc.StatusOK()
}

// untrack removes one or more repositories from the repo track list
func (a *RepoAPI) untrack(params interface{}) (resp *rpc.Response) {
	a.mods.Repo.UnTrack(cast.ToString(params))
	return rpc.StatusOK()
}

// tracked returns tracked repositories and their last updated height
func (a *RepoAPI) tracked(interface{}) (resp *rpc.Response) {
	return rpc.Success(a.mods.Repo.GetTracked())
}

// APIs returns all API handlers
func (a *RepoAPI) APIs() rpc.APISet {
	ns := constants.NamespaceRepo
	return []rpc.MethodInfo{
		{Name: "create", Namespace: ns, Func: a.createRepo, Description: "Create a repository"},
		{Name: "update", Namespace: ns, Func: a.update, Description: "Update a repository"},
		{Name: "upsertOwner", Namespace: ns, Func: a.upsertOwner, Description: "Add or update one or more owners"},
		{Name: "depositPropFee", Namespace: ns, Func: a.depositPropFee, Description: "Deposit fee into a proposal"},
		{Name: "get", Namespace: ns, Func: a.getRepo, Description: "Get a repository"},
		{Name: "addContributor", Namespace: ns, Func: a.addContributor, Description: "Add one or more contributors"},
		{Name: "vote", Namespace: ns, Func: a.vote, Description: "Cast a vote on a repository's proposal"},
		{Name: "track", Namespace: ns, Func: a.track, Description: "Track one or more repositories", Private: true},
		{Name: "untrack", Namespace: ns, Func: a.untrack, Description: "Untrack one or more repositories", Private: true},
		{Name: "tracked", Namespace: ns, Func: a.tracked, Description: "Get all tracked repositories", Private: true},
	}
}
