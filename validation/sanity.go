package validation

import (
	"fmt"
	"strings"

	"github.com/AlekSi/pointer"
	"github.com/make-os/kit/remote/validation"
	"github.com/make-os/kit/types"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/types/txns"
	crypto2 "github.com/make-os/kit/util/crypto"
	errors2 "github.com/make-os/kit/util/errors"
	"github.com/make-os/kit/util/identifier"
	"github.com/pkg/errors"

	"github.com/asaskevich/govalidator"
	"github.com/make-os/kit/crypto/ed25519"
	"github.com/make-os/kit/util"
	"github.com/thoas/go-funk"

	v "github.com/go-ozzo/ozzo-validation"
	"github.com/make-os/kit/params"
	"github.com/shopspring/decimal"
)

// CheckRecipient validates the recipient address
func CheckRecipient(tx *txns.TxRecipient, index int) error {

	recipient := tx.To

	// If unset, return error
	if tx.To.IsEmpty() {
		return feI(index, "to", "recipient address is required")
	}

	// Ok if it is a valid full native namespace for a repo
	if tx.To.IsNativeRepoAddress() || tx.To.IsNativeUserAddress() && tx.To.IsValidNativeAddress() {
		return nil
	}

	// Ok if it is a full namespace but not a native namespace of users (a/...)
	if tx.To.IsFullNamespace() && !tx.To.IsNativeUserAddress() {
		return nil
	}

	// Ok if it is a regular user address
	if recipient.IsUserAddress() {
		return nil
	}

	return feI(index, "to", "recipient address is not valid")
}

func checkValue(tx *txns.TxValue, index int) error {
	if err := v.Validate(tx.Value, v.Required.Error(feI(index, "value",
		"value is required").Error()), v.By(validValueRule("value", index)),
	); err != nil {
		return err
	}
	return nil
}

func checkDescription(tx *txns.TxDescription, required bool, index int) error {

	if required {
		if err := v.Validate(tx.Description,
			v.Required.Error(feI(index, "desc", "requires a description").Error()),
		); err != nil {
			return err
		}
	}

	if len(tx.Description) > params.TxRepoCreateMaxCharDesc {
		return feI(index, "desc", fmt.Sprintf("description length cannot be greater than %d",
			params.TxRepoCreateMaxCharDesc))
	}

	return nil
}

func checkPositiveValue(tx *txns.TxValue, index int) error {
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

func checkType(tx *txns.TxType, expected types.TxCode, index int) error {
	if !tx.Is(expected) {
		return feI(index, "type", "type is invalid")
	}
	return nil
}

func CheckCommon(tx types.BaseTx, index int) error {

	if err := v.Validate(tx.GetNonce(),
		v.Required.Error(feI(index, "nonce", "nonce is required").Error())); err != nil {
		return err
	}

	var baseFee, txSize decimal.Decimal
	if err := v.Validate(tx.GetFee(),
		v.Required.Error(feI(index, "fee", "fee is required").Error()),
		v.By(validValueRule("fee", index)),
	); err != nil {
		return err
	}

	var freeTx []types.TxCode // add tx type that should be excluded for paying fees
	if !funk.Contains(freeTx, tx.GetType()) {

		// Fee must be at least equal to the base fee
		txSize = decimal.NewFromFloat(float64(tx.GetEcoSize()))
		baseFee = params.FeePerByte.Mul(txSize)
		if tx.GetFee().Decimal().LessThan(baseFee) {
			return errors2.FieldErrorWithIndex(index, "fee",
				fmt.Sprintf("fee cannot be lower than the base price of %s", baseFee.StringFixed(4)))
		}
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
func CheckTxCoinTransfer(tx *txns.TxCoinTransfer, index int) error {

	if err := checkType(tx.TxType, txns.TxTypeCoinTransfer, index); err != nil {
		return err
	}

	if err := CheckRecipient(tx.TxRecipient, index); err != nil {
		return err
	}

	if err := checkValue(tx.TxValue, index); err != nil {
		return err
	}

	if err := CheckCommon(tx, index); err != nil {
		return err
	}

	return nil
}

// CheckTxTicketPurchase performs sanity checks on TxTicketPurchase
func CheckTxTicketPurchase(tx *txns.TxTicketPurchase, index int) error {

	if tx.Type != txns.TxTypeHostTicket {
		return feI(index, "type", "type is invalid")
	}

	if err := checkPositiveValue(tx.TxValue, index); err != nil {
		return err
	}

	// Non-delegate host ticket value must reach the minimum stake
	if tx.Is(txns.TxTypeHostTicket) && tx.Delegate.IsEmpty() {
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

	if tx.Is(txns.TxTypeHostTicket) {
		if len(tx.BLSPubKey) == 0 {
			return feI(index, "blsPubKey", "BLS public key is required")
		}
		if len(tx.BLSPubKey) != 128 {
			return feI(index, "blsPubKey", "BLS public key length is invalid")
		}
	}

	if err := CheckCommon(tx, index); err != nil {
		return err
	}

	return nil
}

// CheckTxUnbondTicket performs sanity checks on TxTicketUnbond
func CheckTxUnbondTicket(tx *txns.TxTicketUnbond, index int) error {

	if err := checkType(tx.TxType, txns.TxTypeHostTicket, index); err != nil {
		return err
	}

	if tx.TicketHash.IsEmpty() {
		return feI(index, "ticket", "ticket id is required")
	}

	if err := CheckCommon(tx, index); err != nil {
		return err
	}

	return nil
}

// CheckRepoConfig validates a repo configuration object
func CheckRepoConfig(cfg map[string]interface{}, index int) error {

	// Check if new config can successfully merge into RepoConfig structure without issue
	base := state.MakeDefaultRepoConfig()
	if err := base.Merge(cfg); err != nil {
		return errors.Wrap(err, "dry merge failed")
	}

	govCfg := base.Gov
	sf := fmt.Sprintf

	// Ensure the voter type is known
	if !state.IsValidVoterType(govCfg.Voter) {
		return feI(index, "governance.propVoter", sf("unknown value"))
	}

	// Ensure the proposal creator type is known
	if !state.IsValidProposalCreatorType(govCfg.PropCreator) {
		return feI(index, "governance.propCreator", sf("unknown value"))
	}

	// Ensure the proposer tally method is known
	if !state.IsValidProposalTallyMethod(govCfg.PropTallyMethod) {
		return feI(index, "governance.propTallyMethod", sf("unknown value"))
	}

	// Ensure the refund type method is known
	if !state.IsValidPropFeeRefundTypeType(govCfg.PropFeeRefundType) {
		return feI(index, "governance.propFeeRefundType", sf("unknown value"))
	}

	propDur, err := util.PtrStrToFloatE(govCfg.PropDuration)
	if err != nil || propDur < 0 {
		return feI(index, "governance.propDur", sf("must be a non-negative number"))
	}

	propQuorum, err := util.PtrStrToFloatE(govCfg.PropQuorum)
	if err != nil || propQuorum < 0 {
		return feI(index, "governance.propQuorum", sf("must be a non-negative number"))
	}

	propThreshold, err := util.PtrStrToFloatE(govCfg.PropThreshold)
	if err != nil || propThreshold < 0 {
		return feI(index, "governance.propThreshold", sf("must be a non-negative number"))
	}

	propVetoQuorum, err := util.PtrStrToFloatE(govCfg.PropVetoQuorum)
	if err != nil || propVetoQuorum < 0 {
		return feI(index, "governance.propVetoQuorum", sf("must be a non-negative number"))
	}

	propVetoOwnersQuorum, err := util.PtrStrToFloatE(govCfg.PropVetoOwnersQuorum)
	if err != nil || propVetoOwnersQuorum < 0 {
		return feI(index, "governance.propVetoOwnersQuorum", sf("must be a non-negative number"))
	}

	propFeeDepDur, err := util.PtrStrToFloatE(govCfg.PropFeeDepositDur)
	if err != nil || propFeeDepDur < 0 {
		return feI(index, "governance.propFeeDepDur", sf("must be a non-negative number"))
	}

	propFee, err := util.PtrStrToFloatE(govCfg.PropFee)
	if err != nil || propFee < 0 {
		return feI(index, "governance.propFee", sf("must be a non-negative number"))
	} else if propFee < params.DefaultMinProposalFee {
		return feI(index, "governance.propFee", sf("cannot be lower than network minimum"))
	}

	// When proposer is ProposerOwner, tally method cannot be CoinWeighted or Identity
	tallyMethod := govCfg.PropTallyMethod
	isNotOwnerProposer := pointer.GetInt(govCfg.Voter) != pointer.GetInt(state.VoterOwner.Ptr())
	if isNotOwnerProposer {
		if *tallyMethod == *state.ProposalTallyMethodCoinWeighted.Ptr() || *tallyMethod == *state.ProposalTallyMethodIdentity.Ptr() {
			return feI(index, "config", "when proposer type is not `ProposerOwner`, tally methods "+
				"`CoinWeighted` and `Identity` are not allowed")
		}
	}

	return nil
}

// CheckTxRepoCreate performs sanity checks on TxRepoCreate
func CheckTxRepoCreate(tx *txns.TxRepoCreate, index int) error {
	if err := checkType(tx.TxType, txns.TxTypeRepoCreate, index); err != nil {
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

	if err := checkDescription(tx.TxDescription, true, index); err != nil {
		return err
	}

	if err := CheckRepoConfig(tx.Config.ToMap(), index); err != nil {
		return err
	}

	if err := CheckCommon(tx, index); err != nil {
		return err
	}

	return nil
}

// CheckTxRegisterPushKey performs sanity checks on TxRegisterPushKey
func CheckTxRegisterPushKey(tx *txns.TxRegisterPushKey, index int) error {

	if err := checkType(tx.TxType, txns.TxTypeRegisterPushKey, index); err != nil {
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
	if err := CheckScopes(tx.Scopes, index); err != nil {
		return err
	}

	// If fee cap is set, validate it
	if !tx.FeeCap.Empty() {
		if err := checkFeeCap(tx.FeeCap, index); err != nil {
			return err
		}
	}

	if err := CheckCommon(tx, index); err != nil {
		return err
	}

	return nil
}

// CheckScopes checks a list of strings intended to be used as push key scopes.
func CheckScopes(scopes []string, index int) error {
	for i, s := range scopes {
		if !identifier.IsValidScope(s) {
			msg := "scope is invalid. Expected a namespace path or repository name"
			return feI(index, fmt.Sprintf("scopes[%d]", i), msg)
		}
	}
	return nil
}

// CheckTxUpDelPushKey performs sanity checks on TxRegisterPushKey
func CheckTxUpDelPushKey(tx *txns.TxUpDelPushKey, index int) error {

	if err := checkType(tx.TxType, txns.TxTypeUpDelPushKey, index); err != nil {
		return err
	}

	if tx.ID == "" {
		return feI(index, "id", "push key id is required")
	} else if !crypto2.IsValidPushAddr(tx.ID) {
		return feI(index, "id", "push key id is not valid")
	}

	// If there are scope entries, ensure only namespaces URI,
	// repo names and non-address entries are contained in the list
	if err := CheckScopes(tx.AddScopes, index); err != nil {
		return err
	}

	// If fee cap is set, validate it
	if !tx.FeeCap.Empty() {
		if err := checkFeeCap(tx.FeeCap, index); err != nil {
			return err
		}
	}

	if err := CheckCommon(tx, index); err != nil {
		return err
	}

	return nil
}

// CheckTxSetDelegateCommission performs sanity checks on TxSetDelegateCommission
func CheckTxSetDelegateCommission(tx *txns.TxSetDelegateCommission, index int) error {

	if err := checkType(tx.TxType, txns.TxTypeSetDelegatorCommission, index); err != nil {
		return err
	}

	if err := v.Validate(tx.Commission,
		v.Required.Error(feI(index, "commission", "commission rate is required").Error()),
	); err != nil {
		return err
	}

	if tx.Commission.Decimal().LessThan(params.MinDelegatorCommission) {
		minPct := params.MinDelegatorCommission.String()
		return feI(index, "commission", "rate cannot be below the minimum ("+minPct+" percent)")
	}

	if tx.Commission.Decimal().GreaterThan(decimal.NewFromFloat(100)) {
		return feI(index, "commission", "commission rate cannot exceed 100 percent")
	}

	if err := CheckCommon(tx, index); err != nil {
		return err
	}

	return nil
}

// CheckTxPush performs sanity checks on TxPush
func CheckTxPush(tx *txns.TxPush, index int) error {

	// Expect transaction type to be TxTypePush
	if err := checkType(tx.TxType, txns.TxTypePush, index); err != nil {
		return err
	}

	// The push note object must be set
	if tx.Note == nil {
		return feI(index, "note", "push note is required")
	}

	// Perform sanity check on the push note
	if err := validation.CheckPushNoteSanity(tx.Note); err != nil {
		return err
	}

	// Expect the number of endorsements to equal the number of endorsers quorum size
	if len(tx.Endorsements) < params.PushEndorseQuorumSize {
		return feI(index, "endorsements", "not enough endorsements included")
	}

	// Check each endorsements
	senders := map[string]struct{}{}
	for index, end := range tx.Endorsements {

		// Perform sanity checks on the endorsement.
		// Indicate that the endorsement is from a push transaction.
		if err := validation.CheckEndorsementSanity(end, true, index); err != nil {
			return err
		}

		// Make sure we haven't seen an endorsement from this sender before
		_, ok := senders[end.EndorserPubKey.HexStr()]
		if !ok {
			senders[end.EndorserPubKey.HexStr()] = struct{}{}
		} else {
			return feI(index, "endorsements.pubKey", "multiple endorsement by a single sender not permitted")
		}
	}

	return nil
}

// CheckNamespaceDomains checks namespace domains and targets
func CheckNamespaceDomains(domains map[string]string, index int) error {
	for domain, target := range domains {
		if identifier.IsValidResourceNameNoMinLen(domain) != nil {
			return feI(index, "domains", fmt.Sprintf("domains.%s: name is invalid", domain))
		}
		if !identifier.IsWholeNativeURI(target) {
			return feI(index, "domains", fmt.Sprintf("domains.%s: target is invalid", domain))
		}
		if target[:2] == "a/" && ed25519.IsValidUserAddr(target[2:]) != nil {
			return feI(index, "domains", fmt.Sprintf("domains.%s: target is not a valid address",
				domain))
		}
	}
	return nil
}

// CheckTxNamespaceAcquire performs sanity checks on TxNamespaceRegister
func CheckTxNamespaceAcquire(tx *txns.TxNamespaceRegister, index int) error {

	if err := checkType(tx.TxType, txns.TxTypeNamespaceRegister, index); err != nil {
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

	if tx.To != "" {
		if ed25519.IsValidUserAddr(tx.To) != nil && identifier.IsValidResourceName(tx.To) != nil {
			return feI(index, "to", "invalid value. Expected a user address or a repository name")
		}
	}

	if !tx.Value.Decimal().Equal(params.NamespaceRegFee) {
		return feI(index, "value", fmt.Sprintf("invalid value; has %s, want %s",
			tx.Value, params.NamespaceRegFee.String()))
	}

	if len(tx.Domains) > 0 {
		if err := CheckNamespaceDomains(tx.Domains, index); err != nil {
			return err
		}
	}

	if err := CheckCommon(tx, index); err != nil {
		return err
	}

	return nil
}

// CheckTxNamespaceDomainUpdate performs sanity checks on TxNamespaceDomainUpdate
func CheckTxNamespaceDomainUpdate(tx *txns.TxNamespaceDomainUpdate, index int) error {

	if err := checkType(tx.TxType, txns.TxTypeNamespaceDomainUpdate, index); err != nil {
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

	if err := CheckCommon(tx, index); err != nil {
		return err
	}

	return nil
}

// CheckTxRepoProposalUpsertOwner performs sanity checks on TxRepoProposalUpsertOwner
func CheckTxRepoProposalUpsertOwner(tx *txns.TxRepoProposalUpsertOwner, index int) error {

	if err := checkType(tx.TxType, txns.TxTypeRepoProposalUpsertOwner, index); err != nil {
		return err
	}

	if err := checkRepoName(tx.RepoName, index); err != nil {
		return err
	}

	if err := CheckProposalID(tx.ID, false, index); err != nil {
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

	if err := CheckCommon(tx, index); err != nil {
		return err
	}

	return nil
}

// CheckTxVote performs sanity checks on TxRepoProposalVote
func CheckTxVote(tx *txns.TxRepoProposalVote, index int) error {

	if err := checkType(tx.TxType, txns.TxTypeRepoProposalVote, index); err != nil {
		return err
	}

	if err := checkRepoName(tx.RepoName, index); err != nil {
		return err
	}

	if err := CheckProposalID(tx.ProposalID, true, index); err != nil {
		return err
	}

	// Vote cannot be less than -1 or greater than 1.
	// 0 = No, 1 = Yes, -1 = NoWithVeto, -2 = Abstain
	if tx.Vote < -2 || tx.Vote > 1 {
		return feI(index, "vote", "vote choice is unknown")
	}

	if err := CheckCommon(tx, index); err != nil {
		return err
	}

	return nil
}

// CheckTxRepoProposalSendFee performs sanity checks on TxRepoProposalSendFee
func CheckTxRepoProposalSendFee(tx *txns.TxRepoProposalSendFee, index int) error {

	if err := checkType(tx.TxType, txns.TxTypeRepoProposalSendFee, index); err != nil {
		return err
	}

	if err := checkRepoName(tx.RepoName, index); err != nil {
		return err
	}

	if err := CheckProposalID(tx.ID, false, index); err != nil {
		return err
	}

	if err := checkValue(&txns.TxValue{Value: tx.Value}, index); err != nil {
		return err
	}

	if err := CheckCommon(tx, index); err != nil {
		return err
	}

	return nil
}

// CheckProposalID performs sanity checks of a proposal id
func CheckProposalID(id string, allowPrefix bool, index int) error {

	// If allowPrefix is set to true, strip out system proposal prefixes
	// like MR, which is used to namespace repo proposals
	if allowPrefix {
		if strings.HasPrefix(strings.ToUpper(id), "MR") {
			id = id[2:]
		}
	}

	if id == "" {
		return feI(index, "id", "proposal id is required")
	} else if !govalidator.IsNumeric(id) {
		return feI(index, "id", "proposal id is not valid")
	} else if len(id) > 16 {
		return feI(index, "id", "proposal ID exceeded 16 characters limit")
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
	if err := checkValue(&txns.TxValue{Value: fee}, index); err != nil {
		return err
	} else if fee.Decimal().LessThan(decimal.NewFromFloat(params.DefaultMinProposalFee)) {
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

// CheckTxRepoProposalUpdate performs sanity checks on TxRepoProposalUpdate
func CheckTxRepoProposalUpdate(tx *txns.TxRepoProposalUpdate, index int) error {

	if err := checkType(tx.TxType, txns.TxTypeRepoProposalUpdate, index); err != nil {
		return err
	}

	if err := checkRepoName(tx.RepoName, index); err != nil {
		return err
	}

	if err := CheckProposalID(tx.ID, false, index); err != nil {
		return err
	}

	if err := checkProposalFee(tx.Value, index); err != nil {
		return err
	}

	if len(tx.Config) == 0 && len(tx.Description) == 0 {
		return feI(index, "config|desc", "set either `desc` or `config` fields")
	}

	if err := checkDescription(tx.TxDescription, false, index); err != nil {
		return err
	}

	if err := CheckRepoConfig(tx.Config, index); err != nil {
		return err
	}

	if err := CheckCommon(tx, index); err != nil {
		return err
	}

	return nil
}

// CheckTxRepoProposalRegisterPushKey performs sanity checks on TxRepoProposalRegisterPushKey
func CheckTxRepoProposalRegisterPushKey(tx *txns.TxRepoProposalRegisterPushKey, index int) error {

	if err := checkType(tx.TxType, txns.TxTypeRepoProposalRegisterPushKey, index); err != nil {
		return err
	}

	if err := checkRepoName(tx.RepoName, index); err != nil {
		return err
	}

	if err := CheckProposalID(tx.ID, false, index); err != nil {
		return err
	}

	if err := checkProposalFee(tx.Value, index); err != nil {
		return err
	}

	if len(tx.PushKeys) == 0 {
		return feI(index, "keys", "push key is required")
	}

	// Ensure all push key ids are unique and valid
	found := map[string]struct{}{}
	for _, pkID := range tx.PushKeys {
		if !crypto2.IsValidPushAddr(pkID) {
			return feI(index, "keys", fmt.Sprintf("push key id (%s) is not valid", pkID))
		}
		if _, ok := found[pkID]; ok {
			return feI(index, "keys", fmt.Sprintf("push key id (%s) is a duplicate", pkID))
		}
		found[pkID] = struct{}{}
	}

	// Ensure fee mode is valid
	validFeeModes := []state.FeeMode{state.FeeModePusherPays, state.FeeModeRepoPays, state.FeeModeRepoPaysCapped}
	if !funk.Contains(validFeeModes, tx.FeeMode) {
		return feI(index, "feeMode", "fee mode is unknown")
	}

	// If fee mode is FeeModeRepoPaysCapped, ensure FeeCap is set and non-zero.
	if tx.FeeMode == state.FeeModeRepoPaysCapped {
		if err := checkFeeCap(tx.FeeCap, index); err != nil {
			return err
		}
	} else {
		if !tx.FeeCap.IsZero() {
			return feI(index, "feeCap", "value not expected for the chosen fee mode")
		}
	}

	// When namespace target is set, ensure a valid namespace name is provided.
	// If valid and NamespaceOnly is set, return an error
	if tx.Namespace != "" {
		if identifier.IsValidResourceName(tx.Namespace) != nil {
			return feI(index, "namespace", "value format is not valid")
		}
		if tx.NamespaceOnly != "" {
			return feI(index, "namespaceOnly", "field is not expected because 'namespace' is set")
		}
	}

	// When namespaceOnly target is set, ensure a valid namespace name is provided.
	if tx.NamespaceOnly != "" {
		if identifier.IsValidResourceName(tx.NamespaceOnly) != nil {
			return feI(index, "namespaceOnly", "value format is not valid")
		}
	}

	if err := CheckCommon(tx, index); err != nil {
		return err
	}

	return nil
}
