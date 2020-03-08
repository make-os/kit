package core

import (
	"fmt"
	"strings"

	"github.com/fatih/structs"
	"github.com/stretchr/objx"
	"github.com/vmihailenco/msgpack"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/util"
)

// TxRepoProposalRegisterGPGKey implements BaseTx, it describes a repository proposal
// transaction for adding a new owner to a repository
type TxRepoProposalRegisterGPGKey struct {
	*TxCommon         `json:",flatten" msgpack:"-" mapstructure:"-"`
	*TxType           `json:",flatten" msgpack:"-" mapstructure:"-"`
	*TxProposalCommon `json:",flatten" msgpack:"-" mapstructure:"-"`
	KeyIDs            []string               `json:"ids" msgpack:"ids" mapstructure:"ids"`
	Policies          []*state.RepoACLPolicy `json:"policies" msgpack:"policies,omitempty" mapstructure:"policies,omitempty"`
	FeeMode           state.FeeMode          `json:"feeMode" msgpack:"feeMode,omitempty" mapstructure:"feeMode,omitempty"`
	FeeCap            util.String            `json:"feeCap" msgpack:"feeCap,omitempty" mapstructure:"feeCap,omitempty"`
}

// NewBareRepoProposalRegisterGPGKey returns an instance of TxRepoProposalRegisterGPGKey with zero values
func NewBareRepoProposalRegisterGPGKey() *TxRepoProposalRegisterGPGKey {
	return &TxRepoProposalRegisterGPGKey{
		TxCommon:         NewBareTxCommon(),
		TxType:           &TxType{Type: TxTypeRepoProposalRegisterGPGKey},
		TxProposalCommon: &TxProposalCommon{Value: "0", RepoName: "", ProposalID: ""},
		KeyIDs:           []string{},
		Policies:         []*state.RepoACLPolicy{},
	}
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (tx *TxRepoProposalRegisterGPGKey) EncodeMsgpack(enc *msgpack.Encoder) error {
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
		tx.KeyIDs,
		tx.Policies,
		tx.FeeMode,
		tx.FeeCap)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (tx *TxRepoProposalRegisterGPGKey) DecodeMsgpack(dec *msgpack.Decoder) error {
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
		&tx.KeyIDs,
		&tx.Policies,
		&tx.FeeMode,
		&tx.FeeCap)
}

// Bytes returns the serialized transaction
func (tx *TxRepoProposalRegisterGPGKey) Bytes() []byte {
	return util.ToBytes(tx)
}

// GetBytesNoSig returns the serialized the transaction excluding the signature
func (tx *TxRepoProposalRegisterGPGKey) GetBytesNoSig() []byte {
	sig := tx.Sig
	tx.Sig = nil
	bz := tx.Bytes()
	tx.Sig = sig
	return bz
}

// ComputeHash computes the hash of the transaction
func (tx *TxRepoProposalRegisterGPGKey) ComputeHash() util.Bytes32 {
	return util.BytesToBytes32(util.Blake2b256(tx.Bytes()))
}

// GetHash returns the hash of the transaction
func (tx *TxRepoProposalRegisterGPGKey) GetHash() util.Bytes32 {
	return tx.ComputeHash()
}

// GetID returns the id of the transaction (also the hash)
func (tx *TxRepoProposalRegisterGPGKey) GetID() string {
	return tx.ComputeHash().HexStr()
}

// GetEcoSize returns the size of the transaction for use in protocol economics
func (tx *TxRepoProposalRegisterGPGKey) GetEcoSize() int64 {
	return tx.GetSize()
}

// GetSize returns the size of the tx object (excluding nothing)
func (tx *TxRepoProposalRegisterGPGKey) GetSize() int64 {
	return int64(len(tx.Bytes()))
}

// Sign signs the transaction
func (tx *TxRepoProposalRegisterGPGKey) Sign(privKey string) ([]byte, error) {
	return SignTransaction(tx, privKey)
}

// ToMap returns a map equivalent of the transaction
func (tx *TxRepoProposalRegisterGPGKey) ToMap() map[string]interface{} {
	s := structs.New(tx)
	s.TagName = "json"
	return s.Map()
}

// FromMap populates fields from a map.
// An error will be returned when unable to convert types in map to expected types in the object.
func (tx *TxRepoProposalRegisterGPGKey) FromMap(data map[string]interface{}) error {
	err := tx.TxCommon.FromMap(data)
	err = util.CallOnNilErr(err, func() error { return tx.TxType.FromMap(data) })
	err = util.CallOnNilErr(err, func() error { return tx.TxProposalCommon.FromMap(data) })

	o := objx.New(data)

	// KeyIDs: expects string or slice of string types in map
	if gpgID := o.Get("ids"); !gpgID.IsNil() {
		if gpgID.IsStr() {
			tx.KeyIDs = strings.Split(gpgID.Str(), ",")
		} else if gpgID.IsStrSlice() {
			tx.KeyIDs = gpgID.StrSlice()
		} else {
			return util.FieldError("gpgID", fmt.Sprintf("invalid value type: has %T, "+
				"wants string|[]string", gpgID.Inter()))
		}
	}

	// Policies: expects slice in map
	if acl := o.Get("policies"); !acl.IsNil() {
		if acl.IsMSISlice() {
			var policies []*state.RepoACLPolicy
			for _, m := range acl.MSISlice() {
				var p state.RepoACLPolicy
				_ = util.DecodeMap(m, &p)
				policies = append(policies, &p)
			}
			tx.Policies = policies
		} else {
			return util.FieldError("policies", fmt.Sprintf("invalid value type: has %T, "+
				"wants []map[string]interface{}", acl.Inter()))
		}
	}

	// FeeMode: expects int64 type in map
	if feeMode := o.Get("feeMode"); !feeMode.IsNil() {
		if feeMode.IsInt64() {
			tx.FeeMode = state.FeeMode(feeMode.Int64())
		} else {
			return util.FieldError("feeMode", fmt.Sprintf(
				"invalid value type: has %T, wants string", feeMode.Inter()))
		}
	}

	// FeeCap: expects int64, float64 or string types in map
	if feeCap := o.Get("feeCap"); !feeCap.IsNil() {
		if feeCap.IsInt64() || feeCap.IsFloat64() {
			tx.FeeCap = util.String(fmt.Sprintf("%v", feeCap.Inter()))
		} else if feeCap.IsStr() {
			tx.FeeCap = util.String(feeCap.Str())
		} else {
			return util.FieldError("feeCap", fmt.Sprintf("invalid value type: has %T, "+
				"wants string|int64|float", feeCap.Inter()))
		}
	}

	return err
}
