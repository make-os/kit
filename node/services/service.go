package services

// Service provides operation that access external logic not
// directly offered by any of the packages of the project.
// For instance, access to tendermint RPC is provided.
type Service interface {
	GetBlock(height int64) (map[string]interface{}, error)
}

// NodeService implements types.Service. It provides node specific
// operations that can be used by the JS module, RPC APIs etc
type NodeService struct {
	tmrpc *TMRPC
}

// New creates an instance of NodeService
func New(tmAddress string) *NodeService {
	return &NodeService{
		tmrpc: newTMRPC(tmAddress),
	}
}
