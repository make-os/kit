package repobase

import (
	"context"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/makeos/mosdef/util/logger"
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

// RepoBase implements RepoServer. It provides a system for managing
// and service a git repositories through http and ssh protocols.
type RepoBase struct {
	log        logger.Logger
	wg         *sync.WaitGroup
	srv        *http.Server
	rootDir    string
	addr       string
	gitBinPath string
}

// NewRepoBase creates an instance of RepoBase
func NewRepoBase(log logger.Logger, addr, rootDir, gitBinDir string) *RepoBase {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	return &RepoBase{
		log:        log.Module("RepoBase"),
		addr:       addr,
		rootDir:    rootDir,
		gitBinPath: gitBinDir,
		wg:         wg,
	}
}

// Start starts the server that serves the repos.
func (rb *RepoBase) Start() {
	s := http.NewServeMux()
	s.HandleFunc("/", rb.handler)

	rb.log.Info("Server has started", "Address", rb.addr)
	rb.srv = &http.Server{Addr: rb.addr, Handler: s}
	rb.srv.ListenAndServe()
	rb.wg.Done()
}

// handler handles incoming http request from a git client
func (rb *RepoBase) handler(w http.ResponseWriter, r *http.Request) {

	rb.log.Debug("New request", "Method", r.Method,
		"URLPath", r.URL.Path, "ProtocolVersion", r.Proto)

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

// Wait can be used by the caller to wait till the server terminates
func (rb *RepoBase) Wait() {
	rb.wg.Wait()
}

// Stop shutsdown the server
func (rb *RepoBase) Stop(ctx context.Context) {
	if rb.srv != nil {
		rb.log.Info("Server is stopped")
		rb.srv.Shutdown(ctx)
	}
}
