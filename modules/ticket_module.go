package modules

import (
	"fmt"

	"github.com/c-bata/go-prompt"
	"github.com/mitchellh/mapstructure"
	"github.com/robertkrimen/otto"
	"github.com/shopspring/decimal"
	"gitlab.com/makeos/mosdef/console"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/modules/types"
	"gitlab.com/makeos/mosdef/node/services"
	tickettypes "gitlab.com/makeos/mosdef/ticket/types"
	"gitlab.com/makeos/mosdef/types/constants"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/txns"
	"gitlab.com/makeos/mosdef/util"
)

// TicketModule provides access to various utility functions
type TicketModule struct {
	console.ConsoleSuggestions
	service   services.Service
	logic     core.Logic
	ticketmgr tickettypes.TicketManager
}

// NewTicketModule creates an instance of TicketModule
func NewTicketModule(service services.Service, logic core.Logic, ticketmgr tickettypes.TicketManager) *TicketModule {
	return &TicketModule{
		service:   service,
		ticketmgr: ticketmgr,
		logic:     logic,
	}
}

// globals are functions exposed in the VM's global namespace
func (m *TicketModule) globals() []*types.ModuleFunc {
	return []*types.ModuleFunc{}
}

// ConsoleOnlyMode indicates that this module can be used on console-only mode
func (m *TicketModule) ConsoleOnlyMode() bool {
	return false
}

// methods are functions exposed in the special namespace of this module.
func (m *TicketModule) methods() []*types.ModuleFunc {
	return []*types.ModuleFunc{
		{
			Name:        "buy",
			Value:       m.BuyValidatorTicket,
			Description: "BuyValidatorTicket a validator ticket",
		},
		{
			Name:        "listProposerValidatorTickets",
			Value:       m.ListProposerValidatorTickets,
			Description: "List validator tickets assigned to a given proposer public key",
		},
		{
			Name:        "listProposerHostTickets",
			Value:       m.ListProposerHostTickets,
			Description: "List host tickets assigned to a given proposer public key",
		},
		{
			Name:        "listRecent",
			Value:       m.ListRecent,
			Description: "List most recent tickets up to the given limit",
		},
		{
			Name:        "stats",
			Value:       m.TicketStats,
			Description: "Get ticket stats of network and a public key",
		},
		{
			Name:        "listTopValidators",
			Value:       m.ListTopValidators,
			Description: "List tickets of top network validators up to the given limit",
		},
		{
			Name:        "listTopHosts",
			Value:       m.ListTopHosts,
			Description: "List tickets of top network hosts up to the given limit",
		},
	}
}

// hostFuncs are `host` methods exposed by the module
func (m *TicketModule) hostFuncs() []*types.ModuleFunc {
	return []*types.ModuleFunc{
		{
			Name:        "buy",
			Value:       m.BuyHostTicket,
			Description: "BuyValidatorTicket an host ticket",
		},
		{
			Name:        "unbond",
			Value:       m.UnbondHostTicket,
			Description: "Unbond the stake associated with a host ticket",
		},
	}
}

// ConfigureVM configures the JS context and return
// any number of console prompt suggestions
func (m *TicketModule) ConfigureVM(vm *otto.Otto) prompt.Completer {

	// Set the namespaces
	hostObj := map[string]interface{}{}
	ticketObj := map[string]interface{}{"host": hostObj}
	util.VMSet(vm, constants.NamespaceTicket, ticketObj)
	hostNS := fmt.Sprintf("%s.%s", constants.NamespaceTicket, constants.NamespaceHost)

	for _, f := range m.methods() {
		ticketObj[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", constants.NamespaceTicket, f.Name)
		m.Suggestions = append(m.Suggestions, prompt.Suggest{Text: funcFullName, Description: f.Description})
	}

	for _, f := range m.hostFuncs() {
		hostObj[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", hostNS, f.Name)
		m.Suggestions = append(m.Suggestions, prompt.Suggest{Text: funcFullName, Description: f.Description})
	}

	// Register global functions
	for _, f := range m.globals() {
		_ = vm.Set(f.Name, f.Value)
		m.Suggestions = append(m.Suggestions, prompt.Suggest{Text: f.Name, Description: f.Description})
	}

	return m.Completer
}

// BuyValidatorTicket creates a transaction to acquire a validator ticket
//
// ARGS:
// params <map>
// params.value 		<number|string>: 	The amount to pay for the ticket
// params.delegate 		<string>: 			A base58 public key of an active delegate
// params.nonce 		<number|string>: 	The senders next account nonce
// params.fee 			<number|string>: 	The transaction fee to pay
// params.timestamp 	<number>: 			The unix timestamp
//
// options <[]interface{}>
// options[0] key <string>: 			The signer's private key
// options[1] payloadOnly <bool>: 		When true, returns the payload only, without sending the tx.
//
// RETURNS object <map>
// object.hash <string>: 				The transaction hash
func (m *TicketModule) BuyValidatorTicket(params map[string]interface{}, options ...interface{}) util.Map {
	var err error

	var tx = txns.NewBareTxTicketPurchase(txns.TxTypeValidatorTicket)
	if err = tx.FromMap(params); err != nil {
		panic(util.NewStatusError(400, StatusCodeInvalidParam, "params", err.Error()))
	}

	if finalizeTx(tx, m.logic, options...) {
		return tx.ToMap()
	}

	// Process the transaction
	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		panic(util.NewStatusError(400, StatusCodeMempoolAddFail, "", err.Error()))
	}

	return map[string]interface{}{
		"hash": hash,
	}
}

// BuyHostTicket creates a transaction to acquire a host ticket
//
// ARGS:
// params <map>
// params.value 		<number|string>: 	The amount to pay for the ticket
// params.delegate 		<string>: 			A base58 public key of an active delegate
// params.nonce 		<number|string>: 	The senders next account nonce
// params.fee 			<number|string>: 	The transaction fee to pay
// params.timestamp 	<number>: 			The unix timestamp
//
// options <[]interface{}>
// options[0] key <string>: 			The signer's private key
// options[1] payloadOnly <bool>: 		When true, returns the payload only, without sending the tx.
//
// RETURNS object <map>
// object.hash <string>: 				The transaction hash
func (m *TicketModule) BuyHostTicket(params map[string]interface{}, options ...interface{}) interface{} {
	var err error

	var tx = txns.NewBareTxTicketPurchase(txns.TxTypeHostTicket)
	if err = tx.FromMap(params); err != nil {
		panic(util.NewStatusError(400, StatusCodeInvalidParam, "params", err.Error()))
	}

	// Derive BLS public key
	key, _ := parseOptions(options...)
	pk, _ := crypto.PrivKeyFromBase58(key)
	blsKey := pk.BLSKey()
	tx.BLSPubKey = blsKey.Public().Bytes()

	if finalizeTx(tx, m.logic, options...) {
		return tx.ToMap()
	}

	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		panic(util.NewStatusError(400, StatusCodeMempoolAddFail, "", err.Error()))
	}

	return map[string]interface{}{
		"hash": hash,
	}
}

// ListProposerValidatorTickets finds validator tickets where the given public
// key is the proposer; By default it will filter out decayed tickets. Use query
// option to override this behaviour
//
// ARGS:
// proposerPubKey: The public key of the target proposer
//
// [queryOpts] <map>
// [queryOpts].nonDecayed <bool>	Forces only non-decayed tickets to be returned (default: true)
// [queryOpts].decayed 	<bool>	Forces only decayed tickets to be returned
//
// RETURNS <[]types.Ticket>
func (m *TicketModule) ListProposerValidatorTickets(
	proposerPubKey string,
	queryOpts ...map[string]interface{}) []util.Map {

	var qopts tickettypes.QueryOptions

	if len(queryOpts) > 0 {
		// If the user didn't set 'decay' and 'nonDecayed' filters, we set the
		// default of `nonDecayed` to true to return only non-decayed tickets
		qoMap := queryOpts[0]
		if qoMap["nonDecayed"] == nil && qoMap["decayed"] == nil {
			qopts.NonDecayedOnly = true
		}
		_ = mapstructure.Decode(qoMap, &qopts)
	}

	// If no sort by height option, sort by height in descending order
	if qopts.SortByHeight == 0 {
		qopts.SortByHeight = -1
	}

	pk, err := crypto.PubKeyFromBase58(proposerPubKey)
	if err != nil {
		panic(util.NewStatusError(400, StatusCodeInvalidProposerPubKey, "params", err.Error()))
	}

	tickets, err := m.ticketmgr.GetByProposer(txns.TxTypeValidatorTicket, pk.MustBytes32(), qopts)
	if err != nil {
		panic(util.NewStatusError(500, StatusCodeAppErr, "", err.Error()))
	}

	if len(tickets) == 0 {
		return []util.Map{}
	}

	return util.StructSliceToMap(tickets)
}

// ListProposerHostTickets finds host tickets where the given public
// key is the proposer
//
// ARGS:
// proposerPubKey: The public key of the target proposer
//
// [queryOpts] <map>
// [queryOpts].nonDecayed <bool>	Forces only non-decayed tickets to be returned (default: true)
// [queryOpts].decayed 	<bool>	Forces only decayed tickets to be returned
func (m *TicketModule) ListProposerHostTickets(
	proposerPubKey string,
	queryOpts ...map[string]interface{}) interface{} {

	var qopts tickettypes.QueryOptions
	if len(queryOpts) > 0 {
		_ = mapstructure.Decode(queryOpts[0], &qopts)
	}

	// If no sort by height option, sort by height in descending order
	if qopts.SortByHeight == 0 {
		qopts.SortByHeight = -1
	}

	pk, err := crypto.PubKeyFromBase58(proposerPubKey)
	if err != nil {
		panic(util.NewStatusError(400, StatusCodeInvalidProposerPubKey, "params", err.Error()))
	}

	tickets, err := m.ticketmgr.GetByProposer(txns.TxTypeHostTicket, pk.MustBytes32(), qopts)
	if err != nil {
		panic(util.NewStatusError(500, StatusCodeAppErr, "", err.Error()))
	}

	if len(tickets) == 0 {
		return []util.Map{}
	}

	return tickets
}

// ListTopValidators returns top n validators
//
// ARGS:
// [limit] <int>: Set the number of result to return (default: 0 = no limit)
func (m *TicketModule) ListTopValidators(limit ...int) interface{} {
	n := 0
	if len(limit) > 0 {
		n = limit[0]
	}
	tickets, err := m.ticketmgr.GetTopValidators(n)
	if err != nil {
		panic(util.NewStatusError(500, StatusCodeAppErr, "", err.Error()))
	}
	if len(tickets) == 0 {
		return []util.Map{}
	}
	return tickets
}

// ListTopHosts returns top n hosts
//
// ARGS
// [limit] <int>: Set the number of result to return (default: 0 = no limit)
func (m *TicketModule) ListTopHosts(limit ...int) interface{} {
	n := 0
	if len(limit) > 0 {
		n = limit[0]
	}
	tickets, err := m.ticketmgr.GetTopHosts(n)
	if err != nil {
		panic(util.NewStatusError(500, StatusCodeAppErr, "", err.Error()))
	}
	if len(tickets) == 0 {
		return []util.Map{}
	}
	return tickets
}

// TicketStats returns ticket statistics of the network; If proposerPubKey is
// provided, the proposer's personalized ticket stats are included.
//
// ARGS:
// [proposerPubKey]: Public key of a proposer. Set to return only stats for a given proposer
//
// RETURNS res <map>
// result.valueOfNonDelegated 	<number>: 	The total value of non-delegated tickets owned by the proposer
// result.valueOfDelegated 		<number>: 	The total value of tickets delegated to the proposer
// result.publicKeyPower 		<number>: 	The total value staked coins power assigned to the proposer
// result.valueOfAll 			<number>: 	The total value of all tickets
func (m *TicketModule) TicketStats(proposerPubKey ...string) (result util.Map) {

	valNonDel, valDel := float64(0), float64(0)
	res := make(map[string]interface{})

	if len(proposerPubKey) > 0 {
		pk, err := crypto.PubKeyFromBase58(proposerPubKey[0])
		if err != nil {
			panic(util.NewStatusError(400, StatusCodeInvalidProposerPubKey, "params", err.Error()))
		}

		valNonDel, err = m.ticketmgr.ValueOfNonDelegatedTickets(pk.MustBytes32(), 0)
		if err != nil {
			panic(util.NewStatusError(500, StatusCodeAppErr, "", err.Error()))
		}

		valDel, err = m.ticketmgr.ValueOfDelegatedTickets(pk.MustBytes32(), 0)
		if err != nil {
			panic(util.NewStatusError(500, StatusCodeAppErr, "", err.Error()))
		}

		res["valueOfNonDelegated"] = valNonDel
		res["valueOfDelegated"] = valDel
		res["publicKeyPower"] = decimal.NewFromFloat(valNonDel).
			Add(decimal.NewFromFloat(valDel)).String()
	}

	valAll, err := m.ticketmgr.ValueOfAllTickets(0)
	if err != nil {
		panic(util.NewStatusError(500, StatusCodeAppErr, "", err.Error()))
	}
	res["valueOfAll"] = valAll

	return res
}

// ListRecent returns most recently acquired tickets
//
// ARGS
// [limit] <int>: Set the number of result to return (default: 0 = no limit)
func (m *TicketModule) ListRecent(limit ...int) []util.Map {
	n := 0
	if len(limit) > 0 {
		n = limit[0]
	}

	res := m.ticketmgr.Query(func(t *tickettypes.Ticket) bool { return true }, tickettypes.QueryOptions{
		Limit:        n,
		SortByHeight: -1,
	})

	if len(res) == 0 {
		return []util.Map{}
	}

	return util.StructSliceToMap(res)
}

// unbondHostTicket initiates the release of stake associated with a host
// ticket
//
// ARGS:
// params <map>
// params.hash 			<string>: 			A hash of the host ticket
// params.nonce 		<number|string>: 	The senders next account nonce
// params.fee 			<number|string>: 	The transaction fee to pay
// params.timestamp 	<number>: 			The unix timestamp
//
// options <[]interface{}>
// options[0] key <string>: 			The signer's private key
// options[1] payloadOnly <bool>: 		When true, returns the payload only, without sending the tx.
//
// RETURNS object <map>
// object.hash <string>: 				The transaction hash
func (m *TicketModule) UnbondHostTicket(params map[string]interface{},
	options ...interface{}) interface{} {
	var err error

	var tx = txns.NewBareTxTicketUnbond(txns.TxTypeUnbondHostTicket)
	if err = tx.FromMap(params); err != nil {
		panic(util.NewStatusError(400, StatusCodeInvalidParam, "params", err.Error()))
	}

	if finalizeTx(tx, m.logic, options...) {
		return tx.ToMap()
	}

	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		panic(util.NewStatusError(400, StatusCodeMempoolAddFail, "", err.Error()))
	}

	return map[string]interface{}{
		"hash": hash,
	}
}
