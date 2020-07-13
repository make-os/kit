package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gitlab.com/makeos/mosdef/api/utils"
	"gitlab.com/makeos/mosdef/commands/txcmd"
)

// txCmd represents the repo command
var txCmd = &cobra.Command{
	Use:   "tx",
	Short: "Access network transaction information",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

// txGetCmd represents a sub-command to get a finalized transaction
var txGetCmd = &cobra.Command{
	Use:   "get [flags] <hash>",
	Short: "Get a finalized transaction",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("transaction hash is required")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		_, client, remoteClients := getRepoAndClients(cmd)
		if err := txcmd.GetCmd(&txcmd.GetArgs{
			Hash:           args[0],
			RPCClient:      client,
			RemoteClients:  remoteClients,
			GetTransaction: utils.GetTransaction,
			Stdout:         os.Stdout,
		}); err != nil {
			log.Fatal(err.Error())
		}
	},
}

func init() {
	rootCmd.AddCommand(txCmd)
	txCmd.AddCommand(txGetCmd)

	// API connection config flags
	addAPIConnectionFlags(txCmd.PersistentFlags())
}
