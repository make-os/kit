package client

import (
	"time"

	"github.com/make-os/lobe/types/api"
	"github.com/make-os/lobe/types/state"
	"github.com/make-os/lobe/types/txns"
	"github.com/make-os/lobe/util"
	"github.com/spf13/cast"
)

// UserAPI provides access to user-related RPC methods
type UserAPI struct {
	c *RPCClient
}

// Get gets an account corresponding to a given address
func (u *UserAPI) Get(address string, blockHeight ...uint64) (*api.ResultAccount, error) {

	var height uint64
	if len(blockHeight) > 0 {
		height = blockHeight[0]
	}

	resp, status, err := u.c.call("user_get", util.Map{"address": address, "height": height})
	if err != nil {
		return nil, makeStatusErrorFromCallErr(status, err)
	}

	r := &api.ResultAccount{Account: state.BareAccount()}
	if err = r.Account.FromMap(resp); err != nil {
		return nil, util.ReqErr(500, ErrCodeDecodeFailed, "", err.Error())
	}

	return r, nil
}

// Send sends coins from a user account to another account or repository
func (u *UserAPI) Send(body *api.BodySendCoin) (*api.ResultHash, error) {

	if body.SigningKey == nil {
		return nil, util.ReqErr(400, ErrCodeBadParam, "signingKey", "signing key is required")
	}

	tx := txns.NewBareTxCoinTransfer()
	tx.Nonce = body.Nonce
	tx.Value = util.String(cast.ToString(body.Value))
	tx.Fee = util.String(cast.ToString(body.Fee))
	tx.Timestamp = time.Now().Unix()
	tx.To = body.To
	tx.SenderPubKey = body.SigningKey.PubKey().ToPublicKey()

	// Sign the tx
	var err error
	tx.Sig, err = tx.Sign(body.SigningKey.PrivKey().Base58())
	if err != nil {
		return nil, util.ReqErr(400, ErrCodeClient, "privkey", err.Error())
	}

	resp, status, err := u.c.call("user_send", tx.ToMap())
	if err != nil {
		return nil, makeStatusErrorFromCallErr(status, err)
	}

	var r api.ResultHash
	if err = util.DecodeMap(resp, &r); err != nil {
		return nil, util.ReqErr(500, ErrCodeDecodeFailed, "", err.Error())
	}

	return &r, nil
}

// GetNonce gets the nonce of a user account corresponding to the given address
func (u *UserAPI) GetNonce(address string, blockHeight ...uint64) (uint64, error) {

	var height uint64
	if len(blockHeight) > 0 {
		height = blockHeight[0]
	}

	resp, status, err := u.c.call("user_getNonce", util.Map{"address": address, "height": height})
	if err != nil {
		return 0, makeStatusErrorFromCallErr(status, err)
	}

	return cast.ToUint64(resp["nonce"]), nil
}

// GetKeys finds an account by address
func (u *UserAPI) GetKeys() ([]string, error) {

	resp, status, err := u.c.call("user_getKeys", nil)
	if err != nil {
		return nil, makeStatusErrorFromCallErr(status, err)
	}

	return cast.ToStringSlice(resp["addresses"]), nil
}

// GetBalance returns the spendable balance of an account
func (u *UserAPI) GetBalance(address string, blockHeight ...uint64) (float64, error) {

	var height uint64
	if len(blockHeight) > 0 {
		height = blockHeight[0]
	}

	resp, status, err := u.c.call("user_getBalance", util.Map{"address": address, "height": height})
	if err != nil {
		return 0, makeStatusErrorFromCallErr(status, err)
	}

	return cast.ToFloat64(resp["balance"]), nil
}

// GetStakedBalance returns the staked coin balance of an account
func (u *UserAPI) GetStakedBalance(address string, blockHeight ...uint64) (float64, error) {

	var height uint64
	if len(blockHeight) > 0 {
		height = blockHeight[0]
	}

	resp, status, err := u.c.call("user_getStakedBalance", util.Map{
		"address": address,
		"height":  height})
	if err != nil {
		return 0, makeStatusErrorFromCallErr(status, err)
	}

	return cast.ToFloat64(resp["balance"]), nil
}

// GetValidator get the validator information of the node
func (u *UserAPI) GetValidator(includePrivKey bool) (*api.ResultValidatorInfo, error) {

	resp, status, err := u.c.call("user_getValidator", includePrivKey)
	if err != nil {
		return nil, makeStatusErrorFromCallErr(status, err)
	}

	var r api.ResultValidatorInfo
	if err = util.DecodeMap(resp, &r); err != nil {
		return nil, util.ReqErr(500, ErrCodeDecodeFailed, "", err.Error())
	}

	return &r, nil
}

// GetPrivateKey returns the private key of a key on the keystore
func (u *UserAPI) GetPrivateKey(address string, passphrase string) (string, error) {
	resp, status, err := u.c.call("user_getPrivKey", util.Map{"address": address, "passphrase": passphrase})
	if err != nil {
		return "", makeStatusErrorFromCallErr(status, err)
	}
	return cast.ToString(resp["privkey"]), nil
}

// GetPublicKey returns the public key of a key on the keystore
func (u *UserAPI) GetPublicKey(address string, passphrase string) (string, error) {
	resp, status, err := u.c.call("user_getPubKey", util.Map{"address": address, "passphrase": passphrase})
	if err != nil {
		return "", makeStatusErrorFromCallErr(status, err)
	}
	return cast.ToString(resp["pubkey"]), nil
}

// SetCommission update the validator commission percentage of an account
func (u *UserAPI) SetCommission(body *api.BodySetCommission) (*api.ResultHash, error) {

	if body.SigningKey == nil {
		return nil, util.ReqErr(400, ErrCodeBadParam, "signingKey", "signing key is required")
	}

	tx := txns.NewBareTxSetDelegateCommission()
	tx.Nonce = body.Nonce
	tx.Fee = util.String(cast.ToString(body.Fee))
	tx.Timestamp = time.Now().Unix()
	tx.Commission = util.String(cast.ToString(body.Commission))
	tx.SenderPubKey = body.SigningKey.PubKey().ToPublicKey()

	var err error
	tx.Sig, err = tx.Sign(body.SigningKey.PrivKey().Base58())
	if err != nil {
		return nil, util.ReqErr(400, ErrCodeClient, "privkey", err.Error())
	}

	resp, status, err := u.c.call("user_setCommission", tx.ToMap())
	if err != nil {
		return nil, makeStatusErrorFromCallErr(status, err)
	}

	var r api.ResultHash
	if err = util.DecodeMap(resp, &r); err != nil {
		return nil, util.ReqErr(500, ErrCodeDecodeFailed, "", err.Error())
	}

	return &r, nil
}
