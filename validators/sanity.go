package validators

import (
	"fmt"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"
	"regexp"
	"strings"

	"github.com/asaskevich/govalidator"
	"github.com/thoas/go-funk"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/repo"
	"gitlab.com/makeos/mosdef/util"

	v "github.com/go-ozzo/ozzo-validation"
	"github.com/shopspring/decimal"
	"gitlab.com/makeos/mosdef/params"
)

// CheckRecipient validates the recipient address
func CheckRecipient(tx *core.TxRecipient, index int) error {

	recipient := tx.To.Address()

	if tx.To.Empty() {
		return feI(index, "to", "recipient address is required")
	}

	if strings.Index(recipient.String(), "/") == -1 {
		if crypto.IsValidAddr(recipient.String()) != nil {
			goto bad
		}
		return nil
	}

	if recipient.IsPrefixed() {
		if recipient.IsPrefixedUserAddress() {
			goto bad
		}
		return nil
	}

	if recipient.IsNamespaceURI() {
		return nil
	}

bad:
	return feI(index, "to", "recipient address is not valid")
}

func checkValue(tx *core.TxValue, index int) error {
	if err := v.Validate(tx.Value, v.Required.Error(feI(index, "value",
		"value is required").Error()), v.By(validValueRule("value", index)),
	); err != nil {
		return err
	}
	return nil
}

func checkPositiveValue(tx *core.TxValue, index int) error {
	if err := v.Validate(tx.Value,
		v.Required.Error(feI(index, "value", "value is required").Error()),
		v.By(validValueRule("value", index)),
	); err != nil {
		return err
	}
	if tx.Value.Decimal().LessThanOrEqual(decimal.Zero) {
		return feI(index, "value", "value must be a positive number")
	}
	return nil
}

func checkType(tx *core.TxType, expected int, index int) error {
	if tx.Type != expected {
		return feI(index, "type", "type is invalid")
	}
	return nil
}

func checkCommon(tx types.BaseTx, index int) error {

	var baseFee, txSize decimal.Decimal

	if err := v.Validate(tx.GetNonce(),
		v.Required.Error(feI(index, "nonce", "nonce is required").Error())); err != nil {
		return err
	}

	if err := v.Validate(tx.GetFee(),
		v.Required.Error(feI(index, "fee", "fee is required").Error()),
		v.By(validValueRule("fee", index)),
	); err != nil {
		return err
	}

	// Fee must be at least equal to the base fee
	txSize = decimal.NewFromFloat(float64(tx.GetEcoSize()))
	baseFee = params.FeePerByte.Mul(txSize)
	if tx.GetFee().Decimal().LessThan(baseFee) {
		return util.FieldErrorWithIndex(index, "fee",
			fmt.Sprintf("fee cannot be lower than the base price of %s", baseFee.StringFixed(4)))
	}

	if err := v.Validate(tx.GetTimestamp(),
		v.Required.Error(feI(index, "timestamp", "timestamp is required").Error()),
		v.By(validTimestampRule("timestamp", index)),
	); err != nil {
		return err
	}

	if err := v.Validate(tx.GetSenderPubKey(),
		v.By(isEmptyByte32(feI(index, "senderPubKey", "sender public key is required"))),
		v.By(validPubKeyRule(feI(index, "senderPubKey", "sender public key is not valid"))),
	); err != nil {
		return err
	}

	if err := v.Validate(tx.GetSignature(),
		v.Required.Error(feI(index, "sig", "signature is required").Error()),
	); err != nil {
		return err
	}

	if sigErr := checkSignature(tx, index); len(sigErr) > 0 {
		return sigErr[0]
	}

	return nil
}

// CheckTxCoinTransfer performs sanity checks on TxCoinTransfer
func CheckTxCoinTransfer(tx *core.TxCoinTransfer, index int) error {

	if err := checkType(tx.TxType, core.TxTypeCoinTransfer, index); err != nil {
		return err
	}

	if err := CheckRecipient(tx.TxRecipient, index); err != nil {
		return err
	}

	if err := checkValue(tx.TxValue, index); err != nil {
		return err
	}

	if err := checkCommon(tx, index); err != nil {
		return err
	}

	return nil
}

// CheckTxTicketPurchase performs sanity checks on TxTicketPurchase
func CheckTxTicketPurchase(tx *core.TxTicketPurchase, index int) error {

	if tx.Type != core.TxTypeValidatorTicket && tx.Type != core.TxTypeStorerTicket {
		return feI(index, "type", "type is invalid")
	}

	if err := checkPositiveValue(tx.TxValue, index); err != nil {
		return err
	}

	// Non-delegate storer ticket value must reach the minimum stake
	if tx.Is(core.TxTypeStorerTicket) && tx.Delegate.IsEmpty() {
		if tx.Value.Decimal().LessThan(params.MinStorerStake) {
			return feI(index, "value", fmt.Sprintf("value is lower than minimum storer stake"))
		}
	}

	if !tx.Delegate.IsEmpty() {
		if err := v.Validate(tx.Delegate,
			v.By(validPubKeyRule(feI(index, "delegate", "requires a valid public key"))),
		); err != nil {
			return err
		}
	}

	if tx.Is(core.TxTypeStorerTicket) {
		if len(tx.BLSPubKey) == 0 {
			return feI(index, "blsPubKey", "BLS public key is required")
		}
		if len(tx.BLSPubKey) != 128 {
			return feI(index, "blsPubKey", "BLS public key length is invalid")
		}
	}

	if err := checkCommon(tx, index); err != nil {
		return err
	}

	return nil
}

// CheckTxUnbondTicket performs sanity checks on TxTicketUnbond
func CheckTxUnbondTicket(tx *core.TxTicketUnbond, index int) error {

	if err := checkType(tx.TxType, core.TxTypeStorerTicket, index); err != nil {
		return err
	}

	if tx.TicketHash.IsEmpty() {
		return feI(index, "ticket", "ticket id is required")
	}

	if err := checkCommon(tx, index); err != nil {
		return err
	}

	return nil
}

// CheckRepoConfig validates a repo configuration object
func CheckRepoConfig(cfg map[string]interface{}, index int) error {

	// Overwrite the default config with the user's config.
	// This is what happens during actual tx execution.
	// We mimic this operation to get the true version of
	// the config and validate it
	actual := state.MakeDefaultRepoConfig()
	actual.MergeMap(cfg)
	govCfg := actual.Governace

	// Ensure the proposee type is known
	allowedProposeeChoices := []state.ProposeeType{0,
		state.ProposeeOwner,
		state.ProposeeNetStakeholders,
		state.ProposeeNetStakeholdersAndVetoOwner}
	if !funk.Contains(allowedProposeeChoices, govCfg.ProposalProposee) {
		return feI(index, "config.gov.propProposee", fmt.Sprintf("unknown value"))
	}

	sf := fmt.Sprintf

	// Ensure the proposee tally method is known
	allowedTallyMethod := []state.ProposalTallyMethod{0,
		state.ProposalTallyMethodIdentity,
		state.ProposalTallyMethodCoinWeighted,
		state.ProposalTallyMethodNetStakeOfProposer,
		state.ProposalTallyMethodNetStakeOfDelegators,
		state.ProposalTallyMethodNetStake,
	}
	if !funk.Contains(allowedTallyMethod, govCfg.ProposalTallyMethod) {
		return feI(index, "config.gov.propTallyMethod", sf("unknown value"))
	}

	if govCfg.ProposalQuorum < 0 {
		return feI(index, "config.gov.propQuorum", sf("must be a non-negative number"))
	}

	if govCfg.ProposalThreshold < 0 {
		return feI(index, "config.gov.propThreshold", sf("must be a non-negative number"))
	}

	if govCfg.ProposalVetoQuorum < 0 {
		return feI(index, "config.gov.propVetoQuorum", sf("must be a non-negative number"))
	}

	if govCfg.ProposalVetoOwnersQuorum < 0 {
		return feI(index, "config.gov.propVetoOwnersQuorum", sf("must be a non-negative number"))
	}

	if govCfg.ProposalFee < params.MinProposalFee {
		return feI(index, "config.gov.propFee", sf("cannot be lower than network minimum"))
	}

	// When proposee is ProposeeOwner, tally method cannot be CoinWeighted or Identity
	isNotOwnerProposee := govCfg.ProposalProposee != state.ProposeeOwner
	if isNotOwnerProposee &&
		(govCfg.ProposalTallyMethod == state.ProposalTallyMethodCoinWeighted ||
			govCfg.ProposalTallyMethod == state.ProposalTallyMethodIdentity) {
		return feI(index, "config", "when proposee type is not 'ProposeeOwner', tally methods "+
			"'CoinWeighted' and 'Identity' are not allowed")
	}

	return nil
}

// CheckTxRepoCreate performs sanity checks on TxRepoCreate
func CheckTxRepoCreate(tx *core.TxRepoCreate, index int) error {

	if err := checkType(tx.TxType, core.TxTypeRepoCreate, index); err != nil {
		return err
	}

	if err := checkValue(tx.TxValue, index); err != nil {
		return err
	}

	if err := v.Validate(tx.Name,
		v.Required.Error(feI(index, "name", "requires a unique name").Error()),
		v.By(validObjectNameRule("name", index)),
	); err != nil {
		return err
	}

	if err := CheckRepoConfig(tx.Config, index); err != nil {
		return err
	}

	if err := checkCommon(tx, index); err != nil {
		return err
	}

	return nil
}

// CheckTxAddGPGPubKey performs sanity checks on TxAddGPGPubKey
func CheckTxAddGPGPubKey(tx *core.TxAddGPGPubKey, index int) error {

	if err := checkType(tx.TxType, core.TxTypeAddGPGPubKey, index); err != nil {
		return err
	}

	if err := v.Validate(tx.PublicKey,
		v.Required.Error(feI(index, "pubKey", "public key is required").Error()),
		v.By(validGPGPubKeyRule("pubKey", index)),
	); err != nil {
		return err
	}

	if err := checkCommon(tx, index); err != nil {
		return err
	}

	return nil
}

// CheckTxSetDelegateCommission performs sanity checks on TxSetDelegateCommission
func CheckTxSetDelegateCommission(tx *core.TxSetDelegateCommission, index int) error {

	if err := checkType(tx.TxType, core.TxTypeSetDelegatorCommission, index); err != nil {
		return err
	}

	if err := v.Validate(tx.Commission,
		v.Required.Error(feI(index, "commission", "commission rate is required").Error()),
	); err != nil {
		return err
	}

	if tx.Commission.Decimal().LessThan(params.MinDelegatorCommission) {
		minPct := params.MinDelegatorCommission.String()
		return feI(index, "commission", "rate cannot be below the minimum ("+minPct+"%%)")
	}

	if tx.Commission.Decimal().GreaterThan(decimal.NewFromFloat(100)) {
		return util.FieldErrorWithIndex(index, "commission", "commission rate cannot exceed 100%%")
	}

	if err := checkCommon(tx, index); err != nil {
		return err
	}

	return nil
}

// CheckTxPush performs sanity checks on TxPush
func CheckTxPush(tx *core.TxPush, index int) error {

	if err := checkType(tx.TxType, core.TxTypePush, index); err != nil {
		return err
	}

	if err := v.Validate(tx.PushNote,
		v.Required.Error(feI(index, "pushNote", "push note is required").Error()),
	); err != nil {
		return err
	}

	if err := repo.CheckPushNoteSyntax(tx.PushNote); err != nil {
		return err
	}

	if len(tx.PushOKs) < params.PushOKQuorumSize {
		return feI(index, "endorsements", "not enough endorsements included")
	}

	senders := map[string]struct{}{}
	pushOKRefHashesID := util.EmptyBytes32
	for index, pushOK := range tx.PushOKs {

		if err := repo.CheckPushOK(pushOK, index); err != nil {
			return err
		}

		// Ensure push note id and the target pushOK push note id match
		if !pushOK.PushNoteID.Equal(tx.PushNote.ID()) {
			msg := "push note id and push endorsement id must match"
			return feI(index, "endorsements.pushNoteID", msg)
		}

		// Make sure we haven't seen a PushOK from this sender before
		_, ok := senders[pushOK.SenderPubKey.HexStr()]
		if !ok {
			senders[pushOK.SenderPubKey.HexStr()] = struct{}{}
		} else {
			msg := "multiple endorsement by a single sender not permitted"
			return feI(index, "endorsements.senderPubKey", msg)
		}

		_, err := crypto.PubKeyFromBytes(pushOK.SenderPubKey.Bytes())
		if err != nil {
			return feI(index, "endorsements.senderPubKey", "public key is not valid")
		}

		// Ensure the references hashes are all the same
		if pushOKRefHashesID.IsEmpty() {
			pushOKRefHashesID = pushOK.ReferencesHash.ID()
		}
		if !pushOK.ReferencesHash.ID().Equal(pushOKRefHashesID) {
			msg := "references of all endorsements must match"
			return feI(index, "endorsements.refsHash", msg)
		}
	}

	return nil
}

// CheckNamespaceDomains checks namespace domains and targets
func CheckNamespaceDomains(domains map[string]string, index int) error {
	for domain, target := range domains {
		if !regexp.MustCompile(util.AddressNamespaceDomainNameRegexp).MatchString(domain) {
			return feI(index, "domains", fmt.Sprintf("domains.%s: name is invalid", domain))
		}
		if !util.IsPrefixedAddr(target) {
			return feI(index, "domains", fmt.Sprintf("domains.%s: target is invalid", domain))
		}
		if target[:2] == "a/" && crypto.IsValidAddr(target[2:]) != nil {
			return feI(index, "domains", fmt.Sprintf("domains.%s: target is not a valid address",
				domain))
		}
	}
	return nil
}

// CheckTxNSAcquire performs sanity checks on TxNamespaceAcquire
func CheckTxNSAcquire(tx *core.TxNamespaceAcquire, index int) error {

	if err := checkType(tx.TxType, core.TxTypeNSAcquire, index); err != nil {
		return err
	}

	if err := checkValue(tx.TxValue, index); err != nil {
		return err
	}

	if err := v.Validate(tx.Name,
		v.Required.Error(feI(index, "name", "requires a unique name").Error()),
		v.By(validObjectNameRule("name", index)),
	); err != nil {
		return err
	}

	if tx.TransferToRepo != "" && tx.TransferToAccount != "" {
		return feI(index, "", "can only transfer ownership to either an account or a repo")
	}

	if tx.TransferToAccount != "" {
		if err := v.Validate(tx.TransferToAccount,
			v.By(validAddrRule(feI(index, "transferToAccount", "address is not valid"))),
		); err != nil {
			return err
		}
	}

	if !tx.Value.Decimal().Equal(params.CostOfNamespace) {
		return feI(index, "value", fmt.Sprintf("invalid value; has %s, want %s",
			tx.Value, params.CostOfNamespace.String()))
	}

	if len(tx.Domains) > 0 {
		if err := CheckNamespaceDomains(tx.Domains, index); err != nil {
			return err
		}
	}

	if err := checkCommon(tx, index); err != nil {
		return err
	}

	return nil
}

// CheckTxNamespaceDomainUpdate performs sanity checks on TxNamespaceDomainUpdate
func CheckTxNamespaceDomainUpdate(tx *core.TxNamespaceDomainUpdate, index int) error {

	if err := checkType(tx.TxType, core.TxTypeNSDomainUpdate, index); err != nil {
		return err
	}

	if err := v.Validate(tx.Name,
		v.Required.Error(feI(index, "name", "requires a name").Error()),
		v.By(validObjectNameRule("name", index)),
	); err != nil {
		return err
	}

	if len(tx.Domains) > 0 {
		if err := CheckNamespaceDomains(tx.Domains, index); err != nil {
			return err
		}
	}

	if err := checkCommon(tx, index); err != nil {
		return err
	}

	return nil
}

// CheckTxRepoProposalUpsertOwner performs sanity checks on TxRepoProposalUpsertOwner
func CheckTxRepoProposalUpsertOwner(tx *core.TxRepoProposalUpsertOwner, index int) error {

	if err := checkType(tx.TxType, core.TxTypeRepoProposalUpsertOwner, index); err != nil {
		return err
	}

	if err := v.Validate(tx.RepoName,
		v.Required.Error(feI(index, "name", "repo name is required").Error()),
		v.By(validObjectNameRule("name", index)),
	); err != nil {
		return err
	}

	if tx.ProposalID == "" {
		return feI(index, "id", "proposal id is required")
	} else if !govalidator.IsNumeric(tx.ProposalID) {
		return feI(index, "id", "proposal id is not valid")
	} else if len(tx.ProposalID) > 8 {
		return feI(index, "id", "proposal id limit of 8 bytes exceeded")
	}

	if err := checkValue(&core.TxValue{Value: tx.Value}, index); err != nil {
		return err
	} else if tx.Value.Decimal().
		LessThan(decimal.NewFromFloat(params.MinProposalFee)) {
		return feI(index, "value", "proposal creation fee cannot be "+
			"less than network minimum")
	}

	if len(tx.Addresses) == 0 {
		return feI(index, "addresses", "at least one address is required")
	}

	addresses := strings.Split(tx.Addresses, ",")
	if len(addresses) > 10 {
		return feI(index, "addresses", "only a maximum of 10 addresses are allowed")
	}

	for i, addr := range addresses {
		field := fmt.Sprintf("addresses[%d]", i)
		if err := v.Validate(addr,
			v.By(validAddrRule(feI(index, field, "address is not valid"))),
		); err != nil {
			return err
		}
	}

	if err := checkCommon(tx, index); err != nil {
		return err
	}

	return nil
}

// CheckTxVote performs sanity checks on TxRepoProposalVote
func CheckTxVote(tx *core.TxRepoProposalVote, index int) error {

	if err := checkType(tx.TxType, core.TxTypeRepoProposalVote, index); err != nil {
		return err
	}

	if err := v.Validate(tx.RepoName,
		v.Required.Error(feI(index, "name", "repo name is required").Error()),
		v.By(validObjectNameRule("name", index)),
	); err != nil {
		return err
	}

	if tx.ProposalID == "" {
		return feI(index, "id", "proposal id is required")
	} else if !govalidator.IsNumeric(tx.ProposalID) {
		return feI(index, "id", "proposal id is not valid")
	}

	// Vote cannot be less than -1 or greater than 1.
	// 0 = No, 1 = Yes, -1 = NoWithVeto, -2 = Abstain
	if tx.Vote < -2 || tx.Vote > 1 {
		return feI(index, "vote", "vote choice is unknown")
	}

	if err := checkCommon(tx, index); err != nil {
		return err
	}

	return nil
}

// CheckTxRepoProposalSendFee performs sanity checks on TxRepoProposalFeeSend
func CheckTxRepoProposalSendFee(tx *core.TxRepoProposalFeeSend, index int) error {

	if err := checkType(tx.TxType, core.TxTypeRepoProposalFeeSend, index); err != nil {
		return err
	}

	if err := v.Validate(tx.RepoName,
		v.Required.Error(feI(index, "name", "repo name is required").Error()),
		v.By(validObjectNameRule("name", index)),
	); err != nil {
		return err
	}

	if tx.ProposalID == "" {
		return feI(index, "id", "proposal id is required")
	} else if !govalidator.IsNumeric(tx.ProposalID) {
		return feI(index, "id", "proposal id is not valid")
	} else if len(tx.ProposalID) > 8 {
		return feI(index, "id", "proposal id limit of 8 bytes exceeded")
	}

	if err := checkValue(&core.TxValue{Value: tx.Value}, index); err != nil {
		return err
	}

	if err := checkCommon(tx, index); err != nil {
		return err
	}

	return nil
}

// CheckTxRepoProposalMergeRequest performs sanity checks on TxRepoProposalMergeRequest
func CheckTxRepoProposalMergeRequest(tx *core.TxRepoProposalMergeRequest, index int) error {

	if err := checkType(tx.TxType, core.TxTypeRepoProposalMergeRequest, index); err != nil {
		return err
	}

	if err := v.Validate(tx.RepoName,
		v.Required.Error(feI(index, "name", "repo name is required").Error()),
		v.By(validObjectNameRule("name", index)),
	); err != nil {
		return err
	}

	if tx.ProposalID == "" {
		return feI(index, "id", "proposal id is required")
	} else if !govalidator.IsNumeric(tx.ProposalID) {
		return feI(index, "id", "proposal id is not valid")
	} else if len(tx.ProposalID) > 8 {
		return feI(index, "id", "proposal id limit of 8 bytes exceeded")
	}

	if err := checkValue(&core.TxValue{Value: tx.Value}, index); err != nil {
		return err
	} else if tx.Value.Decimal().
		LessThan(decimal.NewFromFloat(params.MinProposalFee)) {
		return feI(index, "value", "proposal creation fee cannot be "+
			"less than network minimum")
	}

	if tx.BaseBranch == "" {
		return feI(index, "base", "base branch name is required")
	}

	if len(tx.BaseBranchHash) > 0 && len(tx.BaseBranchHash) != 40 {
		return feI(index, "baseHash", "base branch hash is not valid")
	}

	if tx.TargetBranch == "" {
		return feI(index, "target", "target branch name is required")
	}

	if tx.TargetBranchHash == "" {
		return feI(index, "targetHash", "target branch hash is required")
	} else if len(tx.TargetBranchHash) != 40 {
		return feI(index, "targetHash", "target branch hash is not valid")
	}

	if err := checkCommon(tx, index); err != nil {
		return err
	}

	return nil
}

// CheckTxRepoProposalUpdate performs sanity checks on TxRepoProposalUpdate
func CheckTxRepoProposalUpdate(tx *core.TxRepoProposalUpdate, index int) error {

	if err := checkType(tx.TxType, core.TxTypeRepoProposalUpdate, index); err != nil {
		return err
	}

	if err := v.Validate(tx.RepoName,
		v.Required.Error(feI(index, "name", "repo name is required").Error()),
		v.By(validObjectNameRule("name", index)),
	); err != nil {
		return err
	}

	if tx.ProposalID == "" {
		return feI(index, "id", "proposal id is required")
	} else if !govalidator.IsNumeric(tx.ProposalID) {
		return feI(index, "id", "proposal id is not valid")
	} else if len(tx.ProposalID) > 8 {
		return feI(index, "id", "proposal id limit of 8 bytes exceeded")
	}

	if err := checkValue(&core.TxValue{Value: tx.Value}, index); err != nil {
		return err
	} else if tx.Value.Decimal().
		LessThan(decimal.NewFromFloat(params.MinProposalFee)) {
		return feI(index, "value", "proposal creation fee cannot be "+
			"less than network minimum")
	}

	if err := CheckRepoConfig(tx.Config, index); err != nil {
		return err
	}

	if err := checkCommon(tx, index); err != nil {
		return err
	}

	return nil
}
