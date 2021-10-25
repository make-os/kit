package modules

import (
	"fmt"
	"os"
	"strings"

	"github.com/AlekSi/pointer"
	"github.com/acarl005/stripansi"
	"github.com/c-bata/go-prompt"
	"github.com/go-git/go-git/v5"
	gogitcfg "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/make-os/kit/cmd/issuecmd"
	"github.com/make-os/kit/cmd/mergecmd"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/crypto/ed25519"
	modtypes "github.com/make-os/kit/modules/types"
	"github.com/make-os/kit/node/services"
	pl "github.com/make-os/kit/remote/plumbing"
	"github.com/make-os/kit/remote/repo"
	remotetypes "github.com/make-os/kit/remote/types"
	rpctypes "github.com/make-os/kit/rpc/types"
	"github.com/make-os/kit/types"
	"github.com/make-os/kit/types/api"
	"github.com/make-os/kit/types/constants"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/crypto"
	"github.com/make-os/kit/util/identifier"
	"github.com/make-os/kit/util/pushtoken"
	"github.com/pkg/errors"
	"github.com/robertkrimen/otto"
	"github.com/spf13/cast"
	"github.com/stretchr/objx"
)

// RepoModule provides repository functionalities to JS environment
type RepoModule struct {
	modtypes.ModuleCommon
	logic              core.Logic
	service            services.Service
	repoSrv            core.RemoteServer
	PostIDFinder       pl.GetFreePostIDFunc
	GetLocalRepo       repo.GetLocalRepoFunc
	IssueCreate        issuecmd.IssueCreateCmdFunc
	MergeRequestCreate mergecmd.MergeRequestCreateCmdFunc
}

// NewAttachableRepoModule creates an instance of RepoModule suitable in attach mode
func NewAttachableRepoModule(client rpctypes.Client) *RepoModule {
	return &RepoModule{ModuleCommon: modtypes.ModuleCommon{Client: client}}
}

// NewRepoModule creates an instance of RepoModule
func NewRepoModule(service services.Service, repoSrv core.RemoteServer, logic core.Logic) *RepoModule {
	return &RepoModule{
		service:            service,
		logic:              logic,
		repoSrv:            repoSrv,
		PostIDFinder:       pl.GetFreePostID,
		GetLocalRepo:       repo.GetWithGitModule,
		IssueCreate:        issuecmd.IssueCreateCmd,
		MergeRequestCreate: mergecmd.MergeRequestCreateCmd,
	}
}

// methods are functions exposed in the special namespace of this module.
func (m *RepoModule) methods() []*modtypes.VMMember {
	return []*modtypes.VMMember{
		{Name: "create", Value: m.Create, Description: "Create a git repository on the network"},
		{Name: "get", Value: m.Get, Description: "Get and return a repository"},
		{Name: "update", Value: m.Update, Description: "Update a repository"},
		{Name: "upsertOwner", Value: m.UpsertOwner, Description: "Create a proposal to add or update a repository owner"},
		{Name: "vote", Value: m.Vote, Description: "Vote for or against a proposal"},
		{Name: "depositPropFee", Value: m.DepositProposalFee, Description: "Deposit fees into a proposal"},
		{Name: "addContributor", Value: m.AddContributor, Description: "Register one or more push keys as contributors"},
		{Name: "track", Value: m.Track, Description: "Track one or more repositories"},
		{Name: "untrack", Value: m.UnTrack, Description: "Untrack one or more repositories"},
		{Name: "tracked", Value: m.GetTracked, Description: "Get a list of tracked repositories"},
		{Name: "listByCreator", Value: m.GetReposCreatedByAddress, Description: "List repositories created by an address"},

		{Name: "createIssue", Value: m.CreateIssue, Description: "Create an issue, add comments or edit an existing issue"},
		{Name: "createMergeRequest", Value: m.CreateMergeRequest, Description: "Create a merge request, add comments or edit an existing merge request"},
		{Name: "push", Value: m.Push, Description: "Sign and push a commit, tag or note"},

		// Repository query methods.
		{Name: "ls", Value: m.ListPath, Description: "List files and directories of a repository"},
		{Name: "readFileLines", Value: m.ReadFileLines, Description: "Get the lines of a file in a repository"},
		{Name: "readFile", Value: m.ReadFile, Description: "Get the string content of a file in a repository"},
		{Name: "getBranches", Value: m.GetBranches, Description: "Get a list of branches in a repository"},
		{Name: "getLatestCommit", Value: m.GetLatestBranchCommit, Description: "Get the latest commit of a branch in a repository"},
		{Name: "getCommits", Value: m.GetCommits, Description: "Get a list of commits in a branch of a repository"},
		{Name: "getCommit", Value: m.GetCommit, Description: "Get a commit"},
		{Name: "getAncestors", Value: m.GetCommitAncestors, Description: "Get ancestors of a commit in a repository"},
		{Name: "countCommits", Value: m.CountCommits, Description: "Get a branch/reference commit count"},
		{Name: "getDiffOfCommitAndParents", Value: m.GetParentsAndCommitDiff, Description: "Get the diff output of a commit and its parent(s)"},
	}
}

// globals are functions exposed in the VM's global namespace
func (m *RepoModule) globals() []*modtypes.VMMember {
	return []*modtypes.VMMember{}
}

// ConfigureVM configures the JS context and return
// any number of console prompt suggestions
func (m *RepoModule) ConfigureVM(vm *otto.Otto) prompt.Completer {

	// Register the main namespace
	obj := map[string]interface{}{}
	util.VMSet(vm, constants.NamespaceRepo, obj)

	for _, f := range m.methods() {
		obj[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", constants.NamespaceRepo, f.Name)
		m.Suggestions = append(m.Suggestions, prompt.Suggest{Text: funcFullName, Description: f.Description})
	}

	// Register global functions
	for _, f := range m.globals() {
		_ = vm.Set(f.Name, f.Value)
		m.Suggestions = append(m.Suggestions, prompt.Suggest{Text: f.Name, Description: f.Description})
	}

	return m.Completer
}

// Create  registers a git repository on the network
//
// params <map>
//  - name <string>: The name of the namespace
//  - value <string>: The amount to pay for initial resources
//  - nonce <number|string>: The senders next account nonce
//  - fee <number|string>: The transaction fee to pay
//  - timestamp <number>: The unix timestamp
//  - config <object>: The repo configuration
//  - sig <String>: The transaction signature
//
// options <[]interface{}>
//  - [0] key <string>: The signer's private key
//  - [1] payloadOnly <bool>: When true, returns the payload only, without sending the tx.
//
// RETURN object <map>
//  - hash <string>: The transaction hash
//  - address <string: The address of the repository
func (m *RepoModule) Create(params map[string]interface{}, options ...interface{}) util.Map {

	var tx = txns.NewBareTxRepoCreate()
	if err := tx.FromMap(params); err != nil {
		panic(se(400, StatusCodeInvalidParam, "params", err.Error()))
	}
	retPayload, signingKey := finalizeTx(tx, m.logic, m.Client, options...)
	if retPayload {
		return tx.ToMap()
	}

	if m.IsAttached() {
		resp, err := m.Client.Repo().Create(&api.BodyCreateRepo{
			Name:       tx.Name,
			Nonce:      tx.Nonce,
			Value:      cast.ToFloat64(tx.Value.String()),
			Fee:        cast.ToFloat64(tx.Fee.String()),
			Config:     tx.Config.ToMap(),
			SigningKey: ed25519.NewKeyFromPrivKey(signingKey),
		})
		if err != nil {
			panic(err)
		}
		return util.ToMap(resp)
	}

	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		panic(se(400, StatusCodeMempoolAddFail, "", err.Error()))
	}

	return map[string]interface{}{
		"hash":    hash,
		"address": fmt.Sprintf("r/%s", tx.Name),
	}
}

// UpsertOwner creates a proposal to add or update a repository owner
//
// params <map>
//  - id <string>: A unique proposal id
//  - addresses <string>: A comma separated list of addresses
//  - veto <bool>: The senders next account nonce
//  - fee <number|string>: The transaction fee to pay
//  - timestamp <number>: The unix timestamp
//
// options <[]interface{}>
//  - [0] key <string>: The signer's private key
//  - [1] payloadOnly <bool>: When true, returns the payload only, without sending the tx.
//
// RETURN <map>: When payloadOnly is false
//  - hash <string>: The transaction hash
func (m *RepoModule) UpsertOwner(params map[string]interface{}, options ...interface{}) util.Map {
	var err error

	var tx = txns.NewBareRepoProposalUpsertOwner()
	if err = tx.FromMap(params); err != nil {
		panic(se(400, StatusCodeInvalidParam, "params", err.Error()))
	}

	if retPayload, _ := finalizeTx(tx, m.logic, nil, options...); retPayload {
		return tx.ToMap()
	}

	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		panic(se(400, StatusCodeMempoolAddFail, "", err.Error()))
	}

	return map[string]interface{}{
		"hash": hash,
	}
}

// Vote sends a TxTypeRepoCreate transaction to create a git repository
//
// params <map>
//  - id <string>: The proposal ID to vote on
//  - name <string>: The name of the repository
//  - vote <uint>: The vote choice (1) yes (0) no (2) vote no with veto (3) abstain
//  - nonce <number|string>: The senders next account nonce
//  - fee <number|string>: The transaction fee to pay
//  - timestamp <number>: The unix timestamp
//
// options <[]interface{}>
//  - [0] key <string>: The signer's private key
//  - [1] payloadOnly <bool>: When true, returns the payload only, without sending the tx.
//
// RETURN object <map>
//  - hash <string>: The transaction hash
func (m *RepoModule) Vote(params map[string]interface{}, options ...interface{}) util.Map {
	var err error

	var tx = txns.NewBareRepoProposalVote()
	if err = tx.FromMap(params); err != nil {
		panic(se(400, StatusCodeInvalidParam, "params", err.Error()))
	}

	retPayload, signingKey := finalizeTx(tx, m.logic, m.Client, options...)
	if retPayload {
		return tx.ToMap()
	}

	if m.IsAttached() {
		resp, err := m.Client.Repo().VoteProposal(&api.BodyRepoVote{
			RepoName:   tx.RepoName,
			ProposalID: tx.ProposalID,
			Vote:       tx.Vote,
			Nonce:      tx.Nonce,
			Fee:        cast.ToFloat64(tx.Fee.String()),
			SigningKey: ed25519.NewKeyFromPrivKey(signingKey),
		})
		if err != nil {
			panic(err)
		}
		return util.ToMap(resp)
	}

	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		panic(se(400, StatusCodeMempoolAddFail, "", err.Error()))
	}

	return map[string]interface{}{
		"hash": hash,
	}
}

// Get finds and returns a repository.
//
// name: The name of the repository
//
// opts <map>: fetch options
//  - opts.height: Query a specific block
//  - opts.select: Provide a list of dot-notated fields to be returned.
//
// RETURN <state.Repository>
func (m *RepoModule) Get(name string, opts ...modtypes.GetOptions) util.Map {
	var blockHeight uint64
	var selectors []string
	var err error

	if len(opts) > 0 {
		opt := opts[0]
		selectors = opt.Select
		if opt.Height != nil {
			blockHeight, err = cast.ToUint64E(opt.Height)
			if err != nil {
				panic(se(400, StatusCodeInvalidParam, "opts.height", "unexpected type"))
			}
		}
	}

	if m.IsAttached() {
		resp, err := m.Client.Repo().Get(name, &api.GetRepoOpts{
			Height: blockHeight,
		})
		if err != nil {
			panic(err)
		}
		return util.ToMap(resp)
	}

	if identifier.IsFullNamespaceURI(name) {
		nsName := identifier.GetNamespace(name)
		if nsName == identifier.NativeNamespaceRepoChar {
			name = identifier.GetDomain(name)
		} else {
			ns := m.logic.NamespaceKeeper().Get(crypto.MakeNamespaceHash(nsName))
			if ns.IsNil() {
				panic(se(404, StatusCodeInvalidParam, "name", "namespace not found"))
			}
			name = ns.Domains.Get(identifier.GetDomain(name))
			if name == "" {
				panic(se(404, StatusCodeInvalidParam, "name", "namespace domain not found"))
			}
			if !strings.HasPrefix(name, identifier.NativeNamespaceRepo) {
				panic(se(404, StatusCodeInvalidParam, "name", "namespace domain target is not a repository"))
			}
			name = identifier.GetDomain(name)
		}
	}

	r := m.logic.RepoKeeper().Get(name, blockHeight)

	if r.IsEmpty() {
		panic(se(404, StatusCodeRepoNotFound, "name", types.ErrRepoNotFound.Error()))
	}

	if len(selectors) > 0 {
		selected, err := Select(util.MustToJSON(r), selectors...)
		if err != nil {
			panic(se(400, StatusCodeInvalidParam, "select", err.Error()))
		}
		return selected
	}

	return util.ToMap(r)
}

// Update creates a proposal to update a repository
//
// params <map>
//  - name <string>: The name of the repository
//  - id <string>: A unique proposal ID
//  - value <string|number>: The proposal fee
//  - config <map[string]string>: The updated repository config
//  - nonce <number|string>: The senders next account nonce
//  - fee <number|string>: The transaction fee to pay
//  - timestamp <number>: The unix timestamp
//
// options <[]interface{}>
//  - [0] key <string>: The signer's private key
//  - [1] payloadOnly <bool>: When true, returns the payload only, without sending the tx.
//
// RETURN object <map>
//  - hash <string>: The transaction hash
func (m *RepoModule) Update(params map[string]interface{}, options ...interface{}) util.Map {
	var err error

	var tx = txns.NewBareRepoProposalUpdate()
	if err = tx.FromMap(params); err != nil {
		panic(se(400, StatusCodeInvalidParam, "params", err.Error()))
	}

	if retPayload, _ := finalizeTx(tx, m.logic, nil, options...); retPayload {
		return tx.ToMap()
	}

	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		panic(se(400, StatusCodeMempoolAddFail, "", err.Error()))
	}

	return map[string]interface{}{
		"hash": hash,
	}
}

// DepositProposalFee creates a transaction to deposit a fee to a proposal
//
// params <map>
//  - params.name <string>: The name of the repository
//  - params.id <string>: A unique proposal ID
//  - params.value <string|number>: The amount to add
//  - params.nonce <number|string>: The senders next account nonce
//  - params.fee <number|string>: The transaction fee to pay
//  - params.timestamp <number>: The unix timestamp
//
// options <[]interface{}>
//  - [0] key <string>: The signer's private key
//  - [1] payloadOnly <bool>: When true, returns the payload only, without sending the tx.
//
// RETURN object <map>
//  - hash <string>: The transaction hash
func (m *RepoModule) DepositProposalFee(params map[string]interface{}, options ...interface{}) util.Map {
	var err error

	var tx = txns.NewBareRepoProposalFeeSend()
	if err = tx.FromMap(params); err != nil {
		panic(se(400, StatusCodeInvalidParam, "params", err.Error()))
	}

	if retPayload, _ := finalizeTx(tx, m.logic, nil, options...); retPayload {
		return tx.ToMap()
	}

	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		panic(se(400, StatusCodeMempoolAddFail, "", err.Error()))
	}

	return map[string]interface{}{
		"hash": hash,
	}
}

// AddContributor creates a proposal to register one or more push keys
//
// params <map>
//  - name 	<string>: The name of the repository
//  - id <string>: A unique proposal ID
//  - ids <string|[]string>: A list or comma separated list of push key IDs to add
//  - policies <[]map[string]interface{}>: A list of policies
// 	 - sub <string>:	The policy's subject
//	 - obj <string>:	The policy's object
//	 - act <string>:	The policy's action
//  - value <string|number>: The proposal fee to pay
//  - nonce <number|string>: The senders next account nonce
//  - fee <number|string>: The transaction fee to pay
//  - timestamp <number>: The unix timestamp
//  - namespace <string>: A namespace to also register the key to
//  - namespaceOnly <string>: Like namespace but key will not be registered to the repo.
//
// options 			<[]interface{}>
//  - [0] 		key <string>: The signer's private key
//  - [1] 		payloadOnly <bool>: When true, returns the payload only, without sending the tx.
//
// RETURN object <map>
//  - hash <string>: 							The transaction hash
func (m *RepoModule) AddContributor(params map[string]interface{}, options ...interface{}) util.Map {
	var err error

	var tx = txns.NewBareRepoProposalRegisterPushKey()
	if err = tx.FromMap(params); err != nil {
		panic(se(400, StatusCodeInvalidParam, "params", err.Error()))
	}

	retPayload, signingKey := finalizeTx(tx, m.logic, m.Client, options...)
	if retPayload {
		return tx.ToMap()
	}

	if m.IsAttached() {
		resp, err := m.Client.Repo().AddContributors(&api.BodyAddRepoContribs{
			RepoName:      tx.RepoName,
			ProposalID:    tx.ID,
			PushKeys:      tx.PushKeys,
			FeeCap:        cast.ToFloat64(tx.FeeCap.String()),
			FeeMode:       cast.ToInt(tx.FeeMode),
			Nonce:         tx.Nonce,
			Namespace:     tx.Namespace,
			NamespaceOnly: tx.NamespaceOnly,
			Policies:      tx.Policies,
			Value:         cast.ToFloat64(tx.Value.String()),
			Fee:           cast.ToFloat64(tx.Fee.String()),
			SigningKey:    ed25519.NewKeyFromPrivKey(signingKey),
		})
		if err != nil {
			panic(err)
		}
		return util.ToMap(resp)
	}

	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		panic(se(400, StatusCodeMempoolAddFail, "", err.Error()))
	}

	return map[string]interface{}{
		"hash": hash,
	}
}

// Track adds a repository to the track list.
//  - names: A comma-separated list of repository or namespace names.
func (m *RepoModule) Track(names string, height ...uint64) {
	if err := m.logic.RepoSyncInfoKeeper().Track(names, height...); err != nil {
		panic(se(500, StatusCodeServerErr, "", err.Error()))
	}
}

// UnTrack removes a repository from the track list.
//  - names: A comma-separated list of repository or namespace names.
func (m *RepoModule) UnTrack(names string) {
	if err := m.logic.RepoSyncInfoKeeper().UnTrack(names); err != nil {
		panic(se(500, StatusCodeServerErr, "", err.Error()))
	}
}

// GetTracked returns the tracked repositories
func (m *RepoModule) GetTracked() util.Map {
	return util.ToJSONMap(m.logic.RepoSyncInfoKeeper().Tracked())
}

// GetReposCreatedByAddress returns names of repos created by an address
func (m *RepoModule) GetReposCreatedByAddress(address string) []string {
	bz, err := ed25519.DecodeAddr(address)
	if err != nil {
		panic(se(500, StatusCodeServerErr, "", err.Error()))
	}
	repos, err := m.logic.RepoKeeper().GetReposCreatedByAddress(bz[:])
	if err != nil {
		panic(se(500, StatusCodeServerErr, "", err.Error()))
	}
	return repos
}

// ListPath returns a list of entries in a repository's path
//  - name: The name of the target repository.
//  - path: The file or directory path to list
//  - revision: The revision that will be queried (default: HEAD).
func (m *RepoModule) ListPath(name, path string, revision ...string) []util.Map {

	if name == "" {
		panic(se(400, StatusCodeInvalidParam, "name", "repo name is required"))
	}

	repoPath := m.logic.Config().GetRepoPath(name)
	r, err := m.GetLocalRepo(m.logic.Config().Node.GitBinPath, repoPath)
	if err != nil {
		if err == git.ErrRepositoryNotExists {
			panic(se(404, StatusCodeInvalidParam, "name", err.Error()))
		}
		panic(se(400, StatusCodeInvalidParam, "name", err.Error()))
	}

	if strings.HasPrefix(path, "."+string(os.PathSeparator)) {
		path = path[2:]
	}

	var rev = "HEAD"
	if len(revision) > 0 {
		rev = revision[0]
	}

	items, err := r.ListPath(rev, path)
	if err != nil {
		if err == plumbing.ErrReferenceNotFound {
			return []util.Map{}
		}
		if err == repo.ErrPathNotFound {
			panic(se(404, StatusCodePathNotFound, "path", err.Error()))
		}
		panic(se(500, StatusCodeServerErr, "", err.Error()))
	}

	return util.StructSliceToMap(items)
}

// ReadFileLines returns the lines of a file in a repository.
//  - name: The name of the target repository.
//  - filePath: The file path.
//  - revision: The revision that will be queried (default: HEAD).
func (m *RepoModule) ReadFileLines(name, filePath string, revision ...string) []string {

	if name == "" {
		panic(se(400, StatusCodeInvalidParam, "name", "repo name is required"))
	}

	if filePath == "" {
		panic(se(400, StatusCodeInvalidParam, "file", "file path is required"))
	}

	repoPath := m.logic.Config().GetRepoPath(name)
	r, err := m.GetLocalRepo(m.logic.Config().Node.GitBinPath, repoPath)
	if err != nil {
		if err == git.ErrRepositoryNotExists {
			panic(se(404, StatusCodeInvalidParam, "name", err.Error()))
		}
		panic(se(400, StatusCodeInvalidParam, "name", err.Error()))
	}

	if strings.HasPrefix(filePath, "."+string(os.PathSeparator)) {
		filePath = filePath[2:]
	}

	var rev = "HEAD"
	if len(revision) > 0 {
		rev = revision[0]
	}

	lines, err := r.GetFileLines(rev, filePath)
	if err != nil {
		if err == repo.ErrPathNotFound {
			panic(se(404, StatusCodePathNotFound, "file", err.Error()))
		}
		if err == repo.ErrPathNotAFile {
			panic(se(400, StatusCodePathNotAFile, "file", err.Error()))
		}
		panic(se(500, StatusCodeServerErr, "file", err.Error()))
	}

	return lines
}

// ReadFile returns the string content of a file in a repository.
//  - name: The name of the target repository.
//  - filePath: The file path.
//  - revision: The revision that will be queried (default: HEAD).
func (m *RepoModule) ReadFile(name, filePath string, revision ...string) string {

	if name == "" {
		panic(se(400, StatusCodeInvalidParam, "name", "repo name is required"))
	}

	if filePath == "" {
		panic(se(400, StatusCodeInvalidParam, "file", "file path is required"))
	}

	repoPath := m.logic.Config().GetRepoPath(name)
	r, err := m.GetLocalRepo(m.logic.Config().Node.GitBinPath, repoPath)
	if err != nil {
		if err == git.ErrRepositoryNotExists {
			panic(se(404, StatusCodeInvalidParam, "name", err.Error()))
		}
		panic(se(400, StatusCodeInvalidParam, "name", err.Error()))
	}

	if strings.HasPrefix(filePath, "."+string(os.PathSeparator)) {
		filePath = filePath[2:]
	}

	var rev = "HEAD"
	if len(revision) > 0 {
		rev = revision[0]
	}

	str, err := r.GetFile(rev, filePath)
	if err != nil {
		if err == repo.ErrPathNotFound {
			panic(se(404, StatusCodePathNotFound, "file", err.Error()))
		}
		if err == repo.ErrPathNotAFile {
			panic(se(400, StatusCodePathNotAFile, "file", err.Error()))
		}
		panic(se(500, StatusCodeServerErr, "file", err.Error()))
	}

	return str
}

// GetBranches returns the list of branches
//  - name: The name of the target repository.
func (m *RepoModule) GetBranches(name string) []string {
	if name == "" {
		panic(se(400, StatusCodeInvalidParam, "name", "repo name is required"))
	}

	repoPath := m.logic.Config().GetRepoPath(name)
	r, err := m.GetLocalRepo(m.logic.Config().Node.GitBinPath, repoPath)
	if err != nil {
		if err == git.ErrRepositoryNotExists {
			panic(se(404, StatusCodeInvalidParam, "name", err.Error()))
		}
		panic(se(400, StatusCodeInvalidParam, "name", err.Error()))
	}

	branches, err := r.GetBranches()
	if err != nil {
		panic(se(500, StatusCodeServerErr, "", err.Error()))
	}

	for i, branch := range branches {
		branches[i] = "refs/heads/" + branch
	}

	return branches
}

// GetLatestBranchCommit returns the latest commit of a branch in a repository.
//  - name: The name of the target repository.
//  - branch: The name of the branch.
func (m *RepoModule) GetLatestBranchCommit(name, branch string) util.Map {
	if name == "" {
		panic(se(400, StatusCodeInvalidParam, "name", "repo name is required"))
	}

	if branch == "" {
		panic(se(400, StatusCodeInvalidParam, "branch", "branch name is required"))
	}

	repoPath := m.logic.Config().GetRepoPath(name)
	r, err := m.GetLocalRepo(m.logic.Config().Node.GitBinPath, repoPath)
	if err != nil {
		if err == git.ErrRepositoryNotExists {
			panic(se(404, StatusCodeInvalidParam, "name", err.Error()))
		}
		panic(se(400, StatusCodeInvalidParam, "name", err.Error()))
	}

	c, err := r.GetLatestCommit(branch)
	if err != nil {
		if err == plumbing.ErrReferenceNotFound {
			panic(se(404, StatusCodeBranchNotFound, "branch", "branch does not exist"))
		}
		panic(se(500, StatusCodeServerErr, "", err.Error()))
	}

	return util.ToMap(c)
}

// GetCommits returns commits in a branch.
//  - name: The name of the repository.
//  - branch: The target branch.
//  - limit: The number of commit to return. 0 means all.
func (m *RepoModule) GetCommits(name, branch string, limit ...int) []util.Map {
	if name == "" {
		panic(se(400, StatusCodeInvalidParam, "name", "repo name is required"))
	}

	if branch == "" {
		panic(se(400, StatusCodeInvalidParam, "branch", "branch name is required"))
	}

	repoPath := m.logic.Config().GetRepoPath(name)
	r, err := m.GetLocalRepo(m.logic.Config().Node.GitBinPath, repoPath)
	if err != nil {
		if err == git.ErrRepositoryNotExists {
			panic(se(404, StatusCodeInvalidParam, "name", err.Error()))
		}
		panic(se(400, StatusCodeInvalidParam, "name", err.Error()))
	}

	limit_ := 0
	if len(limit) > 0 {
		limit_ = limit[0]
	}

	commits, err := r.GetCommits(branch, limit_)
	if err != nil {
		if err == plumbing.ErrReferenceNotFound {
			panic(se(404, StatusCodeBranchNotFound, "branch", "branch does not exist"))
		}
		panic(se(500, StatusCodeServerErr, "", err.Error()))
	}

	return util.StructSliceToMap(commits)
}

// GetCommit gets a commit.
//  - name: The name of the repository
//  - hash: The commit hash.
func (m *RepoModule) GetCommit(name, hash string) util.Map {
	if name == "" {
		panic(se(400, StatusCodeInvalidParam, "name", "repo name is required"))
	}
	if hash == "" {
		panic(se(400, StatusCodeInvalidParam, "hash", "commit hash is required"))
	}

	repoPath := m.logic.Config().GetRepoPath(name)
	r, err := m.GetLocalRepo(m.logic.Config().Node.GitBinPath, repoPath)
	if err != nil {
		if err == git.ErrRepositoryNotExists {
			panic(se(404, StatusCodeInvalidParam, "name", err.Error()))
		}
		panic(se(400, StatusCodeInvalidParam, "name", err.Error()))
	}

	commit, err := r.GetCommit(hash)
	if err != nil {
		if err == plumbing.ErrObjectNotFound {
			panic(se(404, StatusCodeCommitNotFound, "hash", "commit does not exist"))
		}
		panic(se(500, StatusCodeServerErr, "", err.Error()))
	}

	return util.ToMap(commit)
}

// CountCommits returns the number commits in a branch/reference.
//  - name: The name of the target repository.
//  - ref: The target branch or reference.
func (m *RepoModule) CountCommits(name, ref string) int {
	if name == "" {
		panic(se(400, StatusCodeInvalidParam, "name", "repo name is required"))
	}

	if ref == "" {
		panic(se(400, StatusCodeInvalidParam, "branch", "branch name is required"))
	}

	repoPath := m.logic.Config().GetRepoPath(name)
	r, err := m.GetLocalRepo(m.logic.Config().Node.GitBinPath, repoPath)
	if err != nil {
		if err == git.ErrRepositoryNotExists {
			panic(se(404, StatusCodeInvalidParam, "name", err.Error()))
		}
		panic(se(400, StatusCodeInvalidParam, "name", err.Error()))
	}

	count, err := r.NumCommits(ref, false)
	if err != nil {
		if err == plumbing.ErrReferenceNotFound {
			panic(se(404, StatusCodeBranchNotFound, "branch", "branch does not exist"))
		}
		panic(se(500, StatusCodeServerErr, "", err.Error()))
	}

	return count
}

// GetCommitAncestors returns ancestors of a commit with the given hash.
//  - commitHash: The hash of the commit.
//  - limit: The number of commit to return. 0 means all.
func (m *RepoModule) GetCommitAncestors(name, commitHash string, limit ...int) []util.Map {
	if name == "" {
		panic(se(400, StatusCodeInvalidParam, "name", "repo name is required"))
	}

	if commitHash == "" {
		panic(se(400, StatusCodeInvalidParam, "commitHash", "commit hash is required"))
	}

	repoPath := m.logic.Config().GetRepoPath(name)
	r, err := m.GetLocalRepo(m.logic.Config().Node.GitBinPath, repoPath)
	if err != nil {
		if err == git.ErrRepositoryNotExists {
			panic(se(404, StatusCodeInvalidParam, "name", err.Error()))
		}
		panic(se(400, StatusCodeInvalidParam, "name", err.Error()))
	}

	limit_ := 0
	if len(limit) > 0 {
		limit_ = limit[0]
	}

	commits, err := r.GetCommitAncestors(commitHash, limit_)
	if err != nil {
		if err == plumbing.ErrObjectNotFound {
			panic(se(404, StatusCodeCommitNotFound, "commitHash", "commit does not exist"))
		}
		panic(se(500, StatusCodeServerErr, "", err.Error()))
	}

	return util.StructSliceToMap(commits)
}

// GetParentsAndCommitDiff gets the diff output between a commit and its parent(s).
//  - name: The name of the target repository.
//  - commitHash: The hash of the commit.
func (m *RepoModule) GetParentsAndCommitDiff(name string, commitHash string) util.Map {
	if name == "" {
		panic(se(400, StatusCodeInvalidParam, "name", "repo name is required"))
	}
	if commitHash == "" {
		panic(se(400, StatusCodeInvalidParam, "commitHash", "commit hash is required"))
	}

	repoPath := m.logic.Config().GetRepoPath(name)
	r, err := m.GetLocalRepo(m.logic.Config().Node.GitBinPath, repoPath)
	if err != nil {
		if err == git.ErrRepositoryNotExists {
			panic(se(404, StatusCodeInvalidParam, "name", err.Error()))
		}
		panic(se(400, StatusCodeInvalidParam, "name", err.Error()))
	}

	res, err := r.GetParentAndChildCommitDiff(commitHash)
	if err != nil {
		if err == plumbing.ErrObjectNotFound {
			panic(se(404, StatusCodeCommitNotFound, "commitHash", "commit not found"))
		}
		panic(se(500, StatusCodeServerErr, "", err.Error()))
	}

	return util.ToMap(res)
}

// CreateIssue creates an issue or adds a comment to an issue.
//  - name: The name of the repository.
//  - params: Issue parameters.
//    - id: The new issue ID or ID of an existing issue to reply to.
//    - title: The title of the issue.
//    - body: The body of the issue.
//    - replyHash: The commit hash of a comment being replied to.
//    - reactions: An array of unicode emojis.
//    - labels: A list of labels.
//    - assignees: A list of assignees.
//    - close: Closes the issue status.
func (m *RepoModule) CreateIssue(name string, params map[string]interface{}) util.Map {
	if name == "" {
		panic(se(400, StatusCodeInvalidParam, "name", "repo name is required"))
	}

	repoPath := m.logic.Config().GetRepoPath(name)
	r, err := m.GetLocalRepo(m.logic.Config().Node.GitBinPath, repoPath)
	if err != nil {
		if err == git.ErrRepositoryNotExists {
			panic(se(404, StatusCodeInvalidParam, "name", err.Error()))
		}
		panic(se(400, StatusCodeInvalidParam, "name", err.Error()))
	}

	// Generate a new issue ID
	o := objx.New(params)
	id := cast.ToInt(o.Get("id").Inter())
	if id == 0 {
		id, err = m.PostIDFinder(r, 1, pl.IssueBranchPrefix)
		if err != nil {
			panic(se(500, StatusCodeServerErr, "", err.Error()))
		}
	}

	// Clone the repository and the issue reference.
	// Determine the full issue reference name and check
	// if it exists. If it does not reset to empty string.
	cloneOpts := remotetypes.CloneOptions{Depth: 1, ReferenceName: pl.MakeIssueReference(id)}
	_, err = r.RefGet(cloneOpts.ReferenceName)
	if err != nil {
		cloneOpts.ReferenceName = ""
	}
	cloned, _, err := r.Clone(cloneOpts)
	if err != nil {
		panic(se(500, StatusCodeServerErr, "", errors.Wrap(err, "failed to clone repo").Error()))
	}

	// Create the issue
	args := &issuecmd.IssueCreateArgs{
		ID:                 id,
		Title:              o.Get("title").Str(),
		Body:               o.Get("body").Str(),
		ReplyHash:          o.Get("replyHash").Str(),
		Reactions:          cast.ToStringSlice(o.Get("reactions").Inter()),
		Labels:             cast.ToStringSlice(o.Get("labels").Inter()),
		Assignees:          cast.ToStringSlice(o.Get("assignees").Inter()),
		PostCommentCreator: pl.CreatePostCommit,
	}
	closeIssue := o.Get("close")
	if !closeIssue.IsNil() {
		args.Close = pointer.ToBool(closeIssue.Bool())
	}

	res, err := m.IssueCreate(cloned, args)
	if err != nil {
		panic(se(500, StatusCodeServerErr, "", err.Error()))
	}

	refHash, err := cloned.RefGet(res.Reference)
	if err != nil {
		panic(se(500, StatusCodeServerErr, "", err.Error()))
	}

	// Add cloned repo path to temp repo manager.
	tempRepoID := m.repoSrv.GetTempRepoManager().Add(cloned.GetPath())

	return map[string]interface{}{
		"hash":      refHash,
		"reference": res.Reference,
		"repoID":    tempRepoID,
	}
}

// CreateMergeRequest creates a merge request or adds a comment to an existing merge request.
//  - name: The name of the repository.
//  - params: Issue parameters.
//    - id: The new issue ID or ID of an existing issue to reply to.
//    - base: The base branch name.
//    - baseHash: The base branch hash.
//    - target: The target branch name.
//    - targetHash: The target branch hash.
//    - title: The title of the issue.
//    - body: The body of the issue.
//    - replyHash: The commit hash of a comment being replied to.
//    - reactions: An array of unicode emojis.
//    - close: Closes the issue status.
func (m *RepoModule) CreateMergeRequest(name string, params map[string]interface{}) util.Map {
	if name == "" {
		panic(se(400, StatusCodeInvalidParam, "name", "repo name is required"))
	}

	repoPath := m.logic.Config().GetRepoPath(name)
	r, err := m.GetLocalRepo(m.logic.Config().Node.GitBinPath, repoPath)
	if err != nil {
		if err == git.ErrRepositoryNotExists {
			panic(se(404, StatusCodeInvalidParam, "name", err.Error()))
		}
		panic(se(400, StatusCodeInvalidParam, "name", err.Error()))
	}

	// Generate a new issue ID
	o := objx.New(params)
	id := cast.ToInt(o.Get("id").Inter())
	if id == 0 {
		id, err = m.PostIDFinder(r, 1, pl.MergeRequestBranchPrefix)
		if err != nil {
			panic(se(500, StatusCodeServerErr, "", err.Error()))
		}
	}

	// Clone the repository and the merge request reference.
	// Determine the full issue reference name and check
	// if it exists. If it does not reset to empty string.
	cloneOpts := remotetypes.CloneOptions{Depth: 1, ReferenceName: pl.MakeMergeRequestReference(id)}
	_, err = r.RefGet(cloneOpts.ReferenceName)
	if err != nil {
		cloneOpts.ReferenceName = ""
	}
	cloned, _, err := r.Clone(cloneOpts)
	if err != nil {
		panic(se(500, StatusCodeServerErr, "", errors.Wrap(err, "failed to clone repo").Error()))
	}

	args := &mergecmd.MergeRequestCreateArgs{
		ID:                 id,
		Title:              o.Get("title").Str(),
		Body:               o.Get("body").Str(),
		ReplyHash:          o.Get("replyHash").Str(),
		Reactions:          cast.ToStringSlice(o.Get("reactions").Inter()),
		Base:               o.Get("base").Str(),
		BaseHash:           o.Get("baseHash").Str(),
		Target:             o.Get("target").Str(),
		TargetHash:         o.Get("targetHash").Str(),
		PostCommentCreator: pl.CreatePostCommit,
	}

	closeIssue := o.Get("close")
	if !closeIssue.IsNil() {
		args.Close = pointer.ToBool(closeIssue.Bool())
	}

	res, err := m.MergeRequestCreate(cloned, args)
	if err != nil {
		panic(se(500, StatusCodeServerErr, "", err.Error()))
	}

	refHash, err := cloned.RefGet(res.Reference)
	if err != nil {
		panic(se(500, StatusCodeServerErr, "", err.Error()))
	}

	// Add cloned repo path to temp repo manager.
	tempRepoID := m.repoSrv.GetTempRepoManager().Add(cloned.GetPath())

	return map[string]interface{}{
		"hash":      refHash,
		"reference": res.Reference,
		"repoID":    tempRepoID,
	}
}

// Push signs and pushes a reference in a temporary repository identified by ID.
//   params <map>
//     - id: The unique temporary manager ID of the target repository.
//     - reference: The full reference name of the commit, tag or note to be pushed.
//     - hash: The latest hash of the reference.
//     - value: Set transaction value (if applicable)
//     - fee: Set the transaction fee
//     - nonce: Set the next transaction nonce of the push key owner (optional).
// 	 privateKeyOrPushToken: The private key or push token for signing the transaction
// Blocks until push succeeds.
// Returns the transaction hash on success.
func (m *RepoModule) Push(params map[string]interface{}, privateKeyOrPushToken string) string {

	o := objx.New(params)
	tempRepoMgr := m.repoSrv.GetTempRepoManager()
	path := tempRepoMgr.GetPath(o.Get("id").Str())
	if path == "" {
		panic(se(404, StatusCodeInvalidTempRepoID, "id", "id is expired or invalid"))
	}

	// Get the reference to be pushed and ensure it is valid.
	reference := plumbing.ReferenceName(o.Get("reference").Str())
	if !reference.IsBranch() && !reference.IsNote() && !reference.IsTag() {
		panic(se(400, StatusCodeInvalidReferenceName, "reference", "reference name is not valid"))
	}

	// Private key or push token is required
	if privateKeyOrPushToken == "" {
		panic(se(400, StatusCodeInvalidPrivateKey, "privateKeyOrPushToken",
			"private key is required"))
	}

	// Attempt to decode the privateKeyOrPushToken as though it is a push token.
	// If we succeed, extract the push key from the token
	var pushKeyID string
	if txDetail, _ := pushtoken.Decode(privateKeyOrPushToken); txDetail != nil {
		pushKeyID = txDetail.PushKeyID
	}

	// If the push key is not already known, we assume privateKeyOrPushToken is
	// a base58 encoded private key and as such we attempt to decode it.
	var privKey *ed25519.PrivKey
	var err error
	if pushKeyID == "" {
		privKey, err = ed25519.PrivKeyFromBase58(privateKeyOrPushToken)
		if err != nil {
			panic(se(400, StatusCodeInvalidPrivateKey, "privateKeyOrPushToken",
				"private key or push token is not a valid"))
		}
		pushKeyID = privKey.Wrap().PushAddr().String()
	}

	// Get the working repository
	r, err := m.GetLocalRepo(m.logic.Config().Node.GitBinPath, path)
	if err != nil {
		if err == git.ErrRepositoryNotExists {
			panic(se(404, StatusCodeRepoNotFound, "name", err.Error()))
		}
		panic(se(500, StatusCodeServerErr, "name", err.Error()))
	}

	// Create push token if a private key was provided
	token := privateKeyOrPushToken
	if privKey != nil {
		txDetail := &remotetypes.TxDetail{
			RepoName:  r.GetName(),
			Fee:       util.String(o.Get("fee").Str()),
			Value:     util.String(o.Get("value").Str()),
			Nonce:     cast.ToUint64(o.Get("nonce").Str()),
			PushKeyID: pushKeyID,
			Reference: reference.String(),
			Head:      o.Get("hash").Str(),
		}

		// Get the next nonce, if not set
		if txDetail.Nonce == 0 {
			senderAcct := m.logic.AccountKeeper().Get(privKey.Wrap().Addr())
			txDetail.Nonce = senderAcct.Nonce.UInt64() + 1
		}

		// Create a push token
		token = pushtoken.MakeFromKey(privKey.Wrap(), txDetail)
	}

	// Construct and set the remote address.
	// The remote (origin) must point to a remote server.
	remoteAddr := config.DefaultRemoteServerAddress
	if remoteAddr[:1] == ":" {
		remoteAddr = "127.0.0.1" + remoteAddr
	}
	url := fmt.Sprintf("http://%s/r/%s", remoteAddr, r.GetName())
	curConfig, err := r.Config()
	if err != nil {
		panic(se(500, StatusCodeServerErr, "", err.Error()))
	}
	curConfig.Remotes["origin"] = &gogitcfg.RemoteConfig{Name: "origin", URLs: []string{url}}
	if err = r.SetConfig(curConfig); err != nil {
		panic(se(500, StatusCodeServerErr, "", err.Error()))
	}

	// Push to remote
	progress, err := r.Push(remotetypes.PushOptions{
		RefSpec: fmt.Sprintf("+%s:%s", reference, reference),
		Token:   token,
	})
	if err != nil {
		panic(se(500, StatusCodePushFailure, "", err.Error()))
	}

	_ = tempRepoMgr.Remove(o.Get("id").Str())

	hash := strings.Split(progress.String(), "hash: ")[1]
	hash = strings.TrimSpace(stripansi.Strip(hash))
	return hash
}
