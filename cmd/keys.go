package cmd

import (
	"fmt"
	"os"
	path "path/filepath"

	"github.com/make-os/kit/config"
	"github.com/make-os/kit/crypto"
	"github.com/make-os/kit/keystore"
	"github.com/make-os/kit/keystore/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// keysCmd represents the parent command for all key related commands
var keysCmd = &cobra.Command{
	Use:   "key command [flags]",
	Short: "Create and manage your account and push keys.",
	Long: `This command provides the ability to create, list, import and update 
keys. Keys are stored in an encrypted format using a passphrase. 
Please understand that if you forget the password, it is IMPOSSIBLE to 
unlock your key. 

During creation, if a passphrase is not provided, the key is still encrypted using
a default (unprotected) passphrase and marked as 'unprotected'. You can change the passphrase 
at any time. (not recommended)

Keys are stored under <DATADIR>/` + config.KeystoreDirName + `. It is safe to transfer the 
directory or individual accounts to another node. 

Always backup your keeps regularly.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

// keyCreateCmd represents key creation command
var keyCreateCmd = &cobra.Command{
	Use:   "create [flags]",
	Short: "Create a key.",
	Long: `This command creates a key and encrypts it using a passphrase
you provide. Do not forget your passphrase, you will not be able 
to unlock your key if you do.

Password will be stored under <DATADIR>/` + config.KeystoreDirName + `. 
It is safe to transfer the directory or individual accounts to another node. 

Use --pass to directly specify a password without going interactive mode. You 
can also provide a path to a file containing a password. If a path is provided,
password is fetched with leading and trailing newline character removed. 

Always backup your keeps regularly.`,
	Run: func(cmd *cobra.Command, args []string) {
		seed, _ := cmd.Flags().GetInt64("seed")
		pass, _ := cmd.Flags().GetString("pass")
		nopass, _ := cmd.Flags().GetBool("nopass")
		pushType, _ := cmd.Flags().GetBool("push")

		ks := keystore.New(path.Join(cfg.DataDir(), config.KeystoreDirName))
		kt := types.KeyTypeUser
		if pushType {
			kt = types.KeyTypePush
		}
		_, err := ks.CreateCmd(kt, seed, pass, nopass)
		if err != nil {
			log.Fatal(err.Error())
		}
	},
}

var keyListCmd = &cobra.Command{
	Use:   "list [flags]",
	Short: "List all accounts.",
	Long: `This command lists all accounts existing under <DATADIR>/` + config.KeystoreDirName + `.

Given that keys in the keystore directory are prefixed with their creation timestamp, the 
list is lexicographically sorted such that the oldest keystore will be at the top on the list
`,
	Run: func(cmd *cobra.Command, args []string) {
		ks := keystore.New(path.Join(cfg.DataDir(), config.KeystoreDirName))
		if err := ks.ListCmd(os.Stdout); err != nil {
			log.Fatal(err.Error())
		}
	},
}

var keyUpdateCmd = &cobra.Command{
	Use:   "update [flags] <address>",
	Short: "Update a key",
	Long: `This command allows you to update the password of a key and to
convert a key encrypted in an old format to a new one.
`,
	Run: func(cmd *cobra.Command, args []string) {

		var address string
		if len(args) >= 1 {
			address = args[0]
		}

		pass, _ := cmd.Flags().GetString("pass")

		ks := keystore.New(path.Join(cfg.DataDir(), config.KeystoreDirName))
		if err := ks.UpdateCmd(address, pass); err != nil {
			log.Fatal(err.Error())
		}
	},
}

var keyImportCmd = &cobra.Command{
	Use:   "import [flags] <keyfile>",
	Short: "Import an existing, unencrypted private key.",
	Long: `This command allows you to create a new key by importing a private key from a <keyfile>. 
You will be prompted to provide your password. Your key is saved in an encrypted format.

The keyfile is expected to contain an unencrypted private key in Base58 format.

You can skip the interactive mode by providing your password via the '--pass' flag. 
Also, a path to a file containing a password can be provided to the flag.

You must not forget your password, otherwise you will not be able to unlock your
key.
`,
	Run: func(cmd *cobra.Command, args []string) {

		var keyFile string
		if len(args) >= 1 {
			keyFile = args[0]
		}

		pass, _ := cmd.Flags().GetString("pass")
		pushType, _ := cmd.Flags().GetBool("push")
		kt := types.KeyTypeUser
		if pushType {
			kt = types.KeyTypePush
		}

		ks := keystore.New(path.Join(cfg.DataDir(), config.KeystoreDirName))
		if err := ks.ImportCmd(keyFile, kt, pass); err != nil {
			log.Fatal(err.Error())
		}
	},
}

var keyGetCmd = &cobra.Command{
	Use:   "get [flags] <address>",
	Short: "Get a key",
	Long: `This command gets a key and prints out its information. You will be prompted to 
provide the passphrase to unlock the key.
	
You can skip the interactive mode by providing your password via the '--pass' flag. 
Also, the flag accepts a path to a file containing a password.
`,
	Run: func(cmd *cobra.Command, args []string) {
		showKey, _ := cmd.Flags().GetBool("show-key")

		var address string
		if len(args) >= 1 {
			address = args[0]
		}

		_ = viper.BindPFlag("node.passphrase", cmd.Flags().Lookup("pass"))
		pass := viper.GetString("node.passphrase")

		ks := keystore.New(path.Join(cfg.DataDir(), config.KeystoreDirName))
		if err := ks.GetCmd(address, pass, showKey); err != nil {
			log.Fatal(err.Error())
		}
	},
}

// keyGenCmd command generates ed25519 keys
var keyGenCmd = &cobra.Command{
	Use:   "gen [flags]",
	Short: "Generate an ed25519 key",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		seed, _ := cmd.Flags().GetInt64("seed")

		var key *crypto.Key
		var err error
		if seed != 0 {
			key, err = crypto.NewKey(&seed)
			if err != nil {
				log.Fatal(err.Error())
			}
		} else {
			key, _ = crypto.NewKey(nil)
		}

		fmt.Println("Private Key:", key.PrivKey().Base58())
		fmt.Println("Public Key: ", key.PubKey().Base58())
		fmt.Println("Account Address: ", key.PubKey().Addr())
		fmt.Println("Push Address: ", key.PubKey().PushAddr())
	},
}

func init() {
	rootCmd.AddCommand(keysCmd)
	keysCmd.AddCommand(keyCreateCmd)
	keysCmd.AddCommand(keyListCmd)
	keysCmd.AddCommand(keyUpdateCmd)
	keysCmd.AddCommand(keyImportCmd)
	keysCmd.AddCommand(keyGetCmd)
	keysCmd.AddCommand(keyGenCmd)
	keysCmd.PersistentFlags().String("pass", "", "Password to unlock the target key and skip interactive mode")
	keyCreateCmd.Flags().Int64P("seed", "s", 0, "Provide a strong seed (not recommended)")
	keyGenCmd.Flags().Int64P("seed", "s", 0, "Provide a strong seed (not recommended)")
	keyCreateCmd.Flags().Bool("nopass", false, "Force key to be created with no passphrase")
	keyCreateCmd.Flags().Bool("push", false, "Mark as Push Key")
	keyImportCmd.Flags().Bool("push", false, "Mark as Push Key")
	keyGetCmd.Flags().Bool("show-key", false, "Show the private key")
}
