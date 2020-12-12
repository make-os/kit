package mergecmd

import (
	"fmt"
	"strings"

	"github.com/make-os/kit/remote/plumbing"
	"github.com/make-os/kit/remote/types"
	"github.com/pkg/errors"
)

// NormalMergeReferenceName normalizes a reference from args[0] to one that
// is a valid full merge request reference name.
func NormalMergeReferenceName(curRepo types.LocalRepo, args []string) string {
	var ref string
	var err error

	if len(args) > 0 {
		ref = strings.ToLower(args[0])
	}

	// If reference is not set, use the HEAD as the reference.
	// But only if the reference is a valid merge request reference.
	if ref == "" {
		ref, err = curRepo.Head()
		if err != nil {
			log.Fatal(errors.Wrap(err, "failed to get HEAD").Error())
		}
		if !plumbing.IsMergeRequestReference(ref) {
			log.Fatal(fmt.Sprintf("not a valid merge request path (%s)", ref))
		}
	}

	// If the reference begins with 'merges',
	// Add the full prefix 'refs/heads/' to make it `refs/heads/merges/<ref>`
	if strings.HasPrefix(ref, plumbing.MergeRequestBranchPrefix) {
		ref = fmt.Sprintf("refs/heads/%s", ref)
	}

	// If the reference does not begin with 'refs/heads/merges',
	// convert to 'refs/heads/merges/<ref>'
	if !plumbing.IsMergeRequestReferencePath(ref) {
		ref = plumbing.MakeMergeRequestReference(ref)
	}

	// Finally, if reference is not of the form `refs/heads/merges/*`
	if !plumbing.IsMergeRequestReference(ref) {
		log.Fatal(fmt.Sprintf("not a valid merge request path (%s)", ref))
	}

	return ref
}
