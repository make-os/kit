package repo

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/makeos/mosdef/util"
	"github.com/pkg/errors"
	"gopkg.in/src-d/go-git.v4/plumbing/format/pktline"
)

// PGPPubKeyGetter represents a function for fetching PGP public key
type PGPPubKeyGetter func(pkId string) (string, error)

// service describes a git service and its handler
type service struct {
	method string
	handle func(*serviceParams) error
}

type serviceParams struct {
	w          http.ResponseWriter
	r          *http.Request
	hook       *Hook
	repo       *Repo
	repoDir    string
	op         string
	srvName    string
	gitBinPath string
}

// getInfoRefs Handle incoming request for a repository's references
func getInfoRefs(s *serviceParams) error {

	var err error
	var refs []byte
	var version string
	var args = []string{s.srvName, "--stateless-rpc", "--advertise-refs", "."}
	var isDumb = s.srvName == ""

	// If this is a request from a dumb client, skip to dumb response section
	if isDumb {
		goto dumbReq
	}

	// Execute git command which will return references
	refs, err = execGitCmd(s.gitBinPath, s.repoDir, args...)
	if err != nil {
		return err
	}

	// Configure response headers. Disable cache and set code to 200
	hdrNoCache(s.w)
	s.w.Header().Set("Content-Type", fmt.Sprintf("application/x-git-%s-advertisement", s.srvName))
	s.w.WriteHeader(http.StatusOK)

	version = getVersion(s.r)

	// If request is not a protocol v2 request, write the smart parameters
	// describing the service response
	if version != "2" {
		s.w.Write(packetWrite("# service=git-" + s.srvName + "\n"))
		s.w.Write(packetFlush())
	}

	// Write the references received from the git-upload-pack command
	s.w.Write(refs)
	return nil

	// Handle dumb request
dumbReq:

	hdrNoCache(s.w)

	// At this point, the dumb client needs help getting files (since it does
	// not support pack generation on-the-fly). Generate auxiliary files to help
	// the client discover the references and packs the server has.
	updateServerInfo(s.gitBinPath, s.repoDir)

	// Send the info/refs file back to the client
	return sendFile(s.op, "text/plain; charset=utf-8", s)
}

// serveService handles git-upload & fetch-pack requests
func serveService(s *serviceParams) error {
	w, r, op, dir := s.w, s.r, s.op, s.repoDir
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
	cmd := exec.Command(s.gitBinPath, args...)
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

	// Run BeforeInput service cycle method.
	// In here we capture the repository's current state.
	if err := s.hook.BeforeInput(); err != nil {
		return errors.Wrap(err, "BeforeInput hook err")
	}

	// Here, we send the request data to the git command
	packIns := &PackInspector{}
	io.Copy(in, io.TeeReader(reader, packIns))
	in.Close()
	// time.Sleep(15 * time.Second)

	// Even though we have written to git, git will not apply the updates to the
	// repository until we start to read from its stdin. As a trick, we read 1
	// byte and unread it to get git to update the repository.
	stdoutBf := util.TouchReader(stdout)
	defer stdout.Close()

	// Create a packfile scanner and encoding for reading
	// the stdout buffer and writing to the http response
	scn := pktline.NewScanner(stdoutBf)
	pktEnc := pktline.NewEncoder(w)
	defer pktEnc.Flush()

	// BeforeOutput hook: Do work that needs to be done after the repository is
	// updated - Commit validation, broadcasting etc
	if err := s.hook.BeforeOutput(packIns.GetBranches()); err != nil {
		pktEnc.Encode(sidebandErr(err.Error()))
		return errors.Wrap(err, "BeforeOutput hook err")
	}

	// Read from the git command stdout and pipe the output to http response
	for scn.Scan() {
		if err := pktEnc.Encode(scn.Bytes()); err != nil {
			return errors.Wrap(err, "failed to write git stdout data to http response")
		}
	}

	// Ensure no error occurred while reading from git
	if scn.Err() != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to read from from git-%s", op))
	}

	// AfterOutput hook:
	if err := s.hook.AfterOutput(); err != nil {
		return errors.Wrap(err, "AfterOutput hook err")
	}

	return nil
}
