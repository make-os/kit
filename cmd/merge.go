package cmd

import (
	"github.com/spf13/cobra"
	// "gitlab.com/makeos/mosdef/repo"
)

// mergeReqCmd represents the merge request command
var mergeReqCmd = &cobra.Command{
	Use:   "merge",
	Short: "Create and send a merge request proposal",
	Long:  `Create and send a merge request proposal`,
	Run: func(cmd *cobra.Command, args []string) {
		// account, _ := cmd.Flags().GetString("account")
		// passphrase, _ := cmd.Flags().GetString("pass")
		//
		// if err := repo.CreateAndSendMergeRequestCmd(
		// 	cfg,
		// 	account,
		// 	passphrase); err != nil {
		// 	log.Fatal(err.Error())
		// }
	},
}

func initMerge() {
	pf := mergeReqCmd.PersistentFlags()
	pf.SortFlags = false
	mergeReqCmd.Flags().SortFlags = false

	// Signer account information
	pf.StringP("account", "a", "", "Specify the sending account")
	_ = mergeReqCmd.MarkPersistentFlagRequired("account")
	pf.StringP("pass", "p", "", "Password to unlock signer account and skip interactive mode")

	// Merge request information
	pf.String("name", "", "Specify the name of the target repository")
	pf.StringP("base", "b", "", "The base branch name")
	pf.StringP("baseHash", "c", "", "Specify the current commit hash of the base branch")
	pf.StringP("target", "t", "", "The target branch name")
	pf.StringP("targetHash", "u", "", "Specify the current commit hash of the target branch")

	// Transaction information
	pf.StringP("fee", "f", "0", "Set the transaction fee")
	pf.StringP("nonce", "n", "0", "Set the transaction nonce")

	// API connection config flags
	addAPIConnectionFlags(pf)
}
