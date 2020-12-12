package issuecmd

import (
	"fmt"
	"strings"

	"github.com/make-os/kit/remote/plumbing"
	"github.com/make-os/kit/remote/types"
	"github.com/pkg/errors"
)

// NormalizeIssueReferenceName normalizes a reference from args[0] to one that
// is a valid full issue reference name.
func NormalizeIssueReferenceName(curRepo types.LocalRepo, args []string) string {
	var ref string
	var err error

	if len(args) > 0 {
		ref = args[0]
	}

	// If reference is not set, use the HEAD as the reference.
	// But only if the reference is a valid issue reference.
	if ref == "" {
		ref, err = curRepo.Head()
		if err != nil {
			log.Fatal(errors.Wrap(err, "failed to get HEAD").Error())
		}
		if !plumbing.IsIssueReference(ref) {
			log.Fatal(fmt.Sprintf("not an issue path (%s)", ref))
		}
	}

	// If the reference begins with 'issues',
	// Add the full prefix 'refs/heads/' to make it `refs/heads/issues/<ref>`
	ref = strings.ToLower(ref)
	if strings.HasPrefix(ref, plumbing.IssueBranchPrefix) {
		ref = fmt.Sprintf("refs/heads/%s", ref)
	}

	// If the reference does not begin with 'refs/heads/issues',
	// convert to 'refs/heads/issues/<ref>'
	if !plumbing.IsIssueReferencePath(ref) {
		ref = plumbing.MakeIssueReference(ref)
	}

	// Finally, if reference is not of the form `refs/heads/issues/*`
	if !plumbing.IsIssueReference(ref) {
		log.Fatal(fmt.Sprintf("not an issue path (%s)", ref))
	}

	return ref
}
