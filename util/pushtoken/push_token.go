package pushtoken

import (
	"fmt"

	"github.com/make-os/kit/crypto/ed25519"
	"github.com/make-os/kit/keystore/types"
	remotetypes "github.com/make-os/kit/remote/types"
	"github.com/make-os/kit/util"
	"github.com/mr-tron/base58"
)

var (
	ErrMalformedToken = fmt.Errorf("malformed token")
)

// Decode decodes a push request token.
func Decode(v string) (*remotetypes.TxDetail, error) {
	bz, err := base58.Decode(v)
	if err != nil {
		return nil, ErrMalformedToken
	}

	var txDetail remotetypes.TxDetail
	if err = util.ToObject(bz, &txDetail); err != nil {
		return nil, ErrMalformedToken
	}

	return &txDetail, nil
}

// IsValid checks whether t is a valid push token
func IsValid(t string) bool {
	_, err := Decode(t)
	return err == nil
}

// Make creates a push request token
func Make(key types.StoredKey, txDetail *remotetypes.TxDetail) string {
	return MakeFromKey(key.GetKey(), txDetail)
}

// MakeFromKey creates a push request token
func MakeFromKey(key *ed25519.Key, txDetail *remotetypes.TxDetail) string {
	sig, _ := key.PrivKey().Sign(txDetail.BytesNoSig())
	txDetail.Signature = base58.Encode(sig)
	return base58.Encode(txDetail.Bytes())
}
