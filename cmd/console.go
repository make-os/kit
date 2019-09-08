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
	"github.com/makeos/mosdef/console"
	"github.com/makeos/mosdef/node"
	"github.com/spf13/cobra"
)

// consoleCmd represents the console command
var consoleCmd = &cobra.Command{
	Use:   "console",
	Short: "Start an interactive javascript console mode and start the node",
	Long:  `Start an interactive javascript console mode and start the node`,
	Run: func(cmd *cobra.Command, args []string) {
		log = cfg.G().Log.Module("main")

		// Get and cache node key
		cfg.PrepareNodeKey(tmconfig.NodeKeyFile())

		// Start the node and also start the console
		// after the node has started
		start(func(n *node.Node) {
			console := console.New(cfg.GetConsoleHistoryPath(), cfg, log)
			console.OnStop(func() {
				n.Stop()
				close(interrupt)
			})

			// Run the console
			go func() {
				if err := console.Run(); err != nil {
					log.Fatal(err.Error())
				}
			}()
		})
	},
}
