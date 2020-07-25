package cmd

import (
	"fmt"
	golog "log"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tendermint/tendermint/cmd/tendermint/commands"
	tmcfg "github.com/tendermint/tendermint/config"
	tmtypes "github.com/tendermint/tendermint/types"
	fmt2 "gitlab.com/makeos/lobe/util/colorfmt"
)

// initializeTendermint initializes tendermint
func initializeTendermint() error {
	commands.SetConfig(tmconfig)
	commands.InitFilesCmd.RunE(nil, nil)
	reconfigureTendermint()
	tmcfg.EnsureRoot(tmconfig.RootDir)
	return nil
}

func reconfigureTendermint() {

	// Read the genesis file
	genDoc, err := tmtypes.GenesisDocFromFile(tmconfig.GenesisFile())
	if err != nil {
		golog.Fatalf("Failed to read genesis file: %s", err)
	}

	// Set the chain id
	genDoc.ChainID = viper.GetString("net.version")
	if err = genDoc.SaveAs(tmconfig.GenesisFile()); err != nil {
		golog.Fatalf("Failed set chain id: %s", err)
	}
}

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the application.",
	Long: `This command initializes the applications data directory
and creates default config and keys required to successfully 
launch the node.`,
	Run: func(cmd *cobra.Command, args []string) {
		initializeTendermint()
		fmt.Fprintln(os.Stdout, fmt2.NewColor(color.FgGreen, color.Bold).Sprint("✅ New node initialized!"))
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
