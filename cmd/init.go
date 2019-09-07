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
	"github.com/tendermint/tendermint/cmd/tendermint/commands"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the application.",
	Long: `This command initializes the applications data directory
and creates default config and keys required to successfully 
launch the node.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := commands.InitFilesCmd.Execute(); err != nil {
			golog.Fatalf("Failed to initialize data directory: %s, exiting...", err)
		}
	},
}
