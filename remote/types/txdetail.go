package types

import (
	"bytes"

	"github.com/make-os/kit/util"
	"github.com/mr-tron/base58"
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
	*IssueFields
	*MergeRequestFields

	// Close indicates that the reference is closed
	Close *bool `json:"close" msgpack:"close,omitempty"`
}

func (rd *ReferenceData) EncodeMsgpack(enc *msgpack.Encoder) error {
	return rd.EncodeMulti(enc,
		rd.Close,
		rd.BaseBranch,
		rd.BaseBranchHash,
		rd.TargetBranch,
		rd.TargetBranchHash,
		rd.Labels,
		rd.Assignees,
	)
}

func (rd *ReferenceData) DecodeMsgpack(dec *msgpack.Decoder) (err error) {
	if rd.IssueFields == nil {
		rd.IssueFields = &IssueFields{}
	}
	if rd.MergeRequestFields == nil {
		rd.MergeRequestFields = &MergeRequestFields{}
	}
	return rd.DecodeMulti(dec,
		&rd.Close,
		&rd.BaseBranch,
		&rd.BaseBranchHash,
		&rd.TargetBranch,
		&rd.TargetBranchHash,
		&rd.Labels,
		&rd.Assignees,
	)
}

func (rd *ReferenceData) ToMap() map[string]interface{} {
	return util.ToMap(rd)
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
	Head            string      `json:"head" msgpack:"head,omitempty" mapstructure:"head"`                // Indicates the [tip] hash of the target reference

	// FlagCheckAdminUpdatePolicy indicate the pusher's intention to perform admin update
	// operation that will require an admin update policy specific to the reference
	FlagCheckAdminUpdatePolicy bool `json:"-" msgpack:"-" mapstructure:"-"`

	// ReferenceData includes data that were extracted from a pushed reference.
	ReferenceData *ReferenceData `json:"-" msgpack:"-" mapstructure:"-"`
}

func (t *TxDetail) ToMap() map[string]interface{} {
	return util.ToMap(t)
}

// GetReferenceData initializes and returns the reference data
func (t *TxDetail) GetReferenceData() *ReferenceData {
	if t.ReferenceData == nil {
		t.ReferenceData = &ReferenceData{
			IssueFields:        &IssueFields{},
			MergeRequestFields: &MergeRequestFields{},
		}
	}
	return t.ReferenceData
}

// SignatureToByte returns the signature as byte.
// Panics if signature could not be decoded.
func (t *TxDetail) SignatureToByte() []byte {
	if t.Signature == "" {
		return nil
	}
	sig, err := base58.Decode(t.Signature)
	if err != nil {
		panic(err)
	}
	return sig
}

// Equal checks whether this object is equal to the give object.
// Signature and MergeProposalID fields are excluded from equality check.
func (t *TxDetail) Equal(o *TxDetail) bool {
	return bytes.Equal(t.BytesNoMergeIDAndSig(), o.BytesNoMergeIDAndSig())
}

// Bytes returns the serialized equivalent of tp
func (t *TxDetail) Bytes() []byte {
	return util.ToBytes(t)
}

// BytesNoSig returns bytes version of tp excluding the signature
func (t *TxDetail) BytesNoSig() []byte {
	sig := t.Signature
	t.Signature = ""
	bz := util.ToBytes(t)
	t.Signature = sig
	return bz
}

// BytesNoMergeIDAndSig returns bytes version of tp excluding the signature and merge ID
func (t *TxDetail) BytesNoMergeIDAndSig() []byte {
	sig, mergeID := t.Signature, t.MergeProposalID
	t.Signature, t.MergeProposalID = "", ""
	bz := util.ToBytes(t)
	t.Signature, t.MergeProposalID = sig, mergeID
	return bz
}

func (t *TxDetail) EncodeMsgpack(enc *msgpack.Encoder) error {
	sig := t.SignatureToByte()
	return t.EncodeMulti(enc,
		t.RepoName,
		t.RepoNamespace,
		t.Reference,
		t.Fee,
		t.Value,
		t.Nonce,
		t.PushKeyID,
		sig,
		t.MergeProposalID,
		t.Head)
}

func (t *TxDetail) DecodeMsgpack(dec *msgpack.Decoder) (err error) {
	var sig []byte
	err = t.DecodeMulti(dec,
		&t.RepoName,
		&t.RepoNamespace,
		&t.Reference,
		&t.Fee,
		&t.Value,
		&t.Nonce,
		&t.PushKeyID,
		&sig,
		&t.MergeProposalID,
		&t.Head)
	t.Signature = base58.Encode(sig)
	return
}
