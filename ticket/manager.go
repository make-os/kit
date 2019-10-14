package ticket

import (
	"math/big"
	"math/rand"

	"github.com/makeos/mosdef/crypto"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/params"
	"github.com/makeos/mosdef/types"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/thoas/go-funk"
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
func (m *Manager) Index(tx *types.Transaction, blockHeight uint64, txIndex int) error {

	ticket := &types.Ticket{}

	// By default the proposer is the creator of the transaction.
	// However, if the transaction `to` field is set, the sender
	// is delegating the ticket to the public key set in `to`
	var proposerPubKey = tx.SenderPubKey.String()
	if tx.To.String() != "" {
		proposerPubKey = tx.To.String()
		ticket.Delegator = tx.GetFrom().String()

		// Since this is a delegated ticket, we need to get the
		// proposer's commission rate from their account
		pk, _ := crypto.PubKeyFromBase58(proposerPubKey)
		proposerAcct := m.logic.AccountKeeper().GetAccount(pk.Addr())
		ticket.CommissionRate = proposerAcct.DelegatorCommission
	}

	ticket.Hash = tx.GetHash().HexStr()
	ticket.ProposerPubKey = proposerPubKey
	ticket.Height = blockHeight
	ticket.Index = txIndex
	ticket.Value = tx.Value.String()

	// Set maturity and decay heights
	ticket.MatureBy = blockHeight + uint64(params.MinTicketMatDur)
	ticket.DecayBy = ticket.MatureBy + uint64(params.MaxTicketActiveDur)

	// Determine the ticket's power.
	// A tickets power is the amount of tickets is value can purchase
	curTickPrice := decimal.NewFromFloat(m.logic.Sys().GetCurValidatorTicketPrice())
	numSubTickets := tx.Value.Decimal().Div(curTickPrice).IntPart()
	ticket.Power = numSubTickets

	// Add all tickets to the store
	if err := m.store.Add(ticket); err != nil {
		return err
	}

	return nil
}

// Remove deletes a ticket by its hash
func (m *Manager) Remove(hash string) error {
	return m.store.Remove(hash)
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

// Query finds and returns tickets that match the given query
func (m *Manager) Query(q types.Ticket, queryOpt ...types.QueryOptions) ([]*types.Ticket, error) {
	qOpt := types.EmptyQueryOptions
	if len(queryOpt) > 0 {
		qOpt = queryOpt[0]
	}
	return m.store.Query(q, qOpt)
}

// QueryOne finds and returns a ticket that match the given query
func (m *Manager) QueryOne(q types.Ticket, queryOpt ...types.QueryOptions) (*types.Ticket, error) {
	qOpt := types.EmptyQueryOptions
	if len(queryOpt) > 0 {
		qOpt = queryOpt[0]
	}
	return m.store.QueryOne(q, qOpt)
}

// MarkAsUnbonded sets a ticket unbonded status to true
func (m *Manager) MarkAsUnbonded(hash string) error {
	return m.store.MarkAsUnbonded(hash)
}

// SelectRandom selects random live tickets up to the specified limit.
// The provided see is used to seed the PRNG that is used to select tickets.
func (m *Manager) SelectRandom(height int64, seed []byte, limit int) ([]*types.Ticket, error) {

	tickets, err := m.store.GetLive(height, types.QueryOptions{
		Order: `"power" desc, "height" asc, "index" asc`,
		Limit: 50000,
	})
	if err != nil {
		return nil, err
	}

	// Create a RNG sourced with the seed
	seedInt := new(big.Int).SetBytes(seed)
	r := rand.New(rand.NewSource(seedInt.Int64()))

	// Select random tickets up to the given limit.
	// Note: Only 1 slot per public key.
	selected := map[string]*types.Ticket{}
	for len(selected) < limit && len(tickets) > 0 {

		// Select a candidate ticket and remove it from the list
		i := r.Intn(len(tickets))
		candidate := tickets[i]
		tickets = append(tickets[:i], tickets[i+1:]...)

		// If the candidate has already been selected, ignore
		if _, ok := selected[candidate.ProposerPubKey]; ok {
			continue
		}

		selected[candidate.ProposerPubKey] = candidate
	}

	return funk.Values(selected).([]*types.Ticket), nil
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
