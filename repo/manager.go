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

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/params"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util/logger"
	"github.com/pkg/errors"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

// Git services
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
	cfg             *config.EngineConfig
	log             logger.Logger         // log is the application logger
	wg              *sync.WaitGroup       // wait group for waiting for the manager
	srv             *http.Server          // the http server
	rootDir         string                // the root directory where all repos are stored
	addr            string                // addr is the listening address for the http server
	gitBinPath      string                // gitBinPath is the path of the git executable
	repoDBCache     *DBCache              // stores database handles of repositories
	pool            types.PushPool        // this is the push transaction pool
	logic           types.Logic           // logic is the application logic provider
	nodeKey         *crypto.Key           // the node's private key for signing transactions
	pgpPubKeyGetter types.PGPPubKeyGetter // finds and returns PGP public key
}

// NewManager creates an instance of Manager
func NewManager(cfg *config.EngineConfig, addr string, logic types.Logic) *Manager {
	wg := &sync.WaitGroup{}
	wg.Add(1)

	dbCache, err := NewDBCache(1000, cfg.GetRepoRoot(), 5*time.Minute)
	if err != nil {
		panic(errors.Wrap(err, "failed create repo db cache"))
	}

	key, _ := cfg.G().PrivVal.GetKey()
	mgr := &Manager{
		cfg:         cfg,
		log:         cfg.G().Log.Module("repo-manager"),
		addr:        addr,
		rootDir:     cfg.GetRepoRoot(),
		gitBinPath:  cfg.Node.GitBinPath,
		wg:          wg,
		repoDBCache: dbCache,
		pool:        NewPushPool(params.PushPoolCap),
		logic:       logic,
		nodeKey:     key,
	}
	mgr.pgpPubKeyGetter = mgr.defaultGPGPubKeyGetter

	return mgr
}

// SetRootDir sets the directory where repositories are stored
func (m *Manager) SetRootDir(dir string) {
	m.rootDir = dir
}

func (m *Manager) defaultGPGPubKeyGetter(pkID string) (string, error) {
	gpgPK := m.logic.GPGPubKeyKeeper().GetGPGPubKey(pkID)
	if gpgPK.IsEmpty() {
		return "", fmt.Errorf("gpg public key not found for the given ID")
	}
	return gpgPK.PubKey, nil
}

// Start starts the server that serves the repos.
func (m *Manager) Start() error {
	s := http.NewServeMux()
	s.HandleFunc("/", m.handler)
	m.log.Info("Server has started", "Address", m.addr)

	m.srv = &http.Server{Addr: m.addr, Handler: s}

	go func() {
		m.srv.ListenAndServe()
		m.wg.Done()
	}()

	return nil
}

// GetLogic returns the application logic provider
func (m *Manager) GetLogic() types.Logic {
	return m.logic
}

// GetNodeKey implements RepositoryManager
func (m *Manager) GetNodeKey() *crypto.Key {
	return m.nodeKey
}

// GetPushPool implements RepositoryManager
func (m *Manager) GetPushPool() types.PushPool {
	return m.pool
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

// handler handles incoming http request from a git client
func (m *Manager) handler(w http.ResponseWriter, r *http.Request) {

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
	repoName := pathParts[0]
	fullRepoDir := filepath.Join(m.rootDir, repoName)
	op := pathParts[1]

	// Check if the repository exist
	repoState := m.logic.RepoKeeper().GetRepo(repoName)
	if repoState.IsNil() {
		w.WriteHeader(http.StatusNotFound)
		m.log.Debug("Unknown repository",
			"Name", repoName,
			"StatusCode", http.StatusNotFound,
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
		w: w,
		r: r,
		repo: &Repo{
			name:  repoName,
			git:   repo,
			ops:   NewGitOps(m.gitBinPath, fullRepoDir),
			path:  fullRepoDir,
			db:    NewDBOps(m.repoDBCache, repoName),
			state: repoState,
		},
		repoDir:    fullRepoDir,
		op:         op,
		srvName:    getService(r),
		gitBinPath: m.gitBinPath,
	}

	srvParams.hook = newPushHook(srvParams.repo, m)

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
func (m *Manager) GetPGPPubKeyGetter() types.PGPPubKeyGetter {
	return m.pgpPubKeyGetter
}

// SetPGPPubKeyGetter implements SetPGPPubKeyGetter
func (m *Manager) SetPGPPubKeyGetter(pkGetter types.PGPPubKeyGetter) {
	m.pgpPubKeyGetter = pkGetter
}

// GetRepoState implements RepositoryManager
func (m *Manager) GetRepoState(repo types.BareRepo, options ...types.KVOption) (types.BareRepoState, error) {
	return m.getRepoState(repo, options...), nil
}

// GetRepoState returns the state of the repository
// repo: The target repository
// options: Allows the caller to configure how and what state are gathered
func (m *Manager) getRepoState(repo types.BareRepo, options ...types.KVOption) types.BareRepoState {

	refMatch := ""
	if opt := getKVOpt("match", options); opt != nil {
		refMatch = opt.(string)
	}

	// Get references
	refs := make(map[string]types.Item)
	if refMatch == "" || strings.HasPrefix(refMatch, "refs") {
		refsI, _ := repo.References()
		refsI.ForEach(func(ref *plumbing.Reference) error {

			// Ignore HEAD reference
			if strings.ToLower(ref.Name().String()) == "head" {
				return nil
			}

			// If a ref match is set, ignore a reference whose name does not match
			if refMatch != "" && ref.Name().String() != refMatch {
				return nil
			}

			refs[ref.Name().String()] = &Obj{
				Type: "ref",
				Name: ref.Name().String(),
				Data: ref.Hash().String(),
			}

			return nil
		})
	}

	return &State{
		References: NewObjCol(refs),
	}
}

// Revert implements RepositoryManager
func (m *Manager) Revert(repo types.BareRepo, prevState types.BareRepoState,
	options ...types.KVOption) (*types.Changes, error) {
	return m.revert(repo, prevState, options...)
}

// Wait can be used by the caller to wait till the server terminates
func (m *Manager) Wait() {
	m.wg.Wait()
}

// Stop shutsdown the server
func (m *Manager) Stop(ctx context.Context) {
	if m.srv != nil {
		m.log.Info("Server is stopped")
		m.srv.Shutdown(ctx)
		m.repoDBCache.Clear()
	}
}
