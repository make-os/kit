package types

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/asaskevich/govalidator"
	"github.com/mr-tron/base58"
	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack"
	"gitlab.com/makeos/mosdef/util"
)

const TxDetailPrefix = "tx:"

// TxDetail errors
var (
	ErrTxDetailNotFound = fmt.Errorf("transaction params was not set")
)

// RemoveTxDetail removes all lines beginning with a 'TxDetail' prefix 'tx'.
// NOTE: It is case-sensitive.
func RemoveTxDetail(msg string) string {
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

// SliceOfTxDetailToReferenceTxDetails converts a slice of TxDetail to a map of TxDetail key by their reference
func SliceOfTxDetailToReferenceTxDetails(details []*TxDetail) ReferenceTxDetails {
	refTxDetailsMap := ReferenceTxDetails{}
	for _, detail := range details {
		refTxDetailsMap[detail.Reference] = detail
	}
	return refTxDetailsMap
}

// ReferenceTxDetails represents a collection of transaction details signed
// by same creator push key, creator nonce, targeting same repository
// and repository namespace.
type ReferenceTxDetails map[string]*TxDetail

// Get gets a detail by reference name
func (td ReferenceTxDetails) Get(ref string) *TxDetail {
	return td[ref]
}

// GetPushKeyID returns the push key id of one of the transaction details
func (td ReferenceTxDetails) GetPushKeyID() string {
	for _, detail := range td {
		return detail.PushKeyID
	}
	return ""
}

// GetRepoName returns the target repository name of one of the transaction details
func (td ReferenceTxDetails) GetRepoName() string {
	for _, detail := range td {
		return detail.RepoName
	}
	return ""
}

// GetRepoNamespace returns the target repo namespace of one of the transaction details
func (td ReferenceTxDetails) GetRepoNamespace() string {
	for _, detail := range td {
		return detail.RepoNamespace
	}
	return ""
}

// GetNonce returns the transaction nonce
func (td ReferenceTxDetails) GetNonce() uint64 {
	for _, detail := range td {
		return detail.Nonce
	}
	return 0
}

// TxDetail represents transaction information usually included in commits, notes
// and tag objects
type TxDetail struct {
	util.SerializerHelper
	RepoName        string      `json:"repoName" msgpack:"repoName,omitempty" mapstructure:"repoName"`                // The target repo name
	RepoNamespace   string      `json:"repoNamespace" msgpack:"repoNamespace,omitempty" mapstructure:"repoNamespace"` // The target repo namespace
	Reference       string      `json:"reference" msgpack:"reference,omitempty" mapstructure:"reference"`             // The target reference
	Fee             util.String `json:"fee" msgpack:"fee,omitempty" mapstructure:"fee"`                               // Network fee to be paid for update to the target ref
	Nonce           uint64      `json:"nonce" msgpack:"nonce,omitempty" mapstructure:"nonce"`                         // Nonce of the account paying the network fee and signing the update.
	PushKeyID       string      `json:"pkID" msgpack:"pkID,omitempty" mapstructure:"pkID"`                            // The pusher public key ID.
	Signature       string      `json:"sig" msgpack:"sig,omitempty" mapstructure:"sig"`                               // The signature of the tx parameter
	MergeProposalID string      `json:"mergeID" msgpack:"mergeID,omitempty" mapstructure:"mergeID"`                   // Specifies a merge proposal that the push is meant to fulfil
	Head            string      `json:"head" msgpack:"head,omitempty" mapstructure:"head"`                            // Indicates the HEAD hash of the target reference
}

// MustSignatureAsBytes returns the decoded signature.
// Panics if signature could not be decoded.
func (tp *TxDetail) MustSignatureAsBytes() []byte {
	if tp.Signature == "" {
		return nil
	}
	sig, err := base58.Decode(tp.Signature)
	if err != nil {
		panic(err)
	}
	return sig
}

// Equal checks whether this object is equal to the give object
func (tp *TxDetail) Equal(o *TxDetail) bool {
	return bytes.Equal(tp.BytesNoSig(), o.BytesNoSig())
}

// ToMapForPEMHeader returns a map version of the object suitable for use in a PEM header
func (tp *TxDetail) ToMapForPEMHeader() map[string]string {
	hdr := map[string]string{
		"repoName":      tp.RepoName,
		"repoNamespace": tp.RepoNamespace,
		"reference":     tp.Reference,
		"pkID":          tp.PushKeyID,
		"nonce":         fmt.Sprintf("%d", tp.Nonce),
		"fee":           fmt.Sprintf("%s", tp.Fee),
	}
	if tp.MergeProposalID != "" {
		hdr["mergeID"] = tp.MergeProposalID
	}
	return hdr
}

// TxDetailFromPEMHeader constructs a TxDetail instance from a PEM header
// TODO: add test
func TxDetailFromPEMHeader(hdr map[string]string) (*TxDetail, error) {
	var params = &TxDetail{
		RepoName:      hdr["repoName"],
		RepoNamespace: hdr["repoNamespace"],
		Reference:     hdr["reference"],
		PushKeyID:     hdr["pkID"],
		Fee:           util.String(hdr["fee"]),
	}

	var err error
	params.Nonce, err = strconv.ParseUint(hdr["nonce"], 10, 64)
	if err != nil {
		return nil, errors.Wrap(err, "nonce")
	}

	if mergeID, ok := hdr["mergeID"]; ok {
		params.MergeProposalID = mergeID
	}

	return params, nil
}

// Bytes returns the serialized equivalent of tp
func (tp *TxDetail) Bytes() []byte {
	return util.ToBytes(tp)
}

// Bytes returns the serialized equivalent of tp
func (tp *TxDetail) BytesNoSig() []byte {
	sig := tp.Signature
	tp.Signature = ""
	bz := util.ToBytes(tp)
	tp.Signature = sig
	return bz
}

func (tp *TxDetail) EncodeMsgpack(enc *msgpack.Encoder) error {
	sig := tp.MustSignatureAsBytes()
	return tp.EncodeMulti(enc,
		tp.RepoName,
		tp.RepoNamespace,
		tp.Reference,
		tp.Fee,
		tp.Nonce,
		tp.PushKeyID,
		sig,
		tp.MergeProposalID,
		tp.Head)
}

func (tp *TxDetail) DecodeMsgpack(dec *msgpack.Decoder) (err error) {
	var sig []byte
	err = tp.DecodeMulti(dec,
		&tp.RepoName,
		&tp.RepoNamespace,
		&tp.Reference,
		&tp.Fee,
		&tp.Nonce,
		&tp.PushKeyID,
		&sig,
		&tp.MergeProposalID,
		&tp.Head)
	tp.Signature = base58.Encode(sig)
	return
}

// GetNonceAsString returns the nonce as a string
func (tp *TxDetail) GetNonceAsString() string {
	return strconv.FormatUint(tp.Nonce, 10)
}

// String returns the string equivalent; Panics if error occurs
func (tp *TxDetail) String() string {

	var options []string
	if tp.MergeProposalID != "" {
		options = append(options, fmt.Sprintf("mergeID=%s", tp.MergeProposalID))
	}
	if tp.Head != "" {
		options = append(options, fmt.Sprintf("head=%s", tp.Head))
	}

	// Decode signature
	var err error
	var sig []byte
	if tp.Signature != "" {
		sig, err = base58.Decode(tp.Signature)
		if err != nil {
			panic(err)
		}
	}

	return MakeTxDetail(
		tp.Fee.String(),
		fmt.Sprintf("%d", tp.Nonce),
		tp.PushKeyID,
		sig,
		options...)
}

// MakeTxDetail returns a well formatted txDetail string
func MakeTxDetail(txFee, txNonce, pushKeyID string, sig []byte, options ...string) string {
	str := fmt.Sprintf("tx: fee=%s, nonce=%s, pkID=%s", txFee, txNonce, pushKeyID)
	for _, a := range options {
		str = str + fmt.Sprintf(", %s", a)
	}
	if len(sig) > 0 {
		str = str + fmt.Sprintf(", sig=%s", base58.Encode(sig))
	}
	return str
}

// MakeAndValidateTxDetail is like MakeTxDetail but also validates it
func MakeAndValidateTxDetail(
	txFee,
	txNonce,
	pushKeyID string,
	sig []byte,
	options ...string) (*TxDetail, error) {
	str := MakeTxDetail(txFee, txNonce, pushKeyID, sig, options...)
	txDetail, err := ExtractTxDetail(str)
	if err != nil {
		return nil, err
	}
	return txDetail, nil
}

// ExtractTxDetail finds, parses and returns the txDetail found in the given msg.
// Returns ErrTxDetailNotFound if no txDetail in the message
func ExtractTxDetail(msg string) (*TxDetail, error) {
	lines := strings.Split(msg, "\n")
	txDetailStr := ""

	for i := 0; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], TxDetailPrefix) {
			txDetailStr = lines[i]
		}
	}

	if txDetailStr == "" {
		return nil, ErrTxDetailNotFound
	}

	kvData := strings.Fields(strings.TrimSpace(txDetailStr[3:]))
	sort.Strings(kvData)

	var txDetail = new(TxDetail)
	for _, kv := range kvData {
		kv = strings.TrimRight(strings.TrimSpace(kv), ",")
		kvParts := strings.Split(kv, "=")

		if kvParts[0] == "repoName" {
			if len(kvParts) == 1 {
				return nil, util.FieldError("repoName", "target repo name is required")
			}
			txDetail.RepoName = kvParts[1]
			continue
		}

		if kvParts[0] == "repoNamespace" {
			if len(kvParts) > 1 {
				txDetail.RepoNamespace = kvParts[1]
			}
			continue
		}

		if kvParts[0] == "reference" {
			if len(kvParts) == 1 {
				return nil, util.FieldError("reference", "target reference name is required")
			}
			txDetail.Reference = kvParts[1]
			continue
		}

		if kvParts[0] == "fee" {
			if !govalidator.IsFloat(kvParts[1]) {
				return nil, util.FieldError("fee", "fee must be numeric")
			}
			txDetail.Fee = util.String(kvParts[1])
			continue
		}

		if kvParts[0] == "nonce" {
			nonce, err := strconv.ParseUint(kvParts[1], 10, 64)
			if err != nil {
				return nil, util.FieldError("nonce", "nonce must be an unsigned integer")
			}
			txDetail.Nonce = nonce
			continue
		}

		if kvParts[0] == "pkID" {
			if kvParts[1] == "" {
				return nil, util.FieldError("pkID", "push key id is required")
			}
			if !util.IsValidPushKeyID(kvParts[1]) {
				return nil, util.FieldError("pkID", "push key id is invalid")
			}
			txDetail.PushKeyID = kvParts[1]
			continue
		}

		if kvParts[0] == "sig" {
			if kvParts[1] == "" {
				return nil, util.FieldError("sig", "signature value is required")
			} else if _, err := base58.Decode(kvParts[1]); err != nil {
				return nil, util.FieldError("sig", "signature format is not valid")
			}
			txDetail.Signature = kvParts[1]
			continue
		}

		if kvParts[0] == "head" {
			if len(kvParts) == 1 {
				return nil, util.FieldError("head", "value is required")
			} else if len(kvParts[1]) != 40 {
				return nil, util.FieldError("head", "expect a valid object hash")
			}
			txDetail.Head = kvParts[1]
			continue
		}

		if kvParts[0] == "mergeID" {
			if len(kvParts) == 1 || kvParts[1] == "" {
				return nil, util.FieldError("mergeID", "merge proposal id is required")
			}
			if !govalidator.IsNumeric(kvParts[1]) {
				return nil, util.FieldError("mergeID", "merge proposal id must be numeric")
			}
			if len(kvParts[1]) > 8 {
				return nil, util.FieldError("mergeID", "merge proposal id exceeded 8 bytes limit")
			}
			txDetail.MergeProposalID = kvParts[1]
			continue
		}

		return nil, util.FieldError(kvParts[0], "unknown field")
	}

	return txDetail, nil
}
