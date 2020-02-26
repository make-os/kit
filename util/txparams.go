package util

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/asaskevich/govalidator"
)

const TxParamsPrefix = "tx:"

// TxParams errors
var (
	ErrTxParamsNotFound = fmt.Errorf("txparams was not set")
)

// RemoveTxParams removes all lines beginning with a 'TxParams' prefix 'tx'.
// NOTE: It is case-sensitive.
func RemoveTxParams(msg string) string {
	lines := strings.Split(msg, "\n")
	newMsg := ""
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if !strings.HasPrefix(line, "tx:") {
			newMsg += line
			if (i + 1) != len(lines) {
				newMsg += "\n"
			}
		}
	}
	return newMsg
}

// TxParams represents transaction information usually included in commits, notes
// and tag objects
type TxParams struct {
	Fee             String // Network fee to be paid for update to the target ref
	Nonce           uint64 // Nonce of the account paying the network fee and signing the update.
	PubKeyID        string // The GPG public key ID of the reference updater.
	Signature       string // The signature of the update (only used in note signing for now)
	DeleteRef       bool   // A directive to delete the current/pushed reference.
	MergeProposalID string // A directive to handle a pushed branch based on the constraints defined in a merge proposal
}

// GetNonceString returns the nonce as a string
func (tl *TxParams) GetNonceString() string {
	return strconv.FormatUint(tl.Nonce, 10)
}

func (tl *TxParams) String() string {
	nonceStr := strconv.FormatUint(tl.Nonce, 10)
	return MakeTxParams(tl.Fee.String(), nonceStr, tl.PubKeyID, []byte(tl.Signature))
}

// IsZeroString returns true if str is empty or equal "0"
func IsZeroString(str string) bool {
	return str == "" || str == "0"
}

// MakeTxParams returns a well formatted txparams string
func MakeTxParams(txFee, txNonce, gpgID string, sig []byte, directives ...string) string {
	str := fmt.Sprintf("tx: fee=%s, nonce=%s, gpgID=%s", txFee, txNonce, gpgID)
	for _, a := range directives {
		str = str + fmt.Sprintf(", %s", a)
	}
	if sig != nil {
		str = str + fmt.Sprintf(", sig=%s", ToHex(sig))
	}
	return str
}

// MakeAndValidateTxParams is like MakeTxParams but also validates it
func MakeAndValidateTxParams(
	txFee,
	txNonce,
	gpgID string,
	sig []byte,
	directives ...string) (string, error) {
	str := fmt.Sprintf("tx: fee=%s, nonce=%s, gpgID=%s", txFee, txNonce, gpgID)
	for _, a := range directives {
		str = str + fmt.Sprintf(", %s", a)
	}
	if sig != nil {
		str = str + fmt.Sprintf(", sig=%s", ToHex(sig))
	}

	if _, err := ParseTxParams(str); err != nil {
		return "", err
	}

	return str, nil
}

// ParseTxParams finds, parses and returns the txparams found in the given msg.
// Returns ErrTxParamsNotFound if no txparams in the message
func ParseTxParams(msg string) (*TxParams, error) {
	lines := strings.Split(msg, "\n")
	txparams := ""

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.HasPrefix(line, TxParamsPrefix) {
			txparams = line
		}
	}

	if txparams == "" {
		return nil, ErrTxParamsNotFound
	}

	kvData := strings.Fields(strings.TrimSpace(txparams[3:]))
	sort.Strings(kvData)

	var txParams = new(TxParams)
	for _, kv := range kvData {
		kv = strings.TrimRight(strings.TrimSpace(kv), ",")
		kvParts := strings.Split(kv, "=")

		if kvParts[0] == "fee" {
			if !govalidator.IsFloat(kvParts[1]) {
				return nil, fmt.Errorf("field:fee, msg: fee must be numeric")
			}
			txParams.Fee = String(kvParts[1])
		}

		if kvParts[0] == "nonce" {
			nonce, err := strconv.ParseUint(kvParts[1], 10, 64)
			if err != nil {
				return nil, fmt.Errorf("field:nonce, msg: nonce must be an unsigned integer")
			}
			txParams.Nonce = nonce
		}

		if kvParts[0] == "gpgID" {
			if kvParts[1] == "" {
				return nil, fmt.Errorf("field:gpgID, msg: public key id is required")
			}
			if len(kvParts[1]) != 42 || !IsValidRSAPubKey(kvParts[1]) {
				return nil, fmt.Errorf("field:gpgID, msg: public key id is invalid")
			}
			txParams.PubKeyID = kvParts[1]
		}

		if kvParts[0] == "sig" {
			if kvParts[1] == "" {
				return nil, fmt.Errorf("field:sig, msg: signature value is required")
			}
			if kvParts[1][:2] != "0x" {
				return nil, fmt.Errorf("field:sig, msg: signature format is not valid")
			}
			decSig, err := HexToStr(kvParts[1])
			if err != nil {
				return nil, fmt.Errorf("field:sig, msg: signature format is not valid")
			}
			txParams.Signature = decSig
		}

		if kvParts[0] == "deleteRef" {
			txParams.DeleteRef = true
		}

		if kvParts[0] == "mergeID" {
			if len(kvParts) == 1 || kvParts[1] == "" {
				return nil, fmt.Errorf("merge proposal id is required")
			}
			if !govalidator.IsNumeric(kvParts[1]) {
				return nil, fmt.Errorf("merge proposal id format is not valid")
			}
			if len(kvParts[1]) > 8 {
				return nil, fmt.Errorf("merge id limit of 8 bytes exceeded")
			}
			txParams.MergeProposalID = kvParts[1]
		}
	}

	return txParams, nil
}
