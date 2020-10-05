package chains

type ChainInfo struct {
	NetVersion  string
	GenesisTime uint64
	Validators  []string
}

// TestnetV1 contains configurations for testnet v1 chain
var TestnetV1 = &ChainInfo{

	// NetVersion is the chain version
	NetVersion: "2000",

	// GenesisTime is the commencement time of the chain
	GenesisTime: 1595700581,

	// Validators are the public keys of the initial validators
	Validators: []string{
		"47shQ9ihsZBf2nYL6tAYR8q8Twb47KTNjimowxaNFRyGPL93oZL",
		"48LZFEsZsRPda1q2kiNZKToiTaeSx63GJdq6DWq9m9C4mSvWhHD",
		"48pFW5Yd5BLm4EVUJW8g9oG1BkNQz4wp2saLB8XmkvMRwRAB2FH",
		"48GKXaSLgJ5ox2C1jDshFGtD6Y4Zhd1doxK6iTDp3KCSZjzdWKt",
	},
}
