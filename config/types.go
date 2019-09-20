package config

import (
	"fmt"
	"path/filepath"
)

const (
	// ModeProd refers to production mode
	ModeProd = iota
	// ModeDev refers to development mode
	ModeDev
	// ModeTest refers to test mode
	ModeTest
)

// NodeConfig represents node's configuration
type NodeConfig struct {

	// Mode determines the current environment type
	Mode int `json:"mode" mapstructure:"mode"`

	// Key is the address of the node key to use for start up
	Key string `json:"key" mapstructure:"key"`
}

// VersionInfo describes the clients
// components and runtime version information
type VersionInfo struct {
	BuildVersion string `json:"buildVersion" mapstructure:"buildVersion"`
	BuildCommit  string `json:"buildCommit" mapstructure:"buildCommit"`
	BuildDate    string `json:"buildDate" mapstructure:"buildDate"`
	GoVersion    string `json:"goVersion" mapstructure:"goVersion"`
}

// GenAccount describes root account and its balance
type GenAccount struct {
	Address string `json:"address" mapstructure:"address"`
	Balance string `json:"balance" mapstructure:"balance"`
}

// NetConfig describes network configurations
type NetConfig struct {
	Version uint64 `json:"version" mapstructure:"version"`
}

// RPCConfig describes RPC config settings
type RPCConfig struct {
	Address string `json:"address" mapstructure:"address"`
}

// EngineConfig represents the client's configuration
type EngineConfig struct {

	// Node holds the node configurations
	Node *NodeConfig `json:"node" mapstructure:"node"`

	// Net holds network configurations
	Net *NetConfig `json:"net" mapstructure:"net"`

	// RPC holds RPC configurations
	RPC *RPCConfig `json:"rpc" mapstructure:"rpc"`

	// GenesisAccounts includes the initial/root accounts and their balances
	GenesisAccounts []*GenAccount `json:"genaccounts" mapstructure:"genaccounts"`

	// dataDir is where the node's config and network data is stored
	dataDir string

	// dataDir is where the network's data is stored
	netDataDir string

	// accountDir is where the node's accounts are stored
	accountDir string

	// consoleHistoryPath is the path to the file where console input
	// history is stored.
	consoleHistoryPath string

	// VersionInfo holds version information
	VersionInfo *VersionInfo `json:"-" mapstructure:"-"`

	// g stores references to global objects that can be
	// used anywhere a config is required. Can help to reduce
	// the complexity method definition
	g *Globals
}

// GetConsoleHistoryPath returns the filepath where the console
// input history is stored
func (c *EngineConfig) GetConsoleHistoryPath() string {
	return c.consoleHistoryPath
}

// SetNetDataDir sets the network's data directory
func (c *EngineConfig) SetNetDataDir(d string) {
	c.netDataDir = d
}

// NetDataDir returns the network's data directory
func (c *EngineConfig) NetDataDir() string {
	return c.netDataDir
}

// DataDir returns the application's data directory
func (c *EngineConfig) DataDir() string {
	return c.dataDir
}

// AccountDir returns the application's accounts directory
func (c *EngineConfig) AccountDir() string {
	return c.accountDir
}

// SetDataDir sets the application's data directory
func (c *EngineConfig) SetDataDir(d string) {
	c.dataDir = d
}

// GetDBDir returns the path where database files are stored
func (c *EngineConfig) GetDBDir() string {
	var ns string
	var dbFile = "data%s.db"
	if c.Node.Mode == ModeDev {
		ns = "_" + string(c.g.NodeKey.ID())
	}
	return filepath.Join(c.NetDataDir(), fmt.Sprintf(dbFile, ns))
}

// IsDev checks whether the current environment is 'development'
func (c *EngineConfig) IsDev() bool {
	return c.Node.Mode == ModeDev
}
