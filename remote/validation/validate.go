package validation

import (
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/asaskevich/govalidator"
	"gitlab.com/makeos/mosdef/crypto"
	plumbing2 "gitlab.com/makeos/mosdef/remote/plumbing"
	types2 "gitlab.com/makeos/mosdef/remote/pushpool/types"
	"gitlab.com/makeos/mosdef/remote/repo"
	"gitlab.com/makeos/mosdef/remote/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/util"

	"github.com/pkg/errors"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

var (
	fe                               = util.FieldErrorWithIndex
	ErrSigHeaderAndReqParamsMismatch = fmt.Errorf("request data and signature data mismatched")
)

type ChangeValidatorFunc func(
	repo types2.LocalRepo,
	oldHash string,
	change *core.ItemChange,
	txDetail *types.TxDetail,
	getPushKey core.PushKeyGetter) error

// ValidateChange validates a change to a repository
// repo: The target repository
// oldHash: The hash of the old reference
// change: The change to the reference
// txDetail: The pusher transaction detail
// getPushKey: Getter function for reading push key public key
func ValidateChange(
	localRepo types2.LocalRepo,
	oldHash string,
	change *core.ItemChange,
	detail *types.TxDetail,
	getPushKey core.PushKeyGetter) error {

	refname := change.Item.GetName()
	isIssueRef := plumbing2.IsIssueReferencePath(refname)
	isMergeRequestRef := plumbing2.IsMergeRequestReferencePath(refname)

	// Handle branch validation
	if plumbing2.IsBranch(refname) && !isIssueRef {
		commit, err := localRepo.CommitObject(plumbing.NewHash(change.Item.GetData()))
		if err != nil {
			return errors.Wrap(err, "unable to get commit object")
		}
		return CheckCommit(commit, detail, getPushKey)
	}

	// Handle issue or merge request branch validation.
	if plumbing2.IsBranch(refname) && (isIssueRef || isMergeRequestRef) {
		commit, err := localRepo.WrappedCommitObject(plumbing.NewHash(change.Item.GetData()))
		if err != nil {
			return errors.Wrap(err, "unable to get commit object")
		}
		return ValidatePostCommit(localRepo, commit, &ValidatePostCommitArg{
			OldHash:         oldHash,
			Change:          change,
			TxDetail:        detail,
			PushKeyGetter:   getPushKey,
			CheckCommit:     CheckCommit,
			CheckPostCommit: CheckPostCommit,
		})
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
	repo types2.LocalRepo,
	txDetail *types.TxDetail) error {

	// Get the note current hash
	noteHash, err := repo.RefGet(txDetail.Reference)
	if err != nil {
		return errors.Wrap(err, "failed to get note")
	}

	// Ensure the hash referenced in the tx detail matches the current note hash
	if noteHash != txDetail.Head {
		return fmt.Errorf("current note hash differs from signed note hash")
	}

	return nil
}

// CheckAnnotatedTag validates an annotated tag.
// tag: The target annotated tag
// txDetail: The pusher transaction detail
// getPushKey: Getter function for reading push key public key
func CheckAnnotatedTag(tag *object.Tag, txDetail *types.TxDetail, getPushKey core.PushKeyGetter) error {

	if tag.PGPSignature == "" {
		msg := "tag (%s) is unsigned. Sign the tag with your push key"
		return errors.Errorf(msg, tag.Hash.String())
	}

	pubKey, err := getPushKey(txDetail.PushKeyID)
	if err != nil {
		return errors.Wrapf(err, "failed to get pusher key(%s) to verify commit (%s)",
			txDetail.PushKeyID, tag.Hash.String())
	}

	tagTxDetail, err := VerifyCommitOrTagSignature(tag, pubKey)
	if err != nil {
		return err
	}

	// Ensure the transaction detail from request matches the transaction
	// detail extracted from the signature header
	if !txDetail.Equal(tagTxDetail) {
		return ErrSigHeaderAndReqParamsMismatch
	}

	commit, err := tag.Commit()
	if err != nil {
		return errors.Wrap(err, "unable to get referenced commit")
	}

	return CheckCommit(commit, txDetail, func(string) (key crypto.PublicKey, err error) {
		return pubKey, nil
	})
}

// GetCommitOrTagSigMsg returns the message that is signed to create a commit or tag signature
func GetCommitOrTagSigMsg(obj object.Object) string {
	encoded := &plumbing.MemoryObject{}
	switch o := obj.(type) {
	case *object.Commit:
		o.EncodeWithoutSignature(encoded)
	case *object.Tag:
		o.EncodeWithoutSignature(encoded)
	}
	rdr, _ := encoded.Reader()
	msg, _ := ioutil.ReadAll(rdr)
	return string(msg)
}

// verifyCommitSignature verifies commit and tag signatures
func VerifyCommitOrTagSignature(obj object.Object, pubKey crypto.PublicKey) (*types.TxDetail, error) {
	var sig, hash string

	// Extract the signature for commit or tag object
	encoded := &plumbing.MemoryObject{}
	switch o := obj.(type) {
	case *object.Commit:
		o.EncodeWithoutSignature(encoded)
		sig, hash = o.PGPSignature, o.Hash.String()
	case *object.Tag:
		o.EncodeWithoutSignature(encoded)
		sig, hash = o.PGPSignature, o.Hash.String()
	default:
		return nil, fmt.Errorf("unsupported object type")
	}

	// Decode sig from PEM format
	pemBlock, _ := pem.Decode([]byte(sig))
	if pemBlock == nil {
		return nil, fmt.Errorf("signature is malformed")
	}

	// Re-construct the transaction parameters
	txDetail, err := types.TxDetailFromPEMHeader(pemBlock.Headers)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode PEM header")
	}

	// Re-create the signature message
	rdr, _ := encoded.Reader()
	msg, _ := ioutil.ReadAll(rdr)
	msg = append(msg, txDetail.BytesNoSig()...)

	// Verify the signature
	pk := crypto.MustPubKeyFromBytes(pubKey.Bytes())
	if ok, err := pk.Verify(msg, pemBlock.Bytes); !ok || err != nil {
		return nil, fmt.Errorf("object (%s) signature is invalid", hash)
	}

	return txDetail, nil
}

// CommitChecker describes a function for checking a standard commit
type CommitChecker func(commit *object.Commit, txDetail *types.TxDetail, getPushKey core.PushKeyGetter) error

// CheckCommit validates a commit
// repo: The target repo
// commit: The target commit object
// txDetail: The push transaction detail
// getPushKey: Getter function for fetching push public key
func CheckCommit(commit *object.Commit, txDetail *types.TxDetail, getPushKey core.PushKeyGetter) error {

	// Signature must be set
	if commit.PGPSignature == "" {
		return fmt.Errorf("commit (%s) was not signed", commit.Hash.String())
	}

	// Get the push key for signature verification
	pubKey, err := getPushKey(txDetail.PushKeyID)
	if err != nil {
		return errors.Wrapf(err, "failed to get push key (%s)", txDetail.PushKeyID)
	}

	// Verify the signature
	commitTxDetail, err := VerifyCommitOrTagSignature(commit, pubKey)
	if err != nil {
		return err
	}

	// Ensure the transaction detail from request matches the transaction
	// detail extracted from the signature header
	if !txDetail.Equal(commitTxDetail) {
		return ErrSigHeaderAndReqParamsMismatch
	}

	return nil
}

// IsBlockedByScope checks whether the given tx parameter satisfy a given scope
func IsBlockedByScope(scopes []string, params *types.TxDetail, namespaceFromParams *state.Namespace) bool {
	blocked := true
	for _, scope := range scopes {
		if util.IsNamespaceURI(scope) {
			ns, domain, _ := util.SplitNamespaceDomain(scope)

			// If scope is r/repo-name, make sure tx info namespace is unset and repo name is 'repo-name'.
			// If scope is r/ only, make sure only tx info namespace is set
			if ns == repo.DefaultNS && params.RepoNamespace == "" && (domain == "" || domain == params.RepoName) {
				blocked = false
				break
			}

			// If scope is some_ns/repo-name, make sure tx info namespace and repo name matches the scope
			// namespace and repo name.
			if ns != repo.DefaultNS && ns == params.RepoNamespace && domain == params.RepoName {
				blocked = false
				break
			}

			// If scope is just some_ns/, make sure tx info namespace matches
			if ns != repo.DefaultNS && domain == "" && ns == params.RepoNamespace {
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
