package rpc

import (
	modtypes "github.com/make-os/lobe/modules/types"
	"github.com/make-os/lobe/rpc"
	"github.com/make-os/lobe/types/constants"
	"github.com/make-os/lobe/util"
	"github.com/spf13/cast"
	"github.com/stretchr/objx"
)

// NamespaceAPI provides APIs for accessing the DHT service
type NamespaceAPI struct {
	mods *modtypes.Modules
}

// NewNamespaceAPI creates an instance of NamespaceAPI
func NewNamespaceAPI(mods *modtypes.Modules) *NamespaceAPI {
	return &NamespaceAPI{mods}
}

// register returns a list of connected DHT peer IDs
func (c *NamespaceAPI) register(params interface{}) (resp *rpc.Response) {
	return rpc.Success(c.mods.NS.Register(cast.ToStringMap(params)))
}

// updateDomain updates one or more domains of a namespace
func (c *NamespaceAPI) updateDomain(params interface{}) (resp *rpc.Response) {
	return rpc.Success(c.mods.NS.UpdateDomain(cast.ToStringMap(params)))
}

// getTarget gets the target of a namespace URI
func (a *NamespaceAPI) getTarget(params interface{}) (resp *rpc.Response) {
	o := objx.New(params)
	uri := o.Get("uri").Str()
	blockHeight := cast.ToUint64(o.Get("height").Inter())
	target := a.mods.NS.GetTarget(uri, blockHeight)
	return rpc.Success(util.Map{"target": target})
}

// lookup finds a namespace
func (a *NamespaceAPI) lookup(params interface{}) (resp *rpc.Response) {
	o := objx.New(params)
	name := o.Get("name").Str()
	blockHeight := cast.ToUint64(o.Get("height").Inter())
	return rpc.Success(a.mods.NS.Lookup(name, blockHeight))
}

// APIs returns all API handlers
func (c *NamespaceAPI) APIs() rpc.APISet {
	return []rpc.APIInfo{
		{
			Name:        "register",
			Namespace:   constants.NamespaceNS,
			Description: "Register a namespace",
			Func:        c.register,
		},
		{
			Name:        "updateDomain",
			Namespace:   constants.NamespaceNS,
			Description: "Update one or more domains of a namespace",
			Func:        c.updateDomain,
		},
		{
			Name:        "getTarget",
			Namespace:   constants.NamespaceNS,
			Description: "Get the target of a namespace URI",
			Func:        c.getTarget,
		},
		{
			Name:        "lookup",
			Namespace:   constants.NamespaceNS,
			Description: "Find a namespace by its name",
			Func:        c.lookup,
		},
	}
}
