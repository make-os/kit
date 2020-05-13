package core

import (
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/state"
)

// SystemContract represents a system contract
type SystemContract interface {

	// Init initializes the contract
	// logic is the logic manager
	// tx is the transaction to execute.
	// curChainHeight is the current height of the chain
	Init(logic Logic, tx types.BaseTx, curChainHeight uint64) SystemContract

	// CanExec checks whether the given tx type can be executed by the contract.
	CanExec(tx types.TxCode) bool

	// Exec executes the transaction
	Exec() error
}

// ProposalApplyArgs contains arguments passed to a proposal contract Apply function
type ProposalApplyArgs struct {
	Proposal    state.Proposal
	Repo        *state.Repository
	Keepers     Keepers
	ChainHeight uint64
}

// ProposalContract represents a system contract that is able to execute proposal transactions
// and apply proposal changes to the world state.
type ProposalContract interface {
	SystemContract

	// Apply is called when the proposal needs to be applied to the state.
	Apply(args *ProposalApplyArgs) error
}
