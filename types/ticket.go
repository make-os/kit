package types

// Ticket represents a validator ticket
type Ticket struct {
	DecayBy        uint64 `gorm:"column:decayBy" json:"decayBy"`               // Block height when the ticket becomes decayed
	MatureBy       uint64 `gorm:"column:matureBy" json:"matureBy"`             // Block height when the ticket enters maturity.
	Hash           string `gorm:"column:hash" json:"hash"`                     // Hash of the ticket purchase transaction
	Power          int64  `gorm:"column:power" json:"power,omitempty"`         // Power represents the strength of a ticket
	ProposerPubKey string `gorm:"column:proposerPubKey" json:"proposerPubKey"` // The public key of the validator that owns the ticket.
	Delegator      string `gorm:"column:delegator" json:"delegator"`           // Delegator is the address of the original creator of the ticket
	Height         uint64 `gorm:"column:height" json:"height"`                 // The block height where this ticket was seen.
	Index          int    `gorm:"column:index" json:"index"`                   // The index of the ticket in the transactions list.
	Value          string `gorm:"column:value" json:"value"`                   // The value paid for the ticket (as a child - then for the parent ticket)
}

// QueryOptions describe how a query should be executed.
type QueryOptions struct {
	Limit  int    `json:"limit" mapstructure:"limit"`
	Offset int    `json:"offset" mapstructure:"offset"`
	Order  string `json:"order" mapstructure:"order"`
}

// EmptyQueryOptions is an empty instance of QueryOptions
var EmptyQueryOptions = QueryOptions{}

// TicketManager describes a ticket manager
// Get finds tickets belonging to the given proposer.
type TicketManager interface {
	// Index adds a ticket (and child tickets) to the ticket index.
	Index(tx *Transaction, blockHeight uint64, txIndex int) error

	// GetByProposer finds tickets belonging to the
	// given proposer public key.
	GetByProposer(proposerPubKey string, queryOpt QueryOptions) ([]*Ticket, error)

	// CountLiveTickets returns the number of matured and non-decayed tickets.
	CountLiveTickets(...QueryOptions) (int, error)

	// SelectRandom selects random live tickets up to the specified limit.
	// The provided see is used to seed the PRNG that is used to select tickets.
	SelectRandom(height int64, seed []byte, limit int) ([]*Ticket, error)

	// Query finds and returns tickets that match the given query
	Query(q Ticket, queryOpt ...QueryOptions) ([]*Ticket, error)

	// Stop stops the ticket manager
	Stop() error
}
