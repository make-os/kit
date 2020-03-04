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
// gpgPubKeyGetter: Getter function for reading gpg public key
func validateChange(
	repo core.BareRepo,
	change *core.ItemChange,
	gpgPubKeyGetter core.PGPPubKeyGetter) (*util.TxParams, error) {

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
	repo core.BareRepo,
	noteName string,
	gpgPubKeyGetter core.PGPPubKeyGetter) (*util.TxParams, error) {

	// Find a all notes entries
	noteEntries, err := repo.ListTreeObjects(noteName, false)
	if err != nil {
		msg := fmt.Sprintf("unable to fetch note entries (%s)", noteName)
		return nil, errors.Wrap(err, msg)
	}

	// From the entries, find a blob that contains a txparams format
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
		if string(prefix) == util.TxParamsPrefix {
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
	txParams, err := util.ParseTxParams(string(bz))
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("note (%s)", noteName))
	}

	// Get the public key
	pubKeyStr, err := gpgPubKeyGetter(txParams.PubKeyID)
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
	// TODO: use MakeNoteSigMsg
	msg := []byte(txParams.Fee.String() + txParams.GetNonceAsString() + txParams.PubKeyID + noteHash +
		fmt.Sprintf("%v", txParams.DeleteRef))
	_, err = crypto.VerifyGPGSignature(pubKey, []byte(txParams.Signature), msg)
	if err != nil {
		msg := "note (%s) signature verification failed: %s"
		return nil, errors.Errorf(msg, noteName, err.Error())
	}

	return txParams, nil
}

// checkAnnotatedTag validates an annotated tag.
// tag: The target annotated tag
// repo: The repo where the tag exists in.
// gpgPubKeyGetter: Getter function for reading gpg public key
func checkAnnotatedTag(
	tag *object.Tag,
	repo core.BareRepo,
	gpgPubKeyGetter core.PGPPubKeyGetter) (*util.TxParams, error) {

	// Get and parse txparams from the commit message
	txParams, err := util.ParseTxParams(tag.Message)
	if err != nil {
		msg := fmt.Sprintf("tag (%s)", tag.Hash.String())
		return nil, errors.Wrap(err, msg)
	}

	if tag.PGPSignature == "" {
		msg := "tag (%s) is unsigned. please sign the tag with your gpg key"
		return nil, errors.Errorf(msg, tag.Hash.String())
	}

	// Get the public key
	pubKey, err := gpgPubKeyGetter(txParams.PubKeyID)
	if err != nil {
		msg := "unable to verify tag (%s). public key (id:%s) was not found"
		return nil, errors.Errorf(msg, tag.Hash.String(), txParams.PubKeyID)
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

// checkMergeCompliance checks whether push to a branch satisfied
// an accepted merge proposal
func checkMergeCompliance(
	repo core.BareRepo,
	change *core.ItemChange,
	oldRef core.Item,
	mergeProposalID,
	gpgID string,
	keepers core.Keepers) error {

	ref := plumbing.ReferenceName(change.Item.GetName())
	if !ref.IsBranch() {
		return fmt.Errorf("merge compliance error: pushed reference must be a branch")
	}

	prop := repo.State().Proposals.Get(mergeProposalID)
	if prop == nil {
		return fmt.Errorf("merge compliance error: "+
			"merge proposal (%s) not found", mergeProposalID)
	}

	// Ensure the signer is the creator of the proposal
	gpgKey := keepers.GPGPubKeyKeeper().GetGPGPubKey(gpgID)
	if gpgKey.Address.String() != prop.Creator {
		return fmt.Errorf("merge compliance error: "+
			"signer must be the creator of the merge proposal (%s)", mergeProposalID)
	}

	// Check if the merge proposal has been closed
	closed, err := keepers.RepoKeeper().IsProposalClosed(repo.GetName(), mergeProposalID)
	if err != nil {
		return fmt.Errorf("merge compliance error: %s", err)
	} else if closed {
		return fmt.Errorf("merge compliance error: "+
			"merge proposal (%s) is already closed", mergeProposalID)
	}

	actionKey := core.ProposalActionDataMergeRequest

	// Ensure the proposal's base branch matches the pushed branch
	propBaseBranch := prop.ActionData.Get(actionKey)["base"]
	if ref.Short() != propBaseBranch.(string) {
		return fmt.Errorf("merge compliance error: pushed branch name and " +
			"merge proposal base branch name must match")
	}

	// Check whether the merge proposal has been accepted
	if prop.Outcome != state.ProposalOutcomeAccepted {
		return fmt.Errorf("merge compliance error: "+
			"merge proposal (%s) has not been accepted", mergeProposalID)
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
		return fmt.Errorf("merge compliance error: multiple targets not allowed")
	}

	// Ensure the difference between the target commit and the merger commit
	// only exist in the commit hash and not the tree, author and committer information.
	// By convention, the merger commit can only modify its commit object (time,
	// message and signature).
	targetCommit, _ := commit.Parent(0)
	if commit.GetTreeHash() != targetCommit.GetTreeHash() ||
		commit.GetAuthor().String() != targetCommit.GetAuthor().String() ||
		commit.GetCommitter().String() != targetCommit.GetCommitter().String() {
		return fmt.Errorf("merge compliance error: merger commit " +
			"cannot modify history as seen from target commit")
	}

	// When no older reference (ex. a new/first branch),
	// set default hash value to zero hash.
	oldRefHash := plumbing.ZeroHash.String()
	if oldRef != nil {
		oldRefHash = oldRef.GetData()
	}

	// When no base hash is given, set default hash value to zero hash
	propBaseHash := prop.ActionData.Get(actionKey)["baseHash"]
	propBaseHashStr := plumbing.ZeroHash.String()
	if propBaseHash.(string) != "" {
		propBaseHashStr = propBaseHash.(string)
	}

	// Ensure the proposals base branch hash matches the hash of the current
	// branch before this current push/change.
	if propBaseHashStr != oldRefHash {
		return fmt.Errorf("merge compliance error: merge proposal base " +
			"branch hash is stale or invalid")
	}

	// Ensure the target commit and the proposal target match
	propTargetHash := prop.ActionData.Get(actionKey)["targetHash"]
	if targetCommit.GetHash().String() != propTargetHash.(string) {
		return fmt.Errorf("merge compliance error: target commit hash and " +
			"the merge proposal target hash must match")
	}

	return nil
}

// checkCommit checks a commit txparams and verifies its signature
// commit: The target commit object
// isReferenced: Whether the commit was referenced somewhere (e.g in a tag)
// repo: The target repository where the commit exist in.
// gpgPubKeyGetter: Getter function for reading gpg public key
func checkCommit(
	commit *object.Commit,
	isReferenced bool,
	repo core.BareRepo,
	gpgPubKeyGetter core.PGPPubKeyGetter) (*util.TxParams, error) {

	referencedStr := ""
	if isReferenced {
		referencedStr = "referenced "
	}

	// Get and parse txparams from the commit message
	txParams, err := util.ParseTxParams(commit.Message)
	if err != nil {
		msg := fmt.Sprintf("%scommit (%s)", referencedStr, commit.Hash.String())
		return nil, errors.Wrap(err, msg)
	}

	if commit.PGPSignature == "" {
		msg := "%scommit (%s) is unsigned. please sign the commit with your gpg key"
		return nil, errors.Errorf(msg, referencedStr, commit.Hash.String())
	}

	// Get the public key
	pubKey, err := gpgPubKeyGetter(txParams.PubKeyID)
	if err != nil {
		msg := "unable to verify %scommit (%s). public key (id:%s) was not found"
		return nil, errors.Errorf(msg, referencedStr, commit.Hash.String(), txParams.PubKeyID)
	}

	// Verify commit signature
	if _, err = commit.Verify(pubKey); err != nil {
		msg := "%scommit (%s) signature verification failed: %s"
		return nil, errors.Errorf(msg, referencedStr, commit.Hash.String(), err.Error())
	}

	return txParams, nil
}

// checkPushNoteAgainstTxParamss checks compares the value of fields in the push
// note against the values of same fields in the txparamss.
func checkPushNoteAgainstTxParamss(pn *core.PushNote, txParamss map[string]*util.TxParams) error {

	// Push note pusher public key must match txparams key
	txParamssObjs := funk.Values(txParamss).([]*util.TxParams)
	if !bytes.Equal(pn.PusherKeyID, util.MustDecodeGPGIDToRSAHash(txParamssObjs[0].PubKeyID)) {
		return fmt.Errorf("push note pusher public key id does not match " +
			"txparamss pusher public key id")
	}

	totalFees := decimal.Zero
	for _, txparams := range txParamss {
		totalFees = totalFees.Add(txparams.Fee.Decimal())
	}

	// Push note total fee must matches the total fees computed from all txparamss
	if !pn.GetFee().Decimal().Equal(totalFees) {
		return fmt.Errorf("push note fees does not match total txparamss fees")
	}

	// Check pushed references for consistency with their txparams
	for _, ref := range pn.GetPushedReferences() {
		txparams, ok := txParamss[ref.Name]
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

	if len(tx.PusherKeyID) == 0 {
		return util.FieldError("pusherKeyId", "pusher gpg key id is required")
	}
	if len(tx.PusherKeyID) != 20 {
		return util.FieldError("pusherKeyId", "pusher gpg key is not valid")
	}

	if tx.Timestamp == 0 {
		return util.FieldError("timestamp", "timestamp is required")
	}
	if tx.Timestamp > time.Now().Unix() {
		return util.FieldError("timestamp", "timestamp cannot be a future time")
	}

	if tx.AccountNonce == 0 {
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
func CheckPushNoteConsistency(tx *core.PushNote, logic core.Logic) error {

	// Ensure the repository exist
	repo := logic.RepoKeeper().GetRepo(tx.GetRepoName())
	if repo.IsNil() {
		msg := fmt.Sprintf("repository named '%s' is unknown", tx.GetRepoName())
		return util.FieldError("repoName", msg)
	}

	// Get gpg key of the pusher
	gpgKey := logic.GPGPubKeyKeeper().GetGPGPubKey(util.MustCreateGPGID(tx.PusherKeyID))
	if gpgKey.IsNil() {
		msg := fmt.Sprintf("pusher's public key id '%s' is unknown", tx.PusherKeyID)
		return util.FieldError("pusherKeyId", msg)
	}

	// Ensure the gpg key linked address matches the pusher address
	if gpgKey.Address != tx.PusherAddress {
		return util.FieldError("pusherAddr", "gpg key is not associated with the pusher address")
	}

	// Ensure next pusher account nonce matches the pushed note's account nonce
	pusherAcct := logic.AccountKeeper().GetAccount(tx.PusherAddress)
	if pusherAcct.IsNil() {
		return util.FieldError("pusherAddr", "pusher account not found")
	}
	nextNonce := pusherAcct.Nonce + 1
	if tx.AccountNonce != nextNonce {
		msg := fmt.Sprintf("wrong account nonce '%d', expecting '%d'", tx.AccountNonce, nextNonce)
		return util.FieldError("pusherAddr", msg)
	}

	// Check each references against the state version
	if err := checkPushedReference(
		tx.GetTargetRepo(),
		tx.GetPushedReferences(),
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
		tx.PusherAddress,
		"0",
		tx.GetFee(),
		tx.AccountNonce,
		uint64(bi.Height)); err != nil {
		return err
	}

	return nil
}

// checkPushNote performs validation checks on a push transaction
func checkPushNote(tx core.RepoPushNote, dht types.DHTNode,
	logic core.Logic) error {

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
