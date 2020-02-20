package msgs

import (
	"fmt"
	"github.com/fatih/structs"
	"gitlab.com/makeos/mosdef/util"
	"github.com/stretchr/objx"
	"github.com/vmihailenco/msgpack"
)

// TxSetDelegateCommission implements BaseTx, it describes a transaction that
// sets the signers delegate commission rate.
type TxSetDelegateCommission struct {
	*TxType    `json:",flatten" msgpack:"-" mapstructure:"-"`
	*TxCommon  `json:",flatten" msgpack:"-" mapstructure:"-"`
	Commission util.String `json:"commission" msgpack:"commission" mapstructure:"commission"`
}

// NewBareTxSetDelegateCommission returns an instance of TxSetDelegateCommission with zero values
func NewBareTxSetDelegateCommission() *TxSetDelegateCommission {
	return &TxSetDelegateCommission{
		TxType:     &TxType{Type: TxTypeSetDelegatorCommission},
		TxCommon:   NewBareTxCommon(),
		Commission: "",
	}
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (tx *TxSetDelegateCommission) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.EncodeMulti(
		tx.Type,
		tx.Nonce,
		tx.Fee,
		tx.Sig,
		tx.Timestamp,
		tx.SenderPubKey,
		tx.Commission)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (tx *TxSetDelegateCommission) DecodeMsgpack(dec *msgpack.Decoder) error {
	return tx.DecodeMulti(dec,
		&tx.Type,
		&tx.Nonce,
		&tx.Fee,
		&tx.Sig,
		&tx.Timestamp,
		&tx.SenderPubKey,
		&tx.Commission)
}

// Bytes returns the serialized transaction
func (tx *TxSetDelegateCommission) Bytes() []byte {
	return util.ObjectToBytes(tx)
}

// GetBytesNoSig returns the serialized the transaction excluding the signature
func (tx *TxSetDelegateCommission) GetBytesNoSig() []byte {
	sig := tx.Sig
	tx.Sig = nil
	bz := tx.Bytes()
	tx.Sig = sig
	return bz
}

// ComputeHash computes the hash of the transaction
func (tx *TxSetDelegateCommission) ComputeHash() util.Bytes32 {
	return util.BytesToBytes32(util.Blake2b256(tx.Bytes()))
}

// GetHash returns the hash of the transaction
func (tx *TxSetDelegateCommission) GetHash() util.Bytes32 {
	return tx.ComputeHash()
}

// GetID returns the id of the transaction (also the hash)
func (tx *TxSetDelegateCommission) GetID() string {
	return tx.ComputeHash().HexStr()
}

// GetEcoSize returns the size of the transaction for use in protocol economics
func (tx *TxSetDelegateCommission) GetEcoSize() int64 {
	return tx.GetSize()
}

// GetSize returns the size of the tx object (excluding nothing)
func (tx *TxSetDelegateCommission) GetSize() int64 {
	return int64(len(tx.Bytes()))
}

// Sign signs the transaction
func (tx *TxSetDelegateCommission) Sign(privKey string) ([]byte, error) {
	return SignTransaction(tx, privKey)
}

// ToMap returns a map equivalent of the transaction
func (tx *TxSetDelegateCommission) ToMap() map[string]interface{} {
	s := structs.New(tx)
	s.TagName = "json"
	return s.Map()
}

// FromMap populates fields from a map.
// Note: Default or zero values may be set for fields that aren't present in the
// map. Also, an error will be returned when unable to convert types in map to
// actual types in the object.
func (tx *TxSetDelegateCommission) FromMap(data map[string]interface{}) error {
	err := tx.TxCommon.FromMap(data)
	err = util.CallOnNilErr(err, func() error { return tx.TxType.FromMap(data) })

	o := objx.New(data)

	// Commission: expects int64, float64 or string types in map
	if commissionVal := o.Get("commission"); !commissionVal.IsNil() {
		if commissionVal.IsInt64() || commissionVal.IsFloat64() {
			tx.Commission = util.String(fmt.Sprintf("%v", commissionVal.Inter()))
		} else if commissionVal.IsStr() {
			tx.Commission = util.String(commissionVal.Str())
		} else {
			return util.FieldError("commission", fmt.Sprintf("invalid value type: has %T, "+
				"wants string|int64|float", commissionVal.Inter()))
		}
	}

	return err
}
