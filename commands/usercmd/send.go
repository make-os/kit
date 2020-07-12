package contribcmd

import (
	"fmt"
	"io"
	"strconv"

	"github.com/fatih/color"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
	restclient "gitlab.com/makeos/mosdef/api/remote/client"
	"gitlab.com/makeos/mosdef/api/rpc/client"
	"gitlab.com/makeos/mosdef/api/types"
	"gitlab.com/makeos/mosdef/api/utils"
	"gitlab.com/makeos/mosdef/commands/common"
	"gitlab.com/makeos/mosdef/config"
	fmt2 "gitlab.com/makeos/mosdef/util/colorfmt"
	"gitlab.com/makeos/mosdef/util/identifier"
)

// SendArgs contains arguments for SendCmd.
type SendArgs struct {

	// Recipient is the receiving address
	Recipient string

	// Value is amount of coin to send
	Value float64

	// Nonce is the next nonce of the signing key's account
	Nonce uint64

	// Fee is the transaction fee to be paid by the signing key
	Fee float64

	// SigningKey is the account whose key will be used to sign the transaction.
	SigningKey string

	// AccountPass is the passphrase for unlocking the signing key.
	SigningKeyPass string

	// RPCClient is the RPC client
	RPCClient client.Client

	// RemoteClients is the remote server API client.
	RemoteClients []restclient.Client

	// KeyUnlocker is a function for getting and unlocking a push key from keystore.
	KeyUnlocker common.KeyUnlocker

	// GetNextNonce is a function for getting the next nonce of an account
	GetNextNonce utils.NextNonceGetter

	// SendCoin is a function for sending coins
	SendCoin utils.CoinSender

	Stdout io.Writer
}

// SendCmd creates a transaction to send coins from a user
// account to another user or repository account
func SendCmd(cfg *config.AppConfig, args *SendArgs) error {

	// Get and unlock the signing key
	key, err := args.KeyUnlocker(cfg, &common.UnlockKeyArgs{
		KeyAddrOrIdx: args.SigningKey,
		Passphrase:   args.SigningKeyPass,
		AskPass:      true,
		TargetRepo:   nil,
		Prompt:       "Enter passphrase to unlock signing key:\n",
		Stdout:       args.Stdout,
	})
	if err != nil {
		return errors.Wrap(err, "failed to unlock the signing key")
	}

	// If nonce is unset, get the nonce from a remote server
	nonce := args.Nonce
	if nonce == 0 {
		nextNonce, err := args.GetNextNonce(key.GetAddress(), args.RPCClient, args.RemoteClients)
		if err != nil {
			return errors.Wrap(err, "failed to get signer's next nonce")
		}
		nonce, _ = strconv.ParseUint(nextNonce, 10, 64)
	}

	body := &types.SendCoinBody{
		To:         identifier.Address(args.Recipient),
		Nonce:      nonce,
		Value:      args.Value,
		Fee:        args.Fee,
		SigningKey: key.GetKey(),
	}

	// Create the transaction
	hash, err := args.SendCoin(body, args.RPCClient, args.RemoteClients)
	if err != nil {
		return errors.Wrap(err, "failed to send coins")
	}

	if args.Stdout != nil {
		fmt.Fprintln(args.Stdout, fmt2.NewColor(color.FgGreen, color.Bold).Sprint("✅ Transaction sent!"))
		fmt.Fprintln(args.Stdout, " - To:", fmt2.CyanString(args.Recipient))
		fmt.Fprintln(args.Stdout, " - Amount:", fmt2.CyanString(cast.ToString(args.Value)))
		fmt.Fprintln(args.Stdout, " - Hash:", fmt2.CyanString(hash))
	}

	return nil
}
