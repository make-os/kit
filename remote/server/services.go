package server

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/remote/policy"
	"gitlab.com/makeos/mosdef/remote/push"
	"gitlab.com/makeos/mosdef/remote/types"
	fmt2 "gitlab.com/makeos/mosdef/util/colorfmt"
	"gopkg.in/src-d/go-git.v4/plumbing/format/pktline"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp/sideband"
)

// RequestContext describes a request from the git remote server
type RequestContext struct {
	W           http.ResponseWriter
	R           *http.Request
	TxDetails   []*types.TxDetail
	PolEnforcer policy.EnforcerFunc
	PushHandler push.Handler
	Repo        types.LocalRepo
	RepoDir     string
	Operation   string
	ServiceName string
	GitBinPath  string
}

// sendFile fetches a file and sends it to the requester
// path: the path to the file in the repository
// contentType: The response content type to use
// p: service parameter of the request
func sendFile(path, contentType string, p *RequestContext) error {
	w, r := p.W, p.R
	reqFile := filepath.Join(p.RepoDir, path)

	f, err := os.Stat(reqFile)
	if os.IsNotExist(err) {
		endNotFound(w)
		return fmt.Errorf("requested file not found")
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", f.Size()))
	w.Header().Set("Last-Modified", f.ModTime().Format(http.TimeFormat))
	http.ServeFile(w, r, reqFile)
	return nil
}

// endNotFound sends a 404 response
func endNotFound(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte("Not Found"))
}

// execGitCmd executes git commands and returns the output
// repoDir: The directory of the target repository.
// args: Arguments for the git sub-command
func execGitCmd(gitBinDir, repoDir string, args ...string) ([]byte, error) {
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
func getService(r *http.Request) string {
	service := r.URL.Query().Get("service")
	return strings.ReplaceAll(service, "git-", "")
}

func getVersion(r *http.Request) string {
	return r.Header.Get("Git-Protocol")
}

// hdrNoCache sets no-cache header fields on the given http response
func hdrNoCache(w http.ResponseWriter) {
	w.Header().Set("Expires", "Fri, 01 Jan 1980 00:00:00 GMT")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Cache-Control", "no-cache, max-age=0, must-revalidate")
}

// hdrCacheForever sets a long-term cache on the given http response
func hdrCacheForever(w http.ResponseWriter) {
	now := time.Now().Unix()
	expires := now + 31536000
	w.Header().Set("Date", fmt.Sprintf("%d", now))
	w.Header().Set("Expires", fmt.Sprintf("%d", expires))
	w.Header().Set("Cache-Control", "public, max-age=31536000")
}

// packetFlush returns packfile end bytes
func packetFlush() []byte {
	return []byte("0000")
}

// packetWrite returns valid packfile line for the given string
func packetWrite(str string) []byte {
	s := strconv.FormatInt(int64(len(str)+4), 16)
	if len(s)%4 != 0 {
		s = strings.Repeat("0", 4-len(s)%4) + s
	}
	return []byte(s + str)
}

// writeMethodNotAllowed writes a response indicating that the request method is
// not allowed or expected.
func writeMethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	if r.Proto == "HTTP/1.1" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte("Method Not Allowed"))
	} else {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Bad Request"))
	}
}

func updateServerInfo(gitBinDir, dir string) ([]byte, error) {
	args := []string{"update-server-info"}
	return execGitCmd(gitBinDir, dir, args...)
}

// getTextFile sends a text file
func getTextFile(s *RequestContext) error {
	return sendFile(s.Operation, "text/plain; charset=utf-8", s)
}

// getInfoPacks sends a pack info
func getInfoPacks(s *RequestContext) error {
	hdrCacheForever(s.W)
	return sendFile(s.Operation, "text/plain; charset=utf-8", s)
}

// getPackFile sends a pack file
func getPackFile(s *RequestContext) error {
	hdrCacheForever(s.W)
	return sendFile(s.Operation, "application/x-git-packed-objects", s)
}

// getIdxFile sends a pack index file
func getIdxFile(s *RequestContext) error {
	hdrCacheForever(s.W)
	return sendFile(s.Operation, "application/x-git-packed-objects-toc", s)
}

func sidebandErr(msg string) []byte {
	return sideband.ErrorMessage.WithPayload([]byte(fmt2.RedString(msg)))
}

func sidebandProgress(msg string) []byte {
	return sideband.ProgressMessage.WithPayload([]byte(fmt2.GreenString(msg)))
}

// service describes a git service and its handler
type service struct {
	method string
	handle func(*RequestContext) error
}

// getInfoRefs Handle incoming request for a repository's references
func getInfoRefs(s *RequestContext) error {

	var err error
	var refs []byte
	var version string
	var args = []string{s.ServiceName, "--stateless-rpc", "--advertise-refs", "."}
	var isDumb = s.ServiceName == ""

	// If this is a request from a dumb client, skip to dumb response section
	if isDumb {
		goto dumbReq
	}

	// Execute git command which will return references
	refs, err = execGitCmd(s.GitBinPath, s.RepoDir, args...)
	if err != nil {
		return err
	}

	// ConfigureVM response headers. Disable cache and set code to 200
	hdrNoCache(s.W)
	s.W.Header().Set("Content-Type", fmt.Sprintf("application/x-git-%s-advertisement", s.ServiceName))
	s.W.WriteHeader(http.StatusOK)

	version = getVersion(s.R)

	// If request is not a protocol v2 request, write the smart parameters
	// describing the service response
	if version != "2" {
		s.W.Write(packetWrite("# service=git-" + s.ServiceName + "\n"))
		s.W.Write(packetFlush())
	}

	// Write the references received from the git-upload-pack command
	s.W.Write(refs)
	return nil

	// Handle dumb request
dumbReq:

	hdrNoCache(s.W)

	// At this point, the dumb client needs help getting files (since it does
	// not support pack generation on-the-fly). Generate auxiliary files to help
	// the client discover the references and packs the server has.
	updateServerInfo(s.GitBinPath, s.RepoDir)

	// Send the info/refs file back to the client
	return sendFile(s.Operation, "text/plain; charset=utf-8", s)
}

// serveService handles git-upload & fetch-pack requests
func serveService(s *RequestContext) error {
	w, r, op, dir := s.W, s.R, s.Operation, s.RepoDir
	op = strings.ReplaceAll(op, "git-", "")

	// Set response headers
	w.Header().Set("Content-Type", fmt.Sprintf("application/x-git-%s-result", op))
	w.Header().Set("Connection", "Keep-Alive")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	hdrNoCache(w)

	// Construct the git command
	env := os.Environ()
	args := []string{op, "--stateless-rpc", dir}
	cmd := exec.Command(s.GitBinPath, args...)
	version := r.Header.Get("Git-Protocol")
	cmd.Dir = dir
	cmd.Env = env

	// If client requested v2 protocol, set protocol flag in env
	if len(version) != 0 {
		cmd.Env = append(env, fmt.Sprintf("GIT_PROTOCOL=%s", version))
	}

	// Get the command's stdin pipe
	in, err := cmd.StdinPipe()
	if err != nil {
		return errors.Wrap(err, "failed to get stdin pipe")
	}

	// Get the command's stdout pipe
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "failed to get stdout pipe")
	}
	defer stdout.Close()

	// Start running the command (does not wait)
	err = cmd.Start()
	if err != nil {
		return errors.Wrap(err, "failed to start command")
	}

	// If the request is compressed, we need to uncompress
	// before we feed it to the git.
	var reader io.ReadCloser
	switch r.Header.Get("Content-Encoding") {
	case "gzip":
		reader, err = gzip.NewReader(r.Body)
		defer reader.Close()
	default:
		reader = r.Body
		defer reader.Close()
	}

	// Handle fetch request
	if op == "upload-pack" {
		io.Copy(in, reader)
		in.Close()
		io.Copy(w, stdout)
		return nil
	}

	scn := pktline.NewScanner(stdout)
	pktEnc := pktline.NewEncoder(w)
	defer pktEnc.Flush()

	// Read, analyse and pass the packfile to git
	s.PushHandler.SetGitReceivePackCmd(cmd)
	if err := s.PushHandler.HandleStream(reader, in); err != nil {
		pktEnc.Encode(sidebandErr(errors.Wrap(err, "server error").Error()))
		return errors.Wrap(err, "HandleStream error")
	}

	// Handle validate, revert and broadcast the changes
	if err := s.PushHandler.HandleUpdate(); err != nil {
		pktEnc.Encode(sidebandErr(err.Error()))
		return errors.Wrap(err, "HandleUpdate err")
	}

	noteID := s.PushHandler.(*push.BasicHandler).NoteID
	w.Header().Set("TxID", noteID)
	pktEnc.Encode(sidebandProgress(fmt.Sprintf("Transaction ID: %s", noteID)))

	// Write output from git to the http response
	for scn.Scan() {
		pktEnc.Encode(scn.Bytes())
	}

	return nil
}
