package txns

import (
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/errors"
	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/vmihailenco/msgpack"
)

// TxRepoProposalRegisterPushKey implements BaseTx, it describes a repository proposal
// transaction for adding one or more contributors to a repository
type TxRepoProposalRegisterPushKey struct {
	*TxCommon         `json:",flatten" msgpack:"-" mapstructure:"-"`
	*TxType           `json:",flatten" msgpack:"-" mapstructure:"-"`
	*TxProposalCommon `json:",flatten" msgpack:"-" mapstructure:"-"`
	PushKeys          []string                   `json:"keys" msgpack:"keys" mapstructure:"keys"`
	Policies          []*state.ContributorPolicy `json:"policies" msgpack:"policies,omitempty" mapstructure:"policies,omitempty"`
	FeeMode           state.FeeMode              `json:"feeMode" msgpack:"feeMode,omitempty" mapstructure:"feeMode,omitempty"`
	FeeCap            util.String                `json:"feeCap" msgpack:"feeCap,omitempty" mapstructure:"feeCap,omitempty"`
	Namespace         string                     `json:"namespace" msgpack:"namespace,omitempty" mapstructure:"namespace"`
	NamespaceOnly     string                     `json:"namespaceOnly" msgpack:"namespaceOnly,omitempty" mapstructure:"namespaceOnly"`
}

// NewBareRepoProposalRegisterPushKey returns an instance of TxRepoProposalRegisterPushKey with zero values
func NewBareRepoProposalRegisterPushKey() *TxRepoProposalRegisterPushKey {
	return &TxRepoProposalRegisterPushKey{
		TxCommon:         NewBareTxCommon(),
		TxType:           &TxType{Type: TxTypeRepoProposalRegisterPushKey},
		TxProposalCommon: &TxProposalCommon{Value: "0", RepoName: "", ID: ""},
		PushKeys:         []string{},
		Policies:         []*state.ContributorPolicy{},
		FeeMode:          state.FeeModePusherPays,
	}
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (tx *TxRepoProposalRegisterPushKey) EncodeMsgpack(enc *msgpack.Encoder) error {
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
		tx.PushKeys,
		tx.Policies,
		tx.FeeMode,
		tx.FeeCap,
		tx.Namespace,
		tx.NamespaceOnly)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (tx *TxRepoProposalRegisterPushKey) DecodeMsgpack(dec *msgpack.Decoder) error {
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
		&tx.PushKeys,
		&tx.Policies,
		&tx.FeeMode,
		&tx.FeeCap,
		&tx.Namespace,
		&tx.NamespaceOnly)
}

// Bytes returns the serialized transaction
func (tx *TxRepoProposalRegisterPushKey) Bytes() []byte {
	return util.ToBytes(tx)
}

// GetBytesNoSig returns the serialized the transaction excluding the signature
func (tx *TxRepoProposalRegisterPushKey) GetBytesNoSig() []byte {
	sig := tx.Sig
	tx.Sig = nil
	bz := tx.Bytes()
	tx.Sig = sig
	return bz
}

// ComputeHash computes the hash of the transaction
func (tx *TxRepoProposalRegisterPushKey) ComputeHash() util.Bytes32 {
	return util.BytesToBytes32(tmhash.Sum(tx.Bytes()))
}

// GetHash returns the hash of the transaction
func (tx *TxRepoProposalRegisterPushKey) GetHash() util.HexBytes {
	return tx.ComputeHash().ToHexBytes()
}

// GetID returns the id of the transaction (also the hash)
func (tx *TxRepoProposalRegisterPushKey) GetID() string {
	return tx.ComputeHash().HexStr()
}

// GetEcoSize returns the size of the transaction for use in protocol economics
func (tx *TxRepoProposalRegisterPushKey) GetEcoSize() int64 {
	return tx.GetSize()
}

// GetSize returns the size of the tx object (excluding nothing)
func (tx *TxRepoProposalRegisterPushKey) GetSize() int64 {
	return int64(len(tx.Bytes()))
}

// Sign signs the transaction
func (tx *TxRepoProposalRegisterPushKey) Sign(privKey string) ([]byte, error) {
	return SignTransaction(tx, privKey)
}

// ToMap returns a map equivalent of the transaction
func (tx *TxRepoProposalRegisterPushKey) ToMap() map[string]interface{} {
	return util.ToJSONMap(tx)
}

// FromMap populates fields from a map.
// An error will be returned when unable to convert types in map to expected types in the object.
func (tx *TxRepoProposalRegisterPushKey) FromMap(data map[string]interface{}) error {
	err := tx.TxCommon.FromMap(data)
	err = errors.CallIfNil(err, func() error { return tx.TxType.FromMap(data) })
	err = errors.CallIfNil(err, func() error { return tx.TxProposalCommon.FromMap(data) })
	err = errors.CallIfNil(err, func() error { return util.DecodeMap(data, &tx) })
	return err
}
