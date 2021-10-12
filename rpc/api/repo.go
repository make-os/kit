package api

import (
	modulestypes "github.com/make-os/kit/modules/types"
	"github.com/make-os/kit/rpc"
	"github.com/make-os/kit/types/constants"
	"github.com/make-os/kit/util"
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

// listByCreator returns names of repos created by an address
func (a *RepoAPI) listByCreator(params interface{}) (resp *rpc.Response) {
	m := objx.New(cast.ToStringMap(params))
	names := a.mods.Repo.GetReposCreatedByAddress(m.Get("address").Str())
	return rpc.Success(map[string]interface{}{
		"repos": names,
	})
}

// ls list files and directories of a repository
func (a *RepoAPI) ls(params interface{}) (resp *rpc.Response) {
	m := objx.New(cast.ToStringMap(params))
	var revision []string
	if rev := m.Get("revision").Str(); rev != "" {
		revision = []string{rev}
	}
	return rpc.Success(util.Map{
		"entries": a.mods.Repo.ListPath(m.Get("name").Str(), m.Get("path").Str(), revision...),
	})
}

// readFileLines gets the lines of a file in a repository
func (a *RepoAPI) readFileLines(params interface{}) (resp *rpc.Response) {
	m := objx.New(cast.ToStringMap(params))
	var revision []string
	if rev := m.Get("revision").Str(); rev != "" {
		revision = []string{rev}
	}
	return rpc.Success(util.Map{
		"lines": a.mods.Repo.ReadFileLines(m.Get("name").Str(), m.Get("path").Str(), revision...),
	})
}

// readFile gets the string content of a file in a repository
func (a *RepoAPI) readFile(params interface{}) (resp *rpc.Response) {
	m := objx.New(cast.ToStringMap(params))
	var revision []string
	if rev := m.Get("revision").Str(); rev != "" {
		revision = []string{rev}
	}
	return rpc.Success(util.Map{
		"content": a.mods.Repo.ReadFile(m.Get("name").Str(), m.Get("path").Str(), revision...),
	})
}

// getBranches returns a list of branches in a repository
func (a *RepoAPI) getBranches(params interface{}) (resp *rpc.Response) {
	m := objx.New(cast.ToStringMap(params))
	return rpc.Success(util.Map{"branches": a.mods.Repo.GetBranches(m.Get("name").Str())})
}

// getLatestCommit gets the latest commit of a branch in a repository
func (a *RepoAPI) getLatestCommit(params interface{}) (resp *rpc.Response) {
	m := objx.New(cast.ToStringMap(params))
	return rpc.Success(util.Map{
		"commit": a.mods.Repo.GetLatestBranchCommit(m.Get("name").Str(), m.Get("branch").Str()),
	})
}

// getCommits gets a list of commits of a branch or ancestors of a commit of a repository
func (a *RepoAPI) getCommits(params interface{}) (resp *rpc.Response) {
	m := objx.New(cast.ToStringMap(params))
	var limit []int
	if l := m.Get("limit").Float64(); l > 0 {
		limit = []int{int(l)}
	}
	return rpc.Success(util.Map{
		"commits": a.mods.Repo.GetCommits(m.Get("name").Str(), m.Get("reference").Str(), limit...),
	})
}

// getCommit gets a commit from a repo
func (a *RepoAPI) getCommit(params interface{}) (resp *rpc.Response) {
	m := objx.New(cast.ToStringMap(params))
	return rpc.Success(util.Map{
		"commit": a.mods.Repo.GetCommit(m.Get("name").Str(), m.Get("hash").Str()),
	})
}

// getCommits gets a list of commits of a branch/reference in a repository
func (a *RepoAPI) countCommits(params interface{}) (resp *rpc.Response) {
	m := objx.New(cast.ToStringMap(params))
	return rpc.Success(util.Map{
		"count": a.mods.Repo.CountCommits(m.Get("name").Str(), m.Get("branch").Str()),
	})
}

// getDiffOfCommitAndParents gets the diff output between a commit and its parent(s).
func (a *RepoAPI) getDiffOfCommitAndParents(params interface{}) (resp *rpc.Response) {
	m := objx.New(cast.ToStringMap(params))
	return rpc.Success(a.mods.Repo.GetParentsAndCommitDiff(m.Get("name").Str(), m.Get("commitHash").Str()))
}

// getAncestors gets ancestors of a commit in a repository
func (a *RepoAPI) getAncestors(params interface{}) (resp *rpc.Response) {
	m := objx.New(cast.ToStringMap(params))
	var limit []int
	if l := m.Get("limit").Float64(); l > 0 {
		limit = []int{int(l)}
	}
	return rpc.Success(util.Map{
		"commits": a.mods.Repo.GetCommitAncestors(m.Get("name").Str(), m.Get("commitHash").Str(), limit...),
	})
}

// APIs returns all API handlers
func (a *RepoAPI) APIs() rpc.APISet {
	ns := constants.NamespaceRepo
	return []rpc.MethodInfo{
		{Name: "create", Namespace: ns, Func: a.createRepo, Desc: "Create a repository"},
		{Name: "update", Namespace: ns, Func: a.update, Desc: "Update a repository"},
		{Name: "upsertOwner", Namespace: ns, Func: a.upsertOwner, Desc: "Add or update one or more owners"},
		{Name: "depositPropFee", Namespace: ns, Func: a.depositPropFee, Desc: "Deposit fee into a proposal"},
		{Name: "get", Namespace: ns, Func: a.getRepo, Desc: "Get a repository"},
		{Name: "addContributor", Namespace: ns, Func: a.addContributor, Desc: "Add one or more contributors"},
		{Name: "vote", Namespace: ns, Func: a.vote, Desc: "Cast a vote on a repository's proposal"},
		{Name: "track", Namespace: ns, Func: a.track, Desc: "Track one or more repositories", Private: true},
		{Name: "untrack", Namespace: ns, Func: a.untrack, Desc: "Untrack one or more repositories", Private: true},
		{Name: "tracked", Namespace: ns, Func: a.tracked, Desc: "Get all tracked repositories"},
		{Name: "listByCreator", Namespace: ns, Func: a.listByCreator, Desc: "List repositories created by an address"},
		{Name: "ls", Namespace: ns, Func: a.ls, Desc: "List files and directories of a repository"},
		{Name: "readFileLines", Namespace: ns, Func: a.readFileLines, Desc: "Gets the lines of a file in a repository"},
		{Name: "readFile", Namespace: ns, Func: a.readFile, Desc: "Get the string content of a file in a repository"},
		{Name: "getBranches", Namespace: ns, Func: a.getBranches, Desc: "Get a list of branches in a repository"},
		{Name: "getLatestCommit", Namespace: ns, Func: a.getLatestCommit, Desc: "Gets the latest commit of a branch in a repository"},
		{Name: "getCommits", Namespace: ns, Func: a.getCommits, Desc: "Get a list of commits in a branch of a repository"},
		{Name: "getCommit", Namespace: ns, Func: a.getCommit, Desc: "Get a commit from a repository"},
		{Name: "countCommits", Namespace: ns, Func: a.countCommits, Desc: "Get the number of commits in a reference"},
		{Name: "getAncestors", Namespace: ns, Func: a.getAncestors, Desc: "Get ancestors of a commit in a repository"},
		{Name: "getDiffOfCommitAndParents", Namespace: ns, Func: a.getDiffOfCommitAndParents, Desc: "Get the diff output between a commit and its parent(s)."},
	}
}
