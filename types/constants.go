package types

// Namespace for JS Module and RPC methods
const (
	NamespaceRPC       = "rpc"
	NamespaceNode      = "node"
	NamespaceTx        = "tx"
	NamespaceCoin      = "coin"
	NamespacePool      = "pool"
	NamespaceUser      = "user"
	NamespaceRepo      = "repo"
	NamespaceNS        = "ns"
	NamespaceAccount   = "keystore"
	NamespaceGPG       = "gpg"
	NamespaceExtension = "ext"
	NamespaceDHT       = "dht"
	NamespaceUtil      = "util"
	NamespaceTicket    = "ticket"
	NamespaceHost      = "host"
)

// ABCI related events
const (
	EvtABCICommittedTx = "abci_delivered_valid_tx"
)

// Proposal action data keys
const (
	ActionDataKeyBaseBranch    = "b"
	ActionDataKeyBaseHash      = "bh"
	ActionDataKeyTargetBranch  = "t"
	ActionDataKeyTargetHash    = "th"
	ActionDataKeyAddrs         = "ads"
	ActionDataKeyVeto          = "vet"
	ActionDataKeyIDs           = "ids"
	ActionDataKeyPolicies      = "pol"
	ActionDataKeyFeeMode       = "fm"
	ActionDataKeyFeeCap        = "fc"
	ActionDataKeyCFG           = "cfg"
	ActionDataKeyNamespace     = "ns"
	ActionDataKeyNamespaceOnly = "nso"
)
