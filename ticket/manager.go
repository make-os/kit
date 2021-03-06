package ticket

import (
	"sort"

	storagetypes "github.com/make-os/kit/storage/types"
	tickettypes "github.com/make-os/kit/ticket/types"
	"github.com/make-os/kit/types"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util"
	"github.com/shopspring/decimal"

	"github.com/make-os/kit/crypto/ed25519"

	"github.com/make-os/kit/config"
	"github.com/make-os/kit/params"
)

// Manager implements types.TicketManager.
// It provides ticket management functionalities.
type Manager struct {
	cfg   *config.AppConfig
	logic core.Logic
	s     TicketStore
}

// NewManager returns an instance of Manager.
// Returns error if unable to initialize the store.
func NewManager(db storagetypes.Tx, cfg *config.AppConfig, logic core.Logic) *Manager {
	mgr := &Manager{cfg: cfg, logic: logic}
	mgr.s = NewStore(db)
	return mgr
}

// Index takes a tx and creates a ticket out of it
func (m *Manager) Index(tx types.BaseTx, blockHeight uint64, txIndex int) error {

	t := tx.(*txns.TxTicketPurchase)

	ticket := &tickettypes.Ticket{
		Type:           tx.GetType(),
		Height:         blockHeight,
		Index:          txIndex,
		Value:          t.Value,
		Hash:           t.GetHash(),
		ProposerPubKey: t.GetSenderPubKey().ToBytes32(),
		BLSPubKey:      t.BLSPubKey,
	}

	// By default the proposer is the creator of the transaction.
	// However, if the transaction `delegate` field is set, the sender
	// is delegating the ticket to the public key set in `delegate`
	if !t.Delegate.IsEmpty() {

		// Set the given delegate as the proposer
		ticket.ProposerPubKey = t.Delegate.ToBytes32()

		// Set the sender address as the delegator
		ticket.Delegator = t.GetFrom().String()

		// Since this is a delegated ticket, we need to get the proposer's
		// commission rate from their account, write it to the ticket so that it
		// is locked and immutable by a future commission rate update.
		pk := ed25519.MustPubKeyFromBytes(ticket.ProposerPubKey.Bytes())
		proposerAcct := m.logic.AccountKeeper().Get(pk.Addr())
		ticket.CommissionRate = proposerAcct.DelegatorCommission
	}

	ticket.MatureBy = blockHeight + uint64(params.MinTicketMatDur)

	// Only validator tickets have a pre-determined expire height
	if t.Is(txns.TxTypeValidatorTicket) {
		ticket.ExpireBy = ticket.MatureBy + uint64(params.MaxTicketActiveDur)
	}

	// Register all tickets to the store
	if err := m.s.Add(ticket); err != nil {
		return err
	}

	return nil
}

// GetTopHosts gets host tickets with the most total delegated value.
func (m *Manager) GetTopHosts(limit int) (tickettypes.SelectedTickets, error) {
	return m.getTopTickets(txns.TxTypeHostTicket, limit)
}

// GetTopValidators gets validator tickets with the most total delegated value.
func (m *Manager) GetTopValidators(limit int) (tickettypes.SelectedTickets, error) {
	return m.getTopTickets(txns.TxTypeValidatorTicket, limit)
}

// getTopTickets finds tickets with the most delegated value
func (m *Manager) getTopTickets(ticketType types.TxCode, limit int) (tickettypes.SelectedTickets, error) {

	// Get the last committed block
	bi, err := m.logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		return nil, err
	}

	// Get active host tickets
	activeTickets := m.s.Query(func(t *tickettypes.Ticket) bool {
		return t.Type == ticketType &&
			t.MatureBy <= uint64(bi.Height) &&
			(t.ExpireBy > uint64(bi.Height) || t.ExpireBy == 0)
	})

	// Create an index that maps a proposers to the sum of value of tickets
	// delegated to it. If a proposer already exist in the index, its value is
	// added to the total value of the existing ticket in the index.
	// While doing this, collect the selected tickets.
	var proposerIdx = make(map[string]*tickettypes.SelectedTicket)
	var selectedTickets []*tickettypes.SelectedTicket
	for _, ticket := range activeTickets {
		existingTicket, ok := proposerIdx[ticket.ProposerPubKey.HexStr()]
		if !ok {
			proposerIdx[ticket.ProposerPubKey.HexStr()] = &tickettypes.SelectedTicket{
				Ticket: ticket,
				Power:  ticket.Value,
			}
			selectedTickets = append(selectedTickets, proposerIdx[ticket.ProposerPubKey.HexStr()])
			continue
		}
		updatedVal := existingTicket.Power.Decimal().Add(ticket.Value.Decimal()).String()
		proposerIdx[ticket.ProposerPubKey.HexStr()].Power = util.String(updatedVal)
	}

	// Sort the selected tickets by total delegated value
	sort.Slice(selectedTickets, func(i, j int) bool {
		itemI, itemJ := selectedTickets[i], selectedTickets[j]
		valI := itemI.Power.Decimal()
		valJ := itemJ.Power.Decimal()
		return valI.GreaterThan(valJ)
	})

	if limit > 0 && len(selectedTickets) >= limit {
		return selectedTickets[:limit], nil
	}

	return selectedTickets, nil
}

// Remove deletes a ticket by its hash
func (m *Manager) Remove(hash util.HexBytes) error {
	return m.s.RemoveByHash(hash)
}

// GetByProposer finds tickets belonging to the given proposer public key.
func (m *Manager) GetByProposer(
	ticketType types.TxCode,
	proposerPubKey util.Bytes32,
	queryOpt ...interface{}) ([]*tickettypes.Ticket, error) {

	bi, err := m.logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		return nil, err
	}

	qo := getQueryOptions(queryOpt...)

	res := m.s.Query(func(t *tickettypes.Ticket) bool {
		ok := true
		if t.Type != ticketType || t.ProposerPubKey != proposerPubKey {
			ok = false
		}
		if ok && qo.Immature && t.MatureBy <= uint64(bi.Height) { // reject mature
			ok = false
		}
		if ok && qo.Matured && t.MatureBy > uint64(bi.Height) { // reject immature
			ok = false
		}
		if ok && qo.Expired && t.ExpireBy > uint64(bi.Height) {
			ok = false
		}
		if ok && qo.Active && t.ExpireBy <= uint64(bi.Height) {
			ok = false
		}
		return ok
	}, queryOpt...)

	return res, nil
}

// GetUnExpiredTickets finds unexpired tickets that have the given proposer
// public key as the proposer or the delegator;
// If maturityHeight is non-zero, then only tickets that reached maturity before
// or on the given height are selected. Otherwise, the current chain height is used.
func (m *Manager) GetUnExpiredTickets(pubKey util.Bytes32, maturityHeight uint64) ([]*tickettypes.Ticket, error) {

	bi, err := m.logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		return nil, err
	}

	if maturityHeight <= 0 {
		maturityHeight = uint64(bi.Height)
	}

	pk, err := ed25519.PubKeyFromBytes(pubKey.Bytes())
	if err != nil {
		return nil, err
	}

	result := m.s.Query(func(t *tickettypes.Ticket) bool {
		return t.MatureBy <= maturityHeight && // is mature
			(t.ExpireBy > uint64(bi.Height) ||
				(t.ExpireBy == 0 && t.Type == txns.TxTypeHostTicket)) && // not expired
			(t.ProposerPubKey == pubKey || t.Delegator == pk.Addr().String()) // is delegator or not
	})

	return result, nil
}

// CountActiveValidatorTickets returns the number of matured and unexpired tickets.
func (m *Manager) CountActiveValidatorTickets() (int, error) {

	// Get the last committed block
	bi, err := m.logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		return 0, err
	}

	count := m.s.Count(func(t *tickettypes.Ticket) bool {
		return t.Type == txns.TxTypeValidatorTicket &&
			t.MatureBy <= uint64(bi.Height) &&
			t.ExpireBy > uint64(bi.Height)
	})

	return count, nil
}

// GetNonDelegatedTickets returns all non-delegated,
// unexpired tickets belonging to the given public key.
//
// pubKey: The public key of the proposer
// ticketType: Filter the search to a specific ticket type
func (m *Manager) GetNonDelegatedTickets(
	pubKey util.Bytes32,
	ticketType types.TxCode) ([]*tickettypes.Ticket, error) {

	// Get the last committed block
	bi, err := m.logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		return nil, err
	}

	result := m.s.Query(func(t *tickettypes.Ticket) bool {
		return t.Type == ticketType &&
			t.MatureBy <= uint64(bi.Height) &&
			(t.ExpireBy > uint64(bi.Height) || (t.ExpireBy == 0 && t.Type == txns.TxTypeHostTicket)) &&
			t.ProposerPubKey == pubKey &&
			t.Delegator == ""
	})

	return result, nil
}

// ValueOfNonDelegatedTickets returns the sum of value of all
// non-delegated, unexpired tickets which has the given public
// key as the proposer; Includes both validator and host tickets.
//
// pubKey: The public key of the proposer
// maturityHeight: if set to non-zero, only tickets that reached maturity before
// or on the given height are selected. Otherwise, the current chain height is used.
func (m *Manager) ValueOfNonDelegatedTickets(
	pubKey util.Bytes32,
	maturityHeight uint64) (float64, error) {

	// Get the last committed block
	bi, err := m.logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		return 0, err
	}

	if maturityHeight <= 0 {
		maturityHeight = uint64(bi.Height)
	}

	result := m.s.Query(func(t *tickettypes.Ticket) bool {
		return t.MatureBy <= maturityHeight &&
			(t.ExpireBy > uint64(bi.Height) || (t.ExpireBy == 0 && t.Type == txns.TxTypeHostTicket)) &&
			t.ProposerPubKey == pubKey &&
			t.Delegator == ""
	})

	var sum = decimal.Zero
	for _, res := range result {
		sum = sum.Add(res.Value.Decimal())
	}

	sumF, _ := sum.Float64()
	return sumF, nil
}

// ValueOfDelegatedTickets returns the sum of value of all
// delegated, unexpired tickets which has the given public
// key as the proposer; Includes both validator and host tickets.
//
// pubKey: The public key of the proposer
// maturityHeight: if set to non-zero, only tickets that reached maturity before
// or on the given height are selected. Otherwise, the current chain height is used.
func (m *Manager) ValueOfDelegatedTickets(
	pubKey util.Bytes32,
	maturityHeight uint64) (float64, error) {

	// Get the last committed block
	bi, err := m.logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		return 0, err
	}

	if maturityHeight <= 0 {
		maturityHeight = uint64(bi.Height)
	}

	result := m.s.Query(func(t *tickettypes.Ticket) bool {
		return t.MatureBy <= maturityHeight &&
			(t.ExpireBy > uint64(bi.Height) || (t.ExpireBy == 0 && t.Type == txns.TxTypeHostTicket)) &&
			t.ProposerPubKey == pubKey &&
			t.Delegator != ""
	})

	var sum = decimal.Zero
	for _, res := range result {
		sum = sum.Add(res.Value.Decimal())
	}

	sumF, _ := sum.Float64()
	return sumF, nil
}

// ValueOfTickets returns the sum of value of all unexpired
// tickets where the given public key is the proposer or delegator;
// Includes both validator and host tickets.
//
// pubKey: The public key of the proposer
// maturityHeight: if set to non-zero, only tickets that reached maturity before
// or on the given height are selected. Otherwise, the current chain height is used.
func (m *Manager) ValueOfTickets(
	pubKey util.Bytes32,
	maturityHeight uint64) (float64, error) {

	// Get the last committed block
	bi, err := m.logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		return 0, err
	}

	if maturityHeight <= 0 {
		maturityHeight = uint64(bi.Height)
	}

	pk, err := ed25519.PubKeyFromBytes(pubKey.Bytes())
	if err != nil {
		return 0, err
	}

	result := m.s.Query(func(t *tickettypes.Ticket) bool {
		return t.MatureBy <= maturityHeight && // is mature
			(t.ExpireBy > uint64(bi.Height) ||
				(t.ExpireBy == 0 && t.Type == txns.TxTypeHostTicket)) && // not expired
			(t.ProposerPubKey == pubKey || t.Delegator == pk.Addr().String()) // is delegated or not
	})

	var sum = decimal.Zero
	for _, res := range result {
		sum = sum.Add(res.Value.Decimal())
	}

	sumF, _ := sum.Float64()
	return sumF, nil
}

// ValueOfAllTickets returns the sum of value of all unexpired
// tickets; Includes both validator and host tickets.
//
// maturityHeight: if set to non-zero, only tickets that reached maturity before
// or on the given height are selected. Otherwise, the current chain height is used.
func (m *Manager) ValueOfAllTickets(maturityHeight uint64) (float64, error) {

	// Get the last committed block
	bi, err := m.logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		return 0, err
	}

	if maturityHeight <= 0 {
		maturityHeight = uint64(bi.Height)
	}

	result := m.s.Query(func(t *tickettypes.Ticket) bool {
		return t.MatureBy <= maturityHeight && // is mature
			(t.ExpireBy > uint64(bi.Height) || (t.ExpireBy == 0 && t.Type == txns.TxTypeHostTicket)) // not expired
	})

	var sum = decimal.Zero
	for _, res := range result {
		sum = sum.Add(res.Value.Decimal())
	}

	sumF, _ := sum.Float64()
	return sumF, nil
}

// Query finds and returns tickets that match the given query
func (m *Manager) Query(qf func(t *tickettypes.Ticket) bool, queryOpt ...interface{}) []*tickettypes.Ticket {
	return m.s.Query(qf, queryOpt...)
}

// QueryOne finds and returns a ticket that match the given query
func (m *Manager) QueryOne(qf func(t *tickettypes.Ticket) bool) *tickettypes.Ticket {
	return m.s.QueryOne(qf)
}

// UpdateExpireBy updates the expire height of a ticket
func (m *Manager) UpdateExpireBy(hash util.HexBytes, newExpireHeight uint64) error {
	m.s.UpdateOne(tickettypes.Ticket{ExpireBy: newExpireHeight},
		func(t *tickettypes.Ticket) bool { return t.Hash.Equal(hash) })
	return nil
}

// GetByHash get a ticket by hash
func (m *Manager) GetByHash(hash util.HexBytes) *tickettypes.Ticket {
	return m.QueryOne(func(t *tickettypes.Ticket) bool { return t.Hash.Equal(hash) })
}

// Stop stores the manager
func (m *Manager) Stop() error {
	return nil
}
