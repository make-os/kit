package passcmd

import (
	"io"
	"os"
	"time"

	"github.com/make-os/kit/cmd/common"
	"github.com/make-os/kit/cmd/passcmd/agent"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/keystore"
	"github.com/make-os/kit/util"
	"github.com/pkg/errors"
)

// PassArgs contains parameters for PassCmd
type PassArgs struct {

	// Args is the program argument
	Args []string

	// RepoName is the name of the repository
	RepoName string

	// Key is the unique name for identifying a passphrase
	Key string

	// CacheDuration is the duration to keep a passphrase in memory.
	// If set and agent is not running, the agent is started.
	// If unset, the key will not be cached. Instead, the
	// key will be set on <APPNAME>_PASS env variable.
	CacheDuration string

	// Port determines the port where the agent will or is listen on
	Port string

	// StartAgent indicates that the agent should be started
	StartAgent bool

	// StopAgent indicates that the agent should be stopped
	StopAgent bool

	// CommandCreator creates a wrapped exec.Cmd object
	CommandCreator util.CommandCreator

	// AskPass is a function for reading a passphrase from stdin
	AskPass keystore.AskPassOnceFunc

	// AgentStarter is a function that starts the pass agent service
	AgentStarter agent.RunFunc

	// AgentStop is a function that stops the pass agent service
	AgentStop agent.StopFunc

	// AgentUp is a function that checks if the agent server is running
	AgentUp agent.IsUpFunc

	// AgentSet is a function for sending set request to the agent service
	AgentSet agent.SetFunc

	Stdout io.Writer
	Stderr io.Writer
	Stdin  io.Reader
}

// AskPass prompts for passphrase
func AskPass(prompt ...string) (string, error) {
	ks := keystore.New("")
	return ks.AskForPasswordOnce("")
}

// PassCmd stores a given passphrase to <APP_NAME>_PASS or registers
// it with the passphrase agent.
func PassCmd(args *PassArgs) (err error) {

	if args.AgentStarter == nil {
		args.AgentStarter = agent.Run
	}

	if args.AgentStop == nil {
		args.AgentStop = agent.Stop
	}

	if args.AgentUp == nil {
		args.AgentUp = agent.IsUp
	}

	if args.AgentSet == nil {
		args.AgentSet = agent.Set
	}

	// Use repo name as the default key
	if args.Key == "" {
		args.Key = args.RepoName
	}

	// If --start-agent is set, run the agent only
	if args.StartAgent {
		return args.AgentStarter(args.Port)
	}

	// If --stop-agent is set, send stop request to the agent
	if args.StopAgent {
		return args.AgentStop(args.Port)
	}

	// If caching is requested, run cache agent
	var cacheDur time.Duration
	if args.CacheDuration != "" {
		cacheDur, err = time.ParseDuration(args.CacheDuration)
		if err != nil {
			return errors.Wrap(err, "bad duration")
		}

		if !args.AgentUp(args.Port) {
			cmd := args.CommandCreator(config.AppName, "pass", "--start-agent", "--port", args.Port)
			cmd.SetStdout(args.Stdout)
			cmd.SetStderr(args.Stderr)
			if err := cmd.Start(); err != nil {
				return errors.Wrap(err, "failed to start agent")
			}
		}
	}

	// Request for passphrase
	passphrase, err := args.AskPass("")
	if err != nil {
		return errors.Wrap(err, "failed to ask for passphrase")
	}

	// Store the passphrase to on an env variable where the key unlocker will find it
	_ = os.Setenv(common.MakePassEnvVar(config.AppName), passphrase)

	// If cache is required, send set request to agent
	if args.CacheDuration != "" {
		if err := args.AgentSet(args.Port, args.Key, passphrase, int(cacheDur.Seconds())); err != nil {
			return errors.Wrap(err, "failed to send set request")
		}
	}

	// Return immediately if no additional subcommand to execute.
	if len(args.Args) == 0 {
		return
	}

	// Execute the subcommand
	var cmdArgs []string
	if len(args.Args) > 1 {
		cmdArgs = args.Args[1:]
	}
	cmd := args.CommandCreator(args.Args[0], cmdArgs...)
	cmd.SetStdout(args.Stdout)
	cmd.SetStderr(args.Stderr)
	cmd.SetStdin(args.Stdin)

	if err := cmd.Start(); err != nil {
		return errors.Wrapf(err, "failed to run command %v", args.Args)
	}

	return cmd.Wait()
}
