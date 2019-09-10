package types

const (
	// NamespaceState is the namespace
	// for RPC methods that access the database
	NamespaceState = "state"

	// NamespaceEll is the namespace for RPC methods
	// that interact with the native currency
	NamespaceEll = "ell"

	// NamespaceNode is the namespace for RPC methods
	// that interact and access the node/client properties
	NamespaceNode = "node"

	// NamespacePool is the namespace for RPC methods
	// that access the transaction pool
	NamespacePool = "pool"

	// NamespaceNet is the namespace for RPC methods
	// that perform network actions
	NamespaceNet = "net"

	// NamespaceRPC is the namespace for RPC methods
	// that perform rpc actions
	NamespaceRPC = "rpc"

	// NamespaceLogger is the namespace for RPC methods
	// for configuring the logger
	NamespaceLogger = "logger"

	// NamespaceDebug is the namespace for RPC methods
	// that offer debugging features
	NamespaceDebug = "debug"
)
