package types

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/asaskevich/govalidator"
	"github.com/mr-tron/base58"
	"github.com/vmihailenco/msgpack"
	"gitlab.com/makeos/mosdef/util"
)

// ToReferenceTxDetails converts a slice of TxDetail to a map of TxDetail key by their reference
func ToReferenceTxDetails(details []*TxDetail) ReferenceTxDetails {
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
	RepoName        string      `json:"repo" msgpack:"repo,omitempty" mapstructure:"repo"`                // The target repo name
	RepoNamespace   string      `json:"namespace" msgpack:"namespace,omitempty" mapstructure:"namespace"` // The target repo namespace
	Reference       string      `json:"reference" msgpack:"reference,omitempty" mapstructure:"reference"` // The target reference
	Fee             util.String `json:"fee" msgpack:"fee,omitempty" mapstructure:"fee"`                   // Network fee to be paid for update to the target ref
	Nonce           uint64      `json:"nonce" msgpack:"nonce,omitempty" mapstructure:"nonce"`             // Nonce of the account paying the network fee and signing the update.
	PushKeyID       string      `json:"pkID" msgpack:"pkID,omitempty" mapstructure:"pkID"`                // The pusher public key ID.
	Signature       string      `json:"sig" msgpack:"sig,omitempty" mapstructure:"sig"`                   // The signature of the tx parameter
	MergeProposalID string      `json:"mergeID" msgpack:"mergeID,omitempty" mapstructure:"mergeID"`       // Specifies a merge proposal that the push is meant to fulfil
	Head            string      `json:"head" msgpack:"head,omitempty" mapstructure:"head"`                // Indicates the HEAD hash of the target reference

	// FlagCheckIssueUpdatePolicy instructs the authorization function to check whether the
	// pusher has 'issue-update' permission for the reference
	FlagCheckIssueUpdatePolicy bool `json:"-" msgpack:"-" mapstructure:"-"`
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
		"repo":      tp.RepoName,
		"namespace": tp.RepoNamespace,
		"reference": tp.Reference,
		"pkID":      tp.PushKeyID,
		"nonce":     fmt.Sprintf("%d", tp.Nonce),
		"fee":       fmt.Sprintf("%s", tp.Fee),
	}
	if tp.MergeProposalID != "" {
		hdr["mergeID"] = tp.MergeProposalID
	}
	return hdr
}

// TxDetailFromPEMHeader constructs a TxDetail instance from a PEM header
func TxDetailFromPEMHeader(hdr map[string]string) (*TxDetail, error) {
	var params = &TxDetail{
		RepoName:      hdr["repo"],
		RepoNamespace: hdr["namespace"],
		Reference:     hdr["reference"],
		PushKeyID:     hdr["pkID"],
		Fee:           util.String(hdr["fee"]),
	}

	if !govalidator.IsNumeric(hdr["nonce"]) {
		return nil, fmt.Errorf("nonce must be numeric")
	} else {
		params.Nonce, _ = strconv.ParseUint(hdr["nonce"], 10, 64)
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
