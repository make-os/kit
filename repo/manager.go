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
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

var services = [][]interface{}{
	{"(.*?)/git-upload-pack$", service{method: "POST", handle: doSmartService}},
	{"(.*?)/git-receive-pack$", service{method: "POST", handle: doSmartService}},
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

// Start starts the server that serves the repos.
func (rb *Manager) Start() {
	s := http.NewServeMux()
	s.HandleFunc("/", rb.handler)

	rb.log.Info("Server has started", "Address", rb.addr)
	rb.srv = &http.Server{Addr: rb.addr, Handler: s}
	rb.srv.ListenAndServe()
	rb.wg.Done()
}

// handler handles incoming http request from a git client
func (rb *Manager) handler(w http.ResponseWriter, r *http.Request) {

	rb.log.Debug("New request", "Method", r.Method,
		"URL", r.URL.String(), "ProtocolVersion", r.Proto)

	// De-construct the URL to get the repo name and operation
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	repo := pathParts[0]
	fullRepoDir := filepath.Join(rb.rootDir, repo)
	op := strings.Join(pathParts[1:], "/")
	srvParams := &serviceParams{
		w:          w,
		r:          r,
		repo:       repo,
		repoDir:    fullRepoDir,
		op:         op,
		srvName:    getService(r),
		gitBinPath: rb.gitBinPath,
	}

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
			rb.log.Error("failed to handle request", "Req", srvPattern, "Err", err)
		}

		return
	}

	writeMethodNotAllowed(w, r)
}

// GetRepoState returns the state of the repository at the given path
func (rb *Manager) GetRepoState(path string) (*State, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, err
	}
	return rb.getRepoState(&Repo{
		Repository: repo,
		Path:       path,
	}), nil
}

// GetRepoState returns the state of the repository at the given path
func (rb *Manager) getRepoState(repo *Repo) *State {

	// Get tags
	tags := make(map[string]*Obj)
	tagsI, _ := repo.TagObjects()
	tagsI.ForEach(func(tag *object.Tag) error {
		tags[tag.Name] = &Obj{
			Type: "tag",
			Name: tag.Name,
			Data: tag.ID().String(),
		}
		return nil
	})

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
		Tags: NewObjCol(tags),
		Refs: NewObjCol(refs),
	}
}

// Revert reverts the repository from its current state to the previous state.
// path: The path to a valid repository
func (rb *Manager) Revert(path string, prevState *State) error {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return err
	}
	return rb.revert(&Repo{
		Repository: repo,
		GitOps:     NewGitOps(rb.gitBinPath, path),
		Path:       path,
	}, prevState)
}

// Wait can be used by the caller to wait till the server terminates
func (rb *Manager) Wait() {
	rb.wg.Wait()
}

// Stop shutsdown the server
func (rb *Manager) Stop(ctx context.Context) {
	if rb.srv != nil {
		rb.log.Info("Server is stopped")
		rb.srv.Shutdown(ctx)
	}
}
