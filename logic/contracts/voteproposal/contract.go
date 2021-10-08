package voteproposal

import (
	"github.com/make-os/kit/crypto/ed25519"
	"github.com/make-os/kit/logic/contracts/common"
	"github.com/make-os/kit/types"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/types/txns"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
)

// Contract implements core.SystemContract. It is a system contract for adding a vote on a proposal.
type Contract struct {
	core.Keepers
	tx          *txns.TxRepoProposalVote
	chainHeight uint64
	contracts   []core.SystemContract
}

// NewContract creates a new instance of Contract
func NewContract() *Contract {
	return &Contract{}
}

func (c *Contract) CanExec(typ types.TxCode) bool {
	return typ == txns.TxTypeRepoProposalVote
}

// Init initialize the contract
func (c *Contract) Init(keepers core.Keepers, tx types.BaseTx, curChainHeight uint64) core.SystemContract {
	c.Keepers = keepers
	c.tx = tx.(*txns.TxRepoProposalVote)
	c.chainHeight = curChainHeight
	return c
}

// Exec executes the contract
func (c *Contract) Exec() error {

	var err error
	spk, _ := ed25519.PubKeyFromBytes(c.tx.SenderPubKey.Bytes())

	// Get the repo
	repoKeeper := c.RepoKeeper()
	repo := repoKeeper.Get(c.tx.RepoName)
	prop := repo.Proposals.Get(c.tx.ProposalID)

	increments := float64(0)

	// When proposers are the owners, and tally method is ProposalTallyMethodIdentity
	// each proposer will have 1 voting power.
	if *prop.Config.Voter == *state.VoterOwner.Ptr() &&
		*prop.Config.PropTallyMethod == *state.ProposalTallyMethodIdentity.Ptr() {
		increments = 1
	}

	// When proposers are the owners, and tally method is ProposalTallyMethodCoinWeighted
	// each proposer will use the value of the voter's spendable account balance
	// as their voting power.
	if *prop.Config.Voter == *state.VoterOwner.Ptr() &&
		*prop.Config.PropTallyMethod == *state.ProposalTallyMethodCoinWeighted.Ptr() {
		senderAcct := c.AccountKeeper().Get(spk.Addr())
		increments = senderAcct.GetAvailableBalance(c.chainHeight).Float()
	}

	// For network staked-weighted votes, use the total value of coins directly
	// staked by the voter as their vote power
	if *prop.Config.PropTallyMethod == *state.ProposalTallyMethodNetStakeNonDelegated.Ptr() {
		increments, err = c.GetTicketManager().
			ValueOfNonDelegatedTickets(c.tx.SenderPubKey.ToBytes32(), prop.PowerAge.UInt64())
		if err != nil {
			return errors.Wrap(err, "failed to get value of non-delegated tickets of sender")
		}
	}

	// For network staked-weighted votes, use the total value of coins delegated
	// to the voter as their vote power
	if *prop.Config.PropTallyMethod == *state.ProposalTallyMethodNetStakeOfDelegators.Ptr() {
		increments, err = c.GetTicketManager().
			ValueOfDelegatedTickets(c.tx.SenderPubKey.ToBytes32(), prop.PowerAge.UInt64())
		if err != nil {
			return errors.Wrap(err, "failed to get value of delegated tickets of sender")
		}
	}

	// For network staked-weighted votes, use the total value of coins delegated
	// to the voter as their vote power
	if *prop.Config.PropTallyMethod == *state.ProposalTallyMethodNetStake.Ptr() {

		tickets, err := c.GetTicketManager().GetUnExpiredTickets(c.tx.SenderPubKey.ToBytes32(),
			prop.PowerAge.UInt64())
		if err != nil {
			return errors.Wrap(err, "failed to get unexpired tickets assigned to sender")
		}

		// Calculate the sum of value of all tickets.
		// For delegated tickets, check whether the delegator already voted. If
		// yes, do not count their ticket.
		sumValue := decimal.Zero
		for _, ticket := range tickets {

			proposerPK := ticket.ProposerPubKey

			// Count the ticket if it is not delegated or the delegator is also the voter
			if ticket.Delegator == "" || (ticket.Delegator == spk.Addr().String() &&
				proposerPK.Equal(c.tx.SenderPubKey.ToBytes32())) {
				sumValue = sumValue.Add(ticket.Value.Decimal())
				continue
			}

			// For tickets not delegated by the voter, determine whether the
			// delegator has used their ticket to vote on this same proposal.
			// If yes, we will not count it.
			if ticket.Delegator != spk.Addr().String() {
				_, voted, err := repoKeeper.GetProposalVote(c.tx.RepoName, c.tx.ProposalID, ticket.Delegator)
				if err != nil {
					return errors.Wrap(err, "failed to check ticket's delegator vote status")
				}
				if !voted {
					sumValue = sumValue.Add(ticket.Value.Decimal())
					continue
				}
			}

			// For tickets delegated by the voter to a different user,
			// determine if ticket proposer has voted in this same proposal.
			// If yes, deduct the vote and apply to the delegator's choice vote option
			if ticket.Delegator == spk.Addr().String() {
				proposerAddr := ed25519.MustPubKeyFromBytes(proposerPK.Bytes()).Addr().String()
				vote, voted, err := repoKeeper.GetProposalVote(c.tx.RepoName, c.tx.ProposalID, proposerAddr)
				if err != nil {
					return errors.Wrap(err, "failed to check ticket's proposer vote status")
				}
				if !voted {
					sumValue = sumValue.Add(ticket.Value.Decimal())
					continue
				}

				switch vote {
				case state.ProposalVoteYes:
					newYes := decimal.NewFromFloat(prop.Yes)
					newYes = newYes.Sub(ticket.Value.Decimal())
					prop.Yes, _ = newYes.Float64()

				case state.ProposalVoteNo:
					newNo := decimal.NewFromFloat(prop.No)
					newNo = newNo.Sub(ticket.Value.Decimal())
					prop.Yes, _ = newNo.Float64()

				case state.ProposalVoteNoWithVeto:
					newNoWithVeto := decimal.NewFromFloat(prop.NoWithVeto)
					newNoWithVeto = newNoWithVeto.Sub(ticket.Value.Decimal())
					prop.NoWithVeto, _ = newNoWithVeto.Float64()

				case state.ProposalVoteAbstain:
					newAbstain := decimal.NewFromFloat(prop.Abstain)
					newAbstain = newAbstain.Sub(ticket.Value.Decimal())
					prop.Abstain, _ = newAbstain.Float64()
				}

				sumValue = sumValue.Add(ticket.Value.Decimal())
			}
		}

		increments, _ = sumValue.Float64()
	}

	switch c.tx.Vote {
	case state.ProposalVoteYes:
		prop.Yes += increments
	case state.ProposalVoteNo:
		prop.No += increments
	case state.ProposalVoteAbstain:
		prop.Abstain += increments
	case state.ProposalVoteNoWithVeto:
		prop.NoWithVeto += increments

		// Also, if the proposer type for the proposal is stakeholders and veto
		// owners and voter is an owner, increment NoWithVetoByOwners by 1
		voterOwnerObj := repo.Owners.Get(spk.Addr().String())
		isStakeholderAndVetoOwnerProposer := *prop.Config.Voter == *state.VoterNetStakersAndVetoOwner.Ptr()
		if isStakeholderAndVetoOwnerProposer && voterOwnerObj != nil && voterOwnerObj.Veto {
			prop.NoWithVetoByOwners = 1
		}
	}

	// Update the repo
	repoKeeper.Update(c.tx.RepoName, repo)

	// Deduct fee from sender
	common.DebitAccount(c, spk, c.tx.Fee.Decimal(), c.chainHeight)

	return nil
}
