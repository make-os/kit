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
	"github.com/spf13/viper"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/console"
)

// attachCmd represents the attach command
var attachCmd = &cobra.Command{
	Use:   "attach",
	Short: "Start a Javascript console attached to a node",
	Run: func(cmd *cobra.Command, args []string) {
		viper.Set("attachmode", true)
		execCode, _ := cmd.Flags().GetString("exec")

		console := console.New(cfg)
		console.OnStop(func() {
			itr.Close()
		})

		// Run the console
		go func() {
			if err := console.Run(execCode); err != nil {
				log.Fatal(err.Error())
			}
		}()

		itr.Wait()
	},
}

func init() {
	rootCmd.AddCommand(attachCmd)

	f := attachCmd.Flags()
	f.String("rpc.address", config.DefaultRPCAddress, "Set the RPC server address to connect to")
	f.Bool("rpc.https", false, "Connect using HTTPS protocol")
	f.String("rpc.user", "", "Set the RPC username")
	f.String("rpc.password", "", "Set the RPC password")
	f.String("exec", "", "Execute the given JavaScript code")
	viperBindFlagSet(attachCmd)
}
