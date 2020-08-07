package types

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/asaskevich/govalidator"
	"github.com/mr-tron/base58"
	"github.com/spf13/cast"
	"github.com/themakeos/lobe/util"
	"github.com/vmihailenco/msgpack"
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

// ReferenceData stores additional data extracted from a pushed reference.
type ReferenceData struct {
	util.CodecUtil
	IssueFields
	MergeRequestFields
	// Close indicates that the reference is closed
	Close *bool `json:"close" msgpack:"close,omitempty"`
}

func (rd *ReferenceData) EncodeMsgpack(enc *msgpack.Encoder) error {

	var labels, assignees interface{} = nil, nil
	if rd.Labels != nil {
		labels = strings.Join(*rd.Labels, " ")
	}
	if rd.Assignees != nil {
		assignees = strings.Join(*rd.Assignees, " ")
	}

	return rd.EncodeMulti(enc, []interface{}{
		rd.Close,
		rd.BaseBranch,
		rd.BaseBranchHash,
		rd.TargetBranch,
		rd.TargetBranchHash,
		labels,
		assignees,
	})
}

func (rd *ReferenceData) DecodeMsgpack(dec *msgpack.Decoder) (err error) {

	var data []interface{}
	if err = rd.DecodeMulti(dec, &data); err != nil {
		return
	}

	if v := data[0]; v != nil {
		cls := cast.ToBool(v)
		rd.Close = &cls
	}

	if v := data[1]; v != nil {
		rd.BaseBranch = cast.ToString(v)
	}

	if v := data[2]; v != nil {
		rd.BaseBranchHash = cast.ToString(v)
	}

	if v := data[3]; v != nil {
		rd.TargetBranch = cast.ToString(v)
	}

	if v := data[4]; v != nil {
		rd.TargetBranchHash = cast.ToString(v)
	}

	if v := data[5]; v != nil {
		labels := strings.Fields(cast.ToString(v))
		rd.Labels = &labels
	}

	if v := data[6]; v != nil {
		assignees := strings.Fields(cast.ToString(v))
		rd.Assignees = &assignees
	}

	return
}

// TxDetail represents transaction information required to generate
// a network transaction for updating a repository. It includes
// basic network transaction fields, flags and post validation
// data required to construct a valid push transaction.
type TxDetail struct {
	util.CodecUtil
	RepoName        string      `json:"repo" msgpack:"repo,omitempty" mapstructure:"repo"`                // The target repo name
	RepoNamespace   string      `json:"namespace" msgpack:"namespace,omitempty" mapstructure:"namespace"` // The target repo namespace
	Reference       string      `json:"reference" msgpack:"reference,omitempty" mapstructure:"reference"` // The target reference
	Fee             util.String `json:"fee" msgpack:"fee,omitempty" mapstructure:"fee"`                   // Network fee to be paid for update to the target ref
	Value           util.String `json:"value" msgpack:"value,omitempty" mapstructure:"value"`             // Additional value for paying for special operations
	Nonce           uint64      `json:"nonce" msgpack:"nonce,omitempty" mapstructure:"nonce"`             // Nonce of the account paying the network fee and signing the update.
	PushKeyID       string      `json:"pkID" msgpack:"pkID,omitempty" mapstructure:"pkID"`                // The pusher public key ID.
	Signature       string      `json:"sig" msgpack:"sig,omitempty" mapstructure:"sig"`                   // The signature of the tx parameter
	MergeProposalID string      `json:"mergeID" msgpack:"mergeID,omitempty" mapstructure:"mergeID"`       // Specifies a merge proposal that the push is meant to fulfil
	Head            string      `json:"head" msgpack:"head,omitempty" mapstructure:"head"`                // Indicates the HEAD hash of the target reference

	// FlagCheckAdminUpdatePolicy indicate the pusher's intention to perform admin update
	// operation that will require an admin update policy specific to the reference
	FlagCheckAdminUpdatePolicy bool `json:"-" msgpack:"-" mapstructure:"-"`

	// ReferenceData includes data that were extracted from the pushed reference content.
	ReferenceData *ReferenceData `json:"-" msgpack:"-" mapstructure:"-"`
}

// Data initializes and returns the reference data
func (tp *TxDetail) Data() *ReferenceData {
	if tp.ReferenceData == nil {
		tp.ReferenceData = &ReferenceData{}
	}
	return tp.ReferenceData
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

// Equal checks whether this object is equal to the give object.
// Signature and MergeProposalID fields are excluded from equality check.
func (tp *TxDetail) Equal(o *TxDetail) bool {
	return bytes.Equal(tp.BytesNoMergeIDAndSig(), o.BytesNoMergeIDAndSig())
}

// GetGitSigPEMHeader returns headers to be used as git signature PEM headers
func (tp *TxDetail) GetGitSigPEMHeader() map[string]string {
	hdr := map[string]string{
		"repo":      tp.RepoName,
		"namespace": tp.RepoNamespace,
		"reference": tp.Reference,
		"pkID":      tp.PushKeyID,
		"nonce":     fmt.Sprintf("%d", tp.Nonce),
		"fee":       tp.Fee.String(),
	}
	if tp.Value != "" {
		hdr["value"] = tp.Value.String()
	}
	return hdr
}

// TxDetailFromGitSigPEMHeader constructs a TxDetail instance from a git signature PEM header
func TxDetailFromGitSigPEMHeader(hdr map[string]string) (*TxDetail, error) {
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

	if value, ok := hdr["value"]; ok {
		params.Value = util.String(value)
	}

	return params, nil
}

// Bytes returns the serialized equivalent of tp
func (tp *TxDetail) Bytes() []byte {
	return util.ToBytes(tp)
}

// BytesNoSig returns bytes version of tp excluding the signature
func (tp *TxDetail) BytesNoSig() []byte {
	sig := tp.Signature
	tp.Signature = ""
	bz := util.ToBytes(tp)
	tp.Signature = sig
	return bz
}

// BytesNoMergeIDAndSig returns bytes version of tp excluding the signature and merge ID
func (tp *TxDetail) BytesNoMergeIDAndSig() []byte {
	sig, mergeID := tp.Signature, tp.MergeProposalID
	tp.Signature, tp.MergeProposalID = "", ""
	bz := util.ToBytes(tp)
	tp.Signature, tp.MergeProposalID = sig, mergeID
	return bz
}

func (tp *TxDetail) EncodeMsgpack(enc *msgpack.Encoder) error {
	sig := tp.MustSignatureAsBytes()
	return tp.EncodeMulti(enc,
		tp.RepoName,
		tp.RepoNamespace,
		tp.Reference,
		tp.Fee,
		tp.Value,
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
		&tp.Value,
		&tp.Nonce,
		&tp.PushKeyID,
		&sig,
		&tp.MergeProposalID,
		&tp.Head)
	tp.Signature = base58.Encode(sig)
	return
}
