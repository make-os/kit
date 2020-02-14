package validators

import (
	"crypto/rsa"
	"fmt"

	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/crypto/bls"
	"github.com/makeos/mosdef/params"
	"github.com/makeos/mosdef/repo"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
)

// CheckTxCoinTransferConsistency performs consistency checks on TxCoinTransfer
func CheckTxCoinTransferConsistency(
	tx *types.TxCoinTransfer,
	index int,
	logic types.Logic) error {

	bi, err := logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		return errors.Wrap(err, "failed to fetch current block info")
	}

	recipient := tx.To.Address()

check:
	// If recipient address is a prefixed, repo address, ensure repo exist
	if recipient.IsPrefixedRepoAddress() {
		repo := logic.RepoKeeper().GetRepo(recipient.String()[2:], uint64(bi.Height))
		if repo.IsNil() {
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
	if err = logic.Tx().CanExecCoinTransfer(tx.GetType(), pubKey, tx.Value, tx.Fee,
		tx.GetNonce(), uint64(bi.Height)); err != nil {
		return err
	}

	return nil
}

// CheckTxTicketPurchaseConsistency performs consistency checks on TxTicketPurchase
func CheckTxTicketPurchaseConsistency(
	tx *types.TxTicketPurchase,
	index int,
	logic types.Logic) error {

	bi, err := logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		return errors.Wrap(err, "failed to fetch current block info")
	}

	// When delegate is set, the delegate must have an active, non-delegated ticket
	if !tx.Delegate.IsEmpty() {
		r, err := logic.GetTicketManager().GetNonDelegatedTickets(tx.Delegate, tx.Type)
		if err != nil {
			return errors.Wrap(err, "failed to get active delegate tickets")
		} else if len(r) == 0 {
			return feI(index, "delegate", "specified delegate is not active")
		}
	}

	// For validator ticket transaction, the value must not be lesser than
	// the current price per ticket
	if tx.Type == types.TxTypeValidatorTicket {
		curTicketPrice := logic.Sys().GetCurValidatorTicketPrice()
		if tx.Value.Decimal().LessThan(decimal.NewFromFloat(curTicketPrice)) {
			return feI(index, "value", fmt.Sprintf("value is lower than the"+
				" minimum ticket price (%f)", curTicketPrice))
		}
	}

	pubKey, _ := crypto.PubKeyFromBytes(tx.GetSenderPubKey().Bytes())
	if err = logic.Tx().CanExecCoinTransfer(tx.GetType(), pubKey, tx.Value, tx.Fee,
		tx.GetNonce(), uint64(bi.Height)); err != nil {
		return err
	}

	return nil
}

// CheckTxUnbondTicketConsistency performs consistency checks on TxTicketUnbond
func CheckTxUnbondTicketConsistency(
	tx *types.TxTicketUnbond,
	index int,
	logic types.Logic) error {

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
		if tx.SenderPubKey != ticket.ProposerPubKey {
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
	if err = logic.Tx().CanExecCoinTransfer(tx.GetType(), pubKey, "0", tx.Fee,
		tx.GetNonce(), uint64(bi.Height)); err != nil {
		return err
	}

	return nil
}

// CheckTxRepoCreateConsistency performs consistency checks on TxRepoCreate
func CheckTxRepoCreateConsistency(
	tx *types.TxRepoCreate,
	index int,
	logic types.Logic) error {

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
	if err = logic.Tx().CanExecCoinTransfer(tx.GetType(), pubKey, tx.Value, tx.Fee,
		tx.GetNonce(), uint64(bi.Height)); err != nil {
		return err
	}

	return nil
}

// CheckTxSetDelegateCommissionConsistency performs consistency checks on TxSetDelegateCommission
func CheckTxSetDelegateCommissionConsistency(
	tx *types.TxSetDelegateCommission,
	index int,
	logic types.Logic) error {

	bi, err := logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		return errors.Wrap(err, "failed to fetch current block info")
	}

	pubKey, _ := crypto.PubKeyFromBytes(tx.GetSenderPubKey().Bytes())
	if err = logic.Tx().CanExecCoinTransfer(tx.GetType(), pubKey, "0", tx.Fee,
		tx.GetNonce(), uint64(bi.Height)); err != nil {
		return err
	}

	return nil
}

// CheckTxAddGPGPubKeyConsistency performs consistency checks on TxAddGPGPubKey
func CheckTxAddGPGPubKeyConsistency(
	tx *types.TxAddGPGPubKey,
	index int,
	logic types.Logic) error {

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
	pkID := util.RSAPubKeyID(entity.PrimaryKey.PublicKey.(*rsa.PublicKey))
	gpgPubKey := logic.GPGPubKeyKeeper().GetGPGPubKey(pkID)
	if !gpgPubKey.IsNil() {
		return feI(index, "pubKey", "gpg public key already registered")
	}

	pubKey, _ := crypto.PubKeyFromBytes(tx.GetSenderPubKey().Bytes())
	if err = logic.Tx().CanExecCoinTransfer(tx.GetType(), pubKey, "0", tx.Fee,
		tx.GetNonce(), uint64(bi.Height)); err != nil {
		return err
	}

	return nil
}

// CheckTxPushConsistency performs consistency checks on TxPush.
// EXPECTS: sanity check using CheckTxPush to have been performed.
func CheckTxPushConsistency(
	tx *types.TxPush,
	index int,
	logic types.Logic,
	repoGetter func(name string) (types.BareRepo, error)) error {

	localRepo, err := repoGetter(tx.PushNote.GetRepoName())
	if err != nil {
		return errors.Wrap(err, "failed to get repo")
	}

	storers, err := logic.GetTicketManager().GetTopStorers(params.NumTopStorersLimit)
	if err != nil {
		return errors.Wrap(err, "failed to get top storers")
	}

	pokPubKeys := []*bls.PublicKey{}
	for index, pok := range tx.PushOKs {

		// Perform consistency checks but don't check the signature as we don't
		// care about that when dealing with a TxPush object, instead we care
		// about checking the aggregated BLS signature
		if err := repo.CheckPushOKConsistencyUsingStorer(storers, pok,
			logic, true, index); err != nil {
			return err
		}

		// Get the BLS public key of the PushOK signer
		signerTicket := storers.Get(pok.SenderPubKey)
		if signerTicket == nil {
			return fmt.Errorf("push endorser not part of the top storers")
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
	tx *types.TxNamespaceAcquire,
	index int,
	logic types.Logic) error {

	bi, err := logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		return errors.Wrap(err, "failed to fetch current block info")
	}

	ns := logic.NamespaceKeeper().GetNamespace(tx.Name)
	if !ns.IsNil() && ns.GraceEndAt > uint64(bi.Height) {
		return feI(index, "name", "chosen name is not currently available")
	}

	pubKey, _ := crypto.PubKeyFromBytes(tx.GetSenderPubKey().Bytes())
	if err = logic.Tx().CanExecCoinTransfer(tx.GetType(), pubKey, tx.Value, tx.Fee,
		tx.GetNonce(), uint64(bi.Height)); err != nil {
		return err
	}

	return nil
}

// CheckTxNamespaceDomainUpdateConsistency performs consistency
// checks on TxNamespaceDomainUpdate
func CheckTxNamespaceDomainUpdateConsistency(
	tx *types.TxNamespaceDomainUpdate,
	index int,
	logic types.Logic) error {

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

	if err = logic.Tx().CanExecCoinTransfer(tx.GetType(), pubKey, "0", tx.Fee,
		tx.GetNonce(), uint64(bi.Height)); err != nil {
		return err
	}

	return nil
}

// checkProposalCommonConsistency includes common consistency checks for
// proposal transactions.
func checkProposalCommonConsistency(
	txType int,
	txProposal *types.TxProposalCommon,
	txCommon *types.TxCommon,
	index int,
	logic types.Logic) (*types.Repository, error) {

	bi, err := logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch current block info")
	}

	repo := logic.RepoKeeper().GetRepo(txProposal.RepoName, uint64(bi.Height))
	if repo.IsNil() {
		return nil, feI(index, "name", "repo not found")
	}

	if repo.Proposals.Get(txProposal.ProposalID) != nil {
		return nil, feI(index, "id", "proposal id has been used, choose another")
	}

	repoPropFee := decimal.NewFromFloat(repo.Config.Governace.ProposalFee)
	if repoPropFee.Equal(decimal.Zero) &&
		!txProposal.Value.Decimal().Equal(decimal.Zero) {
		return nil, feI(index, "value", "proposal fee is not required but was provided")
	}

	if repo.Config.Governace.ProposalFeeDepDur == 0 {
		if repoPropFee.GreaterThan(decimal.Zero) &&
			txProposal.Value.Decimal().LessThan(repoPropFee) {
			return nil, feI(index, "value", "proposal fee cannot be less than repo minimum")
		}
	}

	// If the repo is owned by some owners, ensure the sender is one of the owners
	senderOwner := repo.Owners.Get(txCommon.GetFrom().String())
	if repo.Config.Governace.ProposalProposee == types.ProposeeOwner && senderOwner == nil {
		return nil, feI(index, "senderPubKey", "sender is not one of the repo owners")
	}

	pubKey, _ := crypto.PubKeyFromBytes(txCommon.GetSenderPubKey().Bytes())
	if err := logic.Tx().CanExecCoinTransfer(txType, pubKey, txProposal.Value, txCommon.Fee,
		txCommon.GetNonce(), uint64(bi.Height)); err != nil {
		return nil, err
	}

	return repo, nil
}

// CheckTxRepoProposalUpsertOwnerConsistency performs consistency
// checks on CheckTxRepoProposalUpsertOwner
func CheckTxRepoProposalUpsertOwnerConsistency(
	tx *types.TxRepoProposalUpsertOwner,
	index int,
	logic types.Logic) error {

	_, err := checkProposalCommonConsistency(
		tx.Type,
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
	tx *types.TxRepoProposalVote,
	index int,
	logic types.Logic) error {

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

	// If the proposal is targetted at repo owners, then
	// the sender must be an owner
	senderOwner := repo.Owners.Get(tx.GetFrom().String())
	if proposal.GetProposeeType() == types.ProposeeOwner && senderOwner == nil {
		return feI(index, "senderPubKey", "sender is not one of the repo owners")
	}

	// If the proposal is targetted at repo owners and
	// the vote is a NoWithVeto, then the sender must have veto rights.
	if proposal.GetProposeeType() == types.ProposeeOwner &&
		tx.Vote == types.ProposalVoteNoWithVeto && !senderOwner.Veto {
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
	if err = logic.Tx().CanExecCoinTransfer(tx.GetType(), pubKey, "0", tx.Fee,
		tx.GetNonce(), uint64(bi.Height)); err != nil {
		return err
	}

	return nil
}

// CheckTxRepoProposalSendFeeConsistency performs consistency checks on TxRepoProposalFeeSend
func CheckTxRepoProposalSendFeeConsistency(
	tx *types.TxRepoProposalFeeSend,
	index int,
	logic types.Logic) error {

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
	if err = logic.Tx().CanExecCoinTransfer(tx.GetType(), pubKey, "0", tx.Fee,
		tx.GetNonce(), uint64(bi.Height)); err != nil {
		return err
	}

	return nil
}

// CheckTxRepoProposalMergeRequestConsistency performs consistency
// checks on TxRepoProposalMergeRequest
func CheckTxRepoProposalMergeRequestConsistency(
	tx *types.TxRepoProposalMergeRequest,
	index int,
	logic types.Logic) error {

	_, err := checkProposalCommonConsistency(
		tx.Type,
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
	tx *types.TxRepoProposalUpdate,
	index int,
	logic types.Logic) error {

	_, err := checkProposalCommonConsistency(
		tx.Type,
		tx.TxProposalCommon,
		tx.TxCommon,
		index,
		logic)
	if err != nil {
		return err
	}

	return nil
}
