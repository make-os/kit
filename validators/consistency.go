package validators

import (
	"crypto/rsa"
	"fmt"

	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/crypto/bls"
	"gitlab.com/makeos/mosdef/params"
	"gitlab.com/makeos/mosdef/repo"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/util"
)

// CheckTxCoinTransferConsistency performs consistency checks on TxCoinTransfer
func CheckTxCoinTransferConsistency(
	tx *core.TxCoinTransfer,
	index int,
	logic core.Logic) error {

	bi, err := logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		return errors.Wrap(err, "failed to fetch current block info")
	}

	recipient := tx.To.Address()

check:
	// If recipient address is a prefixed repo address, ensure repo exist
	if recipient.IsPrefixedRepoAddress() {
		targetRepo := logic.RepoKeeper().GetRepo(recipient.String()[2:], uint64(bi.Height))
		if targetRepo.IsNil() {
			return feI(index, "to", "recipient repo not found")
		}
	}

	// If the recipient address is a namespace uri, get the target and if the
	// target is a repository address, check that the repo exist.
	if recipient.IsNamespaceURI() {
		prefixedTarget, err := logic.NamespaceKeeper().
			GetTarget(recipient.String(), uint64(bi.Height))
		if err != nil {
			return feI(index, "to", err.Error())
		}
		recipient = util.Address(prefixedTarget)
		goto check
	}

	pubKey, _ := crypto.PubKeyFromBytes(tx.GetSenderPubKey().Bytes())
	if err = logic.Tx().CanExecCoinTransfer(pubKey, tx.Value, tx.Fee,
		tx.GetNonce(), uint64(bi.Height)); err != nil {
		return err
	}

	return nil
}

// CheckTxTicketPurchaseConsistency performs consistency checks on TxTicketPurchase
func CheckTxTicketPurchaseConsistency(
	tx *core.TxTicketPurchase,
	index int,
	logic core.Logic) error {

	bi, err := logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		return errors.Wrap(err, "failed to fetch current block info")
	}

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
	if tx.Type == core.TxTypeValidatorTicket && tx.Delegate.IsEmpty() {
		curTicketPrice := logic.Sys().GetCurValidatorTicketPrice()
		if tx.Value.Decimal().LessThan(decimal.NewFromFloat(curTicketPrice)) {
			return feI(index, "value", fmt.Sprintf("value is lower than the"+
				" minimum ticket price (%f)", curTicketPrice))
		}
	}

	pubKey, _ := crypto.PubKeyFromBytes(tx.GetSenderPubKey().Bytes())
	if err = logic.Tx().CanExecCoinTransfer(pubKey, tx.Value, tx.Fee,
		tx.GetNonce(), uint64(bi.Height)); err != nil {
		return err
	}

	return nil
}

// CheckTxUnbondTicketConsistency performs consistency checks on TxTicketUnbond
func CheckTxUnbondTicketConsistency(
	tx *core.TxTicketUnbond,
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
	decayBy := ticket.DecayBy
	if decayBy != 0 && decayBy > uint64(bi.Height) {
		return feI(index, "hash", "ticket is already decaying")
	} else if decayBy != 0 && decayBy <= uint64(bi.Height) {
		return feI(index, "hash", "ticket has already decayed")
	}

	pubKey, _ := crypto.PubKeyFromBytes(tx.GetSenderPubKey().Bytes())
	if err = logic.Tx().CanExecCoinTransfer(pubKey, "0", tx.Fee,
		tx.GetNonce(), uint64(bi.Height)); err != nil {
		return err
	}

	return nil
}

// CheckTxRepoCreateConsistency performs consistency checks on TxRepoCreate
func CheckTxRepoCreateConsistency(
	tx *core.TxRepoCreate,
	index int,
	logic core.Logic) error {

	bi, err := logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		return errors.Wrap(err, "failed to fetch current block info")
	}

	repo := logic.RepoKeeper().GetRepo(tx.Name)
	if !repo.IsNil() {
		msg := "name is not available. choose another"
		return feI(index, "name", msg)
	}

	pubKey, _ := crypto.PubKeyFromBytes(tx.GetSenderPubKey().Bytes())
	if err = logic.Tx().CanExecCoinTransfer(pubKey, tx.Value, tx.Fee,
		tx.GetNonce(), uint64(bi.Height)); err != nil {
		return err
	}

	return nil
}

// CheckTxSetDelegateCommissionConsistency performs consistency checks on TxSetDelegateCommission
func CheckTxSetDelegateCommissionConsistency(
	tx *core.TxSetDelegateCommission,
	index int,
	logic core.Logic) error {

	bi, err := logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		return errors.Wrap(err, "failed to fetch current block info")
	}

	pubKey, _ := crypto.PubKeyFromBytes(tx.GetSenderPubKey().Bytes())
	if err = logic.Tx().CanExecCoinTransfer(pubKey, "0", tx.Fee,
		tx.GetNonce(), uint64(bi.Height)); err != nil {
		return err
	}

	return nil
}

// CheckTxAddGPGPubKeyConsistency performs consistency checks on TxRegisterGPGPubKey
func CheckTxAddGPGPubKeyConsistency(
	tx *core.TxRegisterGPGPubKey,
	index int,
	logic core.Logic) error {

	bi, err := logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		return errors.Wrap(err, "failed to fetch current block info")
	}

	entity, _ := crypto.PGPEntityFromPubKey(tx.PublicKey)
	pk := entity.PrimaryKey.PublicKey.(*rsa.PublicKey)

	// Ensure bit length is not less than 256
	if pk.Size() < 256 {
		msg := "gpg public key bit length must be at least 2048 bits"
		return feI(index, "pubKey", msg)
	}

	// Check whether there is a matching gpg key already existing
	gpgID := util.CreateGPGIDFromRSA(entity.PrimaryKey.PublicKey.(*rsa.PublicKey))
	gpgPubKey := logic.GPGPubKeyKeeper().GetGPGPubKey(gpgID)
	if !gpgPubKey.IsNil() {
		return feI(index, "pubKey", "gpg public key already registered")
	}

	pubKey, _ := crypto.PubKeyFromBytes(tx.GetSenderPubKey().Bytes())
	if err = logic.Tx().CanExecCoinTransfer(pubKey, "0", tx.Fee,
		tx.GetNonce(), uint64(bi.Height)); err != nil {
		return err
	}

	return nil
}

// CheckTxPushConsistency performs consistency checks on TxPush.
// EXPECTS: sanity check using CheckTxPush to have been performed.
func CheckTxPushConsistency(
	tx *core.TxPush,
	index int,
	logic core.Logic,
	repoGetter func(name string) (core.BareRepo, error)) error {

	localRepo, err := repoGetter(tx.PushNote.GetRepoName())
	if err != nil {
		return errors.Wrap(err, "failed to get repo")
	}

	hosts, err := logic.GetTicketManager().GetTopHosts(params.NumTopHostsLimit)
	if err != nil {
		return errors.Wrap(err, "failed to get top hosts")
	}

	pokPubKeys := []*bls.PublicKey{}
	for index, pok := range tx.PushOKs {

		// Perform consistency checks but don't check the signature as we don't
		// care about that when dealing with a TxPush object, instead we care
		// about checking the aggregated BLS signature
		if err := repo.CheckPushOKConsistencyUsingHost(hosts, pok,
			logic, true, index); err != nil {
			return err
		}

		// Get the BLS public key of the PushOK signer
		signerTicket := hosts.Get(pok.SenderPubKey)
		if signerTicket == nil {
			return fmt.Errorf("push endorser not part of the top hosts")
		}
		blsPubKey, err := bls.BytesToPublicKey(signerTicket.Ticket.BLSPubKey)
		if err != nil {
			return errors.Wrap(err, "failed to decode bls public key of endorser")
		}
		pokPubKeys = append(pokPubKeys, blsPubKey)

		// Ensure the references hash match local history
		for i, refHash := range pok.ReferencesHash {
			ref := tx.PushNote.References[i]
			curRefHash, err := localRepo.TreeRoot(ref.Name)
			if err != nil {
				return errors.Wrapf(err, "failed to get reference (%s) tree root hash", ref.Name)
			}
			if !refHash.Hash.Equal(curRefHash) {
				msg := fmt.Sprintf("wrong tree hash for reference (%s)", ref.Name)
				return feI(index, "endorsements.refsHash", msg)
			}
		}
	}

	// Generate an aggregated public key and use it to check
	// the endorsers aggregated signature
	aggPubKey, _ := bls.AggregatePublicKeys(pokPubKeys)
	err = aggPubKey.Verify(tx.AggPushOKsSig, tx.PushOKs[0].BytesNoSigAndSenderPubKey())
	if err != nil {
		return errors.Wrap(err, "could not verify aggregated endorsers' signature")
	}

	// Check push note
	if err := repo.CheckPushNoteConsistency(tx.PushNote, logic); err != nil {
		return err
	}

	return nil
}

// CheckTxNSAcquireConsistency performs consistency checks on TxNamespaceAcquire
func CheckTxNSAcquireConsistency(
	tx *core.TxNamespaceAcquire,
	index int,
	logic core.Logic) error {

	bi, err := logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		return errors.Wrap(err, "failed to fetch current block info")
	}

	ns := logic.NamespaceKeeper().GetNamespace(tx.Name)
	if !ns.IsNil() && ns.GraceEndAt > uint64(bi.Height) {
		return feI(index, "name", "chosen name is not currently available")
	}

	pubKey, _ := crypto.PubKeyFromBytes(tx.GetSenderPubKey().Bytes())
	if err = logic.Tx().CanExecCoinTransfer(pubKey, tx.Value, tx.Fee,
		tx.GetNonce(), uint64(bi.Height)); err != nil {
		return err
	}

	return nil
}

// CheckTxNamespaceDomainUpdateConsistency performs consistency
// checks on TxNamespaceDomainUpdate
func CheckTxNamespaceDomainUpdateConsistency(
	tx *core.TxNamespaceDomainUpdate,
	index int,
	logic core.Logic) error {

	bi, err := logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		return errors.Wrap(err, "failed to fetch current block info")
	}

	pubKey, _ := crypto.PubKeyFromBytes(tx.GetSenderPubKey().Bytes())

	// Ensure the sender of the transaction is the owner of the namespace
	ns := logic.NamespaceKeeper().GetNamespace(tx.Name)
	if ns.IsNil() {
		return feI(index, "name", "namespace not found")
	}

	if ns.Owner != pubKey.Addr().String() {
		return feI(index, "senderPubKey", "sender not permitted to perform this operation")
	}

	if err = logic.Tx().CanExecCoinTransfer(pubKey, "0", tx.Fee,
		tx.GetNonce(), uint64(bi.Height)); err != nil {
		return err
	}

	return nil
}

// CheckProposalCommonConsistency includes common consistency checks for
// proposal transactions.
func CheckProposalCommonConsistency(
	txProposal *core.TxProposalCommon,
	txCommon *core.TxCommon,
	index int,
	logic core.Logic) (*state.Repository, error) {

	bi, err := logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch current block info")
	}

	targetRepo := logic.RepoKeeper().GetRepo(txProposal.RepoName, uint64(bi.Height))
	if targetRepo.IsNil() {
		return nil, feI(index, "name", "repo not found")
	}

	if targetRepo.Proposals.Get(txProposal.ProposalID) != nil {
		return nil, feI(index, "id", "proposal id has been used, choose another")
	}

	repoPropFee := decimal.NewFromFloat(targetRepo.Config.Governance.ProposalFee)

	// When the repo does not require a proposal deposit,
	// ensure a proposal fee is not set.
	if repoPropFee.Equal(decimal.Zero) &&
		!txProposal.Value.Decimal().Equal(decimal.Zero) {
		return nil, feI(index, "value", "proposal fee is not required but was provided")
	}

	// When the repo does not support a fee deposit duration period,
	// ensure the minimum fee was paid in the current transaction.
	if targetRepo.Config.Governance.ProposalFeeDepDur == 0 {
		if repoPropFee.GreaterThan(decimal.Zero) &&
			txProposal.Value.Decimal().LessThan(repoPropFee) {
			return nil, feI(index, "value", "proposal fee cannot be less than repo minimum")
		}
	}

	// If the repo is owned by some owners, ensure the sender is one of the owners
	senderOwner := targetRepo.Owners.Get(txCommon.GetFrom().String())
	if targetRepo.Config.Governance.ProposalProposee == state.ProposeeOwner && senderOwner == nil {
		return nil, feI(index, "senderPubKey", "sender is not one of the repo owners")
	}

	pubKey, _ := crypto.PubKeyFromBytes(txCommon.GetSenderPubKey().Bytes())
	if err := logic.Tx().CanExecCoinTransfer(pubKey, txProposal.Value, txCommon.Fee,
		txCommon.GetNonce(), uint64(bi.Height)); err != nil {
		return nil, err
	}

	return targetRepo, nil
}

// CheckTxRepoProposalUpsertOwnerConsistency performs consistency
// checks on CheckTxRepoProposalUpsertOwner
func CheckTxRepoProposalUpsertOwnerConsistency(
	tx *core.TxRepoProposalUpsertOwner,
	index int,
	logic core.Logic) error {

	_, err := CheckProposalCommonConsistency(
		tx.TxProposalCommon,
		tx.TxCommon,
		index,
		logic)
	if err != nil {
		return err
	}

	return nil
}

// CheckTxVoteConsistency performs consistency checks on CheckTxVote
func CheckTxVoteConsistency(
	tx *core.TxRepoProposalVote,
	index int,
	logic core.Logic) error {

	// The repo must exist
	repo := logic.RepoKeeper().GetRepo(tx.RepoName)
	if repo.IsNil() {
		return feI(index, "name", "repo not found")
	}

	// The proposal must exist
	proposal := repo.Proposals.Get(tx.ProposalID)
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
	senderOwner := repo.Owners.Get(tx.GetFrom().String())
	if proposal.GetProposeeType() == state.ProposeeOwner && senderOwner == nil {
		return feI(index, "senderPubKey", "sender is not one of the repo owners")
	}

	// If the proposal is targetted at repo owners and
	// the vote is a NoWithVeto, then the sender must have veto rights.
	if proposal.GetProposeeType() == state.ProposeeOwner &&
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
	if err = logic.Tx().CanExecCoinTransfer(pubKey, "0", tx.Fee,
		tx.GetNonce(), uint64(bi.Height)); err != nil {
		return err
	}

	return nil
}

// CheckTxRepoProposalSendFeeConsistency performs consistency checks on TxRepoProposalSendFee
func CheckTxRepoProposalSendFeeConsistency(
	tx *core.TxRepoProposalSendFee,
	index int,
	logic core.Logic) error {

	// The repo must exist
	repo := logic.RepoKeeper().GetRepo(tx.RepoName)
	if repo.IsNil() {
		return feI(index, "name", "repo not found")
	}

	// The proposal must exist
	proposal := repo.Proposals.Get(tx.ProposalID)
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
	if err = logic.Tx().CanExecCoinTransfer(pubKey, "0", tx.Fee,
		tx.GetNonce(), uint64(bi.Height)); err != nil {
		return err
	}

	return nil
}

// CheckTxRepoProposalMergeRequestConsistency performs consistency
// checks on TxRepoProposalMergeRequest
func CheckTxRepoProposalMergeRequestConsistency(
	tx *core.TxRepoProposalMergeRequest,
	index int,
	logic core.Logic) error {

	_, err := CheckProposalCommonConsistency(
		tx.TxProposalCommon,
		tx.TxCommon,
		index,
		logic)
	if err != nil {
		return err
	}

	return nil
}

// CheckTxRepoProposalUpdateConsistency performs consistency checks on CheckTxRepoProposalUpdate
func CheckTxRepoProposalUpdateConsistency(
	tx *core.TxRepoProposalUpdate,
	index int,
	logic core.Logic) error {

	_, err := CheckProposalCommonConsistency(
		tx.TxProposalCommon,
		tx.TxCommon,
		index,
		logic)
	if err != nil {
		return err
	}

	return nil
}

// CheckTxRepoProposalRegisterGPGKeyConsistency performs consistency checks on TxRepoProposalRegisterGPGKey
func CheckTxRepoProposalRegisterGPGKeyConsistency(
	tx *core.TxRepoProposalRegisterGPGKey,
	index int,
	logic core.Logic) error {

	_, err := CheckProposalCommonConsistency(
		tx.TxProposalCommon,
		tx.TxCommon,
		index,
		logic)
	if err != nil {
		return err
	}

	return nil
}
