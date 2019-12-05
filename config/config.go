package config

import (
	"encoding/json"
	"fmt"
	golog "log"
	"os"
	path "path/filepath"
	"strings"
	"time"

	"github.com/makeos/mosdef/params"

	"github.com/gobuffalo/packr"
	"github.com/pkg/errors"

	"github.com/tendermint/tendermint/config"

	"github.com/makeos/mosdef/util/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// AppName is the name of the application
	AppName = "mosdef"

	// DefaultDataDir is the path to the data directory
	DefaultDataDir = os.ExpandEnv("$HOME/." + AppName)

	// DefaultDevDataDir is the path to the data directory in development mode
	DefaultDevDataDir = os.ExpandEnv("$HOME/." + AppName + "_dev")

	// AccountDirName is the name of the directory where accounts are stored
	AccountDirName = "accounts"

	// AppEnvPrefix is used as the prefix for environment variables
	AppEnvPrefix = "MD"

	// DefaultRPCAddress is the default RPC listening address
	DefaultRPCAddress = "127.0.0.1:8999"
)

// getGenesisAccounts returns the genesis/root accounts
func getGenesisAccounts() []map[string]interface{} {
	box := packr.NewBox("../data")
	genesisData, err := box.FindString("genesis_account.json")
	if err != nil {
		panic(errors.Wrap(err, "failed to read genesis file"))
	}

	var data []map[string]interface{}
	if err = json.Unmarshal([]byte(genesisData), &data); err != nil {
		panic(errors.Wrap(err, "failed to decoded genesis account file"))
	}

	return data
}

// setDefaultViperConfig sets default config values.
// They are used when their values is not provided
// in flag, env or config file.
func setDefaultViperConfig(cmd *cobra.Command) {
	viper.SetDefault("net.version", DefaultNetVersion)
	viper.BindPFlag("net.version", cmd.Flags().Lookup("net"))
	viper.SetDefault("rpc.address", DefaultRPCAddress)
	viper.SetDefault("genaccounts", getGenesisAccounts())
	viper.SetDefault("mempool.size", 5000)
	viper.SetDefault("mempool.cacheSize", 10000)
	viper.SetDefault("mempool.maxTxSize", 1024*1024)       // 1MB
	viper.SetDefault("mempool.maxTxsSize", 1024*1024*1024) // 1GB
}

func setDevDefaultConfig() {
	// viper.SetDefault("txPool.capacity", 100)
}

// readTendermintConfig reads tendermint config into a tendermint
// config object
func readTendermintConfig(tmcfg *config.Config, dataDir string) error {
	v := viper.New()
	v.SetEnvPrefix("TM")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()
	v.SetConfigName("config")
	v.AddConfigPath(path.Join(dataDir, "config"))
	if err := v.ReadInConfig(); err != nil {
		return err
	}
	err := v.Unmarshal(tmcfg)
	if err != nil {
		return err
	}
	return nil
}

// Configure sets up the application command structure, tendermint
// and mosdef configuration. This is where all configuration and
// settings are prepared
func Configure(rootCmd *cobra.Command, cfg *EngineConfig, tmcfg *config.Config) {

	var c = EngineConfig{
		Node: &NodeConfig{Mode: ModeProd},
		Net:  &NetConfig{},
		g:    &Globals{},
	}

	dataDir := DefaultDataDir

	// If data directory path is set in a flag, update the default data directory
	dd, _ := rootCmd.PersistentFlags().GetString("home")
	ddPrefix, _ := rootCmd.PersistentFlags().GetString("home.prefix")
	if dd != "" {
		dataDir = dd
	}

	devMode, _ := rootCmd.Flags().GetBool("dev")

	// In development mode, use the development data directory.
	// Attempt to create the directory
	if devMode {
		dataDir = DefaultDevDataDir
		c.Node.Mode = ModeDev

		if ddPrefix != "" {
			dataDir = dataDir + "_" + ddPrefix
		}
	}

	// Create the data directory and other sub directories
	os.MkdirAll(dataDir, 0700)
	os.MkdirAll(path.Join(dataDir, AccountDirName), 0700)

	// Read tendermint config file into tmcfg
	readTendermintConfig(tmcfg, dataDir)

	// Set viper configuration
	setDefaultViperConfig(rootCmd)
	if devMode {
		setDevDefaultConfig()
	}

	viper.SetConfigName(AppName)
	viper.AddConfigPath(dataDir)
	viper.AddConfigPath(".")

	// Create the config file if it does not exist
	if err := viper.ReadInConfig(); err != nil {
		if strings.Index(err.Error(), "Not Found") != -1 {
			viper.SetConfigType("yaml")
			if err = viper.WriteConfigAs(path.Join(dataDir, AppName+".yml")); err != nil {
				golog.Fatalf("Failed to create config file: %s", err)
			}
		} else {
			golog.Fatalf("Failed to read config file: %s", err)
		}
	}

	viper.SetEnvPrefix(AppEnvPrefix)
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)
	viper.AutomaticEnv()

	// Read the loaded config into EngineConfig
	if err := viper.Unmarshal(&c); err != nil {
		golog.Fatalf("Failed to unmarshal configuration file: %s", err)
	}

	// Set network version environment variable
	// if not already set and then reset protocol
	// handlers version.
	SetVersions(uint64(viper.GetInt64("net.version")))

	// Set data and network directories
	c.dataDir = dataDir
	c.netDataDir = path.Join(dataDir, viper.GetString("net.version"))
	c.accountDir = path.Join(c.DataDir(), AccountDirName)
	c.consoleHistoryPath = path.Join(c.DataDir(), ".console_history")
	c.repoDir = path.Join(c.NetDataDir(), "repos")

	// Create network data directory
	os.MkdirAll(c.NetDataDir(), 0700)
	os.MkdirAll(path.Join(c.NetDataDir(), "data"), 0700)
	os.MkdirAll(path.Join(c.NetDataDir(), "repos"), 0700)
	os.MkdirAll(path.Join(c.NetDataDir(), "config"), 0700)

	// Create logger with file rotation enabled
	logPath := path.Join(c.NetDataDir(), "logs")
	os.MkdirAll(logPath, 0700)
	logFile := path.Join(logPath, "main.log")
	c.G().Log = logger.NewLogrusWithFileRotation(logFile)

	if devMode {
		c.G().Log.SetToDebug()
		tmcfg.P2P.AllowDuplicateIP = true
	}

	// If no logger is wanted, set mosdef and tendermint log level to `error`
	noLog, _ := rootCmd.Flags().GetBool("nolog")
	if noLog {
		tmcfg.LogLevel = fmt.Sprintf("*:error")
		c.G().Log.SetToError()
	}

	// Set block time
	tmcfg.Consensus.TimeoutCommit = time.Second * time.Duration(params.BlockTime)

	// Disable tendermint's tx indexer
	tmcfg.TxIndex.Indexer = "null"

	// Set default version information
	c.VersionInfo = &VersionInfo{}
	c.VersionInfo.BuildCommit = ""
	c.VersionInfo.BuildDate = ""
	c.VersionInfo.GoVersion = "go1.12.4"
	c.VersionInfo.BuildVersion = ""

	// Use some of the native config to override tendermint's config
	tmcfg.P2P.ListenAddress = c.Node.ListeningAddr
	tmcfg.P2P.AddrBookStrict = !devMode
	tmcfg.P2P.PersistentPeers = c.Node.Peers
	tmcfg.RPC.ListenAddress = c.RPC.TMRPCAddress

	c.G().TMConfig = tmcfg
	*cfg = c
	*tmcfg = *tmcfg.SetRoot(cfg.NetDataDir())

	return
}
