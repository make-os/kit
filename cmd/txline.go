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

// txlineCmd represents the commit command
var txlineCmd = &cobra.Command{
	Use:   "txline",
	Short: "Add transaction information to git objects",
	Long:  `Add transaction information to git objects`,
	Run: func(cmd *cobra.Command, args []string) {

	},
}

var txlineCommitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Amend the recent commit message to include transaction information",
	Long:  `Amend the recent commit message to include transaction information`,
	Run: func(cmd *cobra.Command, args []string) {
		fee, _ := cmd.Flags().GetString("fee")
		nonce, _ := cmd.Flags().GetString("nonce")
		sk, _ := cmd.Flags().GetString("signingKey")

		if err := repo.AmendRecentCommitTxLine(cfg.Node.GitBinPath, fee, nonce, sk); err != nil {
			cfg.G().Log.Fatal(err.Error())
		}
	},
}

var txlineTagCmd = &cobra.Command{
	Use:   "tag",
	Short: "Create an annotated tag with transaction information",
	Long:  `Create an annotated tags with transaction information`,
	Run: func(cmd *cobra.Command, args []string) {
		fee, _ := cmd.Flags().GetString("fee")
		nonce, _ := cmd.Flags().GetString("nonce")
		sk, _ := cmd.Flags().GetString("signingKey")

		args = cmd.Flags().Args()
		if err := repo.CreateTagWithTxLine(args, cfg.Node.GitBinPath, fee, nonce, sk); err != nil {
			cfg.G().Log.Fatal(err.Error())
		}
	},
}

var txlineSignNoteCmd = &cobra.Command{
	Use:   "note",
	Short: "Sign and add transaction information to a note",
	Long:  `Sign and add transaction information to a note`,
	Run: func(cmd *cobra.Command, args []string) {
		fee, _ := cmd.Flags().GetString("fee")
		nonce, _ := cmd.Flags().GetString("nonce")
		sk, _ := cmd.Flags().GetString("signingKey")

		if len(args) == 0 {
			log.Fatal("tag name is required")
		}

		if err := repo.AddSignedTxBlob(cfg.Node.GitBinPath, fee, nonce, sk, args[0]); err != nil {
			log.Fatal(err.Error())
		}
	},
}

func initCommit() {
	rootCmd.AddCommand(txlineCmd)
	txlineCmd.AddCommand(txlineTagCmd)
	txlineCmd.AddCommand(txlineCommitCmd)
	txlineCmd.AddCommand(txlineSignNoteCmd)

	txlineCmd.PersistentFlags().StringP("fee", "f", "0", "Set the transaction fee")
	txlineCmd.PersistentFlags().StringP("nonce", "n", "0", "Set the transaction nonce")
	txlineCmd.PersistentFlags().StringP("signingKey", "s", "", "Set the GPG signing key ID")
}
