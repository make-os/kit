package api

import (
	modtypes "github.com/make-os/kit/modules/types"
	"github.com/make-os/kit/rpc"
	"github.com/make-os/kit/types/constants"
	"github.com/make-os/kit/util"
	"github.com/spf13/cast"
	"github.com/stretchr/objx"
)

// DHTAPI provides APIs for accessing the DHT service
type DHTAPI struct {
	mods *modtypes.Modules
}

// NewDHTAPI creates an instance of DHTAPI
func NewDHTAPI(mods *modtypes.Modules) *DHTAPI {
	return &DHTAPI{mods}
}

// getPeers returns a list of connected DHT peer IDs
func (c *DHTAPI) getPeers(params interface{}) (resp *rpc.Response) {
	return rpc.Success(util.Map{
		"peers": c.mods.DHT.GetPeers(),
	})
}

// getProviders returns a list of connected DHT peer IDs
func (c *DHTAPI) getProviders(params interface{}) (resp *rpc.Response) {
	return rpc.Success(util.Map{
		"providers": c.mods.DHT.GetProviders(cast.ToString(params)),
	})
}

// announce announces a key
func (c *DHTAPI) announce(params interface{}) (resp *rpc.Response) {
	c.mods.DHT.Announce(cast.ToString(params))
	return rpc.StatusOK()
}

// getRepoObjectProviders gets providers of a given repository object
func (c *DHTAPI) getRepoObjectProviders(params interface{}) (resp *rpc.Response) {
	return rpc.Success(util.Map{
		"providers": c.mods.DHT.GetRepoObjectProviders(cast.ToString(params)),
	})
}

// store stores a value under the given key on the DHT
func (c *DHTAPI) store(params interface{}) (resp *rpc.Response) {
	o := objx.New(cast.ToStringMap(params))
	c.mods.DHT.Store(o.Get("key").Str(), o.Get("value").Str())
	return rpc.StatusOK()
}

// lookup finds a value stored under the given key
func (c *DHTAPI) lookup(params interface{}) (resp *rpc.Response) {
	return rpc.Success(util.Map{
		"value": c.mods.DHT.Lookup(cast.ToString(params)),
	})
}

// APIs returns all API handlers
func (c *DHTAPI) APIs() rpc.APISet {
	return []rpc.MethodInfo{
		{
			Name:      "getPeers",
			Namespace: constants.NamespaceDHT,
			Desc:      "Get a list of connected DHT peer IDs",
			Func:      c.getPeers,
		},
		{
			Name:      "getProviders",
			Namespace: constants.NamespaceDHT,
			Desc:      "Get a list of providers for a given key",
			Func:      c.getProviders,
		},
		{
			Name:      "announce",
			Namespace: constants.NamespaceDHT,
			Desc:      "Announce a key to the network",
			Func:      c.announce,
			Private:   true,
		},
		{
			Name:      "getRepoObjectProviders",
			Namespace: constants.NamespaceDHT,
			Desc:      "Get providers of a given repository object",
			Func:      c.getRepoObjectProviders,
		},
		{
			Name:      "store",
			Namespace: constants.NamespaceDHT,
			Desc:      "Stores a key/value pair on the DHTt",
			Func:      c.store,
			Private:   true,
		},
		{
			Name:      "lookup",
			Namespace: constants.NamespaceDHT,
			Desc:      "Look up the value of a key",
			Func:      c.lookup,
		},
	}
}
