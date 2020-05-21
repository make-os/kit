package txns

import (
	"github.com/fatih/structs"
	"github.com/vmihailenco/msgpack"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/util"
)

// TxCoinTransfer implements BaseTx, it describes a transaction that transfers
// the native coin from one account to another.
type TxCoinTransfer struct {
	*TxType      `json:",flatten" msgpack:"-" mapstructure:"-"`
	*TxCommon    `json:",flatten" msgpack:"-" mapstructure:"-"`
	*TxRecipient `json:",flatten" msgpack:"-" mapstructure:"-"`
	*TxValue     `json:",flatten" msgpack:"-" mapstructure:"-"`
}

// NewBareTxCoinTransfer returns an instance of TxCoinTransfer with zero values
func NewBareTxCoinTransfer() *TxCoinTransfer {
	return &TxCoinTransfer{
		TxType:      &TxType{Type: TxTypeCoinTransfer},
		TxCommon:    NewBareTxCommon(),
		TxRecipient: &TxRecipient{To: ""},
		TxValue:     &TxValue{Value: "0"},
	}
}

// NewCoinTransferTx creates and populates a coin transfer transaction
func NewCoinTransferTx(
	nonce uint64,
	to util.Address,
	senderKey *crypto.Key,
	value util.String,
	fee util.String,
	timestamp int64) (baseTx types.BaseTx) {

	tx := NewBareTxCoinTransfer()
	tx.SetRecipient(to)
	tx.SetValue(value)
	baseTx = tx

	baseTx.SetTimestamp(timestamp)
	baseTx.SetFee(fee)
	baseTx.SetNonce(nonce)
	baseTx.SetSenderPubKey(senderKey.PubKey().MustBytes())
	sig, err := baseTx.Sign(senderKey.PrivKey().Base58())
	if err != nil {
		panic(err)
	}
	baseTx.SetSignature(sig)
	return
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (tx *TxCoinTransfer) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.EncodeMulti(
		tx.Type,
		tx.Nonce,
		tx.Fee,
		tx.Sig,
		tx.Timestamp,
		tx.SenderPubKey,
		tx.To,
		tx.Value)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (tx *TxCoinTransfer) DecodeMsgpack(dec *msgpack.Decoder) error {
	return tx.DecodeMulti(dec,
		&tx.Type,
		&tx.Nonce,
		&tx.Fee,
		&tx.Sig,
		&tx.Timestamp,
		&tx.SenderPubKey,
		&tx.To,
		&tx.Value)
}

// Bytes returns the serialized transaction
func (tx *TxCoinTransfer) Bytes() []byte {
	return util.ToBytes(tx)
}

// GetBytesNoSig returns the serialized the transaction excluding the signature
func (tx *TxCoinTransfer) GetBytesNoSig() []byte {
	sig := tx.Sig
	tx.Sig = nil
	bz := tx.Bytes()
	tx.Sig = sig
	return bz
}

// ComputeHash computes the hash of the transaction
func (tx *TxCoinTransfer) ComputeHash() util.Bytes32 {
	return util.BytesToBytes32(util.Blake2b256(tx.Bytes()))
}

// GetHash returns the hash of the transaction
func (tx *TxCoinTransfer) GetHash() util.Bytes32 {
	return tx.ComputeHash()
}

// GetID returns the id of the transaction (also the hash)
func (tx *TxCoinTransfer) GetID() string {
	return tx.ComputeHash().HexStr()
}

// GetEcoSize returns the size of the transaction for use in protocol economics
func (tx *TxCoinTransfer) GetEcoSize() int64 {
	return tx.GetSize()
}

// GetSize returns the size of the tx object (excluding nothing)
func (tx *TxCoinTransfer) GetSize() int64 {
	return int64(len(tx.Bytes()))
}

// Sign signs the transaction
func (tx *TxCoinTransfer) Sign(privKey string) ([]byte, error) {
	return SignTransaction(tx, privKey)
}

// ToMap returns a map equivalent of the transaction
func (tx *TxCoinTransfer) ToMap() map[string]interface{} {
	s := structs.New(tx)
	s.TagName = "json"
	return s.Map()
}

// FromMap populates fields from a map.
// Note: Default or zero values may be set for fields that aren't present in the
// map. Also, an error will be returned when unable to convert types in map to
// actual types in the object.
func (tx *TxCoinTransfer) FromMap(data map[string]interface{}) error {
	err := tx.TxCommon.FromMap(data)
	err = util.CallOnNilErr(err, func() error { return tx.TxType.FromMap(data) })
	err = util.CallOnNilErr(err, func() error { return tx.TxRecipient.FromMap(data) })
	err = util.CallOnNilErr(err, func() error { return tx.TxValue.FromMap(data) })
	return err
}
