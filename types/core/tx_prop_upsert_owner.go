package core

import (
	"fmt"
	"strings"

	"github.com/fatih/structs"
	"github.com/stretchr/objx"
	"github.com/vmihailenco/msgpack"
	"gitlab.com/makeos/mosdef/util"
)

// TxRepoProposalUpsertOwner implements BaseTx, it describes a repository proposal
// transaction for adding a new owner to a repository
type TxRepoProposalUpsertOwner struct {
	*TxCommon         `json:",flatten" msgpack:"-" mapstructure:"-"`
	*TxType           `json:",flatten" msgpack:"-" mapstructure:"-"`
	*TxProposalCommon `json:",flatten" msgpack:"-" mapstructure:"-"`
	Addresses         []string `json:"addresses" msgpack:"addresses"`
	Veto              bool     `json:"veto" msgpack:"veto"`
}

// NewBareRepoProposalUpsertOwner returns an instance of TxRepoProposalUpsertOwner with zero values
func NewBareRepoProposalUpsertOwner() *TxRepoProposalUpsertOwner {
	return &TxRepoProposalUpsertOwner{
		TxCommon:         NewBareTxCommon(),
		TxType:           &TxType{Type: TxTypeRepoProposalUpsertOwner},
		TxProposalCommon: &TxProposalCommon{Value: "0", RepoName: "", ProposalID: ""},
		Addresses:        []string{},
		Veto:             false,
	}
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (tx *TxRepoProposalUpsertOwner) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.EncodeMulti(
		tx.Type,
		tx.Nonce,
		tx.Value,
		tx.Fee,
		tx.Sig,
		tx.Timestamp,
		tx.SenderPubKey,
		tx.RepoName,
		tx.ProposalID,
		tx.Addresses,
		tx.Veto)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (tx *TxRepoProposalUpsertOwner) DecodeMsgpack(dec *msgpack.Decoder) error {
	return tx.DecodeMulti(dec,
		&tx.Type,
		&tx.Nonce,
		&tx.Value,
		&tx.Fee,
		&tx.Sig,
		&tx.Timestamp,
		&tx.SenderPubKey,
		&tx.RepoName,
		&tx.ProposalID,
		&tx.Addresses,
		&tx.Veto)
}

// Bytes returns the serialized transaction
func (tx *TxRepoProposalUpsertOwner) Bytes() []byte {
	return util.ToBytes(tx)
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

// FromMap populates fields from a map.
// Note: Default or zero values may be set for fields that aren't present in the
// map. Also, an error will be returned when unable to convert types in map to
// actual types in the object.
func (tx *TxRepoProposalUpsertOwner) FromMap(data map[string]interface{}) error {
	err := tx.TxCommon.FromMap(data)
	err = util.CallOnNilErr(err, func() error { return tx.TxType.FromMap(data) })
	err = util.CallOnNilErr(err, func() error { return tx.TxProposalCommon.FromMap(data) })

	o := objx.New(data)

	// Addresses: expects string or slice of string types in map
	if addrVal := o.Get("addresses"); !addrVal.IsNil() {
		if addrVal.IsStr() {
			tx.Addresses = strings.Split(addrVal.Str(), ",")
		} else if addrVal.IsStrSlice() {
			tx.Addresses = addrVal.StrSlice()
		} else {
			return util.FieldError("addresses", fmt.Sprintf("invalid value type: has %T, "+
				"wants string|[]string", addrVal.Inter()))
		}
	}

	// Veto: expects boolean in map
	if vetoVal := o.Get("veto"); !vetoVal.IsNil() {
		if vetoVal.IsBool() {
			tx.Veto = vetoVal.Bool()
		} else {
			return util.FieldError("veto", fmt.Sprintf("invalid value type: has %T, "+
				"wants bool", vetoVal.Inter()))
		}
	}

	return err
}
