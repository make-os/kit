package repo

import (
	"context"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/mr-tron/base58"
	dhttypes "gitlab.com/makeos/mosdef/dht/types"
	tickettypes "gitlab.com/makeos/mosdef/ticket/types"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"

	gv "github.com/asaskevich/govalidator"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/crypto/bls"
	"gitlab.com/makeos/mosdef/params"
	"gitlab.com/makeos/mosdef/util"

	"github.com/pkg/errors"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

var feI = util.FieldErrorWithIndex
var ErrSigHeaderAndReqParamsMismatch = fmt.Errorf("request transaction info and signature " +
	"transaction info did not match")

type changeValidator func(
	repo core.BareRepo,
	change *core.ItemChange,
	txDetail *types.TxDetail,
	getPushKey core.PushKeyGetter) error

// validateChange validates a change to a repository
// repo: The target repository
// change: The item that changed the repository
// txDetail: The pusher transaction detail
// getPushKey: Getter function for reading push key public key
func validateChange(
	repo core.BareRepo,
	change *core.ItemChange,
	txDetail *types.TxDetail,
	getPushKey core.PushKeyGetter) error {

	// Handle branch validation
	if isBranch(change.Item.GetName()) {
		commit, err := repo.CommitObject(plumbing.NewHash(change.Item.GetData()))
		if err != nil {
			return errors.Wrap(err, "unable to get commit object")
		}
		return checkCommit(commit, txDetail, getPushKey)
	}

	// Handle tag validation
	if isTag(change.Item.GetName()) {
		tagRef, err := repo.Tag(strings.ReplaceAll(change.Item.GetName(), "refs/tags/", ""))
		if err != nil {
			return errors.Wrap(err, "unable to get tag object")
		}

		// Get the tag object (for annotated tags)
		tagObj, err := repo.TagObject(tagRef.Hash())
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
			return checkCommit(commit, txDetail, getPushKey)
		}

		// At this point, the tag is an annotated tag.
		// We have to ensure the annotated tag object is signed.
		return checkAnnotatedTag(tagObj, txDetail, getPushKey)
	}

	// Handle note validation
	if isNote(change.Item.GetName()) {
		return checkNote(repo, txDetail)
	}

	return fmt.Errorf("unrecognised change item")
}

// checkNote validates a note.
// repo: The repo where the tag exists in.
// txDetail: The pusher transaction detail
func checkNote(
	repo core.BareRepo,
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

// checkAnnotatedTag validates an annotated tag.
// tag: The target annotated tag
// txDetail: The pusher transaction detail
// getPushKey: Getter function for reading push key public key
func checkAnnotatedTag(
	tag *object.Tag,
	txDetail *types.TxDetail,
	getPushKey core.PushKeyGetter) error {

	if tag.PGPSignature == "" {
		msg := "tag (%s) is unsigned. Sign the tag with your push key"
		return errors.Errorf(msg, tag.Hash.String())
	}

	pubKey, err := getPushKey(txDetail.PushKeyID)
	if err != nil {
		return errors.Wrapf(err, "failed to get pusher key(%s) to verify commit (%s)",
			txDetail.PushKeyID, tag.Hash.String())
	}

	tagTxDetail, err := verifyCommitOrTagSignature(tag, pubKey)
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

	return checkCommit(commit, txDetail, func(string) (key crypto.PublicKey, err error) {
		return pubKey, nil
	})
}

// getCommitOrTagSigMsg returns the message that is signed to create a commit or tag signature
func getCommitOrTagSigMsg(obj object.Object) string {
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
func verifyCommitOrTagSignature(obj object.Object, pubKey crypto.PublicKey) (*types.TxDetail, error) {
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

// checkCommit validates a commit
// commit: The target commit object
// txDetail: The push transaction detail
// getPushKey: Getter function for fetching push public key
func checkCommit(
	commit *object.Commit,
	txDetail *types.TxDetail,
	getPushKey core.PushKeyGetter) error {

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
	commitTxDetail, err := verifyCommitOrTagSignature(commit, pubKey)
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

// CheckPushNoteSyntax performs syntactic checks on the fields of a push transaction
func CheckPushNoteSyntax(tx *core.PushNote) error {

	if tx.RepoName == "" {
		return util.FieldError("repoName", "repo name is required")
	}
	if util.IsValidIdentifierName(tx.RepoName) != nil {
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

		if ref.Fee == "" {
			return fe(i, "fee", "fee is required")
		}
		if !gv.IsFloat(ref.Fee.String()) {
			return fe(i, "fee", "fee must be numeric")
		}

		if ref.MergeProposalID != "" {
			return checkMergeProposalID(ref.MergeProposalID, i)
		}

		if len(ref.PushSig) == 0 {
			return fe(i, "pushSig", "signature is required")
		}
	}

	if len(tx.PushKeyID) == 0 {
		return util.FieldError("pusherKeyId", "push key id is required")
	}
	if len(tx.PushKeyID) != 20 {
		return util.FieldError("pusherKeyId", "push key id is not valid")
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

// checkPushedReferenceConsistency validates pushed references
func checkPushedReferenceConsistency(
	targetRepo core.BareRepo,
	ref *core.PushedReference,
	repo *state.Repository) error {

	name, nonce := ref.Name, ref.Nonce
	oldHashIsZero := plumbing.NewHash(ref.OldHash).IsZero()

	// We need to check if the reference exists in the repo.
	// Ignore references whose old hash is a 0-hash, these are new
	// references and as such we don't expect to find it in the repo.
	if !oldHashIsZero && !repo.References.Has(name) {
		msg := fmt.Sprintf("reference '%s' is unknown", name)
		return util.FieldError("references", msg)
	}

	// If target repo is set and old hash is non-zero, we need to ensure
	// the current hash of the local version of the reference is the same as the old hash,
	// otherwise the pushed reference will not be compatible.
	if targetRepo != nil && !oldHashIsZero {
		localRef, err := targetRepo.Reference(plumbing.ReferenceName(name), false)
		if err != nil {
			msg := fmt.Sprintf("reference '%s' does not exist locally", name)
			return util.FieldError("references", msg)
		}
		if ref.OldHash != localRef.Hash().String() {
			msg := fmt.Sprintf("reference '%s' old hash does not match its local version", name)
			return util.FieldError("references", msg)
		}
	}

	// We need to check that the nonce is the expected next nonce of the
	// reference, otherwise we return an error.
	refInfo := repo.References.Get(name)
	nextNonce := refInfo.Nonce + 1
	if nextNonce != ref.Nonce {
		msg := fmt.Sprintf("reference '%s' has nonce '%d', expecting '%d'", name, nonce, nextNonce)
		return util.FieldError("references", msg)
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

	// Get push key of the pusher
	pushKey := logic.PushKeyKeeper().Get(crypto.BytesToPushKeyID(note.PushKeyID))
	if pushKey.IsNil() {
		msg := fmt.Sprintf("pusher's public key id '%s' is unknown", note.PushKeyID)
		return util.FieldError("pusherKeyId", msg)
	}

	// Ensure the push key linked address matches the pusher address
	if pushKey.Address != note.PusherAddress {
		return util.FieldError("pusherAddr", "push key does not belong to pusher")
	}

	// Ensure next pusher account nonce matches the note's account nonce
	pusherAcct := logic.AccountKeeper().Get(note.PusherAddress)
	if pusherAcct.IsNil() {
		return util.FieldError("pusherAddr", "pusher account not found")
	} else if note.PusherAcctNonce != pusherAcct.Nonce+1 {
		msg := fmt.Sprintf("wrong account nonce '%d', expecting '%d'",
			note.PusherAcctNonce, pusherAcct.Nonce+1)
		return util.FieldError("accountNonce", msg)
	}

	// Check each references against the state
	for i, ref := range note.GetPushedReferences() {
		if err := checkPushedReferenceConsistency(note.GetTargetRepo(), ref, repo); err != nil {
			return err
		}

		// Verify signature
		txDetail := getTxDetailsFromNote(note, ref.Name)[0]
		pushPubKey := crypto.MustPubKeyFromBytes(pushKey.PubKey.Bytes())
		if ok, err := pushPubKey.Verify(txDetail.BytesNoSig(), ref.PushSig); err != nil || !ok {
			msg := fmt.Sprintf("reference (%s) signature is not valid", ref.Name)
			return util.FieldErrorWithIndex(i, "references", msg)
		}
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

// pushNoteChecker describes a function for checking a push note
type pushNoteChecker func(tx core.RepoPushNote, dht dhttypes.DHTNode, logic core.Logic) error

// checkPushNote performs validation checks on a push transaction
func checkPushNote(tx core.RepoPushNote, dht dhttypes.DHTNode, logic core.Logic) error {

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
func fetchAndCheckReferenceObjects(tx core.RepoPushNote, dhtnode dhttypes.DHTNode) error {
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

		// Since the object doesn't exist locally, read the object from the DHTNode
		dhtKey := MakeRepoObjectDHTKey(tx.GetRepoName(), objHash)
		ctx, cn := context.WithTimeout(context.Background(), 60*time.Second)
		objValue, err := dhtnode.GetObject(ctx, &dhttypes.DHTObjectQuery{
			Module:    core.RepoObjectModule,
			ObjectKey: []byte(dhtKey),
		})
		if err != nil {
			cn()
			msg := fmt.Sprintf("failed to fetch object '%s'", objHash)
			return errors.Wrap(err, msg)
		}
		cn()

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

// checkTxDetail performs sanity and consistency checks on a transaction's parameters.
// When authScope is true, only fields necessary for authentication are validated.
func checkTxDetail(params *types.TxDetail, keepers core.Keepers, authScope bool) error {
	if err := checkTxDetailSanity(params, authScope); err != nil {
		return err
	}
	return checkTxDetailConsistency(params, keepers, authScope)
}

// checkTxDetailSanity performs sanity checks on a transaction's parameters.
// When authScope is true, only fields necessary for authentication are validated.
func checkTxDetailSanity(params *types.TxDetail, authScope bool) error {

	// Push key is required and must be valid
	if params.PushKeyID == "" {
		return util.FieldError("pkID", "push key id is required")
	} else if !util.IsValidPushKeyID(params.PushKeyID) {
		return util.FieldError("pkID", "push key id is not valid")
	}

	// Nonce must be set
	if !authScope && params.Nonce == 0 {
		return util.FieldError("nonce", "nonce is required")
	}

	// Fee must be set
	if !authScope && params.Fee.String() == "" {
		return util.FieldError("fee", "fee is required")
	}

	// Fee must be numeric
	if !authScope && !gv.IsFloat(params.Fee.String()) {
		return util.FieldError("fee", "fee must be numeric")
	}

	// Signature format must be valid
	if _, err := base58.Decode(params.Signature); err != nil {
		return util.FieldError("sig", "signature format is not valid")
	}

	// Merge proposal, if set, must be numeric and have 8 bytes length max.
	if !authScope && params.MergeProposalID != "" {
		return checkMergeProposalID(params.MergeProposalID, -1)
	}

	return nil
}

// checkMergeProposalID performs sanity checks on merge proposal ID
func checkMergeProposalID(id string, index int) error {
	if !gv.IsNumeric(id) {
		return util.FieldErrorWithIndex(index, "mergeID", "merge proposal id must be numeric")
	}
	if len(id) > 8 {
		return util.FieldErrorWithIndex(index, "mergeID", "merge proposal id exceeded 8 bytes limit")
	}
	return nil
}

// checkTxDetailConsistency performs consistency checks on a transaction's parameters.
// When authScope is true, only fields necessary for authentication are validated.
func checkTxDetailConsistency(params *types.TxDetail, keepers core.Keepers, authScope bool) error {

	// Pusher key must exist
	pushKey := keepers.PushKeyKeeper().Get(params.PushKeyID)
	if pushKey.IsNil() {
		return util.FieldError("pkID", "push key not found")
	}

	// Ensure the nonce is a future nonce (> current nonce of pusher's account)
	if !authScope {
		keyOwner := keepers.AccountKeeper().Get(pushKey.Address)
		if params.Nonce <= keyOwner.Nonce {
			return util.FieldError("nonce", fmt.Sprintf("nonce (%d) must be "+
				"greater than current key owner nonce (%d)", params.Nonce, keyOwner.Nonce))
		}
	}

	// Use the key to verify the tx params signature
	pubKey, _ := crypto.PubKeyFromBytes(pushKey.PubKey.Bytes())
	if ok, err := pubKey.Verify(params.BytesNoSig(), params.MustSignatureAsBytes()); err != nil || !ok {
		return util.FieldError("sig", "signature is not valid")
	}

	return nil
}

type mergeComplianceChecker func(
	repo core.BareRepo,
	change *core.ItemChange,
	oldRef core.Item,
	mergeProposalID,
	pushKeyID string,
	keepers core.Keepers) error

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
		return fmt.Errorf("merge error: signer did not create the proposal (%s)", mergeProposalID)
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
