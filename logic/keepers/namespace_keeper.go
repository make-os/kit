package keepers

import (
	"fmt"

	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/pkgs/tree"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/util"
)

// NamespaceKeeper manages namespaces.
type NamespaceKeeper struct {
	state *tree.SafeTree
}

// NewNamespaceKeeper creates an instance of NamespaceKeeper
func NewNamespaceKeeper(state *tree.SafeTree) *NamespaceKeeper {
	return &NamespaceKeeper{state: state}
}

// Get finds a namespace by name.
// ARGS:
// name: The name of the namespace to find.
// blockNum: The target block to query (Optional. Default: latest)
//
// CONTRACT: It returns an empty Namespace if no matching namespace is found.
func (a *NamespaceKeeper) Get(name string, blockNum ...uint64) *state.Namespace {

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
		return state.BareNamespace()
	}

	// Otherwise, we decode the bytes to types.Namespace
	ns, err := state.NewNamespaceFromBytes(bs)
	if err != nil {
		panic(errors.Wrap(err, "failed to decode namespace byte slice"))
	}

	return ns
}

// GetTarget looks up the target of a full namespace path
// ARGS:
// path: The path to look up.
// blockNum: The target block to query (Optional. Default: latest)
func (a *NamespaceKeeper) GetTarget(path string, blockNum ...uint64) (string, error) {

	// Get version is provided
	var version uint64
	if len(blockNum) > 0 && blockNum[0] > 0 {
		version = blockNum[0]
	}

	namespace, domain, err := util.SplitNamespaceDomain(path)
	if err != nil {
		return "", err
	}

	actualName := util.Hash20Hex([]byte(namespace))
	ns := a.Get(actualName, version)
	if ns.IsNil() {
		return "", fmt.Errorf("namespace not found")
	}

	target := ns.Domains.Get(domain)
	if target == "" {
		return "", fmt.Errorf("domain not found")
	}

	return target, nil
}

// Update sets a new object at the given name.
// ARGS:
// name: The name of the namespace to update
// udp: The updated namespace object to replace the existing object.
func (a *NamespaceKeeper) Update(name string, upd *state.Namespace) {
	a.state.Set(MakeNamespaceKey(name), upd.Bytes())
}
