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

	"github.com/makeos/mosdef/node"
	"github.com/makeos/mosdef/util/logger"
	"github.com/spf13/cobra"
)

var log logger.Logger

func start(onStart func(n *node.Node)) {

	log.Info("Starting node...", "NodeID", cfg.G().NodeKey.ID(), "DevMode", cfg.IsDev())

	n := node.NewNode(cfg, tmconfig)
	if err := n.OpenDB(); err != nil {
		log.Fatal("Failed to open database", "Err", err)
	}

	log.Info("Database has been loaded", "DatabaseDir", cfg.GetDBDir())

	if err := n.Start(); err != nil {
		log.Fatal("Failed to prepare node", "Err", err)
	}

	if onStart != nil {
		onStart(n)
	}

	<-interrupt

	n.Stop()
}

func listenForInterrupt() {
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		close(interrupt)
	}()
}

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Launch the node to join the network.",
	Long:  `Launch the node to join the network.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Get and cache node key
		cfg.PrepareNodeKey(tmconfig.NodeKeyFile())
		log = cfg.G().Log.Module("main")
		listenForInterrupt()
		start(nil)
	},
}
