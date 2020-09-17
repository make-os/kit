package cmd

import (
	"fmt"
	"io/ioutil"
	golog "log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/fatih/color"
	"github.com/make-os/lobe/config"
	"github.com/make-os/lobe/crypto"
	fmt2 "github.com/make-os/lobe/util/colorfmt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/cmd/tendermint/commands"
	tmcfg "github.com/tendermint/tendermint/config"
	tmcrypto "github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/privval"
	tmtypes "github.com/tendermint/tendermint/types"
)

var cdc = amino.NewCodec()

func init() {
	cdc.RegisterInterface((*tmcrypto.PrivKey)(nil), nil)
	cdc.RegisterConcrete(ed25519.PrivKeyEd25519{}, ed25519.PrivKeyAminoName, nil)
}

func genNodeKey(filePath string, pk ed25519.PrivKeyEd25519) (*p2p.NodeKey, error) {
	nodeKey := &p2p.NodeKey{
		PrivKey: pk,
	}

	jsonBytes, err := cdc.MarshalJSON(nodeKey)
	if err != nil {
		return nil, err
	}
	err = ioutil.WriteFile(filePath, jsonBytes, 0600)
	if err != nil {
		return nil, err
	}
	return nodeKey, nil
}

// tendermintInit initializes tendermint
//
// validatorKey: Is a base58 encoded private key to be used by the node for validator operation.
// If not set, a default key is used.
//
// initValidators: are base58 encoded ed25519 public keys to use as initial validators.
// If non is provided, the node will be the sole initial validator.
//
// genesisTime: sets the genesis file time. If zero, current UTC time is used.
func tendermintInit(validatorKey string, genesisValidators []string, genesisState string, genesisTime uint64) error {

	if config.IsInitialized(tmconfig) {
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
			pubKey = strings.TrimSpace(pubKey)
			if pubKey == "" {
				continue
			}
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

	// Set default genesis app state if not provided.
	// If provided and it is a file path, read the file and use it.
	if genesisState == "" {
		genDoc.AppState = config.GetRawGenesisData(cfg.IsDev())
	} else {
		genDoc.AppState = []byte(genesisState)
		if ok, _ := govalidator.IsFilePath(genesisState); ok || strings.HasPrefix(genesisState, "./") {
			path, _ := filepath.Abs(genesisState)
			state, err := ioutil.ReadFile(path)
			if err != nil {
				golog.Fatalf("Failed to read genesis state file (%s)", genesisState)
			}
			genDoc.AppState = state
		}
	}

	// Save the updated genesis file
	if err = genDoc.SaveAs(tmconfig.GenesisFile()); err != nil {
		golog.Fatalf("Genesis config file initialization failed: %s", err)
	}

	// Set validator key if provided
	if validatorKey != "" {
		vk, err := crypto.ConvertBase58PrivKeyToTMPrivKey(strings.TrimSpace(validatorKey))
		if err != nil {
			golog.Fatalf("Failed to decode validator private key: %s", err.Error())
		}
		pv := privval.GenFilePV(tmconfig.PrivValidatorKeyFile(), tmconfig.PrivValidatorStateFile())
		pv.Key.PrivKey = vk
		pv.Key.Address = vk.PubKey().Address()
		pv.Key.PubKey = vk.PubKey()
		pv.Save()

		// Overwrite node key file with one derived from the validator key.
		// TODO: find a way to do this directly without letting tendermint have a
		//  chance to do it before us or submit a PR to upstream for a third-party friendly approach
		nodeKeyFile := tmconfig.NodeKeyFile()
		os.RemoveAll(nodeKeyFile)
		if _, err = genNodeKey(nodeKeyFile, vk); err != nil {
			golog.Fatalf("Failed to create node key file: %s", err.Error())
		}
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
		genesisTime, _ := cmd.Flags().GetUint64("gen-time")
		genState, _ := cmd.Flags().GetString("gen-state")
		tendermintInit(validatorKey, validators, genState, genesisTime)
		fmt.Fprintln(os.Stdout, fmt2.NewColor(color.FgGreen, color.Bold).Sprint("✅ Node initialized!"))
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringSliceP("validators", "v", nil, "Public key of initial validators")
	initCmd.Flags().StringP("validator-key", "k", "", "Private key to use for validator role")
	initCmd.Flags().Uint64P("gen-time", "t", 0, "Specify genesis time (default: current UTC time)")
	initCmd.Flags().StringP("gen-state", "s", "", "Specify raw or path to genesis state")
}
