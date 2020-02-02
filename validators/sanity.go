package validators

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/asaskevich/govalidator"
	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/repo"
	"github.com/makeos/mosdef/util"
	"github.com/thoas/go-funk"

	v "github.com/go-ozzo/ozzo-validation"
	"github.com/makeos/mosdef/params"
	"github.com/makeos/mosdef/types"
	"github.com/shopspring/decimal"
)

var domainTargetFormat = "[a-zA-Z0-9_-]+/[a-zA-Z0-9_-]+"

func checkRecipient(tx *types.TxRecipient, index int) error {
	if err := v.Validate(tx.To,
		v.Required.Error(feI(index, "to", "recipient address is required").Error()),
		v.By(validAddrRule(feI(index, "to", "recipient address is not valid"))),
	); err != nil {
		return err
	}
	return nil
}

func checkValue(tx *types.TxValue, index int) error {
	if err := v.Validate(tx.Value, v.Required.Error(feI(index, "value",
		"value is required").Error()), v.By(validValueRule("value", index)),
	); err != nil {
		return err
	}
	return nil
}

func checkType(tx *types.TxType, expected int, index int) error {
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
		return types.FieldErrorWithIndex(index, "fee",
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
func CheckTxCoinTransfer(tx *types.TxCoinTransfer, index int) error {

	if err := checkType(tx.TxType, types.TxTypeCoinTransfer, index); err != nil {
		return err
	}

	if err := checkRecipient(tx.TxRecipient, index); err != nil {
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
func CheckTxTicketPurchase(tx *types.TxTicketPurchase, index int) error {

	if tx.Type != types.TxTypeValidatorTicket && tx.Type != types.TxTypeStorerTicket {
		return feI(index, "type", "type is invalid")
	}

	if err := checkValue(tx.TxValue, index); err != nil {
		return err
	}

	if tx.Is(types.TxTypeStorerTicket) {
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

	if tx.Is(types.TxTypeStorerTicket) {
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
func CheckTxUnbondTicket(tx *types.TxTicketUnbond, index int) error {

	if err := checkType(tx.TxType, types.TxTypeStorerTicket, index); err != nil {
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

// CheckTxRepoCreate performs sanity checks on TxRepoCreate
func CheckTxRepoCreate(tx *types.TxRepoCreate, index int) error {

	if err := checkType(tx.TxType, types.TxTypeRepoCreate, index); err != nil {
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

	if err := checkCommon(tx, index); err != nil {
		return err
	}

	return nil
}

// CheckTxAddGPGPubKey performs sanity checks on TxAddGPGPubKey
func CheckTxAddGPGPubKey(tx *types.TxAddGPGPubKey, index int) error {

	if err := checkType(tx.TxType, types.TxTypeAddGPGPubKey, index); err != nil {
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
func CheckTxSetDelegateCommission(tx *types.TxSetDelegateCommission, index int) error {

	if err := checkType(tx.TxType, types.TxTypeSetDelegatorCommission, index); err != nil {
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
		return types.FieldErrorWithIndex(index, "commission", "commission rate cannot exceed 100%%")
	}

	if err := checkCommon(tx, index); err != nil {
		return err
	}

	return nil
}

// CheckTxPush performs sanity checks on TxPush
func CheckTxPush(tx *types.TxPush, index int) error {

	if err := checkType(tx.TxType, types.TxTypePush, index); err != nil {
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

// CheckTxNSAcquire performs sanity checks on TxNamespaceAcquire
func CheckTxNSAcquire(tx *types.TxNamespaceAcquire, index int) error {

	if err := checkType(tx.TxType, types.TxTypeNSAcquire, index); err != nil {
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
		validTargetTypes := []string{"r/", "a/"}
		for i, target := range tx.Domains {
			if !regexp.MustCompile(domainTargetFormat).MatchString(target) {
				return feI(index, "domains", fmt.Sprintf("domains.%s target format is invalid", i))
			}
			if !funk.ContainsString(validTargetTypes, target[:2]) {
				return feI(index, "domains", fmt.Sprintf("domains.%s has unknown target type", i))
			}
			if target[:2] == "a/" && crypto.IsValidAddr(target[2:]) != nil {
				return feI(index, "domains", fmt.Sprintf("domains.%s has invalid address", i))
			}
		}
	}

	if err := checkCommon(tx, index); err != nil {
		return err
	}

	return nil
}

// CheckTxNamespaceDomainUpdate performs sanity checks on TxNamespaceDomainUpdate
func CheckTxNamespaceDomainUpdate(tx *types.TxNamespaceDomainUpdate, index int) error {

	if err := checkType(tx.TxType, types.TxTypeNSDomainUpdate, index); err != nil {
		return err
	}

	if err := v.Validate(tx.Name,
		v.Required.Error(feI(index, "name", "requires a name").Error()),
		v.By(validObjectNameRule("name", index)),
	); err != nil {
		return err
	}

	if len(tx.Domains) > 0 {
		validTargetTypes := []string{"r/", "a/"}
		for i, target := range tx.Domains {
			if target == "" {
				continue
			}
			if !regexp.MustCompile(domainTargetFormat).MatchString(target) {
				return feI(index, "domains", fmt.Sprintf("domains.%s target format is invalid", i))
			}
			if !funk.ContainsString(validTargetTypes, target[:2]) {
				return feI(index, "domains", fmt.Sprintf("domains.%s has unknown target type", i))
			}
			if target[:2] == "a/" && crypto.IsValidAddr(target[2:]) != nil {
				return feI(index, "domains", fmt.Sprintf("domains.%s has invalid address", i))
			}
		}
	}

	if err := checkCommon(tx, index); err != nil {
		return err
	}

	return nil
}

// CheckTxRepoProposalUpsertOwner performs sanity checks on TxRepoProposalUpsertOwner
func CheckTxRepoProposalUpsertOwner(tx *types.TxRepoProposalUpsertOwner, index int) error {

	if err := checkType(tx.TxType, types.TxTypeRepoProposalUpsertOwner, index); err != nil {
		return err
	}

	if err := v.Validate(tx.RepoName,
		v.Required.Error(feI(index, "name", "repo name is required").Error()),
		v.By(validObjectNameRule("name", index)),
	); err != nil {
		return err
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

// CheckTxRepoProposalVote performs sanity checks on TxRepoProposalVote
func CheckTxRepoProposalVote(tx *types.TxRepoProposalVote, index int) error {

	if err := checkType(tx.TxType, types.TxTypeRepoProposalVote, index); err != nil {
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

// CheckTxRepoProposalUpdate performs sanity checks on TxRepoProposalUpdate
func CheckTxRepoProposalUpdate(tx *types.TxRepoProposalUpdate, index int) error {

	if err := checkType(tx.TxType, types.TxTypeRepoProposalUpdate, index); err != nil {
		return err
	}

	if err := v.Validate(tx.RepoName,
		v.Required.Error(feI(index, "name", "repo name is required").Error()),
		v.By(validObjectNameRule("name", index)),
	); err != nil {
		return err
	}

	if err := checkCommon(tx, index); err != nil {
		return err
	}

	return nil
}
