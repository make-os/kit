package api

import (
	modtypes "github.com/make-os/lobe/modules/types"
	"github.com/make-os/lobe/rpc"
	"github.com/make-os/lobe/types/constants"
	"github.com/make-os/lobe/util"
	"github.com/spf13/cast"
	"github.com/stretchr/objx"
)

// UserAPI provides RPC methods for various user-related operations.
type UserAPI struct {
	mods *modtypes.Modules
}

// NewUserAPI creates an instance of UserAPI
func NewUserAPI(mods *modtypes.Modules) *UserAPI {
	return &UserAPI{mods: mods}
}

// getNonce returns the nonce of an account
func (u *UserAPI) getNonce(params interface{}) (resp *rpc.Response) {
	o := objx.New(params)
	address := o.Get("address").Str()
	blockHeight := cast.ToUint64(o.Get("height").Inter())
	nonce := u.mods.User.GetNonce(address, blockHeight)
	return rpc.Success(util.Map{
		"nonce": nonce,
	})
}

// getAccount returns the account corresponding to the given address
func (u *UserAPI) getAccount(params interface{}) (resp *rpc.Response) {
	o := objx.New(params)
	address := o.Get("address").Str()
	blockHeight := cast.ToUint64(o.Get("height").Inter())
	account := u.mods.User.GetAccount(address, blockHeight)
	return rpc.Success(account)
}

// getBalance returns the spendable balance of account corresponding to the given address
func (u *UserAPI) getBalance(params interface{}) (resp *rpc.Response) {
	o := objx.New(params)
	address := o.Get("address").Str()
	blockHeight := cast.ToUint64(o.Get("height").Inter())
	bal := u.mods.User.GetAvailableBalance(address, blockHeight)
	return rpc.Success(util.Map{
		"balance": bal,
	})
}

// getStakedBalance returns the staked coin balance of account
func (u *UserAPI) getStakedBalance(params interface{}) (resp *rpc.Response) {
	o := objx.New(params)
	address := o.Get("address").Str()
	blockHeight := cast.ToUint64(o.Get("height").Inter())
	bal := u.mods.User.GetStakedBalance(address, blockHeight)
	return rpc.Success(util.Map{
		"balance": bal,
	})
}

// sendCoin creates a transaction to transfer coin from a user account to a user/repo account.
func (u *UserAPI) sendCoin(params interface{}) (resp *rpc.Response) {
	return rpc.Success(u.mods.User.SendCoin(cast.ToStringMap(params)))
}

// getKeys returns a list of addresses of keys on the keystore.
func (u *UserAPI) getKeys(interface{}) (resp *rpc.Response) {
	return rpc.Success(util.Map{
		"addresses": u.mods.User.GetKeys(),
	})
}

// getValidator returns the validator information of the node.
// The private key is not returned if request is not local.
func (u *UserAPI) getValidator(params interface{}, ctx *rpc.CallContext) (resp *rpc.Response) {
	includePK := cast.ToBool(params)
	if !ctx.IsLocal {
		includePK = false
	}
	return rpc.Success(u.mods.User.GetValidator(includePK))
}

// getPrivateKey returns the private key of a given key.
// Empty string will be returned if request is not local.
func (u *UserAPI) getPrivateKey(params interface{}, ctx *rpc.CallContext) (resp *rpc.Response) {
	if !ctx.IsLocal {
		return rpc.Success(util.Map{"privkey": ""})
	}

	o := objx.New(params)
	address := o.Get("address").Str()
	pass := o.Get("passphrase").Str()
	return rpc.Success(util.Map{
		"privkey": u.mods.User.GetPrivKey(address, pass),
	})
}

// getPublicKey returns the public key of a given key.
func (u *UserAPI) getPublicKey(params interface{}) (resp *rpc.Response) {
	o := objx.New(params)
	address := o.Get("address").Str()
	pass := o.Get("passphrase").Str()
	return rpc.Success(util.Map{
		"pubkey": u.mods.User.GetPublicKey(address, pass),
	})
}

// APIs returns all API handlers
func (u *UserAPI) APIs() rpc.APISet {
	return []rpc.MethodInfo{
		{
			Name:        "getNonce",
			Namespace:   constants.NamespaceUser,
			Description: "Get the nonce of an account",
			Func:        u.getNonce,
		},
		{
			Name:        "get",
			Namespace:   constants.NamespaceUser,
			Description: "Get the account corresponding to an address",
			Func:        u.getAccount,
		},
		{
			Name:        "getBalance",
			Namespace:   constants.NamespaceUser,
			Description: "Get the spendable balance of an account",
			Func:        u.getBalance,
		},
		{
			Name:        "getStakedBalance",
			Namespace:   constants.NamespaceUser,
			Description: "Get the staked coin balance of an account",
			Func:        u.getStakedBalance,
		},
		{
			Name:        "send",
			Namespace:   constants.NamespaceUser,
			Description: "Send coins to another user account or a repository",
			Func:        u.sendCoin,
		},
		{
			Name:        "getValidator",
			Namespace:   constants.NamespaceUser,
			Description: "Get the validator information of the node",
			Func:        u.getValidator,
			Private:     true,
		},
		{
			Name:        "getKeys",
			Namespace:   constants.NamespaceUser,
			Private:     true,
			Description: "Get addresses of keys on the keystore",
			Func:        u.getKeys,
		},
		{
			Name:        "getPrivKey",
			Namespace:   constants.NamespaceUser,
			Private:     true,
			Description: "Get the private key of a key on the keystore",
			Func:        u.getPrivateKey,
		},
		{
			Name:        "getPubKey",
			Namespace:   constants.NamespaceUser,
			Private:     true,
			Description: "Get the public key of a key on the keystore",
			Func:        u.getPublicKey,
		},
	}
}
