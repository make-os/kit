package types

import "github.com/makeos/mosdef/util"

// BareRepository returns an empty repository object
func BareRepository() *Repository {
	return &Repository{}
}

// Repository represents a git repository
type Repository struct {
	CreatorPubKey util.String `json:"creatorPubKey"`
}

// IsNil returns true if the repo fields are set to their nil value
func (r *Repository) IsNil() bool {
	return *r == Repository{}
}

// Bytes return the bytes equivalent of the account
func (r *Repository) Bytes() []byte {
	return util.ObjectToBytes([]interface{}{
		r.CreatorPubKey.String(),
	})
}

// NewRepositoryFromBytes decodes bz to Repository
func NewRepositoryFromBytes(bz []byte) (*Repository, error) {
	var values []interface{}
	if err := util.BytesToObject(bz, &values); err != nil {
		return nil, err
	}
	return &Repository{
		CreatorPubKey: util.String(values[0].(string)),
	}, nil
}
