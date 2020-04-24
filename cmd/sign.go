package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gitlab.com/makeos/mosdef/config"
	cmd2 "gitlab.com/makeos/mosdef/remote/cmd"
)

// signCmd represents the commit command
var signCmd = &cobra.Command{
	Use:   "sign",
	Short: "Sign a commit, tag or note and generate push request token",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Unknown command. See usage below")
		cmd.Help()
	},
}

var signCommitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Sign or amend current commit",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		fee, _ := cmd.Flags().GetString("fee")
		nonce, _ := cmd.Flags().GetString("nonce")
		sk, _ := cmd.Flags().GetString("signing-key")
		mergeID, _ := cmd.Flags().GetString("merge-id")
		head, _ := cmd.Flags().GetString("head")
		branch, _ := cmd.Flags().GetString("branch")
		forceCheckout, _ := cmd.Flags().GetBool("force")
		amend, _ := cmd.Flags().GetBool("amend")
		pass, _ := cmd.Flags().GetString("pass")
		msg, _ := cmd.Flags().GetString("message")
		targetRemotes, _ := cmd.Flags().GetString("remote")
		resetRemoteTokens, _ := cmd.Flags().GetBool("reset")

		targetRepo, client, remoteClients := getRepoAndClients(cmd, nonce)
		if err := cmd2.SignCommitCmd(cfg, targetRepo, cmd2.SignCommitCmdArgs{
			Message:       msg,
			Fee:           fee,
			Nonce:         nonce,
			AmendCommit:   amend,
			MergeID:       mergeID,
			Head:          head,
			Branch:        branch,
			ForceCheckout: forceCheckout,
			PushKeyID:     sk,
			PushKeyPass:   pass,
			Remote:        targetRemotes,
			ResetTokens:   resetRemoteTokens,
			RPCClient:     client,
			RemoteClients: remoteClients,
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
		nonce, _ := cmd.Flags().GetString("nonce")
		sk, _ := cmd.Flags().GetString("signing-key")
		pass, _ := cmd.Flags().GetString("pass")
		msg, _ := cmd.Flags().GetString("message")
		targetRemotes, _ := cmd.Flags().GetString("remote")
		resetRemoteTokens, _ := cmd.Flags().GetBool("reset")

		targetRepo, client, remoteClients := getRepoAndClients(cmd, nonce)

		args = cmd.Flags().Args()
		if err := cmd2.SignTagCmd(cfg, args, targetRepo, cmd2.SignTagArgs{
			Message:       msg,
			Fee:           fee,
			Nonce:         nonce,
			PushKeyID:     sk,
			PushKeyPass:   pass,
			Remote:        targetRemotes,
			ResetTokens:   resetRemoteTokens,
			RPCClient:     client,
			RemoteClients: remoteClients,
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
		nonce, _ := cmd.Flags().GetString("nonce")
		sk, _ := cmd.Flags().GetString("signing-key")
		pass, _ := cmd.Flags().GetString("pass")
		targetRemotes, _ := cmd.Flags().GetString("remote")
		resetRemoteTokens, _ := cmd.Flags().GetBool("reset")

		if len(args) == 0 {
			log.Fatal("name is required")
		}

		targetRepo, client, remoteClients := getRepoAndClients(cmd, nonce)
		if err := cmd2.SignNoteCmd(cfg, targetRepo, cmd2.SignNoteArgs{
			Fee:           fee,
			Nonce:         nonce,
			PushKeyID:     sk,
			PushKeyPass:   pass,
			Remote:        targetRemotes,
			ResetTokens:   resetRemoteTokens,
			RPCClient:     client,
			RemoteClients: remoteClients,
		}); err != nil {
			log.Fatal(err.Error())
		}
	},
}

func addAPIConnectionFlags(pf *pflag.FlagSet) {
	pf.String("rpc.user", "", "Set the RPC username")
	pf.String("rpc.password", "", "Set the RPC password")
	pf.String("rpc.address", config.DefaultRPCAddress, "Set the RPC listening address")
	pf.Bool("rpc.https", false, "Force the client to use https:// protocol")
	pf.Bool("no.remote", false, "Disable the ability to query the Remote API")
	pf.Bool("no.rpc", false, "Disable the ability to query the JSON-RPC API")
}

func init() {
	rootCmd.AddCommand(signCmd)
	signCmd.AddCommand(signTagCmd)
	signCmd.AddCommand(signCommitCmd)
	signCmd.AddCommand(signNoteCmd)

	pf := signCmd.PersistentFlags()

	// Top-level flags
	pf.StringP("pass", "p", "", "Passphrase used to unlock the signing key")
	pf.BoolP("reset", "x", false, "Clear any existing remote tokens")

	signCommitCmd.Flags().String("merge-id", "", "Provide a merge proposal ID for merge fulfilment")
	signCommitCmd.Flags().String("head", "", "Specify the branch to use as git HEAD")
	signCommitCmd.Flags().StringP("branch", "b", "", "Specify a target branch to sign (default: HEAD)")
	signCommitCmd.Flags().Bool("force", false, "Forcefully checkout the target branch to sign")
	signCommitCmd.Flags().BoolP("amend", "a", false, "Amend and sign the recent comment instead of a new one")

	// Transaction information
	pf.StringP("message", "m", "", "commit or tag message")
	pf.StringP("fee", "f", "0", "Set the transaction fee")
	pf.StringP("nonce", "n", "0", "Set the transaction nonce")
	pf.StringP("signing-key", "s", "", "Set the signing key ID")
	pf.StringP("remote", "r", "origin", "Set push token to a remote")

	// API connection config flags
	addAPIConnectionFlags(pf)
}
