package repo

import (
	"bytes"
	"regexp"

	"gopkg.in/src-d/go-git.v4/plumbing/format/pktline"
)

// packInspectorBufferCap is the max buffer size of the inspector buffer.
// We have set this to 1MB.
var packInspectorBufferCap = 1024 * 1024

// PackInspector implements io.Writer. It is meant to be used with a TeeReader
// to cache incoming git packet data for inspection and data extraction.
type PackInspector struct {
	buf []byte
}

// Write implements the io.Writer interface.
func (pi *PackInspector) Write(p []byte) (int, error) {
	n := len(p)
	if len(pi.buf) < packInspectorBufferCap {
		pi.buf = append(pi.buf, p...)
	}
	return n, nil
}

// GetBranches returns the branch names found in the pack buffer
func (pi *PackInspector) GetBranches() (branches []string) {
	scn := pktline.NewScanner(bytes.NewReader(pi.buf))
	for {
		if !scn.Scan() {
			break
		}
		rg := regexp.MustCompile(`(?i).*(refs/(heads|tags|notes)/([a-z0-9-_]+/?)*).*`)
		groups := rg.FindStringSubmatch(string(scn.Bytes()))
		if len(groups) > 0 {
			branches = append(branches, groups[1])
		}
	}
	return
}
