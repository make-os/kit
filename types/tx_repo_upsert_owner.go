package types

import (
	"github.com/fatih/structs"
	"github.com/makeos/mosdef/util"
	"github.com/vmihailenco/msgpack"
)

// TxRepoProposalUpsertOwner implements BaseTx, it describes a repository proposal
// transaction for adding a new owner to a repository
type TxRepoProposalUpsertOwner struct {
	*TxCommon `json:"-" msgpack:"-" mapstructure:"-"`
	*TxType   `json:"-" msgpack:"-" mapstructure:"-"`
	RepoName  string `json:"name" msgpack:"name"`
	Address   string `json:"address" msgpack:"address"`
}

// NewBareRepoProposalAddOwner returns an instance of TxRepoProposalUpsertOwner with zero values
func NewBareRepoProposalAddOwner() *TxRepoProposalUpsertOwner {
	return &TxRepoProposalUpsertOwner{
		TxCommon: NewBareTxCommon(),
		TxType:   &TxType{Type: TxTypeRepoProposalUpsertOwner},
		RepoName: "",
		Address:  "",
	}
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (tx *TxRepoProposalUpsertOwner) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.EncodeMulti(
		tx.Type,
		tx.Nonce,
		tx.Fee,
		tx.Sig,
		tx.Timestamp,
		tx.SenderPubKey,
		tx.RepoName,
		tx.Address)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (tx *TxRepoProposalUpsertOwner) DecodeMsgpack(dec *msgpack.Decoder) error {
	return tx.DecodeMulti(dec,
		&tx.Type,
		&tx.Nonce,
		&tx.Fee,
		&tx.Sig,
		&tx.Timestamp,
		&tx.SenderPubKey,
		&tx.RepoName,
		&tx.Address)
}

// Bytes returns the serialized transaction
func (tx *TxRepoProposalUpsertOwner) Bytes() []byte {
	return util.ObjectToBytes(tx)
}

// GetBytesNoSig returns the serialized the transaction excluding the signature
func (tx *TxRepoProposalUpsertOwner) GetBytesNoSig() []byte {
	sig := tx.Sig
	tx.Sig = nil
	bz := tx.Bytes()
	tx.Sig = sig
	return bz
}

// ComputeHash computes the hash of the transaction
func (tx *TxRepoProposalUpsertOwner) ComputeHash() util.Bytes32 {
	return util.BytesToBytes32(util.Blake2b256(tx.Bytes()))
}

// GetHash returns the hash of the transaction
func (tx *TxRepoProposalUpsertOwner) GetHash() util.Bytes32 {
	return tx.ComputeHash()
}

// GetID returns the id of the transaction (also the hash)
func (tx *TxRepoProposalUpsertOwner) GetID() string {
	return tx.ComputeHash().HexStr()
}

// GetEcoSize returns the size of the transaction for use in protocol economics
func (tx *TxRepoProposalUpsertOwner) GetEcoSize() int64 {
	return tx.GetSize()
}

// GetSize returns the size of the tx object (excluding nothing)
func (tx *TxRepoProposalUpsertOwner) GetSize() int64 {
	return int64(len(tx.Bytes()))
}

// Sign signs the transaction
func (tx *TxRepoProposalUpsertOwner) Sign(privKey string) ([]byte, error) {
	return SignTransaction(tx, privKey)
}

// ToMap returns a map equivalent of the transaction
func (tx *TxRepoProposalUpsertOwner) ToMap() map[string]interface{} {
	s := structs.New(tx)
	s.TagName = "json"
	return s.Map()
}
