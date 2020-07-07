package state

import (
	"github.com/imdario/mergo"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/cast"
	"github.com/stretchr/objx"
	"github.com/vmihailenco/msgpack"
	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/params"
	"gitlab.com/makeos/mosdef/util"
)

type FeeMode int

const (
	FeeModePusherPays = iota
	FeeModeRepoPays
	FeeModeRepoPaysCapped
)

// BareReference returns an empty reference object
func BareReference() *Reference {
	return &Reference{
		Data: &ReferenceData{},
	}
}

// ReferenceData contain data specific to a reference
type ReferenceData struct {

	// Labels are keywords that describe the reference
	Labels []string `json:"labels" mapstructure:"labels" msgpack:"labels,omitempty"`

	// Assignees contains pushers assigned to the reference
	Assignees []string `json:"assignees" mapstructure:"assignees" msgpack:"assignees,omitempty"`

	// Closed indicates that the reference is closed
	Closed bool `json:"closed" mapstructure:"closed" msgpack:"closed,omitempty"`
}

// IsNil checks whether the object's fields are have zero values
func (i *ReferenceData) IsNil() bool {
	return len(i.Assignees) == 0 && len(i.Labels) == 0 && i.Closed == false
}

// Reference represents a git reference
type Reference struct {
	util.CodecUtil `json:"-" mapstructure:"-" msgpack:"-"`

	// Creator is the raw push key ID of the reference creator
	Creator crypto.PushKey `json:"creator" mapstructure:"creator" msgpack:"creator,omitempty"`

	// Nonce is the current count of commits on the reference.
	// It is used to enforce order of operation to the reference.
	Nonce util.UInt64 `json:"nonce" mapstructure:"nonce" msgpack:"nonce,omitempty"`

	// ReferenceData contains extra data
	Data *ReferenceData `json:"data" mapstructure:"data" msgpack:"data,omitempty"`

	// Hash is the current hash of the reference
	Hash util.Bytes `json:"hash" mapstructure:"hash" msgpack:"hash,omitempty"`
}

// IsNil checks whether the reference fields are all empty
func (r *Reference) IsNil() bool {
	return len(r.Creator) == 0 && len(r.Hash) == 0 && r.Nonce == 0 && r.Data.IsNil()
}

func (r *Reference) EncodeMsgpack(enc *msgpack.Encoder) error {
	return r.EncodeMulti(enc, r.Creator, r.Nonce, r.Hash, r.Data)
}

func (r *Reference) DecodeMsgpack(dec *msgpack.Decoder) error {
	return r.DecodeMulti(dec, &r.Creator, &r.Nonce, &r.Hash, &r.Data)
}

// References represents a collection of references
type References map[string]*Reference

// Get a reference by name, returns empty reference if not found.
func (r *References) Get(name string) *Reference {
	ref := (*r)[name]
	if ref == nil {
		return BareReference()
	}
	return ref
}

// Has checks whether a reference exist
func (r *References) Has(name string) bool {
	return (*r)[name] != nil
}

// RepoOwner describes an owner of a repository
type RepoOwner struct {
	Creator  bool        `json:"creator" mapstructure:"creator" msgpack:"creator,omitempty"`
	JoinedAt util.UInt64 `json:"joinedAt" mapstructure:"joinedAt" msgpack:"joinedAt,omitempty"`
	Veto     bool        `json:"veto" mapstructure:"veto" msgpack:"veto,omitempty"`
}

// RepoOwners represents an index of owners of a repository.
type RepoOwners map[string]*RepoOwner

// Has returns true of address exist
func (r RepoOwners) Has(address string) bool {
	_, has := r[address]
	return has
}

// Get return a repo owner associated with the given address
func (r RepoOwners) Get(address string) *RepoOwner {
	return r[address]
}

// ForEach iterates through the collection passing each item to the iter callback
func (r RepoOwners) ForEach(iter func(o *RepoOwner, addr string)) {
	for key := range r {
		iter(r.Get(key), key)
	}
}

// RepoConfigGovernance contains governance settings for a repository
type RepoConfigGovernance struct {
	Voter                    VoterType             `json:"propVoter" mapstructure:"propVoter,omitempty" msgpack:"propVoter,omitempty"`
	ProposalCreator          ProposalCreatorType   `json:"propCreator" mapstructure:"propCreator,omitempty" msgpack:"propCreator,omitempty"`
	RequireVoterJoinHeight   bool                  `json:"requireVoterJoinHeight" mapstructure:"requireVoterJoinHeight" msgpack:"requireVoterJoinHeight,omitempty"`
	ProposalDuration         util.UInt64           `json:"propDur" mapstructure:"propDur,omitempty" msgpack:"propDur,omitempty"`
	ProposalFeeDepositDur    util.UInt64           `json:"propFeeDepDur" mapstructure:"propFeeDepDur,omitempty" msgpack:"propFeeDepDur,omitempty"`
	ProposalTallyMethod      ProposalTallyMethod   `json:"propTallyMethod" mapstructure:"propTallyMethod,omitempty" msgpack:"propTallyMethod,omitempty"`
	ProposalQuorum           float64               `json:"propQuorum" mapstructure:"propQuorum,omitempty" msgpack:"propQuorum,omitempty"`
	ProposalThreshold        float64               `json:"propThreshold" mapstructure:"propThreshold,omitempty" msgpack:"propThreshold,omitempty"`
	ProposalVetoQuorum       float64               `json:"propVetoQuorum" mapstructure:"propVetoQuorum,omitempty" msgpack:"propVetoQuorum,omitempty"`
	ProposalVetoOwnersQuorum float64               `json:"propVetoOwnersQuorum" mapstructure:"propVetoOwnersQuorum,omitempty" msgpack:"propVetoOwnersQuorum,omitempty"`
	ProposalFee              float64               `json:"propFee" mapstructure:"propFee,omitempty" msgpack:"propFee,omitempty"`
	ProposalFeeRefundType    ProposalFeeRefundType `json:"propFeeRefundType" mapstructure:"propFeeRefundType,omitempty" msgpack:"propFeeRefundType,omitempty"`
	NoProposalFeeForMergeReq bool                  `json:"noPropFeeForMergeReq" mapstructure:"noPropFeeForMergeReq" msgpack:"noPropFeeForMergeReq,omitempty"`
}

// Policy describes a repository access policy
type Policy struct {
	Object  string `json:"obj,omitempty" mapstructure:"obj,omitempty" msgpack:"obj,omitempty"`
	Subject string `json:"sub,omitempty" mapstructure:"sub,omitempty" msgpack:"sub,omitempty"`
	Action  string `json:"act,omitempty" mapstructure:"act,omitempty" msgpack:"act,omitempty"`
}

// ContributorPolicy describes a contributors policy.
// Similar to Policy except the one has no subject field.
type ContributorPolicy struct {
	Object string `json:"obj,omitempty" mapstructure:"obj,omitempty" msgpack:"obj,omitempty"`
	Action string `json:"act,omitempty" mapstructure:"act,omitempty" msgpack:"act,omitempty"`
}

// RepoPolicies represents an index of repo Policies policies
// key is policy id
type RepoPolicies []*Policy

// RepoConfig contains repo-specific configuration settings
type RepoConfig struct {
	util.CodecUtil `json:"-" mapstructure:"-" msgpack:"-"`
	Governance     *RepoConfigGovernance `json:"governance" mapstructure:"governance,omitempty" msgpack:"governance,omitempty"`
	Policies       RepoPolicies          `json:"policies" mapstructure:"policies" msgpack:"policies,omitempty"`
}

// FromMap populates c using m.
// Expects m to only include key and values with basic go primitive types.
func (c *RepoConfig) FromMap(m map[string]interface{}) *RepoConfig {
	cfg := objx.New(m)

	// Populate Governance config
	obj := cfg.Get("governance").ObjxMap()
	c.Governance.Voter = VoterType(cast.ToInt(obj.Get("propVoter").Inter()))
	c.Governance.ProposalCreator = ProposalCreatorType(cast.ToInt(obj.Get("propCreator").Inter()))
	c.Governance.RequireVoterJoinHeight = cast.ToBool(obj.Get("requireVoterJoinHeight").Inter())
	c.Governance.ProposalDuration = util.UInt64(cast.ToUint64(obj.Get("propDur").Inter()))
	c.Governance.ProposalFeeDepositDur = util.UInt64(cast.ToUint64(obj.Get("propFeeDepDur").Inter()))
	c.Governance.ProposalTallyMethod = ProposalTallyMethod(cast.ToInt(obj.Get("propTallyMethod").Inter()))
	c.Governance.ProposalQuorum = cast.ToFloat64(obj.Get("propQuorum").Inter())
	c.Governance.ProposalThreshold = cast.ToFloat64(obj.Get("propThreshold").Inter())
	c.Governance.ProposalVetoQuorum = cast.ToFloat64(obj.Get("propVetoQuorum").Inter())
	c.Governance.ProposalVetoOwnersQuorum = cast.ToFloat64(obj.Get("propVetoOwnersQuorum").Inter())
	c.Governance.ProposalFee = cast.ToFloat64(obj.Get("propFee").Inter())
	c.Governance.ProposalFeeRefundType = ProposalFeeRefundType(cast.ToInt(obj.Get("propFeeRefundType").Inter()))
	c.Governance.NoProposalFeeForMergeReq = cast.ToBool(obj.Get("noPropFeeForMergeReq").Inter())

	// Populate Policies
	policies := cfg.Get("policies").ObjxMapSlice()
	for _, pol := range policies {
		c.Policies = append(c.Policies, &Policy{
			Object:  pol.Get("obj").String(),
			Subject: pol.Get("sub").String(),
			Action:  pol.Get("act").String(),
		})
	}

	return c
}

func (c *RepoConfig) EncodeMsgpack(enc *msgpack.Encoder) error {
	return c.EncodeMulti(enc,
		c.Governance,
		c.Policies)
}

func (c *RepoConfig) DecodeMsgpack(dec *msgpack.Decoder) error {
	return c.DecodeMulti(dec,
		&c.Governance,
		&c.Policies)
}

// Clone clones c
func (c *RepoConfig) Clone() *RepoConfig {
	var clone = BareRepoConfig()
	m := util.ToMap(c)
	_ = mapstructure.Decode(m, &clone)
	return clone
}

// MergeMap merges the specified upd into c.
// Non-empty field in upd will override non-empty field in c.
// Empty field in upd will override non-empty fields in c.
// Slice from upd will be merged into slice field in c.
func (c *RepoConfig) MergeMap(upd map[string]interface{}) error {
	var dst = c.ToBasicMap()
	if err := mergo.Map(&dst, upd,
		mergo.WithOverride,
		mergo.WithOverwriteWithEmptyValue,
		mergo.WithAppendSlice); err != nil {
		return err
	}
	return util.DecodeMap(dst, c)
}

// IsNil checks if the object's field all have zero value
func (c *RepoConfig) IsNil() bool {
	return (c.Governance == nil || *c.Governance == RepoConfigGovernance{}) && len(c.Policies) == 0
}

// ToBasicMap converts the object to a basic map with all custom types stripped.
func (c *RepoConfig) ToBasicMap() map[string]interface{} {
	return util.ToBasicMap(c)
}

// ToMap converts the object to a map
func (c *RepoConfig) ToMap() map[string]interface{} {
	return util.ToMap(c)
}

var (
	// DefaultRepoConfig is a sane default for repository configurations
	DefaultRepoConfig = MakeDefaultRepoConfig()
)

// NewDefaultRepoConfigFromMap creates a repo config composed of default values + m
func NewDefaultRepoConfigFromMap(m map[string]interface{}) *RepoConfig {
	r := BareRepoConfig()
	r.FromMap(m)
	mergo.Merge(r, DefaultRepoConfig)
	return r
}

// MakeDefaultRepoConfig returns sane defaults for repository configurations
func MakeDefaultRepoConfig() *RepoConfig {
	return &RepoConfig{
		Governance: &RepoConfigGovernance{
			Voter:                    VoterOwner,
			ProposalCreator:          ProposalCreatorAny,
			RequireVoterJoinHeight:   false,
			ProposalDuration:         util.UInt64(params.RepoProposalDur),
			ProposalTallyMethod:      ProposalTallyMethodIdentity,
			ProposalQuorum:           params.RepoProposalQuorum,
			ProposalThreshold:        params.RepoProposalThreshold,
			ProposalVetoQuorum:       params.RepoProposalVetoQuorum,
			ProposalVetoOwnersQuorum: params.RepoProposalVetoOwnersQuorum,
			ProposalFee:              params.MinProposalFee,
			ProposalFeeRefundType:    ProposalFeeRefundNo,
			ProposalFeeDepositDur:    0,
			NoProposalFeeForMergeReq: true,
		},
		Policies: []*Policy{},
	}
}

// BareRepoConfig returns empty repository configurations
func BareRepoConfig() *RepoConfig {
	return &RepoConfig{
		Governance: &RepoConfigGovernance{},
		Policies:   RepoPolicies{},
	}
}

// BaseContributor represents the basic information of a contributor
type BaseContributor struct {
	FeeCap   util.String          `json:"feeCap" mapstructure:"feeCap" msgpack:"feeCap"`
	FeeUsed  util.String          `json:"feeUsed" mapstructure:"feeUsed" msgpack:"feeUsed"`
	Policies []*ContributorPolicy `json:"policies" mapstructure:"policies" msgpack:"policies"`
}

// BaseContributors is a collection of repo contributors
type BaseContributors map[string]*BaseContributor

// Has checks whether a gpg id exists
func (rc *BaseContributors) Has(pushKeyID string) bool {
	_, ok := (*rc)[pushKeyID]
	return ok
}

// RepoContributor represents a repository contributor
type RepoContributor struct {
	FeeMode  FeeMode              `json:"feeMode" mapstructure:"feeMode" msgpack:"feeMode"`
	FeeCap   util.String          `json:"feeCap" mapstructure:"feeCap" msgpack:"feeCap"`
	FeeUsed  util.String          `json:"feeUsed" mapstructure:"feeUsed" msgpack:"feeUsed"`
	Policies []*ContributorPolicy `json:"policies" mapstructure:"policies" msgpack:"policies"`
}

// RepoContributors is a collection of repo contributors
type RepoContributors map[string]*RepoContributor

// Has checks whether a push key id exists
func (rc *RepoContributors) Has(pushKeyID string) bool {
	_, ok := (*rc)[pushKeyID]
	return ok
}

// BareRepository returns an empty repository object
func BareRepository() *Repository {
	return &Repository{
		Balance:      "0",
		References:   map[string]*Reference{},
		Owners:       map[string]*RepoOwner{},
		Proposals:    map[string]*RepoProposal{},
		Config:       BareRepoConfig(),
		Contributors: map[string]*RepoContributor{},
	}
}

// Repository represents a git repository.
type Repository struct {
	util.CodecUtil `json:"-" msgpack:"-" mapstructure:"-"`
	Balance        util.String      `json:"balance" msgpack:"balance" mapstructure:"balance"`
	References     References       `json:"references" msgpack:"references" mapstructure:"references"`
	Owners         RepoOwners       `json:"owners" msgpack:"owners" mapstructure:"owners"`
	Proposals      RepoProposals    `json:"proposals" msgpack:"proposals" mapstructure:"proposals"`
	Contributors   RepoContributors `json:"contributors" msgpack:"contributors" mapstructure:"contributors"`
	Config         *RepoConfig      `json:"config" msgpack:"config" mapstructure:"config"`
}

// GetBalance implements types.BalanceAccount
func (r *Repository) GetBalance() util.String {
	return r.Balance
}

// SetBalance implements types.BalanceAccount
func (r *Repository) SetBalance(bal string) {
	r.Balance = util.String(bal)
}

// Clean implements types.BalanceAccount
func (r *Repository) Clean(chainHeight uint64) {}

// AddOwner adds an owner
func (r *Repository) AddOwner(ownerAddress string, owner *RepoOwner) {
	r.Owners[ownerAddress] = owner
}

// IsNil returns true if the repo fields are set to their nil value
func (r *Repository) IsNil() bool {
	return r.Balance.Empty() || r.Balance.Equal("0") &&
		len(r.References) == 0 &&
		len(r.Owners) == 0 &&
		len(r.Proposals) == 0 &&
		r.Config.IsNil()
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (r *Repository) EncodeMsgpack(enc *msgpack.Encoder) error {
	return r.EncodeMulti(enc,
		r.Balance,
		r.Owners,
		r.References,
		r.Proposals,
		r.Config,
		r.Contributors)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (r *Repository) DecodeMsgpack(dec *msgpack.Decoder) error {
	err := r.DecodeMulti(dec,
		&r.Balance,
		&r.Owners,
		&r.References,
		&r.Proposals,
		&r.Config,
		&r.Contributors)
	return err
}

// Bytes return the bytes equivalent of the account
func (r *Repository) Bytes() []byte {
	return util.ToBytes(r)
}

// NewRepositoryFromBytes decodes bz to Repository
func NewRepositoryFromBytes(bz []byte) (*Repository, error) {
	var repo = BareRepository()
	if err := util.ToObject(bz, repo); err != nil {
		return nil, err
	}
	return repo, nil
}
