package testutil

import (
	"os"

	"github.com/makeos/mosdef/util"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	tmconfig "github.com/tendermint/tendermint/config"

	path "path/filepath"

	"github.com/makeos/mosdef/config"
	"github.com/mitchellh/go-homedir"
)

// SetTestCfg prepare a config directory for tests
func SetTestCfg() (*config.EngineConfig, error) {
	var err error
	dir, _ := homedir.Dir()
	dataDir := path.Join(dir, util.RandString(5))
	os.MkdirAll(dataDir, 0700)

	// Create test root command and
	// set required flags and values
	rootCmd := &cobra.Command{}
	rootCmd.PersistentFlags().Uint64("net", config.DefaultNetVersion, "Set the network version")
	rootCmd.PersistentFlags().String("home", "", "Set configuration directory")
	rootCmd.PersistentFlags().Set("home", dataDir)
	rootCmd.PersistentFlags().Set("net", dataDir)
	viper.Set("net.version", 10000000)

	var cfg = &config.EngineConfig{}
	var tmcfg = tmconfig.DefaultConfig()

	// Initialize the config using the test root command
	config.Configure(rootCmd, cfg, tmcfg)
	cfg.Node.Mode = config.ModeTest

	return cfg, err
}
