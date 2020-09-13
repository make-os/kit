package contribcmd

import (
	"fmt"
	"io"
	"strconv"

	"github.com/fatih/color"
	"github.com/make-os/lobe/cmd/common"
	"github.com/make-os/lobe/config"
	"github.com/make-os/lobe/rpc/types"
	api2 "github.com/make-os/lobe/types/api"
	"github.com/make-os/lobe/util/api"
	fmt2 "github.com/make-os/lobe/util/colorfmt"
	"github.com/make-os/lobe/util/identifier"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
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
	RPCClient types.Client

	// KeyUnlocker is a function for getting and unlocking a push key from keystore.
	KeyUnlocker common.KeyUnlocker

	// GetNextNonce is a function for getting the next nonce of an account
	GetNextNonce api.NextNonceGetter

	// SendCoin is a function for sending coins
	SendCoin api.CoinSender

	// ShowTxStatusTracker is a function tracking and displaying tx status
	ShowTxStatusTracker common.TxStatusTrackerFunc

	Stdout io.Writer
}

// SendCmd creates a transaction to send coins from a user
// account to another user or repository account
func SendCmd(cfg *config.AppConfig, args *SendArgs) error {

	// Get and unlock the signing key
	key, err := args.KeyUnlocker(cfg, &common.UnlockKeyArgs{
		KeyStoreID: args.SigningKey,
		Passphrase: args.SigningKeyPass,
		TargetRepo: nil,
		Prompt:     "Enter passphrase to unlock the signing key:\n",
		Stdout:     args.Stdout,
	})
	if err != nil {
		return errors.Wrap(err, "failed to unlock the signing key")
	}

	// If nonce is unset, get the nonce from a remote server
	nonce := args.Nonce
	if nonce == 0 {
		nextNonce, err := args.GetNextNonce(key.GetUserAddress(), args.RPCClient)
		if err != nil {
			return errors.Wrap(err, "failed to get signer's next nonce")
		}
		nonce, _ = strconv.ParseUint(nextNonce, 10, 64)
	}

	body := &api2.BodySendCoin{
		To:         identifier.Address(args.Recipient),
		Nonce:      nonce,
		Value:      args.Value,
		Fee:        args.Fee,
		SigningKey: key.GetKey(),
	}

	// Create the transaction
	hash, err := args.SendCoin(body, args.RPCClient)
	if err != nil {
		return errors.Wrap(err, "failed to send coins")
	}

	// Display transaction info and track status
	if args.Stdout != nil {
		fmt.Fprintln(args.Stdout, fmt2.NewColor(color.FgGreen, color.Bold).Sprint("✅ Transaction sent!"))
		fmt.Fprintln(args.Stdout, " - To:", fmt2.CyanString(args.Recipient))
		fmt.Fprintln(args.Stdout, " - Amount:", fmt2.CyanString(cast.ToString(args.Value)))
		fmt.Fprintln(args.Stdout, " - Hash:", fmt2.CyanString(hash))
		if err := args.ShowTxStatusTracker(args.Stdout, hash, args.RPCClient); err != nil {
			return err
		}
	}

	return nil
}
