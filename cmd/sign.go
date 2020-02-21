// Copyright © 2019 NAME HERE <EMAIL ADDRESS>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"net"
	"strconv"

	"gitlab.com/makeos/mosdef/rpc"

	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/repo"
	"gitlab.com/makeos/mosdef/rpc/client"

	"github.com/spf13/cobra"
)

func getRPCClient(cmd *cobra.Command) (*client.RPCClient, error) {
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

		var client *client.RPCClient
		var err error
		if nonce == "0" {
			client, err = getRPCClient(cmd)
			if err != nil {
				log.Fatal(err.Error())
			}
		}

		if err := repo.SignCommitCmd(cfg.Node.GitBinPath, fee,
			nonce, sk, amend, delete, mergeID, client); err != nil {
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

		var client *client.RPCClient
		var err error
		if nonce == "" || nonce == "0" {
			client, err = getRPCClient(cmd)
			if err != nil {
				log.Fatal(err.Error())
			}
		}

		args = cmd.Flags().Args()
		if err := repo.SignTagCmd(args, cfg.Node.GitBinPath, fee,
			nonce, sk, delete, client); err != nil {
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

		var client *client.RPCClient
		var err error
		if nonce == "" || nonce == "0" {
			client, err = getRPCClient(cmd)
			if err != nil {
				log.Fatal(err.Error())
			}
		}

		if len(args) == 0 {
			log.Fatal("name is required")
		}

		if err := repo.SignNoteCmd(
			cfg.Node.GitBinPath,
			fee,
			nonce,
			sk,
			args[0],
			delete,
			client); err != nil {
			log.Fatal(err.Error())
		}
	},
}

func initCommit() {
	rootCmd.AddCommand(signCmd)
	signCmd.AddCommand(signTagCmd)
	signCmd.AddCommand(signCommitCmd)
	signCmd.AddCommand(signNoteCmd)

	pf := signCmd.PersistentFlags()
	pf.StringP("fee", "f", "0", "Set the transaction fee")
	pf.StringP("nonce", "n", "0", "Set the transaction nonce")
	pf.StringP("signing-key", "s", "", "Set the GPG signing key ID")
	pf.String("rpc.user", "", "Set the RPC username")
	pf.String("rpc.password", "", "Set the RPC password")
	pf.String("rpc.address", config.DefaultRPCAddress, "Set the RPC listening address")
	pf.String("rpc.https", "", "Force the client to use https:// protocol")
	pf.BoolP("delete", "d", false, "Add a directive to delete the target reference")
	signCommitCmd.Flags().StringP("merge-id", "m", "", "Provide a merge proposal ID for merge fulfilment")
	signCommitCmd.Flags().BoolP("amend", "a", false, "Amend and sign the recent comment instead of a new one")
}
