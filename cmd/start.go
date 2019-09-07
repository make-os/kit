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
	"github.com/makeos/mosdef/node"
	"github.com/makeos/mosdef/util/logger"
	"github.com/spf13/cobra"
)

var log logger.Logger

func start() error {

	n := node.NewNode(cfg)
	if err := n.OpenDB(); err != nil {
		log.Error("Failed to open database: %s", err)
		return err
	}

	return nil
}

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Launch the node to join the network.",
	Long:  `Launch the node to join the network.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		log = cfg.G().Log.Module("main")
		return start()
	},
}
