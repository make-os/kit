package validation

import (
	"context"
	"fmt"
	"time"

	"github.com/mr-tron/base58"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/thoas/go-funk"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/crypto/bls"
	dhttypes "gitlab.com/makeos/mosdef/dht/types"
	"gitlab.com/makeos/mosdef/params"
	plumbing2 "gitlab.com/makeos/mosdef/remote/plumbing"
	"gitlab.com/makeos/mosdef/remote/push/types"
	remotetypes "gitlab.com/makeos/mosdef/remote/types"
	tickettypes "gitlab.com/makeos/mosdef/ticket/types"
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/util"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

// CheckPushNoteSyntax performs syntactic checks on the fields of a push transaction
func CheckPushNoteSyntax(tx types.PushNotice) error {

	if tx.GetRepoName() == "" {
		return util.FieldError("repo", "repo name is required")
	}
	if util.IsValidName(tx.GetRepoName()) != nil {
		return util.FieldError("repo", "repo name is not valid")
	}

	if tx.GetNamespace() != "" && util.IsValidName(tx.GetNamespace()) != nil {
		return util.FieldError("namespace", "namespace is not valid")
	}

	for i, ref := range tx.GetPushedReferences() {
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
		} else if !ref.Fee.IsNumeric() {
			return fe(i, "fee", "fee must be numeric")
		}

		if ref.Value != "" && !ref.Value.IsNumeric() {
			return fe(i, "value", "value must be numeric")
		}

		if ref.MergeProposalID != "" {
			return CheckMergeProposalID(ref.MergeProposalID, i)
		}

		if len(ref.PushSig) == 0 {
			return fe(i, "pushSig", "signature is required")
		}
	}

	if len(tx.GetPusherKeyID()) == 0 {
		return util.FieldError("pusherKeyId", "push key id is required")
	}
	if len(tx.GetPusherKeyID()) != 20 {
		return util.FieldError("pusherKeyId", "push key id is not valid")
	}

	if tx.GetTimestamp() == 0 {
		return util.FieldError("timestamp", "timestamp is required")
	}
	if tx.GetTimestamp() > time.Now().Unix() {
		return util.FieldError("timestamp", "timestamp cannot be a future time")
	}

	if tx.GetPusherAccountNonce() == 0 {
		return util.FieldError("accountNonce", "account nonce must be greater than zero")
	}

	if tx.GetNodePubKey().IsEmpty() {
		return util.FieldError("nodePubKey", "push node public key is required")
	}

	pk, err := crypto.PubKeyFromBytes(tx.GetNodePubKey().Bytes())
	if err != nil {
		return util.FieldError("nodePubKey", "push node public key is not valid")
	}

	if len(tx.GetNodeSignature()) == 0 {
		return util.FieldError("nodeSig", "push node signature is required")
	}

	if ok, err := pk.Verify(tx.BytesNoSig(), tx.GetNodeSignature()); err != nil || !ok {
		return util.FieldError("nodeSig", "failed to verify signature")
	}

	return nil
}

// CheckPushedReferenceConsistency validates pushed references
func CheckPushedReferenceConsistency(
	targetRepo remotetypes.LocalRepo,
	ref *types.PushedReference,
	repoState *state.Repository) error {

	name, nonce := ref.Name, ref.Nonce
	oldHashIsZero := plumbing.NewHash(ref.OldHash).IsZero()

	// We need to check if the reference exists in the repo.
	// Ignore references whose old hash is a 0-hash, these are new
	// references and as such we don't expect to find it in the repo.
	if !oldHashIsZero && !repoState.References.Has(name) {
		msg := fmt.Sprintf("reference '%s' is unknown", name)
		return fe(-1, "references", msg)
	}

	// If target repo is set and old hash is non-zero, we need to ensure
	// the current hash of the local version of the reference is the same as the old hash,
	// otherwise the pushed reference will not be compatible.
	if targetRepo != nil && !oldHashIsZero {
		localRef, err := targetRepo.Reference(plumbing.ReferenceName(name), false)
		if err != nil {
			msg := fmt.Sprintf("reference '%s' does not exist locally", name)
			return fe(-1, "references", msg)
		}
		if ref.OldHash != localRef.Hash().String() {
			msg := fmt.Sprintf("reference '%s' old hash does not match its local version", name)
			return fe(-1, "references", msg)
		}
	}

	// We need to check that the nonce is the expected next nonce of the
	// reference, otherwise we return an error.
	refInfo := repoState.References.Get(name)
	nextNonce := refInfo.Nonce + 1
	refPropFee := ref.Value
	if nextNonce != ref.Nonce {
		msg := fmt.Sprintf("reference '%s' has nonce '%d', expecting '%d'", name, nonce, nextNonce)
		return fe(-1, "references", msg)
	}

	// For new merge request, ensure a proposal fee is provided
	isNewRef := !repoState.References.Has(ref.Name)
	if plumbing2.IsMergeRequestReference(ref.Name) && isNewRef {

		govCfg := repoState.Config.Governance

		// When repo does not require a proposal fee, it must not be provided.
		// Skip to end when repo does not require proposal fee
		repoPropFee := govCfg.ProposalFee
		if repoPropFee == 0 {
			if !refPropFee.Empty() {
				return fe(-1, "value", constants.ErrProposalFeeNotExpected.Error())
			}
			goto end
		}

		// When merge request proposal is exempted from paying proposal fee, skip to end
		if govCfg.NoProposalFeeForMergeReq {
			goto end
		}

		if refPropFee.Empty() {
			refPropFee = "0"
		}

		// When repo requires a proposal fee and a deposit period is not allowed,
		// the full proposal fee must be provided.
		hasDepositPeriod := govCfg.ProposalFeeDepositDur > 0
		if !hasDepositPeriod && refPropFee.Decimal().LessThan(decimal.NewFromFloat(repoPropFee)) {
			return fe(-1, "value", constants.ErrFullProposalFeeRequired.Error())
		}
	}

end:
	return nil
}

// GetTxDetailsFromNote creates a slice of TxDetail objects from a push note.
// Limit to references specified in targetRefs
func GetTxDetailsFromNote(note types.PushNotice, targetRefs ...string) (details []*remotetypes.TxDetail) {
	for _, ref := range note.GetPushedReferences() {
		if len(targetRefs) > 0 && !funk.ContainsString(targetRefs, ref.Name) {
			continue
		}
		detail := &remotetypes.TxDetail{
			RepoName:        note.GetRepoName(),
			RepoNamespace:   note.GetNamespace(),
			Reference:       ref.Name,
			Fee:             ref.Fee,
			Value:           ref.Value,
			Nonce:           note.GetPusherAccountNonce(),
			PushKeyID:       crypto.BytesToPushKeyID(note.GetPusherKeyID()),
			Signature:       base58.Encode(ref.PushSig),
			MergeProposalID: ref.MergeProposalID,
		}
		if plumbing2.IsNote(detail.Reference) {
			detail.Head = ref.NewHash
		}
		details = append(details, detail)
	}
	return
}

// CheckPushNoteConsistency performs consistency checks against the state of the
// repository as seen by the node. If the target repo object is not set in tx,
// local reference hash comparision is not performed.
func CheckPushNoteConsistency(note types.PushNotice, logic core.Logic) error {

	// Ensure the repository exist
	repo := logic.RepoKeeper().Get(note.GetRepoName())
	if repo.IsNil() {
		msg := fmt.Sprintf("repository named '%s' is unknown", note.GetRepoName())
		return util.FieldError("repo", msg)
	}

	// If namespace is provide, ensure it exists
	if note.GetNamespace() != "" {
		ns := logic.NamespaceKeeper().Get(util.HashNamespace(note.GetNamespace()))
		if ns.IsNil() {
			return util.FieldError("namespace", fmt.Sprintf("namespace '%s' is unknown", note.GetNamespace()))
		}
		if !funk.ContainsString(funk.Values(ns.Domains).([]string), util.RepoIDPrefix+note.GetRepoName()) {
			return util.FieldError("repo", fmt.Sprintf("repo not a target in namespace '%s'", note.GetNamespace()))
		}
	}

	// Get push key of the pusher
	pushKey := logic.PushKeyKeeper().Get(crypto.BytesToPushKeyID(note.GetPusherKeyID()))
	if pushKey.IsNil() {
		msg := fmt.Sprintf("pusher's public key id '%s' is unknown", note.GetPusherKeyID())
		return util.FieldError("pusherKeyId", msg)
	}

	// Ensure the push key linked address matches the pusher address
	if pushKey.Address != note.GetPusherAddress() {
		return util.FieldError("pusherAddr", "push key does not belong to pusher")
	}

	// Ensure next pusher account nonce matches the note's account nonce
	pusherAcct := logic.AccountKeeper().Get(note.GetPusherAddress())
	if pusherAcct.IsNil() {
		return util.FieldError("pusherAddr", "pusher account not found")
	} else if note.GetPusherAccountNonce() != pusherAcct.Nonce+1 {
		msg := fmt.Sprintf("wrong account nonce '%d', expecting '%d'",
			note.GetPusherAccountNonce(), pusherAcct.Nonce+1)
		return util.FieldError("accountNonce", msg)
	}

	// Check each references against the state
	for i, ref := range note.GetPushedReferences() {
		if err := CheckPushedReferenceConsistency(note.GetTargetRepo(), ref, repo); err != nil {
			return err
		}

		// Verify signature
		txDetail := GetTxDetailsFromNote(note, ref.Name)[0]
		pushPubKey := crypto.MustPubKeyFromBytes(pushKey.PubKey.Bytes())
		if ok, err := pushPubKey.Verify(txDetail.BytesNoSig(), ref.PushSig); err != nil || !ok {
			msg := fmt.Sprintf("reference (%s) signature is not valid", ref.Name)
			return fe(i, "references", msg)
		}
	}

	// Check whether the pusher can pay the specified transaction fee
	bi, err := logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		return errors.Wrap(err, "failed to fetch current block info")
	}
	if err = logic.DrySend(note.GetPusherAddress(), note.GetValue(), note.GetFee(),
		note.GetPusherAccountNonce(), uint64(bi.Height)); err != nil {
		return err
	}

	return nil
}

// PushNoteCheckFunc describes a function for checking a push note
type PushNoteCheckFunc func(tx types.PushNotice, dht dhttypes.DHTNode, logic core.Logic) error

// CheckPushNote performs validation checks on a push transaction
func CheckPushNote(tx types.PushNotice, dht dhttypes.DHTNode, logic core.Logic) error {

	if err := CheckPushNoteSyntax(tx.(*types.PushNote)); err != nil {
		return err
	}

	if err := CheckPushNoteConsistency(tx.(*types.PushNote), logic); err != nil {
		return err
	}

	err := FetchAndCheckReferenceObjects(tx, dht)
	if err != nil {
		return err
	}

	return nil
}

// CheckPushEndorsement performs sanity checks on the given PushEndorsement object
func CheckPushEndorsement(pushEnd *types.PushEndorsement, index int) error {

	// Push note id must be set
	if pushEnd.NoteID.IsEmpty() {
		return fe(index, "endorsements.pushNoteID", "push note id is required")
	}

	// Sender public key must be set
	if pushEnd.EndorserPubKey.IsEmpty() {
		return fe(index, "endorsements.senderPubKey", "sender public key is required")
	}

	return nil
}

// CheckPushEndConsistencyUsingHost performs consistency checks on the given PushEndorsement object
// against the current state of the network.
// EXPECT: Sanity check to have been performed using CheckPushEndorsement
func CheckPushEndConsistencyUsingHost(
	hosts tickettypes.SelectedTickets,
	pushEnd *types.PushEndorsement,
	logic core.Logic,
	noSigCheck bool,
	index int) error {

	// Check if the sender is one of the top hosts.
	// Ensure that the signers of the PushEndorsement are part of the hosts
	signerSelectedTicket := hosts.Get(pushEnd.EndorserPubKey)
	if signerSelectedTicket == nil {
		return fe(index, "endorsements.senderPubKey",
			"sender public key does not belong to an active host")
	}

	// Ensure the signature can be verified using the BLS public key of the selected ticket
	if !noSigCheck {
		blsPubKey, err := bls.BytesToPublicKey(signerSelectedTicket.Ticket.BLSPubKey)
		if err != nil {
			return errors.Wrap(err, "failed to decode bls public key of endorser")
		}
		if err = blsPubKey.Verify(pushEnd.Sig.Bytes(), pushEnd.BytesNoSigAndSenderPubKey()); err != nil {
			return fe(index, "endorsements.sig", "signature could not be verified")
		}
	}

	return nil
}

// CheckPushEndConsistency performs consistency checks on the given PushEndorsement object
// against the current state of the network.
// EXPECT: Sanity check to have been performed using CheckPushEndorsement
func CheckPushEndConsistency(pushEnd *types.PushEndorsement, logic core.Logic, noSigCheck bool, index int) error {
	hosts, err := logic.GetTicketManager().GetTopHosts(params.NumTopHostsLimit)
	if err != nil {
		return errors.Wrap(err, "failed to get top hosts")
	}
	return CheckPushEndConsistencyUsingHost(hosts, pushEnd, logic, noSigCheck, index)
}

// CheckPushEnd performs sanity and state consistency checks on the given PushEndorsement object
func CheckPushEnd(pushEnd *types.PushEndorsement, logic core.Logic, index int) error {
	if err := CheckPushEndorsement(pushEnd, index); err != nil {
		return err
	}
	if err := CheckPushEndConsistency(pushEnd, logic, false, index); err != nil {
		return err
	}
	return nil
}

// FetchAndCheckReferenceObjects attempts to fetch and store new objects
// introduced by the pushed references. After fetching it performs checks
// on the objects
func FetchAndCheckReferenceObjects(tx types.PushNotice, dhtnode dhttypes.DHTNode) error {
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
		dhtKey := plumbing2.MakeRepoObjectDHTKey(tx.GetRepoName(), objHash)
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
