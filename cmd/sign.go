package cmd

import (
	"os"

	"github.com/spf13/cast"
	"github.com/spf13/cobra"
	"github.com/themakeos/lobe/api/utils"
	"github.com/themakeos/lobe/cmd/common"
	"github.com/themakeos/lobe/cmd/signcmd"
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
		rejectFlagCombo(cmd, "ref-only", "token-only")

		fee, _ := cmd.Flags().GetFloat64("fee")
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
		setRemoteTokenOnly, _ := cmd.Flags().GetBool("no-username")
		refOnly, _ := cmd.Flags().GetBool("ref-only")
		tokenOnly, _ := cmd.Flags().GetBool("token-only")
		forceSign, _ := cmd.Flags().GetBool("force-sign")

		targetRepo, client, remoteClients := getRepoAndClients("", cmd)
		if targetRepo == nil {
			log.Fatal("no repository found in current directory")
		}

		if err := signcmd.SignCommitCmd(cfg, targetRepo, &signcmd.SignCommitArgs{
			Message:                       msg,
			Fee:                           cast.ToString(fee),
			Nonce:                         nonce,
			Value:                         value,
			AmendCommit:                   amend,
			MergeID:                       mergeID,
			Head:                          head,
			Branch:                        branch,
			ForceCheckout:                 forceCheckout,
			SigningKey:                    signingKey,
			PushKeyPass:                   signingKeyPass,
			Remote:                        targetRemotes,
			ResetTokens:                   resetRemoteTokens,
			SetRemotePushTokensOptionOnly: setRemoteTokenOnly,
			CreatePushTokenOnly:           tokenOnly,
			SignRefOnly:                   refOnly,
			ForceSign:                     forceSign,
			RPCClient:                     client,
			RemoteClients:                 remoteClients,
			Stdout:                        os.Stdout,
			Stderr:                        os.Stderr,
			KeyUnlocker:                   common.UnlockKey,
			GetNextNonce:                  utils.GetNextNonceOfPushKeyOwner,
			SetRemotePushToken:            server.SetRemotePushToken,
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
		rejectFlagCombo(cmd, "ref-only", "token-only")

		fee, _ := cmd.Flags().GetFloat64("fee")
		value, _ := cmd.Flags().GetString("value")
		nonce, _ := cmd.Flags().GetUint64("nonce")
		signingKey, _ := cmd.Flags().GetString("signing-key")
		signingKeyPass, _ := cmd.Flags().GetString("signing-key-pass")
		msg, _ := cmd.Flags().GetString("message")
		targetRemotes, _ := cmd.Flags().GetString("remote")
		resetRemoteTokens, _ := cmd.Flags().GetBool("reset")
		force, _ := cmd.Flags().GetBool("force")
		setRemoteTokenOnly, _ := cmd.Flags().GetBool("no-username")
		refOnly, _ := cmd.Flags().GetBool("ref-only")
		tokenOnly, _ := cmd.Flags().GetBool("token-only")
		forceSign, _ := cmd.Flags().GetBool("force-sign")

		targetRepo, client, remoteClients := getRepoAndClients("", cmd)
		if targetRepo == nil {
			log.Fatal("no repository found in current directory")
		}

		args = cmd.Flags().Args()
		if err := signcmd.SignTagCmd(cfg, args, targetRepo, &signcmd.SignTagArgs{
			Message:                       msg,
			Fee:                           cast.ToString(fee),
			Nonce:                         nonce,
			Value:                         value,
			SigningKey:                    signingKey,
			PushKeyPass:                   signingKeyPass,
			Force:                         force,
			Remote:                        targetRemotes,
			ResetTokens:                   resetRemoteTokens,
			SetRemotePushTokensOptionOnly: setRemoteTokenOnly,
			CreatePushTokenOnly:           tokenOnly,
			SignRefOnly:                   refOnly,
			ForceSign:                     forceSign,
			RPCClient:                     client,
			RemoteClients:                 remoteClients,
			Stdout:                        os.Stdout,
			Stderr:                        os.Stderr,
			KeyUnlocker:                   common.UnlockKey,
			GetNextNonce:                  utils.GetNextNonceOfPushKeyOwner,
			SetRemotePushToken:            server.SetRemotePushToken,
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
		fee, _ := cmd.Flags().GetFloat64("fee")
		value, _ := cmd.Flags().GetString("value")
		nonce, _ := cmd.Flags().GetUint64("nonce")
		signingKey, _ := cmd.Flags().GetString("signing-key")
		signingKeyPass, _ := cmd.Flags().GetString("signing-key-pass")
		targetRemotes, _ := cmd.Flags().GetString("remote")
		resetRemoteTokens, _ := cmd.Flags().GetBool("reset")
		setRemoteTokenOnly, _ := cmd.Flags().GetBool("no-username")

		if len(args) == 0 {
			log.Fatal("name is required")
		}

		targetRepo, client, remoteClients := getRepoAndClients("", cmd)
		if targetRepo == nil {
			log.Fatal("no repository found in current directory")
		}

		if err := signcmd.SignNoteCmd(cfg, targetRepo, &signcmd.SignNoteArgs{
			Name:                          args[0],
			Fee:                           cast.ToString(fee),
			Nonce:                         nonce,
			Value:                         value,
			SigningKey:                    signingKey,
			PushKeyPass:                   signingKeyPass,
			Remote:                        targetRemotes,
			ResetTokens:                   resetRemoteTokens,
			RPCClient:                     client,
			RemoteClients:                 remoteClients,
			SetRemotePushTokensOptionOnly: setRemoteTokenOnly,
			Stdout:                        os.Stdout,
			Stderr:                        os.Stderr,
			KeyUnlocker:                   common.UnlockKey,
			GetNextNonce:                  utils.GetNextNonceOfPushKeyOwner,
			SetRemotePushToken:            server.SetRemotePushToken,
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
	pf.Bool("no-username", false, "Don't sent tokens on remote URLs")

	signCommitCmd.Flags().String("merge-id", "", "Provide a merge proposal ID for merge fulfilment")
	signCommitCmd.Flags().String("head", "", "Specify the branch to use as git HEAD")
	signCommitCmd.Flags().StringP("branch", "b", "", "Specify a target branch to sign (default: HEAD)")
	signCommitCmd.Flags().Bool("force", false, "Forcefully checkout the target branch to sign")
	signCommitCmd.Flags().BoolP("amend", "a", false, "Amend and sign the recent comment instead of a new one")
	signCommitCmd.Flags().Bool("ref-only", false, "Only sign the commit object")
	signCommitCmd.Flags().Bool("token-only", false, "Only create and sign the push token")
	signCommitCmd.Flags().Bool("force-sign", false, "Forcefully sign the commit even when it has already been signed")
	signTagCmd.Flags().Bool("force", false, "Overwrite existing tag with matching name")
	signTagCmd.Flags().Bool("ref-only", false, "Only sign the tag object")
	signTagCmd.Flags().Bool("token-only", false, "Only create and sign the push token")
	signTagCmd.Flags().Bool("force-sign", false, "Forcefully sign the tag even when it has already been signed")

	// Transaction information
	pf.StringP("message", "m", "", "commit or tag message")
	pf.StringP("value", "v", "", "Set a value for paying additional fees")
	pf.StringP("remote", "r", "origin", "Set push token to a remote")

	// API connection config flags
	addAPIConnectionFlags(pf)

	// Common Tx flags
	addCommonTxFlags(pf)
}
