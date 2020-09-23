package config

import (
	"path/filepath"

	"github.com/spf13/viper"
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

	// PersistentPeers is a comma separated list of persistent peers to connect to.
	PersistentPeers string `json:"addpeer" mapstructure:"addpeer"`

	// GitBinPath is the path to the git executable
	GitBinPath string `json:"gitpath" mapstructure:"gitpath"`

	// Extensions contains list of extensions to run on startup
	Extensions []string `json:"exts" mapstructure:"exts"`

	// ExtensionsArgs contains arguments for extensions
	ExtensionsArgs map[string]string `json:"extsargs" mapstructure:"extsargs"`

	// Validator indicates whether to run the node in validator capacity
	Validator bool `json:"validator" mapstructure:"validator"`

	// IgnoreSeeds will prevent seed address from being used
	IgnoreSeeds bool `json:"ignoreSeeds" mapstructure:"ignoreSeeds"`
}

// RepoConfig represents repo-related configuration
type RepoConfig struct {

	// Track contains names of repositories to be tracked
	Track []string `json:"track" mapstructure:"track"`

	// Untrack contains the names of repositories to be untracked
	Untrack []string `json:"untrack" mapstructure:"untrack"`

	// UntrackAll indicates that all currently tracked repositories are to be untracked
	UntrackAll bool `json:"untrackall" mapstructure:"untrackall"`
}

// VersionInfo describes the clients
// components and runtime version information
type VersionInfo struct {
	BuildVersion string `json:"buildVersion" mapstructure:"buildVersion"`
	BuildCommit  string `json:"buildCommit" mapstructure:"buildCommit"`
	BuildDate    string `json:"buildDate" mapstructure:"buildDate"`
	GoVersion    string `json:"goVersion" mapstructure:"goVersion"`
}

// Genesis data type
const (
	GenDataTypeAccount = "account"
	GenDataTypeRepo    = "repo"
)

// RepoOwner describes an owner of a repository
type RepoOwner struct {
	Creator  bool   `json:"creator" mapstructure:"creator" msgpack:"creator"`
	JoinedAt uint64 `json:"joinedAt" mapstructure:"joinedAt" msgpack:"joinedAt"`
	Veto     bool   `json:"veto" mapstructure:"veto" msgpack:"veto"`
}

// GenDataEntry describes a genesis file data entry
type GenDataEntry struct {
	Type    string `json:"type" mapstructure:"type"`
	Balance string `json:"balance" mapstructure:"balance"`

	// Type: Account
	Address string `json:"address" mapstructure:"address"`

	// Type: Repo
	Name   string                 `json:"name" mapstructure:"name"`
	Helm   bool                   `json:"helm" mapstructure:"helm"`
	Owners map[string]*RepoOwner  `json:"owners" mapstructure:"owners"`
	Config map[string]interface{} `json:"config" mapstructure:"config"`
}

// NetConfig describes network configurations
type NetConfig struct {
	Version uint64 `json:"version" mapstructure:"version"`
}

// RPCConfig describes RPC config settings
type RPCConfig struct {
	On            bool   `json:"on" mapstructure:"on"`
	User          string `json:"user" mapstructure:"user"`
	Password      string `json:"password" mapstructure:"password"`
	DisableAuth   bool   `json:"disableauth" mapstructure:"disableauth"`
	AuthPubMethod bool   `json:"authpubmethod" mapstructure:"authpubmethod"`
	HTTPS         bool   `json:"https" mapstructure:"https"`
	TMRPCAddress  string `json:"tmaddress" mapstructure:"tmaddress"`
}

// DHTConfig describes DHT config parameters
type DHTConfig struct {
	On             bool   `json:"on" mapstructure:"on"`
	Address        string `json:"address" mapstructure:"address"`
	BootstrapPeers string `json:"addpeer" mapstructure:"addpeer"`
}

// RemoteConfig describes repository manager config parameters
type RemoteConfig struct {
	Address string `json:"address" mapstructure:"address"`
	Name    string `json:"name" mapstructure:"name"`
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

	// Repo holds repo-related configuration
	Repo *RepoConfig `json:"repo" mapstructure:"repo"`

	// Net holds network configurations
	Net *NetConfig `json:"net" mapstructure:"net"`

	// RPC holds RPC configurations
	RPC *RPCConfig `json:"rpc" mapstructure:"rpc"`

	// DHT holds DHT configurations
	DHT *DHTConfig `json:"dht" mapstructure:"dht"`

	// Remote holds repository remote configurations
	Remote *RemoteConfig `json:"remote" mapstructure:"remote"`

	// Mempool holds mempool configurations
	Mempool *MempoolConfig `json:"mempool" mapstructure:"mempool"`

	// GenesisFileEntries includes the initial state objects
	GenesisFileEntries []*GenDataEntry `json:"gendata" mapstructure:"gendata"`

	// dataDir is where the node's config and network data is stored
	dataDir string

	// dataDir is where the network's data is stored
	netDataDir string

	// accountDir is where the node's accounts are stored
	keystoreDir string

	// dbDir is where the node's database files are stored
	dbDir string

	// repoDir is where repositories are stored
	repoDir string

	// extensionDir is where extensions are stored
	extensionDir string

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
func EmptyAppConfig() *AppConfig {
	return &AppConfig{
		Node:               &NodeConfig{},
		Net:                &NetConfig{},
		Repo:               &RepoConfig{},
		RPC:                &RPCConfig{},
		DHT:                &DHTConfig{},
		Remote:             &RemoteConfig{},
		Mempool:            &MempoolConfig{},
		GenesisFileEntries: []*GenDataEntry{},
		VersionInfo:        &VersionInfo{},
		g:                  &Globals{},
	}
}

// GetAppName returns the app's name
func (c *AppConfig) GetAppName() string {
	return AppName
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

// KeystoreDir returns the application's accounts directory
func (c *AppConfig) KeystoreDir() string {
	return c.keystoreDir
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

// IsValidatorNode checks if the node is a validator node
func (c *AppConfig) IsValidatorNode() bool {
	return c.Node.Validator
}

// GetExtensionDir returns the extension directory
func (c *AppConfig) GetExtensionDir() string {
	return c.extensionDir
}

// SetRepoRoot sets the repo root directory
func (c *AppConfig) SetRepoRoot(dir string) {
	c.repoDir = dir
}

// IsAttached checks whether the node was started in attach mode
func (c *AppConfig) IsAttachMode() bool {
	return viper.GetBool("attachmode")
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

// IsProd checks whether the current environment is 'production'
func (c *AppConfig) IsProd() bool {
	return c.Node.Mode == ModeProd
}
