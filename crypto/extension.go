package crypto

import (
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/privval"
)

// WrappedPV extends tendermint's privval.FilePV to
// to support conversion of tendermint keys and address
// to the applications preferred format and conventions.
type WrappedPV struct {
	*privval.FilePV
}

// GetKey returns the validator's private key coerced into
// the applications crypto.Key instance
func (pv *WrappedPV) GetKey() (*Key, error) {
	keyBz := pv.Key.PrivKey.(ed25519.PrivKeyEd25519)
	pk, err := PrivKeyFromBytes(keyBz)
	if err != nil {
		return nil, err
	}
	return NewKeyFromPrivKey(pk), nil
}
