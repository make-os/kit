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

// EngineConfig represents the client's configuration
type EngineConfig struct {

	// Node holds the node configurations
	Node *NodeConfig `json:"node" mapstructure:"node"`

	// dataDir is where the node's config and network data is stored
	dataDir string

	// dataDir is where the network's data is stored
	netDataDir string

	// VersionInfo holds version information
	VersionInfo *VersionInfo `json:"-" mapstructure:"-"`

	// g stores references to global objects that can be
	// used anywhere a config is required. Can help to reduce
	// the complexity method definition
	g *Globals
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

// SetDataDir sets the application's data directory
func (c *EngineConfig) SetDataDir(d string) {
	c.dataDir = d
}

// GetDBDir returns the path where database files are stored
func (c *EngineConfig) GetDBDir() string {
	var ns string
	var dbFile = "data%s.db"
	if c.Node.Mode == ModeDev {
		ns = "_" + c.g.NodeKey.Addr().String()
	}
	return filepath.Join(c.NetDataDir(), fmt.Sprintf(dbFile, ns))
}

// IsDev checks whether the current environment is 'development'
func (c *EngineConfig) IsDev() bool {
	return c.Node.Mode == ModeDev
}
