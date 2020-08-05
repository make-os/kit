package validation

import (
	"fmt"
	"time"

	"github.com/mr-tron/base58"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/themakeos/lobe/crypto"
	"github.com/themakeos/lobe/crypto/bls"
	"github.com/themakeos/lobe/params"
	plumbing2 "github.com/themakeos/lobe/remote/plumbing"
	"github.com/themakeos/lobe/remote/push/types"
	remotetypes "github.com/themakeos/lobe/remote/types"
	tickettypes "github.com/themakeos/lobe/ticket/types"
	"github.com/themakeos/lobe/types/constants"
	"github.com/themakeos/lobe/types/core"
	"github.com/themakeos/lobe/types/state"
	"github.com/themakeos/lobe/util"
	crypto2 "github.com/themakeos/lobe/util/crypto"
	"github.com/themakeos/lobe/util/identifier"
	"github.com/thoas/go-funk"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

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
		return fe(-1, "references", fmt.Sprintf("reference '%s' is unknown", name))
	}

	// If target repo is set and old hash is non-zero, we need to ensure
	// the current hash of the local version of the reference is the same as the old hash,
	// otherwise the pushed reference will not be compatible.
	if targetRepo != nil && !oldHashIsZero {
		localRef, err := targetRepo.Reference(plumbing.ReferenceName(name), false)
		if err != nil {
			return fe(-1, "references", fmt.Sprintf("reference '%s' does not exist locally", name))
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
	if nextNonce.UInt64() != ref.Nonce {
		msg := fmt.Sprintf("reference '%s' has nonce '%d', expecting '%d'", name, nonce, nextNonce)
		return fe(-1, "references", msg)
	}

	// For new merge request, ensure a proposal fee is provided
	isNewRef := !repoState.References.Has(ref.Name)
	if plumbing2.IsMergeRequestReference(ref.Name) && isNewRef {

		govCfg := repoState.Config.Gov

		// When repo does not require a proposal fee, it must not be provided.
		// Skip to end when repo does not require proposal fee
		repoPropFee := govCfg.PropFee
		if repoPropFee == 0 {
			if !refPropFee.Empty() {
				return fe(-1, "value", constants.ErrProposalFeeNotExpected.Error())
			}
			goto end
		}

		// When merge request proposal is exempted from paying proposal fee, skip to end
		if govCfg.NoPropFeeForMergeReq {
			goto end
		}

		if refPropFee.Empty() {
			refPropFee = "0"
		}

		// When repo requires a proposal fee and a deposit period is not allowed,
		// the full proposal fee must be provided.
		hasDepositPeriod := govCfg.PropFeeDepositDur > 0
		if !hasDepositPeriod && refPropFee.Decimal().LessThan(decimal.NewFromFloat(repoPropFee)) {
			return fe(-1, "value", constants.ErrFullProposalFeeRequired.Error())
		}
	}

end:
	return nil
}

// GetTxDetailsFromNote creates a slice of TxDetail objects from a push note.
// Limit to references specified in targetRefs
func GetTxDetailsFromNote(note types.PushNote, targetRefs ...string) (details []*remotetypes.TxDetail) {
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

// CheckPushNoteSanity performs syntactic checks on the fields of a push transaction
func CheckPushNoteSanity(note types.PushNote) error {

	if note.GetRepoName() == "" {
		return util.FieldError("repo", "repo name is required")
	}
	if identifier.IsValidResourceName(note.GetRepoName()) != nil {
		return util.FieldError("repo", "repo name is not valid")
	}

	if note.GetNamespace() != "" && identifier.IsValidResourceName(note.GetNamespace()) != nil {
		return util.FieldError("namespace", "namespace is not valid")
	}

	for i, ref := range note.GetPushedReferences() {
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

	if len(note.GetPusherKeyID()) == 0 {
		return util.FieldError("pusherKeyId", "push key id is required")
	}
	if len(note.GetPusherKeyID()) != 20 {
		return util.FieldError("pusherKeyId", "push key id is not valid")
	}

	if note.GetTimestamp() == 0 {
		return util.FieldError("timestamp", "timestamp is required")
	}
	if note.GetTimestamp() > time.Now().Unix() {
		return util.FieldError("timestamp", "timestamp cannot be a future time")
	}

	if note.GetPusherAccountNonce() == 0 {
		return util.FieldError("accountNonce", "account nonce must be greater than zero")
	}

	if note.GetCreatorPubKey().IsEmpty() {
		return util.FieldError("nodePubKey", "push node public key is required")
	}

	pk, err := crypto.PubKeyFromBytes(note.GetCreatorPubKey().Bytes())
	if err != nil {
		return util.FieldError("nodePubKey", "push node public key is not valid")
	}

	if len(note.GetNodeSignature()) == 0 {
		return util.FieldError("nodeSig", "push node signature is required")
	}

	if ok, err := pk.Verify(note.BytesNoSig(), note.GetNodeSignature()); err != nil || !ok {
		return util.FieldError("nodeSig", "failed to verify signature")
	}

	return nil
}

// CheckPushNoteConsistency performs consistency checks against the state of the
// repository as seen by the node. If the target repo object is not set in tx,
// local reference hash comparision is not performed.
func CheckPushNoteConsistency(note types.PushNote, logic core.Logic) error {

	// Ensure the repository exist
	repo := logic.RepoKeeper().Get(note.GetRepoName())
	if repo.IsNil() {
		msg := fmt.Sprintf("repository named '%s' is unknown", note.GetRepoName())
		return util.FieldError("repo", msg)
	}

	// If namespace is provide, ensure it exists
	if note.GetNamespace() != "" {
		ns := logic.NamespaceKeeper().Get(crypto2.MakeNamespaceHash(note.GetNamespace()))
		if ns.IsNil() {
			return util.FieldError("namespace", fmt.Sprintf("namespace '%s' is unknown", note.GetNamespace()))
		}
		if !funk.ContainsString(funk.Values(ns.Domains).([]string), identifier.NativeNamespaceRepo+note.GetRepoName()) {
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
	} else if note.GetPusherAccountNonce() != pusherAcct.Nonce.UInt64()+1 {
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

// NoteChecker describes a function for checking a push note
type NoteChecker func(tx types.PushNote, logic core.Logic) error

// CheckPushNote performs validation checks on a push transaction
func CheckPushNote(note types.PushNote, logic core.Logic) error {
	if err := CheckPushNoteSanity(note); err != nil {
		return err
	}
	if err := CheckPushNoteConsistency(note, logic); err != nil {
		return err
	}
	return nil
}

// CheckEndorsementConsistencyUsingHost performs consistency checks on the given
// push endorsement object against the current state of the network.
// EXPECT: Sanity check to have been performed using CheckEndorsementSanity
func CheckEndorsementConsistencyUsingHost(
	logic core.Logic,
	hosts tickettypes.SelectedTickets,
	end *types.PushEndorsement,
	noBLSSigCheck bool,
	index int) error {

	// Check if the sender is one of the top hosts.
	// Ensure that the signers of the Endorsement are part of the hosts
	selected := hosts.Get(end.EndorserPubKey)
	if selected == nil {
		return fe(index, "endorsements.senderPubKey",
			"sender public key does not belong to an active host")
	}

	// Ensure the BLS signature can be verified using the BLS public key of the selected ticket
	if !noBLSSigCheck {
		blsPubKey, err := bls.BytesToPublicKey(selected.Ticket.BLSPubKey)
		if err != nil {
			return errors.Wrap(err, "failed to decode bls public key of endorser")
		}
		if err = blsPubKey.Verify(end.SigBLS[:], end.BytesForBLSSig()); err != nil {
			return fe(index, "endorsements.sig", "signature could not be verified")
		}
	}

	return nil
}

// CheckEndorsementConsistency performs consistency checks on the given Endorsement object
// against the current state of the network.
// EXPECT: Sanity check to have been performed using CheckEndorsementSanity
func CheckEndorsementConsistency(end *types.PushEndorsement, logic core.Logic, noBLSSigCheck bool, index int) error {
	hosts, err := logic.GetTicketManager().GetTopHosts(params.NumTopHostsLimit)
	if err != nil {
		return errors.Wrap(err, "failed to get top hosts")
	}
	return CheckEndorsementConsistencyUsingHost(logic, hosts, end, noBLSSigCheck, index)
}

// CheckEndorsementSanity performs sanity checks on the given Endorsement object.
// fromPushTx indicates that the endorsement was retrieved from a push transaction.
// noBLSSigRequiredCheck prevents BLS signature requirement check.
func CheckEndorsementSanity(e *types.PushEndorsement, fromPushTx bool, index int) error {

	// Push note id must be set
	if !fromPushTx && len(e.NoteID) == 0 {
		return fe(index, "endorsements.noteID", "push note ID is required")
	}

	// Sender public key must be set
	if e.EndorserPubKey.IsEmpty() {
		return fe(index, "endorsements.pubKey", "endorser's public key is required")
	}

	// For endorsement at index 0, at least, one reference is required.
	// Endorsements at > 0 index can have no references.
	// For an endorsement not from a push transaction, ensure it has at least one reference.
	if !fromPushTx && len(e.References) == 0 {
		return fe(index, "endorsements.refs", "at least one reference is required")
	}

	if fromPushTx {
		// For an endorsement that is the first in a push transaction,
		// ensure it has at least one reference.
		if index == 0 && len(e.References) == 0 {
			return fe(index, "endorsements.refs", "at least one reference is required")
		}

		// Ensure BLS signature is not set.
		if len(e.SigBLS) > 0 {
			return fe(index, "endorsements.sigBLS", "BLS signature is not expected")
		}

		// Ensure NoteID is not set
		if len(e.NoteID) > 0 {
			return fe(index, "endorsements.noteID", "Note ID is not expected")
		}

		// At index > 0, ensure no reference is provided
		if index > 0 && len(e.References) > 0 {
			return fe(index, "endorsements.refs", "references not expected")
		}
	}

	// Endorser's BLS signature is required
	if !fromPushTx && e.SigBLS == nil {
		return fe(index, "endorsements.sigBLS", "endorser's BLS signature is required")
	}

	return nil
}

// EndorsementChecker describes a function for validating a push endorsement
type EndorsementChecker func(end *types.PushEndorsement, logic core.Logic, index int) error

// CheckEndorsement performs sanity and state consistency checks on the given Endorsement object
func CheckEndorsement(end *types.PushEndorsement, logic core.Logic, index int) error {
	if err := CheckEndorsementSanity(end, false, index); err != nil {
		return err
	}
	if err := CheckEndorsementConsistency(end, logic, false, index); err != nil {
		return err
	}
	return nil
}
