package remote

import (
	"encoding/json"
	"net/http"

	"github.com/spf13/cast"
	"github.com/stretchr/objx"
	modulestypes "gitlab.com/makeos/mosdef/modules/types"
	"gitlab.com/makeos/mosdef/util"
)

// CreateRepo handles request to create a repository
func (r *API) CreateRepo(w http.ResponseWriter, req *http.Request) {
	var body = make(map[string]interface{})
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		util.WriteJSON(w, 400, util.RESTApiErrorMsg("malformed body", "", "0"))
		return
	}
	util.WriteJSON(w, 201, r.modules.Repo.Create(body))
}

// GetRepo handles request to retrieve a repository
func (r *API) GetRepo(w http.ResponseWriter, req *http.Request) {
	var body = objx.MustFromURLQuery(req.URL.Query().Encode())

	name := body.Get("name").Str()
	opts := modulestypes.GetOptions{}
	opts.Height = cast.ToUint64(body.Get("height").Str())
	opts.NoProposals = cast.ToBool(body.Get("noProposals").Str())

	util.WriteJSON(w, 200, r.modules.Repo.Get(name, opts))
}

// RepoVote handles request to vote for/against a repository proposal
func (r *API) RepoVote(w http.ResponseWriter, req *http.Request) {
	var body = make(map[string]interface{})
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		util.WriteJSON(w, 400, util.RESTApiErrorMsg("malformed body", "", "0"))
		return
	}
	util.WriteJSON(w, 201, r.modules.Repo.Vote(body))
}

// AddRepoContributors handles request to add a repository contributor
func (r *API) AddRepoContributors(w http.ResponseWriter, req *http.Request) {
	var body = make(map[string]interface{})
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		util.WriteJSON(w, 400, util.RESTApiErrorMsg("malformed body", "", "0"))
		return
	}
	util.WriteJSON(w, 201, r.modules.Repo.AddContributor(body))
}
