package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/themakeos/lobe/api/utils"
	"github.com/themakeos/lobe/commands/common"
	"github.com/themakeos/lobe/commands/repocmd"
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

		_, client, remoteClients := getRepoAndClients(cmd)
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
			KeyUnlocker:    common.UnlockKey,
			GetNextNonce:   utils.GetNextNonceOfAccount,
			CreateRepo:     utils.CreateRepo,
			Stdout:         os.Stdout,
		}); err != nil {
			log.Fatal(err.Error())
		}
	},
}

// repoVoteCmd represents a sub-command for voting on a repository's proposal
var repoVoteCmd = &cobra.Command{
	Use:   "vote [flags] <choice>",
	Short: "Vote for or against a proposal",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("vote choice is required (0 - No, 1 - Yes, 2 - NoWithVeto, 3 - Abstain")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		repoName, _ := cmd.Flags().GetString("repo")
		fee, _ := cmd.Flags().GetFloat64("fee")
		signingKey, _ := cmd.Flags().GetString("signing-key")
		signingKeyPass, _ := cmd.Flags().GetString("signing-key-pass")
		nonce, _ := cmd.Flags().GetUint64("nonce")

		_, client, remoteClients := getRepoAndClients(cmd)
		if err := repocmd.VoteCmd(cfg, &repocmd.VoteArgs{
			RepoName:       repoName,
			ProposalID:     args[0],
			Fee:            fee,
			SigningKey:     signingKey,
			SigningKeyPass: signingKeyPass,
			Nonce:          nonce,
			RPCClient:      client,
			RemoteClients:  remoteClients,
			KeyUnlocker:    common.UnlockKey,
			GetNextNonce:   utils.GetNextNonceOfAccount,
			VoteCreator:    utils.VoteRepoProposal,
			Stdout:         os.Stdout,
		}); err != nil {
			log.Fatal(err.Error())
		}
	},
}

// repoConfigCmd represents a command for configuring a repository
var repoConfigCmd = &cobra.Command{
	Use:   "config [flags]",
	Short: "Configure a repository",
	Run: func(cmd *cobra.Command, args []string) {
		// repoName, _ := cmd.Flags().GetString("repo")
		fee, _ := cmd.Flags().GetFloat64("fee")
		// signingKey, _ := cmd.Flags().GetString("signing-key")
		// signingKeyPass, _ := cmd.Flags().GetString("signing-key-pass")
		// nonce, _ := cmd.Flags().GetUint64("nonce")
		//
		targetRepo, client, remoteClients := getRepoAndClients(cmd)
		if targetRepo == nil {
			log.Fatal("no repository found in current directory")
		}

		if err := repocmd.ConfigCmd(cfg, &repocmd.ConfigArgs{
			Fee:           fee,
			RPCClient:     client,
			RemoteClients: remoteClients,
			KeyUnlocker:   common.UnlockKey,
			GetNextNonce:  utils.GetNextNonceOfAccount,
			Stdout:        os.Stdout,
		}); err != nil {
			log.Fatal(err.Error())
		}
	},
}

func initRepoCreateCmd() {
	sp := repoCreateCmd.Flags().StringP
	fp := repoCreateCmd.Flags().Float64P
	fp("value", "v", 0, "The amount of coins to transfer to the repository")
	sp("config", "c", "", "Path to a file containing a repository configuration")
	addCommonTxFlags(repoCreateCmd.Flags())
	repoCreateCmd.MarkFlagRequired("fee")
	repoCreateCmd.MarkFlagRequired("signing-key")
}

func initRepoVoteCmd() {
	sp := repoVoteCmd.Flags().StringP
	sp("repo", "r", "", "The name of the repository")
	sp("id", "i", "", "The unique ID of the proposal")
	addCommonTxFlags(repoVoteCmd.Flags())
	repoVoteCmd.MarkFlagRequired("fee")
	repoVoteCmd.MarkFlagRequired("signing-key")
}

func initRepoConfigCmd() {
	ssp := repoConfigCmd.Flags().StringSliceP
	f64p := repoConfigCmd.Flags().Float64P
	uint64p := repoConfigCmd.Flags().Uint64P
	boolf := repoConfigCmd.Flags().Bool
	ssp("set-remote", "r", []string{}, "Set one or more remotes")
	f64p("fee", "f", 0, "The fee to pay to the network per push request")
	uint64p("nonce", "n", 0, "The next nonce of the account signing the transaction")
	boolf("no-sign", false, "Disable automatic signing")
	addCommonTxFlags(repoConfigCmd.Flags())
}

func init() {
	rootCmd.AddCommand(repoCmd)
	repoCmd.AddCommand(repoCreateCmd)
	repoCmd.AddCommand(repoVoteCmd)
	repoCmd.AddCommand(repoConfigCmd)

	initRepoCreateCmd()
	initRepoVoteCmd()
	initRepoConfigCmd()

	// API connection config flags
	addAPIConnectionFlags(repoCmd.PersistentFlags())
}
