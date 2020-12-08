package ed25519

import (
	"github.com/tendermint/tendermint/privval"
)

// FilePV wraps and extends tendermint's privval.FilePV
type FilePV struct {
	*privval.FilePV
}

// GetKey returns the validator's private key coerced to a crypto.Key.
func (pv *FilePV) GetKey() (*Key, error) {
	keyBz := pv.Key.PrivKey
	pk, err := PrivKeyFromBytes(keyBz.Bytes())
	if err != nil {
		return nil, err
	}
	return NewKeyFromPrivKey(pk), nil
}
