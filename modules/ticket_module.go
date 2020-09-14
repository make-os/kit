package modules

import (
	"fmt"

	"github.com/c-bata/go-prompt"
	"github.com/make-os/lobe/crypto"
	"github.com/make-os/lobe/modules/types"
	"github.com/make-os/lobe/node/services"
	types2 "github.com/make-os/lobe/rpc/types"
	tickettypes "github.com/make-os/lobe/ticket/types"
	"github.com/make-os/lobe/types/constants"
	"github.com/make-os/lobe/types/core"
	"github.com/make-os/lobe/types/txns"
	"github.com/make-os/lobe/util"
	"github.com/mitchellh/mapstructure"
	"github.com/robertkrimen/otto"
	"github.com/shopspring/decimal"
)

// TicketModule provides access to various utility functions
type TicketModule struct {
	types.ModuleCommon
	service   services.Service
	logic     core.Logic
	ticketmgr tickettypes.TicketManager
}

// NewAttachableTicketModule creates an instance of TicketModule suitable in attach mode
func NewAttachableTicketModule(client types2.Client) *TicketModule {
	return &TicketModule{ModuleCommon: types.ModuleCommon{AttachedClient: client}}
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
func (m *TicketModule) globals() []*types.VMMember {
	return []*types.VMMember{}
}

// methods are functions exposed in the special namespace of this module.
func (m *TicketModule) methods() []*types.VMMember {
	return []*types.VMMember{
		{
			Name:        "buy",
			Value:       m.BuyValidatorTicket,
			Description: "Purchase a validator ticket",
		},
		{
			Name:        "list",
			Value:       m.GetValidatorTicketsByProposer,
			Description: "Get validator tickets assigned to a proposer",
		},
		{
			Name:        "getAll",
			Value:       m.GetAll,
			Description: "Get all validator and host tickets",
		},
		{
			Name:        "getStats",
			Value:       m.GetStats,
			Description: "Get ticket statistics",
		},
		{
			Name:        "top",
			Value:       m.GetTopValidators,
			Description: "Get top validator tickets",
		},
	}
}

// hostFuncs are `host` methods exposed by the module
func (m *TicketModule) hostFuncs() []*types.VMMember {
	return []*types.VMMember{
		{
			Name:        "buy",
			Value:       m.BuyHostTicket,
			Description: "Purchase a host ticket",
		},
		{
			Name:        "list",
			Value:       m.GetHostTicketsByProposer,
			Description: "Get host tickets assigned to a proposer",
		},
		{
			Name:        "unbond",
			Value:       m.UnbondHostTicket,
			Description: "Unbond a host ticket",
		},
		{
			Name:        "top",
			Value:       m.GetTopHosts,
			Description: "Get a list of top host tickets",
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
// params <map>
//  - value <number|string>: The amount to pay for the ticket
//  - delegate <string>: A base58 public key of an active delegate
//  - nonce <number|string>: The senders next account nonce
//  - fee <number|string>: The transaction fee to pay
//  - timestamp <number>: The unix timestamp
//
// options <[]interface{}>
//  - [0] key <string>: The signer's private key
//  - [1] payloadOnly <bool>: When true, returns the payload only, without sending the tx.
//
// RETURNS object <map>
//  - hash <string>: The transaction hash
func (m *TicketModule) BuyValidatorTicket(params map[string]interface{}, options ...interface{}) util.Map {
	var err error

	var tx = txns.NewBareTxTicketPurchase(txns.TxTypeValidatorTicket)
	if err = tx.FromMap(params); err != nil {
		panic(util.ReqErr(400, StatusCodeInvalidParam, "params", err.Error()))
	}

	if printPayload, _ := finalizeTx(tx, m.logic, nil, options...); printPayload {
		return tx.ToMap()
	}

	// Process the transaction
	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		panic(util.ReqErr(400, StatusCodeMempoolAddFail, "", err.Error()))
	}

	return map[string]interface{}{
		"hash": hash,
	}
}

// BuyHostTicket creates a transaction to acquire a host ticket
//
// params <map>
//  - value <number|string>: The amount to pay for the ticket
//  - delegate <string>: A base58 public key of an active delegate
//  - nonce <number|string>: The senders next account nonce
//  - fee <number|string>: The transaction fee to pay
//  - timestamp <number>: The unix timestamp
//
// options <[]interface{}>
//  - [0] key <string>: The signer's private key
//  - [1] payloadOnly <bool>: When true, returns the payload only, without sending the tx.
//
// RETURNS object <map>
// 	- hash <string>: 				The transaction hash
func (m *TicketModule) BuyHostTicket(params map[string]interface{}, options ...interface{}) util.Map {
	var err error

	var tx = txns.NewBareTxTicketPurchase(txns.TxTypeHostTicket)
	if err = tx.FromMap(params); err != nil {
		panic(util.ReqErr(400, StatusCodeInvalidParam, "params", err.Error()))
	}

	key, _ := parseOptions(options...)

	// Derive BLS public key
	if key != nil {
		blsKey := key.BLSKey()
		tx.BLSPubKey = blsKey.Public().Bytes()
	}

	if printPayload, _ := finalizeTx(tx, m.logic, nil, options...); printPayload {
		return tx.ToMap()
	}

	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		panic(util.ReqErr(400, StatusCodeMempoolAddFail, "", err.Error()))
	}

	return map[string]interface{}{
		"hash": hash,
	}
}

// GetValidatorTicketsByProposer finds active validator tickets where the given public
// key is the proposer
//
// proposerPubKey: The public key of the target proposer
//
// [queryOpts] <map>
//  - expired <bool> Forces only expired tickets to be returned
//
// RETURNS <[]types.Ticket>
func (m *TicketModule) GetValidatorTicketsByProposer(proposerPubKey string, queryOpts ...util.Map) []util.Map {

	var qo tickettypes.QueryOptions
	if len(queryOpts) > 0 {
		qo.SortByHeight = -1
		qoMap := queryOpts[0]
		qo.Active = qoMap["expired"] == nil
		mapstructure.Decode(qoMap, &qo)
	}

	pk, err := crypto.PubKeyFromBase58(proposerPubKey)
	if err != nil {
		panic(util.ReqErr(400, StatusCodeInvalidProposerPubKey, "proposerPubKey", err.Error()))
	}

	tickets, err := m.ticketmgr.GetByProposer(txns.TxTypeValidatorTicket, pk.MustBytes32(), qo)
	if err != nil {
		panic(util.ReqErr(500, StatusCodeServerErr, "", err.Error()))
	}

	if len(tickets) == 0 {
		return []util.Map{}
	}

	return util.StructSliceToMap(tickets)
}

// GetHostTicketsByProposer finds host tickets where the
// given public key is the proposer.
//
// proposerPubKey: The public key of the target proposer
//
// [queryOpts] <map>
//  - immature <bool> Return immature tickets
//  - mature <bool> Return mature tickets
//  - expired <bool> Return expired tickets
//  - unexpired <bool> Return expired tickets
//
// RETURNS <[]types.Ticket>
func (m *TicketModule) GetHostTicketsByProposer(proposerPubKey string, queryOpts ...util.Map) []util.Map {

	var qo tickettypes.QueryOptions
	if len(queryOpts) > 0 {
		qo.SortByHeight = -1
		mapstructure.Decode(queryOpts[0], &qo)
	}

	pk, err := crypto.PubKeyFromBase58(proposerPubKey)
	if err != nil {
		panic(util.ReqErr(400, StatusCodeInvalidProposerPubKey, "params", err.Error()))
	}

	tickets, err := m.ticketmgr.GetByProposer(txns.TxTypeHostTicket, pk.MustBytes32(), qo)
	if err != nil {
		panic(util.ReqErr(500, StatusCodeServerErr, "", err.Error()))
	}

	if len(tickets) == 0 {
		return []util.Map{}
	}

	return util.StructSliceToMap(tickets)
}

// ListTopValidators returns top validator tickets
//
// [limit] <int>: Set the number of result to return (default: 0 = no limit)
//
// RETURNS <[]types.Ticket>
func (m *TicketModule) GetTopValidators(limit ...int) []util.Map {

	var n int
	if len(limit) > 0 {
		n = limit[0]
	}

	tickets, err := m.ticketmgr.GetTopValidators(n)
	if err != nil {
		panic(util.ReqErr(500, StatusCodeServerErr, "", err.Error()))
	}

	if len(tickets) == 0 {
		return []util.Map{}
	}

	return util.StructSliceToMap(tickets)
}

// ListTopHosts returns top host tickets
//
// [limit] <int>: Set the number of result to return (default: 0 = no limit)
//
// RETURNS <[]types.Ticket>
func (m *TicketModule) GetTopHosts(limit ...int) []util.Map {
	var n int
	if len(limit) > 0 {
		n = limit[0]
	}

	tickets, err := m.ticketmgr.GetTopHosts(n)
	if err != nil {
		panic(util.ReqErr(500, StatusCodeServerErr, "", err.Error()))
	}

	if len(tickets) == 0 {
		return []util.Map{}
	}

	return util.StructSliceToMap(tickets)
}

// TicketStats returns various statistics about tickets.
// If proposerPubKey is provided, stats will be personalized
// to the given proposer public key.
//
// [proPubKey] <string>: Public key of a proposer
//
// RETURNS result <map>
//  - nonDelegated 	<number>: The total value of non-delegated tickets owned by the proposer
//  - delegated <number>: The total value of tickets delegated to the proposer
//  - power <number>: The total value of staked coins assigned to the proposer
//  - all <number>: The total value of all tickets
func (m *TicketModule) GetStats(proPubKey ...string) (result util.Map) {

	valNonDel, valDel := float64(0), float64(0)
	res := make(map[string]interface{})
	var err error

	// Get value of all tickets
	res["all"], err = m.ticketmgr.ValueOfAllTickets(0)
	if err != nil {
		panic(util.ReqErr(500, StatusCodeServerErr, "", err.Error()))
	}

	// Return if no proposer public key was specified.
	if len(proPubKey) == 0 {
		return res
	}

	// At this point, we need to get stats for the given proposer public key.
	pk, err := crypto.PubKeyFromBase58(proPubKey[0])
	if err != nil {
		panic(util.ReqErr(400, StatusCodeInvalidProposerPubKey, "params", err.Error()))
	}

	// Get value of non-delegated tickets belonging to the public key
	valNonDel, err = m.ticketmgr.ValueOfNonDelegatedTickets(pk.MustBytes32(), 0)
	if err != nil {
		panic(util.ReqErr(500, StatusCodeServerErr, "", err.Error()))
	}

	// Get value of delegated tickets belonging to the public key
	valDel, err = m.ticketmgr.ValueOfDelegatedTickets(pk.MustBytes32(), 0)
	if err != nil {
		panic(util.ReqErr(500, StatusCodeServerErr, "", err.Error()))
	}

	res["nonDelegated"] = valNonDel
	res["delegated"] = valDel
	res["total"] = decimal.NewFromFloat(valNonDel).Add(decimal.NewFromFloat(valDel)).String()

	return res
}

// ListAll returns all tickets, sorted by the most recently added.
//
// [limit] <int>: Set the number of result to return (default: 0 = no limit)
//
// RETURNS <[]types.Ticket>
func (m *TicketModule) GetAll(limit ...int) []util.Map {
	var n int
	if len(limit) > 0 {
		n = limit[0]
	}

	qo := tickettypes.QueryOptions{Limit: n, SortByHeight: -1}
	res := m.ticketmgr.Query(func(t *tickettypes.Ticket) bool { return true }, qo)

	if len(res) == 0 {
		return []util.Map{}
	}

	return util.StructSliceToMap(res)
}

// unbondHostTicket unbonds a host ticket
//
// params <map>
//  - hash <string>: A hash of the host ticket
//  - nonce <number|string>: The senders next account nonce
//  - fee <number|string>: The transaction fee to pay
//  - timestamp <number>: The unix timestamp
//
// options <[]interface{}>
//  - [0] key <string>: The signer's private key
//  - [1] payloadOnly <bool>: When true, returns the payload only, without sending the tx.
//
// RETURNS object <map>
//  - hash <string>: The transaction hash
func (m *TicketModule) UnbondHostTicket(params map[string]interface{}, options ...interface{}) util.Map {
	var err error

	var tx = txns.NewBareTxTicketUnbond(txns.TxTypeUnbondHostTicket)
	if err = tx.FromMap(params); err != nil {
		panic(util.ReqErr(400, StatusCodeInvalidParam, "params", err.Error()))
	}

	if printPayload, _ := finalizeTx(tx, m.logic, nil, options...); printPayload {
		return tx.ToMap()
	}

	hash, err := m.logic.GetMempoolReactor().AddTx(tx)
	if err != nil {
		panic(util.ReqErr(400, StatusCodeMempoolAddFail, "", err.Error()))
	}

	return map[string]interface{}{
		"hash": hash,
	}
}
