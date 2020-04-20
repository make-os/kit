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

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tendermint/tendermint/cmd/tendermint/commands"
	tmcfg "github.com/tendermint/tendermint/config"
	tmtypes "github.com/tendermint/tendermint/types"
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

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the application.",
	Long: `This command initializes the applications data directory
and creates default config and keys required to successfully 
launch the node.`,
	Run: func(cmd *cobra.Command, args []string) {
		initializeTendermint()
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
