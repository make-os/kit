package types

import "github.com/mitchellh/mapstructure"

// ProposeeType represents a type of repo proposal proposee
type ProposeeType int

// Proposee types
const (
	ProposeeOwner ProposeeType = iota + 1
	ProposeeNetStakeholders
	ProposeeAll
)

// ProposalTallyMethod represents a type for repo proposal counting method
type ProposalTallyMethod int

// ProposalTallyMethod types
const (
	ProposalTallyMethodIdentity = iota + 1
	ProposalTallyMethodCoinWeighted
)

// ProposalAction represents proposal action types
type ProposalAction int

// Proposal actions
const (
	ProposalActionAddOwner ProposalAction = iota + 1
)

// Proposal vote choices
const (
	ProposalVoteYes        = 1
	ProposalVoteNo         = 0
	ProposalVoteNoWithVeto = -1
)

// Proposal describes a repository proposal
type Proposal interface {
	GetCreator() string
	GetProposeeType() ProposeeType
	GetProposeeMaxJoinHeight() uint64
	GetEndAt() uint64
	GetQuorum() float64
	GetTallyMethod() ProposalTallyMethod
	GetAction() ProposalAction
	GetActionData() map[string]interface{}
	GetThreshold() float64
	GetVetoQuorum() float64
	GetAccepted() float64
	GetRejected() float64
	GetRejectedWithVeto() float64
	IsFinalized() bool
	SetOutcome(v ProposalOutcome)
}

// ProposalOutcome describes a proposal outcome
type ProposalOutcome int

// Proposal outcomes
const (
	ProposalOutcomeAccepted ProposalOutcome = iota + 1
	ProposalOutcomeRejected
	ProposalOutcomeRejectedWithVeto
	ProposalOutcomeQuorumNotMet
	ProposalOutcomeThresholdNotMet
	ProposalOutcomeTie
)

// RepoProposal represents a repository proposal
type RepoProposal struct {
	Action                ProposalAction         `json:"action" mapstructure:"action" msgpack:"action"`                                              // The action type.
	ActionData            map[string]interface{} `json:"actionData" mapstructure:"actionData" msgpack:"actionData"`                                  // The data to use to perform the action.
	Creator               string                 `json:"creator" mapstructure:"creator" msgpack:"creator"`                                           // The creator is the address of the proposal creator.
	Proposee              ProposeeType           `json:"proposee" mapstructure:"proposee" msgpack:"proposee"`                                        // The set of participants allowed to vote on the proposal.
	ProposeeMaxJoinHeight uint64                 `json:"proposeeMaxJoinHeight" mapstructure:"proposeeMaxJoinHeight" msgpack:"proposeeMaxJoinHeight"` // Used to allow proposee that are active before a specific height.
	EndAt                 uint64                 `json:"endAt" mapstructure:"endAt" msgpack:"endAt"`                                                 // Used to close the proposal after the given height.
	TallyMethod           ProposalTallyMethod    `json:"tallyMethod" mapstructure:"tallyMethod" msgpack:"tallyMethod"`                               // Tally method describes how the votes are counted.
	Quorum                float64                `json:"quorum" mapstructure:"quorum" msgpack:"quorum"`                                              // Quorum describes what the majority is.
	Threshold             float64                `json:"threshold" mapstructure:"threshold" msgpack:"threshold"`                                     // Thresholds describes the minimum "Yes" quorum required to consider a proposal valid.
	VetoQuorum            float64                `json:"vetoQuorum" mapstructure:"vetoQuorum" msgpack:"vetoQuorum"`                                  // Veto quorum describes the quorum among veto-powered proposees required to stop a proposal.
	Yes                   float64                `json:"yes" mapstructure:"yes" msgpack:"yes"`                                                       // Count of "Yes" votes
	No                    float64                `json:"no" mapstructure:"no" msgpack:"no"`                                                          // Count of "No" votes
	NoWithVeto            float64                `json:"noWithVeto" mapstructure:"noWithVeto" msgpack:"noWithVeto"`                                  // Count of "No" votes
	Outcome               ProposalOutcome        `json:"outcome" mapstructure:"outcome" msgpack:"outcome"`
}

// GetCreator implements Proposal
func (p *RepoProposal) GetCreator() string {
	return p.Creator
}

// IsFinalized implements Proposal
func (p *RepoProposal) IsFinalized() bool {
	return p.Outcome > 0
}

// SetOutcome implements Proposal
func (p *RepoProposal) SetOutcome(v ProposalOutcome) {
	p.Outcome = v
}

// GetProposeeType implements Proposal
func (p *RepoProposal) GetProposeeType() ProposeeType {
	return p.Proposee
}

// GetProposeeMaxJoinHeight implements Proposal
func (p *RepoProposal) GetProposeeMaxJoinHeight() uint64 {
	return p.ProposeeMaxJoinHeight
}

// GetEndAt implements Proposal
func (p *RepoProposal) GetEndAt() uint64 {
	return p.EndAt
}

// GetQuorum implements Proposal
func (p *RepoProposal) GetQuorum() float64 {
	return p.Quorum
}

// GetTallyMethod implements Proposal
func (p *RepoProposal) GetTallyMethod() ProposalTallyMethod {
	return p.TallyMethod
}

// GetAction implements Proposal
func (p *RepoProposal) GetAction() ProposalAction {
	return p.Action
}

// GetActionData implements Proposal
func (p *RepoProposal) GetActionData() map[string]interface{} {
	return p.ActionData
}

// GetThreshold implements Proposal
func (p *RepoProposal) GetThreshold() float64 {
	return p.Threshold
}

// GetVetoQuorum implements Proposal
func (p *RepoProposal) GetVetoQuorum() float64 {
	return p.VetoQuorum
}

// GetAccepted implements Proposal
func (p *RepoProposal) GetAccepted() float64 {
	return p.Yes
}

// GetRejected implements Proposal
func (p *RepoProposal) GetRejected() float64 {
	return p.No
}

// GetRejectedWithVeto implements Proposal
func (p *RepoProposal) GetRejectedWithVeto() float64 {
	return p.NoWithVeto
}

// RepoProposals represents an index of proposals for a repo.
// Note: we are using map[string]interface{} instead of map[string]*RepoProposal
// because we want to take advantage of msgpack map sorting which only works on the
// former.
// CONTRACT: interface{} is always *RepoProposal
type RepoProposals map[string]interface{}

// Add adds a proposal map to the give id
func (p *RepoProposals) Add(id string, rp *RepoProposal) {
	(*p)[id] = rp
}

// Has checks whether a repo with the given id exists
func (p *RepoProposals) Has(id string) bool {
	return (*p)[id] != nil
}

// Get returns the proposal corresponding to the given id
func (p *RepoProposals) Get(id string) *RepoProposal {
	switch val := (*p)[id].(type) {
	case map[string]interface{}:
		var proposal RepoProposal
		mapstructure.Decode(val, &proposal)
		p.Add(id, &proposal)
		return &proposal
	case *RepoProposal:
		return val
	}
	return nil
}
