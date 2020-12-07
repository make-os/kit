package cmd

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/make-os/kit/remote/plumbing"
	rr "github.com/make-os/kit/remote/repo"
	"github.com/make-os/kit/remote/types"
	"github.com/make-os/kit/rpc/client"
	types2 "github.com/make-os/kit/rpc/types"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
)

// getRPCClient returns an RPC client. If target repo is provided,
// the RPC server information will be extracted from one of the remote URLs.
// The target remote is set via viper's "remote.name" or "--remote" root flag.
func getRPCClient(targetRepo types.LocalRepo) (*client.RPCClient, error) {
	remoteName := viper.GetString("remote.name")
	rpcAddress := viper.GetString("remote.address")
	rpcUser := viper.GetString("rpc.user")
	rpcPassword := viper.GetString("rpc.password")
	rpcSecured := viper.GetBool("rpc.https")

	var err error
	var host, port string

	// If a target repo is provided, get the URL from the specified remote
	if targetRepo != nil {
		h, p, ok := getRemoteAddrFromRepo(targetRepo, remoteName)
		if ok {
			host, port = h, cast.ToString(p)
			goto create
		}
	}

	host, port, err = net.SplitHostPort(rpcAddress)
	if err != nil {
		return nil, errors.Wrap(err, "failed parse rpc address")
	}

create:
	c := client.NewClient(&types2.Options{
		Host:     host,
		Port:     cast.ToInt(port),
		User:     rpcUser,
		Password: rpcPassword,
		HTTPS:    rpcSecured,
	})

	return c, nil
}

// getRemoteAddrFromRepo gets remote address from the given repo.
// It will return false if no (good) url was found.
// The target remote whose url will be return can be set via --remote flag.
func getRemoteAddrFromRepo(repo types.LocalRepo, remoteName string) (string, int, bool) {
	urls := repo.GetRemoteURLs(remoteName)
	if len(urls) > 0 {
		for _, url := range urls {
			ep, err := transport.NewEndpoint(url)
			if err != nil {
				continue
			}
			return ep.Host, ep.Port, true
		}
	}
	return "", 0, false
}

// getRepoAndClient opens a the repository on the current working directory
// and returns an RPC client.
func getRepoAndClient(repoDir string) (types.LocalRepo, types2.Client) {

	var err error
	var targetRepo types.LocalRepo

	if repoDir == "" {
		targetRepo, err = rr.GetAtWorkingDir(cfg.Node.GitBinPath)
	} else {
		targetRepo, err = rr.GetWithLiteGit(cfg.Node.GitBinPath, repoDir)
	}

	rpcClient, err := getRPCClient(targetRepo)
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

// normalMergeReferenceName normalizes a reference from args[0] to one that
// is a valid full merge request reference name.
func normalMergeReferenceName(curRepo types.LocalRepo, args []string) string {
	var ref string
	var err error

	if len(args) > 0 {
		ref = strings.ToLower(args[0])
	}

	// If reference is not set, use the HEAD as the reference.
	// But only if the reference is a valid merge request reference.
	if ref == "" {
		ref, err = curRepo.Head()
		if err != nil {
			log.Fatal(errors.Wrap(err, "failed to get HEAD").Error())
		}
		if !plumbing.IsMergeRequestReference(ref) {
			log.Fatal(fmt.Sprintf("not a valid merge request path (%s)", ref))
		}
	}

	// If the reference begins with 'merges',
	// Add the full prefix 'refs/heads/' to make it `refs/heads/merges/<ref>`
	if strings.HasPrefix(ref, plumbing.MergeRequestBranchPrefix) {
		ref = fmt.Sprintf("refs/heads/%s", ref)
	}

	// If the reference does not begin with 'refs/heads/merges',
	// convert to 'refs/heads/merges/<ref>'
	if !plumbing.IsMergeRequestReferencePath(ref) {
		ref = plumbing.MakeMergeRequestReference(ref)
	}

	// Finally, if reference is not of the form `refs/heads/merges/*`
	if !plumbing.IsMergeRequestReference(ref) {
		log.Fatal(fmt.Sprintf("not a valid merge request path (%s)", ref))
	}

	return ref
}

// normalizeIssueReferenceName normalizes a reference from args[0] to one that
// is a valid full issue reference name.
func normalizeIssueReferenceName(curRepo types.LocalRepo, args []string) string {
	var ref string
	var err error

	if len(args) > 0 {
		ref = args[0]
	}

	// If reference is not set, use the HEAD as the reference.
	// But only if the reference is a valid issue reference.
	if ref == "" {
		ref, err = curRepo.Head()
		if err != nil {
			log.Fatal(errors.Wrap(err, "failed to get HEAD").Error())
		}
		if !plumbing.IsIssueReference(ref) {
			log.Fatal(fmt.Sprintf("not an issue path (%s)", ref))
		}
	}

	// If the reference begins with 'issues',
	// Add the full prefix 'refs/heads/' to make it `refs/heads/issues/<ref>`
	ref = strings.ToLower(ref)
	if strings.HasPrefix(ref, plumbing.IssueBranchPrefix) {
		ref = fmt.Sprintf("refs/heads/%s", ref)
	}

	// If the reference does not begin with 'refs/heads/issues',
	// convert to 'refs/heads/issues/<ref>'
	if !plumbing.IsIssueReferencePath(ref) {
		ref = plumbing.MakeIssueReference(ref)
	}

	// Finally, if reference is not of the form `refs/heads/issues/*`
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
