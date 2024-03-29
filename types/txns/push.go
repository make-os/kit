package txns

import (
	pptyp "github.com/make-os/kit/remote/push/types"
	"github.com/make-os/kit/types"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/identifier"
	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/vmihailenco/msgpack"
)

type PushEndorsements []*pptyp.PushEndorsement

// ClearNoteID clears the Note ID of all endorsements
func (e PushEndorsements) ClearNoteID() {
	for _, ends := range e {
		ends.NoteID = nil
	}
}

// SetNoteID sets the Note ID of all endorsements
func (e PushEndorsements) SetNoteID(id []byte) {
	for _, ends := range e {
		ends.NoteID = id
	}
}

// SetReferences sets the endorsed references for all endorsements
func (e PushEndorsements) SetReferences(ef pptyp.EndorsedReferences) {
	for _, ends := range e {
		ends.References = ef
	}
}

// ClearReferences sets the endorsed references for all endorsements except ignoreIndex.
func (e PushEndorsements) ClearReferences(ignoreIndex int) {
	for i, ends := range e {
		if i == ignoreIndex {
			continue
		}
		ends.References = nil
	}
}

// TxPush implements BaseTx, it describes a transaction that creates a
// repository for the signer
type TxPush struct {
	*TxCommon `json:",flatten" mapstructure:"-"`
	*TxType   `json:",flatten" msgpack:"-"`

	// Note is the push note
	Note pptyp.PushNote `json:"note" mapstructure:"note"`

	// Endorsements contain push endorsements
	Endorsements PushEndorsements `json:"endorsements" mapstructure:"endorsements"`

	// AggregatedSig contains aggregated BLS signature composed by endorsers
	AggregatedSig []byte `json:"aggEndSig" mapstructure:"aggEndSig"`
}

// NewBareTxPush returns an instance of TxPush with zero values
func NewBareTxPush() *TxPush {
	return &TxPush{
		TxCommon:     NewBareTxCommon(),
		TxType:       &TxType{Type: TxTypePush},
		Note:         &pptyp.Note{BasicMeta: types.NewMeta()},
		Endorsements: []*pptyp.PushEndorsement{},
	}
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (tx *TxPush) EncodeMsgpack(enc *msgpack.Encoder) error {
	return tx.EncodeMulti(enc,
		tx.Type,
		tx.Note,
		tx.Endorsements,
		tx.AggregatedSig)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (tx *TxPush) DecodeMsgpack(dec *msgpack.Decoder) error {
	return tx.DecodeMulti(dec,
		&tx.Type,
		&tx.Note,
		&tx.Endorsements,
		&tx.AggregatedSig)
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
	return util.BytesToBytes32(tmhash.Sum(tx.Bytes()))
}

// GetHash returns the hash of the transaction
func (tx *TxPush) GetHash() util.HexBytes {
	return tx.ComputeHash().ToHexBytes()
}

// GetID returns the hash of the push transaction
func (tx *TxPush) GetID() string {
	return tx.GetHash().String()
}

// GetNoteID returns the note ID of the push transaction
func (tx *TxPush) GetNoteID() string {
	return tx.Note.ID().String()
}

// GetEcoSize returns the size of the transaction for use in economic calculations
func (tx *TxPush) GetEcoSize() int64 {
	size := tx.GetSize()
	pushedObjSize := tx.Note.GetSize()
	return int64(uint64(size) + pushedObjSize)
}

// GetSize returns the size of the tx object (excluding nothing)
func (tx *TxPush) GetSize() int64 {
	return int64(len(tx.Bytes()))
}

// GetTimestamp return the transaction creation unix timestamp.
// Because TxPush is a wrapper transaction, we use the push note timestamp
func (tx *TxPush) GetTimestamp() int64 {
	return tx.Note.GetTimestamp()
}

// GetNonce returns the transaction nonce.
// Because TxPush is a wrapper transaction, we use the Account nonce of the pusher
// which is found in anyone of the pushed reference
func (tx *TxPush) GetNonce() uint64 {
	return tx.Note.GetPusherAccountNonce()
}

// GetFrom returns the address of the transaction sender
// Because TxPush is a wrapper transaction, we use the pusher's address.
func (tx *TxPush) GetFrom() identifier.Address {
	return tx.Note.GetPusherAddress()
}

// Sign signs the transaction
func (tx *TxPush) Sign(privKey string) ([]byte, error) {
	return SignTransaction(tx, privKey)
}

// ToMap returns a map equivalent of the transaction
func (tx *TxPush) ToMap() map[string]interface{} {
	return util.ToJSONMap(tx)
}

// FromMap populates tx with a map generated by tx.ToMap.
func (tx *TxPush) FromMap(_ map[string]interface{}) error {
	panic("FromMap: not supported")
}
