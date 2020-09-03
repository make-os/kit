package config

import (
	"encoding/json"
	"fmt"
	golog "log"
	"os"
	path "path/filepath"
	"strings"
	"time"

	"github.com/gobuffalo/packr"
	"github.com/make-os/lobe/params"
	"github.com/make-os/lobe/util"
	"github.com/olebedev/emitter"
	"github.com/pkg/errors"

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

	// DefaultDevDataDir is the path to the data directory in development mode
	DefaultDevDataDir = os.ExpandEnv("$HOME/." + AppName + "_dev")

	// KeystoreDirName is the name of the directory where accounts are stored
	KeystoreDirName = "keystore"

	// AppEnvPrefix is used as the prefix for environment variables
	AppEnvPrefix = AppName

	// DefaultNodeAddress is the default Node listening address
	DefaultNodeAddress = "127.0.0.1:9000"

	// DefaultTMRPCAddress is the default RPC listening address for the tendermint
	DefaultTMRPCAddress = "127.0.0.1:9001"

	// DefaultRPCAddress is the default RPC listening address
	DefaultRPCAddress = "127.0.0.1:9002"

	// DefaultDHTAddress is the default DHT listening address
	DefaultDHTAddress = "127.0.0.1:9003"

	// DefaultRemoteServerAddress is the default remote server listening address
	DefaultRemoteServerAddress = "127.0.0.1:9004"

	// NoColorFormatting indicates that stdout/stderr output should have no color
	NoColorFormatting = false

	// PersistentPeers are peers are trusted, permanent peers to connect us to the network.
	// They will be redialed on connection failure.
	PersistentPeers = []string{
		"aba9f171986276b6a0f43f89cae96941f77819b6@127.0.0.1:7000",
	}
)

// RawStateToGenesisData returns the genesis data
func RawStateToGenesisData(state json.RawMessage) []*GenDataEntry {
	var data []*GenDataEntry
	if err := json.Unmarshal(state, &data); err != nil {
		panic(errors.Wrap(err, "failed to decoded genesis file"))
	}
	return data
}

// GenesisData returns the genesis data in raw JSON format.
// If devMode is true, the development genesis file is used.
func GetRawGenesisData(devMode bool) json.RawMessage {

	genesisFileName := "genesis.json"
	if devMode {
		genesisFileName = "genesis_dev.json"
	}

	box := packr.NewBox("../" + params.EmbeddableDataDir)
	genesisData, err := box.FindString(genesisFileName)
	if err != nil {
		panic(errors.Wrap(err, "failed to read genesis file"))
	}
	return []byte(genesisData)
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
	var c = EmptyAppConfig()
	c.Node.Mode = ModeProd
	c.g.Interrupt = itr
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
		c.Node.Mode = ModeDev
		ExecName = AppName
		if devDataDirPrefix == "" {
			dataDir = DefaultDevDataDir
		}
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
	c.keystoreDir = path.Join(c.DataDir(), KeystoreDirName)
	c.consoleHistoryPath = path.Join(c.DataDir(), ".console_history")
	c.repoDir = path.Join(c.NetDataDir(), "data", "repos")
	c.extensionDir = path.Join(c.DataDir(), "extensions")
	os.MkdirAll(c.extensionDir, 0700)

	os.MkdirAll(c.NetDataDir(), 0700)
	os.MkdirAll(path.Join(c.NetDataDir(), "data"), 0700)
	os.MkdirAll(path.Join(c.NetDataDir(), "data", "repos"), 0700)
	os.MkdirAll(path.Join(c.NetDataDir(), "config"), 0700)

	// Create logger with file rotation enabled
	logPath := path.Join(c.NetDataDir(), "logs")
	os.MkdirAll(logPath, 0700)
	logFile := path.Join(logPath, "main.log")
	logLevelSetting := util.ParseLogLevel(viper.GetString("loglevel"))
	c.G().Log = logger.NewLogrusWithFileRotation(logFile, logLevelSetting)

	if devMode {
		c.G().Log.SetToDebug()
		tmcfg.P2P.AllowDuplicateIP = true
	}

	// If no logger is wanted, set lobe and tendermint log level to `error`
	noLog := viper.GetBool("no-log")
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
	tmcfg.P2P.PersistentPeers = c.Node.PersistentPeers + "," + strings.Join(PersistentPeers, ",")
	tmcfg.RPC.ListenAddress = "tcp://" + c.RPC.TMRPCAddress

	if c.DHT.Address != "" && c.DHT.Address[:1] == ":" {
		c.DHT.Address = "0.0.0.0" + c.DHT.Address
	}

	if c.RPC.Address != "" && c.RPC.Address[:1] == ":" {
		c.RPC.Address = "0.0.0.0" + c.RPC.Address
	}

	if c.RPC.User == "" && c.RPC.Password == "" {
		c.RPC.DisableAuth = true
	}

	c.G().Bus = emitter.New(0)
	c.G().TMConfig = tmcfg
	*cfg = c
	*tmcfg = *tmcfg.SetRoot(cfg.NetDataDir())

	return
}
