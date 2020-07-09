package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gitlab.com/makeos/mosdef/api/utils"
	cmd2 "gitlab.com/makeos/mosdef/remote/cmd"
	"gitlab.com/makeos/mosdef/remote/cmd/repocmd"
)

// repoCmd represents the repo command
var repoCmd = &cobra.Command{
	Use:   "repo",
	Short: "Create, find and manage repositories",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

// repoCreateCmd represents a sub-command to create a repository
var repoCreateCmd = &cobra.Command{
	Use:   "create [flags] <name>",
	Short: "Create a repository",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("name is required")
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
	rootCmd.AddCommand(repoCmd)
	repoCmd.AddCommand(repoCreateCmd)

	sp := repoCreateCmd.Flags().StringP
	fp := repoCreateCmd.Flags().Float64P

	// Set flags
	fp("value", "v", 0, "The amount of coins to transfer to the repository")
	sp("config", "c", "", "Path to a file containing a repository configuration")

	// Set required field
	repoCreateCmd.MarkFlagRequired("fee")
	repoCreateCmd.MarkFlagRequired("account")

	// API connection config flags
	addAPIConnectionFlags(repoCmd.PersistentFlags())

	// Common Tx flags
	addCommonTxFlags(repoCreateCmd.Flags())
}
