package passcmd

import (
	"os"
	"os/exec"

	"github.com/make-os/kit/config"
	rr "github.com/make-os/kit/remote/repo"
	"github.com/make-os/kit/util"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	cfg = config.GetConfig()
	log = cfg.G().Log
)

// PassAgentCmd represents the PassAgentCmd command
var PassAgentCmd = &cobra.Command{
	Use:   "pass",
	Short: "Ask and cache a passphrase in memory",
	Run: func(cmd *cobra.Command, args []string) {

		var repoName string
		targetRepo, err := rr.GetAtWorkingDir(cfg.Node.GitBinPath)
		if err == nil {
			repoName = targetRepo.GetName()
		}

		// Parse the arguments
		pf := pflag.NewFlagSet("", pflag.ContinueOnError)
		pf.StringP("key", "k", "", "The index or address of the key")
		pf.StringP("cache", "c", "", "Cache the key for the specified duration (e.g 10s, 2m, 24h)")
		pf.Bool("start-agent", false, "Start the passphrase agent service")
		pf.Bool("stop-agent", false, "Stop the passphrase agent service")
		pf.StringP("port", "p", config.DefaultPassAgentPort, "Set the cache agent listening port")
		pf.ParseErrorsWhitelist.UnknownFlags = true
		if err = pf.Parse(args); err != nil {
			log.Fatal(err.Error())
		}

		// Remove known flags
		args = util.RemoveFlag(args, "key", "k", "cache", "c", "port", "p")

		key, _ := pf.GetString("key")
		cacheDurStr, _ := pf.GetString("cache")
		port, _ := pf.GetString("port")
		startAgent, _ := pf.GetBool("start-agent")
		stopAgent, _ := pf.GetBool("stop-agent")

		if err := PassCmd(&PassArgs{
			Args:           args,
			RepoName:       repoName,
			Key:            key,
			CacheDuration:  cacheDurStr,
			Port:           port,
			StartAgent:     startAgent,
			StopAgent:      stopAgent,
			CommandCreator: util.NewCommand,
			AskPass:        AskPass,
			Stdout:         os.Stdout,
			Stderr:         os.Stderr,
			Stdin:          os.Stdin,
		}); err != nil {
			if e, ok := err.(*exec.ExitError); ok {
				os.Exit(e.ExitCode())
			}
			log.Fatal(err.Error())
		}
	},
}

func init() {
	PassAgentCmd.DisableFlagParsing = true
	PassAgentCmd.Flags().StringP("key", "k", "", "The index or address of the key")
	PassAgentCmd.Flags().StringP("cache", "c", "", "Cache the key for the specified duration (e.g 10s, 2m, 24h)")
	PassAgentCmd.Flags().Bool("start-agent", false, "Start the passphrase agent service")
	PassAgentCmd.Flags().Bool("stop-agent", false, "Stop the passphrase agent service")
	PassAgentCmd.Flags().String("port", config.DefaultPassAgentPort, "Set the cache agent listening port")
}
