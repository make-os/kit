package types

import (
	"github.com/makeos/mosdef/util"
	"github.com/vmihailenco/msgpack"
)

// BareReference returns an empty reference object
func BareReference() *Reference {
	return &Reference{}
}

// Reference represents a git reference
type Reference struct {
	Nonce uint64 `json:"nonce" msgpack:"nonce"`
}

// References represents a collection of references
type References map[string]*Reference

// Get a reference by name, returns empty reference if not found.
func (r *References) Get(name string) *Reference {
	ref, _ := (*r)[name]
	if ref == nil {
		return BareReference()
	}
	return ref
}

// BareRepository returns an empty repository object
func BareRepository() *Repository {
	return &Repository{}
}

// Repository represents a git repository
type Repository struct {
	CreatorPubKey util.String `json:"creatorPubKey" msgpack:"creatorPubKey"`
	References    References  `json:"references" msgpack:"references"`
}

// IsNil returns true if the repo fields are set to their nil value
func (r *Repository) IsNil() bool {
	return r.CreatorPubKey.Empty() && len(r.References) == 0
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (r *Repository) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.EncodeMulti(r.CreatorPubKey, r.References)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (r *Repository) DecodeMsgpack(dec *msgpack.Decoder) error {
	return dec.DecodeMulti(&r.CreatorPubKey, &r.References)
}

// Bytes return the bytes equivalent of the account
func (r *Repository) Bytes() []byte {
	return util.ObjectToBytes(r)
}

// NewRepositoryFromBytes decodes bz to Repository
func NewRepositoryFromBytes(bz []byte) (*Repository, error) {
	var repo Repository
	if err := util.BytesToObject(bz, &repo); err != nil {
		return nil, err
	}
	return &repo, nil
}
