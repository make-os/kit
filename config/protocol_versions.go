package config

import "sync"

// DefaultNetVersion is the default network
// version used when no network version is provided.
const DefaultNetVersion = "0001"

// MainNetVersion is the main net version number
const MainNetVersion = "0001"

// BlockVersion is the version of each block of the chain
const BlockVersion = "1"

var (
	// versions contains protocol handlers versions information
	cfgLck        = &sync.RWMutex{}
	curNetVersion = DefaultNetVersion
)

// IsMainNetVersion checks whether a given version represents the mainnet version
func IsMainNetVersion(version string) bool {
	return version == MainNetVersion
}

// IsMainNet returns true if the current network is the mainnet
func IsMainNet() bool {
	return GetNetVersion() == MainNetVersion
}

// SetVersions sets the protocol version.
// All protocol handlers will be prefixed
// with the version to create a
func SetVersions(netVersion string) {
	cfgLck.Lock()
	defer cfgLck.Unlock()

	if netVersion == "" {
		netVersion = DefaultNetVersion
	} else {
		curNetVersion = netVersion
	}
}

// GetNetVersion returns the current network version
func GetNetVersion() string {
	return curNetVersion
}

func init() {
	SetVersions("")
}
