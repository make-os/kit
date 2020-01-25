package ticket

import (
	"sort"

	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/util"

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
func (m *Manager) Index(tx types.BaseTx, blockHeight uint64, txIndex int) error {

	t := tx.(*types.TxTicketPurchase)

	ticket := &types.Ticket{
		Type:           tx.GetType(),
		Height:         blockHeight,
		Index:          txIndex,
		Value:          t.Value,
		Hash:           t.GetHash(),
		ProposerPubKey: t.GetSenderPubKey(),
		BLSPubKey:      t.BLSPubKey,
	}

	// By default the proposer is the creator of the transaction.
	// However, if the transaction `delegate` field is set, the sender
	// is delegating the ticket to the public key set in `delegate`
	if !t.Delegate.IsEmpty() {

		// Set the given delegate as the proposer
		ticket.ProposerPubKey = t.Delegate

		// Set the sender address as the delegator
		ticket.Delegator = t.GetFrom().String()

		// Since this is a delegated ticket, we need to get the proposer's
		// commission rate from their account, write it to the ticket so that it
		// is locked and immutable by a future commission rate update.
		pk := crypto.MustPubKeyFromBytes(ticket.ProposerPubKey.Bytes())
		proposerAcct := m.logic.AccountKeeper().GetAccount(pk.Addr())
		ticket.CommissionRate = proposerAcct.DelegatorCommission
	}

	ticket.MatureBy = blockHeight + uint64(params.MinTicketMatDur)

	// Only validator tickets have a pre-determined decay height
	if t.Is(types.TxTypeValidatorTicket) {
		ticket.DecayBy = ticket.MatureBy + uint64(params.MaxTicketActiveDur)
	}

	// Add all tickets to the store
	if err := m.s.Add(ticket); err != nil {
		return err
	}

	return nil
}

// GetTopStorers gets storer tickets with the most total delegated value.
func (m *Manager) GetTopStorers(limit int) (types.SelectedTickets, error) {
	return m.getTopTickets(types.TxTypeStorerTicket, limit)
}

// GetTopValidators gets validator tickets with the most total delegated value.
func (m *Manager) GetTopValidators(limit int) (types.SelectedTickets, error) {
	return m.getTopTickets(types.TxTypeValidatorTicket, limit)
}

// getTopTickets finds tickets with the most delegated value
func (m *Manager) getTopTickets(ticketType, limit int) (types.SelectedTickets, error) {

	// Get the last committed block
	bi, err := m.logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		return nil, err
	}

	// Get active storer tickets
	activeTickets := m.s.Query(func(t *types.Ticket) bool {
		return t.Type == ticketType &&
			t.MatureBy <= uint64(bi.Height) &&
			(t.DecayBy > uint64(bi.Height) || t.DecayBy == 0)
	})

	// Create an index that maps a proposers to the sum of value of tickets
	// delegated to it. If a proposer already exist in the index, its value is
	// added to the total value of the existing ticket in the index.
	// While doing this, collect the selected tickets.
	var proposerIdx = make(map[string]*types.SelectedTicket)
	var selectedTickets []*types.SelectedTicket
	for _, ticket := range activeTickets {
		existingTicket, ok := proposerIdx[ticket.ProposerPubKey.HexStr()]
		if !ok {
			proposerIdx[ticket.ProposerPubKey.HexStr()] = &types.SelectedTicket{
				Ticket:     ticket,
				TotalValue: ticket.Value,
			}
			selectedTickets = append(selectedTickets, proposerIdx[ticket.ProposerPubKey.HexStr()])
			continue
		}
		updatedVal := existingTicket.TotalValue.Decimal().Add(ticket.Value.Decimal()).String()
		proposerIdx[ticket.ProposerPubKey.HexStr()].TotalValue = util.String(updatedVal)
	}

	// Sort the selected tickets by total delegated value
	sort.Slice(selectedTickets, func(i, j int) bool {
		itemI, itemJ := selectedTickets[i], selectedTickets[j]
		valI := itemI.TotalValue.Decimal()
		valJ := itemJ.TotalValue.Decimal()
		return valI.GreaterThan(valJ)
	})

	if limit > 0 && len(selectedTickets) >= limit {
		return selectedTickets[:limit], nil
	}

	return selectedTickets, nil
}

// Remove deletes a ticket by its hash
func (m *Manager) Remove(hash util.Bytes32) error {
	return m.s.RemoveByHash(hash)
}

// GetByProposer finds tickets belonging to the given proposer public key.
func (m *Manager) GetByProposer(
	ticketType int,
	proposerPubKey util.Bytes32,
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

// GetActiveTicketsByProposer returns all active tickets associated with a proposer
// proposer: The public key of the proposer
// ticketType: Filter the search to a specific ticket type
// addDelegated: When true, delegated tickets are added.
func (m *Manager) GetActiveTicketsByProposer(
	proposer util.Bytes32,
	ticketType int,
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
func (m *Manager) UpdateDecayBy(hash util.Bytes32, newDecayHeight uint64) error {
	m.s.UpdateOne(types.Ticket{DecayBy: newDecayHeight},
		func(t *types.Ticket) bool { return t.Hash == hash })
	return nil
}

// GetByHash get a ticket by hash
func (m *Manager) GetByHash(hash util.Bytes32) *types.Ticket {
	return m.QueryOne(func(t *types.Ticket) bool { return t.Hash == hash })
}

// Stop stores the manager
func (m *Manager) Stop() error {
	return nil
}
