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
	NamespaceAccount   = "account"
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
