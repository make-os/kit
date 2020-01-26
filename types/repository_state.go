package types

import (
	"github.com/makeos/mosdef/util"
	"github.com/mitchellh/mapstructure"
	"github.com/vmihailenco/msgpack"
)

// BareReference returns an empty reference object
func BareReference() *Reference {
	return &Reference{}
}

// Reference represents a git reference
type Reference struct {
	Nonce uint64 `json:"nonce" mapstructure:"nonce" msgpack:"nonce"`
}

// References represents a collection of references
// Note: we are using map[string]interface{} instead of map[string]*Reference
// because we want to take advantage of msgpack map sorting which only works on the
// former.
// CONTRACT: interface{} is always *Reference
type References map[string]interface{}

// Get a reference by name, returns empty reference if not found.
func (r *References) Get(name string) *Reference {
	ref, _ := (*r)[name]
	if ref == nil {
		return BareReference()
	}
	return ref.(*Reference)
}

// Has checks whether a reference exist
func (r *References) Has(name string) bool {
	return (*r)[name] != nil
}

// BareRepository returns an empty repository object
func BareRepository() *Repository {
	return &Repository{
		References: make(map[string]interface{}),
		Owners:     make(map[string]interface{}),
	}
}

// RepoOwner describes an owner of a repository
type RepoOwner struct {
	Creator bool `json:"creator" mapstructure:"creator" msgpack:"creator"`
}

// RepoOwners represents an index of owners of a repository.
// Note: we are using map[string]interface{} instead of map[string]*RepoOwner
// because we want to take advantage of msgpack map sorting which only works on the
// former.
// CONTRACT: interface{} is always *RepoOwner
type RepoOwners map[string]interface{}

// Repository represents a git repository.
type Repository struct {
	References References `json:"references" msgpack:"references"`
	Owners     RepoOwners `json:"owners" msgpack:"owners"`
}

// AddOwner adds an owner
func (r *Repository) AddOwner(ownerPubKey string, owner *RepoOwner) {
	r.Owners[ownerPubKey] = owner
}

// IsNil returns true if the repo fields are set to their nil value
func (r *Repository) IsNil() bool {
	return len(r.References) == 0 && len(r.Owners) == 0
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (r *Repository) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.EncodeMulti(r.References, r.Owners)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (r *Repository) DecodeMsgpack(dec *msgpack.Decoder) error {
	return dec.DecodeMulti(&r.References, &r.Owners)
}

// Bytes return the bytes equivalent of the account
func (r *Repository) Bytes() []byte {
	return util.ObjectToBytes(r)
}

// NewRepositoryFromBytes decodes bz to Repository
func NewRepositoryFromBytes(bz []byte) (*Repository, error) {

	var repo = BareRepository()
	if err := util.BytesToObject(bz, repo); err != nil {
		return nil, err
	}

	for k, v := range repo.References {
		var ref Reference
		mapstructure.Decode(v, &ref)
		repo.References[k] = &ref
	}

	for k, v := range repo.Owners {
		var owner RepoOwner
		mapstructure.Decode(v, &owner)
		repo.AddOwner(k, &owner)
	}

	return repo, nil
}
