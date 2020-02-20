package core

import (
	"fmt"
	"github.com/fatih/structs"
	"github.com/stretchr/objx"
	"github.com/vmihailenco/msgpack"
	"gitlab.com/makeos/mosdef/util"
)

// TxRepoProposalVote implements BaseTx, it describes a transaction for voting
// on a repository proposal
type TxRepoProposalVote struct {
	*TxCommon  `json:",flatten" msgpack:"-" mapstructure:"-"`
	*TxType    `json:",flatten" msgpack:"-" mapstructure:"-"`
	RepoName   string `json:"name" msgpack:"name"`
	ProposalID string `json:"id" msgpack:"id"`
	Vote       int    `json:"vote" msgpack:"vote"`
}

// NewBareRepoProposalVote returns an instance of TxRepoProposalVote with zero values
func NewBareRepoProposalVote() *TxRepoProposalVote {
	return &TxRepoProposalVote{
		TxCommon:   NewBareTxCommon(),
		TxType:     &TxType{Type: TxTypeRepoProposalVote},
		RepoName:   "",
		ProposalID: "",
		Vote:       0,
	}
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (tx *TxRepoProposalVote) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.EncodeMulti(
		tx.Type,
		tx.Nonce,
		tx.Fee,
		tx.Sig,
		tx.Timestamp,
		tx.SenderPubKey,
		tx.RepoName,
		tx.ProposalID,
		tx.Vote)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (tx *TxRepoProposalVote) DecodeMsgpack(dec *msgpack.Decoder) error {
	return tx.DecodeMulti(dec,
		&tx.Type,
		&tx.Nonce,
		&tx.Fee,
		&tx.Sig,
		&tx.Timestamp,
		&tx.SenderPubKey,
		&tx.RepoName,
		&tx.ProposalID,
		&tx.Vote)
}

// Bytes returns the serialized transaction
func (tx *TxRepoProposalVote) Bytes() []byte {
	return util.ObjectToBytes(tx)
}

// GetBytesNoSig returns the serialized the transaction excluding the signature
func (tx *TxRepoProposalVote) GetBytesNoSig() []byte {
	sig := tx.Sig
	tx.Sig = nil
	bz := tx.Bytes()
	tx.Sig = sig
	return bz
}

// ComputeHash computes the hash of the transaction
func (tx *TxRepoProposalVote) ComputeHash() util.Bytes32 {
	return util.BytesToBytes32(util.Blake2b256(tx.Bytes()))
}

// GetHash returns the hash of the transaction
func (tx *TxRepoProposalVote) GetHash() util.Bytes32 {
	return tx.ComputeHash()
}

// GetID returns the id of the transaction (also the hash)
func (tx *TxRepoProposalVote) GetID() string {
	return tx.ComputeHash().HexStr()
}

// GetEcoSize returns the size of the transaction for use in protocol economics
func (tx *TxRepoProposalVote) GetEcoSize() int64 {
	return tx.GetSize()
}

// GetSize returns the size of the tx object (excluding nothing)
func (tx *TxRepoProposalVote) GetSize() int64 {
	return int64(len(tx.Bytes()))
}

// Sign signs the transaction
func (tx *TxRepoProposalVote) Sign(privKey string) ([]byte, error) {
	return SignTransaction(tx, privKey)
}

// ToMap returns a map equivalent of the transaction
func (tx *TxRepoProposalVote) ToMap() map[string]interface{} {
	s := structs.New(tx)
	s.TagName = "json"
	return s.Map()
}

// FromMap populates fields from a map.
// Note: Default or zero values may be set for fields that aren't present in the
// map. Also, an error will be returned when unable to convert types in map to
// actual types in the object.
func (tx *TxRepoProposalVote) FromMap(data map[string]interface{}) error {
	err := tx.TxCommon.FromMap(data)
	err = util.CallOnNilErr(err, func() error { return tx.TxType.FromMap(data) })

	o := objx.New(data)

	// RepoName: expects string type in map
	if nameVal := o.Get("name"); !nameVal.IsNil() {
		if nameVal.IsStr() {
			tx.RepoName = nameVal.Str()
		} else {
			return util.FieldError("name", fmt.Sprintf("invalid value type: has %T, "+
				"wants string", nameVal.Inter()))
		}
	}

	// ProposalID: expects string type in map
	if propIDVal := o.Get("id"); !propIDVal.IsNil() {
		if propIDVal.IsStr() {
			tx.ProposalID = propIDVal.Str()
		} else {
			return util.FieldError("id", fmt.Sprintf("invalid value type: has %T, "+
				"wants string", propIDVal.Inter()))
		}
	}

	// Vote: expects int type in map
	if voteVal := o.Get("vote"); !voteVal.IsNil() {
		if voteVal.IsInt64() {
			tx.Vote = int(voteVal.Int64())
		} else {
			return util.FieldError("vote", fmt.Sprintf("invalid value type: has %T, "+
				"wants int", voteVal.Inter()))
		}
	}

	return err
}
