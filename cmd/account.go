// Copyright © 2018 NAME HERE <EMAIL ADDRESS>
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
	"fmt"
	path "path/filepath"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gitlab.com/makeos/mosdef/account"
	"gitlab.com/makeos/mosdef/config"
)

// accountCmd represents the account command
var accountCmd = &cobra.Command{
	Use:   "account command [flags]",
	Short: "Create and manage your accounts.",
	Long: `Description:
This command provides the ability to create an account, list, import and update 
accounts. Accounts are stored in an encrypted format using a passphrase provided 
by you. Please understand that if you forget the password, it is IMPOSSIBLE to 
unlock your account.

Password will be stored under <DATADIR>/` + config.AccountDirName + `. It is safe to transfer the 
directory or individual accounts to another node. 

Always backup your keeps regularly.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

// accountCreateCmd represents the account command
var accountCreateCmd = &cobra.Command{
	Use:   "create [flags]",
	Short: "Create an account.",
	Long: `Description:
This command creates an account and encrypts it using a passphrase
you provide. Do not forget your passphrase, you will not be able 
to unlock your account if you do.

Password will be stored under <DATADIR>/` + config.AccountDirName + `. 
It is safe to transfer the directory or individual accounts to another node. 

Use --pass to directly specify a password without going interactive mode. You 
can also provide a path to a file containing a password. If a path is provided,
password is fetched with leading and trailing newline character removed. 

Always backup your keeps regularly.`,
	Run: func(cmd *cobra.Command, args []string) {

		_ = viper.BindPFlag("node.password", cmd.Flags().Lookup("pass"))
		_ = viper.BindPFlag("node.seed", cmd.Flags().Lookup("seed"))
		seed := viper.GetInt64("node.seed")
		pass := viper.GetString("node.password")

		am := account.New(path.Join(cfg.DataDir(), config.AccountDirName))
		key, err := am.CreateCmd(false, seed, pass)
		if err != nil {
			log.Fatal(err.Error())
		}

		fmt.Println("New account created, encrypted and stored.")
		fmt.Println("Address:", color.CyanString(key.Addr().String()))
	},
}

var accountListCmd = &cobra.Command{
	Use:   "list [flags]",
	Short: "List all accounts.",
	Long: `Description:
This command lists all accounts existing under <DATADIR>/` + config.AccountDirName + `.

Given that an account in the directory begins with a timestamp of its creation time and the 
list is lexicographically sorted such that the oldest account will be at the top on the list
`,
	Run: func(cmd *cobra.Command, args []string) {
		am := account.New(path.Join(cfg.DataDir(), config.AccountDirName))
		if err := am.ListCmd(); err != nil {
			log.Fatal(err.Error())
		}
	},
}

var accountUpdateCmd = &cobra.Command{
	Use:   "update [flags] <address>",
	Short: "Update an account.",
	Long: `Description:
This command allows you to update the password of an account and to
convert an account encrypted in an old format to a new one.
`,
	Run: func(cmd *cobra.Command, args []string) {

		var address string
		if len(args) >= 1 {
			address = args[0]
		}

		_ = viper.BindPFlag("node.password", cmd.Flags().Lookup("pass"))
		pass := viper.GetString("node.password")

		am := account.New(path.Join(cfg.DataDir(), config.AccountDirName))
		if err := am.UpdateCmd(address, pass); err != nil {
			log.Fatal(err.Error())
		}
	},
}

var accountImportCmd = &cobra.Command{
	Use:   "import [flags] <keyfile>",
	Short: "Import an existing, unencrypted private key.",
	Long: `Description:
This command allows you to import a private key from a <keyfile> and create
a new account. You will be prompted to provide your password. Your account is saved 
in an encrypted format.

The keyfile is expected to contain an unencrypted private key in Base58 format.

You can skip the interactive mode by providing your password via the '--pass' flag. 
Also, a path to a file containing a password can be provided to the flag.

You must not forget your password, otherwise you will not be able to unlock your
account.
`,
	Run: func(cmd *cobra.Command, args []string) {

		var keyFile string
		if len(args) >= 1 {
			keyFile = args[0]
		}

		_ = viper.BindPFlag("node.password", cmd.Flags().Lookup("pass"))
		pass := viper.GetString("node.password")

		am := account.New(path.Join(cfg.DataDir(), config.AccountDirName))
		if err := am.ImportCmd(keyFile, pass); err != nil {
			log.Fatal(err.Error())
		}
	},
}

var accountRevealCmd = &cobra.Command{
	Use:   "reveal [flags] <address>",
	Short: "Reveal the private key of an account.",
	Long: `Description:
This command reveals the private key of an account. You will be prompted to 
provide your password. 
	
You can skip the interactive mode by providing your password via the '--pass' flag. 
Also, the flag accepts a path to a file containing a password.
`,
	Run: func(cmd *cobra.Command, args []string) {

		var address string
		if len(args) >= 1 {
			address = args[0]
		}

		_ = viper.BindPFlag("node.password", cmd.Flags().Lookup("pass"))
		pass := viper.GetString("node.password")

		am := account.New(path.Join(cfg.DataDir(), config.AccountDirName))
		if err := am.RevealCmd(address, pass); err != nil {
			log.Fatal(err.Error())
		}
	},
}

func setAccountCmdAndFlags() {
	accountCmd.AddCommand(accountCreateCmd)
	accountCmd.AddCommand(accountListCmd)
	accountCmd.AddCommand(accountUpdateCmd)
	accountCmd.AddCommand(accountImportCmd)
	accountCmd.AddCommand(accountRevealCmd)
	accountCreateCmd.Flags().Int64P("seed", "s", 0, "Provide a strong seed (not recommended)")
	accountCreateCmd.Flags().String("pass", "", "Password to unlock signer account and skip interactive mode")
	accountImportCmd.Flags().String("pass", "", "Password to unlock the target account and skip interactive mode")
	accountUpdateCmd.Flags().String("pass", "", "Password to unlock the target account and skip interactive mode")
	accountRevealCmd.Flags().String("pass", "", "Password to unlock the target account and skip interactive mode")
	rootCmd.AddCommand(accountCmd)
}
