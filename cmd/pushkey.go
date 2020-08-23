package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/themakeos/lobe/api/utils"
	"github.com/themakeos/lobe/cmd/common"
	"github.com/themakeos/lobe/cmd/pkcmd"
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

		_, client, remoteClients := getRepoAndClients("", cmd)
		if err := pkcmd.RegisterCmd(cfg, &pkcmd.RegisterArgs{
			Target:              target,
			TargetPass:          targetPass,
			Fee:                 fee,
			SigningKey:          signingKey,
			SigningKeyPass:      signingKeyPass,
			Nonce:               nonce,
			RPCClient:           client,
			RemoteClients:       remoteClients,
			Scopes:              scopes,
			FeeCap:              feeCap,
			KeyUnlocker:         common.UnlockKey,
			GetNextNonce:        utils.GetNextNonceOfAccount,
			RegisterPushKey:     utils.RegisterPushKey,
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

	fp := pushKeyRegCmd.Flags().Float64P
	ssp := pushKeyRegCmd.Flags().StringSliceP
	s := pushKeyRegCmd.Flags().String

	// Set flags
	ssp("scopes", "s", []string{}, "Set one or more push key scopes")
	fp("feeCap", "c", 0, "Set the maximum fee the key is allowed to spend on fees")
	s("pass", "", "Specify the passphrase of a chosen push key account")

	// Common Tx flags
	addCommonTxFlags(pushKeyRegCmd.Flags())

	// API connection config flags
	addAPIConnectionFlags(pushKeyCmd.PersistentFlags())

	// Set required field
	pushKeyRegCmd.MarkFlagRequired("fee")
	pushKeyRegCmd.MarkFlagRequired("signing-key")
}
