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

// DecodePushToken decodes a push request token.
func DecodePushToken(v string) (*remotetypes.TxDetail, error) {
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

// IsValidPushToken checks whether t is a valid push token
func IsValidPushToken(t string) bool {
	_, err := DecodePushToken(t)
	if err != nil {
		return false
	}
	return true
}

// MakePushToken creates a push request token
func MakePushToken(key types.StoredKey, txDetail *remotetypes.TxDetail) string {
	return MakePushTokenFromKey(key.GetKey(), txDetail)
}

// MakePushTokenFromKey creates a push request token
func MakePushTokenFromKey(key *ed25519.Key, txDetail *remotetypes.TxDetail) string {
	sig, _ := key.PrivKey().Sign(txDetail.BytesNoSig())
	txDetail.Signature = base58.Encode(sig)
	return base58.Encode(txDetail.Bytes())
}
