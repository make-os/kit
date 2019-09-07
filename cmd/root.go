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

	// rootCmd is the application root command
	rootCmd *cobra.Command

	// tmRootCmd is tendermint's root command.
	// We pass this to tendermint CLI configurer
	tmRootCmd *cobra.Command
)

func makeRootCmd(name string) *cobra.Command {
	// rootCmd represents the base command when called without any subcommands
	rootCmd := &cobra.Command{
		Use:   name,
		Short: "The decentralized software development and collaboration network",
		Long: `Mosdef is the official client for MakeOS network - A decentralized software
	development network that allows anyone, anywhere to create software products
	and organizations without a centralized authority.`,
		Run: func(cmd *cobra.Command, args []string) {},
	}

	rootCmd.PersistentFlags().Bool("dev", false, "Enables development mode")

	return rootCmd
}

// initRootCmd initializes the app
func initRootCmd() {
	rootCmd = makeRootCmd("mosdef")
}

// initTMRootCmd initializes tendermint
func initTMRootCmd() {
	tmRootCmd = makeRootCmd("mosdef")
	tmRootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		commands.InitFilesCmd.PersistentFlags().AddFlagSet(rootCmd.PersistentFlags())
		return commands.RootCmd.PersistentPreRunE(cmd, args)
	}
}

// Initialize setups the environment and returns a Config object
func Initialize() {

	// Initialize root commands
	initRootCmd()
	initTMRootCmd()

	// Add sub commands
	tmRootCmd.AddCommand(initCmd)
	tmRootCmd.AddCommand(startCmd)

	// Configure the root commands
	config.Configure(rootCmd, tmRootCmd, cfg, tmconfig)
}
