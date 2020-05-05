package server

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"gitlab.com/makeos/mosdef/api/rest"
	dhttypes "gitlab.com/makeos/mosdef/dht/types"
	"gitlab.com/makeos/mosdef/node/types"
	"gitlab.com/makeos/mosdef/pkgs/cache"
	"gitlab.com/makeos/mosdef/remote/plumbing"
	"gitlab.com/makeos/mosdef/remote/policy"
	"gitlab.com/makeos/mosdef/remote/pruner"
	"gitlab.com/makeos/mosdef/remote/pushhandler"
	"gitlab.com/makeos/mosdef/remote/pushpool"
	repo2 "gitlab.com/makeos/mosdef/remote/repo"
	"gitlab.com/makeos/mosdef/remote/validation"
	types2 "gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/modules"
	"gitlab.com/makeos/mosdef/types/state"

	"github.com/tendermint/tendermint/p2p"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/params"
	"gitlab.com/makeos/mosdef/pkgs/logger"
	"gitlab.com/makeos/mosdef/util"
	"gopkg.in/src-d/go-git.v4"
)

const (
	// PushNoteReactorChannel is the channel id sending/receiving push notes
	PushNoteReactorChannel = byte(0x32)
	// PushEndReactorChannel is the channel id for sending/receiving push endorsements
	PushEndReactorChannel = byte(0x33)
)

var services = [][]interface{}{
	{"(.*?)/git-upload-pack$", service{method: "POST", handle: serveService}},
	{"(.*?)/git-receive-pack$", service{method: "POST", handle: serveService}},
	{"(.*?)/info/refs$", service{method: "GET", handle: getInfoRefs}},
	{"(.*?)/HEAD$", service{method: "GET", handle: GetTextFile}},
	{"(.*?)/objects/info/alternates$", service{method: "GET", handle: GetTextFile}},
	{"(.*?)/objects/info/http-alternates$", service{method: "GET", handle: GetTextFile}},
	{"(.*?)/objects/info/packs$", service{method: "GET", handle: GetInfoPacks}},
	{"(.*?)/objects/info/[^/]*$", service{method: "GET", handle: GetTextFile}},
	{"(.*?)/objects/[0-9a-f]{2}/[0-9a-f]{38}$", service{method: "GET", handle: GetInfoPacks}},
	{"(.*?)/objects/pack/pack-[0-9a-f]{40}\\.pack$", service{method: "GET", handle: GetPackFile}},
	{"(.*?)/objects/pack/pack-[0-9a-f]{40}\\.idx$", service{method: "GET", handle: GetIdxFile}},
}

// Server implements types.Server. It provides a system for managing
// and service a git repositories through http and ssh protocols.
type Server struct {
	p2p.BaseReactor
	cfg                      *config.AppConfig
	log                      logger.Logger                           // log is the application logger
	wg                       *sync.WaitGroup                         // wait group for waiting for the remote server
	srv                      *http.Server                            // the http server
	rootDir                  string                                  // the root directory where all repos are stored
	addr                     string                                  // addr is the listening address for the http server
	gitBinPath               string                                  // gitBinPath is the path of the git executable
	pushPool                 core.PushPool                           // The transaction pool for push transactions
	mempool                  core.Mempool                            // The general transaction pool for block-bound transaction
	logic                    core.Logic                              // logic is the application logic provider
	privValidatorKey         *crypto.Key                             // the node's private validator key for signing transactions
	pushKeyGetter            core.PushKeyGetter                      // finds and returns PGP public key
	dht                      dhttypes.DHTNode                        // The dht service
	pruner                   core.Pruner                             // The repo runner
	blockGetter              types.BlockGetter                       // Provides access to blocks
	pushNoteSenders          *cache.Cache                            // Store senders of push notes
	pushEndSenders           *cache.Cache                            // Stores senders of PushEndorsement messages
	pushEndorsements         *cache.Cache                            // Store PushEnds
	modulesAgg               modules.ModuleHub                       // Modules aggregator
	authenticate             AuthenticatorFunc                       // Function for performing authentication
	checkPushNote            validation.PushNoteCheckFunc            // Function for performing PushNote validation
	packfileMaker            pushhandler.ReferenceUpdateRequestMaker // Function for creating a packfile for updating a repository
	makePushHandler          pushhandler.PushHandlerFunc             // Function for creating a push handler
	pushedObjectsBroadcaster pushedObjectsBroadcaster                // Function for broadcasting a push note and pushed objects
}

// NewManager creates an instance of Server
func NewManager(
	cfg *config.AppConfig,
	addr string,
	logic core.Logic,
	dht dhttypes.DHTNode,
	mempool core.Mempool,
	blockGetter types.BlockGetter) *Server {

	wg := &sync.WaitGroup{}
	wg.Add(1)

	key, _ := cfg.G().PrivVal.GetKey()
	server := &Server{
		cfg:              cfg,
		log:              cfg.G().Log.Module("remote-server"),
		addr:             addr,
		rootDir:          cfg.GetRepoRoot(),
		gitBinPath:       cfg.Node.GitBinPath,
		wg:               wg,
		pushPool:         pushpool.NewPushPool(params.PushPoolCap, logic, dht),
		logic:            logic,
		privValidatorKey: key,
		dht:              dht,
		mempool:          mempool,
		blockGetter:      blockGetter,
		authenticate:     authenticate,
		checkPushNote:    validation.CheckPushNote,
		packfileMaker:    pushhandler.MakeReferenceUpdateRequest,
		pushNoteSenders:  cache.NewActiveCache(params.PushObjectsSendersCacheSize),
		pushEndSenders:   cache.NewActiveCache(params.PushObjectsSendersCacheSize),
		pushEndorsements: cache.NewActiveCache(params.PushNotesEndorsementsCacheSize),
	}

	server.makePushHandler = server.createPushHandler
	server.pushKeyGetter = server.getPushKey
	server.pushedObjectsBroadcaster = server.broadcastPushedObjects
	server.BaseReactor = *p2p.NewBaseReactor("Reactor", server)
	server.pruner = pruner.NewPruner(server, server.rootDir)

	return server
}

// SetRootDir sets the directory where repositories are stored
func (sv *Server) SetRootDir(dir string) {
	sv.rootDir = dir
}

// RegisterAPIHandlers registers server API handlers
func (sv *Server) RegisterAPIHandlers(agg modules.ModuleHub) {
	sv.modulesAgg = agg
	sv.registerAPIHandlers(sv.srv.Handler.(*http.ServeMux))
}

func (sv *Server) getPushKey(pushKeyID string) (crypto.PublicKey, error) {
	pk := sv.logic.PushKeyKeeper().Get(pushKeyID)
	if pk.IsNil() {
		return crypto.EmptyPublicKey, fmt.Errorf("push key does not exist")
	}
	return pk.PubKey, nil
}

// cacheNoteSender caches a push note sender
func (sv *Server) cacheNoteSender(senderID string, noteID string) {
	key := util.Hash20Hex([]byte(senderID + noteID))
	sv.pushNoteSenders.AddWithExp(key, struct{}{}, time.Now().Add(10*time.Minute))
}

// cachePushEndSender caches a push endorsement sender
func (sv *Server) cachePushEndSender(senderID string, pushEndID string) {
	key := util.Hash20Hex([]byte(senderID + pushEndID))
	sv.pushEndSenders.AddWithExp(key, struct{}{}, time.Now().Add(60*time.Minute))
}

// isPushNoteSender checks whether a push note was sent by the given sender ID
func (sv *Server) isPushNoteSender(senderID string, noteID string) bool {
	key := util.Hash20Hex([]byte(senderID + noteID))
	v := sv.pushNoteSenders.Get(key)
	return v == struct{}{}
}

// isPushEndSender checks whether a push endorsement was sent by the given sender ID
func (sv *Server) isPushEndSender(senderID string, pushEndID string) bool {
	key := util.Hash20Hex([]byte(senderID + pushEndID))
	v := sv.pushEndSenders.Get(key)
	return v == struct{}{}
}

// addPushNoteEndorsement indexes a PushEndorsement for a given push note
func (sv *Server) addPushNoteEndorsement(noteID string, pushEnd *core.PushEndorsement) {
	pushEndList := sv.pushEndorsements.Get(noteID)
	if pushEndList == nil {
		pushEndList = map[string]*core.PushEndorsement{}
	}
	pushEndList.(map[string]*core.PushEndorsement)[pushEnd.ID().String()] = pushEnd
	sv.pushEndorsements.Add(noteID, pushEndList)
}

// Start starts the server that serves the repos.
// Implements p2p.Reactor
func (sv *Server) Start() error {
	s := http.NewServeMux()

	if !sv.cfg.IsValidatorNode() {
		s.HandleFunc("/", sv.gitRequestsHandler)
	}

	sv.log.Info("Server has started", "Address", sv.addr)
	sv.srv = &http.Server{Addr: sv.addr, Handler: s}
	go func() {
		sv.srv.ListenAndServe()
		sv.wg.Done()
	}()

	go sv.subscribe()
	go sv.pruner.Start()

	return nil
}

func (sv *Server) registerAPIHandlers(s *http.ServeMux) {
	api := rest.NewAPI(sv.modulesAgg, sv.log)
	api.RegisterEndpoints(s)
}

// GetLogic returns the application logic provider
func (sv *Server) GetLogic() core.Logic {
	return sv.logic
}

// GetPrivateValidatorKey implements RepositoryManager
func (sv *Server) GetPrivateValidatorKey() *crypto.Key {
	return sv.privValidatorKey
}

// GetPruner returns the repo pruner
func (sv *Server) GetPruner() core.Pruner {
	return sv.pruner
}

// GetPushPool returns the push pool
func (sv *Server) GetPushPool() core.PushPool {
	return sv.pushPool
}

// GetMempool returns the transaction pool
func (sv *Server) GetMempool() core.Mempool {
	return sv.mempool
}

// GetDHT returns the dht service
func (sv *Server) GetDHT() dhttypes.DHTNode {
	return sv.dht
}

// Cfg returns the application config
func (sv *Server) Cfg() *config.AppConfig {
	return sv.cfg
}

func (sv *Server) getRepoPath(name string) string {
	return filepath.Join(sv.rootDir, name)
}

// gitRequestsHandler handles incoming http request from a git client
func (sv *Server) gitRequestsHandler(w http.ResponseWriter, r *http.Request) {

	sv.log.Debug("New request", "Method", r.Method, "URL", r.URL.String())

	// De-construct the URL to get the repo name and operation
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	namespaceName := pathParts[0]
	repoName := pathParts[1]
	op := pathParts[2]

	// Resolve the namespace if the given namespace is not the default
	var namespace *state.Namespace
	if namespaceName != repo2.DefaultNS {

		// Get the namespace, return 404 if not found
		namespace = sv.logic.NamespaceKeeper().Get(util.HashNamespace(namespaceName))
		if namespace.IsNil() {
			w.WriteHeader(http.StatusNotFound)
			sv.log.Debug("Unknown repository", "Name", repoName, "Code", http.StatusNotFound,
				"Status", http.StatusText(http.StatusNotFound))
			return
		}

		// Get the target. If the target is not set or the target is not
		// prefixed with r/, return 404
		target := namespace.Domains.Get(repoName)
		if target == "" || target[:2] != "r/" {
			w.WriteHeader(http.StatusNotFound)
			sv.log.Debug("Unknown repository", "Name", repoName, "Code", http.StatusNotFound,
				"Status", http.StatusText(http.StatusNotFound))
			return
		}

		repoName = target[2:]
	}

	// Check if the repository exist
	fullRepoDir := sv.getRepoPath(repoName)
	repoState := sv.logic.RepoKeeper().Get(repoName)
	if repoState.IsNil() {
		w.WriteHeader(http.StatusNotFound)
		sv.log.Debug("Unknown repository", "Name", repoName, "Code", http.StatusNotFound,
			"Status", http.StatusText(http.StatusNotFound))
		return
	}

	// Authenticate pusher
	txDetails, polEnforcer, err := sv.handleAuth(r, w, repoState, namespace)
	if err != nil {
		w.Header().Set("WWW-Authenticate", "Basic")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// Attempt to load the repository at the given path
	repo, err := git.PlainOpen(fullRepoDir)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if err == git.ErrRepositoryNotExists {
			statusCode = http.StatusNotFound
		}
		w.WriteHeader(statusCode)
		sv.log.Debug("Failed to open target repository",
			"Name", repoName,
			"Code", statusCode,
			"Status", http.StatusText(statusCode))
		return
	}

	req := &RequestContext{
		W:           w,
		R:           r,
		Operation:   op,
		TxDetails:   txDetails,
		PolEnforcer: polEnforcer,
		Repo: &repo2.Repo{
			Name:          repoName,
			Repository:    repo,
			LiteGit:       repo2.NewLiteGit(sv.gitBinPath, fullRepoDir),
			Path:          fullRepoDir,
			State:         repoState,
			NamespaceName: namespaceName,
			Namespace:     namespace,
		},
		RepoDir:     fullRepoDir,
		ServiceName: GetService(r),
		GitBinPath:  sv.gitBinPath,
	}

	req.PushHandler = sv.makePushHandler(req.Repo, txDetails, polEnforcer)

	for _, s := range services {
		srvPattern := s[0].(string)
		srv := s[1].(service)

		re := regexp.MustCompile(srvPattern)
		if m := re.FindStringSubmatch(r.URL.Path); m == nil {
			continue
		}

		if srv.method != r.Method {
			WriteMethodNotAllowed(w, r)
			return
		}

		err := srv.handle(req)
		if err != nil {
			sv.log.Error("failed to handle request", "Req", srvPattern, "Err", err)
		}

		return
	}

	WriteMethodNotAllowed(w, r)
}

// GetPushKeyGetter implements RepositoryManager
func (sv *Server) GetPushKeyGetter() core.PushKeyGetter {
	return sv.pushKeyGetter
}

// createPushHandler creates an instance of Handler
func (sv *Server) createPushHandler(
	targetRepo core.BareRepo,
	txDetails []*types2.TxDetail,
	enforcer policy.EnforcerFunc) *pushhandler.Handler {
	return pushhandler.NewHandler(targetRepo, txDetails, enforcer, sv)
}

// Log returns the logger
func (sv *Server) Log() logger.Logger {
	return sv.log
}

// SetPGPPubKeyGetter implements SetPGPPubKeyGetter
func (sv *Server) SetPGPPubKeyGetter(pkGetter core.PushKeyGetter) {
	sv.pushKeyGetter = pkGetter
}

// GetRepoState implements RepositoryManager
func (sv *Server) GetRepoState(repo core.BareRepo, options ...core.KVOption) (core.BareRepoState, error) {
	return plumbing.GetRepoState(repo, options...), nil
}

// Wait can be used by the caller to wait till the server terminates
func (sv *Server) Wait() {
	sv.wg.Wait()
}

// FindObject implements dht.ObjectFinder
func (sv *Server) FindObject(key []byte) ([]byte, error) {

	repoName, objHash, err := plumbing.ParseRepoObjectDHTKey(string(key))
	if err != nil {
		return nil, fmt.Errorf("invalid repo object key")
	}

	if len(objHash) != 40 {
		return nil, fmt.Errorf("invalid object hash")
	}

	repo, err := repo2.GetRepo(sv.getRepoPath(repoName))
	if err != nil {
		return nil, err
	}

	bz, err := repo.GetCompressedObject(objHash)
	if err != nil {
		return nil, err
	}

	return bz, nil
}

// Get returns a repo handle
func (sv *Server) GetRepo(name string) (core.BareRepo, error) {
	return repo2.GetRepoWithLiteGit(sv.gitBinPath, sv.getRepoPath(name))
}

// Shutdown shuts down the server
func (sv *Server) Shutdown(ctx context.Context) {
	sv.log.Info("Shutting down")
	if sv.srv != nil {
		sv.srv.Shutdown(ctx)
	}
	sv.pruner.Stop()
}

// Stop implements Reactor
func (sv *Server) Stop() error {
	sv.BaseReactor.Stop()
	sv.Shutdown(context.Background())
	sv.log.Info("Shutdown")
	return nil
}
