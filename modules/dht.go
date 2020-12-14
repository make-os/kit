package modules

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/make-os/kit/config"
	"github.com/make-os/kit/dht"
	"github.com/make-os/kit/dht/announcer"
	modulestypes "github.com/make-os/kit/modules/types"
	"github.com/make-os/kit/remote/plumbing"
	types2 "github.com/make-os/kit/rpc/types"
	"github.com/make-os/kit/types/constants"
	"github.com/make-os/kit/util"

	"github.com/c-bata/go-prompt"
	"github.com/robertkrimen/otto"
)

// DHTModule provides access to the DHT service
type DHTModule struct {
	modulestypes.ModuleCommon
	cfg *config.AppConfig
	dht dht.DHT
}

// NewAttachableDHTModule creates an instance of DHTModule suitable in attach mode
func NewAttachableDHTModule(cfg *config.AppConfig, client types2.Client) *DHTModule {
	return &DHTModule{ModuleCommon: modulestypes.ModuleCommon{Client: client}, cfg: cfg}
}

// NewDHTModule creates an instance of DHTModule
func NewDHTModule(cfg *config.AppConfig, dht dht.DHT) *DHTModule {
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
func (m *DHTModule) Store(key string, val string) {

	if m.IsAttached() {
		if err := m.Client.DHT().Store(key, val); err != nil {
			panic(err)
		}
		return
	}

	ctx, cn := context.WithTimeout(context.Background(), 60*time.Second)
	defer cn()
	if err := m.dht.Store(ctx, dht.MakeKey(key), []byte(val)); err != nil {
		panic(util.ReqErr(500, StatusCodeServerErr, "key", err.Error()))
	}
}

// lookup finds a value for a given key
//
// RETURNS <base64 string>: The data stored on the key
func (m *DHTModule) Lookup(key string) string {

	if m.IsAttached() {
		val, err := m.Client.DHT().Lookup(key)
		if err != nil {
			panic(err)
		}
		return base64.StdEncoding.EncodeToString([]byte(val))
	}

	ctx, cn := context.WithTimeout(context.Background(), 60*time.Second)
	defer cn()
	bz, err := m.dht.Lookup(ctx, dht.MakeKey(key))
	if err != nil {
		panic(util.ReqErr(500, StatusCodeServerErr, "key", err.Error()))
	}
	return base64.StdEncoding.EncodeToString(bz)
}

// announce announces to the network that the node can provide value for a given key
func (m *DHTModule) Announce(key string) {

	if m.IsAttached() {
		if err := m.Client.DHT().Announce(key); err != nil {
			panic(err)
		}
		return
	}

	m.dht.Announce(announcer.ObjTypeAny, "", []byte(key), nil)
}

// GetRepoObjectProviders returns the providers for a given repo object
//
// ARGS:
// hash: The repo object's hash or DHT object hex-encoded key
//
// RETURNS: resp <[]map[string]interface{}>
// - resp.id <string>: The peer ID of the provider
// - resp.addresses: <[]string>: A list of p2p multiaddrs of the provider
func (m *DHTModule) GetRepoObjectProviders(hash string) (res []util.Map) {

	if m.IsAttached() {
		res, err := m.Client.DHT().GetRepoObjectProviders(hash)
		if err != nil {
			panic(err)
		}
		return util.StructSliceToMap(res)
	}

	var err error
	var key []byte

	// A key is valid if it is a git SHA1 or a DHT hex-encoded object key
	if govalidator.IsSHA1(hash) {
		key = plumbing.HashToBytes(hash)
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
// - resp.id <string>: The peer ID of the provider
// - resp.addresses: <[]string>: A list of p2p multiaddrs of the provider
func (m *DHTModule) GetProviders(key string) (res []util.Map) {

	if m.IsAttached() {
		res, err := m.Client.DHT().GetProviders(key)
		if err != nil {
			panic(err)
		}
		return util.StructSliceToMap(res)
	}

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

// getPeers returns a list of DHT peer IDs
func (m *DHTModule) GetPeers() (peers []string) {
	if m.IsAttached() {
		res, err := m.Client.DHT().GetPeers()
		if err != nil {
			panic(err)
		}
		return res
	}

	return m.dht.Peers()
}
