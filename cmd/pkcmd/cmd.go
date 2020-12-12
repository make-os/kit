package pkcmd

import (
	"fmt"
	"os"

	"github.com/make-os/kit/cmd/common"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/util/api"
	"github.com/spf13/cobra"
)

var (
	cfg = config.GetConfig()
	log = cfg.G().Log
)

// pushKeyCmd represents
var pushKeyCmd = &cobra.Command{
	Use:   "pk",
	Short: "Register and manage network push keys",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

// PushKeyRegCmd represents a sub-command to register a public key as a push key
var PushKeyRegCmd = &cobra.Command{
	Use:   "register [flags] <publicKey|keyId>",
	Short: "Register a public key on the network",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("account ID or public key is required")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		target := args[0]
		targetPass, _ := cmd.Flags().GetString("pass")
		fee, _ := cmd.Flags().GetFloat64("fee")
		signingKey, _ := cmd.Flags().GetString("signing-key")
		signingKeyPass, _ := cmd.Flags().GetString("signing-key-pass")
		nonce, _ := cmd.Flags().GetUint64("nonce")
		scopes, _ := cmd.Flags().GetStringSlice("scopes")
		feeCap, _ := cmd.Flags().GetFloat64("feeCap")

		_, client := common.GetRepoAndClient(cfg, "")
		if err := RegisterCmd(cfg, &RegisterArgs{
			Target:              target,
			TargetPass:          targetPass,
			Fee:                 fee,
			SigningKey:          signingKey,
			SigningKeyPass:      signingKeyPass,
			Nonce:               nonce,
			RPCClient:           client,
			Scopes:              scopes,
			FeeCap:              feeCap,
			KeyUnlocker:         common.UnlockKey,
			GetNextNonce:        api.GetNextNonceOfAccount,
			RegisterPushKey:     api.RegisterPushKey,
			ShowTxStatusTracker: common.ShowTxStatusTracker,
			Stdout:              os.Stdout,
		}); err != nil {
			log.Fatal(err.Error())
		}
	},
}

func init() {
	pushKeyCmd.AddCommand(PushKeyRegCmd)

	f := PushKeyRegCmd.Flags()

	// Set flags
	f.StringSliceP("scopes", "s", []string{}, "Set one or more push key scopes")
	f.Float64P("feeCap", "c", 0, "Set the maximum fee the key is allowed to spend on fees")
	f.String("pass", "", "Specify the passphrase of a chosen push key account")
	f.Float64P("fee", "f", 0, "Set the network transaction fee")
	f.Uint64P("nonce", "n", 0, "Set the next nonce of the signing account signing")
	f.StringP("signing-key", "u", "",
		"Address or index of local account to use for signing transaction")
	f.StringP("signing-key-pass", "p", "", "Passphrase for unlocking the signing account")

	// Set required field
	_ = PushKeyRegCmd.MarkFlagRequired("fee")
	_ = PushKeyRegCmd.MarkFlagRequired("signing-key")
}
