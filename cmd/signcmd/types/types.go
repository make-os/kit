package types

import (
	"io"

	"github.com/make-os/kit/cmd/common"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/remote/plumbing"
	"github.com/make-os/kit/remote/server"
	rpctypes "github.com/make-os/kit/rpc/types"
	"github.com/make-os/kit/util/api"
)

type SignCommitFunc func(cfg *config.AppConfig, repo plumbing.LocalRepo, args *SignCommitArgs) error

type SignCommitArgs struct {
	// Fee is the network transaction fee
	Fee string

	// Nonce is the signer's next account nonce
	Nonce uint64

	// Value is for sending special fee
	Value string

	// MergeID indicates an optional merge proposal ID to attach to the transaction
	MergeID string

	// Head specifies a reference to use in the transaction info instead of the signed branch reference
	Head string

	// PushKeyID is the signers push key ID
	SigningKey string

	// PushKeyPass is the signers push key passphrase
	PushKeyPass string

	// Remote specifies the remote name whose URL we will attach the push token to
	Remote string

	// ResetTokens clears all push tokens from the remote URL before adding the new one.
	ResetTokens bool

	// NoPrompt prevents key unlocker prompt
	NoPrompt bool

	// RpcClient is the RPC client
	RPCClient rpctypes.Client

	// KeyUnlocker is a function for getting and unlocking a push key from keystore
	KeyUnlocker common.UnlockKeyFunc

	// GetNextNonce is a function for getting the next nonce of the owner account of a pusher key
	GetNextNonce api.NextNonceGetter

	// CreateApplyPushTokenToRemote is a function for creating and applying push tokens on a git remote
	CreateApplyPushTokenToRemote server.MakeAndApplyPushTokenToRemoteFunc

	Stdout io.Writer
	Stderr io.Writer
}

type SignNoteArgs struct {
	// Fee is the network transaction fee
	Fee string

	// Nonce is the signer's next account nonce
	Nonce uint64

	// Value is for sending special fee
	Value string

	// Name is the name of the target note
	Name string

	// PushKeyID is the signers push key ID
	SigningKey string

	// PushKeyPass is the signers push key passphrase
	PushKeyPass string

	// Remote specifies the remote name whose URL we will attach the push token to
	Remote string

	// ResetTokens clears all push tokens from the remote URL before adding the new one.
	ResetTokens bool

	// NoPrompt prevents key unlocker prompt
	NoPrompt bool

	// RpcClient is the RPC client
	RPCClient rpctypes.Client

	// KeyUnlocker is a function for getting and unlocking a push key from keystore
	KeyUnlocker common.UnlockKeyFunc

	// GetNextNonce is a function for getting the next nonce of the owner account of a pusher key
	GetNextNonce api.NextNonceGetter

	CreateApplyPushTokenToRemote server.MakeAndApplyPushTokenToRemoteFunc

	Stdout io.Writer
	Stderr io.Writer
}

type SignNoteFunc func(cfg *config.AppConfig, repo plumbing.LocalRepo, args *SignNoteArgs) error

type SignTagArgs struct {
	// Fee is the network transaction fee
	Fee string

	// Nonce is the signer's next account nonce
	Nonce uint64

	// Value is for sending special fee
	Value string

	// PushKeyID is the signers push key ID
	SigningKey string

	// PushKeyPass is the signers push key passphrase
	PushKeyPass string

	// Remote specifies the remote name whose URL we will attach the push token to
	Remote string

	// ResetTokens clears all push tokens from the remote URL before adding the new one.
	ResetTokens bool

	// NoPrompt prevents key unlocker prompt
	NoPrompt bool

	// RpcClient is the RPC client
	RPCClient rpctypes.Client

	// KeyUnlocker is a function for getting and unlocking a push key from keystore
	KeyUnlocker common.UnlockKeyFunc

	// GetNextNonce is a function for getting the next nonce of the owner account of a pusher key
	GetNextNonce api.NextNonceGetter

	// CreateApplyPushTokenToRemote is a function for creating, signing and apply a push token  to a give remote
	CreateApplyPushTokenToRemote server.MakeAndApplyPushTokenToRemoteFunc

	Stdout io.Writer
	Stderr io.Writer
}

type SignTagFunc func(cfg *config.AppConfig, cmdArg []string, repo plumbing.LocalRepo, args *SignTagArgs) error
