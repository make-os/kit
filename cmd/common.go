package cmd

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/make-os/lobe/remote/plumbing"
	rr "github.com/make-os/lobe/remote/repo"
	"github.com/make-os/lobe/remote/types"
	"github.com/make-os/lobe/rpc/client"
	types2 "github.com/make-os/lobe/rpc/types"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// getRPCClient creates and returns an RPC using RPC-related flags in the given command.
// It will return nil client and nil error if --no.rpc flag is set.
func getRPCClient(cmd *cobra.Command) (*client.RPCClient, error) {
	rpcAddress, _ := cmd.Flags().GetString("remote.address")
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

	c := client.NewClient(&types2.Options{
		Host:     host,
		Port:     portInt,
		User:     rpcUser,
		Password: rpcPassword,
		HTTPS:    rpcSecured,
	})

	return c, nil
}

// getRepoAndClient opens a the repository on the current working directory
// and returns an initialized RPC client.
// If a repository is found on the current working directory,
// the remote urls are collected and used to initialize the client.
func getRepoAndClient(repoDir string, cmd *cobra.Command) (types.LocalRepo, types2.Client) {

	var err error
	var targetRepo types.LocalRepo

	if repoDir == "" {
		targetRepo, err = rr.GetAtWorkingDir(cfg.Node.GitBinPath)
	} else {
		targetRepo, err = rr.GetWithLiteGit(cfg.Node.GitBinPath, repoDir)
	}

	// Get JSON RPCClient client
	rpcClient, err := getRPCClient(cmd)
	if err != nil {
		log.Fatal(err.Error())
	}

	return targetRepo, rpcClient
}

// rejectFlagCombo rejects unwanted flag combination
func rejectFlagCombo(cmd *cobra.Command, flags ...string) {
	var found []string
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
