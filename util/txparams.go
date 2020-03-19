package util

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/asaskevich/govalidator"
	"github.com/vmihailenco/msgpack"
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
	SerializerHelper
	Fee             String `json:"fee" msgpack:"fee" mapstructure:"fee"`                   // Network fee to be paid for update to the target ref
	Nonce           uint64 `json:"nonce" msgpack:"nonce" mapstructure:"nonce"`             // Nonce of the account paying the network fee and signing the update.
	GPGID           string `json:"gpgID" msgpack:"gpgID" mapstructure:"gpgID"`             // The GPG public key ID of the reference updater.
	Signature       string `json:"sig" msgpack:"sig" mapstructure:"sig"`                   // The signature of the update (only used in note signing for now)
	DeleteRef       bool   `json:"deleteRef" msgpack:"deleteRef" mapstructure:"deleteRef"` // A directive to delete the current/pushed reference.
	MergeProposalID string `json:"mergeID" msgpack:"mergeID" mapstructure:"mergeID"`       // A directive to handle a pushed branch based on the constraints defined in a merge proposal
}

// Bytes returns the serialized equivalent of tp
func (tp *TxParams) Bytes() []byte {
	return ToBytes(tp)
}

// Bytes returns the serialized equivalent of tp
func (tp *TxParams) BytesNoSig() []byte {
	sig := tp.Signature
	tp.Signature = ""
	bz := ToBytes(tp)
	tp.Signature = sig
	return bz
}

func (tp *TxParams) EncodeMsgpack(enc *msgpack.Encoder) error {
	return tp.EncodeMulti(enc, tp.Fee, tp.Nonce, tp.GPGID, tp.Signature, tp.DeleteRef, tp.MergeProposalID)
}

func (tp *TxParams) DecodeMsgpack(dec *msgpack.Decoder) error {
	return tp.DecodeMulti(dec, &tp.Fee, &tp.Nonce, &tp.GPGID, &tp.Signature, &tp.DeleteRef, &tp.MergeProposalID)
}

// GetNonceAsString returns the nonce as a string
func (tp *TxParams) GetNonceAsString() string {
	return strconv.FormatUint(tp.Nonce, 10)
}

func (tp *TxParams) String() string {
	nonceStr := strconv.FormatUint(tp.Nonce, 10)
	return MakeTxParams(tp.Fee.String(), nonceStr, tp.GPGID, []byte(tp.Signature))
}

// MakeTxParams returns a well formatted txparams string
func MakeTxParams(txFee, txNonce, gpgID string, sig []byte, directives ...string) string {
	str := fmt.Sprintf("tx: fee=%s, nonce=%s, gpgID=%s", txFee, txNonce, gpgID)
	for _, a := range directives {
		str = str + fmt.Sprintf(", %s", a)
	}
	if len(sig) > 0 {
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
	directives ...string) (*TxParams, error) {
	str := fmt.Sprintf("tx: fee=%s, nonce=%s, gpgID=%s", txFee, txNonce, gpgID)
	for _, a := range directives {
		str = str + fmt.Sprintf(", %s", a)
	}
	if sig != nil {
		str = str + fmt.Sprintf(", sig=%s", ToHex(sig))
	}

	txParams, err := ExtractTxParams(str)
	if err != nil {
		return nil, err
	}

	return txParams, nil
}

// ExtractTxParams finds, parses and returns the txparams found in the given msg.
// Returns ErrTxParamsNotFound if no txparams in the message
func ExtractTxParams(msg string) (*TxParams, error) {
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
				return nil, fieldError("fee", "fee must be numeric")
			}
			txParams.Fee = String(kvParts[1])
			continue
		}

		if kvParts[0] == "nonce" {
			nonce, err := strconv.ParseUint(kvParts[1], 10, 64)
			if err != nil {
				return nil, fieldError("nonce", "nonce must be an unsigned integer")
			}
			txParams.Nonce = nonce
			continue
		}

		if kvParts[0] == "gpgID" {
			if kvParts[1] == "" {
				return nil, fieldError("gpgID", "gpg key id is required")
			}
			if len(kvParts[1]) != 42 || !IsValidPushKeyID(kvParts[1]) {
				return nil, fieldError("gpgID", "gpg key id is invalid")
			}
			txParams.GPGID = kvParts[1]
			continue
		}

		if kvParts[0] == "sig" {
			if kvParts[1] == "" {
				return nil, fieldError("sig", "signature value is required")
			}
			if kvParts[1][:2] != "0x" {
				return nil, fieldError("sig", "signature format is not valid")
			}
			decSig, err := HexToStr(kvParts[1])
			if err != nil {
				return nil, fieldError("sig", "signature format is not valid")
			}
			txParams.Signature = decSig
			continue
		}

		if kvParts[0] == "deleteRef" {
			txParams.DeleteRef = true
			continue
		}

		if kvParts[0] == "mergeID" {
			if len(kvParts) == 1 || kvParts[1] == "" {
				return nil, fieldError("mergeID", "merge proposal id is required")
			}
			if !govalidator.IsNumeric(kvParts[1]) {
				return nil, fieldError("mergeID", "merge proposal id must be numeric")
			}
			if len(kvParts[1]) > 8 {
				return nil, fieldError("mergeID", "merge proposal id exceeded 8 bytes limit")
			}
			txParams.MergeProposalID = kvParts[1]
			continue
		}

		return nil, fieldError(kvParts[0], "unknown field")
	}

	return txParams, nil
}
