package cmd

import (
	"fmt"
	"os"

	"github.com/make-os/lobe/cmd/txcmd"
	"github.com/make-os/lobe/util/api"
	"github.com/spf13/cobra"
)

// txCmd represents the repo command
var txCmd = &cobra.Command{
	Use:   "tx",
	Short: "Create and read transaction data or status",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

// txGetCmd represents a sub-command to get a finalized transaction
var txGetCmd = &cobra.Command{
	Use:   "get [flags] <hash>",
	Short: "Get a transaction object or status by its hash",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("transaction hash is required")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		status, _ := cmd.Flags().GetBool("status")

		_, client := getRepoAndClient("", cmd)
		if err := txcmd.GetCmd(&txcmd.GetArgs{
			Hash:           args[0],
			RPCClient:      client,
			Status:         status,
			GetTransaction: api.GetTransaction,
			Stdout:         os.Stdout,
		}); err != nil {
			log.Fatal(err.Error())
		}
	},
}

func init() {
	rootCmd.AddCommand(txCmd)
	txCmd.AddCommand(txGetCmd)

	txGetCmd.Flags().BoolP("status", "s", false, "Show only status information")
}
