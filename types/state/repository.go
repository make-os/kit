package state

import (
	"github.com/imdario/mergo"
	"github.com/mitchellh/mapstructure"
	"github.com/vmihailenco/msgpack"
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
	return &Reference{}
}

// Reference represents a git reference
type Reference struct {
	util.SerializerHelper `json:"-" mapstructure:"-" msgpack:"-"`
	Nonce                 uint64 `json:"nonce" mapstructure:"nonce" msgpack:"nonce"`
	Hash                  []byte `json:"hash" mapstructure:"hash" msgpack:"hash"`
}

func (r *Reference) EncodeMsgpack(enc *msgpack.Encoder) error {
	return r.EncodeMulti(enc, r.Nonce, r.Hash)
}

func (r *Reference) DecodeMsgpack(dec *msgpack.Decoder) error {
	return r.DecodeMulti(dec, &r.Nonce, &r.Hash)
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
	Creator  bool   `json:"creator" mapstructure:"creator" msgpack:"creator"`
	JoinedAt uint64 `json:"joinedAt" mapstructure:"joinedAt" msgpack:"joinedAt"`
	Veto     bool   `json:"veto" mapstructure:"veto" msgpack:"veto"`
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
	Voter                    VoterType             `json:"propVoter" mapstructure:"propVoter,omitempty" msgpack:"propVoter"`
	ProposalCreator          ProposalCreatorType   `json:"propCreator" mapstructure:"propCreator,omitempty" msgpack:"propCreator"`
	VoterAgeAsCurHeight      bool                  `json:"voterAgeAsCurHeight" mapstructure:"voterAgeAsCurHeight" msgpack:"voterAgeAsCurHeight"`
	DurOfProposal            uint64                `json:"propDur" mapstructure:"propDur,omitempty" msgpack:"propDur"`
	FeeDepositDurOfProposal  uint64                `json:"propFeeDepDur" mapstructure:"propFeeDepDur,omitempty" msgpack:"propFeeDepDur"`
	TallyMethodOfProposal    ProposalTallyMethod   `json:"propTallyMethod" mapstructure:"propTallyMethod,omitempty" msgpack:"propTallyMethod"`
	ProposalQuorum           float64               `json:"propQuorum" mapstructure:"propQuorum,omitempty" msgpack:"propQuorum"`
	ProposalThreshold        float64               `json:"propThreshold" mapstructure:"propThreshold,omitempty" msgpack:"propThreshold"`
	ProposalVetoQuorum       float64               `json:"propVetoQuorum" mapstructure:"propVetoQuorum,omitempty" msgpack:"propVetoQuorum"`
	ProposalVetoOwnersQuorum float64               `json:"propVetoOwnersQuorum" mapstructure:"propVetoOwnersQuorum,omitempty" msgpack:"propVetoOwnersQuorum"`
	ProposalFee              float64               `json:"propFee" mapstructure:"propFee,omitempty" msgpack:"propFee"`
	ProposalFeeRefundType    ProposalFeeRefundType `json:"propFeeRefundType" mapstructure:"propFeeRefundType,omitempty" msgpack:"propFeeRefundType"`
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
	util.SerializerHelper `json:"-" mapstructure:"-" msgpack:"-"`
	Governance            *RepoConfigGovernance `json:"governance" mapstructure:"governance,omitempty" msgpack:"governance"`
	Policies              RepoPolicies          `json:"policies" mapstructure:"policies" msgpack:"policies"`
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
	m := util.StructToMap(c)
	_ = mapstructure.Decode(m, &clone)
	return clone
}

// MergeMap merges the specified map into c
func (c *RepoConfig) MergeMap(m map[string]interface{}) {

	// Decode the specified map to RepoConfig so
	// that we get a compatible source type
	mCfg := RepoConfig{}
	mapstructure.Decode(m, &mCfg)
	src := util.StructToMap(mCfg, "mapstructure")

	dst := util.StructToMap(c, "mapstructure")
	mergo.Merge(&dst, src, mergo.WithOverride, mergo.WithOverwriteWithEmptyValue)
	newCfg := RepoConfig{}
	mapstructure.Decode(dst, &newCfg)

	*c = newCfg
}

// IsNil checks if the object's field all have zero value
func (c *RepoConfig) IsNil() bool {
	return (c.Governance == nil || *c.Governance == RepoConfigGovernance{}) &&
		len(c.Policies) == 0
}

// ToMap converts the object to map
func (c *RepoConfig) ToMap() map[string]interface{} {
	return util.StructToMap(c, "mapstructure")
}

var (
	// DefaultRepoConfig is a sane default for repository configurations
	DefaultRepoConfig = MakeDefaultRepoConfig()
)

// MakeDefaultRepoConfig returns sane defaults for repository configurations
func MakeDefaultRepoConfig() *RepoConfig {
	return &RepoConfig{
		Governance: &RepoConfigGovernance{
			Voter:                    VoteByOwner,
			ProposalCreator:          ProposalCreatorAny,
			VoterAgeAsCurHeight:      false,
			DurOfProposal:            params.RepoProposalDur,
			TallyMethodOfProposal:    ProposalTallyMethodIdentity,
			ProposalQuorum:           params.RepoProposalQuorum,
			ProposalThreshold:        params.RepoProposalThreshold,
			ProposalVetoQuorum:       params.RepoProposalVetoQuorum,
			ProposalVetoOwnersQuorum: params.RepoProposalVetoOwnersQuorum,
			ProposalFee:              params.MinProposalFee,
			ProposalFeeRefundType:    ProposalFeeRefundNo,
			FeeDepositDurOfProposal:  0,
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
		References:   make(map[string]*Reference),
		Owners:       make(map[string]*RepoOwner),
		Proposals:    make(map[string]*RepoProposal),
		Config:       BareRepoConfig(),
		Contributors: make(map[string]*RepoContributor),
	}
}

// Repository represents a git repository.
type Repository struct {
	util.SerializerHelper `json:"-" msgpack:"-" mapstructure:"-"`
	Balance               util.String      `json:"balance" msgpack:"balance" mapstructure:"balance"`
	References            References       `json:"references" msgpack:"references" mapstructure:"references"`
	Owners                RepoOwners       `json:"owners" msgpack:"owners" mapstructure:"owners"`
	Proposals             RepoProposals    `json:"proposals" msgpack:"proposals" mapstructure:"proposals"`
	Contributors          RepoContributors `json:"contributors" msgpack:"contributors" mapstructure:"contributors"`
	Config                *RepoConfig      `json:"config" msgpack:"config" mapstructure:"config"`
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
	return r.DecodeMulti(dec,
		&r.Balance,
		&r.Owners,
		&r.References,
		&r.Proposals,
		&r.Config,
		&r.Contributors)
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
