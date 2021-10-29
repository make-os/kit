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

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/format/pktline"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/crypto/ed25519"
	dht2 "github.com/make-os/kit/net/dht"
	"github.com/make-os/kit/net/dht/announcer"
	nodeService "github.com/make-os/kit/node/services"
	"github.com/make-os/kit/params"
	"github.com/make-os/kit/pkgs/cache"
	"github.com/make-os/kit/pkgs/logger"
	"github.com/make-os/kit/remote/fetcher"
	"github.com/make-os/kit/remote/plumbing"
	"github.com/make-os/kit/remote/policy"
	"github.com/make-os/kit/remote/push"
	"github.com/make-os/kit/remote/push/pool"
	pushtypes "github.com/make-os/kit/remote/push/types"
	"github.com/make-os/kit/remote/refsync"
	rstypes "github.com/make-os/kit/remote/refsync/types"
	"github.com/make-os/kit/remote/repo"
	"github.com/make-os/kit/remote/temprepomgr"
	remotetypes "github.com/make-os/kit/remote/types"
	"github.com/make-os/kit/remote/validation"
	"github.com/make-os/kit/rpc"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/types/state"
	crypto2 "github.com/make-os/kit/util/crypto"
	"github.com/pkg/errors"
	"github.com/tendermint/tendermint/p2p"
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
	{"(.*?)/HEAD$", service{method: "GET", handle: getTextFile}},
	{"(.*?)/objects/info/alternates$", service{method: "GET", handle: getTextFile}},
	{"(.*?)/objects/info/http-alternates$", service{method: "GET", handle: getTextFile}},
	{"(.*?)/objects/info/packs$", service{method: "GET", handle: getInfoPacks}},
	{"(.*?)/objects/info/[^/]*$", service{method: "GET", handle: getTextFile}},
	{"(.*?)/objects/[0-9a-f]{2}/[0-9a-f]{38}$", service{method: "GET", handle: getInfoPacks}},
	{"(.*?)/objects/pack/pack-[0-9a-f]{40}\\.pack$", service{method: "GET", handle: getPackFile}},
	{"(.*?)/objects/pack/pack-[0-9a-f]{40}\\.idx$", service{method: "GET", handle: getIdxFile}},
}

// Server implements types.Server. It provides a system for managing
// and service a git repositories through http and ssh protocols.
type Server struct {
	p2p.BaseReactor
	cfg           *config.AppConfig
	log           logger.Logger               // log is the application logger
	wg            *sync.WaitGroup             // wait group for waiting for the remote server
	mux           *http.ServeMux              // The request multiplexer
	srv           *http.Server                // The http server
	rpcHandler    *rpc.Handler                // JSON-RPC 2.0 handler
	rootDir       string                      // the root directory where all repos are stored
	validatorKey  *ed25519.Key                // the node's private validator key for signing transactions
	addr          string                      // addr is the listening address for the http server
	gitBinPath    string                      // gitBinPath is the path of the git executable
	pushPool      pushtypes.PushPool          // The transaction pool for push transactions
	mempool       core.Mempool                // The general transaction pool for block-bound transaction
	logic         core.Logic                  // logic is the application logic provider
	nodeService   nodeService.Service         // The node external service provider
	pushKeyGetter core.PushKeyGetter          // finds and returns PGP public key
	dht           dht2.DHT                    // The dht service
	objFetcher    fetcher.ObjectFetcher       // The object fetcher service
	blockGetter   core.BlockGetter            // Provides access to blocks
	refSyncer     rstypes.RefSync             // Responsible for syncing pushed references in a push transaction
	tmpRepoMgr    temprepomgr.TempRepoManager // The temporary repo manager

	// Indexes
	noteSenders        *cache.Cache // Store senders of push notes
	endorsementSenders *cache.Cache // Stores senders of Endorsement messages
	endorsements       *cache.Cache // Stores push endorsements
	notesReceived      *cache.Cache // Stores ID of push notes recently received

	// Composable functions members
	authenticate               AuthenticatorFunc                       // Function for performing authentication
	checkPushNote              validation.CheckPushNoteFunc            // Function for performing PushNote validation
	makeReferenceUpdatePack    push.MakeReferenceUpdateRequestPackFunc // Function for creating a reference update pack for updating a repository
	makePushHandler            PushHandlerFunc                         // Function for creating a push handler
	noteAndEndorserBroadcaster BroadcastNoteAndEndorsementFunc         // Function for broadcasting a push note and its endorsement
	makePushTx                 CreatePushTxFunc                        // Function for creating a push transaction and adding it to the mempool
	processPushNote            MaybeProcessPushNoteFunc                // Function for processing a push note
	checkEndorsement           validation.CheckEndorsementFunc         // Function for checking push endorsement
	endorsementBroadcaster     BroadcastEndorsementFunc                // Function for broadcasting an endorsement
	noteBroadcaster            BroadcastPushNoteFunc                   // Function for broadcasting a push note
	endorsementCreator         CreateEndorsementFunc                   // Function for creating an endorsement for a given push note
	tryScheduleReSync          ScheduleReSyncFunc                      // Function for scheduling a resync of a repository
}

// New creates an instance of Server
func New(
	cfg *config.AppConfig,
	addr string,
	appLogic core.Logic,
	dht dht2.DHT,
	mempool core.Mempool,
	nodeService nodeService.Service,
	blockGetter core.BlockGetter,
) *Server {

	// Create wait group
	wg := &sync.WaitGroup{}
	wg.Add(1)

	// Create fetcher instance
	mFetcher := fetcher.NewFetcher(dht, cfg)

	// Get the private validator key
	key, _ := cfg.G().PrivVal.GetKey()

	// Create the push pool
	pushPool := pool.NewPushPool(params.PushPoolCap, appLogic)

	// Create an instance of Server
	server := &Server{
		cfg:                     cfg,
		log:                     cfg.G().Log.Module("remote-server"),
		addr:                    addr,
		mux:                     http.NewServeMux(),
		rootDir:                 cfg.GetRepoRoot(),
		gitBinPath:              cfg.Node.GitBinPath,
		wg:                      wg,
		pushPool:                pushPool,
		nodeService:             nodeService,
		logic:                   appLogic,
		validatorKey:            key,
		dht:                     dht,
		objFetcher:              mFetcher,
		mempool:                 mempool,
		blockGetter:             blockGetter,
		refSyncer:               refsync.New(cfg, pushPool, mFetcher, dht, appLogic),
		tmpRepoMgr:              temprepomgr.New(),
		authenticate:            authenticate,
		checkPushNote:           validation.CheckPushNote,
		makeReferenceUpdatePack: push.MakeReferenceUpdateRequestPack,
		noteSenders:             cache.NewCacheWithExpiringEntry(params.PushNotesEndorsementsCacheSize),
		endorsementSenders:      cache.NewCacheWithExpiringEntry(params.PushObjectsSendersCacheSize),
		endorsements:            cache.NewCacheWithExpiringEntry(params.RecentlySeenPacksCacheSize),
		notesReceived:           cache.NewCacheWithExpiringEntry(params.NotesReceivedCacheSize),
		checkEndorsement:        validation.CheckEndorsement,
	}

	// Instantiate RPC handler
	server.rpcHandler = rpc.New(server.mux, cfg)

	// Set concrete functions for various function typed fields
	server.makePushHandler = server.createPushHandler
	server.pushKeyGetter = server.getPushKey
	server.noteAndEndorserBroadcaster = server.BroadcastNoteAndEndorsement
	server.makePushTx = server.createPushTx
	server.endorsementBroadcaster = server.broadcastEndorsement
	server.noteBroadcaster = server.broadcastPushNote
	server.endorsementCreator = createEndorsement
	server.processPushNote = server.maybeProcessPushNote
	server.tryScheduleReSync = server.maybeScheduleReSync

	// Instantiate the base reactor
	server.BaseReactor = *p2p.NewBaseReactor("Reactor", server)

	// Start reference synchronization and object fetcher in non-validator or test mode.
	if !cfg.Node.Validator && cfg.Node.Mode != config.ModeTest {
		server.objFetcher.Start()
	}

	// Register DHT object checkers
	if dht != nil {
		dht.RegisterChecker(announcer.ObjTypeRepoName, server.checkRepo)
		dht.RegisterChecker(announcer.ObjTypeGit, server.checkRepoObject)
	}

	// Apply repo tracking configurations
	server.applyRepoTrackingConfig()

	return server
}

// applyRepoTrackingConfig handles repo tracking related configs
func (sv *Server) applyRepoTrackingConfig() {

	// Add repositories to the tracking list
	for _, repository := range sv.cfg.Repo.Track {
		_ = sv.logic.RepoSyncInfoKeeper().Track(repository)
	}

	// Remove repositories from tracking list
	for _, repository := range sv.cfg.Repo.Untrack {
		_ = sv.logic.RepoSyncInfoKeeper().UnTrack(repository)
	}

	// If request to untrack all repos is enabled, untrack all.
	if sv.cfg.Repo.UntrackAll {
		for repository := range sv.logic.RepoSyncInfoKeeper().Tracked() {
			_ = sv.logic.RepoSyncInfoKeeper().UnTrack(repository)
		}
	}
}

// SetRootDir sets the directory where repositories are stored
func (sv *Server) SetRootDir(dir string) {
	sv.rootDir = dir
}

// GetFetcher returns the fetcher service
func (sv *Server) GetFetcher() fetcher.ObjectFetcher {
	return sv.objFetcher
}

// GetTempRepoManager returns the temporary repository manager
func (sv *Server) GetTempRepoManager() temprepomgr.TempRepoManager {
	return sv.tmpRepoMgr
}

// getPushKey returns a pusher key by its ID
func (sv *Server) getPushKey(pushKeyID string) (ed25519.PublicKey, error) {
	pk := sv.logic.PushKeyKeeper().Get(pushKeyID)
	if pk.IsNil() {
		return ed25519.EmptyPublicKey, fmt.Errorf("push key does not exist")
	}
	return pk.PubKey, nil
}

// checkRepo implements dht.CheckFunc for checking
// the existence of a repository.
func (sv *Server) checkRepo(_ string, key []byte) bool {
	_, err := sv.GetRepo(string(key))
	return err == nil
}

// CheckNote validates a push note
func (sv *Server) CheckNote(note pushtypes.PushNote) error {
	return sv.checkPushNote(note, sv.logic)
}

// TryScheduleReSync may schedule a local reference for resynchronization if the pushed
// reference old state does not match the current network state of the reference
func (sv *Server) TryScheduleReSync(note pushtypes.PushNote, ref string, fromBeginning bool) error {
	return sv.tryScheduleReSync(note, ref, fromBeginning)
}

// checkRepoObject implements dht.CheckFunc for checking the existence
// of an object in the given repository.
func (sv *Server) checkRepoObject(repo string, key []byte) bool {
	r, err := sv.GetRepo(repo)
	if err != nil {
		return false
	}
	return r.GetStorer().HasEncodedObject(plumbing.BytesToHash(key)) == nil
}

// registerNoteSender caches a push note sender
func (sv *Server) registerNoteSender(senderID string, noteID string) {
	key := crypto2.Hash20Hex([]byte(senderID + noteID))
	sv.noteSenders.Add(key, struct{}{}, time.Now().Add(10*time.Minute))
}

// registerEndorsementSender caches a push endorsement sender
func (sv *Server) registerEndorsementSender(senderID string, pushEndID string) {
	key := crypto2.Hash20Hex([]byte(senderID + pushEndID))
	sv.endorsementSenders.Add(key, struct{}{}, time.Now().Add(30*time.Minute))
}

// isNoteSender checks whether a push note was sent by the given sender ID
func (sv *Server) isNoteSender(senderID string, noteID string) bool {
	key := crypto2.Hash20Hex([]byte(senderID + noteID))
	return sv.noteSenders.Get(key) == struct{}{}
}

// isEndorsementSender checks whether a push endorsement was sent by the given sender ID
func (sv *Server) isEndorsementSender(senderID string, pushEndID string) bool {
	key := crypto2.Hash20Hex([]byte(senderID + pushEndID))
	return sv.endorsementSenders.Get(key) == struct{}{}
}

// registerNoteEndorsement indexes a push endorsement for a given push note
func (sv *Server) registerNoteEndorsement(noteID string, endorsement *pushtypes.PushEndorsement) {
	entries := sv.endorsements.Get(noteID)
	if entries == nil {
		entries = map[string]*pushtypes.PushEndorsement{}
	}
	entries.(map[string]*pushtypes.PushEndorsement)[endorsement.ID().String()] = endorsement
	sv.endorsements.Add(noteID, entries)
}

// markNoteAsSeen marks a note as seen
func (sv *Server) markNoteAsSeen(noteID string) {
	key := crypto2.Hash20Hex([]byte(noteID))
	sv.notesReceived.Add(key, struct{}{}, time.Now().Add(5*time.Minute))
}

// isNoteSeen checks if a note has been seen
func (sv *Server) isNoteSeen(noteID string) bool {
	key := crypto2.Hash20Hex([]byte(noteID))
	return sv.notesReceived.Get(key) == struct{}{}
}

// GetRPCHandler returns the RPC handler
func (sv *Server) GetRPCHandler() *rpc.Handler {
	return sv.rpcHandler
}

// Start starts the server that serves the repos.
// Implements p2p.Reactor
func (sv *Server) Start() error {

	// In non-validator mode, apply handler for git requests
	if !sv.cfg.IsValidatorNode() {
		sv.mux.HandleFunc("/", sv.gitRequestsHandler)
	}

	sv.log.Info("Server has started", "Address", sv.addr)
	sv.srv = &http.Server{Addr: sv.addr, Handler: sv.mux}

	go func() {
		if err := sv.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			sv.log.Error("Failed to serve remote server", "Err", err)
		}
		sv.wg.Done()
	}()

	go sv.subscribe()

	return nil
}

// GetLogic returns the application logic provider
func (sv *Server) GetLogic() core.Logic {
	return sv.logic
}

// GetRepo get a local repository
func (sv *Server) GetRepo(name string) (plumbing.LocalRepo, error) {
	repository, err := repo.GetWithGitModule(sv.gitBinPath, sv.getRepoPath(name))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get local repo")
	}
	return repository, nil
}

// GetPrivateValidatorKey implements RepositoryManager
func (sv *Server) GetPrivateValidatorKey() *ed25519.Key {
	return sv.validatorKey
}

// GetPushPool returns the push pool
func (sv *Server) GetPushPool() pushtypes.PushPool {
	return sv.pushPool
}

// GetMempool returns the transaction pool
func (sv *Server) GetMempool() core.Mempool {
	return sv.mempool
}

// GetDHT returns the dht service
func (sv *Server) GetDHT() dht2.DHT {
	return sv.dht
}

// Cfg returns the application config
func (sv *Server) Cfg() *config.AppConfig {
	return sv.cfg
}

func (sv *Server) getRepoPath(name string) string {
	return filepath.Join(sv.rootDir, name)
}

// Announce announces a key on the DHT network
func (sv *Server) Announce(objType int, repo string, hash []byte, doneCB func(error)) bool {
	return sv.dht.Announce(objType, repo, hash, doneCB)
}

// gitRequestsHandler handles incoming http request from a git client
func (sv *Server) gitRequestsHandler(w http.ResponseWriter, r *http.Request) {

	sv.log.Debug("New request", "Method", r.Method, "URL", r.URL.String())

	// handle panics gracefully
	defer func() {
		if rcv, ok := recover().(error); ok {
			w.WriteHeader(http.StatusInternalServerError)
			sv.log.Error("Request error", "Err", rcv.Error())
		}
	}()

	// De-construct the URL to get the repo name and operation
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	namespaceName := pathParts[0]
	repoName := pathParts[1]
	op := pathParts[2]

	// Resolve the namespace if the given namespace is not the default
	var namespace *state.Namespace
	if namespaceName != remotetypes.DefaultNS {

		// Get the namespace, return 404 if not found
		namespace = sv.logic.NamespaceKeeper().Get(crypto2.MakeNamespaceHash(namespaceName))
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
	repoState := sv.logic.RepoKeeper().Get(repoName)
	if repoState.IsEmpty() {
		w.WriteHeader(http.StatusNotFound)
		sv.log.Debug("Unknown repository", "Name", repoName, "Code", http.StatusNotFound)
		return
	}

	pktEnc := pktline.NewEncoder(w)

	// Authenticate pusher
	txDetails, polEnforcer, err := sv.handleAuth(r, repoState, namespace)
	if err != nil {
		if err == ErrPushTokenRequired {
			w.Header().Set("WWW-Authenticate", "Basic")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = pktEnc.Encode(plumbing.SidebandInfoln("authentication has failed"))
		_ = pktEnc.Encode(plumbing.SidebandErr(err.Error()))
		_ = pktEnc.Flush()
		return
	}

	// Attempt to load the repository at the given path
	targetRepo, err := sv.GetRepo(repoName)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if err == git.ErrRepositoryNotExists {
			statusCode = http.StatusNotFound
		}
		w.WriteHeader(statusCode)
		sv.log.Debug("Failed to open target repository", "Name", repoName, "Code", statusCode)
		return
	}

	req := &RequestContext{
		W:           w,
		R:           r,
		Operation:   op,
		TxDetails:   txDetails,
		PolEnforcer: polEnforcer,
		Repo: &repo.Repo{
			Repository:     targetRepo.(*repo.Repo).Repository,
			BasicGitModule: targetRepo.(*repo.Repo).BasicGitModule,
			Path:           targetRepo.GetPath(),
			State:          repoState,
			NamespaceName:  namespaceName,
			Namespace:      namespace,
		},
		RepoDir:     targetRepo.GetPath(),
		ServiceName: getService(r),
		GitBinPath:  sv.gitBinPath,
		pktEnc:      pktEnc,
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
			writeMethodNotAllowed(w, r)
			break
		}

		err := srv.handle(req)
		if err != nil {
			sv.log.Error("failed to handle request", "Req", srvPattern, "Err", err)
		}

		return
	}

	writeMethodNotAllowed(w, r)
}

// GetPushKeyGetter implements RepositoryManager
func (sv *Server) GetPushKeyGetter() core.PushKeyGetter {
	return sv.pushKeyGetter
}

// PushHandlerFunc describes a function for creating a push handler
type PushHandlerFunc func(
	targetRepo plumbing.LocalRepo,
	txDetails []*remotetypes.TxDetail,
	enforcer policy.EnforcerFunc) pushtypes.Handler

// createPushHandler creates an instance of BasicHandler
func (sv *Server) createPushHandler(
	targetRepo plumbing.LocalRepo,
	txDetails []*remotetypes.TxDetail,
	enforcer policy.EnforcerFunc) pushtypes.Handler {
	return push.NewHandler(targetRepo, txDetails, enforcer, sv)
}

// Log returns the logger
func (sv *Server) Log() logger.Logger {
	return sv.log
}

// GetRepoState implements RepositoryManager
func (sv *Server) GetRepoState(repo plumbing.LocalRepo, options ...plumbing.KVOption) (plumbing.RepoRefsState, error) {
	return plumbing.GetRepoState(repo, options...), nil
}

// Wait can be used by the caller to wait till the server terminates
func (sv *Server) Wait() {
	sv.wg.Wait()
}

// Shutdown shuts down the server
func (sv *Server) Shutdown(ctx context.Context) {
	if sv.srv != nil {
		_ = sv.srv.Shutdown(ctx)
	}
}

// Stop implements Reactor
func (sv *Server) Stop() error {
	sv.log.Info("Gracefully shutting down server")
	sv.BaseReactor.Stop()
	sv.objFetcher.Stop()
	ctx, cc := context.WithTimeout(context.Background(), 15*time.Second)
	defer cc()
	sv.Shutdown(ctx)
	return nil
}
