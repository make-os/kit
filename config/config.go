package config

import (
	"encoding/json"
	"fmt"
	golog "log"
	"os"
	path "path/filepath"
	"strings"
	"time"

	"github.com/make-os/lobe/data"
	"github.com/make-os/lobe/params"
	"github.com/make-os/lobe/util"
	"github.com/olebedev/emitter"
	"github.com/pkg/errors"
	"github.com/tendermint/tendermint/libs/common"

	"github.com/tendermint/tendermint/config"

	"github.com/make-os/lobe/pkgs/logger"
	"github.com/spf13/viper"
)

var (
	// AppName is the name of the application
	AppName = "lobe"

	// ExecName is the short name for the executable
	ExecName = "lob"

	// DefaultDataDir is the path to the data directory
	DefaultDataDir = os.ExpandEnv("$HOME/." + AppName)

	// KeystoreDirName is the name of the directory where accounts are stored
	KeystoreDirName = "keystore"

	// AppEnvPrefix is used as the prefix for environment variables
	AppEnvPrefix = AppName

	// DefaultNodeAddress is the default Node listening address
	DefaultNodeAddress = ":9000"

	// DefaultTMRPCAddress is the default RPC listening address for the tendermint
	DefaultTMRPCAddress = "127.0.0.1:9001"

	// DefaultRemoteServerAddress is the default remote server listening address
	DefaultRemoteServerAddress = ":9002"

	// DefaultDHTAddress is the default DHT listening address
	DefaultDHTAddress = ":9003"

	// NoColorFormatting indicates that stdout/stderr output should have no color
	NoColorFormatting = false

	// PersistentSeedPeers are peers are trusted, permanent peers to connect us to the network.
	// They will be redialed on connection failure.
	PersistentSeedPeers = []string{
		"a2f1e5786d3564c14faafffd6a050d2f81c655d9@s1.seeders.live:9000",
		"9cd75740de0c9d7b2a5d3921b78abbbb39b1bebe@s2.seeders.live:9000",
		"3ccd79a6f332f83b85f63290ca53187022aada0a@s3.seeders.live:9000",
		"d0165f00485e22ec0197e15a836ce66587515a84@s4.seeders.live:9000",
	}

	// SeedDHTPeers are DHT seed peers to connect to.
	SeedDHTPeers = []string{
		"/dns4/s1.seeders.live/tcp/9003/p2p/12D3KooWAeorTJTi3uRDC3nSMa1V9CujJQg5XcN3UjSSV2HDceQU",
		"/dns4/s2.seeders.live/tcp/9003/p2p/12D3KooWEksv3Nvbv5dRwKRkLJjoLvsuC6hyokj5sERx8mWrxMoB",
		"/dns4/s3.seeders.live/tcp/9003/p2p/12D3KooWJzM4Hf5KWrXnAJjgJkro7zK2edtDu8ocYt8UgU7vsmFa",
		"/dns4/s4.seeders.live/tcp/9003/p2p/12D3KooWE7KybnAaoxuw6UiMpof2LT9hMky8k83tZgpdNCqRWx9P",
	}
)

// RawStateToGenesisData returns the genesis data
func RawStateToGenesisData(state json.RawMessage) (entries []*GenDataEntry) {
	if err := json.Unmarshal(state, &entries); err != nil {
		panic(errors.Wrap(err, "failed to decoded genesis file"))
	}
	return entries
}

// GenesisData returns the genesis data in raw JSON format.
// If devMode is true, the development genesis file is used.
func GetRawGenesisData(devMode bool) json.RawMessage {
	if !devMode {
		return []byte(data.GenesisData)
	}
	return []byte(data.GenesisDataDev)
}

// setDefaultViperConfig sets default viper config values.
func setDefaultViperConfig() {
	viper.SetDefault("mempool.size", 5000)
	viper.SetDefault("mempool.cacheSize", 10000)
	viper.SetDefault("mempool.maxTxSize", 1024*1024)       // 1MB
	viper.SetDefault("mempool.maxTxsSize", 1024*1024*1024) // 1GB
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

// IsInitialized checks if node is initialized
func IsInitialized(tmcfg *config.Config) bool {
	return common.FileExists(tmcfg.PrivValidatorKeyFile())
}

// ConfigureVM sets up the application command structure, tendermint
// and lobe configuration. This is where all configuration and
// settings are prepared
func Configure(cfg *AppConfig, tmcfg *config.Config, itr *util.Interrupt) {
	NoColorFormatting = viper.GetBool("no-colors")

	// Populate viper from environment variables
	viper.SetEnvPrefix(AppEnvPrefix)
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Create app config and populate with default values
	cfg.Node.Mode = ModeProd
	cfg.g.Interrupt = itr
	dataDir := viper.GetString("home")
	devDataDirPrefix := viper.GetString("home.prefix")
	devMode := viper.GetBool("dev")

	// If home.prefix is set, set mode to dev mode.
	if devDataDirPrefix != "" {
		devMode = true
		dataDir = dataDir + "_" + devDataDirPrefix
	}

	// In development mode, use the development data directory.
	if devMode {
		cfg.Node.Mode = ModeDev
		ExecName = AppName
	}

	// Create the data directory and other sub directories
	os.MkdirAll(dataDir, 0700)
	os.MkdirAll(path.Join(dataDir, KeystoreDirName), 0700)

	// Read tendermint config file into tmcfg
	readTendermintConfig(tmcfg, dataDir)

	// Set viper configuration
	setDefaultViperConfig()
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

	// Read the loaded config into AppConfig
	if err := viper.Unmarshal(&cfg); err != nil {
		golog.Fatalf("Failed to unmarshal configuration file: %s", err)
	}

	// Set network version environment variable
	// if not already set and then reset protocol
	// handlers version.
	SetVersions(uint64(viper.GetInt64("net.version")))

	// Set data and network directories
	cfg.dataDir = dataDir
	cfg.netDataDir = path.Join(dataDir, viper.GetString("net.version"))
	cfg.keystoreDir = path.Join(cfg.DataDir(), KeystoreDirName)
	cfg.consoleHistoryPath = path.Join(cfg.DataDir(), ".console_history")
	cfg.repoDir = path.Join(cfg.NetDataDir(), "data", "repos")
	cfg.extensionDir = path.Join(cfg.DataDir(), "extensions")
	os.MkdirAll(cfg.extensionDir, 0700)

	os.MkdirAll(cfg.NetDataDir(), 0700)
	os.MkdirAll(path.Join(cfg.NetDataDir(), "data"), 0700)
	os.MkdirAll(path.Join(cfg.NetDataDir(), "data", "repos"), 0700)
	os.MkdirAll(path.Join(cfg.NetDataDir(), "config"), 0700)

	// Create logger with file rotation enabled
	logPath := path.Join(cfg.NetDataDir(), "logs")
	os.MkdirAll(logPath, 0700)
	logFile := path.Join(logPath, "main.log")
	logLevelSetting := util.ParseLogLevel(viper.GetString("loglevel"))
	cfg.G().Log = logger.NewLogrusWithFileRotation(logFile, logLevelSetting)

	if devMode {
		cfg.G().Log.SetToDebug()
		tmcfg.P2P.AllowDuplicateIP = true
	}

	// If no logger is wanted, set lobe and tendermint log level to `error`
	noLog := viper.GetBool("no-log")
	if noLog {
		tmcfg.LogLevel = fmt.Sprintf("*:error")
		cfg.G().Log.SetToError()
	}

	// Set block time
	tmcfg.Consensus.TimeoutCommit = time.Second * time.Duration(params.BlockTime)

	// Disable tendermint's tx indexer
	tmcfg.TxIndex.Indexer = "null"

	// Set default version information
	cfg.VersionInfo = &VersionInfo{}
	cfg.VersionInfo.BuildCommit = ""
	cfg.VersionInfo.BuildDate = ""
	cfg.VersionInfo.GoVersion = "go1.12.4"
	cfg.VersionInfo.BuildVersion = ""

	// Use some of the native config to override tendermint's config
	tmcfg.P2P.ListenAddress = cfg.Node.ListeningAddr
	tmcfg.P2P.AddrBookStrict = !devMode
	tmcfg.RPC.ListenAddress = "tcp://" + cfg.RPC.TMRPCAddress

	// Add seed peers if .IgnoreSeeds is false
	if !cfg.Node.IgnoreSeeds {
		tmcfg.P2P.PersistentPeers = cfg.Node.PersistentPeers + "," + strings.Join(PersistentSeedPeers, ",")
		cfg.DHT.BootstrapPeers = cfg.DHT.BootstrapPeers + "," + strings.Join(SeedDHTPeers, ",")
	}

	if cfg.DHT.Address != "" && cfg.DHT.Address[:1] == ":" {
		cfg.DHT.Address = "0.0.0.0" + cfg.DHT.Address
	}

	if cfg.RPC.User == "" && cfg.RPC.Password == "" {
		cfg.RPC.DisableAuth = true
	}

	if tmcfg.P2P.ListenAddress != "" && tmcfg.P2P.ListenAddress[:1] == ":" {
		tmcfg.P2P.ListenAddress = "0.0.0.0" + tmcfg.P2P.ListenAddress
	}

	cfg.G().Bus = emitter.New(0)
	cfg.G().TMConfig = tmcfg
	tmcfg.SetRoot(cfg.NetDataDir())
}
