package types

import (
	"github.com/makeos/mosdef/util"
)

// Ticket represents a validator ticket
type Ticket struct {
	Type           int         `gorm:"column:type" json:"type"`                     // The type of ticket
	Hash           string      `gorm:"column:hash" json:"hash"`                     // Hash of the ticket purchase transaction
	DecayBy        uint64      `gorm:"column:decayBy" json:"decayBy"`               // Block height when the ticket becomes decayed
	MatureBy       uint64      `gorm:"column:matureBy" json:"matureBy"`             // Block height when the ticket enters maturity.
	ProposerPubKey string      `gorm:"column:proposerPubKey" json:"proposerPubKey"` // The public key of the validator that owns the ticket.
	Delegator      string      `gorm:"column:delegator" json:"delegator"`           // Delegator is the address of the original creator of the ticket
	Height         uint64      `gorm:"column:height" json:"height"`                 // The block height where this ticket was seen.
	Index          int         `gorm:"column:index" json:"index"`                   // The index of the ticket in the transactions list.
	Value          util.String `gorm:"column:value" json:"value"`                   // The value paid for the ticket (as a child - then for the parent ticket)
	CommissionRate float64     `gorm:"column:commissionRate" json:"commissionRate"` // The percentage of reward paid to the validator
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
	Remove(hash string) error

	// GetByProposer finds tickets belonging to the
	// given proposer public key.
	GetByProposer(ticketType int, proposerPubKey string,
		queryOpt ...interface{}) ([]*Ticket, error)

	// CountActiveValidatorTickets returns the number of matured and non-decayed tickets.
	CountActiveValidatorTickets() (int, error)

	// GetActiveTicketsByProposer returns all active tickets associated to a
	// proposer
	// proposer: The public key of the proposer
	// ticketType: Filter the search to a specific ticket type
	// addDelegated: When true, delegated tickets are added.
	GetActiveTicketsByProposer(proposer string, ticketType int, addDelegated bool) ([]*Ticket, error)

	// SelectRandom selects random live tickets up to the specified limit.
	// The provided see is used to seed the PRNG that is used to select tickets.
	SelectRandom(height int64, seed []byte, limit int) ([]*Ticket, error)

	// Query finds and returns tickets that match the given query
	Query(qf func(t *Ticket) bool, queryOpt ...interface{}) []*Ticket

	// QueryOne finds and returns a ticket that match the given query
	QueryOne(qf func(t *Ticket) bool) *Ticket

	// GetByHash get a ticket by hash
	GetByHash(hash string) *Ticket

	// UpdateDecayBy updates the decay height of a ticket
	UpdateDecayBy(hash string, newDecayHeight uint64) error

	// GetOrderedLiveValidatorTickets returns live tickets ordered by
	// value in desc. order, height asc order and index asc order
	GetOrderedLiveValidatorTickets(height int64, limit int) []*Ticket

	// Stop stops the ticket manager
	Stop() error
}
