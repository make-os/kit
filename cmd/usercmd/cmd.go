package usercmd

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

// UserCmd represents the user command
var UserCmd = &cobra.Command{
	Use:   "user",
	Short: "Access and manage a user's personal resources",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

// userSendCmd represents a sub-command to send coins
var userSendCmd = &cobra.Command{
	Use:   "send [flags] <address>",
	Short: "Send coins from user account to another user account or a repository",
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

		_, client := common.GetRepoAndClient(cmd, cfg, "")
		if err := SendCmd(cfg, &SendArgs{
			Recipient:           args[0],
			Value:               value,
			Nonce:               nonce,
			Fee:                 fee,
			SigningKey:          signingKey,
			SigningKeyPass:      signingKeyPass,
			RPCClient:           client,
			KeyUnlocker:         common.UnlockKey,
			GetNextNonce:        api.GetNextNonceOfAccount,
			SendCoin:            api.SendCoin,
			ShowTxStatusTracker: common.ShowTxStatusTracker,
			Stdout:              os.Stdout,
		}); err != nil {
			log.Fatal(err.Error())
		}
	},
}

func init() {
	UserCmd.AddCommand(userSendCmd)

	// Set flags
	f := userSendCmd.Flags()
	f.Float64P("value", "v", 0, "Set the amount of coin to send")
	f.Float64P("fee", "f", 0, "Set the network transaction fee")
	f.Uint64P("nonce", "n", 0, "Set the next nonce of the signing account signing")
	f.StringP("signing-key", "u", "", "Address or index of local account to use for signing transaction")
	f.StringP("signing-key-pass", "p", "", "Passphrase for unlocking the signing account")
	_ = userSendCmd.MarkFlagRequired("fee")
	_ = userSendCmd.MarkFlagRequired("signing-key")
}
