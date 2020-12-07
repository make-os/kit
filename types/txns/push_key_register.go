package txns

import (
	"github.com/make-os/kit/crypto"
	"github.com/make-os/kit/util"
	crypto2 "github.com/make-os/kit/util/crypto"
	"github.com/stretchr/objx"
	"github.com/vmihailenco/msgpack"
)

// TxRegisterPushKey implements BaseTx, it describes a transaction that registers a push key
type TxRegisterPushKey struct {
	*TxCommon `json:",flatten" msgpack:"-" mapstructure:"-"`
	*TxType   `json:",flatten" msgpack:"-" mapstructure:"-"`
	PublicKey crypto.PublicKey `json:"pubKey" msgpack:"pubKey" mapstructure:"pubKey"`
	Scopes    []string         `json:"scopes" msgpack:"scopes" mapstructure:"scopes"`
	FeeCap    util.String      `json:"feeCap" msgpack:"feeCap" mapstructure:"feeCap"`
}

// NewBareTxRegisterPushKey returns an instance of TxRegisterPushKey with zero values
func NewBareTxRegisterPushKey() *TxRegisterPushKey {
	return &TxRegisterPushKey{
		TxType:   &TxType{Type: TxTypeRegisterPushKey},
		TxCommon: NewBareTxCommon(),
	}
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (tx *TxRegisterPushKey) EncodeMsgpack(enc *msgpack.Encoder) error {
	return tx.EncodeMulti(enc,
		tx.Type,
		tx.Nonce,
		tx.Fee,
		tx.Sig,
		tx.Timestamp,
		tx.SenderPubKey,
		tx.PublicKey,
		tx.Scopes,
		tx.FeeCap)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (tx *TxRegisterPushKey) DecodeMsgpack(dec *msgpack.Decoder) error {
	return tx.DecodeMulti(dec,
		&tx.Type,
		&tx.Nonce,
		&tx.Fee,
		&tx.Sig,
		&tx.Timestamp,
		&tx.SenderPubKey,
		&tx.PublicKey,
		&tx.Scopes,
		&tx.FeeCap)
}

// Bytes returns the serialized transaction
func (tx *TxRegisterPushKey) Bytes() []byte {
	return util.ToBytes(tx)
}

// GetBytesNoSig returns the serialized the transaction excluding the signature
func (tx *TxRegisterPushKey) GetBytesNoSig() []byte {
	sig := tx.Sig
	tx.Sig = nil
	bz := tx.Bytes()
	tx.Sig = sig
	return bz
}

// ComputeHash computes the hash of the transaction
func (tx *TxRegisterPushKey) ComputeHash() util.Bytes32 {
	return util.BytesToBytes32(crypto2.Blake2b256(tx.Bytes()))
}

// GetHash returns the hash of the transaction
func (tx *TxRegisterPushKey) GetHash() util.HexBytes {
	return tx.ComputeHash().ToHexBytes()
}

// GetID returns the id of the transaction (also the hash)
func (tx *TxRegisterPushKey) GetID() string {
	return tx.ComputeHash().HexStr()
}

// GetEcoSize returns the size of the transaction for use in protocol economics
func (tx *TxRegisterPushKey) GetEcoSize() int64 {
	return tx.GetSize()
}

// GetSize returns the size of the tx object (excluding nothing)
func (tx *TxRegisterPushKey) GetSize() int64 {
	return int64(len(tx.Bytes()))
}

// Sign signs the transaction
func (tx *TxRegisterPushKey) Sign(privKey string) ([]byte, error) {
	return SignTransaction(tx, privKey)
}

// ToBasicMap returns a map equivalent of the transaction
func (tx *TxRegisterPushKey) ToMap() map[string]interface{} {
	return util.ToBasicMap(tx)
}

// FromMap populates tx with a map generated by tx.ToMap.
func (tx *TxRegisterPushKey) FromMap(data map[string]interface{}) error {
	err := tx.TxCommon.FromMap(data)
	err = util.CallOnNilErr(err, func() error { return tx.TxType.FromMap(data) })

	fe := util.FieldError
	o := objx.New(data)

	// PublicKey: expects string type, base58 encoded
	if pubKeyVal := o.Get("pubKey"); !pubKeyVal.IsNil() && pubKeyVal.IsStr() {
		pubKey, err := crypto.PubKeyFromBase58(pubKeyVal.Str())
		if err != nil {
			return fe("pubKey", "unable to decode from base58")
		}
		o.Set("pubKey", crypto.BytesToPublicKey(pubKey.MustBytes()))
	}

	err = util.CallOnNilErr(err, func() error { return util.DecodeMap(data, &tx) })
	return err
}
