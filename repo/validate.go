package repo

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/util"

	"github.com/pkg/errors"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

// validateChange validates a change to a repository
// repo: The target repository
// change: The item that changed the repository
// gpgPubKeyGetter: Getter function for reading gpg public key
func validateChange(
	repo *Repo,
	change *ItemChange,
	gpgPubKeyGetter PGPPubKeyGetter) error {

	var commit *object.Commit
	var err error
	var tagObj *object.Tag
	var tagRef *plumbing.Reference

	if isBranch(change.Item.GetName()) {
		goto validateBranch
	}
	if isTag(change.Item.GetName()) {
		goto validateTag
	}

	if isNote(change.Item.GetName()) {
		goto validatedNote
	}

	return fmt.Errorf("unrecognised change item")

validateBranch:
	commit, err = repo.CommitObject(plumbing.NewHash(change.Item.GetData()))
	if err != nil {
		return errors.Wrap(err, "unable to get commit object")
	}
	return checkCommit(commit, false, repo, gpgPubKeyGetter)

validateTag:
	tagRef, err = repo.Tag(strings.ReplaceAll(change.Item.GetName(), "refs/tags/", ""))
	if err != nil {
		return errors.Wrap(err, "unable to get tag object")
	}

	// Get the tag object (for annotated tags)
	tagObj, err = repo.TagObject(tagRef.Hash())
	if err != nil && err != plumbing.ErrObjectNotFound {
		return err
	}

	// Here, the tag is not an annotated tag, so we need to
	// ensure the referenced commit is signed correctly
	if tagObj == nil {
		commit, err := repo.CommitObject(tagRef.Hash())
		if err != nil {
			return errors.Wrap(err, "unable to get commit")
		}
		return checkCommit(commit, true, repo, gpgPubKeyGetter)
	}

	// At this point, the tag is an annotated tag.
	// We have to ensure the annotated tag object is signed.
	return checkAnnotatedTag(tagObj, repo, gpgPubKeyGetter)

validatedNote:
	noteName := change.Item.GetName()
	return checkNote(repo, noteName, gpgPubKeyGetter)
}

// checkNote validates a note.
// noteName: The target note name
// repo: The repo where the tag exists in.
// gpgPubKeyGetter: Getter function for reading gpg public key
func checkNote(
	repo *Repo,
	noteName string,
	gpgPubKeyGetter PGPPubKeyGetter) error {

	// Find a all notes entries
	noteEntries, err := repo.ListTreeObjects(noteName, false)
	if err != nil {
		msg := fmt.Sprintf("unable to fetch note entries (%s)", noteName)
		return errors.Wrap(err, msg)
	}

	// From the entries, find a blob that contains a txline format
	// and stop after the first one is found
	var txBlob *object.Blob
	for hash := range noteEntries {
		obj, err := repo.BlobObject(plumbing.NewHash(hash))
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("failed to read object (%s)", hash))
		}
		r, err := obj.Reader()
		if err != nil {
			return err
		}
		prefix := make([]byte, 3)
		r.Read(prefix)
		if string(prefix) == util.TxLinePrefix {
			txBlob = obj
			break
		}
	}

	// Reject note if we didn't find a tx blob
	if txBlob == nil {
		return fmt.Errorf("unacceptable note. it does not have a signed transaction object")
	}

	// Get and parse the transaction line
	r, err := txBlob.Reader()
	if err != nil {
		return err
	}
	bz, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}
	txLine, err := util.ParseTxLine(string(bz))
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("note (%s)", noteName))
	}

	// Get the public key
	pubKeyStr, err := gpgPubKeyGetter(txLine.PubKeyID)
	if err != nil {
		msg := "unable to verify note (%s). public key was not found"
		return errors.Errorf(msg, noteName)
	}
	pubKey, err := crypto.PGPEntityFromPubKey(pubKeyStr)
	if err != nil {
		msg := "unable to verify note (%s). public key is not valid"
		return errors.Errorf(msg, noteName)
	}

	// Get the parent of the commit referenced by the note.
	// We need to use it to reconstruct the signature message in exactly the
	// same way it was constructed on the client side.
	noteHash := ""
	noteRef, err := repo.Reference(plumbing.ReferenceName(noteName), false)
	if err != nil {
		return errors.Wrap(err, "failed to get note reference")
	} else if noteRef != nil {
		noteRefCommit, err := repo.CommitObject(noteRef.Hash())
		if err != nil {
			return err
		}
		parent, err := noteRefCommit.Parent(0)
		if err != nil {
			return err
		}
		noteHash = parent.Hash.String()
	}

	// Now, verify the signature
	msg := []byte(txLine.Fee.String() + txLine.GetNonceString() + txLine.PubKeyID + noteHash)
	_, err = crypto.VerifyGPGSignature(pubKey, []byte(txLine.Signature), msg)
	if err != nil {
		msg := "note (%s) signature verification failed: %s"
		return errors.Errorf(msg, noteName, err.Error())
	}

	return nil
}

// checkAnnotatedTag validates an annotated tag.
// tag: The target annotated tag
// repo: The repo where the tag exists in.
// gpgPubKeyGetter: Getter function for reading gpg public key
func checkAnnotatedTag(
	tag *object.Tag,
	repo *Repo,
	gpgPubKeyGetter PGPPubKeyGetter) error {

	// Get and parse tx line from the commit message
	txLine, err := util.ParseTxLine(tag.Message)
	if err != nil {
		msg := fmt.Sprintf("tag (%s)", tag.Hash.String())
		return errors.Wrap(err, msg)
	}

	if tag.PGPSignature == "" {
		msg := "tag (%s) is unsigned. please sign the tag with your gpg key"
		return errors.Errorf(msg, tag.Hash.String())
	}

	// Get the public key
	pubKey, err := gpgPubKeyGetter(txLine.PubKeyID)
	if err != nil {
		msg := "unable to verify tag (%s). public key (id:%s) was not found"
		return errors.Errorf(msg, tag.Hash.String(), txLine.PubKeyID)
	}

	// Verify tag signature
	if _, err = tag.Verify(pubKey); err != nil {
		msg := "tag (%s) signature verification failed: %s"
		return errors.Errorf(msg, tag.Hash.String(), err.Error())
	}

	commit, err := tag.Commit()
	if err != nil {
		return errors.Wrap(err, "unable to get referenced commit")
	}
	return checkCommit(commit, true, repo, gpgPubKeyGetter)
}

// checkCommit checks a commit txline and verifies its signature
// commit: The target commit object
// isReferenced: Whether the commit was referenced somewhere (e.g in a tag)
// repo: The target repository where the commit exist in.
// gpgPubKeyGetter: Getter function for reading gpg public key
func checkCommit(
	commit *object.Commit,
	isReferenced bool,
	repo *Repo,
	gpgPubKeyGetter PGPPubKeyGetter) error {

	referencedStr := ""
	if isReferenced {
		referencedStr = "referenced "
	}

	// Get and parse tx line from the commit message
	txLine, err := util.ParseTxLine(commit.Message)
	if err != nil {
		msg := fmt.Sprintf("%scommit (%s)", referencedStr, commit.Hash.String())
		return errors.Wrap(err, msg)
	}

	if commit.PGPSignature == "" {
		msg := "%scommit (%s) is unsigned. please sign the commit with your gpg key"
		return errors.Errorf(msg, referencedStr, commit.Hash.String())
	}

	// Get the public key
	pubKey, err := gpgPubKeyGetter(txLine.PubKeyID)
	if err != nil {
		msg := "unable to verify %scommit (%s). public key (id:%s) was not found"
		return errors.Errorf(msg, referencedStr, commit.Hash.String(), txLine.PubKeyID)
	}

	// Verify commit signature
	if _, err = commit.Verify(pubKey); err != nil {
		msg := "%scommit (%s) signature verification failed: %s"
		return errors.Errorf(msg, referencedStr, commit.Hash.String(), err.Error())
	}

	return nil
}
