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
	"github.com/spf13/cobra"
	"gitlab.com/makeos/mosdef/console"
	"gitlab.com/makeos/mosdef/node"
)

// consoleCmd represents the console command
var consoleCmd = &cobra.Command{
	Use:   "console",
	Short: "Start a Javascript console mode and start the node",
	Run: func(cmd *cobra.Command, args []string) {

		// Start the node and also start the console after the node has started
		start(func(n *node.Node) {
			console := console.New(cfg)

			// On stop, close the node and interrupt other processes
			console.OnStop(func() {
				itr.Close()
			})

			// Register JS module hub
			console.SetModulesHub(n.GetModulesHub())

			// Run the console
			go func() {
				if err := console.Run(); err != nil {
					log.Fatal(err.Error())
				}
			}()
		})
	},
}

func init() {
	rootCmd.AddCommand(consoleCmd)
	setStartFlags(consoleCmd)
}
