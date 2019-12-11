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

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/node"
	"github.com/makeos/mosdef/rpc"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func start(onStart func(n *node.Node)) {

	log.Info("Starting node...", "NodeID", cfg.G().NodeKey.ID(), "DevMode", cfg.IsDev())

	// Create the node and open the database
	n := node.NewNode(cfg, tmconfig)
	if err := n.OpenDB(); err != nil {
		log.Fatal("Failed to open database", "Err", err)
	}

	log.Info("App database has been loaded", "AppDBDir", cfg.GetAppDBDir())

	// Start the node
	if err := n.Start(); err != nil {
		log.Fatal("Failed to prepare node", "Err", err)
	}

	// Start the RPC server
	rpcAddr := viper.GetString("rpc.address")
	rpcServer := rpc.NewServer(rpcAddr, cfg, log.Module("rpc-sever"), &itr)
	go rpcServer.Serve()

	// Once all processes have been started call the onStart callback
	// so the caller can perform other operations that rely on the already
	// started processes.
	if onStart != nil {
		onStart(n)
	}

	itr.Wait()
	rpcServer.Stop()
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
		cmd.Flags().String("node.address", config.DefaultNodeAddress,
			"Set the node's p2p listening address")
		cmd.Flags().String("rpc.address", config.DefaultRPCAddress,
			"Set the RPC listening address")
		cmd.Flags().String("rpc.tmaddress", config.DefaultTMRPCAddress,
			"Set tendermint RPC listening address")
		cmd.Flags().String("dht.address", config.DefaultDHTAddress,
			"Set the DHT listening address")
		cmd.Flags().String("repoman.address", config.DefaultRepoManagerAddress,
			"Set the repository manager listening address")
		cmd.Flags().String("node.addpeer", "",
			"Connect to one or more persistent node")
		cmd.Flags().StringSlice("node.exts", []string{}, "Specify an extension to run on startup")

		viper.BindPFlag("rpc.address", cmd.Flags().Lookup("rpc.address"))
		viper.BindPFlag("node.address", cmd.Flags().Lookup("node.address"))
		viper.BindPFlag("dht.address", cmd.Flags().Lookup("dht.address"))
		viper.BindPFlag("rpc.tmaddress", cmd.Flags().Lookup("rpc.tmaddress"))
		viper.BindPFlag("repoman.address", cmd.Flags().Lookup("repoman.address"))
		viper.BindPFlag("node.addpeer", cmd.Flags().Lookup("node.addpeer"))
		viper.BindPFlag("node.exts", cmd.Flags().Lookup("node.exts"))
	}
}
