package repo

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	types2 "gitlab.com/makeos/mosdef/dht/types"
	modtypes "gitlab.com/makeos/mosdef/modules/types"
	"gitlab.com/makeos/mosdef/node/types"
	"gitlab.com/makeos/mosdef/types/core"

	"gitlab.com/makeos/mosdef/pkgs/cache"
	"gitlab.com/makeos/mosdef/rest"

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
	// PushOKReactorChannel is the channel id for sending/receiving push okays
	PushOKReactorChannel = byte(0x33)
)

// Constants
const (
	ServiceReceivePack = "receive-pack"
	ServiceUploadPack  = "upload-pack"
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
	{"(.*?)/objects/[0-9a-f]{2}/[0-9a-f]{38}$", service{method: "GET", handle: getLooseObject}},
	{"(.*?)/objects/pack/pack-[0-9a-f]{40}\\.pack$", service{method: "GET", handle: getPackFile}},
	{"(.*?)/objects/pack/pack-[0-9a-f]{40}\\.idx$", service{method: "GET", handle: getIdxFile}},
}

// Manager implements types.Manager. It provides a system for managing
// and service a git repositories through http and ssh protocols.
type Manager struct {
	p2p.BaseReactor
	cfg                  *config.AppConfig
	log                  logger.Logger              // log is the application logger
	wg                   *sync.WaitGroup            // wait group for waiting for the manager
	srv                  *http.Server               // the http server
	rootDir              string                     // the root directory where all repos are stored
	addr                 string                     // addr is the listening address for the http server
	gitBinPath           string                     // gitBinPath is the path of the git executable
	pushPool             core.PushPool              // The transaction pool for push transactions
	mempool              core.Mempool               // The general transaction pool for block-bound transaction
	logic                core.Logic                 // logic is the application logic provider
	privValidatorKey     *crypto.Key                // the node's private validator key for signing transactions
	pgpPubKeyGetter      core.PGPPubKeyGetter       // finds and returns PGP public key
	dht                  types2.DHTNode             // The dht service
	pruner               core.Pruner                // The repo runner
	blockGetter          types.BlockGetter          // Provides access to blocks
	pushNoteSenders      *cache.Cache               // Store senders of push notes
	pushOKSenders        *cache.Cache               // Stores senders of PushOK messages
	pushNoteEndorsements *cache.Cache               // Store PushOKs
	modulesAgg           modtypes.ModulesAggregator // Modules aggregator
}

// NewManager creates an instance of Manager
func NewManager(
	cfg *config.AppConfig,
	addr string,
	logic core.Logic,
	dht types2.DHTNode,
	mempool core.Mempool,
	blockGetter types.BlockGetter) *Manager {

	wg := &sync.WaitGroup{}
	wg.Add(1)

	key, _ := cfg.G().PrivVal.GetKey()
	mgr := &Manager{
		cfg:                  cfg,
		log:                  cfg.G().Log.Module("repo-manager"),
		addr:                 addr,
		rootDir:              cfg.GetRepoRoot(),
		gitBinPath:           cfg.Node.GitBinPath,
		wg:                   wg,
		pushPool:             NewPushPool(params.PushPoolCap, logic, dht),
		logic:                logic,
		privValidatorKey:     key,
		dht:                  dht,
		mempool:              mempool,
		blockGetter:          blockGetter,
		pushNoteSenders:      cache.NewActiveCache(params.PushObjectsSendersCacheSize),
		pushOKSenders:        cache.NewActiveCache(params.PushObjectsSendersCacheSize),
		pushNoteEndorsements: cache.NewActiveCache(params.PushNotesEndorsementsCacheSize),
	}

	mgr.pgpPubKeyGetter = mgr.defaultGPGPubKeyGetter
	mgr.BaseReactor = *p2p.NewBaseReactor("Reactor", mgr)
	mgr.pruner = newPruner(mgr, mgr.rootDir)

	return mgr
}

// SetRootDir sets the directory where repositories are stored
func (m *Manager) SetRootDir(dir string) {
	m.rootDir = dir
}

// RegisterAPIHandlers registers server API handlers
func (m *Manager) RegisterAPIHandlers(agg modtypes.ModulesAggregator) {
	m.modulesAgg = agg
	m.registerAPIHandlers(m.srv.Handler.(*http.ServeMux))
}

func (m *Manager) defaultGPGPubKeyGetter(pkID string) (string, error) {
	gpgPK := m.logic.GPGPubKeyKeeper().GetGPGPubKey(pkID)
	if gpgPK.IsNil() {
		return "", fmt.Errorf("gpg public key not found for the given ID")
	}
	return gpgPK.PubKey, nil
}

// cachePushNoteSender caches a push note sender
func (m *Manager) cachePushNoteSender(senderID string, pushNoteID string) {
	key := util.Hash20Hex([]byte(senderID + pushNoteID))
	m.pushNoteSenders.AddWithExp(key, struct{}{}, time.Now().Add(10*time.Minute))
}

// cachePushOkSender caches a push OK sender
func (m *Manager) cachePushOkSender(senderID string, pushOkID string) {
	key := util.Hash20Hex([]byte(senderID + pushOkID))
	m.pushOKSenders.AddWithExp(key, struct{}{}, time.Now().Add(60*time.Minute))
}

// isPushNoteSender checks whether a push note was sent by the given sender ID
func (m *Manager) isPushNoteSender(senderID string, txID string) bool {
	key := util.Hash20Hex([]byte(senderID + txID))
	v := m.pushNoteSenders.Get(key)
	return v == struct{}{}
}

// isPushOKSender checks whether a "push OK" was sent by the given sender ID
func (m *Manager) isPushOKSender(senderID string, txID string) bool {
	key := util.Hash20Hex([]byte(senderID + txID))
	v := m.pushOKSenders.Get(key)
	return v == struct{}{}
}

// addPushNoteEndorsement indexes a PushOK for a given push note
func (m *Manager) addPushNoteEndorsement(pnID string, pok *core.PushOK) {
	pokList := m.pushNoteEndorsements.Get(pnID)
	if pokList == nil {
		pokList = map[string]*core.PushOK{}
	}
	pokList.(map[string]*core.PushOK)[pok.ID().String()] = pok
	m.pushNoteEndorsements.Add(pnID, pokList)
}

// Start starts the server that serves the repos.
// Implements p2p.Reactor
func (m *Manager) Start() error {
	s := http.NewServeMux()

	if !m.cfg.IsValidatorNode() {
		s.HandleFunc("/", m.gitRequestsHandler)
	}

	m.log.Info("Server has started", "Address", m.addr)
	m.srv = &http.Server{Addr: m.addr, Handler: s}
	go func() {
		m.srv.ListenAndServe()
		m.wg.Done()
	}()

	go m.subscribe()
	go m.pruner.Start()

	return nil
}

func (m *Manager) registerAPIHandlers(s *http.ServeMux) {
	rest := api.New(m.modulesAgg, m.log)
	rest.RegisterEndpoints(s)
}

// GetLogic returns the application logic provider
func (m *Manager) GetLogic() core.Logic {
	return m.logic
}

// GetPrivateValidatorKey implements RepositoryManager
func (m *Manager) GetPrivateValidatorKey() *crypto.Key {
	return m.privValidatorKey
}

// GetPruner returns the repo pruner
func (m *Manager) GetPruner() core.Pruner {
	return m.pruner
}

// GetPushPool returns the push pool
func (m *Manager) GetPushPool() core.PushPool {
	return m.pushPool
}

// GetMempool returns the transaction pool
func (m *Manager) GetMempool() core.Mempool {
	return m.mempool
}

// GetDHT returns the dht service
func (m *Manager) GetDHT() types2.DHTNode {
	return m.dht
}

// Cfg returns the application config
func (m *Manager) Cfg() *config.AppConfig {
	return m.cfg
}

// TODO: Authorization
func (m *Manager) handleAuth(r *http.Request) error {
	// username, password, _ := r.BasicAuth()
	// pp.Println(username, password)
	// if username != "username" || password != "password" {
	// 	return fmt.Errorf("unauthorized")
	// }
	return nil
}

func (m *Manager) getRepoPath(name string) string {
	return filepath.Join(m.rootDir, name)
}

// gitRequestsHandler handles incoming http request from a git client
func (m *Manager) gitRequestsHandler(w http.ResponseWriter, r *http.Request) {

	m.log.Debug("New request",
		"Method", r.Method,
		"URL", r.URL.String(),
		"ProtocolVersion", r.Proto)

	if err := m.handleAuth(r); err != nil {
		w.Header().Set("WWW-Authenticate", "Basic")
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, http.StatusText(http.StatusUnauthorized))
		return
	}

	// De-construct the URL to get the repo name and operation
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	namespace := pathParts[0]
	repoName := pathParts[1]
	op := pathParts[2]

	// Resolve the namespace if the given namespace is not the default
	if namespace != "r" {

		// Get the namespace, return 404 if not found
		ns := m.logic.NamespaceKeeper().GetNamespace(util.Hash20Hex([]byte(namespace)))
		if ns.IsNil() {
			w.WriteHeader(http.StatusNotFound)
			m.log.Debug("Unknown repository", "Name", repoName, "StatusCode", http.StatusNotFound,
				"StatusText", http.StatusText(http.StatusNotFound))
			return
		}

		// Get the target. If the target is not set or the target is not
		// prefixed with r/, return 404
		target := ns.Domains.Get(repoName)
		if target == "" || target[:2] != "r/" {
			w.WriteHeader(http.StatusNotFound)
			m.log.Debug("Unknown repository", "Name", repoName, "StatusCode", http.StatusNotFound,
				"StatusText", http.StatusText(http.StatusNotFound))
			return
		}

		repoName = target[2:]
	}

	// Check if the repository exist
	fullRepoDir := m.getRepoPath(repoName)
	repoState := m.logic.RepoKeeper().GetRepo(repoName)
	if repoState.IsNil() {
		w.WriteHeader(http.StatusNotFound)
		m.log.Debug("Unknown repository", "Name", repoName, "StatusCode", http.StatusNotFound,
			"StatusText", http.StatusText(http.StatusNotFound))
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
		m.log.Debug("Failed to open target repository",
			"Name", repoName,
			"StatusCode", statusCode,
			"StatusText", http.StatusText(statusCode))
		return
	}

	srvParams := &serviceParams{
		w:  w,
		r:  r,
		op: op,
		repo: &Repo{
			name:  repoName,
			git:   repo,
			ops:   NewGitOps(m.gitBinPath, fullRepoDir),
			path:  fullRepoDir,
			state: repoState,
		},
		repoDir:    fullRepoDir,
		srvName:    getService(r),
		gitBinPath: m.gitBinPath,
	}

	srvParams.pushHandler = newPushHandler(srvParams.repo, m)

	for _, s := range services {
		srvPattern := s[0].(string)
		srv := s[1].(service)

		re := regexp.MustCompile(srvPattern)
		if m := re.FindStringSubmatch(r.URL.Path); m == nil {
			continue
		}

		if srv.method != r.Method {
			writeMethodNotAllowed(w, r)
			return
		}

		err := srv.handle(srvParams)
		if err != nil {
			m.log.Error("failed to handle request", "Req", srvPattern, "Err", err)
		}

		return
	}

	writeMethodNotAllowed(w, r)
}

// GetPGPPubKeyGetter implements RepositoryManager
func (m *Manager) GetPGPPubKeyGetter() core.PGPPubKeyGetter {
	return m.pgpPubKeyGetter
}

// Log returns the logger
func (m *Manager) Log() logger.Logger {
	return m.log
}

// SetPGPPubKeyGetter implements SetPGPPubKeyGetter
func (m *Manager) SetPGPPubKeyGetter(pkGetter core.PGPPubKeyGetter) {
	m.pgpPubKeyGetter = pkGetter
}

// GetRepoState implements RepositoryManager
func (m *Manager) GetRepoState(repo core.BareRepo, options ...core.KVOption) (core.BareRepoState, error) {
	return getRepoState(repo, options...), nil
}

// Wait can be used by the caller to wait till the server terminates
func (m *Manager) Wait() {
	m.wg.Wait()
}

// FindObject implements dht.ObjectFinder
func (m *Manager) FindObject(key []byte) ([]byte, error) {

	repoName, objHash, err := ParseRepoObjectDHTKey(string(key))
	if err != nil {
		return nil, fmt.Errorf("invalid repo object key")
	}

	if len(objHash) != 40 {
		return nil, fmt.Errorf("invalid object hash")
	}

	repo, err := GetRepo(m.getRepoPath(repoName))
	if err != nil {
		return nil, err
	}

	bz, err := repo.GetCompressedObject(objHash)
	if err != nil {
		return nil, err
	}

	return bz, nil
}

// GetRepo returns a repo handle
func (m *Manager) GetRepo(name string) (core.BareRepo, error) {
	return getRepoWithGitOpt(m.gitBinPath, m.getRepoPath(name))
}

// Shutdown shuts down the server
func (m *Manager) Shutdown(ctx context.Context) {
	m.log.Info("Shutting down")
	if m.srv != nil {
		m.srv.Shutdown(ctx)
	}
	m.pruner.Stop()
}

// Stop implements Reactor
func (m *Manager) Stop() error {
	m.BaseReactor.Stop()
	m.Shutdown(context.Background())
	m.log.Info("Shutdown")
	return nil
}
