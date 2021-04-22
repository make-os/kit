package constants

// Namespace for JS Module and RPC methods
const (
	NamespaceRPC         = "rpc"
	NamespaceNode        = "node"
	NamespaceTx          = "tx"
	NamespacePool        = "pool"
	NamespaceUser        = "user"
	NamespaceRepo        = "repo"
	NamespaceNS          = "ns"
	NamespacePushKey     = "pk"
	NamespaceExtension   = "ext"
	NamespaceDHT         = "dht"
	NamespaceConsoleUtil = "util"
	NamespaceDev         = "dev"
	NamespaceTicket      = "ticket"
	NamespaceHost        = "host"
	NamespaceMiner       = "miner"
)

// Proposal action data keys
const (
	ActionDataKeyBaseBranch    = "b"
	ActionDataKeyBaseHash      = "bh"
	ActionDataKeyTargetBranch  = "t"
	ActionDataKeyTargetHash    = "th"
	ActionDataKeyAddrs         = "ads"
	ActionDataKeyVeto          = "vet"
	ActionDataKeyIDs           = "keys"
	ActionDataKeyPolicies      = "pol"
	ActionDataKeyFeeMode       = "fm"
	ActionDataKeyFeeCap        = "fc"
	ActionDataKeyCFG           = "cfg"
	ActionDataKeyNamespace     = "ns"
	ActionDataKeyNamespaceOnly = "nso"
)

const (
	AddrHRP     = "os"
	PushAddrHRP = "pk"
)
