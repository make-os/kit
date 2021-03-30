package common

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/make-os/kit/cmd/passcmd/agent"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/keystore"
	"github.com/make-os/kit/keystore/types"
	types3 "github.com/make-os/kit/modules/types"
	rr "github.com/make-os/kit/remote/repo"
	remotetypes "github.com/make-os/kit/remote/types"
	"github.com/make-os/kit/rpc/client"
	types2 "github.com/make-os/kit/rpc/types"
	api2 "github.com/make-os/kit/types/api"
	"github.com/make-os/kit/util/api"
	"github.com/make-os/kit/util/colorfmt"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cast"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	ErrBodyRequired           = fmt.Errorf("body is required")
	ErrTitleRequired          = fmt.Errorf("title is required")
	ErrSigningKeyPassRequired = fmt.Errorf("passphrase of signing key is required")
)

// pagerWriter describes a function for writing a specified content to a pager program
type PagerWriter func(pagerCmd string, content io.Reader, stdOut, stdErr io.Writer)

// WriteToPager spawns the specified page, passing the given content to it
func WriteToPager(pagerCmd string, content io.Reader, stdOut, stdErr io.Writer) {
	args := strings.Split(pagerCmd, " ")
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	cmd.Stdin = content
	if err := cmd.Run(); err != nil {
		_, _ = fmt.Fprintln(stdOut, err.Error())
		_, _ = fmt.Fprint(stdOut, content)
		return
	}
}

// UnlockKeyArgs contains arguments for UnlockKey
type UnlockKeyArgs struct {

	// KeyStoreID is the index or address of the key on the keystore
	KeyStoreID string

	// Passphrase is the passphrase to use for unlocking the key
	Passphrase string

	// TargetRepo is the target repository in the current working directory.
	// It's config `user.passphrase` is checked for the passphrase.
	TargetRepo remotetypes.LocalRepo

	// NoPrompt if true, will launch a prompt if passphrase was not gotten from other means
	NoPrompt bool

	// Prompt is a message to print out when launching a prompt.
	Prompt string

	Stdout io.Writer
}

// UnlockKeyFunc describes a function for unlocking a keystore key.
type UnlockKeyFunc func(cfg *config.AppConfig, args *UnlockKeyArgs) (types.StoredKey, error)

// UnlockKey takes a key address or index, unlocks it and returns the key.
// - It will using the given passphrase if set, otherwise
// - if the target repo is set, it will try to get it from the git config (user.passphrase).
// - If passphrase is still unknown, it will attempt to get it from an environment variable.
// - On success, args.Passphrase is updated with the passphrase used to unlock the key.
func UnlockKey(cfg *config.AppConfig, args *UnlockKeyArgs) (types.StoredKey, error) {

	// Get the key from the key store
	ks := keystore.New(cfg.KeystoreDir())
	if args.Stdout != nil {
		ks.SetOutput(args.Stdout)
	}

	// Get the key by address or index
	key, err := ks.GetByIndexOrAddress(args.KeyStoreID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find key (%s)", args.KeyStoreID)
	}

	// If passphrase is unset and target repo is set, attempt to get the
	// passphrase from 'user.passphrase' config.
	protected := !key.IsUnprotected()
	if protected && args.Passphrase == "" && args.TargetRepo != nil {
		repoCfg, err := args.TargetRepo.Config()
		if err != nil {
			return nil, errors.Wrap(err, "unable to get repo config")
		}
		args.Passphrase = repoCfg.Raw.Section("user").Option("passphrase")

		// If we still don't have a passphrase, get it from the repo scoped env variable.
		if args.Passphrase == "" {
			args.Passphrase = os.Getenv(MakeRepoScopedEnvVar(cfg.GetAppName(), args.TargetRepo.GetName(), "PASS"))
		}
	}

	// If key is protected and still no passphrase,
	// try to get it from the general passphrase env variable
	if protected && args.Passphrase == "" {
		args.Passphrase = os.Getenv(MakePassEnvVar(cfg.GetAppName()))
	}

	// Attempt to get the passphrase from the pass agent
	if protected && args.Passphrase == "" {

		// Determine the port; Use default or the value of env <APPNAME>_PASSAGENT_PORT
		port := config.DefaultPassAgentPort
		if envPort := os.Getenv(fmt.Sprintf("%s_PASSAGENT_PORT",
			strings.ToUpper(config.AppName))); envPort != "" {
			port = envPort
		}

		// Get passphrase from pass-agent associated with the key's user address
		if passphrase, err := agent.Get(port, key.GetUserAddress()); err == nil && passphrase != "" {
			args.Passphrase = passphrase
			goto endAgentQuery
		}

		// Get passphrase from pass-agent associated with the key's push address
		if passphrase, err := agent.Get(port, key.GetPushKeyAddress()); err == nil && passphrase != "" {
			args.Passphrase = passphrase
			goto endAgentQuery
		}

		// Get passphrase from pass-agent associated with the original query key
		if passphrase, err := agent.Get(port, args.KeyStoreID); err == nil && passphrase != "" {
			args.Passphrase = passphrase
			goto endAgentQuery
		}

		// Get passphrase from pass-agent associated with the target repository as the key
		if args.TargetRepo != nil {
			if passphrase, err := agent.Get(port, args.TargetRepo.GetName()); err == nil && passphrase != "" {
				args.Passphrase = passphrase
				goto endAgentQuery
			}
		}
	endAgentQuery:
	}

	// If key is protected, still no passphrase and prompting is not allowed -> exit with error
	if protected && args.Passphrase == "" && args.NoPrompt {
		return nil, ErrSigningKeyPassRequired
	}

	key, passphrase, err := ks.UnlockKeyUI(args.KeyStoreID, args.Passphrase, args.Prompt)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to unlock key (%s)", args.KeyStoreID)
	}

	// Set env variable for other components that
	// may require the user-provided passphrase
	_ = os.Setenv(MakePassEnvVar(config.AppName), passphrase)

	return key, nil
}

// MakeRepoScopedEnvVar returns a repo-specific env variable
func MakeRepoScopedEnvVar(appName, repoName, varName string) string {
	return strings.ToUpper(fmt.Sprintf("%s_%s_%s", appName, repoName, varName))
}

// MakePassEnvVar is the name of the env variable expected to contain a key's passphrase.
func MakePassEnvVar(appName string) string {
	return strings.ToUpper(fmt.Sprintf("%s_PASS", appName))
}

type TxStatusTrackerFunc func(stdout io.Writer, hash string, rpcClient types2.Client) error

// ShowTxStatusTracker tracks transaction status and displays updates to stdout.
func ShowTxStatusTracker(stdout io.Writer, hash string, rpcClient types2.Client) error {
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Writer = stdout
	s.Prefix = " "
	s.Start()
	lastStatus := ""

	var err error
	var resp *api2.ResultTx
	attempts := 0
	for {
		attempts += 1
		if attempts == 3 {
			return err
		}

		time.Sleep(1 * time.Second)
		resp, err = api.GetTransaction(hash, rpcClient)
		if err != nil {
			s.Stop()
			continue
		}

		if lastStatus == resp.Status {
			continue
		}

		lastStatus = resp.Status
		if resp.Status == types3.TxStatusInMempool {
			s.Suffix = colorfmt.YellowStringf(" In mempool")
		} else if resp.Status == types3.TxStatusInPushpool {
			s.Suffix = colorfmt.YellowStringf(" In pushpool")
		} else {
			s.FinalMSG = colorfmt.GreenString("   Confirmed!\n")
			s.Stop()
			break
		}
	}
	return nil
}

// GetRPCClient returns an RPC client. If target repo is provided,
// the RPC server information will be extracted from one of the remote URLs.
// The target remote is set via viper's "remote.name" or "--remote" root flag.
func GetRPCClient(cmd *cobra.Command, targetRepo remotetypes.LocalRepo) (*client.RPCClient, error) {
	remoteName := viper.GetString("remote.name")
	rpcAddress := viper.GetString("remote.address")
	rpcUser := viper.GetString("rpc.user")
	rpcPassword := viper.GetString("rpc.password")
	rpcSecured := viper.GetBool("rpc.https")

	var err error
	var host, port string

	// If a target repo is provided and --remote.address flag is unset,
	// get the rpc address from the specified repo remote.
	if targetRepo != nil && !cmd.Flags().Changed("remote.address") {
		h, p, ok := GetRemoteAddrFromRepo(targetRepo, remoteName)
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

// GetRemoteAddrFromRepo gets remote address from the given repo.
// It will return false if no (good) url was found.
func GetRemoteAddrFromRepo(repo remotetypes.LocalRepo, remoteName string) (string, int, bool) {
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

// GetRepoAndClient opens a the repository on the current working directory
// and returns an RPC client.
func GetRepoAndClient(cmd *cobra.Command, cfg *config.AppConfig, repoDir string) (remotetypes.LocalRepo, types2.Client) {

	var err error
	var targetRepo remotetypes.LocalRepo

	if repoDir == "" {
		targetRepo, err = rr.GetAtWorkingDir(cfg.Node.GitBinPath)
	} else {
		targetRepo, err = rr.GetWithGitModule(cfg.Node.GitBinPath, repoDir)
	}

	rpcClient, err := GetRPCClient(cmd, targetRepo)
	if err != nil {
		log.Fatal(err.Error())
	}

	return targetRepo, rpcClient
}
