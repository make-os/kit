package config

import "sync"

// DefaultNetVersion is the default network
// version used when no network version is provided.
const DefaultNetVersion = uint64(1)

// MainNetVersion is the main net version number
const MainNetVersion = uint64(1)

var (
	// versions contains protocol handlers versions information
	cfgLck        = &sync.RWMutex{}
	curNetVersion = DefaultNetVersion
)

// IsMainNetVersion checks whether a given version represents the mainnet version
func IsMainNetVersion(version uint64) bool {
	return version == MainNetVersion
}

// IsMainNet returns true if the current network is the mainnet
func IsMainNet() bool {
	return GetNetVersion() == MainNetVersion
}

// SetVersion sets the protocol version.
// All protocol handlers will be prefixed
// with the version to create a
func SetVersion(netVersion uint64) {
	cfgLck.Lock()
	defer cfgLck.Unlock()

	if netVersion == 0 {
		netVersion = DefaultNetVersion
	} else {
		curNetVersion = netVersion
	}
}

// GetNetVersion returns the current network version
func GetNetVersion() uint64 {
	return curNetVersion
}

func init() {
	SetVersion(1)
}
