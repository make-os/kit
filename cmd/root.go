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
	"encoding/pem"
	"fmt"
	"os"
	"strings"

	"github.com/thoas/go-funk"
	"gitlab.com/makeos/mosdef/pkgs/logger"
	cmd2 "gitlab.com/makeos/mosdef/remote/cmd"
	"gitlab.com/makeos/mosdef/remote/cmd/gitcmd"
	"gitlab.com/makeos/mosdef/remote/repo"
	"gitlab.com/makeos/mosdef/util"

	tmcfg "github.com/tendermint/tendermint/config"
	"gitlab.com/makeos/mosdef/config"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// BuildVersion is the build version set by goreleaser
	BuildVersion = ""

	// BuildCommit is the git hash of the build. It is set by goreleaser
	BuildCommit = ""

	// BuildDate is the date the build was created. Its is set by goreleaser
	BuildDate = ""

	// GoVersion is the version of go used to build the client
	GoVersion = "go1.13"
)

var (
	log logger.Logger

	// cfg is the application config
	cfg = &config.AppConfig{}

	// Get a reference to tendermint's config object
	tmconfig = tmcfg.DefaultConfig()

	// itr is used to inform the stoppage of all modules
	itr = util.Interrupt(make(chan struct{}))
)

// Execute the root command or fallback command when command is unknown.
func Execute() {

	// When command is unknown, run the root command PersistentPreRun
	// then run the fallback command
	_, _, err := rootCmd.Find(os.Args[1:])
	if err != nil && strings.Index(err.Error(), "unknown command") != -1 {
		rootCmd.PersistentPreRun(fallbackCmd, os.Args)
		fallbackCmd.Run(&cobra.Command{}, os.Args)
		return
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "mosdef",
	Short: "The decentralized software development and collaboration network",
	Long: `Mosdef is the official client for MakeOS network - A decentralized software
development network that allows anyone, anywhere to create software products
and organizations without a centralized authority.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		config.Configure(cfg, tmconfig, &itr)
		log = cfg.G().Log

		if cmd.CalledAs() != "init" {
			cfg.LoadKeys(tmconfig.NodeKeyFile(), tmconfig.PrivValidatorKeyFile(),
				tmconfig.PrivValidatorStateFile())
		}

		// Set version information
		cfg.VersionInfo = &config.VersionInfo{}
		cfg.VersionInfo.BuildCommit = BuildCommit
		cfg.VersionInfo.BuildDate = BuildDate
		cfg.VersionInfo.GoVersion = GoVersion
		cfg.VersionInfo.BuildVersion = BuildVersion
	},
}

// isGitSignRequest checks whether the program arguments
// indicate a request from git to sign a message
func isGitSignRequest(args []string) bool {
	return len(args) == 4 && args[1] == "--status-fd=2" && args[2] == "-bsau"
}

// isGitVerifyRequest checks whether the program arguments
// indicate a request from git to verify a signature
func isGitVerifyRequest(args []string) bool {
	return len(args) == 6 && funk.ContainsString(args, "--verify")
}

// fallbackCmd is called any time an unknown command is executed
var fallbackCmd = &cobra.Command{
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {

		if isGitSignRequest(args) {
			if err := gitcmd.GitSignCmd(cfg, os.Stdin, &gitcmd.GitSignArgs{
				Args:            os.Args,
				RepoGetter:      repo.Get,
				PushKeyUnlocker: cmd2.UnlockPushKey,
				StdErr:          os.Stderr,
				StdOut:          os.Stdout,
			}); err != nil {
				log.Fatal(err.Error())
			}
			os.Exit(0)
		}

		if isGitVerifyRequest(args) {
			if err := gitcmd.GitVerifyCmd(cfg, &gitcmd.GitVerifyArgs{
				Args:            args,
				RepoGetter:      repo.Get,
				PushKeyUnlocker: cmd2.UnlockPushKey,
				PemDecoder:      pem.Decode,
				StdOut:          os.Stdout,
				StdErr:          os.Stderr,
				StdIn:           os.Stdin,
			}); err != nil {
				log.Fatal(err.Error())
			}
			os.Exit(0)
		}

		fmt.Print("Unknown command. Use --help to see commands.\n")
		os.Exit(1)
	},
}

func init() {
	rootCmd.AddCommand(fallbackCmd)

	// Register flags
	rootCmd.PersistentFlags().Bool("dev", false, "Enables development mode")
	rootCmd.PersistentFlags().String("home", config.DefaultDataDir, "Set the path to the home directory")
	rootCmd.PersistentFlags().String("home.prefix", "", "Adds a prefix to the home directory in dev mode")
	rootCmd.PersistentFlags().Uint64("net", config.DefaultNetVersion, "Set network/chain ID")
	rootCmd.PersistentFlags().Bool("nolog", false, "Disables loggers")
	rootCmd.PersistentFlags().String("gitbin", "/usr/bin/git", "GetPath to git executable")

	// Hidden flags relevant to git gpg interface conformance
	rootCmd.PersistentFlags().String("keyid-format", "", "")
	rootCmd.PersistentFlags().MarkHidden("keyid-format")
	rootCmd.PersistentFlags().String("status-fd", "", "")
	rootCmd.PersistentFlags().MarkHidden("status-fd")
	rootCmd.PersistentFlags().Bool("verify", false, "")
	rootCmd.PersistentFlags().MarkHidden("verify")

	// Viper bindings
	viper.BindPFlag("node.gitbin", rootCmd.PersistentFlags().Lookup("gitbin"))
	viper.BindPFlag("net.version", rootCmd.PersistentFlags().Lookup("net"))
	viper.BindPFlag("dev", rootCmd.PersistentFlags().Lookup("dev"))
	viper.BindPFlag("home", rootCmd.PersistentFlags().Lookup("home"))
	viper.BindPFlag("home.prefix", rootCmd.PersistentFlags().Lookup("home.prefix"))
	viper.BindPFlag("nolog", rootCmd.PersistentFlags().Lookup("nolog"))
}
