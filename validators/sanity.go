package validators

import (
	"fmt"
	"regexp"
	"strings"

	"gitlab.com/makeos/mosdef/remote/validation"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"

	"github.com/asaskevich/govalidator"
	"github.com/thoas/go-funk"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/util"

	v "github.com/go-ozzo/ozzo-validation"
	"github.com/shopspring/decimal"
	"gitlab.com/makeos/mosdef/params"
)

// CheckRecipient validates the recipient address
func CheckRecipient(tx *core.TxRecipient, index int) error {

	recipient := tx.To

	if tx.To.IsEmpty() {
		return feI(index, "to", "recipient address is required")
	}

	if strings.Index(recipient.String(), "/") == -1 {
		if crypto.IsValidAccountAddr(recipient.String()) != nil {
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

func checkType(tx *core.TxType, expected types.TxCode, index int) error {
	if !tx.Is(expected) {
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

	if tx.Type != core.TxTypeValidatorTicket && tx.Type != core.TxTypeHostTicket {
		return feI(index, "type", "type is invalid")
	}

	if err := checkPositiveValue(tx.TxValue, index); err != nil {
		return err
	}

	// Non-delegate host ticket value must reach the minimum stake
	if tx.Is(core.TxTypeHostTicket) && tx.Delegate.IsEmpty() {
		if tx.Value.Decimal().LessThan(params.MinHostStake) {
			return feI(index, "value", fmt.Sprintf("value is lower than minimum host stake"))
		}
	}

	if !tx.Delegate.IsEmpty() {
		if err := v.Validate(tx.Delegate,
			v.By(validPubKeyRule(feI(index, "delegate", "requires a valid public key"))),
		); err != nil {
			return err
		}
	}

	if tx.Is(core.TxTypeHostTicket) {
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

	if err := checkType(tx.TxType, core.TxTypeHostTicket, index); err != nil {
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
	govCfg := actual.Governance

	// Ensure the voter type is known
	allowedVoterChoices := []state.VoterType{0,
		state.VoterOwner,
		state.VoterNetStakers,
		state.VoterNetStakersAndVetoOwner}
	if !funk.Contains(allowedVoterChoices, govCfg.Voter) {
		return feI(index, "config.gov.propVoter", fmt.Sprintf("unknown value"))
	}

	// Ensure the proposal creator type is known
	allowedPropCreator := []state.ProposalCreatorType{0,
		state.ProposalCreatorAny,
		state.ProposalCreatorOwner,
		state.ProposalCreatorOwnerAndAnyForMerge}
	if !funk.Contains(allowedPropCreator, govCfg.ProposalCreator) {
		return feI(index, "config.gov.propCreator", fmt.Sprintf("unknown value"))
	}

	sf := fmt.Sprintf

	// Ensure the proposer tally method is known
	allowedTallyMethod := []state.ProposalTallyMethod{0,
		state.ProposalTallyMethodIdentity,
		state.ProposalTallyMethodCoinWeighted,
		state.ProposalTallyMethodNetStakeOfProposer,
		state.ProposalTallyMethodNetStakeOfDelegators,
		state.ProposalTallyMethodNetStake,
	}
	if !funk.Contains(allowedTallyMethod, govCfg.TallyMethodOfProposal) {
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

	// When proposer is ProposerOwner, tally method cannot be CoinWeighted or Identity
	isNotOwnerProposer := govCfg.Voter != state.VoterOwner
	if isNotOwnerProposer &&
		(govCfg.TallyMethodOfProposal == state.ProposalTallyMethodCoinWeighted ||
			govCfg.TallyMethodOfProposal == state.ProposalTallyMethodIdentity) {
		return feI(index, "config", "when proposer type is not 'ProposerOwner', tally methods "+
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

// CheckTxRegisterPushKey performs sanity checks on TxRegisterPushKey
func CheckTxRegisterPushKey(tx *core.TxRegisterPushKey, index int) error {

	if err := checkType(tx.TxType, core.TxTypeRegisterPushKey, index); err != nil {
		return err
	}

	if err := v.Validate(tx.PublicKey,
		v.By(isEmptyByte32(feI(index, "pubKey", "public key is required"))),
		v.By(validPubKeyRule(feI(index, "pubKey", "invalid public key"))),
	); err != nil {
		return err
	}

	// If there are scope entries, ensure only namespaces URI,
	// repo names and non-address entries are contained in the list
	if err := checkScopes(tx.Scopes, index); err != nil {
		return err
	}

	// If fee cap is set, validate it
	if !tx.FeeCap.Empty() {
		if err := checkFeeCap(tx.FeeCap, index); err != nil {
			return err
		}
	}

	if err := checkCommon(tx, index); err != nil {
		return err
	}

	return nil
}

func checkScopes(scopes []string, index int) error {
	for i, s := range scopes {
		if (!util.IsNamespaceURI(s) && util.IsValidName(s) != nil) || util.IsValidAddr(s) == nil {
			return feI(index, fmt.Sprintf("scopes[%d]", i), "not an acceptable scope. "+
				"Expects a namespace URI or repository name")
		}
	}
	return nil
}

// CheckTxUpDelPushKey performs sanity checks on TxRegisterPushKey
func CheckTxUpDelPushKey(tx *core.TxUpDelPushKey, index int) error {

	if err := checkType(tx.TxType, core.TxTypeUpDelPushKey, index); err != nil {
		return err
	}

	if tx.ID == "" {
		return feI(index, "id", "push key id is required")
	} else if !util.IsValidPushAddr(tx.ID) {
		return feI(index, "id", "push key id is not valid")
	}

	// If there are scope entries, ensure only namespaces URI,
	// repo names and non-address entries are contained in the list
	if err := checkScopes(tx.AddScopes, index); err != nil {
		return err
	}

	// If fee cap is set, validate it
	if !tx.FeeCap.Empty() {
		if err := checkFeeCap(tx.FeeCap, index); err != nil {
			return err
		}
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

	if err := validation.CheckPushNoteSyntax(tx.PushNote); err != nil {
		return err
	}

	if len(tx.PushEnds) < params.PushEndorseQuorumSize {
		return feI(index, "endorsements", "not enough endorsements included")
	}

	senders := map[string]struct{}{}
	pushEndRefHashesID := util.EmptyBytes32
	for index, pushEnd := range tx.PushEnds {

		if err := validation.CheckPushEndorsement(pushEnd, index); err != nil {
			return err
		}

		// Ensure push note id and the target pushEnd push note id match
		if !pushEnd.NoteID.Equal(tx.PushNote.ID()) {
			msg := "push note id and push endorsement id must match"
			return feI(index, "endorsements.pushNoteID", msg)
		}

		// Make sure we haven't seen a PushEndorsement from this sender before
		_, ok := senders[pushEnd.EndorserPubKey.HexStr()]
		if !ok {
			senders[pushEnd.EndorserPubKey.HexStr()] = struct{}{}
		} else {
			msg := "multiple endorsement by a single sender not permitted"
			return feI(index, "endorsements.senderPubKey", msg)
		}

		_, err := crypto.PubKeyFromBytes(pushEnd.EndorserPubKey.Bytes())
		if err != nil {
			return feI(index, "endorsements.senderPubKey", "public key is not valid")
		}

		// Ensure the references hashes are all the same
		if pushEndRefHashesID.IsEmpty() {
			pushEndRefHashesID = pushEnd.References.ID()
		}
		if !pushEnd.References.ID().Equal(pushEndRefHashesID) {
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
		if target[:2] == "a/" && crypto.IsValidAccountAddr(target[2:]) != nil {
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

	if tx.TransferTo != "" {
		if crypto.IsValidAccountAddr(tx.TransferTo) != nil && util.IsValidName(tx.TransferTo) != nil {
			return feI(index, "to", "invalid value. Expected an address or a repository name")
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

	if err := checkRepoName(tx.RepoName, index); err != nil {
		return err
	}

	if err := checkProposalID(tx.ProposalID, index); err != nil {
		return err
	}

	if err := checkProposalFee(tx.Value, index); err != nil {
		return err
	}

	if len(tx.Addresses) == 0 {
		return feI(index, "addresses", "at least one address is required")
	}

	if len(tx.Addresses) > 10 {
		return feI(index, "addresses", "only a maximum of 10 addresses are allowed")
	}

	for i, addr := range tx.Addresses {
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

	if err := checkRepoName(tx.RepoName, index); err != nil {
		return err
	}

	if err := checkProposalID(tx.ProposalID, index); err != nil {
		return err
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

// CheckTxRepoProposalSendFee performs sanity checks on TxRepoProposalSendFee
func CheckTxRepoProposalSendFee(tx *core.TxRepoProposalSendFee, index int) error {

	if err := checkType(tx.TxType, core.TxTypeRepoProposalSendFee, index); err != nil {
		return err
	}

	if err := checkRepoName(tx.RepoName, index); err != nil {
		return err
	}

	if err := checkProposalID(tx.ProposalID, index); err != nil {
		return err
	}

	if err := checkValue(&core.TxValue{Value: tx.Value}, index); err != nil {
		return err
	}

	if err := checkCommon(tx, index); err != nil {
		return err
	}

	return nil
}

// checkProposalID performs sanity checks of a proposal id
func checkProposalID(id string, index int) error {
	if id == "" {
		return feI(index, "id", "proposal id is required")
	} else if !govalidator.IsNumeric(id) {
		return feI(index, "id", "proposal id is not valid")
	} else if len(id) > 8 {
		return feI(index, "id", "proposal id limit of 8 bytes exceeded")
	}
	return nil
}

// checkRepoName performs sanity checks on a repository name
func checkRepoName(name string, index int) error {
	return v.Validate(name,
		v.Required.Error(feI(index, "name", "repo name is required").Error()),
		v.By(validObjectNameRule("name", index)),
	)
}

// checkProposalFee performs sanity checks on a proposal fee
func checkProposalFee(fee util.String, index int) error {
	if err := checkValue(&core.TxValue{Value: fee}, index); err != nil {
		return err
	} else if fee.Decimal().LessThan(decimal.NewFromFloat(params.MinProposalFee)) {
		return feI(index, "value", "proposal creation fee cannot be "+
			"less than network minimum")
	}
	return nil
}

// checkFeeCap performs sanity checks on a fee cap
func checkFeeCap(fee util.String, index int) error {
	field := "feeCap"
	if err := v.Validate(fee,
		v.Required.Error(feI(index, field, "value is required").Error()),
		v.By(validValueRule(field, index)),
	); err != nil {
		return err
	}
	if fee.Decimal().LessThanOrEqual(decimal.Zero) {
		return feI(index, field, "value must be a positive number")
	}
	return nil
}

// CheckTxRepoProposalMergeRequest performs sanity checks on TxRepoProposalMergeRequest
func CheckTxRepoProposalMergeRequest(tx *core.TxRepoProposalMergeRequest, index int) error {

	if err := checkType(tx.TxType, core.TxTypeRepoProposalMergeRequest, index); err != nil {
		return err
	}

	if err := checkRepoName(tx.RepoName, index); err != nil {
		return err
	}

	if err := checkProposalID(tx.ProposalID, index); err != nil {
		return err
	}

	if err := checkProposalFee(tx.Value, index); err != nil {
		return err
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

	if err := checkRepoName(tx.RepoName, index); err != nil {
		return err
	}

	if err := checkProposalID(tx.ProposalID, index); err != nil {
		return err
	}

	if err := checkProposalFee(tx.Value, index); err != nil {
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

// CheckTxRepoProposalRegisterPushKey performs sanity checks on TxRepoProposalRegisterPushKey
func CheckTxRepoProposalRegisterPushKey(tx *core.TxRepoProposalRegisterPushKey, index int) error {

	if err := checkType(tx.TxType, core.TxTypeRepoProposalRegisterPushKey, index); err != nil {
		return err
	}

	if err := checkRepoName(tx.RepoName, index); err != nil {
		return err
	}

	if err := checkProposalID(tx.ProposalID, index); err != nil {
		return err
	}

	if err := checkProposalFee(tx.Value, index); err != nil {
		return err
	}

	// Ensure all push key ids are unique and valid
	found := map[string]struct{}{}
	for _, pkID := range tx.KeyIDs {
		if !util.IsValidPushAddr(pkID) {
			return feI(index, "ids", fmt.Sprintf("push key id (%s) is not valid", pkID))
		}
		if _, ok := found[pkID]; ok {
			return feI(index, "ids", fmt.Sprintf("push key id (%s) is a duplicate", pkID))
		}
		found[pkID] = struct{}{}
	}

	// Ensure fee mode is valid
	validFeeModes := []int{state.FeeModePusherPays, state.FeeModeRepoPays, state.FeeModeRepoPaysCapped}
	if !funk.ContainsInt(validFeeModes, int(tx.FeeMode)) {
		return feI(index, "feeMode", "fee mode is unknown")
	}

	// If fee mode is FeeModeRepoPaysCapped, ensure FeeCap is set and non-zero.
	if tx.FeeMode == state.FeeModeRepoPaysCapped {
		if err := checkFeeCap(tx.FeeCap, index); err != nil {
			return err
		}
	} else {
		if tx.FeeCap != "" {
			return feI(index, "feeCap", "value not expected for the chosen fee mode")
		}
	}

	// When namespace target is set, ensure a valid namespace name is provided.
	// If valid and NamespaceOnly is set, return an error
	if tx.Namespace != "" {
		if util.IsValidName(tx.Namespace) != nil {
			return feI(index, "namespace", "value format is not valid")
		}
		if tx.NamespaceOnly != "" {
			return feI(index, "namespaceOnly", "field is not expected because 'namespace' is set")
		}
	}

	// When namespaceOnly target is set, ensure a valid namespace name is provided.
	if tx.NamespaceOnly != "" {
		if util.IsValidName(tx.NamespaceOnly) != nil {
			return feI(index, "namespaceOnly", "value format is not valid")
		}
	}

	if err := checkCommon(tx, index); err != nil {
		return err
	}

	return nil
}
