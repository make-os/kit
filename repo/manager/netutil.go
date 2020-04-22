package manager

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/repo/policy"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp/sideband"
)

// RequestContext describes a request from the git remote server
type RequestContext struct {
	W           http.ResponseWriter
	R           *http.Request
	TxDetails   []*types.TxDetail
	PolEnforcer policy.EnforcerFunc
	PushHandler *PushHandler
	Repo        core.BareRepo
	RepoDir     string
	Operation   string
	SrvName     string
	GitBinPath  string
}

// sendFile fetches a file and sends it to the requester
// path: the path to the file in the repository
// contentType: The response content type to use
// p: service parameter of the request
func SendFile(path, contentType string, p *RequestContext) error {
	w, r := p.W, p.R
	reqFile := filepath.Join(p.RepoDir, path)

	f, err := os.Stat(reqFile)
	if os.IsNotExist(err) {
		EndNotFound(w)
		return fmt.Errorf("requested file not found")
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", f.Size()))
	w.Header().Set("Last-Modified", f.ModTime().Format(http.TimeFormat))
	http.ServeFile(w, r, reqFile)
	return nil
}

// endNotFound sends a 404 response
func EndNotFound(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte("Not Found"))
}

// execGitCmd executes git commands and returns the output
// repoDir: The directory of the target repository.
// args: Arguments for the git sub-command
func ExecGitCmd(gitBinDir, repoDir string, args ...string) ([]byte, error) {
	cmd := exec.Command(gitBinDir, args...)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, errors.Wrap(err, fmt.Sprintf("exec error: cmd=%s, output=%s",
			cmd.String(), string(out)))
	}
	return out, nil
}

// getService returns the requested service
func GetService(r *http.Request) string {
	service := r.URL.Query().Get("service")
	return strings.ReplaceAll(service, "git-", "")
}

func GetVersion(r *http.Request) string {
	return r.Header.Get("Git-Protocol")
}

// hdrNoCache sets no-cache header fields on the given http response
func HdrNoCache(w http.ResponseWriter) {
	w.Header().Set("Expires", "Fri, 01 Jan 1980 00:00:00 GMT")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Cache-Control", "no-cache, max-age=0, must-revalidate")
}

// hdrCacheForever sets a long-term cache on the given http response
func HdrCacheForever(w http.ResponseWriter) {
	now := time.Now().Unix()
	expires := now + 31536000
	w.Header().Set("Date", fmt.Sprintf("%d", now))
	w.Header().Set("Expires", fmt.Sprintf("%d", expires))
	w.Header().Set("Cache-Control", "public, max-age=31536000")
}

// packetFlush returns packfile end bytes
func PacketFlush() []byte {
	return []byte("0000")
}

// packetWrite returns valid packfile line for the given string
func PacketWrite(str string) []byte {
	s := strconv.FormatInt(int64(len(str)+4), 16)

	if len(s)%4 != 0 {
		s = strings.Repeat("0", 4-len(s)%4) + s
	}

	return []byte(s + str)
}

// writeMethodNotAllowed writes a response indicating that the request method is
// not allowed or expected.
func WriteMethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	if r.Proto == "HTTP/1.1" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte("Method Not Allowed"))
	} else {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Bad Request"))
	}
}

func UpdateServerInfo(gitBinDir, dir string) ([]byte, error) {
	args := []string{"update-server-info"}
	return ExecGitCmd(gitBinDir, dir, args...)
}

// getTextFile sends a text file
func GetTextFile(s *RequestContext) error {
	return SendFile(s.Operation, "text/plain; charset=utf-8", s)
}

// getLooseObject sends a loose object
func GetLooseObject(s *RequestContext) error {
	HdrCacheForever(s.W)
	return SendFile(s.Operation, "application/x-git-loose-object", s)
}

// getInfoPacks sends a pack info
func GetInfoPacks(s *RequestContext) error {
	HdrCacheForever(s.W)
	return SendFile(s.Operation, "text/plain; charset=utf-8", s)
}

// getPackFile sends a pack file
func GetPackFile(s *RequestContext) error {
	HdrCacheForever(s.W)
	return SendFile(s.Operation, "application/x-git-packed-objects", s)
}

// getIdxFile sends a pack index file
func GetIdxFile(s *RequestContext) error {
	HdrCacheForever(s.W)
	return SendFile(s.Operation, "application/x-git-packed-objects-toc", s)
}

func SidebandErr(msg string) []byte {
	return sideband.ErrorMessage.WithPayload([]byte(color.RedString(msg)))
}

func SidebandProgress(msg string) []byte {
	return sideband.ProgressMessage.WithPayload([]byte(color.GreenString(msg)))
}
