package cmd

import (
	"github.com/spf13/cobra"
	"github.com/themakeos/lobe/api/utils"
	"github.com/themakeos/lobe/commands/common"
	"github.com/themakeos/lobe/commands/signcmd"
	"github.com/themakeos/lobe/remote/server"
)

// signCmd represents the commit command
var signCmd = &cobra.Command{
	Use:   "sign",
	Short: "Sign a commit, tag or note and generate push request token",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var signCommitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Sign or amend current commit",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		fee, _ := cmd.Flags().GetString("fee")
		value, _ := cmd.Flags().GetString("value")
		nonce, _ := cmd.Flags().GetUint64("nonce")
		signingKey, _ := cmd.Flags().GetString("signing-key")
		mergeID, _ := cmd.Flags().GetString("merge-id")
		head, _ := cmd.Flags().GetString("head")
		branch, _ := cmd.Flags().GetString("branch")
		forceCheckout, _ := cmd.Flags().GetBool("force")
		amend, _ := cmd.Flags().GetBool("amend")
		signingKeyPass, _ := cmd.Flags().GetString("signing-key-pass")
		msg, _ := cmd.Flags().GetString("message")
		targetRemotes, _ := cmd.Flags().GetString("remote")
		resetRemoteTokens, _ := cmd.Flags().GetBool("reset")

		targetRepo, client, remoteClients := getRepoAndClients(cmd)
		if targetRepo == nil {
			log.Fatal("no repository found in current directory")
		}

		if err := signcmd.SignCommitCmd(cfg, targetRepo, &signcmd.SignCommitArgs{
			Message:               msg,
			Fee:                   fee,
			Nonce:                 nonce,
			Value:                 value,
			AmendCommit:           amend,
			MergeID:               mergeID,
			Head:                  head,
			Branch:                branch,
			ForceCheckout:         forceCheckout,
			PushKeyID:             signingKey,
			PushKeyPass:           signingKeyPass,
			Remote:                targetRemotes,
			ResetTokens:           resetRemoteTokens,
			RPCClient:             client,
			RemoteClients:         remoteClients,
			KeyUnlocker:           common.UnlockKey,
			GetNextNonce:          utils.GetNextNonceOfPushKeyOwner,
			RemoteURLTokenUpdater: server.UpdateRemoteURLsWithPushToken,
		}); err != nil {
			cfg.G().Log.Fatal(err.Error())
		}
	},
}

var signTagCmd = &cobra.Command{
	Use:   "tag",
	Short: "Create and sign an annotated tag",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		fee, _ := cmd.Flags().GetString("fee")
		value, _ := cmd.Flags().GetString("value")
		nonce, _ := cmd.Flags().GetUint64("nonce")
		signingKey, _ := cmd.Flags().GetString("signing-key")
		signingKeyPass, _ := cmd.Flags().GetString("signing-key-pass")
		msg, _ := cmd.Flags().GetString("message")
		targetRemotes, _ := cmd.Flags().GetString("remote")
		resetRemoteTokens, _ := cmd.Flags().GetBool("reset")

		targetRepo, client, remoteClients := getRepoAndClients(cmd)
		if targetRepo == nil {
			log.Fatal("no repository found in current directory")
		}

		args = cmd.Flags().Args()
		if err := signcmd.SignTagCmd(cfg, args, targetRepo, &signcmd.SignTagArgs{
			Message:               msg,
			Fee:                   fee,
			Nonce:                 nonce,
			Value:                 value,
			PushKeyID:             signingKey,
			PushKeyPass:           signingKeyPass,
			Remote:                targetRemotes,
			ResetTokens:           resetRemoteTokens,
			RPCClient:             client,
			RemoteClients:         remoteClients,
			KeyUnlocker:           common.UnlockKey,
			GetNextNonce:          utils.GetNextNonceOfPushKeyOwner,
			RemoteURLTokenUpdater: server.UpdateRemoteURLsWithPushToken,
		}); err != nil {
			cfg.G().Log.Fatal(err.Error())
		}
	},
}

var signNoteCmd = &cobra.Command{
	Use:   "notes",
	Short: "Sign a note",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		fee, _ := cmd.Flags().GetString("fee")
		value, _ := cmd.Flags().GetString("value")
		nonce, _ := cmd.Flags().GetUint64("nonce")
		signingKey, _ := cmd.Flags().GetString("signing-key")
		signingKeyPass, _ := cmd.Flags().GetString("signing-key-pass")
		targetRemotes, _ := cmd.Flags().GetString("remote")
		resetRemoteTokens, _ := cmd.Flags().GetBool("reset")

		if len(args) == 0 {
			log.Fatal("name is required")
		}

		targetRepo, client, remoteClients := getRepoAndClients(cmd)
		if targetRepo == nil {
			log.Fatal("no repository found in current directory")
		}

		if err := signcmd.SignNoteCmd(cfg, targetRepo, &signcmd.SignNoteArgs{
			Name:                  args[0],
			Fee:                   fee,
			Nonce:                 nonce,
			Value:                 value,
			PushKeyID:             signingKey,
			PushKeyPass:           signingKeyPass,
			Remote:                targetRemotes,
			ResetTokens:           resetRemoteTokens,
			RPCClient:             client,
			RemoteClients:         remoteClients,
			KeyUnlocker:           common.UnlockKey,
			GetNextNonce:          utils.GetNextNonceOfPushKeyOwner,
			RemoteURLTokenUpdater: server.UpdateRemoteURLsWithPushToken,
		}); err != nil {
			log.Fatal(err.Error())
		}
	},
}

func init() {
	rootCmd.AddCommand(signCmd)
	signCmd.AddCommand(signTagCmd)
	signCmd.AddCommand(signCommitCmd)
	signCmd.AddCommand(signNoteCmd)

	pf := signCmd.PersistentFlags()

	// Top-level flags
	pf.BoolP("reset", "x", false, "Clear any existing remote tokens")

	signCommitCmd.Flags().String("merge-id", "", "Provide a merge proposal ID for merge fulfilment")
	signCommitCmd.Flags().String("head", "", "Specify the branch to use as git HEAD")
	signCommitCmd.Flags().StringP("branch", "b", "", "Specify a target branch to sign (default: HEAD)")
	signCommitCmd.Flags().Bool("force", false, "Forcefully checkout the target branch to sign")
	signCommitCmd.Flags().BoolP("amend", "a", false, "Amend and sign the recent comment instead of a new one")

	// Transaction information
	pf.StringP("message", "m", "", "commit or tag message")
	pf.StringP("value", "v", "", "Set a value for paying additional fees")
	pf.StringP("remote", "r", "origin", "Set push token to a remote")

	// API connection config flags
	addAPIConnectionFlags(pf)

	// Common Tx flags
	addCommonTxFlags(pf)
}
