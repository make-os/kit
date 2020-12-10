package cmd

import (
	"os"

	"github.com/make-os/kit/cmd/common"
	"github.com/make-os/kit/cmd/signcmd"
	"github.com/make-os/kit/remote/server"
	"github.com/make-os/kit/util/api"
	"github.com/spf13/cast"
	"github.com/spf13/cobra"
)

// signCmd represents the commit command
var signCmd = &cobra.Command{
	Use:   "sign",
	Short: "Sign a commit, tag or note and generate a push request token",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		signCommitCmd.Run(cmd, args)
	},
}

var signCommitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Sign or amend current commit",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		fee, _ := cmd.Flags().GetFloat64("fee")
		value, _ := cmd.Flags().GetString("value")
		nonce, _ := cmd.Flags().GetUint64("nonce")
		signingKey, _ := cmd.Flags().GetString("signing-key")
		mergeID, _ := cmd.Flags().GetString("merge-id")
		head, _ := cmd.Flags().GetString("head")
		signingKeyPass, _ := cmd.Flags().GetString("signing-key-pass")
		msg, _ := cmd.Flags().GetString("message")
		targetRemotes, _ := cmd.Flags().GetString("remote")
		resetRemoteTokens, _ := cmd.Flags().GetBool("reset")

		targetRepo, client := getRepoAndClient("")
		if targetRepo == nil {
			log.Fatal("no repository found in current directory")
		}

		if err := signcmd.SignCommitCmd(cfg, targetRepo, &signcmd.SignCommitArgs{
			Message:                      msg,
			Fee:                          cast.ToString(fee),
			Nonce:                        nonce,
			Value:                        value,
			MergeID:                      mergeID,
			Head:                         head,
			SigningKey:                   signingKey,
			PushKeyPass:                  signingKeyPass,
			Remote:                       targetRemotes,
			ResetTokens:                  resetRemoteTokens,
			RPCClient:                    client,
			Stdout:                       os.Stdout,
			Stderr:                       os.Stderr,
			KeyUnlocker:                  common.UnlockKey,
			GetNextNonce:                 api.GetNextNonceOfPushKeyOwner,
			CreateApplyPushTokenToRemote: server.MakeAndApplyPushTokenToRemote,
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
		fee, _ := cmd.Flags().GetFloat64("fee")
		value, _ := cmd.Flags().GetString("value")
		nonce, _ := cmd.Flags().GetUint64("nonce")
		signingKey, _ := cmd.Flags().GetString("signing-key")
		signingKeyPass, _ := cmd.Flags().GetString("signing-key-pass")
		targetRemotes, _ := cmd.Flags().GetString("remote")
		resetRemoteTokens, _ := cmd.Flags().GetBool("reset")

		targetRepo, client := getRepoAndClient("")
		if targetRepo == nil {
			log.Fatal("no repository found in current directory")
		}

		args = cmd.Flags().Args()
		if err := signcmd.SignTagCmd(cfg, args, targetRepo, &signcmd.SignTagArgs{
			Fee:                          cast.ToString(fee),
			Nonce:                        nonce,
			Value:                        value,
			SigningKey:                   signingKey,
			PushKeyPass:                  signingKeyPass,
			Remote:                       targetRemotes,
			ResetTokens:                  resetRemoteTokens,
			RPCClient:                    client,
			Stdout:                       os.Stdout,
			Stderr:                       os.Stderr,
			KeyUnlocker:                  common.UnlockKey,
			GetNextNonce:                 api.GetNextNonceOfPushKeyOwner,
			CreateApplyPushTokenToRemote: server.MakeAndApplyPushTokenToRemote,
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

		if len(args) == 0 {
			log.Fatal("name is required")
		}

		targetRepo, client := getRepoAndClient("")
		if targetRepo == nil {
			log.Fatal("no repository found in current directory")
		}

		if err := signcmd.SignNoteCmd(cfg, targetRepo, &signcmd.SignNoteArgs{
			Name:                         args[0],
			Fee:                          cast.ToString(fee),
			Nonce:                        nonce,
			Value:                        value,
			SigningKey:                   signingKey,
			PushKeyPass:                  signingKeyPass,
			Remote:                       targetRemotes,
			ResetTokens:                  resetRemoteTokens,
			RPCClient:                    client,
			Stdout:                       os.Stdout,
			Stderr:                       os.Stderr,
			KeyUnlocker:                  common.UnlockKey,
			GetNextNonce:                 api.GetNextNonceOfPushKeyOwner,
			CreateApplyPushTokenToRemote: server.MakeAndApplyPushTokenToRemote,
		}); err != nil {
			log.Fatal(err.Error())
		}
	},
}

func setupSignCommitCmd(cmd *cobra.Command) {
	cmd.Flags().String("merge-id", "", "Provide a merge proposal ID for merge fulfilment")
	cmd.Flags().String("head", "", "Specify the branch to use as git HEAD")
}

func setupSignTagCmd(cmd *cobra.Command) {
	cmd.Flags().Bool("force", false, "Overwrite existing tag with matching name")
}

func init() {
	rootCmd.AddCommand(signCmd)
	signCmd.AddCommand(signTagCmd)
	signCmd.AddCommand(signCommitCmd)
	signCmd.AddCommand(signNoteCmd)

	pf := signCmd.PersistentFlags()

	// Top-level flags
	pf.BoolP("reset", "x", false, "Clear any existing remote tokens")
	pf.StringP("message", "m", "", "commit message")
	pf.StringP("value", "v", "", "Set a value for paying additional fees")
	pf.StringP("remote", "r", "origin", "Set push token to a remote")

	setupSignCommitCmd(signCommitCmd)
	setupSignCommitCmd(rootCmd)
	setupSignTagCmd(signTagCmd)

	pf.Float64P("fee", "f", 0, "Set the network transaction fee")
	pf.Uint64P("nonce", "n", 0, "Set the next nonce of the signing account signing")
	pf.StringP("signing-key", "u", "", "Address or index of local account to use for signing transaction")
	pf.StringP("signing-key-pass", "p", "", "Passphrase for unlocking the signing account")
	_ = signCmd.MarkFlagRequired("fee")
	_ = signCmd.MarkFlagRequired("signing-key")
}
