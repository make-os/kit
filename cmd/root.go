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
	golog "log"
	"os"

	"gitlab.com/makeos/mosdef/pkgs/logger"
	"gitlab.com/makeos/mosdef/util"

	tmtypes "github.com/tendermint/tendermint/types"

	"github.com/tendermint/tendermint/cmd/tendermint/commands"
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
	GoVersion = "go1.12.4"
)

var (
	log logger.Logger

	// cfg is the application config
	cfg = &config.AppConfig{}

	// Get a reference to tendermint's config object
	tmconfig = tmcfg.DefaultConfig()

	// rootCmd is the root command
	rootCmd *cobra.Command

	// itr is used to inform the stoppage of all modules
	itr = util.Interrupt(make(chan struct{}))
)

// initializeTendermint initializes tendermint
func initializeTendermint() error {
	commands.SetConfig(tmconfig)
	commands.InitFilesCmd.RunE(nil, nil)
	reconfigureTendermint()
	tmcfg.EnsureRoot(tmconfig.RootDir)
	return nil
}

func reconfigureTendermint() {

	// Read the genesis file
	genDoc, err := tmtypes.GenesisDocFromFile(tmconfig.GenesisFile())
	if err != nil {
		golog.Fatalf("Failed to read genesis file: %s", err)
	}

	// Set the chain id
	genDoc.ChainID = viper.GetString("net.version")
	if err = genDoc.SaveAs(tmconfig.GenesisFile()); err != nil {
		golog.Fatalf("Failed set chain id: %s", err)
	}
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {

	// rootCmd represents the base command when called without any subcommands
	rootCmd = &cobra.Command{
		Use:   "mosdef",
		Short: "The decentralized software development and collaboration network",
		Long: `Mosdef is the official client for MakeOS network - A decentralized software
development network that allows anyone, anywhere to create software products
and organizations without a centralized authority.`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {

			config.Configure(rootCmd, cfg, tmconfig, &itr)
			log = cfg.G().Log

			if cmd.CalledAs() != "init" {
				// Get and cache node and validators keys
				cfg.PrepareNodeValKeys(tmconfig.NodeKeyFile(),
					tmconfig.PrivValidatorKeyFile(),
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

	initialize()
}

// initialize setups the environment and returns a Config object
func initialize() {

	// Add commands
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(consoleCmd)
	rootCmd.AddCommand(mergeReqCmd)

	// Add flags
	rootCmd.PersistentFlags().Bool("dev", false, "Enables development mode")
	rootCmd.PersistentFlags().String("home", config.DefaultDataDir, "Set the path to the home directory")
	rootCmd.PersistentFlags().String("home.prefix", "", "Adds a prefix to the home directory in dev mode")
	rootCmd.PersistentFlags().Uint64("net", config.DefaultNetVersion, "Set network/chain ID")
	rootCmd.PersistentFlags().Bool("nolog", false, "Disables loggers")
	rootCmd.PersistentFlags().String("gitbin", "/usr/bin/git", "Path to git executable")
	consoleCmd.Flags().Bool("only", false, "Run only the console (no servers)")

	// Viper bindings
	viper.BindPFlag("node.gitbin", rootCmd.PersistentFlags().Lookup("gitbin"))
	viper.BindPFlag("net.version", rootCmd.PersistentFlags().Lookup("net"))
	viper.BindPFlag("console.only", consoleCmd.Flags().Lookup("only"))

	setStartFlags(startCmd, consoleCmd)
	setAccountCmdAndFlags()
	initSign()
	initMerge()
}
