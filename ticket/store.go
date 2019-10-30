package ticket

import (
	"bytes"
	"sort"

	"github.com/imdario/mergo"
	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
)

const (
	// Separator separates prefixes
	Separator = ":"
	// TagTicket is the prefix for ticket data
	TagTicket = "tkt"
)

// MakeKey creates a key for storing a ticket.
func MakeKey(hash []byte, height uint64, index int) []byte {
	bzSep := []byte(Separator)
	tagBz := []byte(TagTicket)
	bzHeight := util.EncodeNumber(uint64(height))
	bzIndex := util.EncodeNumber(uint64(index))
	return bytes.Join([][]byte{tagBz, bzSep, hash, bzSep, bzHeight, bzSep, bzIndex}, nil)
}

// MakeHashKey creates a key for storing a ticket.
func MakeHashKey(hash []byte) []byte {
	bzSep := []byte(Separator)
	tagBz := []byte(TagTicket)
	return bytes.Join([][]byte{tagBz, bzSep, hash}, nil)
}

// Storer describes the functions of a ticker store
type Storer interface {

	// Add adds one or more tickets to the store
	Add(tickets ...*types.Ticket) error

	// GetByHash queries a ticket by its hash
	GetByHash(hash string) *types.Ticket

	// RemoveByHash deletes a ticket by its hash
	RemoveByHash(hash string) error

	// QueryOne iterates over the tickets and returns the first ticket
	// for which the predicate returns true.
	QueryOne(predicate func(*types.Ticket) bool) *types.Ticket

	// Query iterates over the tickets and returns all tickets
	// for which the predicate returns true.
	Query(predicate func(*types.Ticket) bool, queryOpt ...interface{}) []*types.Ticket

	// Count counts tickets for which the predicate returns true.
	Count(predicate func(*types.Ticket) bool) int

	// UpdateOne update a ticket for which the query predicate returns true.
	// NOTE: If the fields that make up the key of the ticket is updated, a new record
	// with different key composition will be created.
	UpdateOne(upd types.Ticket, queryPredicate func(*types.Ticket) bool)
}

// Store implements Storer
type Store struct {
	db       storage.Tx // The DB transaction
	fromHead bool              // If true, the iterator iterates from the tail
}

// NewStore creates an instance of Store
func NewStore(db storage.Tx) *Store {
	return &Store{db: db, fromHead: true}
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

// Add adds one or more tickets to the store
func (s *Store) Add(tickets ...*types.Ticket) error {
	for _, ticket := range tickets {
		key := MakeKey([]byte(ticket.Hash), ticket.Height, ticket.Index)
		rec := storage.NewRecord(key, util.ObjectToBytes(ticket))
		if err := s.db.Put(rec); err != nil {
			return err
		}
	}
	return nil
}

// GetByHash queries a ticket by its hash
func (s *Store) GetByHash(hash string) *types.Ticket {
	var t *types.Ticket
	s.db.Iterate(MakeHashKey([]byte(hash)), false, func(r *storage.Record) bool {
		r.Scan(&t)
		return true
	})
	return t
}

// RemoveByHash deletes a ticket by its hash
func (s *Store) RemoveByHash(hash string) error {
	t := s.GetByHash(hash)
	if t == nil {
		return nil
	}
	return s.db.Del(MakeKey([]byte(hash), t.Height, t.Index))
}

// QueryOne iterates over the tickets and returns the first ticket
// for which the predicate returns true.
func (s *Store) QueryOne(predicate func(*types.Ticket) bool) *types.Ticket {
	var selected *types.Ticket
	s.db.Iterate([]byte(TagTicket), s.fromHead, func(rec *storage.Record) bool {
		var t types.Ticket
		rec.Scan(&t)
		if predicate(&t) {
			selected = &t
			return true
		}
		return false
	})
	return selected
}

// Query iterates over the tickets and returns all tickets
// for which the predicate returns true.
func (s *Store) Query(predicate func(*types.Ticket) bool,
	queryOpt ...interface{}) []*types.Ticket {
	var selected []*types.Ticket
	var qo = getQueryOptions(queryOpt...)
	s.db.Iterate([]byte(TagTicket), s.fromHead, func(rec *storage.Record) bool {

		// Apply limit only when limit is set and sorting is not required
		if qo.Limit > 0 && qo.Limit == len(selected) && qo.SortByHeight == 0 {
			return true
		}

		var t types.Ticket
		rec.Scan(&t)
		if predicate(&t) {
			selected = append(selected, &t)
		}
		return false
	})

	// Sort by height if required
	if dir := qo.SortByHeight; dir != 0 {
		sort.Slice(selected, func(i, j int) bool {
			if dir == -1 {
				return selected[i].Height > selected[j].Height
			}
			return selected[i].Height < selected[j].Height
		})
	}

	// If limit and sort option was set, apply the limit here
	if qo.Limit > 0 && qo.SortByHeight != 0 && len(selected) >= qo.Limit {
		selected = selected[:qo.Limit]
	}

	return selected
}

// FromTail returns a new instance of Store that will set iterators
// to start reading records from the bottom/tail
func (s *Store) FromTail() *Store {
	ts := &Store{db: s.db, fromHead: false}
	return ts
}

// Count counts tickets for which the predicate returns true.
func (s *Store) Count(predicate func(*types.Ticket) bool) int {
	var count int
	s.db.Iterate([]byte(TagTicket), s.fromHead, func(rec *storage.Record) bool {
		var t types.Ticket
		rec.Scan(&t)
		if predicate(&t) {
			count++
		}
		return false
	})
	return count
}

// UpdateOne update a ticket for which the query predicate returns true.
// NOTE: If the fields that make up the key of the ticket is updated, a new record
// with different key composition will be created.
func (s *Store) UpdateOne(upd types.Ticket, queryPredicate func(*types.Ticket) bool) {
	target := s.QueryOne(queryPredicate)
	if target == nil {
		return
	}
	mergo.Merge(&upd, target)
	key := MakeKey([]byte(target.Hash), target.Height, target.Index)
	s.db.Del(key)
	s.Add(&upd)
}
