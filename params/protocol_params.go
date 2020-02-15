package params

import (
	"time"

	"github.com/shopspring/decimal"
)

const (
	// AddressVersion is the base58 encode version adopted
	AddressVersion byte = 92

	// PublicKeyVersion is the base58 encode version adopted for public keys
	PublicKeyVersion byte = 93

	// PrivateKeyVersion is the base58 encode version adopted for private keys
	PrivateKeyVersion byte = 94
)

var (
	// BlockTime is the number of seconds between blocks
	BlockTime = 15

	// MaxBlockSize is the max size of a block
	MaxBlockSize = int64(1000000)

	// FeePerByte is the cost per byte of a transaction
	FeePerByte = decimal.NewFromFloat(0.00001)

	// TxTTL is the number of days a transaction
	// can last for in the pool
	TxTTL = 7

	// TxPoolCap is the number of transactions the tx pool can contain
	TxPoolCap = int64(10000)

	// MinTicketMatDur is the number of blocks that must be created
	// before a ticket is considered matured.
	MinTicketMatDur = 3

	// MaxTicketActiveDur is the number of blocks before a matured
	// ticket is considered spent or decayed.
	MaxTicketActiveDur = 100

	// NumBlocksInThawPeriod is the number of blocks a decayed ticket will
	// exist for before it can be unbonded
	NumBlocksInThawPeriod = 10

	// MinValidatorsTicketPrice is the minimum price of a ticket
	MinValidatorsTicketPrice = float64(100)

	// NumBlocksPerPriceWindow is the number of blocks before price is increased
	NumBlocksPerPriceWindow = 100

	// PricePercentIncrease is the percentage increase of ticket price
	PricePercentIncrease = float64(0.2)

	// MaxValTicketsPerBlock is the max number of validators
	// ticket transaction a block can include.
	MaxValTicketsPerBlock = 1

	// NumBlocksPerEpoch is the number of blocks in an epoch
	NumBlocksPerEpoch = 5

	// NumBlocksToEffectValChange is the number of block tendermint uses to
	// effect validation change.
	NumBlocksToEffectValChange = 2

	// MaxValidatorsPerEpoch is the maximum number validators per epoch
	MaxValidatorsPerEpoch = 1

	// MinDelegatorCommission is the number of percentage delegators pay validators
	MinDelegatorCommission = decimal.NewFromFloat(10)

	// MinStorerStake is the minimum stake for a storer ticket
	MinStorerStake = decimal.NewFromFloat(10)

	// NumBlocksInStorerThawPeriod is the number of blocks before a storer stake
	// is unbonded
	NumBlocksInStorerThawPeriod = 10

	// ValidatorTicketPoolSize is the size of the ticket pool from which validator
	// tickets are selected randomly
	ValidatorTicketPoolSize = 60000

	// PushPoolCap is the pool transaction capacity
	PushPoolCap = 1000

	// PushPoolCleanUpInt is duration between each push pool clean-up operation
	PushPoolCleanUpInt = 30 * time.Minute

	// PushPoolItemTTL is the maximum life time of an item in the push pool
	PushPoolItemTTL = 24 * 3 * time.Hour

	// UnfinalizedObjectsCacheSize is the max size for unfinalized objects cache
	UnfinalizedObjectsCacheSize = 10000

	// PushObjectsSendersCacheSize is the max size for push note senders cache
	PushObjectsSendersCacheSize = 5000

	// PushNotesEndorsementsCacheSize is the max size for push note senders cache
	PushNotesEndorsementsCacheSize = 5000

	// RepoPrunerTickDur is the duration between each repo pruning operation
	RepoPrunerTickDur = 10 * time.Second

	// PushOKQuorumSize is the minimum number of PushOKs a push note requires
	// for approval
	PushOKQuorumSize = 2

	// NumTopStorersLimit is maximum the number of top storers
	NumTopStorersLimit = 21

	// CostOfNamespace is the amount of native coin required to obtain a
	// repo namespace
	CostOfNamespace = decimal.NewFromFloat(1)

	// MaxNamespaceRevealDur is the number of blocks within which a namespace
	// must be revealed
	MaxNamespaceRevealDur = 10

	// NamespaceTTL is the number of blocks of a namespace life span
	NamespaceTTL = 10

	// NamespaceGraceDur is the number of blocks before a namespace expires
	NamespaceGraceDur = 10

	// TreasuryAddress is the address where treasury-bound payments are deposited
	TreasuryAddress = "e4Tkr4AMxhPPjptDSMzX98F2BwHvQM2DKx"
)

// Repo config
var (
	// RepoProposalDur is the number of blocks a repo proposal can remain active
	RepoProposalDur = uint64(10)
	// RepoProposalQuorum is the minimum percentage of voters required to
	// consider a proposal valid.
	RepoProposalQuorum = float64(10)
	// RepoProposalThreshold is the minimum percentage required to consider a
	// proposal accepted ("YES" voted)
	RepoProposalThreshold = float64(51)
	// RepoProposalVetoQuorum is the minimum percentage required for veto
	// members to overturn a "Yes" quorum
	RepoProposalVetoQuorum = float64(33)
	// RepoProposalVetoOwnersQuorum is the minimum percentage required for veto
	// members to overturn a "Yes" quorum in a proposal where stakeholder and
	// owners are eligible to vote
	RepoProposalVetoOwnersQuorum = float64(0)
	// MinProposalFee is the minimum fee to be paid for each new proposal
	// NOTE: This should probably be set to zero, otherwise every proposal (even
	// by owners) will require an additional fee.
	MinProposalFee = float64(0)
	// HelmProposalFeeSplit is the percentage of proposal fee distributed to the
	// helm repo
	HelmProposalFeeSplit = 0.2
	// TargetRepoProposalFeeSplit is the percentage of proposal fee distributed to the
	// repo that received and resolved a proposal
	TargetRepoProposalFeeSplit = 0.8
)
