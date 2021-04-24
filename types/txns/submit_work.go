package txns

import (
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/errors"
	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/vmihailenco/msgpack"
)

// TxSubmitWork implements BaseTx, it describes a transaction for
// submitting a proof of work nonce for a given epoch.
type TxSubmitWork struct {
	*TxCommon `json:",flatten" msgpack:"-" mapstructure:"-"`
	*TxType   `json:",flatten" msgpack:"-" mapstructure:"-"`
	Epoch     int64  `json:"epoch" msgpack:"epoch" mapstructure:"epoch"`
	WorkNonce uint64 `json:"wnonce" msgpack:"wnonce" mapstructure:"wnonce"`
}

// NewBareTxSubmitWork returns an instance of TxSubmitWork with zero values
func NewBareTxSubmitWork() *TxSubmitWork {
	return &TxSubmitWork{
		TxCommon: NewBareTxCommon(),
		TxType:   &TxType{Type: TxTypeSubmitWork},
	}
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (tx *TxSubmitWork) EncodeMsgpack(enc *msgpack.Encoder) error {
	return tx.EncodeMulti(enc,
		tx.Type,
		tx.Nonce,
		tx.Fee,
		tx.Sig,
		tx.Timestamp,
		tx.SenderPubKey,
		tx.Epoch,
		tx.WorkNonce)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (tx *TxSubmitWork) DecodeMsgpack(dec *msgpack.Decoder) error {
	return tx.DecodeMulti(dec,
		&tx.Type,
		&tx.Nonce,
		&tx.Fee,
		&tx.Sig,
		&tx.Timestamp,
		&tx.SenderPubKey,
		&tx.Epoch,
		&tx.WorkNonce)
}

// Bytes returns the serialized transaction
func (tx *TxSubmitWork) Bytes() []byte {
	return util.ToBytes(tx)
}

// GetBytesNoSig returns the serialized the transaction excluding the signature
func (tx *TxSubmitWork) GetBytesNoSig() []byte {
	sig := tx.Sig
	tx.Sig = nil
	bz := tx.Bytes()
	tx.Sig = sig
	return bz
}

// ComputeHash computes the hash of the transaction
func (tx *TxSubmitWork) ComputeHash() util.Bytes32 {
	return util.BytesToBytes32(tmhash.Sum(tx.Bytes()))
}

// GetHash returns the hash of the transaction
func (tx *TxSubmitWork) GetHash() util.HexBytes {
	return tx.ComputeHash().ToHexBytes()
}

// GetID returns the id of the transaction (also the hash)
func (tx *TxSubmitWork) GetID() string {
	return tx.ComputeHash().HexStr()
}

// GetEcoSize returns the size of the transaction for use in protocol economics
func (tx *TxSubmitWork) GetEcoSize() int64 {
	return tx.GetSize()
}

// GetSize returns the size of the tx object (excluding nothing)
func (tx *TxSubmitWork) GetSize() int64 {
	return int64(len(tx.Bytes()))
}

// Sign signs the transaction
func (tx *TxSubmitWork) Sign(privKey string) ([]byte, error) {
	return SignTransaction(tx, privKey)
}

// ToMap returns a map equivalent of the transaction
func (tx *TxSubmitWork) ToMap() map[string]interface{} {
	return util.ToJSONMap(tx)
}

// FromMap populates fields from a map.
// An error will be returned when unable to convert types in map to expected types in the object.
func (tx *TxSubmitWork) FromMap(data map[string]interface{}) error {
	err := tx.TxCommon.FromMap(data)
	err = errors.CallOnNilErr(err, func() error { return tx.TxType.FromMap(data) })
	err = errors.CallOnNilErr(err, func() error { return util.DecodeMap(data, &tx) })
	return err
}
