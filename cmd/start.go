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
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/node"
)

func start(onStart func(n *node.Node)) {

	// Create the node
	n := node.NewNode(cfg, tmconfig)

	// Start the node
	if err := n.Start(); err != nil {
		log.Fatal("Failed to prepare node", "Err", err)
	}

	// Once all processes have been started call the onStart callback
	// so the caller can perform other operations that rely on the already
	// started processes.
	if onStart != nil {
		onStart(n)
	}

	itr.Wait()
	n.Stop()
}

func listenForInterrupt() {
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		itr.Close()
	}()
}

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Launch the node to join the network.",
	Long:  `Launch the node to join the network.`,
	Run: func(cmd *cobra.Command, args []string) {
		log = cfg.G().Log.Module("main")
		listenForInterrupt()
		start(nil)
	},
}

func setStartFlags(cmds ...*cobra.Command) {
	for _, cmd := range cmds {
		f := cmd.Flags()
		f.String("node.address", config.DefaultNodeAddress, "Set the node's p2p listening address")
		f.Bool("rpc.on", false, "Start the RPC service")
		f.String("rpc.user", "", "Set the RPC username")
		f.String("rpc.password", "", "Set the RPC password")
		f.Bool("rpc.disableauth", false, "Disable RPC authentication")
		f.Bool("rpc.authpubmethod", false, "Enable RPC authentication for non-private methods")
		f.Bool("node.validator", false, "Run the node in validator mode")
		f.String("rpc.address", config.DefaultRPCAddress, "Set the RPC listening address")
		f.String("rpc.tmaddress", config.DefaultTMRPCAddress, "Set tendermint RPC listening address")
		f.String("dht.address", config.DefaultDHTAddress, "Set the DHT listening address")
		f.String("repoman.address", config.DefaultRepoManagerAddress, "Set the repository manager listening address")
		f.String("node.addpeer", "", "Connect to one or more persistent node")
		f.Bool("dht.on", true, "Run the DHT service and join the network")
		f.String("dht.addpeer", "", "Register bootstrap peers for joining the DHT network")
		f.StringSlice("node.exts", []string{}, "Specify an extension to run on startup")
		extArgsMap := map[string]string{}
		f.StringToStringVar(&extArgsMap, "node.extsargs", map[string]string{}, "Specify arguments for extensions")

		// Apply only to the active command
		if len(os.Args) > 1 && os.Args[1] == cmd.Name() {
			viper.BindPFlag("rpc.on", cmd.Flags().Lookup("rpc.on"))
			viper.BindPFlag("rpc.address", cmd.Flags().Lookup("rpc.address"))
			viper.BindPFlag("rpc.user", cmd.Flags().Lookup("rpc.user"))
			viper.BindPFlag("rpc.password", cmd.Flags().Lookup("rpc.password"))
			viper.BindPFlag("rpc.disableauth", cmd.Flags().Lookup("rpc.disableauth"))
			viper.BindPFlag("rpc.authpubmethod", cmd.Flags().Lookup("rpc.authpubmethod"))
			viper.BindPFlag("node.address", cmd.Flags().Lookup("node.address"))
			viper.BindPFlag("dht.address", cmd.Flags().Lookup("dht.address"))
			viper.BindPFlag("rpc.tmaddress", cmd.Flags().Lookup("rpc.tmaddress"))
			viper.BindPFlag("repoman.address", cmd.Flags().Lookup("repoman.address"))
			viper.BindPFlag("node.addpeer", cmd.Flags().Lookup("node.addpeer"))
			viper.BindPFlag("node.exts", cmd.Flags().Lookup("node.exts"))
			viper.BindPFlag("node.validator", cmd.Flags().Lookup("node.validator"))
			viper.BindPFlag("dht.on", cmd.Flags().Lookup("dht.on"))
			viper.BindPFlag("dht.addpeer", cmd.Flags().Lookup("dht.addpeer"))
			viper.Set("node.extsargs", &extArgsMap)
		}
	}

}

func init() {
	rootCmd.AddCommand(startCmd)
	setStartFlags(startCmd)
}
