package cmd

import (
	"fmt"
	"os"

	"github.com/make-os/lobe/cmd/common"
	"github.com/make-os/lobe/cmd/pkcmd"
	"github.com/make-os/lobe/util/api"
	"github.com/spf13/cobra"
)

// pushKeyCmd represents
var pushKeyCmd = &cobra.Command{
	Use:   "pk",
	Short: "Register and manage network push keys",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

// pushKeyRegCmd represents a sub-command to register a public key as a push key
var pushKeyRegCmd = &cobra.Command{
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

		_, client := getRepoAndClient("")
		if err := pkcmd.RegisterCmd(cfg, &pkcmd.RegisterArgs{
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
	rootCmd.AddCommand(pushKeyCmd)
	pushKeyCmd.AddCommand(pushKeyRegCmd)

	f := pushKeyRegCmd.Flags()

	// Set flags
	f.StringSliceP("scopes", "s", []string{}, "Set one or more push key scopes")
	f.Float64P("feeCap", "c", 0, "Set the maximum fee the key is allowed to spend on fees")
	f.String("pass", "", "Specify the passphrase of a chosen push key account")
	f.Float64P("fee", "f", 0, "Set the network transaction fee")
	f.Uint64P("nonce", "n", 0, "Set the next nonce of the signing account signing")
	f.StringP("signing-key", "u", "", "Address or index of local account to use for signing transaction")
	f.StringP("signing-key-pass", "p", "", "Passphrase for unlocking the signing account")

	// Set required field
	pushKeyRegCmd.MarkFlagRequired("fee")
	pushKeyRegCmd.MarkFlagRequired("signing-key")
}
