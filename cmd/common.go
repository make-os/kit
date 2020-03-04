package cmd

import (
	"fmt"
	"net"
	"strconv"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/thoas/go-funk"
	restclient "gitlab.com/makeos/mosdef/api/rest/client"
	"gitlab.com/makeos/mosdef/api/rpc/client"
	"gitlab.com/makeos/mosdef/repo"
	"gitlab.com/makeos/mosdef/types/core"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
)

// getJSONRPCClient returns a JSON-RPCclient or error if unable to
// create one. It will return nil client and nil error if --no.rpc
// is true.
func getJSONRPCClient(cmd *cobra.Command) (*client.RPCClient, error) {
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
func getRemoteAPIClients(cmd *cobra.Command, repo core.BareRepo) (clients []restclient.RestClient) {
	noRemote, _ := cmd.Flags().GetBool("no.remote")
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

		clients = append(clients, restclient.NewClient(apiURL))
	}
	return
}

// getClients returns RPCClient and Remote API clients
func getRepoAndClients(cmd *cobra.Command, nonceFromFlag string) (core.BareRepo,
	*client.RPCClient, []restclient.RestClient) {

	// Get the repository
	targetRepo, err := repo.GetCurrentWDRepo(cfg.Node.GitBinPath)
	if err != nil {
		log.Fatal(err.Error())
	}

	// Get JSON RPCClient client
	var rpcClient *client.RPCClient
	rpcClient, err = getJSONRPCClient(cmd)
	if err != nil {
		log.Fatal(err.Error())
	}

	// Get remote APIs from the repository
	remoteClients := getRemoteAPIClients(cmd, targetRepo)

	return targetRepo, rpcClient, remoteClients
}
