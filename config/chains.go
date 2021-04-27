package config

import (
	"time"

	"github.com/spf13/cast"
	tmcfg "github.com/tendermint/tendermint/config"
)

// networks stores known networks information
var networks = map[string]Info{}

func init() {
	networks[cast.ToString(TestnetV1.GetVersion())] = TestnetV1
	networks[cast.ToString(Dev.GetVersion())] = Dev
}

// Get finds a chain's info  by version
func Get(version string) Info {
	n, ok := networks[version]
	if !ok {
		return nil
	}
	return n
}

// Info describes a chain version
type Info interface {
	GetVersion() int
	GetName() string
	Configure(cfg *AppConfig, tmc *tmcfg.Config)
}

// ChainInfo implements Info
type ChainInfo struct {
	NetVersion  int
	GenesisTime uint64
	Validators  []string
	Name        string
	Configurer  func(cfg *AppConfig, tmcfg *tmcfg.Config)
}

// Configure updates the given config objects
func (ci *ChainInfo) Configure(cfg *AppConfig, tmcfg *tmcfg.Config) {
	ci.Configurer(cfg, tmcfg)
}

// GetName returns the name of the name
func (ci *ChainInfo) GetName() string {
	return ci.Name
}

// GetVersion returns the chain's numeric version
func (ci *ChainInfo) GetVersion() int {
	return ci.NetVersion
}

// Dev contains configurations for development
var Dev = &ChainInfo{
	Name:        "dev",
	NetVersion:  1000,
	GenesisTime: 1595700581,
	Validators:  []string{},
	Configurer: func(cfg *AppConfig, tmc *tmcfg.Config) {
		tmc.Consensus.CreateEmptyBlocksInterval = 1 * time.Minute
	},
}

// TestnetV1 contains configurations for testnet v1 chain
var TestnetV1 = &ChainInfo{
	Name:        "testnet-v1",
	NetVersion:  2000,
	GenesisTime: 1595700581,
	Validators: []string{
		"47shQ9ihsZBf2nYL6tAYR8q8Twb47KTNjimowxaNFRyGPL93oZL",
		"48LZFEsZsRPda1q2kiNZKToiTaeSx63GJdq6DWq9m9C4mSvWhHD",
		"48pFW5Yd5BLm4EVUJW8g9oG1BkNQz4wp2saLB8XmkvMRwRAB2FH",
		"48GKXaSLgJ5ox2C1jDshFGtD6Y4Zhd1doxK6iTDp3KCSZjzdWKt",
	},
	Configurer: func(cfg *AppConfig, tmc *tmcfg.Config) {
		tmc.Consensus.CreateEmptyBlocksInterval = 10 * time.Minute
	},
}
