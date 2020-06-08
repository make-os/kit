package ticket

import (
	"bytes"
	"sort"

	types2 "gitlab.com/makeos/mosdef/ticket/types"

	"gitlab.com/makeos/mosdef/storage"
	"gitlab.com/makeos/mosdef/util"
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
	bzHeight := util.EncodeNumber(height)
	bzIndex := util.EncodeNumber(uint64(index))
	return bytes.Join([][]byte{tagBz, bzSep, hash, bzSep, bzHeight, bzSep, bzIndex}, nil)
}

// MakeHashKey creates a key for storing a ticket.
func MakeHashKey(hash []byte) []byte {
	bzSep := []byte(Separator)
	tagBz := []byte(TagTicket)
	return bytes.Join([][]byte{tagBz, bzSep, hash}, nil)
}

// TicketStore describes the functions of a ticket store
type TicketStore interface {

	// Register adds one or more tickets to the store
	Add(tickets ...*types2.Ticket) error

	// GetByHash queries a ticket by its hash
	GetByHash(hash util.Bytes32) *types2.Ticket

	// RemoveByHash deletes a ticket by its hash
	RemoveByHash(hash util.Bytes32) error

	// QueryOne iterates over the tickets and returns the first ticket
	// for which the predicate returns true.
	QueryOne(predicate func(*types2.Ticket) bool) *types2.Ticket

	// Query iterates over the tickets and returns all tickets
	// for which the predicate returns true.
	Query(predicate func(*types2.Ticket) bool, queryOpt ...interface{}) []*types2.Ticket

	// Count counts tickets for which the predicate returns true.
	Count(predicate func(*types2.Ticket) bool) int

	// UpdateOne update a ticket for which the query predicate returns true.
	// NOTE: If the fields that make up the key of the ticket is updated, a new record
	// with different key composition will be created.
	UpdateOne(upd types2.Ticket, queryPredicate func(*types2.Ticket) bool)
}

// Store implements TicketStore
type Store struct {
	db       storage.Tx // The DB transaction
	fromHead bool       // If true, the iterator iterates from the tail
}

// NewStore creates an instance of Store
func NewStore(db storage.Tx) *Store {
	return &Store{db: db, fromHead: true}
}

// getQueryOptions returns a types.QueryOptions stored in a slice of interface
func getQueryOptions(queryOptions ...interface{}) types2.QueryOptions {
	if len(queryOptions) > 0 {
		opts, ok := queryOptions[0].(types2.QueryOptions)
		if ok {
			return opts
		}
	}
	return types2.QueryOptions{}
}

// Register adds one or more tickets to the store
func (s *Store) Add(tickets ...*types2.Ticket) error {
	for _, ticket := range tickets {
		key := MakeKey(ticket.Hash.Bytes(), ticket.Height, ticket.Index)
		rec := storage.NewRecord(key, util.ToBytes(ticket))
		if err := s.db.Put(rec); err != nil {
			return err
		}
	}
	return nil
}

// GetByHash queries a ticket by its hash
func (s *Store) GetByHash(hash util.Bytes32) *types2.Ticket {
	var t *types2.Ticket
	s.db.Iterate(MakeHashKey(hash.Bytes()), false, func(r *storage.Record) bool {
		r.Scan(&t)
		return true
	})
	return t
}

// RemoveByHash deletes a ticket by its hash
func (s *Store) RemoveByHash(hash util.Bytes32) error {
	t := s.GetByHash(hash)
	if t == nil {
		return nil
	}
	return s.db.Del(MakeKey(hash.Bytes(), t.Height, t.Index))
}

// QueryOne iterates over the tickets and returns the first ticket
// for which the predicate returns true.
func (s *Store) QueryOne(predicate func(*types2.Ticket) bool) *types2.Ticket {
	var selected *types2.Ticket
	s.db.Iterate([]byte(TagTicket), s.fromHead, func(rec *storage.Record) bool {
		var t types2.Ticket
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
func (s *Store) Query(predicate func(*types2.Ticket) bool,
	queryOpt ...interface{}) []*types2.Ticket {
	var selected []*types2.Ticket
	var qo = getQueryOptions(queryOpt...)
	s.db.Iterate([]byte(TagTicket), s.fromHead, func(rec *storage.Record) bool {

		// Apply limit only when limit is set and sorting is not required
		if qo.Limit > 0 && qo.Limit == len(selected) && qo.SortByHeight == 0 {
			return true
		}

		var t types2.Ticket
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
func (s *Store) Count(predicate func(*types2.Ticket) bool) int {
	var count int
	s.db.Iterate([]byte(TagTicket), s.fromHead, func(rec *storage.Record) bool {
		var t types2.Ticket
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
func (s *Store) UpdateOne(upd types2.Ticket, queryPredicate func(*types2.Ticket) bool) {
	target := s.QueryOne(queryPredicate)
	if target == nil {
		return
	}

	if upd.DecayBy != 0 {
		target.DecayBy = upd.DecayBy
	}

	if upd.MatureBy != 0 {
		target.MatureBy = upd.MatureBy
	}

	key := MakeKey(target.Hash.Bytes(), target.Height, target.Index)
	s.db.Del(key)
	s.Add(target)
}
