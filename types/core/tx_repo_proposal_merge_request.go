package core

import (
	"fmt"
	"github.com/fatih/structs"
	"github.com/stretchr/objx"
	"github.com/vmihailenco/msgpack"
	"gitlab.com/makeos/mosdef/util"
)

// TxRepoProposalMergeRequest implements BaseTx, it describes a transaction for
// requesting permission to merge a target branch into a base branch
type TxRepoProposalMergeRequest struct {
	*TxCommon         `json:",flatten" msgpack:"-" mapstructure:"-"`
	*TxType           `json:",flatten" msgpack:"-" mapstructure:"-"`
	*TxProposalCommon `json:",flatten" msgpack:"-" mapstructure:"-"`
	BaseBranch        string `json:"base" msgpack:"base"`
	BaseBranchHash    string `json:"baseHash" msgpack:"baseHash"`
	TargetBranch      string `json:"target" msgpack:"target"`
	TargetBranchHash  string `json:"targetHash" msgpack:"targetHash"`
}

// NewBareRepoProposalMergeRequest returns an instance of TxRepoProposalMergeRequest with zero values
func NewBareRepoProposalMergeRequest() *TxRepoProposalMergeRequest {
	return &TxRepoProposalMergeRequest{
		TxCommon:         NewBareTxCommon(),
		TxType:           &TxType{Type: TxTypeRepoProposalMergeRequest},
		TxProposalCommon: &TxProposalCommon{Value: "0", RepoName: "", ProposalID: ""},
		BaseBranch:       "",
		BaseBranchHash:   "",
		TargetBranch:     "",
		TargetBranchHash: "",
	}
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (tx *TxRepoProposalMergeRequest) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.EncodeMulti(
		tx.Type,
		tx.Nonce,
		tx.Value,
		tx.Fee,
		tx.Sig,
		tx.Timestamp,
		tx.SenderPubKey,
		tx.RepoName,
		tx.ProposalID,
		tx.BaseBranch,
		tx.BaseBranchHash,
		tx.TargetBranch,
		tx.TargetBranchHash)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (tx *TxRepoProposalMergeRequest) DecodeMsgpack(dec *msgpack.Decoder) error {
	return tx.DecodeMulti(dec,
		&tx.Type,
		&tx.Nonce,
		&tx.Value,
		&tx.Fee,
		&tx.Sig,
		&tx.Timestamp,
		&tx.SenderPubKey,
		&tx.RepoName,
		&tx.ProposalID,
		&tx.BaseBranch,
		&tx.BaseBranchHash,
		&tx.TargetBranch,
		&tx.TargetBranchHash)
}

// Bytes returns the serialized transaction
func (tx *TxRepoProposalMergeRequest) Bytes() []byte {
	return util.ObjectToBytes(tx)
}

// GetBytesNoSig returns the serialized the transaction excluding the signature
func (tx *TxRepoProposalMergeRequest) GetBytesNoSig() []byte {
	sig := tx.Sig
	tx.Sig = nil
	bz := tx.Bytes()
	tx.Sig = sig
	return bz
}

// ComputeHash computes the hash of the transaction
func (tx *TxRepoProposalMergeRequest) ComputeHash() util.Bytes32 {
	return util.BytesToBytes32(util.Blake2b256(tx.Bytes()))
}

// GetHash returns the hash of the transaction
func (tx *TxRepoProposalMergeRequest) GetHash() util.Bytes32 {
	return tx.ComputeHash()
}

// GetID returns the id of the transaction (also the hash)
func (tx *TxRepoProposalMergeRequest) GetID() string {
	return tx.ComputeHash().HexStr()
}

// GetEcoSize returns the size of the transaction for use in protocol economics
func (tx *TxRepoProposalMergeRequest) GetEcoSize() int64 {
	return tx.GetSize()
}

// GetSize returns the size of the tx object (excluding nothing)
func (tx *TxRepoProposalMergeRequest) GetSize() int64 {
	return int64(len(tx.Bytes()))
}

// Sign signs the transaction
func (tx *TxRepoProposalMergeRequest) Sign(privKey string) ([]byte, error) {
	return SignTransaction(tx, privKey)
}

// ToMap returns a map equivalent of the transaction
func (tx *TxRepoProposalMergeRequest) ToMap() map[string]interface{} {
	s := structs.New(tx)
	s.TagName = "json"
	return s.Map()
}

// FromMap populates fields from a map.
// Note: Default or zero values may be set for fields that aren't present in the
// map. Also, an error will be returned when unable to convert types in map to
// actual types in the object.
func (tx *TxRepoProposalMergeRequest) FromMap(data map[string]interface{}) error {
	err := tx.TxCommon.FromMap(data)
	err = util.CallOnNilErr(err, func() error { return tx.TxType.FromMap(data) })
	err = util.CallOnNilErr(err, func() error { return tx.TxProposalCommon.FromMap(data) })

	o := objx.New(data)

	// BaseBranch: expects string type in map
	if baseVal := o.Get("base"); !baseVal.IsNil() {
		if baseVal.IsStr() {
			tx.BaseBranch = baseVal.Str()
		} else {
			return util.FieldError("base", fmt.Sprintf("invalid value type: has %T, "+
				"wants string", baseVal.Inter()))
		}
	}

	// BaseBranchHash: expects string type in map
	if baseHashVal := o.Get("baseHash"); !baseHashVal.IsNil() {
		if baseHashVal.IsStr() {
			tx.BaseBranchHash = baseHashVal.Str()
		} else {
			return util.FieldError("baseHash", fmt.Sprintf("invalid value type: has %T, "+
				"wants string", baseHashVal.Inter()))
		}
	}

	// TargetBranch: expects string type in map
	if targetVal := o.Get("target"); !targetVal.IsNil() {
		if targetVal.IsStr() {
			tx.TargetBranch = targetVal.Str()
		} else {
			return util.FieldError("target", fmt.Sprintf("invalid value type: has %T, "+
				"wants string", targetVal.Inter()))
		}
	}

	// TargetBranchHash: expects string type in map
	if targetHashVal := o.Get("targetHash"); !targetHashVal.IsNil() {
		if targetHashVal.IsStr() {
			tx.TargetBranchHash = targetHashVal.Str()
		} else {
			return util.FieldError("targetHash", fmt.Sprintf("invalid value type: has %T, "+
				"wants string", targetHashVal.Inter()))
		}
	}

	return err
}
