package cmd

import (
	"os"

	"github.com/make-os/lobe/cmd/common"
	"github.com/make-os/lobe/cmd/signcmd"
	"github.com/make-os/lobe/remote/server"
	"github.com/make-os/lobe/util/api"
	"github.com/spf13/cast"
	"github.com/spf13/cobra"
)

// signCmd represents the commit command
var signCmd = &cobra.Command{
	Use:   "sign",
	Short: "Sign a commit, tag or note and generate a push request token",
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

		targetRepo, client := getRepoAndClient("")
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
			Stdout:                        os.Stdout,
			Stderr:                        os.Stderr,
			KeyUnlocker:                   common.UnlockKey,
			GetNextNonce:                  api.GetNextNonceOfPushKeyOwner,
			SetRemotePushToken:            server.GenSetPushToken,
		}); err != nil {
			cfg.G().Log.Fatal(err.Error())
		}
	},
}

var signTagCmd = &cobra.Command{
	Use:   "tag <name>",
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

		targetRepo, client := getRepoAndClient("")
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
			Stdout:                        os.Stdout,
			Stderr:                        os.Stderr,
			KeyUnlocker:                   common.UnlockKey,
			GetNextNonce:                  api.GetNextNonceOfPushKeyOwner,
			SetRemotePushToken:            server.GenSetPushToken,
		}); err != nil {
			cfg.G().Log.Fatal(err.Error())
		}
	},
}

var signNoteCmd = &cobra.Command{
	Use:   "note <name>",
	Short: "Create a signed push request token for a note",
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

		targetRepo, client := getRepoAndClient("")
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
			SetRemotePushTokensOptionOnly: setRemoteTokenOnly,
			Stdout:                        os.Stdout,
			Stderr:                        os.Stderr,
			KeyUnlocker:                   common.UnlockKey,
			GetNextNonce:                  api.GetNextNonceOfPushKeyOwner,
			SetRemotePushToken:            server.GenSetPushToken,
		}); err != nil {
			log.Fatal(err.Error())
		}
	},
}

func setupSignCommitCmd(cmd *cobra.Command) {
	cmd.Flags().String("merge-id", "", "Provide a merge proposal ID for merge fulfilment")
	cmd.Flags().String("head", "", "Specify the branch to use as git HEAD")
	cmd.Flags().StringP("branch", "b", "", "Specify a target branch to sign (default: HEAD)")
	cmd.Flags().Bool("force", false, "Forcefully checkout the target branch to sign")
	cmd.Flags().BoolP("amend", "a", false, "Amend and sign the recent comment instead of a new one")
	cmd.Flags().Bool("ref-only", false, "Only sign the commit object")
	cmd.Flags().Bool("token-only", false, "Only create and sign the push token")
	cmd.Flags().Bool("force-sign", false, "Forcefully sign the commit even when it has already been signed")
}

func setupSignTagCmd(cmd *cobra.Command) {
	cmd.Flags().Bool("force", false, "Overwrite existing tag with matching name")
	cmd.Flags().Bool("ref-only", false, "Only sign the tag object")
	cmd.Flags().Bool("token-only", false, "Only create and sign the push token")
	cmd.Flags().Bool("force-sign", false, "Forcefully sign the tag even when it has already been signed")
}

func init() {
	rootCmd.AddCommand(signCmd)
	signCmd.AddCommand(signTagCmd)
	signCmd.AddCommand(signCommitCmd)
	signCmd.AddCommand(signNoteCmd)

	pf := signCmd.PersistentFlags()

	// Top-level flags
	pf.BoolP("reset", "x", false, "Clear any existing remote tokens")
	pf.Bool("no-username", false, "Do not add tokens to the username part of the remote URLs")
	pf.StringP("message", "m", "", "commit or tag message")
	pf.StringP("value", "v", "", "Set a value for paying additional fees")
	pf.StringP("remote", "r", "origin", "Set push token to a remote")

	setupSignCommitCmd(signCommitCmd)
	setupSignTagCmd(signTagCmd)

	pf.Float64P("fee", "f", 0, "Set the network transaction fee")
	pf.Uint64P("nonce", "n", 0, "Set the next nonce of the signing account signing")
	pf.StringP("signing-key", "u", "", "Address or index of local account to use for signing transaction")
	pf.StringP("signing-key-pass", "p", "", "Passphrase for unlocking the signing account")
	signCmd.MarkFlagRequired("fee")
	signCmd.MarkFlagRequired("signing-key")
}
