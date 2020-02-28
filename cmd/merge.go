package cmd

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	repo2 "gitlab.com/makeos/mosdef/repo"
	// "gitlab.com/makeos/mosdef/repo"
)

// mergeReqCmd represents the merge request command
var mergeReqCmd = &cobra.Command{
	Use:   "merge",
	Short: "Create and send a merge request proposal",
	Long:  `Create and send a merge request proposal`,
	Run: func(cmd *cobra.Command, args []string) {
		account, _ := cmd.Flags().GetString("account")
		passphrase, _ := cmd.Flags().GetString("pass")
		repoName, _ := cmd.Flags().GetString("name")
		propID, _ := cmd.Flags().GetString("id")
		baseBranch, _ := cmd.Flags().GetString("base")
		baseBranchHash, _ := cmd.Flags().GetString("baseHash")
		targetBranch, _ := cmd.Flags().GetString("target")
		targetBranchHash, _ := cmd.Flags().GetString("targetHash")
		fee, _ := cmd.Flags().GetString("fee")
		nonce, _ := cmd.Flags().GetString("nonce")

		repo, client, remoteClients := getRepoAndClients(cmd, nonce)

		// When target branch hash is not provided or is equal to '.', automatically
		// read the latest reference hash of the target branch.
		if targetBranchHash == "" || targetBranchHash == "." {
			ref, err := repo.RefGet(targetBranch)
			if err != nil {
				log.Fatal(errors.Wrap(err, "failed to get target branch").Error())
			}
			targetBranchHash = ref
		}

		// When base branch hash is '.', we take this to mean the user wants us to
		// automatically read the latest reference hash of the base branch. We chose solely
		// this convention over an empty value because an empty base hash is interpreted
		// as zero sha1 value by the network.
		if baseBranchHash == "." {
			ref, err := repo.RefGet(baseBranch)
			if err != nil {
				log.Fatal(errors.Wrap(err, "failed to get base branch").Error())
			}
			baseBranchHash = ref
		}

		if err := repo2.CreateAndSendMergeRequestCmd(cfg, account, passphrase, repoName, propID, baseBranch,
			baseBranchHash, targetBranch, targetBranchHash, fee, nonce, client, remoteClients); err != nil {
			log.Fatal(err.Error())
		}
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
	pf.String("id", "", "Specify a unique merge proposal ID")
	pf.StringP("base", "b", "", "The base branch name")
	_ = mergeReqCmd.MarkPersistentFlagRequired("base")
	pf.StringP("baseHash", "c", "", "Specify the current commit hash of the base branch")
	pf.StringP("target", "t", "", "The target branch name")
	_ = mergeReqCmd.MarkPersistentFlagRequired("target")
	pf.StringP("targetHash", "u", "", "Specify the current commit hash of the target branch")

	// Transaction information
	pf.StringP("fee", "f", "0", "Set the transaction fee")
	pf.StringP("nonce", "n", "0", "Set the transaction nonce")

	// API connection config flags
	addAPIConnectionFlags(pf)
}
