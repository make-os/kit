package validation

import (
	"fmt"
	"strings"

	"github.com/asaskevich/govalidator"
	plumbing2 "github.com/make-os/kit/remote/plumbing"
	"github.com/make-os/kit/remote/types"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/identifier"

	"github.com/pkg/errors"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

var (
	fe                             = util.FieldErrorWithIndex
	ErrPushedAndSignedHeadMismatch = fmt.Errorf("pushed object hash differs from signed reference hash")
)

type ChangeValidatorFunc func(
	keepers core.Keepers,
	repo types.LocalRepo,
	oldHash string,
	change *types.ItemChange,
	txDetail *types.TxDetail,
	getPushKey core.PushKeyGetter) error

// ValidateChange validates a change to a repository
// repo: The target repository
// oldHash: The hash of the old reference
// change: The change to the reference
// txDetail: The pusher transaction detail
// getPushKey: Getter function for reading push key public key
func ValidateChange(
	keepers core.Keepers,
	localRepo types.LocalRepo,
	oldHash string,
	change *types.ItemChange,
	detail *types.TxDetail,
	getPushKey core.PushKeyGetter) error {

	refname := change.Item.GetName()
	isIssueRef := plumbing2.IsIssueReferencePath(refname)
	isMergeRequestRef := plumbing2.IsMergeRequestReferencePath(refname)

	// Handle issue or merge request branch validation.
	if plumbing2.IsBranch(refname) && (isIssueRef || isMergeRequestRef) {
		commit, err := localRepo.WrappedCommitObject(plumbing.NewHash(change.Item.GetData()))
		if err != nil {
			return errors.Wrap(err, "unable to get commit object")
		}
		return ValidatePostCommit(localRepo, commit, &ValidatePostCommitArg{
			Keepers:         keepers,
			OldHash:         oldHash,
			Change:          change,
			TxDetail:        detail,
			PushKeyGetter:   getPushKey,
			CheckCommit:     CheckCommit,
			CheckPostCommit: CheckPostCommit,
		})
	}

	// Handle regular branch validation
	if plumbing2.IsBranch(refname) {
		commit, err := localRepo.CommitObject(plumbing.NewHash(change.Item.GetData()))
		if err != nil {
			return errors.Wrap(err, "unable to get commit object")
		}
		return CheckCommit(commit, detail, getPushKey)
	}

	// Handle tag validation
	if plumbing2.IsTag(change.Item.GetName()) {
		tagRef, err := localRepo.Tag(strings.ReplaceAll(change.Item.GetName(), "refs/tags/", ""))
		if err != nil {
			return errors.Wrap(err, "unable to get tag object")
		}

		// Get the tag object (for annotated tags)
		tagObj, err := localRepo.TagObject(tagRef.Hash())
		if err != nil && err != plumbing.ErrObjectNotFound {
			return err
		}

		// Here, the tag is not an annotated tag, so we need to
		// ensure the referenced commit is signed correctly
		if tagObj == nil {
			commit, err := localRepo.CommitObject(tagRef.Hash())
			if err != nil {
				return errors.Wrap(err, "unable to get commit")
			}
			return CheckCommit(commit, detail, getPushKey)
		}

		// At this point, the tag is an annotated tag.
		// We have to ensure the annotated tag object is signed.
		return CheckAnnotatedTag(tagObj, detail, getPushKey)
	}

	// Handle note validation
	if plumbing2.IsNote(change.Item.GetName()) {
		return CheckNote(localRepo, detail)
	}

	return fmt.Errorf("unrecognised change item")
}

// CheckNote validates a note.
// repo: The repo where the tag exists in.
// txDetail: The pusher transaction detail
func CheckNote(
	repo types.LocalRepo,
	txDetail *types.TxDetail) error {

	// Get the note current hash
	noteHash, err := repo.RefGet(txDetail.Reference)
	if err != nil {
		return errors.Wrap(err, "failed to get note")
	}

	// Ensure the reference hash in the tx detail matches the current object hash
	if noteHash != txDetail.Head {
		return ErrPushedAndSignedHeadMismatch
	}

	return nil
}

// CheckAnnotatedTag validates an annotated tag.
// tag: The target annotated tag
// txDetail: The pusher transaction detail
// getPushKey: Getter function for reading push key public key
func CheckAnnotatedTag(tag *object.Tag, txDetail *types.TxDetail, _ core.PushKeyGetter) error {

	// Ensure the reference hash in the tx detail matches the current object hash
	if tag.Hash.String() != txDetail.Head {
		return ErrPushedAndSignedHeadMismatch
	}

	return nil
}

// CommitChecker describes a function for checking a standard commit
type CommitChecker func(commit *object.Commit, txDetail *types.TxDetail, getPushKey core.PushKeyGetter) error

// CheckCommit validates a commit
// repo: The target repo
// commit: The target commit object
// txDetail: The push transaction detail
// getPushKey: Getter function for fetching push public key
func CheckCommit(commit *object.Commit, txDetail *types.TxDetail, _ core.PushKeyGetter) error {

	// Ensure the reference hash in the tx detail matches the current object hash
	if commit.Hash.String() != txDetail.Head {
		return ErrPushedAndSignedHeadMismatch
	}

	return nil
}

// IsBlockedByScope checks whether the given tx parameter satisfy a given scope
func IsBlockedByScope(scopes []string, params *types.TxDetail, namespaceFromParams *state.Namespace) bool {
	blocked := true
	for _, scope := range scopes {
		if identifier.IsNamespace(scope) {
			ns, domain, _ := util.SplitNamespaceDomain(scope)

			// If scope is r/repo-name, make sure tx info namespace is unset and repo name is 'repo-name'.
			// If scope is r/ only, make sure only tx info namespace is set
			if ns == types.DefaultNS && params.RepoNamespace == "" && (domain == "" || domain == params.RepoName) {
				blocked = false
				break
			}

			// If scope is some_ns/repo-name, make sure tx info namespace and repo name matches the scope
			// namespace and repo name.
			if ns != types.DefaultNS && ns == params.RepoNamespace && domain == params.RepoName {
				blocked = false
				break
			}

			// If scope is just some_ns/, make sure tx info namespace matches
			if ns != types.DefaultNS && domain == "" && ns == params.RepoNamespace {
				blocked = false
				break
			}
		}

		// At this point, the scope is just a target repo name.
		// e.g unblock if tx info namespace is default and the repo name matches the scope
		if params.RepoNamespace == "" && params.RepoName == scope {
			blocked = false
			break
		}

		// But if the scope's repo name is set, ensure the domain target matches the sc
		if params.RepoNamespace != "" {
			if target := namespaceFromParams.Domains[params.RepoName]; target != "" && target[2:] == scope {
				blocked = false
				break
			}
		}
	}

	return blocked
}

// CheckMergeProposalID performs sanity checks on merge proposal ID
func CheckMergeProposalID(id string, index int) error {
	if !govalidator.IsNumeric(id) {
		return fe(index, "mergeID", "merge proposal id must be numeric")
	}
	if len(id) > 8 {
		return fe(index, "mergeID", "merge proposal id exceeded 8 bytes limit")
	}
	return nil
}
