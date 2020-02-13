package types

import (
	"github.com/fatih/structs"
	"github.com/makeos/mosdef/util"
	"github.com/vmihailenco/msgpack"
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
		TxProposalCommon: &TxProposalCommon{Value: "0", RepoName: ""},
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
