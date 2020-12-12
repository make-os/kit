package signcmd

import (
	"os"

	"github.com/make-os/kit/cmd/common"
	"github.com/make-os/kit/cmd/signcmd/types"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/remote/server"
	"github.com/make-os/kit/util/api"
	"github.com/spf13/cast"
	"github.com/spf13/cobra"
)

var (
	cfg = config.GetConfig()
	log = cfg.G().Log
)

// SignCmd represents the commit command
var SignCmd = &cobra.Command{
	Use:   "sign [command]",
	Short: "Sign a commit, tag or note",
	Long:  `Sign a commit, tag or note. Run 'kit sign' to sign the current commit.`,
	Run: func(cmd *cobra.Command, args []string) {
		signCommitCmd.Run(cmd, args)
	},
}

var signCommitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Sign a commit or branch",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		fee, _ := cmd.Flags().GetFloat64("fee")
		value, _ := cmd.Flags().GetString("value")
		nonce, _ := cmd.Flags().GetUint64("nonce")
		signingKey, _ := cmd.Flags().GetString("signing-key")
		mergeID, _ := cmd.Flags().GetString("merge-id")
		head, _ := cmd.Flags().GetString("head")
		signingKeyPass, _ := cmd.Flags().GetString("signing-key-pass")
		targetRemotes, _ := cmd.Flags().GetString("remote")
		resetRemoteTokens, _ := cmd.Flags().GetBool("reset")

		targetRepo, client := common.GetRepoAndClient(cfg, "")
		if targetRepo == nil {
			log.Fatal("no repository found in current directory")
		}

		if err := SignCommitCmd(cfg, targetRepo, &types.SignCommitArgs{
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
	Short: "Sign an annotated tag",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		fee, _ := cmd.Flags().GetFloat64("fee")
		value, _ := cmd.Flags().GetString("value")
		nonce, _ := cmd.Flags().GetUint64("nonce")
		signingKey, _ := cmd.Flags().GetString("signing-key")
		signingKeyPass, _ := cmd.Flags().GetString("signing-key-pass")
		targetRemotes, _ := cmd.Flags().GetString("remote")
		resetRemoteTokens, _ := cmd.Flags().GetBool("reset")

		targetRepo, client := common.GetRepoAndClient(cfg, "")
		if targetRepo == nil {
			log.Fatal("no repository found in current directory")
		}

		args = cmd.Flags().Args()
		if err := SignTagCmd(cfg, args, targetRepo, &types.SignTagArgs{
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

		if len(args) == 0 {
			log.Fatal("name is required")
		}

		targetRepo, client := common.GetRepoAndClient(cfg, "")
		if targetRepo == nil {
			log.Fatal("no repository found in current directory")
		}

		if err := SignNoteCmd(cfg, targetRepo, &types.SignNoteArgs{
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
	cmd.Flags().StringP("merge-id", "m", "", "Provide a merge proposal ID for merge fulfilment")
	cmd.Flags().String("head", "", "Specify the branch to use as git HEAD")
}

func init() {
	SignCmd.AddCommand(signTagCmd)
	SignCmd.AddCommand(signCommitCmd)
	SignCmd.AddCommand(signNoteCmd)

	pf := SignCmd.PersistentFlags()

	// Top-level flags
	pf.BoolP("reset", "x", false, "Clear existing remote tokens before creating a new one")
	pf.StringP("value", "v", "", "Set a value for paying additional fees")
	pf.StringP("remote", "r", "origin", "Set push token to a remote")

	setupSignCommitCmd(signCommitCmd)
	setupSignCommitCmd(SignCmd)

	pf.Float64P("fee", "f", 0, "Set the network transaction fee")
	pf.Uint64P("nonce", "n", 0, "Set the next nonce of the signing account signing")
	pf.StringP("signing-key", "u", "", "Address or index of local account to use for signing transaction")
	pf.StringP("signing-key-pass", "p", "", "Passphrase for unlocking the signing account")
	_ = SignCmd.MarkFlagRequired("fee")
	_ = SignCmd.MarkFlagRequired("signing-key")
}
