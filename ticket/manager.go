package ticket

import (
	"math/big"
	"math/rand"
	"sort"

	"github.com/makeos/mosdef/storage"

	"github.com/makeos/mosdef/crypto"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/params"
	"github.com/makeos/mosdef/types"
)

// Manager implements types.TicketManager.
// It provides ticket management functionalities.
type Manager struct {
	cfg   *config.AppConfig
	logic types.Logic
	s     Storer
}

// NewManager returns an instance of Manager.
// Returns error if unable to initialize the store.
func NewManager(db storage.Tx, cfg *config.AppConfig, logic types.Logic) *Manager {
	mgr := &Manager{cfg: cfg, logic: logic}
	mgr.s = NewStore(db)
	return mgr
}

// Index takes a tx and creates a ticket out of it
func (m *Manager) Index(tx *types.Transaction, blockHeight uint64, txIndex int) error {

	ticket := &types.Ticket{
		Type:           tx.Type,
		Height:         blockHeight,
		Index:          txIndex,
		Value:          tx.Value,
		Hash:           tx.GetHash().HexStr(),
		ProposerPubKey: tx.SenderPubKey.String(),
	}

	// By default the proposer is the creator of the transaction.
	// However, if the transaction `to` field is set, the sender
	// is delegating the ticket to the public key set in `to`
	if !tx.To.Empty() {
		ticket.ProposerPubKey = tx.To.String()
		ticket.Delegator = tx.GetFrom().String()

		// Since this is a delegated ticket, we need to get the
		// proposer's commission rate from their account
		pk, _ := crypto.PubKeyFromBase58(ticket.ProposerPubKey)
		proposerAcct := m.logic.AccountKeeper().GetAccount(pk.Addr())
		ticket.CommissionRate = proposerAcct.DelegatorCommission
	}

	if tx.Type == types.TxTypeValidatorTicket {
		// Set maturity and decay heights
		ticket.MatureBy = blockHeight + uint64(params.MinTicketMatDur)
		ticket.DecayBy = ticket.MatureBy + uint64(params.MaxTicketActiveDur)
	}

	// Add all tickets to the store
	if err := m.s.Add(ticket); err != nil {
		return err
	}

	return nil
}

// Remove deletes a ticket by its hash
func (m *Manager) Remove(hash string) error {
	return m.s.RemoveByHash(hash)
}

// GetByProposer finds tickets belonging to the given proposer public key.
func (m *Manager) GetByProposer(ticketType int, proposerPubKey string,
	queryOpt ...interface{}) ([]*types.Ticket, error) {
	res := m.s.Query(func(t *types.Ticket) bool {
		return t.Type == ticketType && t.ProposerPubKey == proposerPubKey
	}, queryOpt...)
	return res, nil
}

// CountActiveValidatorTickets returns the number of matured and non-decayed tickets.
func (m *Manager) CountActiveValidatorTickets() (int, error) {

	// Get the last committed block
	bi, err := m.logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		return 0, err
	}

	count := m.s.Count(func(t *types.Ticket) bool {
		return t.Type == types.TxTypeValidatorTicket &&
			t.MatureBy <= uint64(bi.Height) &&
			t.DecayBy > uint64(bi.Height)
	})

	return count, nil
}

// GetActiveTicketsByProposer returns all active tickets associated to a
// proposer
// proposer: The public key of the proposer
// ticketType: Filter the search to a specific ticket type
// addDelegated: When true, delegated tickets are added.
func (m *Manager) GetActiveTicketsByProposer(proposer string, ticketType int,
	addDelegated bool) ([]*types.Ticket, error) {

	// Get the last committed block
	bi, err := m.logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		return nil, err
	}

	result := m.s.Query(func(t *types.Ticket) bool {
		return t.Type == ticketType &&
			t.MatureBy <= uint64(bi.Height) &&
			(t.DecayBy > uint64(bi.Height) || (t.DecayBy == 0 && t.Type == types.TxTypeStorerTicket)) &&
			t.ProposerPubKey == proposer &&
			(t.Delegator == "" || t.Delegator != "" && addDelegated)
	})

	return result, nil
}

// Query finds and returns tickets that match the given query
func (m *Manager) Query(qf func(t *types.Ticket) bool, queryOpt ...interface{}) []*types.Ticket {
	return m.s.Query(qf, queryOpt...)
}

// QueryOne finds and returns a ticket that match the given query
func (m *Manager) QueryOne(qf func(t *types.Ticket) bool) *types.Ticket {
	return m.s.QueryOne(qf)
}

// UpdateDecayBy updates the decay height of a ticket
func (m *Manager) UpdateDecayBy(hash string, newDecayHeight uint64) error {
	m.s.UpdateOne(types.Ticket{DecayBy: newDecayHeight},
		func(t *types.Ticket) bool { return t.Hash == hash })
	return nil
}

// GetOrderedLiveValidatorTickets returns live tickets ordered by
// value in desc. order, height asc order and index asc order
func (m *Manager) GetOrderedLiveValidatorTickets(height int64, limit int) []*types.Ticket {

	// Get matured, non-decayed tickets
	tickets := m.s.Query(func(t *types.Ticket) bool {
		return t.Type == types.TxTypeValidatorTicket &&
			t.MatureBy <= uint64(height) &&
			t.DecayBy > uint64(height)
	}, types.QueryOptions{Limit: limit})

	sort.Slice(tickets, func(i, j int) bool {
		iVal := tickets[i].Value.Decimal()
		jVal := tickets[j].Value.Decimal()
		if iVal.GreaterThan(jVal) {
			return true
		} else if iVal.LessThan(jVal) {
			return false
		}

		if tickets[i].Height < tickets[j].Height {
			return true
		} else if tickets[i].Height > tickets[j].Height {
			return false
		}

		return tickets[i].Index < tickets[j].Index
	})

	return tickets
}

// SelectRandom selects random live tickets up to the specified limit.
// The provided see is used to seed the PRNG that is used to select tickets.
func (m *Manager) SelectRandom(height int64, seed []byte, limit int) ([]*types.Ticket, error) {

	tickets := m.GetOrderedLiveValidatorTickets(height, params.ValidatorTicketPoolSize)

	// Create a RNG sourced with the seed
	seedInt := new(big.Int).SetBytes(seed)
	r := rand.New(rand.NewSource(seedInt.Int64()))

	// Select random tickets up to the given limit.
	// Note: Only 1 slot per public key.
	index := make(map[string]struct{})
	selected := []*types.Ticket{}
	for len(index) < limit && len(tickets) > 0 {

		// Select a candidate ticket and remove it from the list
		i := r.Intn(len(tickets))
		candidate := tickets[i]
		tickets = append(tickets[:i], tickets[i+1:]...)

		// If the candidate has already been selected, ignore
		if _, ok := index[candidate.ProposerPubKey]; ok {
			continue
		}

		index[candidate.ProposerPubKey] = struct{}{}
		selected = append(selected, candidate)
	}

	return selected, nil
}

// GetByHash get a ticket by hash
func (m *Manager) GetByHash(hash string) *types.Ticket {
	return m.QueryOne(func(t *types.Ticket) bool { return t.Hash == hash })
}

// Stop stores the manager
func (m *Manager) Stop() error {
	return nil
}
