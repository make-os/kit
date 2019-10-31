package testutil

import (
	"os"

	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/util/logger"

	"github.com/tendermint/tendermint/cmd/tendermint/commands"

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

	commands.SetLoggerToNoop()

	// Initialize the config using the test root command
	config.Configure(rootCmd, cfg, tmcfg)
	cfg.Node.Mode = config.ModeTest

	// Initialize the directory
	commands.SetConfig(tmcfg)
	commands.InitFilesCmd.RunE(nil, nil)
	tmconfig.EnsureRoot(tmcfg.RootDir)

	// Replace logger with Noop logger
	cfg.G().Log = logger.NewLogrusNoOp()

	return cfg, err
}

// GetDB test databases
func GetDB(cfg *config.EngineConfig) (*storage.Badger, *storage.Badger) {
	appDB := storage.NewBadger()
	if err := appDB.Init(cfg.GetAppDBDir()); err != nil {
		panic(err)
	}
	stateTreeDB := storage.NewBadger()
	if err := stateTreeDB.Init(cfg.GetStateTreeDBDir()); err != nil {
		panic(err)
	}
	return appDB, stateTreeDB
}
