package txns

import (
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/errors"
	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/vmihailenco/msgpack"
)

// TxRepoProposalUpdate implements BaseTx, it describes a repository proposal
// transaction for updating a repository
type TxRepoProposalUpdate struct {
	*TxCommon         `json:",flatten" msgpack:"-" mapstructure:"-"`
	*TxType           `json:",flatten" msgpack:"-" mapstructure:"-"`
	*TxProposalCommon `json:",flatten" msgpack:"-" mapstructure:"-"`
	*TxDescription    `json:",flatten" msgpack:"-" mapstructure:"-"`
	Config            *state.RepoConfig `json:"config" msgpack:"config" mapstructure:"config"`
}

// NewBareRepoProposalUpdate returns an instance of TxRepoProposalUpdate with zero values
func NewBareRepoProposalUpdate() *TxRepoProposalUpdate {
	return &TxRepoProposalUpdate{
		TxCommon:         NewBareTxCommon(),
		TxType:           &TxType{Type: TxTypeRepoProposalUpdate},
		TxProposalCommon: &TxProposalCommon{Value: "0", RepoName: "", ID: ""},
		TxDescription:    &TxDescription{Description: ""},
		Config:           &state.RepoConfig{},
	}
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (tx *TxRepoProposalUpdate) EncodeMsgpack(enc *msgpack.Encoder) error {
	return tx.EncodeMulti(enc,
		tx.Type,
		tx.Nonce,
		tx.Value,
		tx.Fee,
		tx.Sig,
		tx.Timestamp,
		tx.SenderPubKey,
		tx.RepoName,
		tx.ID,
		tx.Description,
		tx.Config.ToMap(),
	)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (tx *TxRepoProposalUpdate) DecodeMsgpack(dec *msgpack.Decoder) error {
	return tx.DecodeMulti(dec,
		&tx.Type,
		&tx.Nonce,
		&tx.Value,
		&tx.Fee,
		&tx.Sig,
		&tx.Timestamp,
		&tx.SenderPubKey,
		&tx.RepoName,
		&tx.ID,
		&tx.Description,
		&tx.Config)
}

// Bytes returns the serialized transaction
func (tx *TxRepoProposalUpdate) Bytes() []byte {
	return util.ToBytes(tx)
}

// GetBytesNoSig returns the serialized the transaction excluding the signature
func (tx *TxRepoProposalUpdate) GetBytesNoSig() []byte {
	sig := tx.Sig
	tx.Sig = nil
	bz := tx.Bytes()
	tx.Sig = sig
	return bz
}

// ComputeHash computes the hash of the transaction
func (tx *TxRepoProposalUpdate) ComputeHash() util.Bytes32 {
	return util.BytesToBytes32(tmhash.Sum(tx.Bytes()))
}

// GetHash returns the hash of the transaction
func (tx *TxRepoProposalUpdate) GetHash() util.HexBytes {
	return tx.ComputeHash().ToHexBytes()
}

// GetID returns the id of the transaction (also the hash)
func (tx *TxRepoProposalUpdate) GetID() string {
	return tx.ComputeHash().HexStr()
}

// GetEcoSize returns the size of the transaction for use in protocol economics
func (tx *TxRepoProposalUpdate) GetEcoSize() int64 {
	return tx.GetSize()
}

// GetSize returns the size of the tx object (excluding nothing)
func (tx *TxRepoProposalUpdate) GetSize() int64 {
	return int64(len(tx.Bytes()))
}

// Sign signs the transaction
func (tx *TxRepoProposalUpdate) Sign(privKey string) ([]byte, error) {
	return SignTransaction(tx, privKey)
}

// ToMap returns a map equivalent of the transaction
func (tx *TxRepoProposalUpdate) ToMap() map[string]interface{} {
	return util.ToJSONMap(tx)
}

// FromMap populates tx with a map generated by tx.ToMap.
func (tx *TxRepoProposalUpdate) FromMap(data map[string]interface{}) error {
	err := tx.TxCommon.FromMap(data)
	err = errors.CallIfNil(err, func() error { return tx.TxType.FromMap(data) })
	err = errors.CallIfNil(err, func() error { return tx.TxDescription.FromMap(data) })
	err = errors.CallIfNil(err, func() error { return tx.TxProposalCommon.FromMap(data) })
	err = errors.CallIfNil(err, func() error { return util.DecodeMap(data, &tx) })
	return err
}
