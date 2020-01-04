package keepers

import (
	"github.com/makeos/mosdef/storage/tree"
	"github.com/pkg/errors"

	"github.com/makeos/mosdef/types"
)

// NamespaceKeeper manages namespaces.
type NamespaceKeeper struct {
	state *tree.SafeTree
}

// NewNamespaceKeeper creates an instance of NamespaceKeeper
func NewNamespaceKeeper(state *tree.SafeTree) *NamespaceKeeper {
	return &NamespaceKeeper{state: state}
}

// GetNamespace finds a namespace by name.
//
// ARGS:
// name: The name of the namespace to find.
// blockNum: The target block to query (Optional. Default: latest)
//
// CONTRACT: It returns an empty Namespace if no matching namespace is found.
func (a *NamespaceKeeper) GetNamespace(name string, blockNum ...uint64) *types.Namespace {

	// Get version is provided
	var version uint64
	if len(blockNum) > 0 && blockNum[0] > 0 {
		version = blockNum[0]
	}

	// Query the namespace by key. If version is provided, we do a versioned
	// query, otherwise we query the latest.
	key := MakeNamespaceKey(name)
	var bs []byte
	if version != 0 {
		_, bs = a.state.GetVersioned(key, int64(version))
	} else {
		_, bs = a.state.Get(key)
	}

	// If we don't find the namespace, we return an empty namespace.
	if bs == nil {
		return types.BareNamespace()
	}

	// Otherwise, we decode the bytes to types.Namespace
	ns, err := types.NewNamespaceFromBytes(bs)
	if err != nil {
		panic(errors.Wrap(err, "failed to decode namespace byte slice"))
	}

	return ns
}

// Update sets a new object at the given name.
//
// ARGS:
// name: The name of the namespace to update
// udp: The updated namespace object to replace the existing object.
func (a *NamespaceKeeper) Update(name string, upd *types.Namespace) {
	a.state.Set(MakeNamespaceKey(name), upd.Bytes())
}
