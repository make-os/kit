package repo

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// sendFile fetches a file and sends it to the requester
// path: the path to the file in the repository
// contentType: The response content type to use
// p: service parameter of the request
func sendFile(path, contentType string, p *serviceParams) error {
	w, r := p.w, p.r
	reqFile := filepath.Join(p.repoDir, path)

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
	out, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute git command: "+cmd.String())
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
func getTextFile(s *serviceParams) error {
	return sendFile(s.op, "text/plain; charset=utf-8", s)
}

// getLooseObject sends a loose object
func getLooseObject(s *serviceParams) error {
	hdrCacheForever(s.w)
	return sendFile(s.op, "application/x-git-loose-object", s)
}

// getInfoPacks sends a pack info
func getInfoPacks(s *serviceParams) error {
	hdrCacheForever(s.w)
	return sendFile(s.op, "text/plain; charset=utf-8", s)
}

// getPackFile sends a pack file
func getPackFile(s *serviceParams) error {
	hdrCacheForever(s.w)
	return sendFile(s.op, "application/x-git-packed-objects", s)
}

// getIdxFile sends a pack index file
func getIdxFile(s *serviceParams) error {
	hdrCacheForever(s.w)
	return sendFile(s.op, "application/x-git-packed-objects-toc", s)
}
