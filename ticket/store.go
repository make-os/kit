package ticket

import (
	"strings"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/makeos/mosdef/types"
)

// Store describes an interface for storing and accessing tickets.
type Store interface {
	// Add stores tickets
	Add(t ...*types.Ticket) error
	// Query queries tickets that match the given query
	Query(query types.Ticket, queryOptions ...interface{}) ([]*types.Ticket, error)
	// QueryOne finds a ticket that match the given query
	QueryOne(query types.Ticket, queryOptions ...interface{}) (*types.Ticket, error)
	// Count counts tickets that match the given query
	Count(query types.Ticket, queryOptions ...interface{}) (int, error)
	// GetLive returns matured and non-decayed tickets
	GetLive(height int64, queryOptions ...interface{}) ([]*types.Ticket, error)
	// CountLive returns the number of matured and live tickets
	CountLive(height int64, queryOptions ...interface{}) (int, error)
	// MarkAsUnbonded sets a ticket unbonded status to true
	MarkAsUnbonded(hash string) error
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

// Add stores tickets
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

func applyQueryOpts(q *gorm.DB, opts types.QueryOptions) *gorm.DB {
	if opts.Limit > 0 {
		q = q.Limit(opts.Limit)
	}

	if opts.Offset > 0 {
		q = q.Offset(opts.Offset)
	}

	if opts.Order != "" {
		for _, orderPart := range strings.Split(opts.Order, ",") {
			q = q.Order(orderPart)
		}
	}

	return q
}

// Query queries tickets that match the given query
func (s *SQLStore) Query(
	query types.Ticket,
	queryOptions ...interface{}) ([]*types.Ticket, error) {

	opts := getQueryOptions(queryOptions...)
	q := s.db.Where(query)
	q = applyQueryOpts(q, opts)

	var tickets []*types.Ticket
	return tickets, q.Find(&tickets).Error
}

// QueryOne finds a ticket that match the given query
func (s *SQLStore) QueryOne(
	query types.Ticket,
	queryOptions ...interface{}) (*types.Ticket, error) {

	opts := getQueryOptions(queryOptions...)
	q := s.db.Where(query)
	q = applyQueryOpts(q, opts)

	var ticket types.Ticket
	if err := q.Find(&ticket).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			return nil, err
		}
		return nil, nil
	}

	return &ticket, nil
}

// GetLive returns matured and live tickets.
// The argument height is the current/latest block;
func (s *SQLStore) GetLive(
	height int64,
	queryOptions ...interface{}) ([]*types.Ticket, error) {

	opts := getQueryOptions(queryOptions...)
	q := s.db.
		Where(`"matureBy" <= ?`, height).
		Where(`"decayBy" > ?`, height)
	q = applyQueryOpts(q, opts)

	var tickets []*types.Ticket
	return tickets, q.Find(&tickets).Error
}

// MarkAsUnbonded sets a ticket unbonded status to true
func (s *SQLStore) MarkAsUnbonded(hash string) error {
	return s.db.Model(&types.Ticket{}).
		Where(&types.Ticket{Hash: hash}).
		Update(&types.Ticket{Unbonded: true}).Error
}

// CountLive returns the number of matured and live tickets.
// The argument height is the current/latest block;
func (s *SQLStore) CountLive(
	height int64,
	queryOptions ...interface{}) (int, error) {

	opts := getQueryOptions(queryOptions...)
	q := s.db.
		Model(types.Ticket{}).
		Where(`"matureBy" <= ?`, height).
		Where(`"decayBy" > ?`, height)
	q = applyQueryOpts(q, opts)

	var count int
	return count, q.Count(&count).Error
}

// Count counts tickets that match the given query
func (s *SQLStore) Count(
	query types.Ticket,
	queryOptions ...interface{}) (int, error) {

	q := s.db.Model(types.Ticket{}).Where(query)

	var count int
	return count, q.Count(&count).Error
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
