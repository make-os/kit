package txns

import (
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/crypto"
	"github.com/vmihailenco/msgpack"
)

// TxRepoProposalSendFee implements BaseTx, it describes a transaction for
// contributing to a proposal's deposit fee
type TxRepoProposalSendFee struct {
	*TxCommon         `json:",flatten" msgpack:"-" mapstructure:"-"`
	*TxType           `json:",flatten" msgpack:"-" mapstructure:"-"`
	*TxProposalCommon `json:",flatten" msgpack:"-" mapstructure:"-"`
}

// NewBareRepoProposalFeeSend returns an instance of TxRepoProposalSendFee with zero values
func NewBareRepoProposalFeeSend() *TxRepoProposalSendFee {
	return &TxRepoProposalSendFee{
		TxCommon:         NewBareTxCommon(),
		TxType:           &TxType{Type: TxTypeRepoProposalSendFee},
		TxProposalCommon: &TxProposalCommon{Value: "0"},
	}
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (tx *TxRepoProposalSendFee) EncodeMsgpack(enc *msgpack.Encoder) error {
	return tx.EncodeMulti(enc,
		tx.Type,
		tx.Nonce,
		tx.Value,
		tx.Fee,
		tx.Sig,
		tx.Timestamp,
		tx.SenderPubKey,
		tx.RepoName,
		tx.ID)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (tx *TxRepoProposalSendFee) DecodeMsgpack(dec *msgpack.Decoder) error {
	return tx.DecodeMulti(dec,
		&tx.Type,
		&tx.Nonce,
		&tx.Value,
		&tx.Fee,
		&tx.Sig,
		&tx.Timestamp,
		&tx.SenderPubKey,
		&tx.RepoName,
		&tx.ID)
}

// Bytes returns the serialized transaction
func (tx *TxRepoProposalSendFee) Bytes() []byte {
	return util.ToBytes(tx)
}

// GetBytesNoSig returns the serialized the transaction excluding the signature
func (tx *TxRepoProposalSendFee) GetBytesNoSig() []byte {
	sig := tx.Sig
	tx.Sig = nil
	bz := tx.Bytes()
	tx.Sig = sig
	return bz
}

// ComputeHash computes the hash of the transaction
func (tx *TxRepoProposalSendFee) ComputeHash() util.Bytes32 {
	return util.BytesToBytes32(crypto.Blake2b256(tx.Bytes()))
}

// GetHash returns the hash of the transaction
func (tx *TxRepoProposalSendFee) GetHash() util.HexBytes {
	return tx.ComputeHash().ToHexBytes()
}

// GetID returns the id of the transaction (also the hash)
func (tx *TxRepoProposalSendFee) GetID() string {
	return tx.ComputeHash().HexStr()
}

// GetEcoSize returns the size of the transaction for use in protocol economics
func (tx *TxRepoProposalSendFee) GetEcoSize() int64 {
	return tx.GetSize()
}

// GetSize returns the size of the tx object (excluding nothing)
func (tx *TxRepoProposalSendFee) GetSize() int64 {
	return int64(len(tx.Bytes()))
}

// Sign signs the transaction
func (tx *TxRepoProposalSendFee) Sign(privKey string) ([]byte, error) {
	return SignTransaction(tx, privKey)
}

// ToBasicMap returns a map equivalent of the transaction
func (tx *TxRepoProposalSendFee) ToMap() map[string]interface{} {
	return util.ToBasicMap(tx)
}

// FromMap populates tx with a map generated by tx.ToMap.
func (tx *TxRepoProposalSendFee) FromMap(data map[string]interface{}) error {
	err := tx.TxCommon.FromMap(data)
	err = util.CallOnNilErr(err, func() error { return tx.TxType.FromMap(data) })
	err = util.CallOnNilErr(err, func() error { return tx.TxProposalCommon.FromMap(data) })
	return err
}
