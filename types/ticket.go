package types

import (
	"github.com/makeos/mosdef/util"
)

// Ticket represents a validator ticket
type Ticket struct {
	Type           int          `json:"type"`           // The type of ticket
	Hash           util.Bytes32 `json:"hash"`           // Hash of the ticket purchase transaction
	DecayBy        uint64       `json:"decayBy"`        // Block height when the ticket becomes decayed
	MatureBy       uint64       `json:"matureBy"`       // Block height when the ticket enters maturity.
	ProposerPubKey util.Bytes32 `json:"proposerPubKey"` // The public key of the validator that owns the ticket.
	BLSPubKey      []byte       `json:"blsPubKey"`      // The BLS public key derived from the same private key of proposer
	Delegator      string       `json:"delegator"`      // Delegator is the address of the original creator of the ticket
	Height         uint64       `json:"height"`         // The block height where this ticket was seen.
	Index          int          `json:"index"`          // The index of the ticket in the transactions list.
	Value          util.String  `json:"value"`          // The value paid for the ticket (as a child - then for the parent ticket)
	CommissionRate float64      `json:"commissionRate"` // The percentage of reward paid to the validator
}

// QueryOptions describe how a query should be executed.
type QueryOptions struct {
	Limit        int `json:"limit" mapstructure:"limit"`
	SortByHeight int `json:"sortByHeight" mapstructure:"sortByHeight"`
}

// EmptyQueryOptions is an empty instance of QueryOptions
var EmptyQueryOptions = QueryOptions{}

// TicketManager describes a ticket manager
// Get finds tickets belonging to the given proposer.
type TicketManager interface {

	// Index adds a ticket (and child tickets) to the ticket index.
	Index(tx BaseTx, blockHeight uint64, txIndex int) error

	// Remove deletes a ticket by its hash
	Remove(hash util.Bytes32) error

	// GetByProposer finds tickets belonging to the
	// given proposer public key.
	GetByProposer(ticketType int, proposerPubKey util.Bytes32,
		queryOpt ...interface{}) ([]*Ticket, error)

	// CountActiveValidatorTickets returns the number of matured and non-decayed tickets.
	CountActiveValidatorTickets() (int, error)

	// GetNonDelegatedTickets returns all non-delegated, active tickets
	// belonging to the given public key
	//
	// pubKey: The public key of the pubKey
	// ticketType: Filter the search to a specific ticket type
	GetNonDelegatedTickets(pubKey util.Bytes32, ticketType int) ([]*Ticket, error)

	// Query finds and returns tickets that match the given query
	Query(qf func(t *Ticket) bool, queryOpt ...interface{}) []*Ticket

	// QueryOne finds and returns a ticket that match the given query
	QueryOne(qf func(t *Ticket) bool) *Ticket

	// GetByHash get a ticket by hash
	GetByHash(hash util.Bytes32) *Ticket

	// UpdateDecayBy updates the decay height of a ticket
	UpdateDecayBy(hash util.Bytes32, newDecayHeight uint64) error

	// GetTopStorers gets storer tickets with the most total delegated value.
	GetTopStorers(limit int) (SelectedTickets, error)

	// GetTopValidators gets validator tickets with the most total delegated value.
	GetTopValidators(limit int) (SelectedTickets, error)

	// ValueOfNonDelegatedTickets returns the sum of value of all
	// non-delegated, non-decayed tickets which has the given public
	// key as the proposer; Includes both validator and storer tickets.
	//
	// pubKey: The public key of the proposer
	// maturityHeight: if set to non-zero, only tickets that reached maturity before
	// or on the given height are selected. Otherwise, the current chain height is used.
	ValueOfNonDelegatedTickets(pubKey util.Bytes32, maturityHeight uint64) (float64, error)

	// ValueOfDelegatedTickets returns the sum of value of all
	// delegated, non-decayed tickets which has the given public
	// key as the proposer; Includes both validator and storer tickets.
	//
	// pubKey: The public key of the proposer
	// maturityHeight: if set to non-zero, only tickets that reached maturity before
	// or on the given height are selected. Otherwise, the current chain height is used.
	ValueOfDelegatedTickets(pubKey util.Bytes32, maturityHeight uint64) (float64, error)

	// ValueOfTickets returns the sum of value of all non-decayed
	// tickets where the given public key is the proposer or delegator;
	// Includes both validator and storer tickets.
	//
	// pubKey: The public key of the proposer
	// maturityHeight: if set to non-zero, only tickets that reached maturity before
	// or on the given height are selected. Otherwise, the current chain height is used.
	ValueOfTickets(pubKey util.Bytes32, maturityHeight uint64) (float64, error)

	// ValueOfAllTickets returns the sum of value of all non-decayed
	// tickets; Includes both validator and storer tickets.
	//
	// maturityHeight: if set to non-zero, only tickets that reached maturity before
	// or on the given height are selected. Otherwise, the current chain height is used.
	ValueOfAllTickets(maturityHeight uint64) (float64, error)

	// GetNonDecayedTickets finds non-decayed tickets that have the given proposer
	// public key as the proposer or the delegator;
	//
	// pubKey: The public key of the proposer
	// maturityHeight: if set to non-zero, only tickets that reached maturity before
	// or on the given height are selected. Otherwise, the current chain height is used.
	GetNonDecayedTickets(pubKey util.Bytes32, maturityHeight uint64) ([]*Ticket, error)

	// Stop stops the ticket manager
	Stop() error
}

// SelectedTicket represents data of a selected ticket
type SelectedTicket struct {
	Ticket     *Ticket     // The selected ticket
	TotalValue util.String // Sum of ticket.Value and all delegated ticket value
}

// SelectedTickets is a collection of SelectedTicket
type SelectedTickets []*SelectedTicket

// Has checks if an entry matching a public key exists
func (v *SelectedTickets) Has(proposerPubKey util.Bytes32) bool {
	for _, t := range *v {
		if t.Ticket.ProposerPubKey.Equal(proposerPubKey) {
			return true
		}
	}
	return false
}

// Get finds a ticket by proposer public key
func (v *SelectedTickets) Get(proposerPubKey util.Bytes32) *SelectedTicket {
	for _, t := range *v {
		if t.Ticket.ProposerPubKey.Equal(proposerPubKey) {
			return t
		}
	}
	return nil
}

// GetWithIndex finds a ticket by proposer public key and also returns its index
// in the list.
func (v *SelectedTickets) GetWithIndex(proposerPubKey util.Bytes32) (*SelectedTicket, int) {
	for i, t := range *v {
		if t.Ticket.ProposerPubKey.Equal(proposerPubKey) {
			return t, i
		}
	}
	return nil, -1
}

// IndexOf returns the index of the ticket associated with the given proposer
// public key. It returns -1 if not ticket was found.
func (v *SelectedTickets) IndexOf(proposerPubKey util.Bytes32) int {
	for i, t := range *v {
		if t.Ticket.ProposerPubKey.Equal(proposerPubKey) {
			return i
		}
	}
	return -1
}
