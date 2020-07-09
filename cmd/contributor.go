package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gitlab.com/makeos/mosdef/api/utils"
	cmd2 "gitlab.com/makeos/mosdef/remote/cmd"
	"gitlab.com/makeos/mosdef/remote/cmd/contribcmd"
	"gitlab.com/makeos/mosdef/types/state"
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
			return fmt.Errorf("at least one push key address is required")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		propID, _ := cmd.Flags().GetString("propId")
		feeMode, _ := cmd.Flags().GetInt("feeMode")
		feeCap, _ := cmd.Flags().GetFloat64("feeCap")
		namespace, _ := cmd.Flags().GetString("namespace")
		namespaceOnly, _ := cmd.Flags().GetString("namespaceOnly")
		policies, _ := cmd.Flags().GetString("policies")
		propFee, _ := cmd.Flags().GetFloat64("value")
		fee, _ := cmd.Flags().GetFloat64("fee")
		signingKey, _ := cmd.Flags().GetString("signing-key")
		signingKeyPass, _ := cmd.Flags().GetString("signing-key-pass")
		nonce, _ := cmd.Flags().GetUint64("nonce")

		// Decode `policies` from JSON
		policies = fmt.Sprintf("[%s]", policies)
		var contribPolicy = []*state.ContributorPolicy{}
		if err := json.Unmarshal([]byte(policies), &contribPolicy); err != nil {
			log.Fatal("failed to decode policies", "Err", err)
		}

		_, client, remoteClients := getRepoAndClients(cmd, true)
		if err := contribcmd.AddCmd(cfg, &contribcmd.AddArgs{
			Name:                name,
			PushKeys:            args,
			PropID:              propID,
			FeeCap:              feeCap,
			FeeMode:             feeMode,
			Value:               propFee,
			Policies:            contribPolicy,
			Namespace:           namespace,
			NamespaceOnly:       namespaceOnly,
			Nonce:               nonce,
			Fee:                 fee,
			SigningKey:          signingKey,
			SigningKeyPass:      signingKeyPass,
			RPCClient:           client,
			RemoteClients:       remoteClients,
			KeyUnlocker:         cmd2.UnlockKey,
			GetNextNonce:        utils.GetNextNonceOfAccount,
			AddRepoContributors: utils.AddRepoContributors,
			Stdout:              os.Stdout,
		}); err != nil {
			log.Fatal(err.Error())
		}
	},
}

func init() {
	rootCmd.AddCommand(contribCmd)
	contribCmd.AddCommand(contribAddCmd)

	// Set flags
	contribAddCmd.Flags().StringP("name", "r", "", "The name of the target repository")
	contribAddCmd.Flags().String("propId", "", "The unique proposal ID (default: current unix timestamp)")
	contribAddCmd.Flags().Int("feeMode", 0, "Specify who pays the fees: 0=contributor, 1=repo, 2=repo (capped) ")
	contribAddCmd.Flags().Float64("feeCap", 0, "Max. amount of repo balance the contributor(s) can spend on fees")
	contribAddCmd.Flags().String("namespace", "", "Add contributor(s) to the given repo-owned namespace")
	contribAddCmd.Flags().String("namespaceOnly", "", "Only add contributor(s) to the given repo-owned namespace")
	contribAddCmd.Flags().String("policies", "", "Set repository policies")
	contribAddCmd.Flags().Float64P("value", "v", 0, "The proposal fee to be paid if required by the repository")

	// API connection config flags
	addAPIConnectionFlags(contribCmd.PersistentFlags())

	// Common Tx flags
	addCommonTxFlags(contribAddCmd.Flags())

	// Set required field
	contribAddCmd.MarkFlagRequired("name")
	contribAddCmd.MarkFlagRequired("fee")
	contribAddCmd.MarkFlagRequired("account")
}
