package jsmodules

import (
	"fmt"
	"time"

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
}

// NewTicketModule creates an instance of TicketModule
func NewTicketModule(
	vm *otto.Otto,
	service types.Service,
	ticketmgr types.TicketManager) *TicketModule {
	return &TicketModule{vm: vm, service: service, ticketmgr: ticketmgr}
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
			Name:        "find",
			Value:       m.find,
			Description: "Get tickets belonging to a given public key",
		},
		&types.JSModuleFunc{
			Name:        "top",
			Value:       m.Top,
			Description: "Get most recent tickets up to the given limit",
		},
	}
}

// Configure configures the JS context and return
// any number of console prompt suggestions
func (m *TicketModule) Configure() []prompt.Suggest {
	suggestions := []prompt.Suggest{}

	// Add the main namespace
	obj := map[string]interface{}{}
	util.VMSet(m.vm, types.NamespaceTicket, obj)

	for _, f := range m.funcs() {
		obj[f.Name] = f.Value
		funcFullName := fmt.Sprintf("%s.%s", types.NamespaceTicket, f.Name)
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
func (m *TicketModule) buy(txObj interface{}, options ...interface{}) interface{} {

	var err error

	// Decode parameters into a transaction object
	var tx = types.NewBareTx(types.TxTypeGetTicket)
	if err = mapstructure.Decode(txObj, tx); err != nil {
		panic(errors.Wrap(err, types.ErrArgDecode("types.Transaction", 0).Error()))
	}

	// - Expect options[0] to be the private key (base58 encoded)
	// - options[0] must be a string
	// - options[0] must be a valid key
	var key string
	var ok bool
	if len(options) > 0 {
		key, ok = options[0].(string)
		if !ok {
			panic(types.ErrArgDecode("string", 1))
		} else if err := crypto.IsValidPrivKey(key); err != nil {
			panic(errors.Wrap(err, types.ErrInvalidPrivKey.Error()))
		}
	} else {
		panic(fmt.Errorf("key is required"))
	}

	// Set tx public key
	pk, _ := crypto.PrivKeyFromBase58(key)
	tx.SetSenderPubKey(util.String(crypto.NewKeyFromPrivKey(pk).PubKey().Base58()))

	// Set timestamp if not already set
	if tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}

	// Set nonce if nonce is not provided
	if tx.Nonce == 0 {
		nonce, err := m.service.GetNonce(tx.GetFrom())
		if err != nil {
			panic(errors.Wrap(err, "failed to get sender's nonce"))
		}
		tx.Nonce = nonce + 1
	}

	// Sign the tx
	tx.Sig, err = tx.Sign(key)
	if err != nil {
		panic(errors.Wrap(err, "failed to sign transaction"))
	}

	// Process the transaction
	hash, err := m.service.SendCoin(tx)
	if err != nil {
		panic(errors.Wrap(err, "failed to send transaction"))
	}

	return util.EncodeForJS(map[string]interface{}{
		"hash": hash,
	})
}

// find finds tickets owned by a given public key
func (m *TicketModule) find(
	proposerPubKey string,
	queryOpts ...map[string]interface{}) interface{} {

	var qopts types.QueryOptions
	if len(queryOpts) > 0 {
		mapstructure.Decode(queryOpts[0], &qopts)
	}

	res, err := m.ticketmgr.GetByProposer(proposerPubKey, qopts)
	if err != nil {
		panic(err)
	}

	return res
}

// Top returns most recent tickets up to the given limit
func (m *TicketModule) Top(limit int) interface{} {
	res, err := m.ticketmgr.Query(types.Ticket{}, types.QueryOptions{
		Limit: limit,
		Order: `"height" desc`,
	})
	if err != nil {
		panic(err)
	}

	return res
}
