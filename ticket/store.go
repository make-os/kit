package ticket

import (
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/makeos/mosdef/types"
)

// Store describes an interface for storing and accessing tickets.
type Store interface {
	// Add adds a ticket
	Add(t ...*types.Ticket) error
	// Query queries tickets
	Query(query types.Ticket, queryOptions ...interface{}) ([]*types.Ticket, error)
	// Close closes the store
	Close() error
}

// SQLStore implements Store. It stores tickets in an SQLite backend.
type SQLStore struct {
	db *gorm.DB
}

// NewSQLStore opens the store and returns an instance of SQLStore
func NewSQLStore(dbPath string) (*SQLStore, error) {
	db, err := gorm.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	db.AutoMigrate(&types.Ticket{})
	return &SQLStore{
		db: db,
	}, nil
}

// Add stores a ticket
func (s *SQLStore) Add(tickets ...*types.Ticket) error {
	db := s.db.Begin()
	for _, t := range tickets {
		if err := db.Create(t).Error; err != nil {
			db.Rollback()
			return err
		}
	}
	return db.Commit().Error
}

// getQueryOptions returns a types.QueryOptions stored in a slice of interface
func getQueryOptions(queryOptions ...interface{}) types.QueryOptions {
	if len(queryOptions) > 0 {
		opts, ok := queryOptions[0].(types.QueryOptions)
		if ok {
			return opts
		}
	}
	return types.QueryOptions{}
}

// Query fetches tickets by validator proposedPubKey
func (s *SQLStore) Query(
	query types.Ticket,
	queryOptions ...interface{}) ([]*types.Ticket, error) {
	opts := getQueryOptions(queryOptions...)
	q := s.db.Where(query)

	if opts.Limit > 0 {
		q = q.Limit(opts.Limit)
	}

	if opts.Offset > 0 {
		q = q.Offset(opts.Offset)
	}

	if opts.Order != "" {
		q = q.Order(opts.Order)
	}

	if opts.NoChild {
		q = q.Where(`"childOf" = ?`, "")
	}

	var tickets []*types.Ticket
	return tickets, q.Find(&tickets).Error
}

// GetTicketByProposerPubKey fetches tickets by validator proposedPubKey
func (s *SQLStore) GetTicketByProposerPubKey(
	proposedPubKey string,
	queryOptions ...interface{}) ([]*types.Ticket, error) {
	return s.Query(types.Ticket{ProposerPubKey: proposedPubKey}, queryOptions...)
}

// Close closes the store and releases held resources
func (s *SQLStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
