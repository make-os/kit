package state

import (
	"encoding/json"

	"github.com/AlekSi/pointer"
	"github.com/make-os/kit/crypto/ed25519"
	"github.com/make-os/kit/params"
	"github.com/make-os/kit/util"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/cast"
	"github.com/vmihailenco/msgpack"
)

type FeeMode int

const (
	FeeModePusherPays FeeMode = iota
	FeeModeRepoPays
	FeeModeRepoPaysCapped
)

// BareReference returns an empty reference object
func BareReference() *Reference {
	return &Reference{
		CodecUtil: util.CodecUtil{Version: "0.1"},
		Data:      &ReferenceData{},
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
	Creator ed25519.PushKey `json:"creator" mapstructure:"creator" msgpack:"creator,omitempty"`

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
	Voter                *int    `json:"propVoter,omitempty" mapstructure:"propVoter,omitempty" msgpack:"propVoter,omitempty"`
	PropCreator          *int    `json:"propCreator,omitempty" mapstructure:"propCreator,omitempty" msgpack:"propCreator,omitempty"`
	PropDuration         *string `json:"propDur,omitempty" mapstructure:"propDur,omitempty" msgpack:"propDur,omitempty"`
	PropFee              *string `json:"propFee,omitempty" mapstructure:"propFee,omitempty" msgpack:"propFee,omitempty"`
	PropFeeDepositDur    *string `json:"propFeeDepDur,omitempty" mapstructure:"propFeeDepDur,omitempty" msgpack:"propFeeDepDur,omitempty"`
	PropQuorum           *string `json:"propQuorum,omitempty" mapstructure:"propQuorum,omitempty" msgpack:"propQuorum,omitempty"`
	PropVetoQuorum       *string `json:"propVetoQuorum,omitempty" mapstructure:"propVetoQuorum,omitempty" msgpack:"propVetoQuorum,omitempty"`
	PropVetoOwnersQuorum *string `json:"propVetoOwnersQuorum,omitempty" mapstructure:"propVetoOwnersQuorum,omitempty" msgpack:"propVetoOwnersQuorum,omitempty"`
	PropThreshold        *string `json:"propThreshold,omitempty" mapstructure:"propThreshold,omitempty" msgpack:"propThreshold,omitempty"`
	PropFeeRefundType    *int    `json:"propFeeRefundType,omitempty" mapstructure:"propFeeRefundType,omitempty" msgpack:"propFeeRefundType,omitempty"`
	PropTallyMethod      *int    `json:"propTallyMethod,omitempty" mapstructure:"propTallyMethod,omitempty" msgpack:"propTallyMethod,omitempty"`
	UsePowerAge          *bool   `json:"usePowerAge,omitempty" mapstructure:"usePowerAge,omitempty" msgpack:"usePowerAge,omitempty"`
	CreatorAsContributor *bool   `json:"creatorAsContrib,omitempty" mapstructure:"creatorAsContrib,omitempty" msgpack:"creatorAsContrib,omitempty"`
	NoPropFeeForMergeReq *bool   `json:"noPropFeeForMergeReq,omitempty" mapstructure:"noPropFeeForMergeReq,omitempty" msgpack:"noPropFeeForMergeReq,omitempty"`
}

func (b RepoConfigGovernance) MarshalJSON() ([]byte, error) {
	m := util.ToMap(b)
	if _, ok := m["propVoter"]; ok {
		m["propVoter"] = cast.ToString(m["propVoter"])
	}
	if _, ok := m["propCreator"]; ok {
		m["propCreator"] = cast.ToString(m["propCreator"])
	}
	if _, ok := m["propFeeRefundType"]; ok {
		m["propFeeRefundType"] = cast.ToString(m["propFeeRefundType"])
	}
	if _, ok := m["propTallyMethod"]; ok {
		m["propTallyMethod"] = cast.ToString(m["propTallyMethod"])
	}
	return json.Marshal(m)
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
	Gov            *RepoConfigGovernance `json:"governance,omitempty" mapstructure:"governance,omitempty" msgpack:"governance,omitempty"`
	Policies       RepoPolicies          `json:"policies,omitempty" mapstructure:"policies,omitempty" msgpack:"policies,omitempty"`
}

func (c *RepoConfig) EncodeMsgpack(enc *msgpack.Encoder) error {
	return c.EncodeMulti(enc,
		c.Gov,
		c.Policies)
}

func (c *RepoConfig) DecodeMsgpack(dec *msgpack.Decoder) error {
	return c.DecodeMulti(dec,
		&c.Gov,
		&c.Policies)
}

// Clone clones c
func (c *RepoConfig) Clone() *RepoConfig {
	var clone = BareRepoConfig()
	m := util.ToJSONMap(c)
	_ = mapstructure.Decode(m, &clone)
	return clone
}

// Merge merges the upd into c.
// Slice from upd will replace slice in c.
func (c *RepoConfig) Merge(upd map[string]interface{}) error {
	return util.DecodeMap(upd, c)
}

// IsEmpty checks if c considered empty
func (c *RepoConfig) IsEmpty() bool {
	return (c.Gov == nil || len(util.ToMap(c.Gov)) == 0) && len(c.Policies) == 0
}

// ToJSONToMap converts c to a JSON map and the map to go map.
//
// This allows us to control how c is marshalled to go map
// via custom marshal types.
func (c *RepoConfig) ToJSONToMap() map[string]interface{} {
	return util.ToJSONMap(c)
}

// ToMap converts the object to a map
func (c *RepoConfig) ToMap() map[string]interface{} {
	return util.ToMap(c)
}

var (
	// DefaultRepoConfig is a sane default for repository configurations
	DefaultRepoConfig = MakeDefaultRepoConfig()
)

// MakeDefaultRepoConfig returns sane defaults for repository configurations
func MakeDefaultRepoConfig() *RepoConfig {
	return &RepoConfig{
		Gov: &RepoConfigGovernance{
			CreatorAsContributor: pointer.ToBool(true),
			Voter:                VoterOwner.Ptr(),
			PropCreator:          ProposalCreatorAny.Ptr(),
			UsePowerAge:          pointer.ToBool(false),
			PropDuration:         pointer.ToString(cast.ToString(params.RepoProposalTTL)),
			PropTallyMethod:      ProposalTallyMethodIdentity.Ptr(),
			PropQuorum:           pointer.ToString(cast.ToString(params.DefaultRepoProposalQuorum)),
			PropThreshold:        pointer.ToString(cast.ToString(params.DefaultRepoProposalThreshold)),
			PropVetoQuorum:       pointer.ToString(cast.ToString(params.DefaultRepoProposalVetoQuorum)),
			PropVetoOwnersQuorum: pointer.ToString(cast.ToString(params.DefaultRepoProposalVetoOwnersQuorum)),
			PropFee:              pointer.ToString(cast.ToString(params.DefaultMinProposalFee)),
			PropFeeRefundType:    ProposalFeeRefundNo.Ptr(),
			PropFeeDepositDur:    pointer.ToString("0"),
			NoPropFeeForMergeReq: pointer.ToBool(true),
		},
		Policies: []*Policy{},
	}
}

// MakeZeroValueRepoConfig returns a RepoConfig with all fields set to their zero value
func MakeZeroValueRepoConfig() *RepoConfig {
	return &RepoConfig{
		Gov: &RepoConfigGovernance{
			CreatorAsContributor: pointer.ToBool(false),
			Voter:                pointer.ToInt(0),
			PropCreator:          pointer.ToInt(0),
			UsePowerAge:          pointer.ToBool(false),
			PropDuration:         pointer.ToString("0"),
			PropTallyMethod:      pointer.ToInt(0),
			PropQuorum:           pointer.ToString("0"),
			PropThreshold:        pointer.ToString("0"),
			PropVetoQuorum:       pointer.ToString("0"),
			PropVetoOwnersQuorum: pointer.ToString("0"),
			PropFee:              pointer.ToString("0"),
			PropFeeRefundType:    pointer.ToInt(0),
			PropFeeDepositDur:    pointer.ToString("0"),
			NoPropFeeForMergeReq: pointer.ToBool(false),
		},
		Policies: []*Policy{},
	}
}

// BareRepoConfig returns empty repository configurations
func BareRepoConfig() *RepoConfig {
	return &RepoConfig{
		Gov:      &RepoConfigGovernance{},
		Policies: RepoPolicies{},
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
		Description:  "",
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

	// Balance is the native coin balance
	Balance util.String `json:"balance" msgpack:"balance" mapstructure:"balance"`

	// Description describes the repository
	Description string `json:"desc" msgpack:"desc" mapstructure:"desc"`

	// References contains the repository reference information
	References References `json:"references" msgpack:"references" mapstructure:"references"`

	// Owners contains the address of the repository owners
	Owners RepoOwners `json:"owners" msgpack:"owners" mapstructure:"owners"`

	// Proposals contains the repository governance proposals
	Proposals RepoProposals `json:"proposals" msgpack:"proposals" mapstructure:"proposals"`

	// Contributors contains push keys that can push
	Contributors RepoContributors `json:"contributors" msgpack:"contributors" mapstructure:"contributors"`

	// Config is the repository configuration
	Config *RepoConfig `json:"config" msgpack:"config" mapstructure:"config"`

	// CreatedAt is the block height the reference was created
	CreatedAt util.UInt64 `json:"createdAt" mapstructure:"createdAt" msgpack:"createdAt,omitempty"`

	// UpdatedAt is the block height the reference was last updated
	UpdatedAt util.UInt64 `json:"updatedAt" mapstructure:"updatedAt" msgpack:"updatedAt,omitempty"`
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
func (r *Repository) Clean(_ uint64) {}

// AddOwner adds an owner
func (r *Repository) AddOwner(ownerAddress string, owner *RepoOwner) {
	r.Owners[ownerAddress] = owner
}

// IsEmpty returns true if the repo is considered empty
func (r *Repository) IsEmpty() bool {
	return r.Balance.IsZero() &&
		len(r.Description) == 0 &&
		len(r.References) == 0 &&
		len(r.Owners) == 0 &&
		len(r.Proposals) == 0 &&
		len(r.Contributors) == 0 &&
		r.Config.IsEmpty() &&
		r.CreatedAt == 0 &&
		r.UpdatedAt == 0
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (r *Repository) EncodeMsgpack(enc *msgpack.Encoder) error {
	return r.EncodeMulti(enc,
		r.Balance,
		r.Description,
		r.Owners,
		r.References,
		r.Proposals,
		r.Config,
		r.Contributors,
		r.CreatedAt,
		r.UpdatedAt,
	)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (r *Repository) DecodeMsgpack(dec *msgpack.Decoder) error {
	err := r.DecodeMulti(dec,
		&r.Balance,
		&r.Description,
		&r.Owners,
		&r.References,
		&r.Proposals,
		&r.Config,
		&r.Contributors,
		&r.CreatedAt,
		&r.UpdatedAt,
	)
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
