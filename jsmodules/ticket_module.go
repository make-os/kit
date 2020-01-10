package jsmodules

import (
	"fmt"

	"github.com/c-bata/go-prompt"
	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/robertkrimen/otto"
)

// TicketModule provides access to various utility functions
type TicketModule struct {
	vm        *otto.Otto
	service   types.Service
	ticketmgr types.TicketManager
	storerObj map[string]interface{}
}

// NewTicketModule creates an instance of TicketModule
func NewTicketModule(
	vm *otto.Otto,
	service types.Service,
	ticketmgr types.TicketManager) *TicketModule {
	return &TicketModule{
		vm:        vm,
		service:   service,
		ticketmgr: ticketmgr,
		storerObj: make(map[string]interface{}),
	}
}

func (m *TicketModule) globals() []*types.JSModuleFunc {
	return []*types.JSModuleFunc{}
}

// funcs exposed by the module
func (m *TicketModule) funcs() []*types.JSModuleFunc {
	return []*types.JSModuleFunc{
		&types.JSModuleFunc{
			Name:        "buy",
			Value:       m.buy,
			Description: "Buy a validator ticket",
		},
		&types.JSModuleFunc{
			Name:        "findByProposer",
			Value:       m.findByProposer,
			Description: "Get tickets associated to a given public key",
		},
		&types.JSModuleFunc{
			Name:        "top",
			Value:       m.top,
			Description: "Get most recent tickets up to the given limit",
		},
	}
}

// storerFuncs are `storer` funcs exposed by the module
func (m *TicketModule) storerFuncs() []*types.JSModuleFunc {
	return []*types.JSModuleFunc{
		&types.JSModuleFunc{
			Name:        "buy",
			Value:       m.storerBuy,
			Description: "Buy an storer ticket",
		},
		&types.JSModuleFunc{
			Name:        "unbond",
			Value:       m.unbondStorerTicket,
			Description: "Unbond the stake associated with a storer ticket",
		},
	}
}

// Configure configures the JS context and return
// any number of console prompt suggestions
func (m *TicketModule) Configure() []prompt.Suggest {
	suggestions := []prompt.Suggest{}

	// Set the namespaces
	ticketObj := map[string]interface{}{"storer": m.storerObj}
	util.VMSet(m.vm, types.NamespaceTicket, ticketObj)
	storerNS := fmt.Sprintf("%s.%s", types.NamespaceTicket, types.NamespaceStorer)

	for _, f := range m.funcs() {
		ticketObj[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", types.NamespaceTicket, f.Name)
		suggestions = append(suggestions, prompt.Suggest{Text: funcFullName,
			Description: f.Description})
	}

	for _, f := range m.storerFuncs() {
		m.storerObj[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", storerNS, f.Name)
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

// buy creates and executes a ticket purchase order
//
// params {
// 		nonce: number,
//		fee: string,
// 		value: string,
//		delegate: string
//		timestamp: number
// }
// options: key
func (m *TicketModule) buy(params map[string]interface{}, options ...interface{}) interface{} {
	var err error

	// Decode parameters into a transaction object
	var tx = types.NewBareTxTicketPurchase(types.TxTypeValidatorTicket)
	mapstructure.Decode(params, tx)

	if nonce, ok := params["nonce"]; ok {
		defer castPanic("nonce")
		tx.Nonce = uint64(nonce.(int64))
	}

	if fee, ok := params["fee"]; ok {
		defer castPanic("fee")
		tx.Fee = util.String(fee.(string))
	}

	if value, ok := params["value"]; ok {
		defer castPanic("value")
		tx.Value = util.String(value.(string))
	}

	delegate := ""
	if to, ok := params["delegate"]; ok {
		defer castPanic("delegate")
		delegate = to.(string)
	}

	if timestamp, ok := params["timestamp"]; ok {
		defer castPanic("timestamp")
		tx.Timestamp = timestamp.(int64)
	}

	if delegate != "" {
		pubKey, err := crypto.PubKeyFromBase58(delegate)
		if err != nil {
			panic(errors.Wrap(err, "failed to decode 'delegate' to public key"))
		}
		tx.Delegate = util.BytesToBytes32(pubKey.MustBytes())
	}

	setCommonTxFields(tx, m.service, options...)

	// Process the transaction
	hash, err := m.service.SendTx(tx)
	if err != nil {
		panic(errors.Wrap(err, "failed to send transaction"))
	}

	return EncodeForJS(map[string]interface{}{
		"hash": hash,
	})
}

// buy creates and executes a ticket purchase order
//
// params {
// 		nonce: number,
//		fee: string,
// 		value: string,
//		delegate: string
//		timestamp: number
// }
// options: key
func (m *TicketModule) storerBuy(params map[string]interface{}, options ...interface{}) interface{} {
	var err error

	// Decode parameters into a transaction object
	var tx = types.NewBareTxTicketPurchase(types.TxTypeStorerTicket)
	mapstructure.Decode(params, tx)

	if nonce, ok := params["nonce"]; ok {
		defer castPanic("nonce")
		tx.Nonce = uint64(nonce.(int64))
	}

	if fee, ok := params["fee"]; ok {
		defer castPanic("fee")
		tx.Fee = util.String(fee.(string))
	}

	if value, ok := params["value"]; ok {
		defer castPanic("value")
		tx.Value = util.String(value.(string))
	}

	delegate := ""
	if to, ok := params["delegate"]; ok {
		defer castPanic("delegate")
		delegate = to.(string)
	}

	if timestamp, ok := params["timestamp"]; ok {
		defer castPanic("timestamp")
		tx.Timestamp = timestamp.(int64)
	}

	if delegate != "" {
		pubKey, err := crypto.PubKeyFromBase58(delegate)
		if err != nil {
			panic(errors.Wrap(err, "failed to decode 'delegate' to public key"))
		}
		tx.Delegate = util.BytesToBytes32(pubKey.MustBytes())
	}

	setCommonTxFields(tx, m.service, options...)

	// Process the transaction
	hash, err := m.service.SendTx(tx)
	if err != nil {
		panic(errors.Wrap(err, "failed to send transaction"))
	}

	return EncodeForJS(map[string]interface{}{
		"hash": hash,
	})
}

// findByProposer finds tickets owned by a given public key
func (m *TicketModule) findByProposer(
	proposerPubKey string,
	queryOpts ...map[string]interface{}) interface{} {

	var qopts types.QueryOptions
	if len(queryOpts) > 0 {
		mapstructure.Decode(queryOpts[0], &qopts)
	}

	// If no sort by height directive, sort by height in descending order
	if qopts.SortByHeight == 0 {
		qopts.SortByHeight = -1
	}

	res, err := m.ticketmgr.GetByProposer(types.TxTypeValidatorTicket, proposerPubKey, qopts)
	if err != nil {
		panic(err)
	}

	return res
}

// top returns most recent tickets up to the given limit
func (m *TicketModule) top(limit int) interface{} {
	res := m.ticketmgr.Query(func(t *types.Ticket) bool { return true }, types.QueryOptions{
		Limit:        limit,
		SortByHeight: -1,
	})
	return res
}

// unbondStorerTicket initiates the release of stake associated with a storer
// ticket
//
// params {
// 		nonce: number,
//		fee: string,
//		hash: string    // ticket hash
//		timestamp: number
// }
// options: key
func (m *TicketModule) unbondStorerTicket(params map[string]interface{},
	options ...interface{}) interface{} {
	var err error

	// Decode parameters into a transaction object
	var tx = types.NewBareTxTicketUnbond(types.TxTypeUnbondStorerTicket)
	mapstructure.Decode(params, tx)

	if nonce, ok := params["nonce"]; ok {
		defer castPanic("nonce")
		tx.Nonce = uint64(nonce.(int64))
	}

	if fee, ok := params["fee"]; ok {
		defer castPanic("fee")
		tx.Fee = util.String(fee.(string))
	}

	if ticketHash, ok := params["hash"]; ok {
		defer castPanic("hash")
		tx.TicketHash = ticketHash.(string)
	}

	if timestamp, ok := params["timestamp"]; ok {
		defer castPanic("timestamp")
		tx.Timestamp = timestamp.(int64)
	}

	setCommonTxFields(tx, m.service, options...)

	// Process the transaction
	hash, err := m.service.SendTx(tx)
	if err != nil {
		panic(errors.Wrap(err, "failed to send transaction"))
	}

	return EncodeForJS(map[string]interface{}{
		"hash": hash,
	})
}
