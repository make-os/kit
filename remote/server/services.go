package server

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/src-d/go-git.v4/plumbing/format/pktline"
)

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
	refs, err = ExecGitCmd(s.GitBinPath, s.RepoDir, args...)
	if err != nil {
		return err
	}

	// Configure response headers. Disable cache and set code to 200
	HdrNoCache(s.W)
	s.W.Header().Set("Content-Type", fmt.Sprintf("application/x-git-%s-advertisement", s.ServiceName))
	s.W.WriteHeader(http.StatusOK)

	version = GetVersion(s.R)

	// If request is not a protocol v2 request, write the smart parameters
	// describing the service response
	if version != "2" {
		s.W.Write(PacketWrite("# service=git-" + s.ServiceName + "\n"))
		s.W.Write(PacketFlush())
	}

	// Write the references received from the git-upload-pack command
	s.W.Write(refs)
	return nil

	// Handle dumb request
dumbReq:

	HdrNoCache(s.W)

	// At this point, the dumb client needs help getting files (since it does
	// not support pack generation on-the-fly). Generate auxiliary files to help
	// the client discover the references and packs the server has.
	UpdateServerInfo(s.GitBinPath, s.RepoDir)

	// Send the info/refs file back to the client
	return SendFile(s.Operation, "text/plain; charset=utf-8", s)
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
	HdrNoCache(w)

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
	if err := s.PushHandler.HandleStream(reader, in); err != nil {
		pktEnc.Encode(SidebandErr(errors.Wrap(err, "server error").Error()))
		return errors.Wrap(err, "HandleStream error")
	}

	// Handle validate, revert and broadcast the changes
	if err := s.PushHandler.HandleUpdate(); err != nil {
		pktEnc.Encode(SidebandErr(err.Error()))
		return errors.Wrap(err, "HandleUpdate err")
	}

	w.Header().Set("TxID", s.PushHandler.NoteID)
	pktEnc.Encode(SidebandProgress(fmt.Sprintf("Transaction ID: %s", s.PushHandler.NoteID)))

	// Write output from git to the http response
	for scn.Scan() {
		pktEnc.Encode(scn.Bytes())
	}

	return nil
}
