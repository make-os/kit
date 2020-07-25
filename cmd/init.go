package cmd

import (
	"fmt"
	golog "log"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tendermint/tendermint/cmd/tendermint/commands"
	tmcfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/libs/common"
	"github.com/tendermint/tendermint/privval"
	tmtypes "github.com/tendermint/tendermint/types"
	"github.com/themakeos/lobe/crypto"
	fmt2 "github.com/themakeos/lobe/util/colorfmt"
)

// tendermintInit initializes tendermint
//
// validatorKey: Is a base58 encoded private key to be used by the node for validator operation.
// If not set, a default key is used.
//
// initValidators: are base58 encoded ed25519 public keys to use as initial validators.
// If non is provided, the node will be the sole initial validator.
//
// genesisTime: sets the genesis file time. If zero, current UTC time is used.
func tendermintInit(validatorKey string, genesisValidators []string, genesisTime uint64) error {

	// Do nothing if already initialized
	if common.FileExists(tmconfig.PrivValidatorKeyFile()) {
		return nil
	}

	defer tmcfg.EnsureRoot(tmconfig.RootDir)
	commands.SetConfig(tmconfig)
	commands.InitFilesCmd.RunE(nil, nil)

	// Read the genesis file
	genDoc, err := tmtypes.GenesisDocFromFile(tmconfig.GenesisFile())
	if err != nil {
		golog.Fatalf("Failed to read genesis file: %s", err)
	}

	// Replace genesis validators if provided
	if len(genesisValidators) > 0 {
		genDoc.Validators = []tmtypes.GenesisValidator{}
		for _, pubKey := range genesisValidators {
			pk, err := crypto.ConvertBase58PubKeyToTMPubKey(pubKey)
			if err != nil {
				golog.Fatalf("Failed to decode genesis validator public key %s", pubKey)
			}
			genDoc.Validators = append(genDoc.Validators, tmtypes.GenesisValidator{
				Power:   10,
				PubKey:  pk,
				Address: pk.Address(),
			})
		}
	}

	// Set the chain ID
	genDoc.ChainID = viper.GetString("net.version")

	// Set genesis time if provided
	if genesisTime != 0 {
		genDoc.GenesisTime = time.Unix(int64(genesisTime), 0)
	}

	// Save the updated genesis file
	if err = genDoc.SaveAs(tmconfig.GenesisFile()); err != nil {
		golog.Fatalf("Failed set chain id: %s", err)
	}

	// Set validator key if provided
	if validatorKey != "" {
		vk, err := crypto.ConvertBase58PrivKeyToTMPrivKey(validatorKey)
		if err != nil {
			golog.Fatalf("Failed to decode validator private key")
		}
		pv := privval.GenFilePV(tmconfig.PrivValidatorKeyFile(), tmconfig.PrivValidatorStateFile())
		pv.Key.PrivKey = vk
		pv.Key.Address = vk.PubKey().Address()
		pv.Key.PubKey = vk.PubKey()
		pv.Save()
	}

	return nil
}

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the application.",
	Long:  `This command initializes the node's data directory and config files.`,
	Run: func(cmd *cobra.Command, args []string) {
		validators, _ := cmd.Flags().GetStringSlice("validators")
		validatorKey, _ := cmd.Flags().GetString("validator-key")
		genesisTime, _ := cmd.Flags().GetUint64("genesis-time")
		tendermintInit(validatorKey, validators, genesisTime)
		fmt.Fprintln(os.Stdout, fmt2.NewColor(color.FgGreen, color.Bold).Sprint("✅ Node initialized!"))
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringSliceP("validators", "v", nil, "Public key of initial validators")
	initCmd.Flags().StringP("validator-key", "k", "", "Private key to use for validator role")
	initCmd.Flags().Uint64P("genesis-time", "t", 0, "Specify genesis time (default: current UTC time)")
}
