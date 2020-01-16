package validators

import (
	"fmt"
	"regexp"

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

	if tx.Is(types.TxTypeEpochSeed) {
		goto pub_sig_check
	}

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

pub_sig_check:

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

	if tx.GetType() == types.TxTypeStorerTicket {
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

// CheckTxEpochSeed performs sanity checks on TxEpochSeed
func CheckTxEpochSeed(tx *types.TxEpochSeed, index int) error {

	if err := checkType(tx.TxType, types.TxTypeEpochSeed, index); err != nil {
		return err
	}

	if tx.Output.IsEmpty() {
		return feI(index, "output", "output is required")
	}

	prooflen := len(tx.Proof)
	if prooflen == 0 {
		return feI(index, "proof", "proof is required")
	} else if prooflen != 96 {
		return feI(index, "proof", "proof length is invalid")
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

	pushOKSenders := map[string]struct{}{}
	pushOKRefHashesID := util.EmptyBytes32
	for _, pushOK := range tx.PushOKs {

		// Ensure PushOKs have same sender
		_, hasSender := pushOKSenders[pushOK.SenderPubKey.HexStr()]
		if !hasSender {
			pushOKSenders[pushOK.SenderPubKey.HexStr()] = struct{}{}
		} else {
			if _, ok := pushOKSenders[pushOK.SenderPubKey.HexStr()]; ok {
				return feI(index, "endorsements.senderPubKey", "multiple endorsement by a "+
					"single sender not permitted")
			}
		}

		// Ensure push note id and the target pushOK push note id match
		if !pushOK.PushNoteID.Equal(tx.PushNote.ID()) {
			return feI(index, "endorsements.pushNoteID", "value does not match push tx note id")
		}

		spk, err := crypto.PubKeyFromBytes(pushOK.SenderPubKey.Bytes())
		if err != nil {
			return feI(index, "endorsements.senderPubKey", "public key is not valid")
		}

		ok, err := spk.Verify(pushOK.BytesNoSig(), pushOK.Sig.Bytes())
		if err != nil || !ok {
			if !ok {
				return feI(index, "endorsements.sig", "signature is invalid")
			}
			return feI(index, "endorsements.sig", "failed to verify signature")
		}

		// Ensure the references hashes are all the same
		if pushOKRefHashesID.IsEmpty() {
			pushOKRefHashesID = pushOK.ReferencesHash.ID()
		} else {
			if !pushOK.ReferencesHash.ID().Equal(pushOKRefHashesID) {
				return feI(index, "endorsements.refsHash", "varied references hash; push "+
					"endorsements can't have unmatched hashes for references")
			}
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
