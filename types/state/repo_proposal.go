package state

import (
	"github.com/AlekSi/pointer"
	"github.com/make-os/kit/types"
	"github.com/make-os/kit/util"
	"github.com/shopspring/decimal"
	"github.com/spf13/cast"
	"github.com/thoas/go-funk"
	"github.com/vmihailenco/msgpack"
)

// VoterType represents a type of repository voter type
type VoterType int

func (p VoterType) Ptr() *int {
	v := int(p)
	return &v
}

const (
	VoterOwner                  VoterType = iota // Only owners can vote
	VoterNetStakers                              // Only network stakeholders can vote.
	VoterNetStakersAndVetoOwner                  // Only network stakeholders and veto owners can vote.
)

// IsValidVoterType checks if v is a valid VoterType
func IsValidVoterType(v *int) bool {
	return funk.Contains([]VoterType{
		VoterOwner,
		VoterNetStakers,
		VoterNetStakersAndVetoOwner,
	}, VoterType(pointer.GetInt(v)))
}

// ProposalCreatorType describes types of proposal creators
type ProposalCreatorType int

func (p ProposalCreatorType) Ptr() *int {
	v := int(p)
	return &v
}

const (
	ProposalCreatorAny ProposalCreatorType = iota
	ProposalCreatorOwner
)

// IsValidProposalCreatorType checks if v is a valid ProposalCreatorType
func IsValidProposalCreatorType(v *int) bool {
	return funk.Contains([]ProposalCreatorType{
		ProposalCreatorAny,
		ProposalCreatorOwner,
	}, ProposalCreatorType(pointer.GetInt(v)))
}

// PropFeeRefundType describes the typeof refund scheme supported
type PropFeeRefundType int

func (p PropFeeRefundType) Ptr() *int {
	v := int(p)
	return &v
}

const (
	ProposalFeeRefundNo PropFeeRefundType = iota
	ProposalFeeRefundOnAccept
	ProposalFeeRefundOnAcceptReject
	ProposalFeeRefundOnAcceptAllReject
	ProposalFeeRefundOnBelowThreshold
	ProposalFeeRefundOnBelowThresholdAccept
	ProposalFeeRefundOnBelowThresholdAcceptReject
	ProposalFeeRefundOnBelowThresholdAcceptAllReject
)

// IsValidPropFeeRefundTypeType checks if v is a valid PropFeeRefundTypeType
func IsValidPropFeeRefundTypeType(v *int) bool {
	return funk.Contains([]PropFeeRefundType{
		ProposalFeeRefundNo,
		ProposalFeeRefundOnAccept,
		ProposalFeeRefundOnAcceptReject,
		ProposalFeeRefundOnAcceptAllReject,
		ProposalFeeRefundOnBelowThreshold,
		ProposalFeeRefundOnBelowThresholdAccept,
		ProposalFeeRefundOnBelowThresholdAcceptReject,
		ProposalFeeRefundOnBelowThresholdAcceptAllReject,
	}, PropFeeRefundType(pointer.GetInt(v)))
}

// ProposalTallyMethod represents a type for repo proposal counting method
type ProposalTallyMethod int

func (p ProposalTallyMethod) Ptr() *int {
	v := int(p)
	return &v
}

const (
	ProposalTallyMethodIdentity ProposalTallyMethod = iota
	ProposalTallyMethodCoinWeighted
	ProposalTallyMethodNetStake
	ProposalTallyMethodNetStakeNonDelegated
	ProposalTallyMethodNetStakeOfDelegators
)

// IsValidProposalTallyMethod checks if v is a valid ProposalTallyMethod
func IsValidProposalTallyMethod(v *int) bool {
	return funk.Contains([]ProposalTallyMethod{
		ProposalTallyMethodIdentity,
		ProposalTallyMethodCoinWeighted,
		ProposalTallyMethodNetStake,
		ProposalTallyMethodNetStakeNonDelegated,
		ProposalTallyMethodNetStakeOfDelegators,
	}, ProposalTallyMethod(pointer.GetInt(v)))
}

// ProposalFees contains address and fees paid by proposal creators
type ProposalFees map[string]string

// Add adds a new fee entry
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

// Proposal vote choices
const (
	ProposalVoteNo         = 0
	ProposalVoteYes        = 1
	ProposalVoteNoWithVeto = 2
	ProposalVoteAbstain    = 3
)

// Proposal describes a repository proposal
type Proposal interface {
	GetCreator() string
	GetVoterType() VoterType
	GetPowerAge() uint64
	GetEndAt() uint64
	GetQuorum() float64
	GetTallyMethod() ProposalTallyMethod
	GetAction() types.TxCode
	GetActionData() map[string]util.Bytes
	GetThreshold() float64
	GetVetoQuorum() float64
	GetVetoOwnersQuorum() float64
	GetAccepted() float64
	GetRejected() float64
	GetRejectedWithVeto() float64
	GetRejectedWithVetoByOwners() float64
	GetFees() ProposalFees
	GetRefundType() PropFeeRefundType
	IsFinalized() bool
	SetOutcome(v ProposalOutcome)
	IncrAccept()
	IsFeeDepositEnabled() bool
	IsDepositedFeeOK() bool
	IsDepositPeriod(curChainHeight uint64) bool
}

// ProposalOutcome describes a proposal outcome
type ProposalOutcome int

func (p ProposalOutcome) Ptr() *int {
	v := int(p)
	return &v
}

// Proposal outcomes
const (
	ProposalOutcomeAccepted ProposalOutcome = iota + 1
	ProposalOutcomeRejected
	ProposalOutcomeRejectedWithVeto
	ProposalOutcomeRejectedWithVetoByOwners
	ProposalOutcomeQuorumNotMet
	ProposalOutcomeBelowThreshold
	ProposalOutcomeInsufficientDeposit
)

// RepoProposal represents a repository proposal
type RepoProposal struct {
	util.CodecUtil     `json:"-" msgpack:"-"`
	ID                 string                `json:"-" mapstructure:"-" msgpack:"-"`
	Action             types.TxCode          `json:"action" mapstructure:"action" msgpack:"action"`                                     // The action type.
	ActionData         map[string]util.Bytes `json:"actionData" mapstructure:"actionData" msgpack:"actionData"`                         // The data to use to perform the action.
	Creator            string                `json:"creator" mapstructure:"creator" msgpack:"creator"`                                  // The creator is the address of the proposal creator.
	Height             util.UInt64           `json:"height" mapstructure:"height" msgpack:"height"`                                     // The height of the block the proposal was added
	Config             *RepoConfigGovernance `json:"config" mapstructure:"config" msgpack:"-"`                                          // The repo config to used to evaluate the proposal
	EndAt              util.UInt64           `json:"endAt" mapstructure:"endAt" msgpack:"endAt"`                                        // Used to close the proposal after the given height.
	FeeDepositEndAt    util.UInt64           `json:"feeDepEndAt" mapstructure:"feeDepEndAt" msgpack:"feeDepEndAt"`                      // Used to close the proposal after the given height.
	PowerAge           util.UInt64           `json:"powerAge" mapstructure:"powerAge" msgpack:"powerAge"`                               // Used to set the age of a voter's power source.
	Yes                float64               `json:"yes" mapstructure:"yes" msgpack:"yes"`                                              // Count of "Yes" votes
	No                 float64               `json:"no" mapstructure:"no" msgpack:"no"`                                                 // Count of "No" votes
	NoWithVeto         float64               `json:"noWithVeto" mapstructure:"noWithVeto" msgpack:"noWithVeto"`                         // Count of "No" votes from owners/stakeholders veto power
	NoWithVetoByOwners float64               `json:"noWithVetoByOwners" mapstructure:"noWithVetoByOwners" msgpack:"noWithVetoByOwners"` // Count of "No" votes specifically from owners veto power
	Abstain            float64               `json:"abstain" mapstructure:"abstain" msgpack:"abstain"`                                  // Count of explicit "abstain" votes
	Fees               ProposalFees          `json:"fees" mapstructure:"fees" msgpack:"fees"`                                           // Count of explicit "abstain" votes
	Outcome            ProposalOutcome       `json:"outcome" mapstructure:"outcome" msgpack:"outcome"`                                  // The outcome of the proposal vote.
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
		Config:     BareRepoConfig().Gov,
		ActionData: make(map[string]util.Bytes),
		Fees:       make(map[string]string),
	}
}

// IsDepositPeriod checks whether the proposal is in the deposit period
func (p *RepoProposal) IsDepositPeriod(chainHeight uint64) bool {
	return p.FeeDepositEndAt != 0 && p.FeeDepositEndAt >= util.UInt64(chainHeight)
}

// IsFeeDepositEnabled checks whether fee deposit is enabled on the proposal
func (p *RepoProposal) IsFeeDepositEnabled() bool {
	return p.FeeDepositEndAt != 0
}

// IsDepositedFeeOK checks whether the fees deposited to the proposal
// meets the minimum required deposit
func (p *RepoProposal) IsDepositedFeeOK() bool {
	propFee := decimal.NewFromFloat(cast.ToFloat64(*p.Config.PropFee))
	return p.Fees.Total().GreaterThanOrEqual(propFee)
}

// GetCreator implements Proposal
func (p *RepoProposal) GetCreator() string {
	return p.Creator
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (p *RepoProposal) EncodeMsgpack(enc *msgpack.Encoder) error {
	return p.EncodeMulti(enc,
		p.ID,
		p.Action,
		p.ActionData,
		p.Creator,
		p.Height,
		p.Config,
		p.EndAt,
		p.FeeDepositEndAt,
		p.PowerAge,
		p.Yes,
		p.No,
		p.NoWithVeto,
		p.NoWithVetoByOwners,
		p.Abstain,
		p.Fees,
		p.Outcome)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (p *RepoProposal) DecodeMsgpack(dec *msgpack.Decoder) error {
	return p.DecodeMulti(dec,
		&p.ID,
		&p.Action,
		&p.ActionData,
		&p.Creator,
		&p.Height,
		&p.Config,
		&p.EndAt,
		&p.FeeDepositEndAt,
		&p.PowerAge,
		&p.Yes,
		&p.No,
		&p.NoWithVeto,
		&p.NoWithVetoByOwners,
		&p.Abstain,
		&p.Fees,
		&p.Outcome)
}

// IsFinalized implements Proposal
func (p *RepoProposal) IsFinalized() bool {
	return p.Outcome > 0
}

// IsAccepted implements Proposal
func (p *RepoProposal) IsAccepted() bool {
	return p.Outcome == ProposalOutcomeAccepted
}

// SetOutcome implements Proposal
func (p *RepoProposal) SetOutcome(v ProposalOutcome) {
	p.Outcome = v
}

// IncrAccept increments 'Yes' by 1
func (p *RepoProposal) IncrAccept() {
	p.Yes++
}

// GetVoterType implements Proposal
func (p *RepoProposal) GetVoterType() VoterType {
	return VoterType(pointer.GetInt(p.Config.Voter))
}

// GetPowerAge implements Proposal
func (p *RepoProposal) GetPowerAge() uint64 {
	return uint64(p.PowerAge)
}

// GetEndAt implements Proposal
func (p *RepoProposal) GetEndAt() uint64 {
	return p.EndAt.UInt64()
}

// GetFees implements Proposal
func (p *RepoProposal) GetFees() ProposalFees {
	return p.Fees
}

// GetRefundType implements Proposal
func (p *RepoProposal) GetRefundType() PropFeeRefundType {
	return PropFeeRefundType(pointer.GetInt(p.Config.PropFeeRefundType))
}

// GetQuorum implements Proposal
func (p *RepoProposal) GetQuorum() float64 {
	return cast.ToFloat64(pointer.GetString(p.Config.PropQuorum))
}

// GetTallyMethod implements Proposal
func (p *RepoProposal) GetTallyMethod() ProposalTallyMethod {
	return ProposalTallyMethod(*p.Config.PropTallyMethod)
}

// GetAction implements Proposal
func (p *RepoProposal) GetAction() types.TxCode {
	return p.Action
}

// GetActionData implements Proposal
func (p *RepoProposal) GetActionData() map[string]util.Bytes {
	return p.ActionData
}

// GetThreshold implements Proposal
func (p *RepoProposal) GetThreshold() float64 {
	return cast.ToFloat64(pointer.GetString(p.Config.PropThreshold))
}

// GetVetoQuorum implements Proposal
func (p *RepoProposal) GetVetoQuorum() float64 {
	return cast.ToFloat64(pointer.GetString(p.Config.PropVetoQuorum))
}

// GetVetoOwnersQuorum implements Proposal
func (p *RepoProposal) GetVetoOwnersQuorum() float64 {
	return cast.ToFloat64(pointer.GetString(p.Config.PropVetoOwnersQuorum))
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
