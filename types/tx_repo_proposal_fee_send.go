package types

import (
	"fmt"

	"github.com/fatih/structs"
	"github.com/makeos/mosdef/util"
	"github.com/stretchr/objx"
	"github.com/vmihailenco/msgpack"
)

// TxRepoProposalFeeSend implements BaseTx, it describes a transaction for
// sending units of the native coin as proposal fee.
type TxRepoProposalFeeSend struct {
	*TxCommon  `json:",flatten" msgpack:"-" mapstructure:"-"`
	*TxType    `json:",flatten" msgpack:"-" mapstructure:"-"`
	*TxValue   `json:",flatten" msgpack:"-" mapstructure:"-"`
	RepoName   string `json:"name" msgpack:"name"`
	ProposalID string `json:"id" msgpack:"id"`
}

// NewBareRepoProposalFeeSend returns an instance of TxRepoProposalFeeSend with zero values
func NewBareRepoProposalFeeSend() *TxRepoProposalFeeSend {
	return &TxRepoProposalFeeSend{
		TxCommon:   NewBareTxCommon(),
		TxType:     &TxType{Type: TxTypeRepoProposalFeeSend},
		TxValue:    &TxValue{Value: "0"},
		RepoName:   "",
		ProposalID: "",
	}
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (tx *TxRepoProposalFeeSend) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.EncodeMulti(
		tx.Type,
		tx.Nonce,
		tx.Value,
		tx.Fee,
		tx.Sig,
		tx.Timestamp,
		tx.SenderPubKey,
		tx.RepoName,
		tx.ProposalID)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (tx *TxRepoProposalFeeSend) DecodeMsgpack(dec *msgpack.Decoder) error {
	return tx.DecodeMulti(dec,
		&tx.Type,
		&tx.Nonce,
		&tx.Value,
		&tx.Fee,
		&tx.Sig,
		&tx.Timestamp,
		&tx.SenderPubKey,
		&tx.RepoName,
		&tx.ProposalID)
}

// Bytes returns the serialized transaction
func (tx *TxRepoProposalFeeSend) Bytes() []byte {
	return util.ObjectToBytes(tx)
}

// GetBytesNoSig returns the serialized the transaction excluding the signature
func (tx *TxRepoProposalFeeSend) GetBytesNoSig() []byte {
	sig := tx.Sig
	tx.Sig = nil
	bz := tx.Bytes()
	tx.Sig = sig
	return bz
}

// ComputeHash computes the hash of the transaction
func (tx *TxRepoProposalFeeSend) ComputeHash() util.Bytes32 {
	return util.BytesToBytes32(util.Blake2b256(tx.Bytes()))
}

// GetHash returns the hash of the transaction
func (tx *TxRepoProposalFeeSend) GetHash() util.Bytes32 {
	return tx.ComputeHash()
}

// GetID returns the id of the transaction (also the hash)
func (tx *TxRepoProposalFeeSend) GetID() string {
	return tx.ComputeHash().HexStr()
}

// GetEcoSize returns the size of the transaction for use in protocol economics
func (tx *TxRepoProposalFeeSend) GetEcoSize() int64 {
	return tx.GetSize()
}

// GetSize returns the size of the tx object (excluding nothing)
func (tx *TxRepoProposalFeeSend) GetSize() int64 {
	return int64(len(tx.Bytes()))
}

// Sign signs the transaction
func (tx *TxRepoProposalFeeSend) Sign(privKey string) ([]byte, error) {
	return SignTransaction(tx, privKey)
}

// ToMap returns a map equivalent of the transaction
func (tx *TxRepoProposalFeeSend) ToMap() map[string]interface{} {
	s := structs.New(tx)
	s.TagName = "json"
	return s.Map()
}

// FromMap populates fields from a map.
// Note: Default or zero values may be set for fields that aren't present in the
// map. Also, an error will be returned when unable to convert types in map to
// actual types in the object.
func (tx *TxRepoProposalFeeSend) FromMap(data map[string]interface{}) error {
	err := tx.TxCommon.FromMap(data)
	err = util.CallOnNilErr(err, func() error { return tx.TxType.FromMap(data) })
	err = util.CallOnNilErr(err, func() error { return tx.TxValue.FromMap(data) })

	o := objx.New(data)

	// RepoName: expects string type in map
	if nameVal := o.Get("name"); !nameVal.IsNil() {
		if nameVal.IsStr() {
			tx.RepoName = nameVal.Str()
		} else {
			return FieldError("name", fmt.Sprintf("invalid value type: has %T, "+
				"wants string", nameVal.Inter()))
		}
	}

	// ProposalID: expects string type in map
	if proposalID := o.Get("id"); !proposalID.IsNil() {
		if proposalID.IsStr() {
			tx.ProposalID = proposalID.Str()
		} else {
			return FieldError("id", fmt.Sprintf("invalid value type: has %T, "+
				"wants string", proposalID.Inter()))
		}
	}

	return err
}
