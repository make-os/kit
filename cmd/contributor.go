package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gitlab.com/makeos/mosdef/api/utils"
	cmd2 "gitlab.com/makeos/mosdef/remote/cmd"
	"gitlab.com/makeos/mosdef/remote/cmd/repocmd"
)

// contribCmd represents the contributor command
var contribCmd = &cobra.Command{
	Use:   "contributor",
	Short: "Manage repository contributors",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

// contribAddCmd represents a sub-command to add contributors to a repository
var contribAddCmd = &cobra.Command{
	Use:   "add [flags] <name>",
	Short: "Add one or more contributors to a repository",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("account ID or public key is required")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		fee, _ := cmd.Flags().GetFloat64("fee")
		value, _ := cmd.Flags().GetFloat64("value")
		signingKey, _ := cmd.Flags().GetString("signing-key")
		signingKeyPass, _ := cmd.Flags().GetString("signing-key-pass")
		nonce, _ := cmd.Flags().GetUint64("nonce")
		configPath, _ := cmd.Flags().GetString("config")

		_, client, remoteClients := getRepoAndClients(cmd, true)
		if err := repocmd.CreateCmd(cfg, &repocmd.CreateArgs{
			Name:           args[0],
			Fee:            fee,
			Value:          value,
			SigningKey:     signingKey,
			SigningKeyPass: signingKeyPass,
			Nonce:          nonce,
			Config:         configPath,
			RPCClient:      client,
			RemoteClients:  remoteClients,
			KeyUnlocker:    cmd2.UnlockKey,
			GetNextNonce:   utils.GetNextNonceOfAccount,
			CreateRepo:     utils.CreateRepo,
			Stdout:         os.Stdout,
		}); err != nil {
			log.Fatal(err.Error())
		}
	},
}

func init() {
	rootCmd.AddCommand(contribCmd)
	contribCmd.AddCommand(contribAddCmd)

	sp := contribAddCmd.Flags().StringP
	fp := contribAddCmd.Flags().Float64P

	// Set flags
	fp("value", "v", 0, "The amount of coins to transfer to the repository")
	sp("config", "c", "", "Path to a file containing a repository configuration")

	// Set required field
	contribAddCmd.MarkFlagRequired("fee")
	contribAddCmd.MarkFlagRequired("account")

	// API connection config flags
	addAPIConnectionFlags(contribCmd.PersistentFlags())

	// Common Tx flags
	addCommonTxFlags(contribAddCmd.Flags())
}
