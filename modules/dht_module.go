package modules

import (
	"context"
	"fmt"
	"time"

	"gitlab.com/makeos/mosdef/config"
	dht2 "gitlab.com/makeos/mosdef/dht"
	"gitlab.com/makeos/mosdef/remote/types"
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/types/modules"

	"gitlab.com/makeos/mosdef/util"

	"github.com/c-bata/go-prompt"
	"github.com/robertkrimen/otto"
)

// DHTModule provides access to the DHT service
type DHTModule struct {
	cfg *config.AppConfig
	vm  *otto.Otto
	dht dht2.DHT
}

// NewDHTModule creates an instance of DHTModule
func NewDHTModule(cfg *config.AppConfig, vm *otto.Otto, dht dht2.DHT) *DHTModule {
	return &DHTModule{
		cfg: cfg,
		vm:  vm,
		dht: dht,
	}
}

func (m *DHTModule) namespacedFuncs() []*modules.ModuleFunc {
	return []*modules.ModuleFunc{
		{
			Name:        "store",
			Value:       m.Store,
			Description: "Register a value that correspond to a given key",
		},
		{
			Name:        "lookup",
			Value:       m.Lookup,
			Description: "Get a record that correspond to a given key",
		},
		{
			Name:        "announce",
			Value:       m.Announce,
			Description: "Inform the network that this node can provide value for a key",
		},
		{
			Name:        "getProviders",
			Value:       m.GetProviders,
			Description: "Get providers for a given key",
		},
		{
			Name:        "getRepoObject",
			Value:       m.GetRepoObject,
			Description: "Get and return a repo object",
		},
		{
			Name:        "getPeers",
			Value:       m.GetPeers,
			Description: "Returns a list of all DHT peers",
		},
	}
}

func (m *DHTModule) globals() []*modules.ModuleFunc {
	return []*modules.ModuleFunc{}
}

// Configure configures the JS context and return
// any number of console prompt suggestions
func (m *DHTModule) Configure() []prompt.Suggest {
	fMap := map[string]interface{}{}
	var suggestions []prompt.Suggest

	// Set the namespace object
	util.VMSet(m.vm, constants.NamespaceDHT, fMap)

	// add namespaced functions
	for _, f := range m.namespacedFuncs() {
		fMap[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", constants.NamespaceDHT, f.Name)
		suggestions = append(suggestions, prompt.Suggest{Text: funcFullName,
			Description: f.Description})
	}

	// Register global functions
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
func (m *DHTModule) Store(key string, val string) {
	ctx, cn := context.WithTimeout(context.Background(), 15*time.Second)
	defer cn()
	if err := m.dht.Store(ctx, key, []byte(val)); err != nil {
		panic(util.NewStatusError(500, StatusCodeAppErr, "key", err.Error()))
	}
}

// lookup finds a value for a given key
//
// ARGS:
// key: The data query key
//
// RETURNS: <[]bytes> - The data stored on the key
func (m *DHTModule) Lookup(key string) interface{} {
	ctx, cn := context.WithTimeout(context.Background(), 15*time.Second)
	defer cn()
	bz, err := m.dht.Lookup(ctx, key)
	if err != nil {
		panic(util.NewStatusError(500, StatusCodeAppErr, "key", err.Error()))
	}
	return bz
}

// announce announces to the network that the node can provide value for a given key
//
// ARGS:
// - key: The data query key
func (m *DHTModule) Announce(key string) {
	ctx, cn := context.WithTimeout(context.Background(), 15*time.Second)
	defer cn()
	if err := m.dht.Announce(ctx, []byte(key)); err != nil {
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
func (m *DHTModule) GetProviders(key string) (res []map[string]interface{}) {
	ctx, cn := context.WithTimeout(context.Background(), 15*time.Second)
	defer cn()
	peers, err := m.dht.GetProviders(ctx, []byte(key))
	if err != nil {
		panic(util.NewStatusError(500, StatusCodeAppErr, "key", err.Error()))
	}
	for _, p := range peers {
		var address []string
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
func (m *DHTModule) GetRepoObject(objURI string) []byte {
	ctx, cn := context.WithTimeout(context.Background(), 15*time.Second)
	defer cn()
	bz, err := m.dht.GetObject(ctx, &dht2.DHTObjectQuery{
		Module:    types.RepoObjectModule,
		ObjectKey: []byte(objURI),
	})
	if err != nil {
		panic(util.NewStatusError(500, StatusCodeAppErr, "", err.Error()))
	}

	return bz
}

// getPeers returns a list of all connected peers
func (m *DHTModule) GetPeers() []string {
	peers := m.dht.Peers()
	if len(peers) == 0 {
		return []string{}
	}
	return peers
}
