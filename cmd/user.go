package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/themakeos/lobe/api/utils"
	"github.com/themakeos/lobe/commands/common"
	usercmd "github.com/themakeos/lobe/commands/usercmd"
)

// userCmd represents the user command
var userCmd = &cobra.Command{
	Use:   "user",
	Short: "Access and manage a user's personal resources",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

// userSend represents a sub-command to send coins
var userSend = &cobra.Command{
	Use:   "send [flags] <address>",
	Short: "Send coins from a user account to another user or a repository account",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("user or repository address is required")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		value, _ := cmd.Flags().GetFloat64("value")
		fee, _ := cmd.Flags().GetFloat64("fee")
		signingKey, _ := cmd.Flags().GetString("signing-key")
		signingKeyPass, _ := cmd.Flags().GetString("signing-key-pass")
		nonce, _ := cmd.Flags().GetUint64("nonce")

		_, client, remoteClients := getRepoAndClients(cmd)
		if err := usercmd.SendCmd(cfg, &usercmd.SendArgs{
			Recipient:           args[0],
			Value:               value,
			Nonce:               nonce,
			Fee:                 fee,
			SigningKey:          signingKey,
			SigningKeyPass:      signingKeyPass,
			RPCClient:           client,
			RemoteClients:       remoteClients,
			KeyUnlocker:         common.UnlockKey,
			GetNextNonce:        utils.GetNextNonceOfAccount,
			SendCoin:            utils.SendCoin,
			ShowTxStatusTracker: common.ShowTxStatusTracker,
			Stdout:              os.Stdout,
		}); err != nil {
			log.Fatal(err.Error())
		}
	},
}

func init() {
	rootCmd.AddCommand(userCmd)
	userCmd.AddCommand(userSend)

	// Set flags
	userSend.Flags().Float64P("value", "v", 0, "Set the amount of coin to send")

	// API connection config flags
	addAPIConnectionFlags(userCmd.PersistentFlags())

	// Common Tx flags
	addCommonTxFlags(userSend.Flags())

	// Set required field
	userSend.MarkFlagRequired("fee")
	userSend.MarkFlagRequired("signing-key")
}
