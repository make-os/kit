package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/themakeos/lobe/api/utils"
	"github.com/themakeos/lobe/commands/common"
	"github.com/themakeos/lobe/commands/repocmd"
	"github.com/themakeos/lobe/commands/signcmd"
	"github.com/themakeos/lobe/remote/server"
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

func initRepoCreateCmd() {
	sp := repoCreateCmd.Flags().StringP
	fp := repoCreateCmd.Flags().Float64P
	fp("value", "v", 0, "The amount of coins to transfer to the repository")
	sp("config", "c", "", "Path to a file containing a repository configuration")
	addCommonTxFlags(repoCreateCmd.Flags())
	repoCreateCmd.MarkFlagRequired("fee")
	repoCreateCmd.MarkFlagRequired("signing-key")
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

func initRepoVoteCmd() {
	sp := repoVoteCmd.Flags().StringP
	sp("repo", "r", "", "The name of the repository")
	sp("id", "i", "", "The unique ID of the proposal")
	addCommonTxFlags(repoVoteCmd.Flags())
	repoVoteCmd.MarkFlagRequired("fee")
	repoVoteCmd.MarkFlagRequired("signing-key")
}

// repoConfigCmd represents a command for configuring a repository
var repoConfigCmd = &cobra.Command{
	Use:     "config [flags]",
	Aliases: []string{"set"},
	Short:   "Configure repository settings",
	Run: func(cmd *cobra.Command, args []string) {
		fee, _ := cmd.Flags().GetFloat64("fee")
		value, _ := cmd.Flags().GetFloat64("value")
		signingKey, _ := cmd.Flags().GetString("signing-key")
		signingKeyPass, _ := cmd.Flags().GetString("signing-key-pass")
		nonce, _ := cmd.Flags().GetUint64("nonce")
		noSign, _ := cmd.Flags().GetBool("no-sign")
		amendCommit, _ := cmd.Flags().GetBool("commit.amend")
		remotes, _ := cmd.Flags().GetStringSlice("remote")
		evalPrintOut, _ := cmd.Flags().GetBool("print-out")

		targetRepo, client, remoteClients := getRepoAndClients(cmd)
		if targetRepo == nil {
			log.Fatal("no repository found in current directory")
		}

		var remoteObjs []repocmd.Remote
		for _, r := range remotes {
			path := strings.Fields(r)
			if len(path) < 2 {
				log.Fatal("invalid remote format. Expected '<name> <url>'")
			}
			remoteObjs = append(remoteObjs, repocmd.Remote{Name: path[0], URL: path[1]})
		}

		configArgs := &repocmd.ConfigArgs{
			Value:           &value,
			Nonce:           &nonce,
			Fee:             &fee,
			AmendCommit:     &amendCommit,
			RPCClient:       client,
			SigningKey:      &signingKey,
			SigningKeyPass:  &signingKeyPass,
			NoHook:          noSign,
			PrintOutForEval: evalPrintOut,
			RemoteClients:   remoteClients,
			Remotes:         remoteObjs,
			KeyUnlocker:     common.UnlockKey,
			GetNextNonce:    utils.GetNextNonceOfAccount,
			Stdout:          os.Stdout,
		}

		if !cmd.Flags().Changed("fee") {
			configArgs.Fee = nil
		}

		if !cmd.Flags().Changed("value") {
			configArgs.Value = nil
		}

		if !cmd.Flags().Changed("nonce") {
			configArgs.Nonce = nil
		}

		if !cmd.Flags().Changed("signing-key") {
			configArgs.SigningKey = nil
		}

		if !cmd.Flags().Changed("signing-key-pass") {
			configArgs.SigningKeyPass = nil
		}

		if !cmd.Flags().Changed("commit.amend") {
			configArgs.AmendCommit = nil
		}

		if err := repocmd.ConfigCmd(targetRepo, configArgs); err != nil {
			log.Fatal(err.Error())
		}
	},
}

func initRepoConfigCmd() {
	ssp := repoConfigCmd.Flags().StringSliceP
	bf := repoConfigCmd.Flags().Bool
	bfp := repoConfigCmd.Flags().BoolP
	fp := repoConfigCmd.Flags().Float64P
	ssp("remote", "r", []string{}, "Set one or more remotes")
	bf("no-sign", false, "Disable automatic signing")
	bf("commit.amend", true, "Sign an amended commit (instead of creating a new one)")
	bfp("print-out", "o", false, "Print out more config to pass to eval()")
	fp("value", "v", 0, "Set transaction value")
	addCommonTxFlags(repoConfigCmd.Flags())
}

// repoHookCmd is a command handles git hooks
var repoHookCmd = &cobra.Command{
	Use:   "hook [flags] <remote>",
	Short: "Performs hook operations",
	Run: func(cmd *cobra.Command, args []string) {
		authMode, _ := cmd.Flags().GetBool("askpass")

		targetRepo, client, remoteClients := getRepoAndClients(cmd)
		if targetRepo == nil {
			log.Fatal("no repository found in current directory")
		}

		if err := repocmd.HookCmd(cfg, targetRepo, &repocmd.HookArgs{
			Args:               args,
			AskPass:            authMode,
			RemoteClients:      remoteClients,
			RPCClient:          client,
			KeyUnlocker:        common.UnlockKey,
			GetNextNonce:       utils.GetNextNonceOfPushKeyOwner,
			SetRemotePushToken: server.SetRemotePushToken,
			CommitSigner:       signcmd.SignCommitCmd,
			TagSigner:          signcmd.SignTagCmd,
			NoteSigner:         signcmd.SignNoteCmd,
			Stdout:             os.Stdout,
			Stdin:              os.Stdin,
			Stderr:             os.Stderr,
		}); err != nil {
			log.Fatal(errors.Wrap(err, "hook error").Error())
		}
	},
}

func initRepoHookCmd() {
	bf := repoHookCmd.Flags().Bool
	bf("askpass", false, "Mode for outputting credentials to git")
}

func init() {
	rootCmd.AddCommand(repoCmd)
	repoCmd.AddCommand(repoCreateCmd)
	repoCmd.AddCommand(repoVoteCmd)
	rootCmd.AddCommand(repoConfigCmd)
	repoCmd.AddCommand(repoHookCmd)

	initRepoCreateCmd()
	initRepoVoteCmd()
	initRepoConfigCmd()
	initRepoHookCmd()

	// API connection config flags
	addAPIConnectionFlags(repoCmd.PersistentFlags())
	addAPIConnectionFlags(repoConfigCmd.PersistentFlags())
}
