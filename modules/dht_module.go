package modules

import (
	"context"
	"fmt"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/make-os/lobe/api/rpc/client"
	"github.com/make-os/lobe/config"
	"github.com/make-os/lobe/dht"
	"github.com/make-os/lobe/dht/server/types"
	modulestypes "github.com/make-os/lobe/modules/types"
	"github.com/make-os/lobe/remote/plumbing"
	"github.com/make-os/lobe/types/constants"
	"github.com/make-os/lobe/util"

	"github.com/c-bata/go-prompt"
	"github.com/robertkrimen/otto"
)

// DHTModule provides access to the DHT service
type DHTModule struct {
	modulestypes.ModuleCommon
	cfg *config.AppConfig
	dht types.DHT
}

// NewAttachableDHTModule creates an instance of DHTModule suitable in attach mode
func NewAttachableDHTModule(client client.Client) *DHTModule {
	return &DHTModule{ModuleCommon: modulestypes.ModuleCommon{AttachedClient: client}}
}

// NewDHTModule creates an instance of DHTModule
func NewDHTModule(cfg *config.AppConfig, dht types.DHT) *DHTModule {
	return &DHTModule{cfg: cfg, dht: dht}
}

// methods are functions exposed in the special namespace of this module.
func (m *DHTModule) methods() []*modulestypes.VMMember {
	return []*modulestypes.VMMember{
		{
			Name:        "store",
			Value:       m.Store,
			Description: "Store a value for a given key",
		},
		{
			Name:        "lookup",
			Value:       m.Lookup,
			Description: "Get a record that correspond to a given key",
		},
		{
			Name:        "announce",
			Value:       m.Announce,
			Description: "Announce ability to provide a key to the network",
		},
		{
			Name:        "getRepoObjectProviders",
			Value:       m.GetRepoObjectProviders,
			Description: "Get providers of a given repository object",
		},
		{
			Name:        "getProviders",
			Value:       m.GetProviders,
			Description: "Get providers for a given key",
		},
		{
			Name:        "getPeers",
			Value:       m.GetPeers,
			Description: "Returns a list of all DHT peers",
		},
	}
}

// globals are functions exposed in the VM's global namespace
func (m *DHTModule) globals() []*modulestypes.VMMember {
	return []*modulestypes.VMMember{}
}

// ConfigureVM configures the JS context and return
// any number of console prompt suggestions
func (m *DHTModule) ConfigureVM(vm *otto.Otto) prompt.Completer {

	// Set the namespace object
	nsMap := map[string]interface{}{}
	util.VMSet(vm, constants.NamespaceDHT, nsMap)

	// add methods functions
	for _, f := range m.methods() {
		nsMap[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", constants.NamespaceDHT, f.Name)
		m.Suggestions = append(m.Suggestions, prompt.Suggest{Text: funcFullName, Description: f.Description})
	}

	// Register global functions
	for _, f := range m.globals() {
		vm.Set(f.Name, f.Value)
		m.Suggestions = append(m.Suggestions, prompt.Suggest{Text: f.Name, Description: f.Description})
	}

	return m.Completer
}

// store stores a value corresponding to the given key
//
// ARGS:
// key: The data query key
// val: The data to be stored
func (m *DHTModule) Store(key string, val string) {
	ctx, cn := context.WithTimeout(context.Background(), 60*time.Second)
	defer cn()
	if err := m.dht.Store(ctx, key, []byte(val)); err != nil {
		panic(util.ReqErr(500, StatusCodeServerErr, "key", err.Error()))
	}
}

// lookup finds a value for a given key
//
// ARGS:
// key: The data query key
//
// RETURNS: <[]bytes> - The data stored on the key
func (m *DHTModule) Lookup(key string) interface{} {
	ctx, cn := context.WithTimeout(context.Background(), 60*time.Second)
	defer cn()
	bz, err := m.dht.Lookup(ctx, key)
	if err != nil {
		panic(util.ReqErr(500, StatusCodeServerErr, "key", err.Error()))
	}
	return bz
}

// announce announces to the network that the node can provide value for a given key
//
// ARGS:
// key: The data query key
func (m *DHTModule) Announce(key string) {
	m.dht.Announce([]byte(key), nil)
}

// GetRepoObjectProviders returns the providers for a given repo object
//
// ARGS:
// hash: The repo object's hash or DHT object hex-encoded key
//
// RETURNS: resp <[]map[string]interface{}>
// resp.id <string>: The peer ID of the provider
// resp.addresses	<[]string>: A list of p2p multiaddrs of the provider
func (m *DHTModule) GetRepoObjectProviders(hash string) (res []map[string]interface{}) {

	var err error
	var key []byte

	// A key is valid if it is a git SHA1 or a DHT hex-encoded object key
	if govalidator.IsSHA1(hash) {
		key = dht.MakeObjectKey(plumbing.HashToBytes(hash))
	} else {
		key, err = util.FromHex(hash)
		if err != nil {
			panic(util.ReqErr(400, StatusCodeInvalidParam, "hash", "invalid object key"))
		}
	}

	ctx, cn := context.WithTimeout(context.Background(), 60*time.Second)
	defer cn()
	peers, err := m.dht.GetProviders(ctx, key)
	if err != nil {
		panic(util.ReqErr(500, StatusCodeServerErr, "key", err.Error()))
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

// GetProviders returns the providers for a given key
//
// ARGS:
// hash: The data key
//
// RETURNS: resp <[]map[string]interface{}>
// resp.id <string>: The peer ID of the provider
// resp.addresses	<[]string>: A list of p2p multiaddrs of the provider
func (m *DHTModule) GetProviders(key string) (res []map[string]interface{}) {
	ctx, cn := context.WithTimeout(context.Background(), 60*time.Second)
	defer cn()
	peers, err := m.dht.GetProviders(ctx, []byte(key))
	if err != nil {
		panic(util.ReqErr(500, StatusCodeServerErr, "key", err.Error()))
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

// getPeers returns a list of all connected peers
func (m *DHTModule) GetPeers() (peers []string) {
	peers = m.dht.Peers()
	return
}
