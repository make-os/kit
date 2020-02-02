package types

import (
	"github.com/fatih/structs"
	"github.com/makeos/mosdef/util"
	"github.com/vmihailenco/msgpack"
)

// TxRepoProposalUpdate implements BaseTx, it describes a repository proposal
// transaction for updating a repository
type TxRepoProposalUpdate struct {
	*TxCommon `json:"-" msgpack:"-" mapstructure:"-"`
	*TxType   `json:"-" msgpack:"-" mapstructure:"-"`
	RepoName  string      `json:"name" msgpack:"name"`
	Config    *RepoConfig `json:"config" msgpack:"config"`
}

// NewBareRepoProposalUpdate returns an instance of TxRepoProposalUpdate with zero values
func NewBareRepoProposalUpdate() *TxRepoProposalUpdate {
	return &TxRepoProposalUpdate{
		TxCommon: NewBareTxCommon(),
		TxType:   &TxType{Type: TxTypeRepoProposalUpdate},
		RepoName: "",
		Config:   BareRepoConfig(),
	}
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (tx *TxRepoProposalUpdate) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.EncodeMulti(
		tx.Type,
		tx.Nonce,
		tx.Fee,
		tx.Sig,
		tx.Timestamp,
		tx.SenderPubKey,
		tx.RepoName,
		tx.Config)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (tx *TxRepoProposalUpdate) DecodeMsgpack(dec *msgpack.Decoder) error {
	return tx.DecodeMulti(dec,
		&tx.Type,
		&tx.Nonce,
		&tx.Fee,
		&tx.Sig,
		&tx.Timestamp,
		&tx.SenderPubKey,
		&tx.RepoName,
		&tx.Config)
}

// Bytes returns the serialized transaction
func (tx *TxRepoProposalUpdate) Bytes() []byte {
	return util.ObjectToBytes(tx)
}

// GetBytesNoSig returns the serialized the transaction excluding the signature
func (tx *TxRepoProposalUpdate) GetBytesNoSig() []byte {
	sig := tx.Sig
	tx.Sig = nil
	bz := tx.Bytes()
	tx.Sig = sig
	return bz
}

// ComputeHash computes the hash of the transaction
func (tx *TxRepoProposalUpdate) ComputeHash() util.Bytes32 {
	return util.BytesToBytes32(util.Blake2b256(tx.Bytes()))
}

// GetHash returns the hash of the transaction
func (tx *TxRepoProposalUpdate) GetHash() util.Bytes32 {
	return tx.ComputeHash()
}

// GetID returns the id of the transaction (also the hash)
func (tx *TxRepoProposalUpdate) GetID() string {
	return tx.ComputeHash().HexStr()
}

// GetEcoSize returns the size of the transaction for use in protocol economics
func (tx *TxRepoProposalUpdate) GetEcoSize() int64 {
	return tx.GetSize()
}

// GetSize returns the size of the tx object (excluding nothing)
func (tx *TxRepoProposalUpdate) GetSize() int64 {
	return int64(len(tx.Bytes()))
}

// Sign signs the transaction
func (tx *TxRepoProposalUpdate) Sign(privKey string) ([]byte, error) {
	return SignTransaction(tx, privKey)
}

// ToMap returns a map equivalent of the transaction
func (tx *TxRepoProposalUpdate) ToMap() map[string]interface{} {
	s := structs.New(tx)
	s.TagName = "json"
	return s.Map()
}
