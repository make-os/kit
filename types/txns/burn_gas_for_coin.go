package txns

import (
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/errors"
	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/vmihailenco/msgpack"
)

// TxBurnGasForCoin implements BaseTx, it describes a
// transaction for burning converting gas balance to native coin
type TxBurnGasForCoin struct {
	*TxType   `json:",flatten" msgpack:"-" mapstructure:"-"`
	*TxCommon `json:",flatten" msgpack:"-" mapstructure:"-"`
	Amount    util.String `json:"amount" msgpack:"amount" mapstructure:"amount"`
}

// NewBareTxTxBurnGasForCoin returns an instance of TxBurnGasForCoin with zero values
func NewBareTxTxBurnGasForCoin() *TxBurnGasForCoin {
	return &TxBurnGasForCoin{
		TxType:   &TxType{Type: TxTypeBurnGasForCoin},
		TxCommon: NewBareTxCommon(),
		Amount:   "0",
	}
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (tx *TxBurnGasForCoin) EncodeMsgpack(enc *msgpack.Encoder) error {
	return tx.EncodeMulti(enc,
		tx.Type,
		tx.Nonce,
		tx.Fee,
		tx.Sig,
		tx.Timestamp,
		tx.SenderPubKey,
		tx.Amount)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (tx *TxBurnGasForCoin) DecodeMsgpack(dec *msgpack.Decoder) error {
	return tx.DecodeMulti(dec,
		&tx.Type,
		&tx.Nonce,
		&tx.Fee,
		&tx.Sig,
		&tx.Timestamp,
		&tx.SenderPubKey,
		&tx.Amount)
}

// Bytes returns the serialized transaction
func (tx *TxBurnGasForCoin) Bytes() []byte {
	return util.ToBytes(tx)
}

// GetBytesNoSig returns the serialized the transaction excluding the signature
func (tx *TxBurnGasForCoin) GetBytesNoSig() []byte {
	sig := tx.Sig
	tx.Sig = nil
	bz := tx.Bytes()
	tx.Sig = sig
	return bz
}

// ComputeHash computes the hash of the transaction
func (tx *TxBurnGasForCoin) ComputeHash() util.Bytes32 {
	return util.BytesToBytes32(tmhash.Sum(tx.Bytes()))
}

// GetHash returns the hash of the transaction
func (tx *TxBurnGasForCoin) GetHash() util.HexBytes {
	return tx.ComputeHash().ToHexBytes()
}

// GetID returns the id of the transaction (also the hash)
func (tx *TxBurnGasForCoin) GetID() string {
	return tx.ComputeHash().HexStr()
}

// GetEcoSize returns the size of the transaction for use in protocol economics
func (tx *TxBurnGasForCoin) GetEcoSize() int64 {
	return tx.GetSize()
}

// GetSize returns the size of the tx object (excluding nothing)
func (tx *TxBurnGasForCoin) GetSize() int64 {
	return int64(len(tx.Bytes()))
}

// Sign signs the transaction
func (tx *TxBurnGasForCoin) Sign(privKey string) ([]byte, error) {
	return SignTransaction(tx, privKey)
}

// ToMap returns a map equivalent of the transaction
func (tx *TxBurnGasForCoin) ToMap() map[string]interface{} {
	return util.ToJSONMap(tx)
}

// FromMap populates tx with a map generated by tx.ToMap.
func (tx *TxBurnGasForCoin) FromMap(data map[string]interface{}) error {
	err := tx.TxCommon.FromMap(data)
	err = errors.CallOnNilErr(err, func() error { return tx.TxType.FromMap(data) })
	err = errors.CallOnNilErr(err, func() error { return util.DecodeMap(data, &tx) })
	return err
}
