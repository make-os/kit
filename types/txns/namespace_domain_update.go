package txns

import (
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/errors"
	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/vmihailenco/msgpack"
)

// TxNamespaceDomainUpdate implements BaseTx, it describes a transaction for acquiring a namespace
type TxNamespaceDomainUpdate struct {
	*TxType   `json:",flatten" msgpack:"-" mapstructure:"-"`
	*TxCommon `json:",flatten" msgpack:"-" mapstructure:"-"`
	Name      string            `json:"name" msgpack:"name" mapstructure:"name"`
	Domains   map[string]string `json:"domains" msgpack:"domains" mapstructure:"domains"`
}

// NewBareTxNamespaceDomainUpdate returns an instance of TxNamespaceDomainUpdate with zero values
func NewBareTxNamespaceDomainUpdate() *TxNamespaceDomainUpdate {
	return &TxNamespaceDomainUpdate{
		TxType:   &TxType{Type: TxTypeNamespaceDomainUpdate},
		TxCommon: NewBareTxCommon(),
		Name:     "",
		Domains:  make(map[string]string),
	}
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (tx *TxNamespaceDomainUpdate) EncodeMsgpack(enc *msgpack.Encoder) error {
	return tx.EncodeMulti(enc,
		tx.Type,
		tx.Nonce,
		tx.Fee,
		tx.Sig,
		tx.Timestamp,
		tx.SenderPubKey,
		tx.Name,
		tx.Domains)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (tx *TxNamespaceDomainUpdate) DecodeMsgpack(dec *msgpack.Decoder) error {
	return tx.DecodeMulti(dec,
		&tx.Type,
		&tx.Nonce,
		&tx.Fee,
		&tx.Sig,
		&tx.Timestamp,
		&tx.SenderPubKey,
		&tx.Name,
		&tx.Domains)
}

// Bytes returns the serialized transaction
func (tx *TxNamespaceDomainUpdate) Bytes() []byte {
	return util.ToBytes(tx)
}

// GetBytesNoSig returns the serialized the transaction excluding the signature
func (tx *TxNamespaceDomainUpdate) GetBytesNoSig() []byte {
	sig := tx.Sig
	tx.Sig = nil
	bz := tx.Bytes()
	tx.Sig = sig
	return bz
}

// ComputeHash computes the hash of the transaction
func (tx *TxNamespaceDomainUpdate) ComputeHash() util.Bytes32 {
	return util.BytesToBytes32(tmhash.Sum(tx.Bytes()))
}

// GetHash returns the hash of the transaction
func (tx *TxNamespaceDomainUpdate) GetHash() util.HexBytes {
	return tx.ComputeHash().ToHexBytes()
}

// GetID returns the id of the transaction (also the hash)
func (tx *TxNamespaceDomainUpdate) GetID() string {
	return tx.ComputeHash().HexStr()
}

// GetEcoSize returns the size of the transaction for use in protocol economics
func (tx *TxNamespaceDomainUpdate) GetEcoSize() int64 {
	return tx.GetSize()
}

// GetSize returns the size of the tx object (excluding nothing)
func (tx *TxNamespaceDomainUpdate) GetSize() int64 {
	return int64(len(tx.Bytes()))
}

// Sign signs the transaction
func (tx *TxNamespaceDomainUpdate) Sign(privKey string) ([]byte, error) {
	return SignTransaction(tx, privKey)
}

// ToJSONMap returns a map equivalent of the transaction
func (tx *TxNamespaceDomainUpdate) ToMap() map[string]interface{} {
	return util.ToJSONMap(tx)
}

// FromMap populates tx with a map generated by tx.ToMap.
func (tx *TxNamespaceDomainUpdate) FromMap(data map[string]interface{}) error {
	err := tx.TxCommon.FromMap(data)
	err = errors.CallOnNilErr(err, func() error { return tx.TxType.FromMap(data) })
	err = errors.CallOnNilErr(err, func() error { return util.DecodeMap(data, &tx) })
	return err
}
