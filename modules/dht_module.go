package modules

import (
	"context"
	"fmt"

	"gitlab.com/makeos/mosdef/config"
	types2 "gitlab.com/makeos/mosdef/dht/types"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"

	"gitlab.com/makeos/mosdef/util"

	prompt "github.com/c-bata/go-prompt"
	"github.com/robertkrimen/otto"
	moduletypes "gitlab.com/makeos/mosdef/modules/types"
)

// DHTModule provides gpg key management functionality
type DHTModule struct {
	cfg *config.AppConfig
	vm  *otto.Otto
	dht types2.DHTNode
}

// NewDHTModule creates an instance of DHTModule
func NewDHTModule(cfg *config.AppConfig, vm *otto.Otto, dht types2.DHTNode) *DHTModule {
	return &DHTModule{
		cfg: cfg,
		vm:  vm,
		dht: dht,
	}
}

func (m *DHTModule) namespacedFuncs() []*moduletypes.ModulesAggregatorFunc {
	return []*moduletypes.ModulesAggregatorFunc{
		{
			Name:        "store",
			Value:       m.store,
			Description: "Add a value that correspond to a given key",
		},
		{
			Name:        "lookup",
			Value:       m.lookup,
			Description: "Find a record that correspond to a given key",
		},
		{
			Name:        "announce",
			Value:       m.announce,
			Description: "Inform the network that this node can provide value for a key",
		},
		{
			Name:        "getProviders",
			Value:       m.getProviders,
			Description: "Get providers for a given key",
		},
		{
			Name:        "getRepoObject",
			Value:       m.getRepoObject,
			Description: "Find and return a repo object",
		},
		{
			Name:        "getPeers",
			Value:       m.getPeers,
			Description: "Returns a list of all DHTNode peers",
		},
	}
}

func (m *DHTModule) globals() []*moduletypes.ModulesAggregatorFunc {
	return []*moduletypes.ModulesAggregatorFunc{}
}

// Configure configures the JS context and return
// any number of console prompt suggestions
func (m *DHTModule) Configure() []prompt.Suggest {
	fMap := map[string]interface{}{}
	suggestions := []prompt.Suggest{}

	// Set the namespace object
	util.VMSet(m.vm, types.NamespaceDHT, fMap)

	// add namespaced functions
	for _, f := range m.namespacedFuncs() {
		fMap[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", types.NamespaceDHT, f.Name)
		suggestions = append(suggestions, prompt.Suggest{Text: funcFullName,
			Description: f.Description})
	}

	// Add global functions
	for _, f := range m.globals() {
		m.vm.Set(f.Name, f.Value)
		suggestions = append(suggestions, prompt.Suggest{Text: f.Name,
			Description: f.Description})
	}

	return suggestions
}

// store stores a value corresponding to the given key
//
// ARGS:
// key: The data query key
// val: The data to be stored
func (m *DHTModule) store(key string, val string) {
	if err := m.dht.Store(context.Background(), key, []byte(val)); err != nil {
		panic(util.NewStatusError(500, StatusCodeAppErr, "key", err.Error()))
	}
}

// lookup finds a value for a given key
//
// ARGS:
// key: The data query key
//
// RETURNS: <[]bytes> - The data stored on the key
func (m *DHTModule) lookup(key string) interface{} {
	bz, err := m.dht.Lookup(context.Background(), key)
	if err != nil {
		panic(util.NewStatusError(500, StatusCodeAppErr, "key", err.Error()))
	}
	return bz
}

// announce announces to the network that the node can provide value for a given key
//
// ARGS:
// - key: The data query key
func (m *DHTModule) announce(key string) {
	if err := m.dht.Announce(context.Background(), []byte(key)); err != nil {
		panic(util.NewStatusError(500, StatusCodeAppErr, "key", err.Error()))
	}
}

// getProviders returns the providers for a given key
//
// ARGS:
// key: The data query key
//
// RETURNS: resp <[]map[string]interface{}>
// resp.id <string>: The libp2p ID of the provider
// resp.addresses	<[]string>: A list of p2p multiaddrs of the provider
func (m *DHTModule) getProviders(key string) (res []map[string]interface{}) {
	peers, err := m.dht.GetProviders(context.Background(), []byte(key))
	if err != nil {
		panic(util.NewStatusError(500, StatusCodeAppErr, "key", err.Error()))
	}
	for _, p := range peers {
		address := []string{}
		for _, addr := range p.Addrs {
			address = append(address, addr.String())
		}
		res = append(res, map[string]interface{}{
			"id":        p.ID.String(),
			"addresses": address,
		})
	}
	return
}

// getRepoObject finds a repository object from a provider
//
// ARGS:
// objURI: The repo object URI
func (m *DHTModule) getRepoObject(objURI string) []byte {
	bz, err := m.dht.GetObject(context.Background(), &types2.DHTObjectQuery{
		Module:    core.RepoObjectModule,
		ObjectKey: []byte(objURI),
	})
	if err != nil {
		panic(util.NewStatusError(500, StatusCodeAppErr, "", err.Error()))
	}

	return bz
}

// getPeers returns a list of all connected peers
func (m *DHTModule) getPeers() []string {
	peers := m.dht.Peers()
	if len(peers) == 0 {
		return []string{}
	}
	return peers
}
