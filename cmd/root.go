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
	"os"

	"github.com/makeos/mosdef/config"
	"github.com/tendermint/tendermint/cmd/tendermint/commands"
	tmcfg "github.com/tendermint/tendermint/config"

	"github.com/spf13/cobra"
)

var (
	// cfg is the application config
	cfg = &config.EngineConfig{}

	// Get a reference to tendermint's config object
	tmconfig = tmcfg.DefaultConfig()

	// rootCmd is the root command
	rootCmd *cobra.Command

	// interrupt is used to inform the stoppage of all modules
	interrupt = make(chan struct{})
)

// initializeTendermint initializes tendermint
func initializeTendermint() error {
	commands.SetConfig(tmconfig)
	commands.InitFilesCmd.RunE(nil, nil)
	return nil
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
			config.Configure(rootCmd, cfg, tmconfig)
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

	// Add flags
	rootCmd.PersistentFlags().Bool("dev", false, "Enables development mode")
	rootCmd.PersistentFlags().String("home", config.DefaultDataDir, "Enables development mode")
	setStartFlags(startCmd, consoleCmd)
	setAccountCmdAndFlags()
}
