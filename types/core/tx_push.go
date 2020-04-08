package core

import (
	"github.com/fatih/structs"
	"github.com/vmihailenco/msgpack"
	"gitlab.com/makeos/mosdef/util"
)

// TxPush implements BaseTx, it describes a transaction that creates a
// repository for the signer
type TxPush struct {
	*TxCommon      `json:",flatten" mapstructure:"-"`
	*TxType        `json:",flatten" msgpack:"-"`
	PushNote       *PushNote          `json:"pushNote" mapstructure:"pushNote"`
	PushEnds       []*PushEndorsement `json:"endorsements" mapstructure:"endorsements"`
	AggPushEndsSig []byte             `json:"aggEndorsersPubKey" mapstructure:"aggEndorsersPubKey"`
}

// NewBareTxPush returns an instance of TxPush with zero values
func NewBareTxPush() *TxPush {
	return &TxPush{
		TxCommon: NewBareTxCommon(),
		TxType:   &TxType{Type: TxTypePush},
		PushNote: &PushNote{},
		PushEnds: []*PushEndorsement{},
	}
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (tx *TxPush) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.EncodeMulti(
		tx.Type,
		tx.PushNote,
		tx.PushEnds,
		tx.AggPushEndsSig)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (tx *TxPush) DecodeMsgpack(dec *msgpack.Decoder) error {
	return tx.DecodeMulti(dec,
		&tx.Type,
		&tx.PushNote,
		&tx.PushEnds,
		&tx.AggPushEndsSig)
}

// Bytes returns the serialized transaction
func (tx *TxPush) Bytes() []byte {
	return util.ToBytes(tx)
}

// GetBytesNoSig returns the serialized the transaction excluding the signature
func (tx *TxPush) GetBytesNoSig() []byte {
	sig := tx.Sig
	tx.Sig = nil
	bz := tx.Bytes()
	tx.Sig = sig
	return bz
}

// ComputeHash computes the hash of the transaction
func (tx *TxPush) ComputeHash() util.Bytes32 {
	return util.BytesToBytes32(util.Blake2b256(tx.Bytes()))
}

// GetHash returns the hash of the transaction
func (tx *TxPush) GetHash() util.Bytes32 {
	return tx.PushNote.ID()
}

// GetID returns the id of the transaction (also the hash)
func (tx *TxPush) GetID() string {
	return tx.PushNote.ID().String()
}

// GetEcoSize returns the size of the transaction for use in economic calculations
func (tx *TxPush) GetEcoSize() int64 {
	size := tx.GetSize()
	pushedObjSize := tx.PushNote.Size
	return int64(uint64(size) + pushedObjSize)
}

// GetSize returns the size of the tx object (excluding nothing)
func (tx *TxPush) GetSize() int64 {
	return int64(len(tx.Bytes()))
}

// GetTimestamp return the transaction creation unix timestamp.
// Because TxPush is a wrapper transaction, we use the push note timestamp
func (tx *TxPush) GetTimestamp() int64 {
	return tx.PushNote.Timestamp
}

// GetNonce returns the transaction nonce.
// Because TxPush is a wrapper transaction, we use the Account nonce of the pusher
// which is found in anyone of the pushed reference
func (tx *TxPush) GetNonce() uint64 {
	return tx.PushNote.PusherAcctNonce
}

// GetFrom returns the address of the transaction sender
// Because TxPush is a wrapper transaction, we use the pusher's address.
func (tx *TxPush) GetFrom() util.Address {
	return tx.PushNote.PusherAddress
}

// Sign signs the transaction
func (tx *TxPush) Sign(privKey string) ([]byte, error) {
	return SignTransaction(tx, privKey)
}

// ToMap returns a map equivalent of the transaction
func (tx *TxPush) ToMap() map[string]interface{} {
	s := structs.New(tx)
	s.TagName = "json"
	return s.Map()
}

// FromMap populates fields from a map.
// Note: Default or zero values may be set for fields that aren't present in the
// map. Also, an error will be returned when unable to convert types in map to
// actual types in the object.
func (tx *TxPush) FromMap(data map[string]interface{}) error {
	panic("FromMap: not supported")
}
