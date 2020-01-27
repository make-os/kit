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
	ProposalTallyMethodOneVote = iota + 1
	ProposalTallyMethodTokenWeighted
)

// ProposalAction represents proposal action types
type ProposalAction int

// Proposal actions
const (
	ProposalActionAddOwner ProposalAction = iota + 1
)

// Proposal describes a repository proposal
type Proposal interface {
	GetCreator() string
	GetProposeeType() ProposeeType
	GetProposeeAge() uint64
	GetEndAt() uint64
	GetQuorum() float64
	GetTallyMethod() ProposalTallyMethod
	GetAction() ProposalAction
	GetActionData() map[string]interface{}
	GetThreshold() float64
	GetVetoQuorum() float64
	GetAccepted() float64
	GetRejected() float64
	IsFinalized() bool
	SetFinalized(v bool)
	IsSelfAccepted() bool
	SetSelfAccepted(v bool)
}

// RepoProposal represents a repository proposal
type RepoProposal struct {
	Action       ProposalAction         `json:"action" mapstructure:"action" msgpack:"action"`                         // The action type.
	ActionData   map[string]interface{} `json:"actionData" mapstructure:"actionData" msgpack:"actionData"`             // The data to use to perform the action.
	Creator      string                 `json:"creator" mapstructure:"creator" msgpack:"creator"`                      // The creator is the address of the proposal creator.
	Proposee     ProposeeType           `json:"proposee" mapstructure:"proposee" msgpack:"proposee"`                   // The set of participants allowed to vote on the proposal.
	ProposeeAge  uint64                 `json:"proposeeBefore" mapstructure:"proposeeBefore" msgpack:"proposeeBefore"` // Used to allow proposee that are active before a specific height.
	EndAt        uint64                 `json:"endAt" mapstructure:"endAt" msgpack:"endAt"`                            // Used to close the proposal after the given height.
	TallyMethod  ProposalTallyMethod    `json:"tallyMethod" mapstructure:"tallyMethod" msgpack:"tallyMethod"`          // Tally method describes how the votes are counted.
	Quorum       float64                `json:"quorum" mapstructure:"quorum" msgpack:"quorum"`                         // Quorum describes what the majority is.
	Threshold    float64                `json:"threshold" mapstructure:"threshold" msgpack:"threshold"`                // Thresholds describes the minimum "Yes" quorum required to consider a proposal valid.
	VetoQuorum   float64                `json:"vetoQuorum" mapstructure:"vetoQuorum" msgpack:"vetoQuorum"`             // Veto quorum describes the quorum among veto-powered proposees required to stop a proposal.
	Accepted     float64                `json:"accepted" mapstructure:"accepted" msgpack:"accepted"`                   // Count of "Yes" votes
	Rejected     float64                `json:"rejected" mapstructure:"rejected" msgpack:"rejected"`                   // Count of "No" votes
	Finalized    bool                   `json:"finalized" mapstructure:"finalized" msgpack:"finalized"`
	SelfAccepted bool                   `json:"selfAccepted" mapstructure:"selfAccepted" msgpack:"selfAccepted"` // Indicates that the proposal was immediately accepted by a sole repo owner who is also the proposal creator
}

// GetCreator implements Proposal
func (p *RepoProposal) GetCreator() string {
	return p.Creator
}

// IsFinalized implements Proposal
func (p *RepoProposal) IsFinalized() bool {
	return p.Finalized
}

// SetFinalized implements Proposal
func (p *RepoProposal) SetFinalized(v bool) {
	p.Finalized = v
}

// IsSelfAccepted implements Proposal
func (p *RepoProposal) IsSelfAccepted() bool {
	return p.SelfAccepted
}

// SetSelfAccepted implements Proposal
func (p *RepoProposal) SetSelfAccepted(v bool) {
	p.SelfAccepted = v
}

// GetProposeeType implements Proposal
func (p *RepoProposal) GetProposeeType() ProposeeType {
	return p.Proposee
}

// GetProposeeAge implements Proposal
func (p *RepoProposal) GetProposeeAge() uint64 {
	return p.ProposeeAge
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
	return p.Accepted
}

// GetRejected implements Proposal
func (p *RepoProposal) GetRejected() float64 {
	return p.Rejected
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

// Get returns the proposal corresponding to the given id
func (p *RepoProposals) Get(id string) *RepoProposal {
	switch val := (*p)[id].(type) {
	case map[string]interface{}:
		var proposal RepoProposal
		mapstructure.Decode(val, &proposal)
		return &proposal
	case *RepoProposal:
		return val
	}
	return nil
}
