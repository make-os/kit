package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/asaskevich/govalidator"
	"github.com/make-os/lobe/cmd/common"
	"github.com/make-os/lobe/cmd/repocmd"
	"github.com/make-os/lobe/cmd/signcmd"
	"github.com/make-os/lobe/config"
	"github.com/make-os/lobe/remote/server"
	"github.com/make-os/lobe/util"
	"github.com/make-os/lobe/util/api"
	"github.com/make-os/lobe/util/colorfmt"
	"github.com/make-os/lobe/util/identifier"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
	"github.com/spf13/cobra"
	"gopkg.in/src-d/go-git.v4"
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

		_, client := getRepoAndClient("")
		if err := repocmd.CreateCmd(cfg, &repocmd.CreateArgs{
			Name:                args[0],
			Fee:                 fee,
			Value:               value,
			SigningKey:          signingKey,
			SigningKeyPass:      signingKeyPass,
			Nonce:               nonce,
			Config:              configPath,
			RPCClient:           client,
			KeyUnlocker:         common.UnlockKey,
			GetNextNonce:        api.GetNextNonceOfAccount,
			CreateRepo:          api.CreateRepo,
			ShowTxStatusTracker: common.ShowTxStatusTracker,
			Stdout:              os.Stdout,
		}); err != nil {
			log.Fatal(err.Error())
		}
	},
}

func setupRepoCreateCmd(cmd *cobra.Command) {
	f := cmd.Flags()

	f.StringP("config", "c", "", "Specify repository settings or a file containing it")

	if f.Lookup("value") == nil {
		f.Float64P("value", "v", 0, "The amount of coins to transfer to the repository")
	}

	if f.Lookup("fee") == nil {
		f.Float64P("fee", "f", 0, "Set the network transaction fee")
	}

	if f.Lookup("nonce") == nil {
		f.Uint64P("nonce", "n", 0, "Set the next nonce of the signing account signing")
	}

	if f.Lookup("signing-key") == nil {
		f.StringP("signing-key", "u", "", "Address or index of local account to use for signing transaction")
	}

	if f.Lookup("signing-key-pass") == nil {
		f.StringP("signing-key-pass", "p", "", "Passphrase for unlocking the signing account")
	}

	cmd.MarkFlagRequired("fee")
	cmd.MarkFlagRequired("signing-key")
}

// repoVoteCmd represents a sub-command for voting on a repository's proposal
var repoVoteCmd = &cobra.Command{
	Use:   "vote [flags] <choice>",
	Short: "Vote for or against a proposal",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("vote choice is required (0 - No, 1 - Yes, 2 - NoWithVeto, 3 - Abstain)")
		}
		if !govalidator.IsNumeric(args[0]) {
			return fmt.Errorf("vote choice is invalid. Epected: 0 - No, 1 - Yes, 2 - NoWithVeto, 3 - Abstain")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		rejectFlagCombo(cmd, "id", "mr")

		repoName, _ := cmd.Flags().GetString("repo")
		fee, _ := cmd.Flags().GetFloat64("fee")
		id, _ := cmd.Flags().GetString("id")
		mrID, _ := cmd.Flags().GetString("mr")
		signingKey, _ := cmd.Flags().GetString("signing-key")
		signingKeyPass, _ := cmd.Flags().GetString("signing-key-pass")
		nonce, _ := cmd.Flags().GetUint64("nonce")

		// If --id is not set, use --mr with a 'MR' prefix.
		proposalID := id
		if mrID != "" {
			proposalID = "MR" + mrID
		}

		_, client := getRepoAndClient("")
		if err := repocmd.VoteCmd(cfg, &repocmd.VoteArgs{
			RepoName:            repoName,
			ProposalID:          proposalID,
			Vote:                cast.ToInt(args[0]),
			Fee:                 fee,
			SigningKey:          signingKey,
			SigningKeyPass:      signingKeyPass,
			Nonce:               nonce,
			RPCClient:           client,
			KeyUnlocker:         common.UnlockKey,
			GetNextNonce:        api.GetNextNonceOfAccount,
			VoteCreator:         api.VoteRepoProposal,
			ShowTxStatusTracker: common.ShowTxStatusTracker,
			Stdout:              os.Stdout,
		}); err != nil {
			log.Fatal(err.Error())
		}
	},
}

func setupRepoVoteCmd(cmd *cobra.Command) {
	f := cmd.Flags()
	f.StringP("repo", "r", "", "The name of the repository")
	f.StringP("id", "i", "", "The unique ID of the proposal")
	f.StringP("mr", "m", "", "The unique ID of a merge request") // Prepends `MR` to the id
	f.Float64P("fee", "f", 0, "Set the network transaction fee")
	f.Uint64P("nonce", "n", 0, "Set the next nonce of the signing account signing")
	f.StringP("signing-key", "u", "", "Address or index of local account to use for signing transaction")
	f.StringP("signing-key-pass", "p", "", "Passphrase for unlocking the signing account")
	cmd.MarkFlagRequired("fee")
	cmd.MarkFlagRequired("signing-key")
}

// repoConfigCmd represents a command for configuring a repository
var repoConfigCmd = &cobra.Command{
	Use:     "config [flags] [<directory>]",
	Aliases: []string{"set"},
	Short:   "Configure repository settings",
	Run: func(cmd *cobra.Command, args []string) {
		fee, _ := cmd.Flags().GetFloat64("fee")
		value, _ := cmd.Flags().GetFloat64("value")
		pushKey, _ := cmd.Flags().GetString("push-key")
		signingKey, _ := cmd.Flags().GetString("signing-key")
		signingKeyPass, _ := cmd.Flags().GetString("signing-key-pass")
		nonce, _ := cmd.Flags().GetUint64("nonce")
		noSign, _ := cmd.Flags().GetBool("no-sign")
		amendCommit, _ := cmd.Flags().GetBool("commit.amend")
		remotes, _ := cmd.Flags().GetStringSlice("remote")

		var targetRepoDir string
		var err error
		if len(args) > 0 {
			targetRepoDir, err = filepath.Abs(args[0])
			if err != nil {
				log.Fatal(err.Error())
			}
		}

		targetRepo, client := getRepoAndClient(targetRepoDir)
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
			Value:          &value,
			Nonce:          &nonce,
			Fee:            &fee,
			AmendCommit:    &amendCommit,
			RPCClient:      client,
			PushKey:        &pushKey,
			SigningKey:     &signingKey,
			SigningKeyPass: &signingKeyPass,
			NoHook:         noSign,
			Remotes:        remoteObjs,
			KeyUnlocker:    common.UnlockKey,
			GetNextNonce:   api.GetNextNonceOfAccount,
			Stdout:         os.Stdout,
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

		if err := repocmd.ConfigCmd(cfg, targetRepo, configArgs); err != nil {
			log.Fatal(err.Error())
		}
	},
}

func setupRepoConfigCmd(cmd *cobra.Command) {
	f := cmd.Flags()

	f.StringSliceP("remote", "r", []string{}, "Set one or more remotes")
	f.Bool("no-sign", false, "Do not enable automatic signing hook")
	f.Bool("commit.amend", true, "Sign an amended commit (instead of creating a new one)")

	if f.Lookup("value") == nil {
		f.Float64P("value", "v", 0, "Set transaction value")
	}

	if f.Lookup("fee") == nil {
		f.Float64P("fee", "f", 0, "Set the network transaction fee")
	}

	if f.Lookup("nonce") == nil {
		f.Uint64P("nonce", "n", 0, "Set the next nonce of the signing account signing")
	}

	if f.Lookup("signing-key") == nil {
		f.StringP("signing-key", "u", "", "Address or index of local account to use for signing transaction")
	}

	if f.Lookup("signing-key-pass") == nil {
		f.StringP("signing-key-pass", "p", "", "Passphrase for unlocking the signing account")
	}

	f.StringP("push-key", "k", "", "Specify the push key (defaults to signing key)")
}

// repoHookCmd is a command handles git hooks
var repoHookCmd = &cobra.Command{
	Use:   "hook [flags] <remote>",
	Short: "Handles git hook events",
	Run: func(cmd *cobra.Command, args []string) {
		targetRepo, client := getRepoAndClient("")
		if targetRepo == nil {
			log.Fatal("no repository found in current directory")
		}

		if err := repocmd.HookCmd(cfg, targetRepo, &repocmd.HookArgs{
			Args:               args,
			RPCClient:          client,
			KeyUnlocker:        common.UnlockKey,
			GetNextNonce:       api.GetNextNonceOfPushKeyOwner,
			SetRemotePushToken: server.GenSetPushToken,
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

// repoInitCmd represents a sub-command to initialize a new repository
var repoInitCmd = &cobra.Command{
	Use:   "init [flags] <name>",
	Short: "Register a repository, initialize and configure it locally.",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("repository name is required")
		}

		if identifier.IsValidResourceName(args[0]) != nil {
			return fmt.Errorf("name (%s) is not valid", args[0])
		}

		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {

		// Ensure no matching file or directory exist in the current directory
		repoPath, err := filepath.Abs(args[0])
		if err != nil {
			log.Fatal(err.Error())
		} else if util.IsPathOk(repoPath) {
			log.Fatal(fmt.Sprintf("a file or directory matching the name (%s) already exists", args[0]))
		}

		// Try to create the repo on the network
		fmt.Println(colorfmt.YellowStringf("Step 1:"), "Registering repository on the network")
		repoCreateCmd.Run(cmd, args)

		// Git initialize the repository
		fmt.Println(colorfmt.YellowStringf("Step 2:"), "Initialized repository")
		_, err = git.PlainInit(repoPath, false)
		if err != nil {
			log.Fatal(err.Error())
		}

		// Configure the repository
		fmt.Println(colorfmt.YellowStringf("Step 3:"), "Configured repository")
		repoConfigCmd.Run(cmd, args)

		fmt.Printf(`

Success! Created a new repository %s:
Enter the repository by typing:

  `+colorfmt.CyanString("cd "+args[0])+`

Inside that repository, you can run the following commands:

  `+colorfmt.CyanString("git push")+`:
    To push your commits, tags and notes with automatic signing.

  `+colorfmt.CyanString(config.ExecName+" config")+`:
    To change network and repository configurations (e.g fees, nonce, remotes etc)

  `+colorfmt.CyanString(config.ExecName+" sign")+`:
    To manually sign your commit, tags and nodes.

Happy hacking!
`, colorfmt.CyanString(args[0]))
	},
}

func setupRepoInitCmd(cmd *cobra.Command) {
	setupRepoCreateCmd(cmd)
	setupRepoConfigCmd(cmd)
}

func init() {
	rootCmd.AddCommand(repoCmd)
	repoCmd.AddCommand(repoCreateCmd)
	repoCmd.AddCommand(repoVoteCmd)
	repoCmd.AddCommand(repoConfigCmd)
	repoCmd.AddCommand(repoHookCmd)
	repoCmd.AddCommand(repoInitCmd)

	setupRepoCreateCmd(repoCreateCmd)
	setupRepoVoteCmd(repoVoteCmd)
	setupRepoConfigCmd(repoConfigCmd)
	setupRepoInitCmd(repoInitCmd)
}
