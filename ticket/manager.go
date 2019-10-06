package ticket

import (
	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/params"
	"github.com/makeos/mosdef/types"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
)

// Manager implements types.TicketManager.
// It provides ticket management functionalities.
type Manager struct {
	store Store
	cfg   *config.EngineConfig
	logic types.Logic
}

// NewManager returns an instance of Manager.
// Returns error if unable to initialize the store.
func NewManager(cfg *config.EngineConfig, logic types.Logic) (*Manager, error) {
	mgr := &Manager{cfg: cfg, logic: logic}
	store, err := NewSQLStore(cfg.GetTicketDBDir())
	if err != nil {
		return nil, errors.Wrap(err, "failed to start manager")
	}
	mgr.store = store
	return mgr, nil
}

// Index takes a tx and creates a ticket out of it
func (m *Manager) Index(
	tx *types.Transaction,
	proposerPubKey string,
	blockHeight uint64,
	txIndex int) error {

	tickets := []*types.Ticket{}

	// Create the ticket
	ticket := &types.Ticket{
		Hash:           tx.GetHash().HexStr(),
		ProposerPubKey: proposerPubKey,
		Height:         blockHeight,
		Index:          txIndex,
		Value:          tx.Value.String(),
	}

	// Set maturity and decay heights
	ticket.MatureBy = blockHeight + uint64(params.MinTicketMatDur)
	ticket.DecayBy = ticket.MatureBy + uint64(params.MaxTicketActiveDur)

	// Add the ticket
	tickets = append(tickets, ticket)

	// Determine if a child ticket can be created.
	// Child tickets are created when the value of the ticket
	// is sufficient to purchase additional tickets
	curTickPrice := decimal.NewFromFloat(m.logic.Sys().GetCurValidatorTicketPrice())
	numSubTickets := tx.Value.Decimal().Div(curTickPrice).IntPart()
	for i := int64(1); i < numSubTickets; i++ {
		childTicket := *ticket
		childTicket.ChildOf = ticket.Hash
		childTicket.Index = int(i) - 1
		tickets = append(tickets, &childTicket)
	}

	// Add all tickets to the store
	if err := m.store.Add(tickets...); err != nil {
		return err
	}

	return nil
}

// GetByProposer finds tickets belonging to the
// given proposer public key.
func (m *Manager) GetByProposer(proposerPubKey string, queryOpt types.QueryOptions) ([]*types.Ticket, error) {
	res, err := m.store.Query(types.Ticket{ProposerPubKey: proposerPubKey}, queryOpt)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// CountLiveTickets returns the number of matured and non-decayed tickets.
func (m *Manager) CountLiveTickets(queryOpt ...types.QueryOptions) (int, error) {

	qOpt := types.EmptyQueryOptions
	if len(queryOpt) > 0 {
		qOpt = queryOpt[0]
	}

	// Get the last committed block
	bi, err := m.logic.SysKeeper().GetLastBlockInfo()
	if err != nil {
		return 0, err
	}

	count, err := m.store.CountLive(bi.Height, qOpt)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// Stop stores the manager
func (m *Manager) Stop() error {
	if m.store != nil {
		if err := m.store.Close(); err != nil {
			return err
		}
	}
	return nil
}
