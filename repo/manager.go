package repo

import (
	"context"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/util/logger"
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

// StateOperator provides functionality for manipulating a repository's state
type StateOperator interface {
	GetRepoState(path string) (*State, error)
	Revert(path string, prevState *State) error
}

// Manager implements types.Manager. It provides a system for managing
// and service a git repositories through http and ssh protocols.
type Manager struct {
	log        logger.Logger
	wg         *sync.WaitGroup
	srv        *http.Server
	rootDir    string
	addr       string
	gitBinPath string
}

// NewManager creates an instance of Manager
func NewManager(cfg *config.EngineConfig, addr string) *Manager {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	return &Manager{
		log:        cfg.G().Log.Module("Manager"),
		addr:       addr,
		rootDir:    cfg.GetRepoRoot(),
		gitBinPath: cfg.Node.GitBinPath,
		wg:         wg,
	}
}

// SetRootDir sets the directory where repositories are stored
func (m *Manager) SetRootDir(dir string) {
	m.rootDir = dir
}

// Start starts the server that serves the repos.
func (m *Manager) Start() {
	s := http.NewServeMux()
	s.HandleFunc("/", m.handler)

	m.log.Info("Server has started", "Address", m.addr)
	m.srv = &http.Server{Addr: m.addr, Handler: s}
	m.srv.ListenAndServe()
	m.wg.Done()
}

// handler handles incoming http request from a git client
func (m *Manager) handler(w http.ResponseWriter, r *http.Request) {

	m.log.Debug("New request", "Method", r.Method,
		"URL", r.URL.String(), "ProtocolVersion", r.Proto)

	// De-construct the URL to get the repo name and operation
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	repoName := pathParts[0]
	fullRepoDir := filepath.Join(m.rootDir, repoName)
	op := pathParts[1]

	// Attempt to load the repository at the given directory
	repo, err := git.PlainOpen(fullRepoDir)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if err == git.ErrRepositoryNotExists {
			statusCode = http.StatusNotFound
		}
		w.WriteHeader(statusCode)
		m.log.Debug("Failed to open target repository",
			"StatusCode", statusCode,
			"StatusText", http.StatusText(statusCode))
		return
	}

	srvParams := &serviceParams{
		w: w,
		r: r,
		repo: &Repo{
			Repository: repo,
			GitOps:     NewGitOps(m.gitBinPath, fullRepoDir),
			Path:       fullRepoDir,
		},
		repoDir:    fullRepoDir,
		op:         op,
		srvName:    getService(r),
		gitBinPath: m.gitBinPath,
	}

	srvParams.hook = NewHook(strings.ReplaceAll(op, "git-", ""), srvParams.repo, m)

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

// GetRepoState returns the state of the repository at the given path
func (m *Manager) GetRepoState(path string) (*State, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, err
	}
	return m.getRepoState(&Repo{
		Repository: repo,
		Path:       path,
	}), nil
}

// GetRepoState returns the state of the repository
func (m *Manager) getRepoState(repo *Repo) *State {

	// Get references
	refs := make(map[string]*Obj)
	refsI, _ := repo.References()
	refsI.ForEach(func(ref *plumbing.Reference) error {
		if strings.ToLower(ref.Name().String()) != "head" {
			refs[ref.Name().String()] = &Obj{
				Type: "ref",
				Name: ref.Name().String(),
				Data: ref.Hash().String(),
			}
		}
		return nil
	})

	return &State{
		Refs: NewObjCol(refs),
	}
}

// Revert reverts the repository from its current state to the previous state.
// path: The path to a valid repository
func (m *Manager) Revert(path string, prevState *State) error {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return err
	}
	return m.revert(&Repo{
		Repository: repo,
		GitOps:     NewGitOps(m.gitBinPath, path),
		Path:       path,
	}, prevState)
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
	}
}
