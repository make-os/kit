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
	"github.com/makeos/mosdef/repo"

	"github.com/spf13/cobra"
)

// commitCmd represents the commit command
var commitCmd = &cobra.Command{
	Use:   "txline",
	Short: "Set transaction arguments to the recent commit message",
	Long:  `Set transaction arguments to the recent commit message`,
	Run: func(cmd *cobra.Command, args []string) {
		fee, _ := cmd.Flags().GetString("fee")
		nonce, _ := cmd.Flags().GetString("nonce")
		sk, _ := cmd.Flags().GetString("signingKey")

		if err := repo.AmendRecentTxLine(cfg.Node.GitBinPath, fee, nonce, sk); err != nil {
			cfg.G().Log.Fatal(err.Error())
		}
	},
}

func initCommit() {
	rootCmd.AddCommand(commitCmd)
	commitCmd.Flags().StringP("fee", "f", "", "Set the push transaction fee")
	commitCmd.MarkFlagRequired("fee")
	commitCmd.Flags().String("nonce", "", "Set the push transaction nonce")
	commitCmd.Flags().StringP("signingKey", "s", "", "Set the GPG signing key ID")
}
