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

func samplePGPPubKeyGetter(pkID string) (string, error) {
	return `-----BEGIN PGP PUBLIC KEY BLOCK-----

mQENBF3D1yoBCACjdSC/KibksNrQ+gMb3Cw0I603SMwK8rvw5rE/L3oif7xc9Ghw
ZeQbSgpNCFVY9yUGX0WznQirAd5o4pleb6p/AmFtj3huLuPQ9IPA5xvPvf8k39Ky
aos5KHLK/tt6f+kG36IQpV2xryZs7ny4tNFKIHcl0HPC1oySFmAo0nVzDcpjFkYU
k2tryQo8JerFfOLp6NwTdXSsqFozKSSXHOwDDi8v811Wik48RKWaJ68LCS50CGFl
NYlYVkmZd29QIqJc4nUXrR/PmZqOklXC3feEJhSlmoFgMAWpfE6ffkGzqK7BQfAh
BarTbNGyV7mGZvY7w1wklFc6dlBGMWrsFZ6JABEBAAG0EEtlbm5lZHkgKHRlc3Qg
MymJAU4EEwEIADgWIQRpC08nO1qMBK/UHh3hTuV6RZk83wUCXcPXKgIbAwULCQgH
AgYVCgkICwIEFgIDAQIeAQIXgAAKCRDhTuV6RZk832u8B/9gZ4cT5rCkUUxH4s6F
oRtnEL01Q+iK9IyissVY1ZMM7p4+u5eXwljCqG5pw/KoHHIOZ98NuytRcgAM9dsi
vaWjKGxEOWD1VeKNEPDHu7KEQBfwYzfz+obf01e89E1NwvTQWmu/lK75hNajZPrh
EBIFoYI8ZiSsCnHESqI8hblezGYhxwXysD6zz3+tE5mcCswT5s95JQ6uYmeWrmlh
1B07BQ7d5GH5XAI+Bg4O90AXODCr4OKnuDcquqkpgwjBs1dDMFOtqn7V3qIsfsQF
cDwi7Nac0GbnW4arjTozjzYwEN34vDxJvvRQNM8467fZh4YHMWVnI80wf/HeI5ZR
ELi6uQENBF3D1yoBCADNLl6k97YZyKO30UE4/tyG0eQuEvCWa504MBIaVNa77F7e
snZaekKFIzrTAZJACu/2uCEJIfNyvsMp8EovVScw3Zm8SK4BVscot1KAntXZlf/3
4vWUnQqUb5ANav3I0l1a5ndtOmQCTuiZ5kW+6eUjra01pt1J9GxUMc/2DDC+HkYY
/emc/Uc44HPbIy8NlGCjSXCG0/QvyB+nHBxQtEAyX/aK5ylUQ/frPakS23yFviZs
cYb3ywAfMadWtchk7eG2ywLHpSVhuKhbHQdTtUSjLhllcjzrfMF1qUplrk+IDnp4
SRwSdbZ2E2CbeL0h/hifzGkYblWdYDe+lh5i+IDvABEBAAGJATYEGAEIACAWIQRp
C08nO1qMBK/UHh3hTuV6RZk83wUCXcPXKgIbDAAKCRDhTuV6RZk832c+CACIpykT
D3ZtAg+YsF2cb0xeQtvK4Hm0q2eaj0ri04b56K8+LeQxruuiQVEffE72lX+Sqpin
765wmOoK26eQ1IlRlwUEgoSusdko2cpgNaC5IgYXyG3pyRQ9wewudXM68jYXy5x9
FmSjybTOkWVO5qudYk2Cu6g4T7UyPrgGJ2iMunjDAVyK+BvhwZhx/HxLBTAx3uve
QpQXS1MnYXkyQz5mbqElHf0ELDX5zQ0JPNEL7CEf9dgBGUo02aGFCl0/oFR7O2el
yYXxF8MfL+q9HPVL7IrFOI3bLtrVuEt1qE6/vCzC804ODi4gfc9a2di3bKpMyoUU
svCU0gx1j1vi1SKS
=vHUA
-----END PGP PUBLIC KEY BLOCK-----`, nil
}

// RepositoryManager provides functionality for manipulating a repositories.
type RepositoryManager interface {

	// GetRepoState returns the state of the repository at the given path
	// options: Allows the caller to configure how and what state are gathered
	GetRepoState(target *Repo, options ...KVOption) (*State, error)

	// Revert reverts the repository from its current state to the previous state.
	Revert(target *Repo, prevState *State, options ...KVOption) (*Changes, error)

	// GetPGPPubKeyGetter returns the gpg getter function for finding GPG public
	// keys by their ID
	GetPGPPubKeyGetter() PGPPubKeyGetter

	// GetLogic returns the application logic provider
	GetLogic() types.Logic

	// GetNodeKey returns the node's private key
	GetNodeKey() *crypto.Key

	// GetPushPool returns the push pool
	GetPushPool() *PushPool

	// Start starts the server
	Start()

	// Wait can be used by the caller to wait till the server terminates
	Wait()

	// Stop shutsdown the server
	Stop(ctx context.Context)

	// CreateRepository creates a local git repository
	CreateRepository(name string) error
}

// Manager implements types.Manager. It provides a system for managing
// and service a git repositories through http and ssh protocols.
type Manager struct {
	log         logger.Logger   // log is the application logger
	wg          *sync.WaitGroup // wait group for waiting for the manager
	srv         *http.Server    // the http server
	rootDir     string          // the root directory where all repos are stored
	addr        string          // addr is the listening address for the http server
	gitBinPath  string          // gitBinPath is the path of the git executable
	repoDBCache *DBCache        // stores database handles of repositories
	pool        *PushPool       // this is the push transaction pool
	logic       types.Logic     // logic is the application logic provider
	nodeKey     *crypto.Key     // the node's private key for signing transactions
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
	return &Manager{
		log:         cfg.G().Log.Module("Manager"),
		addr:        addr,
		rootDir:     cfg.GetRepoRoot(),
		gitBinPath:  cfg.Node.GitBinPath,
		wg:          wg,
		repoDBCache: dbCache,
		pool:        NewPushPool(params.PushPoolCap),
		logic:       logic,
		nodeKey:     key,
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

// GetLogic returns the application logic provider
func (m *Manager) GetLogic() types.Logic {
	return m.logic
}

// GetNodeKey implements RepositoryManager
func (m *Manager) GetNodeKey() *crypto.Key {
	return m.nodeKey
}

// GetPushPool implements RepositoryManager
func (m *Manager) GetPushPool() *PushPool {
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
			Name:       repoName,
			Repository: repo,
			GitOps:     NewGitOps(m.gitBinPath, fullRepoDir),
			Path:       fullRepoDir,
			DBOps:      NewDBOps(m.repoDBCache, repoName),
			state:      repoState,
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
// TODO: Requires full implementation
func (m *Manager) GetPGPPubKeyGetter() PGPPubKeyGetter {
	return samplePGPPubKeyGetter
}

// GetRepoState implements RepositoryManager
func (m *Manager) GetRepoState(repo *Repo, options ...KVOption) (*State, error) {
	return m.getRepoState(repo, options...), nil
}

// GetRepoState returns the state of the repository
// repo: The target repository
// options: Allows the caller to configure how and what state are gathered
func (m *Manager) getRepoState(repo *Repo, options ...KVOption) *State {

	refMatch := ""
	if opt := getKVOpt("match", options); opt != nil {
		refMatch = opt.(string)
	}

	// Get references
	refs := make(map[string]Item)
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
func (m *Manager) Revert(repo *Repo, prevState *State, options ...KVOption) (*Changes, error) {
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
