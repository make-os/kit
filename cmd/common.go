package cmd

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/thoas/go-funk"
	remote "gitlab.com/makeos/mosdef/api/remote/client"
	"gitlab.com/makeos/mosdef/api/rpc/client"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/remote/plumbing"
	rr "gitlab.com/makeos/mosdef/remote/repo"
	"gitlab.com/makeos/mosdef/remote/types"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
)

// getRPCClient creates and returns an RPC using RPC-related flags in the given command.
// It will return nil client and nil error if --no.rpc flag is set.
func getRPCClient(cmd *cobra.Command) (*client.RPCClient, error) {
	noRPC, _ := cmd.Flags().GetBool("no.rpc")
	if noRPC {
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

	c := client.NewClient(&client.Options{
		Host:     host,
		Port:     portInt,
		User:     rpcUser,
		Password: rpcPassword,
		HTTPS:    rpcSecured,
	})

	return c, nil
}

// getRemoteAPIClients gets REST clients for every  http(s) remote
// URL set on the given repository. Immediately returns nothing if
// --no.remote is true.
func getRemoteAPIClients(cmd *cobra.Command, repo types.LocalRepo) (clients []remote.Client) {
	noRemote, _ := cmd.Flags().GetBool("no.api")
	if noRemote {
		return
	}

	for _, url := range repo.GetRemoteURLs() {
		ep, _ := transport.NewEndpoint(url)
		if !funk.ContainsString([]string{"http", "https"}, ep.Protocol) {
			continue
		}

		apiURL := fmt.Sprintf("%s://%s", ep.Protocol, ep.Host)
		if ep.Port != 0 {
			apiURL = fmt.Sprintf("%s:%d", apiURL, ep.Port)
		}

		clients = append(clients, remote.NewClient(apiURL))
	}
	return
}

// getClients returns an RPC and Remote API clients.
// If a repository is found on the current working directory,
// its remote endpoints are collected and converted to remote clients.
// If `api.address` is set, another remote client is created for it.
func getRepoAndClients(cmd *cobra.Command) (types.LocalRepo, client.Client, []remote.Client) {

	var err error
	var targetRepo types.LocalRepo
	var remoteClients []remote.Client

	targetRepo, err = rr.GetAtWorkingDir(cfg.Node.GitBinPath)
	if targetRepo != nil {
		remoteClients = getRemoteAPIClients(cmd, targetRepo)
	}

	// If remote API address flag was set, create a client with it
	if apiAddr, _ := cmd.Flags().GetString("api.address"); apiAddr != "" {
		proto := "http"
		if ok, _ := cmd.Flags().GetBool("api.https"); ok {
			proto = "https"
		}
		remoteClients = append(remoteClients, remote.NewClient(fmt.Sprintf("%s://%s", proto, apiAddr)))
	}

	// Get JSON RPCClient client
	rpcClient, err := getRPCClient(cmd)
	if err != nil {
		log.Fatal(err.Error())
	}

	return targetRepo, rpcClient, remoteClients
}

// addAPIConnectionFlags adds flags used for connecting to a RPC or Remote API server.
func addAPIConnectionFlags(pf *pflag.FlagSet) {
	pf.String("rpc.user", "", "Set the RPC username")
	pf.String("rpc.password", "", "Set the RPC password")
	pf.String("rpc.address", config.DefaultRPCAddress, "Set the RPC listening address")
	pf.Bool("rpc.https", false, "Force the client to use https:// protocol")
	pf.Bool("no.rpc", false, "Disable the ability to query the JSON-RPC API")
	pf.Bool("no.api", false, "Disable the ability to query the Remote API")
	pf.String("api.address", "", "Set the Remote API address")
	pf.Bool("api.https", false, "Force the client to use https:// protocol")
}

// addCommonTxFlags adds flags required for commands that create network transactions
func addCommonTxFlags(fs *pflag.FlagSet) {
	if fs.Lookup("fee") == nil {
		fs.Float64P("fee", "f", 0, "The transaction fee to pay to the network")
	}
	if fs.Lookup("nonce") == nil {
		fs.Uint64P("nonce", "n", 0, "The next nonce of the account signing the transaction")
	}
	if fs.Lookup("signing-key") == nil {
		fs.StringP("signing-key", "u", "", "Address or index of local account to use for signing the transaction")
	}
	if fs.Lookup("signing-key-pass") == nil {
		fs.StringP("signing-key-pass", "p", "", "Passphrase for unlocking the signing account")
	}
}

// rejectFlagCombo rejects unwanted flag combination
func rejectFlagCombo(cmd *cobra.Command, flags ...string) {
	var found = []string{}
	for _, f := range flags {
		if len(found) > 0 && cmd.Flags().Changed(f) {
			str := "--" + f
			if fShort := cmd.Flags().Lookup(f).Shorthand; fShort != "" {
				str += "|-" + fShort
			}
			found = append(found, str)
			log.Fatal(fmt.Sprintf("flags %s can't be used together", strings.Join(found, ", ")))
		}
		if cmd.Flags().Changed(f) {
			str := "--" + f
			if fShort := cmd.Flags().Lookup(f).Shorthand; fShort != "" {
				str += "|-" + fShort
			}
			found = append(found, str)
		}
	}
}

func getMergeRef(curRepo types.LocalRepo, args []string) string {
	var ref string
	var err error

	if len(args) > 0 {
		ref = strings.ToLower(args[0])
	}

	if ref == "" {
		ref, err = curRepo.Head()
		if err != nil {
			log.Fatal(errors.Wrap(err, "failed to get HEAD").Error())
		}
		if !plumbing.IsMergeRequestReference(ref) {
			log.Fatal(fmt.Sprintf("not a valid merge request path (%s)", ref))
		}
	}

	if strings.HasPrefix(ref, plumbing.MergeRequestBranchPrefix) {
		ref = fmt.Sprintf("refs/heads/%s", ref)
	}
	if !plumbing.IsMergeRequestReferencePath(ref) {
		ref = plumbing.MakeMergeRequestReference(ref)
	}
	if !plumbing.IsMergeRequestReference(ref) {
		log.Fatal(fmt.Sprintf("not a valid merge request path (%s)", ref))
	}

	return ref
}

func getIssueRef(curRepo types.LocalRepo, args []string) string {
	var ref string
	var err error

	if len(args) > 0 {
		ref = args[0]
	}

	if ref == "" {
		ref, err = curRepo.Head()
		if err != nil {
			log.Fatal(errors.Wrap(err, "failed to get HEAD").Error())
		}
		if !plumbing.IsIssueReference(ref) {
			log.Fatal(fmt.Sprintf("not an issue path (%s)", ref))
		}
	}

	ref = strings.ToLower(ref)
	if strings.HasPrefix(ref, plumbing.IssueBranchPrefix) {
		ref = fmt.Sprintf("refs/heads/%s", ref)
	}
	if !plumbing.IsIssueReferencePath(ref) {
		ref = plumbing.MakeIssueReference(ref)
	}
	if !plumbing.IsIssueReference(ref) {
		log.Fatal(fmt.Sprintf("not an issue path (%s)", ref))
	}

	return ref
}

// viperBindFlagSet binds flags of a command to viper only if the command
// is the currently executed command.
func viperBindFlagSet(cmd *cobra.Command) {
	if len(os.Args) > 1 && os.Args[1] == cmd.Name() {
		viper.BindPFlags(cmd.Flags())
	}
}
