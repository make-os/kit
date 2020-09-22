package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/make-os/lobe/cmd/common"
	"github.com/make-os/lobe/cmd/contribcmd"
	"github.com/make-os/lobe/types/state"
	"github.com/make-os/lobe/util/api"
	"github.com/spf13/cobra"
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
	Use:   "add [flags] <pushKey>",
	Short: "Add one or more contributors to a repository or a namespace or both",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("at least one push key address is required")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		propID, _ := cmd.Flags().GetString("id")
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

		_, client := getRepoAndClient("")
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
			KeyUnlocker:         common.UnlockKey,
			GetNextNonce:        api.GetNextNonceOfAccount,
			AddRepoContributors: api.AddRepoContributors,
			ShowTxStatusTracker: common.ShowTxStatusTracker,
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
	contribF := contribAddCmd.Flags()
	contribF.StringP("name", "r", "", "The name of the target repository")
	contribF.String("id", "", "The unique proposal ID (default: current unix timestamp)")
	contribF.Int("feeMode", 0, "Specify who pays the fees: 0=contributor, 1=repo, 2=repo (capped) ")
	contribF.Float64("feeCap", 0, "Max. amount of repo balance the contributor(s) can spend on fees")
	contribF.String("namespace", "", "Add contributor(s) to the given repo-owned namespace")
	contribF.String("namespaceOnly", "", "Only add contributor(s) to the given repo-owned namespace")
	contribF.String("policies", "", "Set repository policies")
	contribF.Float64P("value", "v", 0, "The proposal fee to be paid if required by the repository")

	contribF.Float64P("fee", "f", 0, "Set the network transaction fee")
	contribF.Uint64P("nonce", "n", 0, "Set the next nonce of the signing account signing")
	contribF.StringP("signing-key", "u", "", "Address or index of local account to use for signing transaction")
	contribF.StringP("signing-key-pass", "p", "", "Passphrase for unlocking the signing account")

	// Set required field
	contribAddCmd.MarkFlagRequired("name")
	contribAddCmd.MarkFlagRequired("fee")
	contribAddCmd.MarkFlagRequired("signing-key")
}
