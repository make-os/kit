package repo

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"gitlab.com/makeos/mosdef/dht/types"
	tickettypes "gitlab.com/makeos/mosdef/ticket/types"
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"

	gv "github.com/asaskevich/govalidator"
	"github.com/shopspring/decimal"
	"github.com/thoas/go-funk"

	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/crypto/bls"
	"gitlab.com/makeos/mosdef/params"
	"gitlab.com/makeos/mosdef/util"

	"github.com/pkg/errors"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

var feI = util.FieldErrorWithIndex

// validateChange validates a change to a repository
// repo: The target repository
// change: The item that changed the repository
// pushKeyGetter: Getter function for reading push key public key
func validateChange(
	repo core.BareRepo,
	change *core.ItemChange,
	pushKeyGetter core.PushKeyGetter) (*util.TxParams, error) {

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
	return checkCommit(commit, repo, pushKeyGetter)

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
		return checkCommit(commit, repo, pushKeyGetter)
	}

	// At this point, the tag is an annotated tag.
	// We have to ensure the annotated tag object is signed.
	return checkAnnotatedTag(tagObj, repo, pushKeyGetter)

validatedNote:
	noteName := change.Item.GetName()
	return checkNote(repo, noteName, pushKeyGetter)
}

// checkNote validates a note.
// noteName: The target note name
// repo: The repo where the tag exists in.
// pushKeyGetter: Getter function for reading push key public key
func checkNote(
	repo core.BareRepo,
	noteName string,
	pushKeyGetter core.PushKeyGetter) (*util.TxParams, error) {

	// Get a all notes entries
	noteEntries, err := repo.ListTreeObjects(noteName, false)
	if err != nil {
		msg := fmt.Sprintf("unable to fetch note entries (%s)", noteName)
		return nil, errors.Wrap(err, msg)
	}

	// From the entries, find the blob that contains the transaction parameters
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
		if string(prefix) == util.TxParamsPrefix {
			txBlob = obj
			break
		}
	}

	// Reject note if we didn't find a tx blob
	if txBlob == nil {
		return nil, fmt.Errorf("note does not include a transaction parameter blob")
	}

	// Get and parse the transaction line
	r, _ := txBlob.Reader()
	bz, _ := ioutil.ReadAll(r)
	txParams, err := util.ExtractTxParams(string(bz))
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("note (%s) has invalid transaction parameters", noteName))
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

	pubKey, err := pushKeyGetter(txParams.PushKeyID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get push key (%s)", txParams.PushKeyID)
	}
	pk := crypto.MustPubKeyFromBytes(pubKey.Bytes())

	// Now, verify the signature
	msg := MakeNoteSigMsg(txParams.Fee.String(), txParams.GetNonceAsString(),
		txParams.PushKeyID, noteHash, txParams.DeleteRef)
	ok, err := pk.Verify(msg, []byte(txParams.Signature))
	if err != nil || !ok {
		return nil, errors.Errorf("note (%s) signature verification failed: %s", noteName, err)
	}

	return txParams, nil
}

// checkAnnotatedTag validates an annotated tag.
// tag: The target annotated tag
// repo: The repo where the tag exists in.
// pushKeyGetter: Getter function for reading push key public key
func checkAnnotatedTag(
	tag *object.Tag,
	repo core.BareRepo,
	pushKeyGetter core.PushKeyGetter) (*util.TxParams, error) {

	txParams, err := util.ExtractTxParams(tag.Message)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("tag (%s)", tag.Hash.String()))
	}

	if tag.PGPSignature == "" {
		msg := "tag (%s) is unsigned. Sign the tag with your push key"
		return nil, errors.Errorf(msg, tag.Hash.String())
	}

	pubKey, err := pushKeyGetter(txParams.PushKeyID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get pusher key(%s) to verify commit (%s)",
			txParams.PushKeyID, tag.Hash.String())
	}

	if err = verifyCommitOrTagSignature(tag, pubKey); err != nil {
		return nil, err
	}

	commit, err := tag.Commit()
	if err != nil {
		return nil, errors.Wrap(err, "unable to get referenced commit")
	}

	return checkCommit(commit, repo, pushKeyGetter)
}

// checkMergeCompliance checks whether push to a branch satisfied
// an accepted merge proposal
func checkMergeCompliance(
	repo core.BareRepo,
	change *core.ItemChange,
	oldRef core.Item,
	mergeProposalID,
	pushKeyID string,
	keepers core.Keepers) error {

	ref := plumbing.ReferenceName(change.Item.GetName())
	if !ref.IsBranch() {
		return fmt.Errorf("merge error: pushed reference must be a branch")
	}

	prop := repo.State().Proposals.Get(mergeProposalID)
	if prop == nil {
		return fmt.Errorf("merge error: merge proposal (%s) not found", mergeProposalID)
	}

	// Ensure the signer is the creator of the proposal
	pushKey := keepers.PushKeyKeeper().Get(pushKeyID)
	if pushKey.Address.String() != prop.Creator {
		return fmt.Errorf("merge error: signer did not create the proposal  (%s)", mergeProposalID)
	}

	// Check if the merge proposal has been closed
	closed, err := keepers.RepoKeeper().IsProposalClosed(repo.GetName(), mergeProposalID)
	if err != nil {
		return fmt.Errorf("merge error: %s", err)
	} else if closed {
		return fmt.Errorf("merge error: merge proposal (%s) is already closed", mergeProposalID)
	}

	// Ensure the proposal's base branch matches the pushed branch
	var propBaseBranch string
	_ = util.ToObject(prop.ActionData[constants.ActionDataKeyBaseBranch], &propBaseBranch)
	if ref.Short() != propBaseBranch {
		return fmt.Errorf("merge error: pushed branch name and proposal base branch name must match")
	}

	// Check whether the merge proposal has been accepted
	if prop.Outcome != state.ProposalOutcomeAccepted {
		return fmt.Errorf("merge error: merge proposal (%s) has not been accepted", mergeProposalID)
	}

	// Get the commit that initiated the merge operation (a.k.a "merger commit").
	// Since by convention, its parent is considered the actual merge target.
	// As such, we need to perform some validation before we compare it with
	// the merge proposal target hash.
	commit, err := repo.WrappedCommitObject(plumbing.NewHash(change.Item.GetData()))
	if err != nil {
		return errors.Wrap(err, "unable to get commit object")
	}

	// Ensure the merger commit does not have multiple parents
	if commit.NumParents() > 1 {
		return fmt.Errorf("merge error: multiple targets not allowed")
	}

	// Ensure the difference between the target commit and the merger commit
	// only exist in the commit hash and not the tree, author and committer information.
	// By convention, the merger commit can only modify its commit object (time,
	// message and signature).
	targetCommit, _ := commit.Parent(0)
	if commit.GetTreeHash() != targetCommit.GetTreeHash() ||
		commit.GetAuthor().String() != targetCommit.GetAuthor().String() ||
		commit.GetCommitter().String() != targetCommit.GetCommitter().String() {
		return fmt.Errorf("merge error: merger commit " +
			"cannot modify history as seen from target commit")
	}

	// When no older reference (ex. a new/first branch),
	// set default hash value to zero hash.
	oldRefHash := plumbing.ZeroHash.String()
	if oldRef != nil {
		oldRefHash = oldRef.GetData()
	}

	// When no base hash is given, set default hash value to zero hash
	var propBaseHash string
	_ = util.ToObject(prop.ActionData[constants.ActionDataKeyBaseHash], &propBaseHash)
	propBaseHashStr := plumbing.ZeroHash.String()
	if propBaseHash != "" {
		propBaseHashStr = propBaseHash
	}

	// Ensure the proposals base branch hash matches the hash of the current
	// branch before this current push/change.
	if propBaseHashStr != oldRefHash {
		return fmt.Errorf("merge error: merge proposal base " +
			"branch hash is stale or invalid")
	}

	// Ensure the target commit and the proposal target match
	var propTargetHash string
	_ = util.ToObject(prop.ActionData[constants.ActionDataKeyTargetHash], &propTargetHash)
	if targetCommit.GetHash().String() != propTargetHash {
		return fmt.Errorf("merge error: target commit hash and " +
			"the merge proposal target hash must match")
	}

	return nil
}

// verifyCommitSignature verifies commit and tag signatures
func verifyCommitOrTagSignature(obj object.Object, pubKey crypto.PublicKey) error {
	encoded := &plumbing.MemoryObject{}
	sig := []byte{}
	hash := ""
	switch o := obj.(type) {
	case *object.Commit:
	case *object.Tag:
		o.EncodeWithoutSignature(encoded)
		sig = []byte(o.PGPSignature)
		hash = o.Hash.String()
	default:
		return fmt.Errorf("unsupported object type")
	}

	rdr, _ := encoded.Reader()
	msg, _ := ioutil.ReadAll(rdr)
	pk := crypto.MustPubKeyFromBytes(pubKey.Bytes())
	if ok, err := pk.Verify(msg, sig); !ok || err != nil {
		return errors.Wrapf(err, "commit (%s) signature is invalid", hash)
	}

	return nil
}

// checkCommit validates a commit
// commit: The target commit object
// repo: The target repository where the commit exist in.
// pushKeyGetter: Getter function for fetching push public key
func checkCommit(commit *object.Commit, repo core.BareRepo, pushKeyGetter core.PushKeyGetter) (*util.TxParams, error) {

	txParams, err := util.ExtractTxParams(commit.Message)
	if err != nil {
		return nil, errors.Wrapf(err, "commit (%s)", commit.Hash.String())
	}

	if commit.PGPSignature == "" {
		return nil, errors.Wrapf(err, "commit (%s) was not signed", commit.Hash.String())
	}

	pubKey, err := pushKeyGetter(txParams.PushKeyID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get push key (%s)", txParams.PushKeyID)
	}

	if err = verifyCommitOrTagSignature(commit, pubKey); err != nil {
		return nil, err
	}

	return txParams, nil
}

// checkPushNoteAgainstTxParams checks compares the value of fields in the push
// note against the values of same fields in the txparams.
func checkPushNoteAgainstTxParams(pn *core.PushNote, txParams map[string]*util.TxParams) error {

	// Push note pusher public key must match txparams key
	txParamsObjs := funk.Values(txParams).([]*util.TxParams)
	if !bytes.Equal(pn.PushKeyID, util.MustDecodePushKeyID(txParamsObjs[0].PushKeyID)) {
		return fmt.Errorf("push note pusher key id does not match " +
			"push key in tx parameter")
	}

	totalFees := decimal.Zero
	for _, txparams := range txParams {
		totalFees = totalFees.Add(txparams.Fee.Decimal())
	}

	// Push note total fee must matches the total fees computed from all txparams
	if !pn.GetFee().Decimal().Equal(totalFees) {
		return fmt.Errorf("push note fees does not match total txparams fees")
	}

	// Check pushed references for consistency with their txparams
	for _, ref := range pn.GetPushedReferences() {
		txparams, ok := txParams[ref.Name]
		if !ok {
			return fmt.Errorf("push note has unexpected pushed reference (%s)", ref.Name)
		}

		if txparams.DeleteRef != ref.Delete {
			return fmt.Errorf("pushed reference (%s) has an "+
				"unexpected delete directive value", ref.Name)
		}
	}

	return nil
}

// CheckPushNoteSyntax performs syntactic checks on the fields of a push transaction
func CheckPushNoteSyntax(tx *core.PushNote) error {

	if tx.RepoName == "" {
		return util.FieldError("repoName", "repo name is required")
	} else if util.IsValidIdentifierName(tx.RepoName) != nil {
		return util.FieldError("repoName", "repo name is not valid")
	}

	if tx.Namespace != "" && util.IsValidIdentifierName(tx.Namespace) != nil {
		return util.FieldError("namespace", "namespace is not valid")
	}

	fe := util.FieldErrorWithIndex
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
		for j, obj := range ref.Objects {
			if len(obj) != 40 {
				return fe(i, fmt.Sprintf("references.objects.%d", j), "object hash is not valid")
			}
		}
	}

	if len(tx.PushKeyID) == 0 {
		return util.FieldError("pusherKeyId", "pusher push key key id is required")
	}
	if len(tx.PushKeyID) != 20 {
		return util.FieldError("pusherKeyId", "pusher push key key is not valid")
	}

	if tx.Timestamp == 0 {
		return util.FieldError("timestamp", "timestamp is required")
	}
	if tx.Timestamp > time.Now().Unix() {
		return util.FieldError("timestamp", "timestamp cannot be a future time")
	}

	if tx.PusherAcctNonce == 0 {
		return util.FieldError("accountNonce", "account nonce must be greater than zero")
	}

	if tx.Fee == "" {
		return util.FieldError("fee", "fee is required")
	}
	if !gv.IsFloat(tx.Fee.String()) {
		return util.FieldError("fee", "fee must be numeric")
	}

	if tx.NodePubKey.IsEmpty() {
		return util.FieldError("nodePubKey", "push node public key is required")
	}

	pk, err := crypto.PubKeyFromBytes(tx.NodePubKey.Bytes())
	if err != nil {
		return util.FieldError("nodePubKey", "push node public key is not valid")
	}

	if len(tx.NodeSig) == 0 {
		return util.FieldError("nodeSig", "push node signature is required")
	}

	if ok, err := pk.Verify(tx.BytesNoSig(), tx.NodeSig); err != nil || !ok {
		return util.FieldError("nodeSig", "failed to verify signature")
	}

	return nil
}

// checkPushedReference validates pushed transactions
func checkPushedReference(
	targetRepo core.BareRepo,
	pRefs core.PushedReferences,
	repo *state.Repository,
	keepers core.Keepers) error {
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
			return util.FieldErrorWithIndex(i, "references", msg)
		}

		// 2. If target repo is set and old hash is non-zero, we need to ensure
		// the current hash of the local version of the reference is the same as the old hash,
		// otherwise the pushed reference will not be compatible.
		if targetRepo != nil && !oldHashIsZero {
			localRef, err := targetRepo.Reference(plumbing.ReferenceName(rName), false)
			if err != nil {
				msg := fmts("reference '%s' does not exist locally", rName)
				return util.FieldErrorWithIndex(i, "references", msg)
			}

			if ref.OldHash != localRef.Hash().String() {
				msg := fmts("reference '%s' old hash does not match its local version", rName)
				return util.FieldErrorWithIndex(i, "references", msg)
			}
		}

		// 3. We need to check that the nonce is the expected next nonce of the
		// reference, otherwise we return an error.
		refInfo := repo.References.Get(rName)
		nextNonce := refInfo.Nonce + 1
		if nextNonce != ref.Nonce {
			msg := fmts("reference '%s' has nonce '%d', expecting '%d'", rName, rNonce, nextNonce)
			return util.FieldErrorWithIndex(i, "references", msg)
		}
	}

	return nil
}

// CheckPushNoteConsistency performs consistency checks against the state of the
// repository as seen by the node. If the target repo object is not set in tx,
// local reference hash comparision is not performed.
func CheckPushNoteConsistency(note *core.PushNote, logic core.Logic) error {

	// Ensure the repository exist
	repo := logic.RepoKeeper().Get(note.GetRepoName())
	if repo.IsNil() {
		msg := fmt.Sprintf("repository named '%s' is unknown", note.GetRepoName())
		return util.FieldError("repoName", msg)
	}

	// If namespace is provide, ensure it exists
	if note.Namespace != "" {
		if logic.NamespaceKeeper().Get(util.HashNamespace(note.Namespace)).IsNil() {
			return util.FieldError("namespace", fmt.Sprintf("namespace '%s' is unknown", note.Namespace))
		}
	}

	// Get push key key of the pusher
	pushKey := logic.PushKeyKeeper().Get(crypto.BytesToPushKeyID(note.PushKeyID))
	if pushKey.IsNil() {
		msg := fmt.Sprintf("pusher's public key id '%s' is unknown", note.PushKeyID)
		return util.FieldError("pusherKeyId", msg)
	}

	// Ensure the push key key linked address matches the pusher address
	if pushKey.Address != note.PusherAddress {
		return util.FieldError("pusherAddr", "push key key is not associated with the pusher address")
	}

	// Ensure next pusher account nonce matches the pushed note's keystore nonce
	pusherAcct := logic.AccountKeeper().Get(note.PusherAddress)
	if pusherAcct.IsNil() {
		return util.FieldError("pusherAddr", "pusher account not found")
	}
	nextNonce := pusherAcct.Nonce + 1
	if note.PusherAcctNonce != nextNonce {
		msg := fmt.Sprintf("wrong account nonce '%d', expecting '%d'", note.PusherAcctNonce, nextNonce)
		return util.FieldError("accountNonce", msg)
	}

	// Check each references against the state version
	if err := checkPushedReference(
		note.GetTargetRepo(),
		note.GetPushedReferences(),
		repo,
		logic); err != nil {
		return err
	}

	// Check whether the pusher can pay the specified transaction fee
	bi, err := logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		return errors.Wrap(err, "failed to fetch current block info")
	}
	if err = logic.Tx().CanExecCoinTransfer(
		note.PusherAddress,
		"0",
		note.GetFee(),
		note.PusherAcctNonce,
		uint64(bi.Height)); err != nil {
		return err
	}

	return nil
}

// checkPushNote performs validation checks on a push transaction
func checkPushNote(tx core.RepoPushNote, dht types.DHTNode, logic core.Logic) error {

	if err := CheckPushNoteSyntax(tx.(*core.PushNote)); err != nil {
		return err
	}

	if err := CheckPushNoteConsistency(tx.(*core.PushNote), logic); err != nil {
		return err
	}

	err := fetchAndCheckReferenceObjects(tx, dht)
	if err != nil {
		return err
	}

	return nil
}

// CheckPushOK performs sanity checks on the given PushOK object
func CheckPushOK(pushOK *core.PushOK, index int) error {

	// Push note id must be set
	if pushOK.PushNoteID.IsEmpty() {
		return feI(index, "endorsements.pushNoteID", "push note id is required")
	}

	// Sender public key must be set
	if pushOK.SenderPubKey.IsEmpty() {
		return feI(index, "endorsements.senderPubKey", "sender public key is required")
	}

	return nil
}

// CheckPushOKConsistencyUsingHost performs consistency checks on the given PushOK object
// against the current state of the network.
// EXPECT: Sanity check to have been performed using CheckPushOK
func CheckPushOKConsistencyUsingHost(hosts tickettypes.SelectedTickets, pushOK *core.PushOK, logic core.Logic, noSigCheck bool, index int) error {

	// Check if the sender is one of the top hosts.
	// Ensure that the signers of the PushOK are part of the hosts
	signerSelectedTicket := hosts.Get(pushOK.SenderPubKey)
	if signerSelectedTicket == nil {
		return feI(index, "endorsements.senderPubKey",
			"sender public key does not belong to an active host")
	}

	// Ensure the signature can be verified using the BLS public key of the selected ticket
	if !noSigCheck {
		blsPubKey, err := bls.BytesToPublicKey(signerSelectedTicket.Ticket.BLSPubKey)
		if err != nil {
			return errors.Wrap(err, "failed to decode bls public key of endorser")
		}
		if err = blsPubKey.Verify(pushOK.Sig.Bytes(), pushOK.BytesNoSigAndSenderPubKey()); err != nil {
			return feI(index, "endorsements.sig", "signature could not be verified")
		}
	}

	return nil
}

// CheckPushOKConsistency performs consistency checks on the given PushOK object
// against the current state of the network.
// EXPECT: Sanity check to have been performed using CheckPushOK
func CheckPushOKConsistency(pushOK *core.PushOK, logic core.Logic, noSigCheck bool, index int) error {
	hosts, err := logic.GetTicketManager().GetTopHosts(params.NumTopHostsLimit)
	if err != nil {
		return errors.Wrap(err, "failed to get top hosts")
	}
	return CheckPushOKConsistencyUsingHost(hosts, pushOK, logic, noSigCheck, index)
}

// checkPushOK performs sanity and state consistency checks on the given PushOK object
func checkPushOK(pushOK *core.PushOK, logic core.Logic, index int) error {
	if err := CheckPushOK(pushOK, index); err != nil {
		return err
	}
	if err := CheckPushOKConsistency(pushOK, logic, false, index); err != nil {
		return err
	}
	return nil
}

// fetchAndCheckReferenceObjects attempts to fetch and store new objects
// introduced by the pushed references. After fetching it performs checks
// on the objects
func fetchAndCheckReferenceObjects(tx core.RepoPushNote, dhtnode types.DHTNode) error {
	objectsSize := int64(0)

	for _, objHash := range tx.GetPushedObjects(false) {

	getSize:
		// Attempt to get the object's size. If we find it, it means the object
		// already exist so we don't have to fetch it from the dhtnode.
		objSize, err := tx.GetTargetRepo().GetObjectSize(objHash)
		if err == nil {
			objectsSize += objSize
			continue
		}

		// Since the object doesn't exist locally, read the object from the DHTNode
		dhtKey := MakeRepoObjectDHTKey(tx.GetRepoName(), objHash)
		ctx, cn := context.WithTimeout(context.Background(), 60*time.Second)
		defer cn()
		objValue, err := dhtnode.GetObject(ctx, &types.DHTObjectQuery{
			Module:    core.RepoObjectModule,
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
		return util.FieldError("size", msg)
	}

	return nil
}

// checkPushRequestTokenData performs sanity and consistency checks on a push request token
func checkPushRequestTokenData(prt *PushRequestTokenData, keepers core.Keepers) error {
	if err := checkPushRequestTokenDataSanity(prt); err != nil {
		return err
	}
	return checkPushRequestTokenDataConsistency(prt, keepers)
}

// checkPushRequestTokenDataSanity performs sanity checks on push request token data
func checkPushRequestTokenDataSanity(prt *PushRequestTokenData) error {

	if prt.PushKeyID == "" {
		return util.FieldError("pkID", "push key id is required")
	}
	if !util.IsValidPushKeyID(prt.PushKeyID) {
		return util.FieldError("pkID", "push key id is not valid")
	}

	if prt.Nonce == 0 {
		return util.FieldError("nonce", "nonce must be a positive number")
	}

	return nil
}

// checkPushRequestTokenDataConsistency performs consistency checks on the push request token data
func checkPushRequestTokenDataConsistency(prt *PushRequestTokenData, keepers core.Keepers) error {

	pushKey := keepers.PushKeyKeeper().Get(prt.PushKeyID)
	if pushKey.IsNil() {
		return util.FieldError("pkID", "push key id is required")
	}

	keyOwner := keepers.AccountKeeper().Get(pushKey.Address)
	if prt.Nonce <= keyOwner.Nonce {
		return util.FieldError("nonce", fmt.Sprintf("nonce (%d) must be "+
			"greater than current key owner nonce (%d)", prt.Nonce, keyOwner.Nonce))
	}

	return nil
}
