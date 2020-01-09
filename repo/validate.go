package repo

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	gv "github.com/asaskevich/govalidator"

	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/types"
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
	repo types.BareRepo,
	change *types.ItemChange,
	gpgPubKeyGetter types.PGPPubKeyGetter) (*util.TxLine, error) {

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

	return nil, fmt.Errorf("unrecognised change item")

validateBranch:
	commit, err = repo.CommitObject(plumbing.NewHash(change.Item.GetData()))
	if err != nil {
		return nil, errors.Wrap(err, "unable to get commit object")
	}
	return checkCommit(commit, false, repo, gpgPubKeyGetter)

validateTag:
	tagRef, err = repo.Tag(strings.ReplaceAll(change.Item.GetName(), "refs/tags/", ""))
	if err != nil {
		return nil, errors.Wrap(err, "unable to get tag object")
	}

	// Get the tag object (for annotated tags)
	tagObj, err = repo.TagObject(tagRef.Hash())
	if err != nil && err != plumbing.ErrObjectNotFound {
		return nil, err
	}

	// Here, the tag is not an annotated tag, so we need to
	// ensure the referenced commit is signed correctly
	if tagObj == nil {
		commit, err := repo.CommitObject(tagRef.Hash())
		if err != nil {
			return nil, errors.Wrap(err, "unable to get commit")
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
	repo types.BareRepo,
	noteName string,
	gpgPubKeyGetter types.PGPPubKeyGetter) (*util.TxLine, error) {

	// Find a all notes entries
	noteEntries, err := repo.ListTreeObjects(noteName, false)
	if err != nil {
		msg := fmt.Sprintf("unable to fetch note entries (%s)", noteName)
		return nil, errors.Wrap(err, msg)
	}

	// From the entries, find a blob that contains a txline format
	// and stop after the first one is found
	var txBlob *object.Blob
	for hash := range noteEntries {
		obj, err := repo.BlobObject(plumbing.NewHash(hash))
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("failed to read object (%s)", hash))
		}
		r, err := obj.Reader()
		if err != nil {
			return nil, err
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
		return nil, fmt.Errorf("unacceptable note. it does not have a signed transaction object")
	}

	// Get and parse the transaction line
	r, err := txBlob.Reader()
	if err != nil {
		return nil, err
	}
	bz, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	txLine, err := util.ParseTxLine(string(bz))
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("note (%s)", noteName))
	}

	// Get the public key
	pubKeyStr, err := gpgPubKeyGetter(txLine.PubKeyID)
	if err != nil {
		msg := "unable to verify note (%s). public key was not found"
		return nil, errors.Errorf(msg, noteName)
	}
	pubKey, err := crypto.PGPEntityFromPubKey(pubKeyStr)
	if err != nil {
		msg := "unable to verify note (%s). public key is not valid"
		return nil, errors.Errorf(msg, noteName)
	}

	// Get the parent of the commit referenced by the note.
	// We need to use it to reconstruct the signature message in exactly the
	// same way it was constructed on the client side.
	noteHash := ""
	noteRef, err := repo.Reference(plumbing.ReferenceName(noteName), false)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get note reference")
	} else if noteRef != nil {
		noteRefCommit, err := repo.CommitObject(noteRef.Hash())
		if err != nil {
			return nil, err
		}
		parent, err := noteRefCommit.Parent(0)
		if err != nil {
			return nil, err
		}
		noteHash = parent.Hash.String()
	}

	// Now, verify the signature
	msg := []byte(txLine.Fee.String() + txLine.GetNonceString() + txLine.PubKeyID + noteHash)
	_, err = crypto.VerifyGPGSignature(pubKey, []byte(txLine.Signature), msg)
	if err != nil {
		msg := "note (%s) signature verification failed: %s"
		return nil, errors.Errorf(msg, noteName, err.Error())
	}

	return txLine, nil
}

// checkAnnotatedTag validates an annotated tag.
// tag: The target annotated tag
// repo: The repo where the tag exists in.
// gpgPubKeyGetter: Getter function for reading gpg public key
func checkAnnotatedTag(
	tag *object.Tag,
	repo types.BareRepo,
	gpgPubKeyGetter types.PGPPubKeyGetter) (*util.TxLine, error) {

	// Get and parse tx line from the commit message
	txLine, err := util.ParseTxLine(tag.Message)
	if err != nil {
		msg := fmt.Sprintf("tag (%s)", tag.Hash.String())
		return nil, errors.Wrap(err, msg)
	}

	if tag.PGPSignature == "" {
		msg := "tag (%s) is unsigned. please sign the tag with your gpg key"
		return nil, errors.Errorf(msg, tag.Hash.String())
	}

	// Get the public key
	pubKey, err := gpgPubKeyGetter(txLine.PubKeyID)
	if err != nil {
		msg := "unable to verify tag (%s). public key (id:%s) was not found"
		return nil, errors.Errorf(msg, tag.Hash.String(), txLine.PubKeyID)
	}

	// Verify tag signature
	if _, err = tag.Verify(pubKey); err != nil {
		msg := "tag (%s) signature verification failed: %s"
		return nil, errors.Errorf(msg, tag.Hash.String(), err.Error())
	}

	commit, err := tag.Commit()
	if err != nil {
		return nil, errors.Wrap(err, "unable to get referenced commit")
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
	repo types.BareRepo,
	gpgPubKeyGetter types.PGPPubKeyGetter) (*util.TxLine, error) {

	referencedStr := ""
	if isReferenced {
		referencedStr = "referenced "
	}

	// Get and parse tx line from the commit message
	txLine, err := util.ParseTxLine(commit.Message)
	if err != nil {
		msg := fmt.Sprintf("%scommit (%s)", referencedStr, commit.Hash.String())
		return nil, errors.Wrap(err, msg)
	}

	if commit.PGPSignature == "" {
		msg := "%scommit (%s) is unsigned. please sign the commit with your gpg key"
		return nil, errors.Errorf(msg, referencedStr, commit.Hash.String())
	}

	// Get the public key
	pubKey, err := gpgPubKeyGetter(txLine.PubKeyID)
	if err != nil {
		msg := "unable to verify %scommit (%s). public key (id:%s) was not found"
		return nil, errors.Errorf(msg, referencedStr, commit.Hash.String(), txLine.PubKeyID)
	}

	// Verify commit signature
	if _, err = commit.Verify(pubKey); err != nil {
		msg := "%scommit (%s) signature verification failed: %s"
		return nil, errors.Errorf(msg, referencedStr, commit.Hash.String(), err.Error())
	}

	return txLine, nil
}

// CheckPushNoteSyntax performs syntactic checks on the fields of a push transaction
func CheckPushNoteSyntax(tx *types.PushNote) error {

	if tx.RepoName == "" {
		return types.FieldError("repoName", "repo name is required")
	}

	fe := types.FieldErrorWithIndex
	refsAcctNonce := uint64(0)
	for i, ref := range tx.References {
		if ref.Name == "" {
			return fe(i, "references.name", "name is required")
		}
		if ref.OldHash == "" {
			return fe(i, "references.oldHash", "old hash is required")
		}
		if len(ref.OldHash) != 40 {
			return fe(i, "references.oldHash", "old hash is not valid")
		}
		if ref.NewHash == "" {
			return fe(i, "references.newHash", "new hash is required")
		}
		if len(ref.NewHash) != 40 {
			return fe(i, "references.newHash", "new hash is not valid")
		}
		if ref.Nonce == 0 {
			return fe(i, "references.nonce", "reference nonce must be greater than zero")
		}
		if ref.AccountNonce == 0 {
			return fe(i, "references.accountNonce", "account nonce must be greater than zero")
		}
		if refsAcctNonce != 0 && ref.AccountNonce != refsAcctNonce {
			return fe(i, "references.accountNonce", "varying account nonce in a push note not allowed")
		}
		if ref.Fee == "" || !gv.IsFloat(ref.Fee.String()) {
			return fe(i, "references.fee", "fee must be numeric")
		}
		for j, obj := range ref.Objects {
			if len(obj) != 40 {
				return fe(i, fmt.Sprintf("references.objects.%d", j), "object hash is not valid")
			}
		}
		refsAcctNonce = ref.AccountNonce
	}

	if len(tx.PusherKeyID) == 0 {
		return types.FieldError("pusherKeyId", "pusher gpg key id is required")
	}
	if len(tx.PusherKeyID) != 20 {
		return types.FieldError("pusherKeyId", "pusher gpg key is not valid")
	}

	if tx.Timestamp > time.Now().Unix() {
		return types.FieldError("timestamp", "timestamp cannot be a future time")
	}

	if tx.NodePubKey.IsEmpty() {
		return types.FieldError("nodePubKey", "push node public key is required")
	}

	pk, err := crypto.PubKeyFromBytes(tx.NodePubKey.Bytes())
	if err != nil {
		return types.FieldError("nodePubKey", "push node public key is not valid")
	}

	if len(tx.NodeSig) == 0 {
		return types.FieldError("nodeSig", "push node signature is required")
	}

	if ok, err := pk.Verify(tx.BytesNoSig(), tx.NodeSig); err != nil || !ok {
		return types.FieldError("nodeSig", "failed to verify signature with public key")
	}

	return nil
}

// checkPushedReference validates pushed transactions
func checkPushedReference(
	targetRepo types.BareRepo,
	pRefs types.PushedReferences,
	repo *types.Repository,
	gpgKey *types.GPGPubKey,
	keepers types.Keepers) error {
	for i, ref := range pRefs {

		rName := ref.Name
		rNonce := ref.Nonce
		fmts := fmt.Sprintf
		oldHashIsZero := plumbing.NewHash(ref.OldHash).IsZero()

		// 1. We need to check if the reference exists in the repo.
		// Ignore references whose old hash is a 0-hash, these are new
		// references and as such we don't expect to find it in the repo.
		if !oldHashIsZero && !repo.References.Has(rName) {
			msg := fmts("reference '%s' is unknown", rName)
			return types.FieldErrorWithIndex(i, "references", msg)
		}

		// 2. If target repo is set and old hash is non-zero, we need to ensure
		// the current hash of the local version of the reference is the same as the old hash,
		// otherwise the pushed reference will not be compatible.
		if targetRepo != nil && !oldHashIsZero {
			localRef, err := targetRepo.Reference(plumbing.ReferenceName(rName), false)
			if err != nil {
				msg := fmts("reference '%s' does not exist locally", rName)
				return types.FieldErrorWithIndex(i, "references", msg)
			}

			if ref.OldHash != localRef.Hash().String() {
				msg := fmts("reference '%s' old hash does not match its local version", rName)
				return types.FieldErrorWithIndex(i, "references", msg)
			}
		}

		// 3. We need to check that the nonce is the expected next nonce of the
		// reference, otherwise we return an error.
		refInfo := repo.References.Get(rName)
		nextNonce := refInfo.Nonce + 1
		if nextNonce != ref.Nonce {
			msg := fmts("reference '%s' has nonce '%d', expecting '%d'", rName, rNonce, nextNonce)
			return types.FieldErrorWithIndex(i, "references", msg)
		}

		// 4. We need to ensure that the pusher's account nonce is the expected
		// next nonce, otherwise we return an error.
		pusherAccount := keepers.AccountKeeper().GetAccount(gpgKey.Address)
		nextNonce = pusherAccount.Nonce + 1
		if nextNonce != ref.AccountNonce {
			msg := fmts("reference '%s' has account nonce '%d', expecting '%d'", rName,
				ref.AccountNonce, nextNonce)
			return types.FieldErrorWithIndex(i, "references", msg)
		}
	}

	return nil
}

// CheckPushNoteConsistency performs consistency checks against the state of the
// repository as seen by the node. If the target repo object is not set in tx,
// local reference hash comparision is not performed.
func CheckPushNoteConsistency(tx *types.PushNote, keepers types.Keepers) error {

	// 1. Ensure the repository exist
	repo := keepers.RepoKeeper().GetRepo(tx.GetRepoName())
	if repo.IsNil() {
		msg := fmt.Sprintf("repository named '%s' is unknown", tx.GetRepoName())
		return types.FieldError("repoName", msg)
	}

	// 2. Get gpg key of the pusher
	gpgKey := keepers.GPGPubKeyKeeper().GetGPGPubKey(util.ToHex(tx.PusherKeyID))
	if gpgKey.IsNil() {
		msg := fmt.Sprintf("pusher's public key id '%s' is unknown", tx.PusherKeyID)
		return types.FieldError("pusherKeyId", msg)
	}

	// 3. Check each references against the state version
	if err := checkPushedReference(
		tx.GetTargetRepo(),
		tx.GetPushedReferences(),
		repo,
		gpgKey,
		keepers); err != nil {
		return err
	}

	return nil
}

// checkPushNote performs validation checks on a push transaction
func checkPushNote(tx types.RepoPushNote, keepers types.Keepers, dht types.DHT) error {

	if err := CheckPushNoteSyntax(tx.(*types.PushNote)); err != nil {
		return err
	}

	if err := CheckPushNoteConsistency(tx.(*types.PushNote), keepers); err != nil {
		return err
	}

	err := fetchAndCheckReferenceObjects(tx, dht)
	if err != nil {
		return err
	}

	return nil
}

// fetchAndCheckReferenceObjects attempts to fetch and store new objects
// introduced by the pushed references. After fetching it performs checks
// on the objects
func fetchAndCheckReferenceObjects(tx types.RepoPushNote, dht types.DHT) error {
	objectsSize := int64(0)

	for _, objHash := range tx.GetPushedObjects() {

	getSize:
		// Attempt to get the object's size. If we find it, it means the object
		// already exist so we don't have to fetch it from the dht.
		objSize, err := tx.GetTargetRepo().GetObjectSize(objHash)
		if err == nil {
			objectsSize += objSize
			continue
		}

		// Since the object doesn't exist locally, read the object from the DHT
		dhtKey := MakeRepoObjectDHTKey(tx.GetRepoName(), objHash)
		ctx, cn := context.WithTimeout(context.Background(), 60*time.Second)
		defer cn()
		objValue, err := dht.GetObject(ctx, &types.DHTObjectQuery{
			Module:    RepoObjectModule,
			ObjectKey: []byte(dhtKey),
		})
		if err != nil {
			msg := fmt.Sprintf("failed to fetch object '%s'", objHash)
			return errors.Wrap(err, msg)
		}

		// Write the object's content to disk
		if err := tx.GetTargetRepo().WriteObjectToFile(objHash, objValue); err != nil {
			msg := fmt.Sprintf("failed to write fetched object '%s' to disk", objHash)
			return errors.Wrap(err, msg)
		}

		goto getSize
	}

	if objectsSize != int64(tx.GetSize()) {
		msg := fmt.Sprintf("invalid size (%d bytes). "+
			"actual object size (%d bytes) is different", tx.GetSize(), objectsSize)
		return types.FieldError("size", msg)
	}

	return nil
}
