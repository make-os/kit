package config

import (
	golog "log"
	"os"
	"path"
	"strings"

	"github.com/makeos/mosdef/util/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tendermint/tendermint/libs/cli"
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
)

// setDefaultConfig sets default config values.
// They are used when their values is not provided
// in flag, env or config file.
func setDefaultConfig() {
	viper.SetDefault("net.version", DefaultNetVersion)
}

func setDevDefaultConfig() {
	// viper.SetDefault("txPool.capacity", 100)
}

// Configure sets up the application command structure, tendermint
// and mosdef configuration. This is where all configuration and
// settings are prepared
func Configure(rootCmd, tmRootCmd *cobra.Command, cfg *EngineConfig) {

	if err := rootCmd.Execute(); err != nil {
		golog.Fatalf("Failed to execute root command")
	}

	var c = EngineConfig{
		Node: &NodeConfig{Mode: ModeProd},
		g:    &Globals{},
	}

	dataDir := DefaultDataDir

	// If data directory path is set in a flag, update the default data directory
	dd, _ := rootCmd.PersistentFlags().GetString("home")
	if dd != "" {
		dataDir = dd
	}

	devMode, _ := rootCmd.Flags().GetBool("dev")

	// In development mode, use the development data directory.
	// Attempt to create the directory
	if devMode {
		dataDir = DefaultDevDataDir
		c.Node.Mode = ModeDev
	}

	// Create the data directory and other sub directories
	os.MkdirAll(dataDir, 0700)
	os.MkdirAll(path.Join(dataDir, AccountDirName), 0700)

	// Set viper configuration
	setDefaultConfig()
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
	SetVersions(viper.GetString("net.version"))

	// Set data and network directories
	c.SetNetDataDir(path.Join(dataDir, viper.GetString("net.version")))

	// Create network data directory
	os.MkdirAll(c.NetDataDir(), 0700)

	// Create logger with file rotation enabled
	logPath := path.Join(c.NetDataDir(), "logs")
	os.MkdirAll(logPath, 0700)
	logFile := path.Join(logPath, "main.log")
	c.G().Log = logger.NewLogrusWithFileRotation(logFile)

	// Set default version information
	c.VersionInfo = &VersionInfo{}
	c.VersionInfo.BuildCommit = ""
	c.VersionInfo.BuildDate = ""
	c.VersionInfo.GoVersion = "go1.12.4"
	c.VersionInfo.BuildVersion = ""

	*cfg = c

	// Add flags and prefix all env exposed with MD
	executor := cli.PrepareMainCmd(tmRootCmd, AppEnvPrefix, dataDir)
	if err := executor.Execute(); err != nil {
		golog.Fatalf("Failed executing tendermint prepare command: %s, exiting...", err)
	}

	return
}
