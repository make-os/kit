package api

import (
	modulestypes "github.com/make-os/kit/modules/types"
	"github.com/make-os/kit/rpc"
	"github.com/make-os/kit/types/constants"
	"github.com/make-os/kit/util"
	"github.com/spf13/cast"
	"github.com/stretchr/objx"
)

// TicketAPI provides RPC methods for purchasing and managing tickets.
type TicketAPI struct {
	mods *modulestypes.Modules
}

// NewTicketAPI creates an instance of TicketAPI
func NewTicketAPI(mods *modulestypes.Modules) *TicketAPI {
	return &TicketAPI{mods: mods}
}

// buy purchases a validator ticket
func (a *TicketAPI) buy(params interface{}) (resp *rpc.Response) {
	return rpc.Success(a.mods.Ticket.BuyValidatorTicket(cast.ToStringMap(params)))
}

// buyHostTicket purchases a host ticket
func (a *TicketAPI) buyHostTicket(params interface{}) (resp *rpc.Response) {
	return rpc.Success(a.mods.Ticket.BuyHostTicket(cast.ToStringMap(params)))
}

// list returns active validator tickets associated with the given proposer public key
func (a *TicketAPI) list(params interface{}) (resp *rpc.Response) {
	var m = objx.New(cast.ToStringMap(params))
	proposerPubKey := m.Get("proposer").Str()
	qo := m.Get("queryOpts").MSI()
	return rpc.Success(util.Map{
		"tickets": a.mods.Ticket.GetValidatorTicketsByProposer(proposerPubKey, qo),
	})
}

// listHost returns active hosts tickets associated with the given proposer public key
func (a *TicketAPI) listHost(params interface{}) (resp *rpc.Response) {
	var m = objx.New(cast.ToStringMap(params))
	proposerPubKey := m.Get("proposer").Str()
	qo := m.Get("queryOpts").MSI()
	return rpc.Success(util.Map{
		"tickets": a.mods.Ticket.GetHostTicketsByProposer(proposerPubKey, qo),
	})
}

// getTopValidators returns the top validator tickets
func (a *TicketAPI) getTopValidators(params interface{}) (resp *rpc.Response) {
	return rpc.Success(util.Map{
		"tickets": a.mods.Ticket.GetTopValidators(cast.ToInt(params)),
	})
}

// getTopHosts returns the top host tickets
func (a *TicketAPI) getTopHosts(params interface{}) (resp *rpc.Response) {
	return rpc.Success(util.Map{
		"tickets": a.mods.Ticket.GetTopHosts(cast.ToInt(params)),
	})
}

// getStats gets ticket statistics
func (a *TicketAPI) getStats(params interface{}) (resp *rpc.Response) {
	return rpc.Success(a.mods.Ticket.GetStats(cast.ToString(params)))
}

// getAll gets all validator and host tickets
func (a *TicketAPI) getAll(params interface{}) (resp *rpc.Response) {
	return rpc.Success(util.Map{
		"tickets": a.mods.Ticket.GetAll(cast.ToInt(params)),
	})
}

// unbondHost unbonds a host ticket
func (a *TicketAPI) unbondHost(params interface{}) (resp *rpc.Response) {
	return rpc.Success(a.mods.Ticket.UnbondHostTicket(cast.ToStringMap(params)))
}

// APIs returns all API handlers
func (a *TicketAPI) APIs() rpc.APISet {
	return []rpc.MethodInfo{
		{
			Name:      "buy",
			Namespace: constants.NamespaceTicket,
			Func:      a.buy,
			Desc:      "Purchase a validator ticket",
		},
		{
			Name:      "buyHost",
			Namespace: constants.NamespaceTicket,
			Func:      a.buyHostTicket,
			Desc:      "Purchase a host ticket",
		},
		{
			Name:      "list",
			Namespace: constants.NamespaceTicket,
			Func:      a.list,
			Desc:      "List active validator tickets associated with a proposer",
		},
		{
			Name:      "listHost",
			Namespace: constants.NamespaceTicket,
			Func:      a.listHost,
			Desc:      "List active host tickets associated with a proposer",
		},
		{
			Name:      "top",
			Namespace: constants.NamespaceTicket,
			Func:      a.getTopValidators,
			Desc:      "Get the top validator tickets",
		},
		{
			Name:      "topHosts",
			Namespace: constants.NamespaceTicket,
			Func:      a.getTopHosts,
			Desc:      "Get the top host tickets",
		},
		{
			Name:      "getStats",
			Namespace: constants.NamespaceTicket,
			Func:      a.getStats,
			Desc:      "Get ticket statistics",
		},
		{
			Name:      "getAll",
			Namespace: constants.NamespaceTicket,
			Func:      a.getAll,
			Desc:      "Get all validator and host tickets",
		},
		{
			Name:      "unbondHost",
			Namespace: constants.NamespaceTicket,
			Func:      a.unbondHost,
			Desc:      "Unbond a host ticket",
		},
	}
}
