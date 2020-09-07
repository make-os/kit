package validation

import (
	"bytes"
	"fmt"

	"github.com/make-os/lobe/crypto"
	"github.com/make-os/lobe/crypto/bls"
	"github.com/make-os/lobe/params"
	"github.com/make-os/lobe/remote/plumbing"
	"github.com/make-os/lobe/remote/validation"
	"github.com/make-os/lobe/types/constants"
	"github.com/make-os/lobe/types/core"
	"github.com/make-os/lobe/types/state"
	"github.com/make-os/lobe/types/txns"
	crypto2 "github.com/make-os/lobe/util/crypto"
	"github.com/make-os/lobe/util/identifier"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	plumbing2 "gopkg.in/src-d/go-git.v4/plumbing"
)

// CheckTxCoinTransferConsistency performs consistency checks on TxCoinTransfer
func CheckTxCoinTransferConsistency(
	tx *txns.TxCoinTransfer,
	index int,
	logic core.Logic) error {
	recipient := tx.To
check:

	// If recipient address is a native namespace repo address, ensure repo exist
	if recipient.IsNativeRepoAddress() {
		targetRepo := logic.RepoKeeper().Get(recipient.String()[2:])
		if targetRepo.IsNil() {
			return feI(index, "to", "recipient repo not found")
		}
	}

	// If the recipient address is a user namespace address, get the target and if the
	// target is a repository address, check that the repo exist.
	if recipient.IsUserNamespace() {
		prefixedTarget, err := logic.NamespaceKeeper().GetTarget(recipient.String())
		if err != nil {
			return feI(index, "to", err.Error())
		}
		recipient = identifier.Address(prefixedTarget)
		goto check
	}

	pubKey, _ := crypto.PubKeyFromBytes(tx.GetSenderPubKey().Bytes())
	if err := logic.DrySend(pubKey, tx.Value, tx.Fee, tx.GetNonce(), 0); err != nil {
		return err
	}

	return nil
}

// CheckTxTicketPurchaseConsistency performs consistency checks on TxTicketPurchase
func CheckTxTicketPurchaseConsistency(
	tx *txns.TxTicketPurchase,
	index int,
	logic core.Logic) error {

	// When delegate is set, the delegate must have an active,
	// non-delegated ticket
	if !tx.Delegate.IsEmpty() {
		r, err := logic.GetTicketManager().GetNonDelegatedTickets(tx.Delegate.ToBytes32(), tx.Type)
		if err != nil {
			return errors.Wrap(err, "failed to get active delegate tickets")
		} else if len(r) == 0 {
			return feI(index, "delegate", "specified delegate is not active")
		}
	}

	// For non-delegated validator ticket transaction, the value
	// must not be lesser than the current price per ticket
	if tx.Type == txns.TxTypeValidatorTicket && tx.Delegate.IsEmpty() {
		curTicketPrice := params.MinValidatorsTicketPrice
		if tx.Value.Decimal().LessThan(decimal.NewFromFloat(curTicketPrice)) {
			return feI(index, "value", fmt.Sprintf("value is lower than the"+
				" minimum ticket price (%f)", curTicketPrice))
		}
	}

	pubKey, _ := crypto.PubKeyFromBytes(tx.GetSenderPubKey().Bytes())
	if err := logic.DrySend(pubKey, tx.Value, tx.Fee, tx.GetNonce(), 0); err != nil {
		return err
	}

	return nil
}

// CheckTxUnbondTicketConsistency performs consistency checks on TxTicketUnbond
func CheckTxUnbondTicketConsistency(
	tx *txns.TxTicketUnbond,
	index int,
	logic core.Logic) error {

	bi, err := logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		return errors.Wrap(err, "failed to fetch current block info")
	}

	// The ticket must exist
	ticket := logic.GetTicketManager().GetByHash(tx.TicketHash)
	if ticket == nil {
		return feI(index, "hash", "ticket not found")
	}

	// Ensure the tx creator is the owner of the ticket.
	// For delegated ticket, compare the delegator address with the sender address
	authErr := feI(index, "hash", "sender not authorized to unbond this ticket")
	if ticket.Delegator == "" {
		if tx.SenderPubKey.ToBytes32() != ticket.ProposerPubKey {
			return authErr
		}
	} else if ticket.Delegator != tx.GetFrom().String() {
		return authErr
	}

	// Ensure the ticket is still active
	expireBy := ticket.ExpireBy
	if expireBy != 0 && expireBy > uint64(bi.Height) {
		return feI(index, "hash", "ticket is already expiring")
	} else if expireBy != 0 && expireBy <= uint64(bi.Height) {
		return feI(index, "hash", "ticket has already expired")
	}

	pubKey, _ := crypto.PubKeyFromBytes(tx.GetSenderPubKey().Bytes())
	if err = logic.DrySend(pubKey, "0", tx.Fee, tx.GetNonce(), 0); err != nil {
		return err
	}

	return nil
}

// CheckTxRepoCreateConsistency performs consistency checks on TxRepoCreate
func CheckTxRepoCreateConsistency(tx *txns.TxRepoCreate, index int, logic core.Logic) error {

	repoState := logic.RepoKeeper().Get(tx.Name)
	if !repoState.IsNil() {
		return feI(index, "name", "name is not available. choose another")
	}

	pubKey, _ := crypto.PubKeyFromBytes(tx.GetSenderPubKey().Bytes())
	if err := logic.DrySend(pubKey, tx.Value, tx.Fee, tx.GetNonce(), 0); err != nil {
		return err
	}

	return nil
}

// CheckTxSetDelegateCommissionConsistency performs consistency checks on TxSetDelegateCommission
func CheckTxSetDelegateCommissionConsistency(tx *txns.TxSetDelegateCommission, index int, logic core.Logic) error {
	pubKey, _ := crypto.PubKeyFromBytes(tx.GetSenderPubKey().Bytes())
	if err := logic.DrySend(pubKey, "0", tx.Fee, tx.GetNonce(), 0); err != nil {
		return err
	}
	return nil
}

// CheckTxRegisterPushKeyConsistency performs consistency checks on TxRegisterPushKey
func CheckTxRegisterPushKeyConsistency(
	tx *txns.TxRegisterPushKey,
	index int,
	logic core.Logic) error {

	// Check whether there is a matching push key already existing
	pushKeyID := crypto.CreatePushKeyID(tx.PublicKey)
	pushKey := logic.PushKeyKeeper().Get(pushKeyID)
	if !pushKey.IsNil() {
		return feI(index, "pubKey", "push key already registered")
	}

	pubKey, _ := crypto.PubKeyFromBytes(tx.GetSenderPubKey().Bytes())
	if err := logic.DrySend(pubKey, "0", tx.Fee, tx.GetNonce(), 0); err != nil {
		return err
	}

	return nil
}

// CheckTxRegisterPushKeyConsistency performs consistency checks on TxUpDelPushKey
func CheckTxUpDelPushKeyConsistency(
	tx *txns.TxUpDelPushKey,
	index int,
	logic core.Logic) error {

	key := logic.PushKeyKeeper().Get(tx.ID)
	if key.IsNil() {
		return feI(index, "id", "push key not found")
	}

	// Ensure sender is the owner of the key
	if !tx.SenderPubKey.MustAddress().Equal(key.Address) {
		return feI(index, "senderPubKey", "sender is not the owner of the key")
	}

	// Ensure the index of scopes to be removed are not out of range
	if len(tx.RemoveScopes) > 0 {
		for i, si := range tx.RemoveScopes {
			if si >= len(key.Scopes) {
				return feI(index, fmt.Sprintf("removeScopes[%d]", i), "index out of range")
			}
		}
	}

	pubKey, _ := crypto.PubKeyFromBytes(tx.GetSenderPubKey().Bytes())
	if err := logic.DrySend(pubKey, "0", tx.Fee, tx.GetNonce(), 0); err != nil {
		return err
	}

	return nil
}

// CheckTxPushConsistency performs consistency checks on TxPush.
// EXPECTS: sanity check using CheckTxPush to have been performed.
func CheckTxPushConsistency(tx *txns.TxPush, index int, logic core.Logic) error {

	repoState := logic.RepoKeeper().Get(tx.Note.GetRepoName())
	if repoState.IsNil() {
		return fmt.Errorf("repo not found")
	}

	hosts, err := logic.GetTicketManager().GetTopHosts(params.NumTopHostsLimit)
	if err != nil {
		return errors.Wrap(err, "failed to get top hosts")
	}

	var endPubKeys []*bls.PublicKey
	for index, end := range tx.Endorsements {

		// Perform consistency checks but don't check the BLS signature as we don't
		// care about that when dealing with a TxPush object, instead we care
		// about checking the aggregated BLS signature.
		err := validation.CheckEndorsementConsistencyUsingHost(hosts, end, true, index)
		if err != nil {
			return err
		}

		signerTicket := hosts.Get(end.EndorserPubKey)
		blsPubKey, err := bls.BytesToPublicKey(signerTicket.Ticket.BLSPubKey)
		if err != nil {
			return errors.Wrap(err, "failed to decode bls public key of endorser")
		}
		endPubKeys = append(endPubKeys, blsPubKey)

		// Verify the endorsements
		for i, endorsement := range end.References {
			ref := tx.Note.GetPushedReferences()[i]

			// If reference doesnt exist in the repo state, we don't expect
			// the endorsement to include a hash.
			if endorsement.Hash == nil && !repoState.References.Has(ref.Name) {
				continue
			}

			// Get the current state hash of the reference.
			// If unset, use zero hash.
			curRefHash := repoState.References.Get(ref.Name).Hash
			if curRefHash.IsEmpty() {
				curRefHash = plumbing2.ZeroHash[:]
			}

			// We expect the reference hash to match the endorsement hash.
			if !bytes.Equal(endorsement.Hash, curRefHash) {
				msg := fmt.Sprintf("hash (%x) of endorsed reference (%s) is not the expected hash (%s)",
					endorsement.Hash, ref.Name, plumbing.BytesToHex(curRefHash))
				return feI(index, "endorsements.hash", msg)
			}
		}
	}

	// Temporarily set the endorser's NoteID to be the note ID.
	// Endorsements are not expected to transmit the note ID but we need
	// it to properly verify the BLS signature.
	tx.Endorsements[0].NoteID = tx.Note.ID().Bytes()
	defer tx.Endorsements.ClearNoteID()

	// Generate an aggregated public key and use it to check the endorsers aggregated signature.
	// Use the bytes output of the first endorsement since all endorsement are expected to be the same.
	aggPubKey, _ := bls.AggregatePublicKeys(endPubKeys)
	err = aggPubKey.Verify(tx.AggregatedSig, tx.Endorsements[0].BytesForBLSSig())
	if err != nil {
		return errors.Wrap(err, "could not verify aggregated endorsers' signature")
	}

	// Check push note
	if err := validation.CheckPushNoteConsistency(tx.Note, logic); err != nil {
		return err
	}

	return nil
}

// CheckTxNSAcquireConsistency performs consistency checks on TxNamespaceRegister
func CheckTxNSAcquireConsistency(tx *txns.TxNamespaceRegister, index int, logic core.Logic) error {

	bi, err := logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		return errors.Wrap(err, "failed to fetch current block info")
	}

	ns := logic.NamespaceKeeper().Get(tx.Name)
	if !ns.IsNil() && ns.GraceEndAt.UInt64() > uint64(bi.Height) {
		return feI(index, "name", "chosen name is not currently available")
	}

	// If transfer recipient is a repo name
	if tx.To != "" &&
		identifier.IsValidResourceName(tx.To) == nil &&
		crypto.IsValidUserAddr(tx.To) != nil {
		if logic.RepoKeeper().Get(tx.To).IsNil() {
			return feI(index, "to", "repo does not exist")
		}
	}

	// If transfer recipient is an address of an account
	if tx.To != "" && identifier.IsValidUserAddr(tx.To) == nil {
		if logic.AccountKeeper().Get(identifier.Address(tx.To)).IsNil() {
			return feI(index, "to", "account does not exist")
		}
	}

	pubKey, _ := crypto.PubKeyFromBytes(tx.GetSenderPubKey().Bytes())
	if err = logic.DrySend(pubKey, tx.Value, tx.Fee, tx.GetNonce(), 0); err != nil {
		return err
	}

	return nil
}

// CheckTxNamespaceDomainUpdateConsistency performs consistency
// checks on TxNamespaceDomainUpdate
func CheckTxNamespaceDomainUpdateConsistency(tx *txns.TxNamespaceDomainUpdate, index int, logic core.Logic) error {

	pubKey, _ := crypto.PubKeyFromBytes(tx.GetSenderPubKey().Bytes())

	// Ensure the sender of the transaction is the owner of the namespace
	ns := logic.NamespaceKeeper().Get(tx.Name)
	if ns.IsNil() {
		return feI(index, "name", "namespace not found")
	}

	if ns.Owner != pubKey.Addr().String() {
		return feI(index, "senderPubKey", "sender not permitted to perform this operation")
	}

	if err := logic.DrySend(pubKey, "0", tx.Fee, tx.GetNonce(), 0); err != nil {
		return err
	}

	return nil
}

// CheckProposalCommonConsistency includes common consistency checks for
// proposal transactions.
func CheckProposalCommonConsistency(
	prop *txns.TxProposalCommon,
	txCommon *txns.TxCommon,
	index int,
	logic core.Logic) (*state.Repository, error) {

	// Find the repository
	repo := logic.RepoKeeper().Get(prop.RepoName)
	if repo.IsNil() {
		return nil, feI(index, "name", "repo not found")
	}

	// Ensure no proposal with matching ID exist
	if repo.Proposals.Get(prop.ID) != nil {
		return nil, feI(index, "id", "proposal id has been used, choose another")
	}

	repoPropFee := repo.Config.Gov.PropFee
	propFeeDec := decimal.NewFromFloat(repoPropFee)

	// When the repo does not require a proposal deposit,
	// ensure a proposal fee is not set.
	if propFeeDec.Equal(decimal.Zero) &&
		!prop.Value.Decimal().Equal(decimal.Zero) {
		return nil, feI(index, "value", constants.ErrProposalFeeNotExpected.Error())
	}

	// When the repo does not support a fee deposit duration period,
	// ensure the minimum fee was paid in the current transaction.
	if repo.Config.Gov.PropFeeDepositDur == 0 {
		if propFeeDec.GreaterThan(decimal.Zero) && prop.Value.Decimal().LessThan(propFeeDec) {
			msg := fmt.Sprintf("proposal fee cannot be less than repo minimum (%f)", repoPropFee)
			return nil, feI(index, "value", msg)
		}
	}

	// Check if the sender is permitted to create the proposal.
	// When proposal creator parameter is ProposalCreatorOwner, the sender is permitted only if they are an owner...
	owner := repo.Owners.Get(txCommon.GetFrom().String())
	propCreator := repo.Config.Gov.PropCreator
	if propCreator == state.ProposalCreatorOwner && owner == nil {
		return nil, feI(index, "senderPubKey", "sender is not permitted to create proposal")
	}

	pubKey, _ := crypto.PubKeyFromBytes(txCommon.GetSenderPubKey().Bytes())
	if err := logic.DrySend(pubKey, prop.Value, txCommon.Fee, txCommon.GetNonce(), 0); err != nil {
		return nil, err
	}

	return repo, nil
}

// CheckTxRepoProposalUpsertOwnerConsistency performs consistency
// checks on CheckTxRepoProposalUpsertOwner
func CheckTxRepoProposalUpsertOwnerConsistency(
	tx *txns.TxRepoProposalUpsertOwner,
	index int,
	logic core.Logic) error {

	_, err := CheckProposalCommonConsistency(tx.TxProposalCommon, tx.TxCommon, index, logic)
	if err != nil {
		return err
	}

	return nil
}

// CheckTxVoteConsistency performs consistency checks on CheckTxVote
func CheckTxVoteConsistency(
	tx *txns.TxRepoProposalVote,
	index int,
	logic core.Logic) error {

	// The repo must exist
	repoState := logic.RepoKeeper().Get(tx.RepoName)
	if repoState.IsNil() {
		return feI(index, "name", "repo not found")
	}

	// The proposal must exist
	proposal := repoState.Proposals.Get(tx.ProposalID)
	if proposal == nil {
		return feI(index, "id", "proposal not found")
	}

	// Ensure repo has not been finalized
	if proposal.IsFinalized() {
		return feI(index, "id", "proposal has concluded")
	}

	bi, err := logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		return errors.Wrap(err, "failed to fetch current block info")
	}

	// Ensure repo is not currently within a fee deposit period
	if proposal.IsDepositPeriod(uint64(bi.Height + 1)) {
		return feI(index, "id", "proposal is currently in fee deposit period")
	}

	// If the proposal has fee deposit period enabled, ensure the
	// total proposal fee has been deposited
	if proposal.IsFeeDepositEnabled() && !proposal.IsDepositedFeeOK() {
		return feI(index, "id", "total deposited proposal fee is insufficient")
	}

	// If the proposal is targeted at repo owners, then
	// the sender must be an owner
	senderOwner := repoState.Owners.Get(tx.GetFrom().String())
	if proposal.GetVoterType() == state.VoterOwner && senderOwner == nil {
		return feI(index, "senderPubKey", "sender is not one of the repo owners")
	}

	// If the proposal is targeted at repo owners and
	// the vote is a NoWithVeto, then the sender must have veto rights.
	if proposal.GetVoterType() == state.VoterOwner &&
		tx.Vote == state.ProposalVoteNoWithVeto && !senderOwner.Veto {
		return feI(index, "senderPubKey", "sender cannot vote 'no with veto' because "+
			"they have no veto right")
	}

	// Ensure the sender had not previously voted
	_, voted, err := logic.RepoKeeper().
		GetProposalVote(tx.RepoName, tx.ProposalID, tx.GetFrom().String())
	if err != nil {
		return errors.Wrap(err, "failed to check proposal vote")
	} else if voted {
		return feI(index, "id", "vote already cast on the target proposal")
	}

	pubKey, _ := crypto.PubKeyFromBytes(tx.GetSenderPubKey().Bytes())
	if err = logic.DrySend(pubKey, "0", tx.Fee,
		tx.GetNonce(), uint64(bi.Height)); err != nil {
		return err
	}

	return nil
}

// CheckTxRepoProposalSendFeeConsistency performs consistency checks on TxRepoProposalSendFee
func CheckTxRepoProposalSendFeeConsistency(
	tx *txns.TxRepoProposalSendFee,
	index int,
	logic core.Logic) error {

	// The repo must exist
	repoState := logic.RepoKeeper().Get(tx.RepoName)
	if repoState.IsNil() {
		return feI(index, "name", "repo not found")
	}

	// The proposal must exist
	proposal := repoState.Proposals.Get(tx.ID)
	if proposal == nil {
		return feI(index, "id", "proposal not found")
	}

	// Ensure repo has not been finalized
	if proposal.IsFinalized() {
		return feI(index, "id", "proposal has concluded")
	}

	bi, err := logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		return errors.Wrap(err, "failed to fetch current block info")
	}

	// Ensure the proposal supports fee deposit
	if !proposal.IsFeeDepositEnabled() {
		return feI(index, "id", "fee deposit not enabled for the proposal")
	}

	// Ensure repo is within a fee deposit period
	if !proposal.IsDepositPeriod(uint64(bi.Height + 1)) {
		return feI(index, "id", "proposal fee deposit period has closed")
	}

	pubKey, _ := crypto.PubKeyFromBytes(tx.GetSenderPubKey().Bytes())
	if err = logic.DrySend(pubKey, "0", tx.Fee,
		tx.GetNonce(), uint64(bi.Height)); err != nil {
		return err
	}

	return nil
}

// CheckTxRepoProposalUpdateConsistency performs consistency checks on CheckTxRepoProposalUpdate
func CheckTxRepoProposalUpdateConsistency(tx *txns.TxRepoProposalUpdate, index int, logic core.Logic) error {
	_, err := CheckProposalCommonConsistency(tx.TxProposalCommon, tx.TxCommon, index, logic)
	if err != nil {
		return err
	}
	return nil
}

// CheckTxRepoProposalRegisterPushKeyConsistency performs consistency checks on TxRepoProposalRegisterPushKey
func CheckTxRepoProposalRegisterPushKeyConsistency(
	tx *txns.TxRepoProposalRegisterPushKey,
	index int,
	logic core.Logic) error {

	// Check whether the namespace provided in both Namespace or NamespaceOnly
	// fields exist and are owned by the target repository.
	ns, nsField := tx.Namespace, "namespace"
	if tx.NamespaceOnly != "" {
		ns = tx.NamespaceOnly
		nsField = "namespaceOnly"
	}

	if ns != "" {
		ns = crypto2.MakeNamespaceHash(ns)
		found := logic.NamespaceKeeper().Get(ns)
		if found.IsNil() {
			return feI(index, nsField, "namespace not found")
		}
		if found.Owner != tx.RepoName {
			return feI(index, nsField, "namespace not owned by the target repository")
		}
	}

	_, err := CheckProposalCommonConsistency(tx.TxProposalCommon, tx.TxCommon, index, logic)
	if err != nil {
		return err
	}

	return nil
}
