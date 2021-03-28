package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	path "path/filepath"
	"strings"
	"time"

	"github.com/make-os/kit/data"
	"github.com/make-os/kit/pkgs/logger"
	"github.com/make-os/kit/util"
	"github.com/mitchellh/go-homedir"
	"github.com/olebedev/emitter"
	"github.com/pkg/errors"
	tmos "github.com/tendermint/tendermint/libs/os"

	"github.com/tendermint/tendermint/config"

	"github.com/spf13/viper"
)

var (
	cfg = EmptyAppConfig()

	// itr is used to interrupt subscribed components
	itr = util.Interrupt(make(chan struct{}))

	// AppName is the name of the application
	AppName = "kit"

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

	// DefaultPassAgentPort is the port on which the passphrase cache agent listens on
	DefaultPassAgentPort = "9004"

	// NoColorFormatting indicates that stdout/stderr output should have no color
	NoColorFormatting = false

	// DefaultLightNodeTrustPeriod is the trusting period that headers can be
	// verified within. Should be significantly less than the unbonding period.
	// TODO: Determine actual value for production env
	DefaultLightNodeTrustPeriod = 168 * time.Hour

	// ChainSeedPeers are peers are trusted, permanent peers to connect us to the network.
	// They will be redialed on connection failure.
	ChainSeedPeers = []string{
		"a2f1e5786d3564c14faafffd6a050d2f81c655d9@s1.seeders.live:9000",
		"9cd75740de0c9d7b2a5d3921b78abbbb39b1bebe@s2.seeders.live:9000",
		"3ccd79a6f332f83b85f63290ca53187022aada0a@s3.seeders.live:9000",
		"d0165f00485e22ec0197e15a836ce66587515a84@s4.seeders.live:9000",
	}

	// DHTSeedPeers are DHT seed peers to connect to.
	DHTSeedPeers = []string{
		"/dns4/s1.seeders.live/tcp/9003/p2p/12D3KooWAeorTJTi3uRDC3nSMa1V9CujJQg5XcN3UjSSV2HDceQU",
		"/dns4/s2.seeders.live/tcp/9003/p2p/12D3KooWEksv3Nvbv5dRwKRkLJjoLvsuC6hyokj5sERx8mWrxMoB",
		"/dns4/s3.seeders.live/tcp/9003/p2p/12D3KooWJzM4Hf5KWrXnAJjgJkro7zK2edtDu8ocYt8UgU7vsmFa",
		"/dns4/s4.seeders.live/tcp/9003/p2p/12D3KooWE7KybnAaoxuw6UiMpof2LT9hMky8k83tZgpdNCqRWx9P",
	}
)

// GetConfig get the app config
func GetConfig() *AppConfig {
	return cfg
}

// GetInterrupt returns the component interrupt channel
func GetInterrupt() *util.Interrupt {
	return &itr
}

func init() {
	DefaultDataDir, _ = homedir.Expand(path.Join("~", "."+AppName))
}

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

// readTendermintConfig reads tendermint config into a tendermint config object
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

// IsTendermintInitialized checks if node is initialized
func IsTendermintInitialized(tmcfg *config.Config) bool {
	return tmos.FileExists(tmcfg.PrivValidatorKeyFile())
}

// ConfigureVM sets up the application command structure, tendermint
// and kit configuration. This is where all configuration and
// settings are prepared
func Configure(appCfg *AppConfig, tmcfg *config.Config, initializing bool) {
	NoColorFormatting = viper.GetBool("no-colors")

	// Set default version information
	appCfg.VersionInfo = &VersionInfo{}

	// Setup viper and app directories
	setup(appCfg, tmcfg, initializing)

	// Tendermint config overwrites
	setupTendermintCfg(appCfg, tmcfg)

	// Setup logger
	setupLogger(appCfg, tmcfg)

	// In dev mode, do not use the production persistent seed peers
	if cfg.IsDev() {
		ChainSeedPeers = []string{}
		DHTSeedPeers = []string{}
	}

	// Add seed peers if .IgnoreSeeds is false
	if !appCfg.Node.IgnoreSeeds {
		tmcfg.P2P.PersistentPeers = appCfg.Node.PersistentPeers + "," + strings.Join(ChainSeedPeers, ",")
		appCfg.DHT.BootstrapPeers = appCfg.DHT.BootstrapPeers + "," + strings.Join(DHTSeedPeers, ",")
	}

	if appCfg.DHT.Address != "" && appCfg.DHT.Address[:1] == ":" {
		appCfg.DHT.Address = "0.0.0.0" + appCfg.DHT.Address
	}

	if appCfg.RPC.User == "" && appCfg.RPC.Password == "" {
		appCfg.RPC.DisableAuth = true
	}

	if tmcfg.P2P.ListenAddress != "" && tmcfg.P2P.ListenAddress[:1] == ":" {
		tmcfg.P2P.ListenAddress = "0.0.0.0" + tmcfg.P2P.ListenAddress
	}

	appCfg.G().Bus = emitter.New(0)
	appCfg.G().TMConfig = tmcfg
}

func setupTendermintCfg(cfg *AppConfig, tmcfg *config.Config) {
	tmcfg.TxIndex.Indexer = "kv"
	tmcfg.P2P.ListenAddress = cfg.Node.ListeningAddr
	tmcfg.P2P.AddrBookStrict = !cfg.IsDev()
	tmcfg.RPC.ListenAddress = "tcp://" + cfg.RPC.TMRPCAddress

	if cfg.IsTest() {
		return
	}

	// Check if there is a pre-defined chain configurer for the version.
	// If yes, use it to apply configurations, otherwise, return
	netVersion := viper.GetString("net.version")
	chain := Get(netVersion)
	if chain != nil {
		chain.Config(cfg, tmcfg)
	}
}

func setup(cfg *AppConfig, tmcfg *config.Config, initializing bool) {

	viper.SetEnvPrefix(AppEnvPrefix)
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Set mode
	if cfg.Node.Mode == 0 {
		cfg.Node.Mode = ModeProd
		if viper.GetBool("dev") {
			cfg.Node.Mode = ModeDev
		}
	}

	// Get home directory ID and set mode to Dev if user provided a home.id value
	homeID := viper.GetString("home.id")
	if homeID != "" {
		cfg.Node.Mode = ModeDev
		homeID = fmt.Sprintf("_%s", homeID)
	}

	// Construct data directory, if not set in config
	dataDir := cfg.dataDir
	if dataDir == "" {
		var err error
		dataDir, err = homedir.Expand(path.Join("~", "."+AppName+homeID))
		if err != nil {
			log.Fatalf("Failed to get home directory: %s", err)
		}
	}

	// Create the data directory and keystore directory
	_ = os.MkdirAll(dataDir, 0700)
	_ = os.MkdirAll(path.Join(dataDir, KeystoreDirName), 0700)

	// Set viper configuration
	setDefaultViperConfig()
	viper.SetConfigName(AppName)
	viper.AddConfigPath(dataDir)
	viper.AddConfigPath(".")

	// Attempt to read the config file
	noConfigFile := false
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			noConfigFile = true
		} else {
			log.Fatalf("Failed to read config file: %s", err)
		}
	}

	// Set tendermint root directory
	tmcfg.SetRoot(path.Join(dataDir, viper.GetString("net.version")))

	// Ensure tendermint files have been initialized in the network directory
	if !initializing && !IsTendermintInitialized(tmcfg) {
		log.Fatalf("Data directory has not been initialized yet (have you ran `%s init` ?)", AppName)
	}

	// Attempt to read tendermint config file when not initializing
	if !initializing {
		if err := readTendermintConfig(tmcfg, tmcfg.RootDir); err != nil {
			log.Fatalf("Failed to read tendermint configuration:%s ", err.Error())
		}
	}

	// Allow nodes to share same IP locally
	if cfg.IsDev() {
		tmcfg.P2P.AllowDuplicateIP = true
	}

	// Create the config file if it doesn't exist
	if noConfigFile {
		viper.SetConfigType("yaml")
		if err := viper.WriteConfigAs(path.Join(dataDir, AppName+".yml")); err != nil {
			log.Fatalf("Failed to create config file: %s", err)
		}
	}

	// Read the config file into AppConfig if it exists
	if err := viper.Unmarshal(&cfg); err != nil {
		log.Fatalf("Failed to unmarshal configuration file: %s", err)
	}

	// Set network version environment variable if not already
	// set and then reset protocol handlers version.
	SetVersion(uint64(viper.GetInt64("net.version")))

	// Set data and network directories
	cfg.dataDir = dataDir
	cfg.netDataDir = path.Join(dataDir, viper.GetString("net.version"))
	cfg.keystoreDir = path.Join(cfg.DataDir(), KeystoreDirName)
	cfg.consoleHistoryPath = path.Join(cfg.DataDir(), ".console_history")
	cfg.repoDir = path.Join(cfg.NetDataDir(), "data", "repos")
	cfg.extensionDir = path.Join(cfg.DataDir(), "extensions")
	_ = os.MkdirAll(cfg.extensionDir, 0700)
	_ = os.MkdirAll(cfg.NetDataDir(), 0700)
	_ = os.MkdirAll(path.Join(cfg.NetDataDir(), "data"), 0700)
	_ = os.MkdirAll(path.Join(cfg.NetDataDir(), "data", "repos"), 0700)
	_ = os.MkdirAll(path.Join(cfg.NetDataDir(), "config"), 0700)
}

func setupLogger(cfg *AppConfig, tmcfg *config.Config) {

	// Create logger with file rotation enabled
	logPath := path.Join(cfg.NetDataDir(), "logs")
	_ = os.MkdirAll(logPath, 0700)
	logFile := path.Join(logPath, "main.log")
	logLevelSetting := util.ParseLogLevel(viper.GetString("loglevel"))
	cfg.G().Log = logger.NewLogrusWithFileRotation(logFile, logLevelSetting)

	if cfg.IsDev() {
		cfg.G().Log.SetToDebug()
	}

	// If no logger is wanted, set kit and tendermint log level to `error`
	noLog := viper.GetBool("no-log")
	if noLog {
		tmcfg.LogLevel = fmt.Sprintf("*:error")
		cfg.G().Log.SetToError()
	}
}
