package config

import (
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

	// ListeningAddr is the node's listening address
	ListeningAddr string `json:"address" mapstructure:"address"`

	// Peers is a comma separated list of persistent peers to connect to.
	Peers string `json:"addpeer" mapstructure:"addpeer"`

	// GitBinPath is the path to the git executable
	GitBinPath string `json:"gitbin" mapstructure:"gitbin"`
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
	Address      string `json:"address" mapstructure:"address"`
	TMRPCAddress string `json:"tmaddress" mapstructure:"tmaddress"`
}

// DHTConfig describes DHT config parameters
type DHTConfig struct {
	Address string `json:"address" mapstructure:"address"`
}

// RepoManagerConfig describes repository manager config parameters
type RepoManagerConfig struct {
	Address string `json:"address" mapstructure:"address"`
}

// MempoolConfig describes mempool config parameters
type MempoolConfig struct {
	Size       int   `json:"size" mapstructure:"size"`
	CacheSize  int   `json:"cacheSize" mapstructure:"cacheSize"`
	MaxTxSize  int   `json:"maxTxSize" mapstructure:"maxTxSize"`
	MaxTxsSize int64 `json:"maxTxsSize" mapstructure:"maxTxsSize"`
}

// AppConfig represents the applications configuration
type AppConfig struct {

	// Node holds the node configurations
	Node *NodeConfig `json:"node" mapstructure:"node"`

	// Net holds network configurations
	Net *NetConfig `json:"net" mapstructure:"net"`

	// RPC holds RPC configurations
	RPC *RPCConfig `json:"rpc" mapstructure:"rpc"`

	// DHT holds DHT configurations
	DHT *DHTConfig `json:"dht" mapstructure:"dht"`

	// RepoMan holds repository manager configurations
	RepoMan *RepoManagerConfig `json:"repoman" mapstructure:"repoman"`

	// Mempool holds mempool configurations
	Mempool *MempoolConfig `json:"mempool" mapstructure:"mempool"`

	// GenesisAccounts includes the initial/root accounts and their balances
	GenesisAccounts []*GenAccount `json:"genaccounts" mapstructure:"genaccounts"`

	// dataDir is where the node's config and network data is stored
	dataDir string

	// dataDir is where the network's data is stored
	netDataDir string

	// accountDir is where the node's accounts are stored
	accountDir string

	// dbDir is where the node's database files are stored
	dbDir string

	// repoDir is where repositories are stored
	repoDir string

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

// EmptyAppConfig returns an empty Config Object
func EmptyAppConfig() AppConfig {
	return AppConfig{
		Node:            &NodeConfig{},
		Net:             &NetConfig{},
		RPC:             &RPCConfig{},
		DHT:             &DHTConfig{},
		RepoMan:         &RepoManagerConfig{},
		Mempool:         &MempoolConfig{},
		GenesisAccounts: []*GenAccount{},
		VersionInfo:     &VersionInfo{},
		g:               &Globals{},
	}
}

// GetConsoleHistoryPath returns the filepath where the console
// input history is stored
func (c *AppConfig) GetConsoleHistoryPath() string {
	return c.consoleHistoryPath
}

// SetNetDataDir sets the network's data directory
func (c *AppConfig) SetNetDataDir(d string) {
	c.netDataDir = d
}

// NetDataDir returns the network's data directory
func (c *AppConfig) NetDataDir() string {
	return c.netDataDir
}

// DataDir returns the application's data directory
func (c *AppConfig) DataDir() string {
	return c.dataDir
}

// AccountDir returns the application's accounts directory
func (c *AppConfig) AccountDir() string {
	return c.accountDir
}

// SetDataDir sets the application's data directory
func (c *AppConfig) SetDataDir(d string) {
	c.dataDir = d
}

// GetDBRootDir returns the directory where all database files are stored
func (c *AppConfig) GetDBRootDir() string {
	return filepath.Join(c.NetDataDir(), "data")
}

// GetRepoRoot returns the repo root directory
func (c *AppConfig) GetRepoRoot() string {
	return c.repoDir
}

// SetRepoRoot sets the repo root directory
func (c *AppConfig) SetRepoRoot(dir string) {
	c.repoDir = dir
}

// GetAppDBDir returns the path where app's database files are stored
func (c *AppConfig) GetAppDBDir() string {
	return filepath.Join(c.GetDBRootDir(), "appdata.db")
}

// GetDHTStoreDir returns the path where dht database files are stored
func (c *AppConfig) GetDHTStoreDir() string {
	return filepath.Join(c.GetDBRootDir(), "dht.db")
}

// GetStateTreeDBDir returns the path where state's database files are stored
func (c *AppConfig) GetStateTreeDBDir() string {
	return filepath.Join(c.GetDBRootDir(), "appstate.db")
}

// IsDev checks whether the current environment is 'development'
func (c *AppConfig) IsDev() bool {
	return c.Node.Mode == ModeDev
}
