package cmd

import (
	"fmt"
	"net"
	"strconv"

	"gitlab.com/makeos/mosdef/rest"
	"gitlab.com/makeos/mosdef/rpc"
	"gitlab.com/makeos/mosdef/types/core"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"

	"github.com/pkg/errors"
	"github.com/thoas/go-funk"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/repo"
	"gitlab.com/makeos/mosdef/rpc/client"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// getRPCClient returns a JSON-RPC client or error if unable to
// create one. It will return nil client and nil error if --use.rpc
// is false.
func getRPCClient(cmd *cobra.Command) (*client.RPCClient, error) {
	useRPC, _ := cmd.Flags().GetBool("use.rpc")
	if !useRPC {
		return nil, nil
	}

	rpcAddress, _ := cmd.Flags().GetString("rpc.address")
	rpcUser, _ := cmd.Flags().GetString("rpc.user")
	rpcPassword, _ := cmd.Flags().GetString("rpc.password")
	rpcSecured, _ := cmd.Flags().GetBool("rpc.https")

	host, port, err := net.SplitHostPort(rpcAddress)
	if err != nil {
		return nil, errors.Wrap(err, "failed parse rpc address")
	}

	portInt, err := strconv.Atoi(port)
	if err != nil {
		return nil, errors.Wrap(err, "failed convert rpc port")
	}

	c := client.NewClient(&rpc.Options{
		Host:     host,
		Port:     portInt,
		User:     rpcUser,
		Password: rpcPassword,
		HTTPS:    rpcSecured,
	})

	return c, nil
}

// getRemoteAPIClients gets REST clients for every
// http(s) remote URL set on the given repository
func getRemoteAPIClients(repo core.BareRepo) (clients []*rest.Client) {
	for _, url := range repo.GetRemoteURLs() {
		ep, _ := transport.NewEndpoint(url)
		if !funk.ContainsString([]string{"http", "https"}, ep.Protocol) {
			continue
		}

		apiURL := fmt.Sprintf("%s://%s", ep.Protocol, ep.Host)
		if ep.Port != 0 {
			apiURL = fmt.Sprintf("%s:%d", apiURL, ep.Port)
		}

		clients = append(clients, rest.NewClient(apiURL))
	}
	return
}

// getClients returns RPC and Remote API clients
func getRepoAndClients(cmd *cobra.Command, nonce string) (core.BareRepo,
	*client.RPCClient, []*rest.Client) {

	useRemote, _ := cmd.Flags().GetBool("use.remote")

	// Get JSON RPC client
	var client *client.RPCClient
	var err error
	if nonce == "0" {
		client, err = getRPCClient(cmd)
		if err != nil {
			log.Fatal(err.Error())
		}
	}

	// Get the repository
	targetRepo, err := repo.GetCurrentWDRepo(cfg.Node.GitBinPath)
	if err != nil {
		log.Fatal(err.Error())
	}

	// Get remote APIs from the repository
	remoteClients := []*rest.Client{}
	if useRemote && nonce == "0" {
		remoteClients = getRemoteAPIClients(targetRepo)
	}

	return targetRepo, client, remoteClients
}

// signCmd represents the commit command
var signCmd = &cobra.Command{
	Use:   "sign",
	Short: "Signs git commit, tag and note and adds network transaction parameters.",
	Long:  `Signs git commit, tag and note and adds network transaction parameters.`,
	Run:   func(cmd *cobra.Command, args []string) {},
}

var signCommitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Adds a signed commit containing transaction information in the commit message",
	Long: `
Adds a signed commit containing transaction information in the commit message. Use '--amend' or '-a'
flags to update the current commit instead of creating a new one.

The transaction information is included in the commit message and signed. The GPG key used for 
signing the commit is derived from git 'user.signingKey' config parameter. It can also be directly
passed via '--signing-key' or '-s' flags. 

The signer's network account nonce is required in the transaction information, therefore the signer 
may provide their account nonce using '--nonce' or '-n' flags. If not, the command attempts to 
fetch the nonce by first querying the git Remote API and a JSON-RPC API as a fallback. Use the 
'--rpc-*' flags to overwrite the default JSON-RPC connection details.

Setting '--delete' or '-d' to true will include a reference delete directive in the signed
transaction information which will cause the reference to be deleted from the remote. 

If a merge proposal ID is provided via '--merge' or '-m' flags, a merge directive is included in
the signed transaction information which will cause the remote to consider the push operation as
a merge request that will be validated according to the merge proposal contract. 
`,
	Run: func(cmd *cobra.Command, args []string) {
		fee, _ := cmd.Flags().GetString("fee")
		nonce, _ := cmd.Flags().GetString("nonce")
		sk, _ := cmd.Flags().GetString("signingKey")
		delete, _ := cmd.Flags().GetBool("delete")
		mergeID, _ := cmd.Flags().GetString("merge-id")
		amend, _ := cmd.Flags().GetBool("amend")

		targetRepo, client, remoteClients := getRepoAndClients(cmd, nonce)

		if err := repo.SignCommitCmd(targetRepo, fee,
			nonce, sk, amend, delete, mergeID, client, remoteClients); err != nil {
			cfg.G().Log.Fatal(err.Error())
		}
	},
}

var signTagCmd = &cobra.Command{
	Use:   "tag",
	Short: "Create a signed annotated tag containing transaction information.",
	Long: `
Create a signed annotated tag containing transaction information. 

The transaction information is included in the message and signed. The GPG key used for 
signing the commit is derived from git 'user.signingKey' config parameter. It can also 
be directly passed via '--signing-key' or '-s' flags. 

The signer's network account nonce is required in the transaction information, therefore the 
signer may provide their account nonce using '--nonce' or '-n' flags. If not, the command 
attempts to fetch the nonce by first querying the git Remote API and a JSON-RPC API as a 
fallback. Use the '--rpc-*' flags to overwrite the default JSON-RPC connection details.

Setting '--delete' or '-d' to true will include a reference delete directive in the signed
transaction information which will cause the reference to be deleted from the remote. `,
	Run: func(cmd *cobra.Command, args []string) {
		fee, _ := cmd.Flags().GetString("fee")
		nonce, _ := cmd.Flags().GetString("nonce")
		sk, _ := cmd.Flags().GetString("signingKey")
		delete, _ := cmd.Flags().GetBool("delete")

		targetRepo, client, remoteClients := getRepoAndClients(cmd, nonce)

		args = cmd.Flags().Args()
		if err := repo.SignTagCmd(args, targetRepo, fee,
			nonce, sk, delete, client, remoteClients); err != nil {
			cfg.G().Log.Fatal(err.Error())
		}
	},
}

var signNoteCmd = &cobra.Command{
	Use:   "notes",
	Short: "Create a signed note containing transaction information. ",
	Long:  `Create a signed note containing transaction information. `,
	Run: func(cmd *cobra.Command, args []string) {
		fee, _ := cmd.Flags().GetString("fee")
		nonce, _ := cmd.Flags().GetString("nonce")
		sk, _ := cmd.Flags().GetString("signingKey")
		delete, _ := cmd.Flags().GetBool("delete")

		targetRepo, client, remoteClients := getRepoAndClients(cmd, nonce)

		if len(args) == 0 {
			log.Fatal("name is required")
		}

		if err := repo.SignNoteCmd(
			targetRepo,
			fee,
			nonce,
			sk,
			args[0],
			delete,
			client,
			remoteClients); err != nil {
			log.Fatal(err.Error())
		}
	},
}

func addAPIConnectionFlags(pf *pflag.FlagSet) {
	pf.String("rpc.user", "", "Set the RPC username")
	pf.String("rpc.password", "", "Set the RPC password")
	pf.String("rpc.address", config.DefaultRPCAddress, "Set the RPC listening address")
	pf.Bool("rpc.https", false, "Force the client to use https:// protocol")
	pf.Bool("use.remote", true, "Enable the ability to query the Remote API")
	pf.Bool("use.rpc", true, "Enable the ability to query the JSON-RPC API")
}

func initSign() {
	rootCmd.AddCommand(signCmd)
	signCmd.AddCommand(signTagCmd)
	signCmd.AddCommand(signCommitCmd)
	signCmd.AddCommand(signNoteCmd)

	pf := signCmd.PersistentFlags()

	pf.BoolP("delete", "d", false, "Add a directive to delete the target reference")
	signCommitCmd.Flags().StringP("merge-id", "m", "", "Provide a merge proposal ID for merge fulfilment")
	signCommitCmd.Flags().BoolP("amend", "a", false, "Amend and sign the recent comment instead of a new one")

	// Transaction information
	pf.StringP("fee", "f", "0", "Set the transaction fee")
	pf.StringP("nonce", "n", "0", "Set the transaction nonce")
	pf.StringP("signing-key", "s", "", "Set the GPG signing key ID")

	// API connection config flags
	addAPIConnectionFlags(pf)
}
