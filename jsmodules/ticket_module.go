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
	"github.com/shopspring/decimal"
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
			Name:        "listValidatorTicketsOfProposer",
			Value:       m.listValidatorTicketsOfProposer,
			Description: "List validator tickets where given public key is the proposer",
		},
		&types.JSModuleFunc{
			Name:        "listStorerTicketsOfProposer",
			Value:       m.listStorerTicketsOfProposer,
			Description: "List storer tickets where given public key is the proposer",
		},
		&types.JSModuleFunc{
			Name:        "listRecent",
			Value:       m.listRecent,
			Description: "List most recent tickets up to the given limit",
		},
		&types.JSModuleFunc{
			Name:        "stats",
			Value:       m.ticketStats,
			Description: "Get ticket stats of network and a public key",
		},
		&types.JSModuleFunc{
			Name:        "listTopValidators",
			Value:       m.listTopValidators,
			Description: "List tickets of top network validators up to the given limit",
		},
		&types.JSModuleFunc{
			Name:        "listTopStorers",
			Value:       m.listTopStorers,
			Description: "List tickets of top network storers up to the given limit",
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
	decodeCommon(tx, params)

	if value, ok := params["value"]; ok {
		defer castPanic("value")
		tx.Value = util.String(value.(string))
	}

	delegate := ""
	if to, ok := params["delegate"]; ok {
		defer castPanic("delegate")
		delegate = to.(string)
	}

	if delegate != "" {
		pubKey, err := crypto.PubKeyFromBase58(delegate)
		if err != nil {
			panic(errors.Wrap(err, "failed to decode 'delegate' to public key"))
		}
		tx.Delegate = util.BytesToBytes32(pubKey.MustBytes())
	}

	finalizeTx(tx, m.service, options...)

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
	decodeCommon(tx, params)

	if value, ok := params["value"]; ok {
		defer castPanic("value")
		tx.Value = util.String(value.(string))
	}

	delegate := ""
	if to, ok := params["delegate"]; ok {
		defer castPanic("delegate")
		delegate = to.(string)
	}

	if delegate != "" {
		pubKey, err := crypto.PubKeyFromBase58(delegate)
		if err != nil {
			panic(errors.Wrap(err, "failed to decode 'delegate' to public key"))
		}
		tx.Delegate = util.BytesToBytes32(pubKey.MustBytes())
	}

	// Derive BLS public key
	key := checkAndGetKey(options...)
	pk, _ := crypto.PrivKeyFromBase58(key)
	blsKey := pk.BLSKey()
	tx.BLSPubKey = blsKey.Public().Bytes()

	finalizeTx(tx, m.service, options...)

	// Process the transaction
	hash, err := m.service.SendTx(tx)
	if err != nil {
		panic(errors.Wrap(err, "failed to send transaction"))
	}

	return EncodeForJS(map[string]interface{}{
		"hash": hash,
	})
}

// listValidatorTicketsOfProposer finds validator tickets where the given public
// key is the proposer; By default it will filter out decayed tickets. Use query
// option to override this behaviour
func (m *TicketModule) listValidatorTicketsOfProposer(
	proposerPubKey string,
	queryOpts ...map[string]interface{}) interface{} {

	var qopts types.QueryOptions

	// Prepare query options
	if len(queryOpts) > 0 {
		qoMap := queryOpts[0]

		// If the user didn't set 'decay' and 'nonDecayed' filters, we set the
		// default of `nonDecayed` tp true to return only non-decayed tickets
		if qoMap["nonDecayed"] == nil && qoMap["decayed"] == nil {
			qopts.NonDecayedOnly = true
		}

		mapstructure.Decode(qoMap, &qopts)
	}

	// If no sort by height directive, sort by height in descending order
	if qopts.SortByHeight == 0 {
		qopts.SortByHeight = -1
	}

	pk, err := crypto.PubKeyFromBase58(proposerPubKey)
	if err != nil {
		panic(errors.Wrap(err, "failed to decode proposer public key"))
	}

	res, err := m.ticketmgr.GetByProposer(types.TxTypeValidatorTicket, pk.MustBytes32(), qopts)
	if err != nil {
		panic(err)
	}

	return EncodeManyForJS(res)
}

// listStorerTicketsOfProposer finds storer tickets where the given public
// key is the proposer
func (m *TicketModule) listStorerTicketsOfProposer(
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

	pk, err := crypto.PubKeyFromBase58(proposerPubKey)
	if err != nil {
		panic(errors.Wrap(err, "failed to decode proposer public key"))
	}

	res, err := m.ticketmgr.GetByProposer(types.TxTypeStorerTicket, pk.MustBytes32(), qopts)
	if err != nil {
		panic(err)
	}

	return EncodeManyForJS(res)
}

// listTopValidators returns top n validators
func (m *TicketModule) listTopValidators(limit ...int) interface{} {
	n := 0
	if len(limit) > 0 {
		n = limit[0]
	}
	tickets, err := m.ticketmgr.GetTopValidators(n)
	if err != nil {
		panic(err)
	}
	return EncodeManyForJS(tickets)
}

// listTopStorers returns top n storers
func (m *TicketModule) listTopStorers(limit ...int) interface{} {
	n := 0
	if len(limit) > 0 {
		n = limit[0]
	}
	tickets, err := m.ticketmgr.GetTopStorers(n)
	if err != nil {
		panic(err)
	}
	return EncodeManyForJS(tickets)
}

// ticketStats returns ticket statistics of the network; If proposerPubKey is
// provided, the proposer's personalized ticket stats are included.
func (m *TicketModule) ticketStats(proposerPubKey ...string) interface{} {

	valNonDel, valDel := float64(0), float64(0)
	res := make(map[string]interface{})

	if len(proposerPubKey) > 0 {
		pk, err := crypto.PubKeyFromBase58(proposerPubKey[0])
		if err != nil {
			panic(errors.Wrap(err, "failed to decode proposer public key"))
		}

		valNonDel, err = m.ticketmgr.ValueOfNonDelegatedTickets(pk.MustBytes32(), 0)
		if err != nil {
			panic(err)
		}

		valDel, err = m.ticketmgr.ValueOfDelegatedTickets(pk.MustBytes32(), 0)
		if err != nil {
			panic(err)
		}

		res["valueOfNonDelegated"] = valNonDel
		res["valueOfDelegated"] = valDel
		res["publicKeyPower"] = decimal.NewFromFloat(valNonDel).
			Add(decimal.NewFromFloat(valDel)).String()
	}

	valAll, err := m.ticketmgr.ValueOfAllTickets(0)
	if err != nil {
		panic(err)
	}
	res["valueOfAll"] = valAll

	return EncodeForJS(res)
}

// listRecent returns most recent tickets up to the given limit
func (m *TicketModule) listRecent(limit ...int) interface{} {
	n := 0
	if len(limit) > 0 {
		n = limit[0]
	}
	res := m.ticketmgr.Query(func(t *types.Ticket) bool { return true }, types.QueryOptions{
		Limit:        n,
		SortByHeight: -1,
	})
	return EncodeManyForJS(res)
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
	decodeCommon(tx, params)

	if ticketHash, ok := params["hash"]; ok {
		defer castPanic("fee")
		bz, err := util.FromHex(ticketHash.(string))
		if err != nil {
			panic(errors.Wrap(err, "failed to decode ticket hash from hex"))
		}
		tx.TicketHash = util.BytesToBytes32(bz)
	}

	finalizeTx(tx, m.service, options...)

	// Process the transaction
	hash, err := m.service.SendTx(tx)
	if err != nil {
		panic(errors.Wrap(err, "failed to send transaction"))
	}

	return EncodeForJS(map[string]interface{}{
		"hash": hash,
	})
}
