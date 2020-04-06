package state

import (
	"github.com/shopspring/decimal"
	"github.com/vmihailenco/msgpack"
	"gitlab.com/makeos/mosdef/util"
)

// ProposeeType represents a type of repo proposal proposee
type ProposeeType int

// Proposee types
const (
	ProposeeOwner                       ProposeeType = iota + 1 // Proposals will allow only owners to vote
	ProposeeNetStakeholders                                     // Proposals will allow network stakeholders to vote
	ProposeeNetStakeholdersAndVetoOwner                         // Proposals will allow stakeholders and veto owners to vote
)

// ProposalFeeRefundType describes the type of refund scheme supported
type ProposalFeeRefundType int

// Proposal fee refund types
const (
	ProposalFeeRefundNo ProposalFeeRefundType = iota
	ProposalFeeRefundOnAccept
	ProposalFeeRefundOnAcceptReject
	ProposalFeeRefundOnAcceptAllReject
	ProposalFeeRefundOnBelowThreshold
	ProposalFeeRefundOnBelowThresholdAccept
	ProposalFeeRefundOnBelowThresholdAcceptReject
	ProposalFeeRefundOnBelowThresholdAcceptAllReject
)

// ProposalTallyMethod represents a type for repo proposal counting method
type ProposalTallyMethod int

// ProposalTallyMethod types
const (
	ProposalTallyMethodIdentity ProposalTallyMethod = iota + 1
	ProposalTallyMethodCoinWeighted
	ProposalTallyMethodNetStake
	ProposalTallyMethodNetStakeOfProposer
	ProposalTallyMethodNetStakeOfDelegators
)

// ProposalFees contains address and fees paid by proposal creators
type ProposalFees map[string]string

// Register adds an entry
func (pf ProposalFees) Add(address string, fee string) {
	pf[address] = fee
}

// Has checks if an address exists
func (pf ProposalFees) Has(address string) bool {
	_, ok := pf[address]
	return ok
}

// Get return the fee associated with an address
func (pf ProposalFees) Get(address string) util.String {
	if fee, ok := pf[address]; ok {
		return util.String(fee)
	}
	return ""
}

// Total returns the total fees
func (pf ProposalFees) Total() decimal.Decimal {
	sum := decimal.Zero
	for _, fee := range pf {
		feeD, _ := decimal.NewFromString(fee)
		sum = sum.Add(feeD)
	}
	return sum
}

// ProposalAction represents proposal action types
type ProposalAction int

// Proposal actions
const (
	ProposalActionAddOwner ProposalAction = iota + 1
	ProposalActionRepoUpdate
	ProposalActionMergeRequest
	ProposalActionRegisterPushKeyIDs
)

// Proposal vote choices
const (
	ProposalVoteYes        = 1
	ProposalVoteNo         = 0
	ProposalVoteNoWithVeto = -1
	ProposalVoteAbstain    = -2
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
	GetActionData() map[string][]byte
	GetThreshold() float64
	GetVetoQuorum() float64
	GetVetoOwnersQuorum() float64
	GetAccepted() float64
	GetRejected() float64
	GetRejectedWithVeto() float64
	GetRejectedWithVetoByOwners() float64
	GetFees() ProposalFees
	GetRefundType() ProposalFeeRefundType
	IsFinalized() bool
	SetOutcome(v ProposalOutcome)
	IncrAccept()
	IsFeeDepositEnabled() bool
	IsDepositedFeeOK() bool
	IsDepositPeriod(curChainHeight uint64) bool
}

// ProposalOutcome describes a proposal outcome
type ProposalOutcome int

// Proposal outcomes
const (
	ProposalOutcomeAccepted ProposalOutcome = iota + 1
	ProposalOutcomeRejected
	ProposalOutcomeRejectedWithVeto
	ProposalOutcomeRejectedWithVetoByOwners
	ProposalOutcomeQuorumNotMet
	ProposalOutcomeThresholdNotMet
	ProposalOutcomeBelowThreshold
	ProposalOutcomeInsufficientDeposit
)

// RepoProposal represents a repository proposal
type RepoProposal struct {
	util.SerializerHelper `json:"-" msgpack:"-"`
	ID                    string                `json:"-" mapstructure:"-" msgpack:"-"`
	Action                ProposalAction        `json:"action" mapstructure:"action" msgpack:"action"`                                              // The action type.
	ActionData            map[string][]byte     `json:"actionData" mapstructure:"actionData" msgpack:"actionData"`                                  // The data to use to perform the action.
	Creator               string                `json:"creator" mapstructure:"creator" msgpack:"creator"`                                           // The creator is the address of the proposal creator.
	Height                uint64                `json:"height" mapstructure:"height" msgpack:"height"`                                              // The height of the block the proposal was added
	Config                *RepoConfigGovernance `json:"config" mapstructure:"config" msgpack:"-"`                                                   // The repo config to used to evaluate the proposal
	EndAt                 uint64                `json:"endAt" mapstructure:"endAt" msgpack:"endAt"`                                                 // Used to close the proposal after the given height.
	FeeDepositEndAt       uint64                `json:"feeDepEndAt" mapstructure:"feeDepEndAt" msgpack:"feeDepEndAt"`                               // Used to close the proposal after the given height.
	ProposeeMaxJoinHeight uint64                `json:"proposeeMaxJoinHeight" mapstructure:"proposeeMaxJoinHeight" msgpack:"proposeeMaxJoinHeight"` // Used to allow proposee that are active before a specific height.
	Yes                   float64               `json:"yes" mapstructure:"yes" msgpack:"yes"`                                                       // Count of "Yes" votes
	No                    float64               `json:"no" mapstructure:"no" msgpack:"no"`                                                          // Count of "No" votes
	NoWithVeto            float64               `json:"noWithVeto" mapstructure:"noWithVeto" msgpack:"noWithVeto"`                                  // Count of "No" votes from owners/stakeholders veto power
	NoWithVetoByOwners    float64               `json:"noWithVetoByOwners" mapstructure:"noWithVetoByOwners" msgpack:"noWithVetoByOwners"`          // Count of "No" votes specifically from owners veto power
	Abstain               float64               `json:"abstain" mapstructure:"abstain" msgpack:"abstain"`                                           // Count of explicit "abstain" votes
	Fees                  ProposalFees          `json:"fees" mapstructure:"fees" msgpack:"fees"`                                                    // Count of explicit "abstain" votes
	Outcome               ProposalOutcome       `json:"outcome" mapstructure:"outcome" msgpack:"outcome"`                                           // The outcome of the proposal vote.
}

// ProposalActionData represents action data of a proposal
type ProposalActionData map[string]interface{}

// Get returns the data corresponding to the given action name
func (d *ProposalActionData) Get(actionName string) map[string]interface{} {
	data := (*d)[actionName]
	if data == nil {
		return nil
	}
	return data.(map[string]interface{})
}

// BareRepoProposal returns RepoProposal object with empty values
func BareRepoProposal() *RepoProposal {
	return &RepoProposal{
		Config:     BareRepoConfig().Governance,
		ActionData: make(map[string][]byte),
		Fees:       make(map[string]string),
	}
}

// IsDepositPeriod checks whether the proposal is in the deposit period
func (p *RepoProposal) IsDepositPeriod(chainHeight uint64) bool {
	return p.FeeDepositEndAt != 0 && p.FeeDepositEndAt >= chainHeight
}

// IsFeeDepositEnabled checks whether fee deposit is enabled on the proposal
func (p *RepoProposal) IsFeeDepositEnabled() bool {
	return p.FeeDepositEndAt != 0
}

// IsDepositedFeeOK checks whether the fees deposited to the proposal
// meets the minimum required deposit
func (p *RepoProposal) IsDepositedFeeOK() bool {
	propFee := decimal.NewFromFloat(p.Config.ProposalFee)
	return p.Fees.Total().GreaterThanOrEqual(propFee)
}

// GetCreator implements Proposal
func (p *RepoProposal) GetCreator() string {
	return p.Creator
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (p *RepoProposal) EncodeMsgpack(enc *msgpack.Encoder) error {
	return p.EncodeMulti(enc, p.ID, p.Action, p.ActionData, p.Creator, p.Height, p.Config,
		p.EndAt, p.FeeDepositEndAt, p.ProposeeMaxJoinHeight, p.Yes, p.No,
		p.NoWithVeto, p.NoWithVetoByOwners, p.Abstain, p.Fees, p.Outcome)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (p *RepoProposal) DecodeMsgpack(dec *msgpack.Decoder) error {
	return p.DecodeMulti(dec, &p.ID, &p.Action, &p.ActionData, &p.Creator, &p.Height, &p.Config,
		&p.EndAt, &p.FeeDepositEndAt, &p.ProposeeMaxJoinHeight, &p.Yes, &p.No,
		&p.NoWithVeto, &p.NoWithVetoByOwners, &p.Abstain, &p.Fees, &p.Outcome)
}

// IsFinalized implements Proposal
func (p *RepoProposal) IsFinalized() bool {
	return p.Outcome > 0
}

// SetOutcome implements Proposal
func (p *RepoProposal) SetOutcome(v ProposalOutcome) {
	p.Outcome = v
}

// IncrAccept increments 'Yes' by 1
func (p *RepoProposal) IncrAccept() {
	p.Yes++
}

// GetProposeeType implements Proposal
func (p *RepoProposal) GetProposeeType() ProposeeType {
	return p.Config.ProposalProposee
}

// GetProposeeMaxJoinHeight implements Proposal
func (p *RepoProposal) GetProposeeMaxJoinHeight() uint64 {
	return p.ProposeeMaxJoinHeight
}

// GetEndAt implements Proposal
func (p *RepoProposal) GetEndAt() uint64 {
	return p.EndAt
}

// GetFees implements Proposal
func (p *RepoProposal) GetFees() ProposalFees {
	return p.Fees
}

// GetRefundType implements Proposal
func (p *RepoProposal) GetRefundType() ProposalFeeRefundType {
	return p.Config.ProposalFeeRefundType
}

// GetQuorum implements Proposal
func (p *RepoProposal) GetQuorum() float64 {
	return p.Config.ProposalQuorum
}

// GetTallyMethod implements Proposal
func (p *RepoProposal) GetTallyMethod() ProposalTallyMethod {
	return p.Config.ProposalTallyMethod
}

// GetAction implements Proposal
func (p *RepoProposal) GetAction() ProposalAction {
	return p.Action
}

// GetActionData implements Proposal
func (p *RepoProposal) GetActionData() map[string][]byte {
	return p.ActionData
}

// GetThreshold implements Proposal
func (p *RepoProposal) GetThreshold() float64 {
	return p.Config.ProposalThreshold
}

// GetVetoQuorum implements Proposal
func (p *RepoProposal) GetVetoQuorum() float64 {
	return p.Config.ProposalVetoQuorum
}

// GetVetoOwnersQuorum implements Proposal
func (p *RepoProposal) GetVetoOwnersQuorum() float64 {
	return p.Config.ProposalVetoOwnersQuorum
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

// GetRejectedWithVetoByOwners implements Proposal
func (p *RepoProposal) GetRejectedWithVetoByOwners() float64 {
	return p.NoWithVetoByOwners
}

// RepoProposals represents an index of proposals for a repo.
type RepoProposals map[string]*RepoProposal

// Register adds a proposal map to the give id
func (p *RepoProposals) Add(id string, rp *RepoProposal) {
	(*p)[id] = rp
}

// Has checks whether a repo with the given id exists
func (p *RepoProposals) Has(id string) bool {
	return (*p)[id] != nil
}

// Get returns the proposal corresponding to the given id
func (p *RepoProposals) Get(id string) *RepoProposal {
	return (*p)[id]
}

// ForEach iterates through items, passing each to the callback function
func (p *RepoProposals) ForEach(itr func(prop *RepoProposal, id string) error) error {
	for id, prop := range *p {
		if err := itr(prop, id); err != nil {
			return err
		}
	}
	return nil
}
