package modules

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/c-bata/go-prompt"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/make-os/kit/crypto/ed25519"
	modulestypes "github.com/make-os/kit/modules/types"
	"github.com/make-os/kit/node/services"
	repo2 "github.com/make-os/kit/remote/repo"
	types2 "github.com/make-os/kit/rpc/types"
	"github.com/make-os/kit/types"
	"github.com/make-os/kit/types/api"
	"github.com/make-os/kit/types/constants"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/crypto"
	"github.com/make-os/kit/util/identifier"
	"github.com/robertkrimen/otto"
	"github.com/spf13/cast"
)

// RepoModule provides repository functionalities to JS environment
type RepoModule struct {
	modulestypes.ModuleCommon
	logic   core.Logic
	service services.Service
	repoSrv core.RemoteServer
}

// NewAttachableRepoModule creates an instance of RepoModule suitable in attach mode
func NewAttachableRepoModule(client types2.Client) *RepoModule {
	return &RepoModule{ModuleCommon: modulestypes.ModuleCommon{Client: client}}
}

// NewRepoModule creates an instance of RepoModule
func NewRepoModule(service services.Service, repoSrv core.RemoteServer, logic core.Logic) *RepoModule {
	return &RepoModule{service: service, logic: logic, repoSrv: repoSrv}
}

// methods are functions exposed in the special namespace of this module.
func (m *RepoModule) methods() []*modulestypes.VMMember {
	return []*modulestypes.VMMember{
		{Name: "create", Value: m.Create, Description: "Create a git repository on the network"},
		{Name: "get", Value: m.Get, Description: "Get and return a repository"},
		{Name: "update", Value: m.Update, Description: "Update a repository"},
		{Name: "upsertOwner", Value: m.UpsertOwner, Description: "Create a proposal to add or update a repository owner"},
		{Name: "vote", Value: m.Vote, Description: "Vote for or against a proposal"},
		{Name: "depositPropFee", Value: m.DepositProposalFee, Description: "Deposit fees into a proposal"},
		{Name: "addContributor", Value: m.AddContributor, Description: "Register one or more push keys as contributors"},
		{Name: "track", Value: m.Track, Description: "Track one or more repositories"},
		{Name: "untrack", Value: m.UnTrack, Description: "Untrack one or more repositories"},
		{Name: "tracked", Value: m.GetTracked, Description: "Returns the tracked repositories"},
		{Name: "ls", Value: m.ListPath, Description: "List files and directories in a repository's path"},
	}
}

// globals are functions exposed in the VM's global namespace
func (m *RepoModule) globals() []*modulestypes.VMMember {
	return []*modulestypes.VMMember{}
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
		vm.Set(f.Name, f.Value)
		m.Suggestions = append(m.Suggestions, prompt.Suggest{Text: f.Name, Description: f.Description})
	}

	return m.Completer
}

// create registers a git repository on the network
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
			Config:     tx.Config,
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

// upsertOwner creates a proposal to add or update a repository owner
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

// voteOnProposal sends a TxTypeRepoCreate transaction to create a git repository
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
func (m *RepoModule) Get(name string, opts ...modulestypes.GetOptions) util.Map {
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

	repo := m.logic.RepoKeeper().Get(name, blockHeight)

	if repo.IsNil() {
		panic(se(404, StatusCodeRepoNotFound, "name", types.ErrRepoNotFound.Error()))
	}

	if len(selectors) > 0 {
		selected, err := Select(util.MustToJSON(repo), selectors...)
		if err != nil {
			panic(se(400, StatusCodeInvalidParam, "select", err.Error()))
		}
		return selected
	}

	return util.ToMap(repo)
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

// Register creates a proposal to register one or more push keys
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

// ListPath returns a list of entries in a repository's path
//  - name: The name of the target repository.
//  - path: The file or directory path to list
//  - revision: The revision that will be queried (default: HEAD).
func (m *RepoModule) ListPath(name, path string, revision ...string) []util.Map {

	if name == "" {
		panic(se(400, StatusCodeInvalidParam, "name", "repo name is required"))
	}

	repoPath := filepath.Join(m.logic.Config().GetRepoRoot(), name)
	repo, err := repo2.GetWithGitModule(m.logic.Config().Node.GitBinPath, repoPath)
	if err != nil {
		panic(se(400, StatusCodeInvalidParam, "name", err.Error()))
	}

	if strings.HasPrefix(path, "."+string(os.PathSeparator)) {
		path = path[2:]
	}

	var rev = "HEAD"
	if len(revision) > 0 {
		rev = revision[0]
	}

	items, err := repo.ListPath(rev, path)
	if err != nil {
		if err == repo2.ErrPathNotFound {
			panic(se(404, StatusCodePathNotFound, "path", err.Error()))
		}
		if err == plumbing.ErrReferenceNotFound {
			panic(se(404, StatusCodeReferenceNotFound, "revision", err.Error()))
		}
		panic(se(500, StatusCodeServerErr, "", err.Error()))
	}

	return util.StructSliceToMap(items)
}
