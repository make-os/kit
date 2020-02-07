package types

import (
	"reflect"

	"github.com/fatih/structs"
	"github.com/makeos/mosdef/params"
	"github.com/makeos/mosdef/util"
	"github.com/mitchellh/mapstructure"
	"github.com/vmihailenco/msgpack"
)

// BareReference returns an empty reference object
func BareReference() *Reference {
	return &Reference{}
}

// Reference represents a git reference
type Reference struct {
	Nonce uint64 `json:"nonce" mapstructure:"nonce" msgpack:"nonce"`
}

// References represents a collection of references
// Note: we are using map[string]interface{} instead of map[string]*Reference
// because we want to take advantage of msgpack map sorting which only works on the
// former.
// CONTRACT: interface{} is always *Reference
type References map[string]interface{}

// Get a reference by name, returns empty reference if not found.
func (r *References) Get(name string) *Reference {
	ref, _ := (*r)[name]
	if ref == nil {
		return BareReference()
	}
	return ref.(*Reference)
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
// Note: we are using map[string]interface{} instead of map[string]*RepoOwner
// because we want to take advantage of msgpack map sorting which only works on the
// former.
// CONTRACT: interface{} is always *RepoOwner
type RepoOwners map[string]interface{}

// Has returns true of address exist
func (r RepoOwners) Has(address string) bool {
	return r[address] != nil
}

// Get return a repo owner associated with the given address
func (r RepoOwners) Get(address string) *RepoOwner {
	ro, ok := r[address]
	if !ok {
		return nil
	}
	switch v := ro.(type) {
	case *RepoOwner:
		return v
	case map[string]interface{}:
		var ro RepoOwner
		mapstructure.Decode(v, &ro)
		r[address] = &ro
		return &ro
	}
	return nil
}

// ForEach iterates through the collection passing each item to the iter callback
func (r RepoOwners) ForEach(iter func(o *RepoOwner, addr string)) {
	for key := range r {
		iter(r.Get(key), key)
	}
}

// RepoConfigGovernance contains governance settings for a repository
type RepoConfigGovernance struct {
	ProposalProposee                 ProposeeType          `json:"propProposee" mapstructure:"propProposee,omitempty" msgpack:"propProposee"`
	ProposalProposeeLimitToCurHeight bool                  `json:"propProposeeLimitToCurHeight" mapstructure:"propProposeeLimitToCurHeight,omitempty" msgpack:"propProposeeLimitToCurHeight"`
	ProposalDur                      uint64                `json:"propDuration" mapstructure:"propDuration,omitempty" msgpack:"propDuration"`
	ProposalTallyMethod              ProposalTallyMethod   `json:"propTallyMethod" mapstructure:"propTallyMethod,omitempty" msgpack:"propTallyMethod"`
	ProposalQuorum                   float64               `json:"propQuorum" mapstructure:"propQuorum,omitempty" msgpack:"propQuorum"`
	ProposalThreshold                float64               `json:"propThreshold" mapstructure:"propThreshold,omitempty" msgpack:"propThreshold"`
	ProposalVetoQuorum               float64               `json:"propVetoQuorum" mapstructure:"propVetoQuorum,omitempty" msgpack:"propVetoQuorum"`
	ProposalVetoOwnersQuorum         float64               `json:"propVetoOwnersQuorum" mapstructure:"propVetoOwnersQuorum,omitempty" msgpack:"propVetoOwnersQuorum"`
	ProposalFee                      float64               `json:"propFee" mapstructure:"propFee,omitempty" msgpack:"propFee"`
	ProposalFeeRefundType            ProposalFeeRefundType `json:"propFeeRefundType" mapstructure:"propFeeRefundType,omitempty" msgpack:"propFeeRefundType"`
}

// RepoConfig contains repo-specific configuration settings
type RepoConfig struct {
	Governace *RepoConfigGovernance `json:"gov" mapstructure:"gov" msgpack:"gov"`
}

// Clone clones c
func (c *RepoConfig) Clone() *RepoConfig {
	var clone RepoConfig
	m := util.StructToMap(c)
	mapstructure.Decode(m, &clone)
	return &clone
}

// MergeMap merges map o into c
func (c *RepoConfig) MergeMap(o map[string]interface{}) {
	baseMap := util.StructToMap(c)
	mapstructure.Decode(o, &baseMap)
	mapstructure.Decode(baseMap, c)
}

// Merge merges non-zero fields of o into c
func (c *RepoConfig) Merge(o *RepoConfig) {
	if c.Governace == nil || o == nil || o.Governace == nil {
		return
	}

	var merger func(base, target *structs.Struct)
	merger = func(base, target *structs.Struct) {
		for _, baseField := range base.Fields() {
			if targetField := target.Field(baseField.Name()); !targetField.IsZero() ||
				targetField.Kind() == reflect.Bool {
				if structs.IsStruct(targetField.Value()) && structs.IsStruct(baseField.Value()) {
					merger(structs.New(baseField.Value()), structs.New(targetField.Value()))
					continue
				}
				baseField.Set(targetField.Value())
			}
		}
	}

	merger(structs.New(c), structs.New(o))
}

// IsNil checks if the object's field all have zero value
func (c *RepoConfig) IsNil() bool {
	return c.Governace == nil || *c.Governace == RepoConfigGovernance{}
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
		Governace: &RepoConfigGovernance{
			ProposalProposee:                 ProposeeOwner,
			ProposalProposeeLimitToCurHeight: false,
			ProposalDur:                      params.RepoProposalDur,
			ProposalTallyMethod:              ProposalTallyMethodIdentity,
			ProposalQuorum:                   params.RepoProposalQuorum,
			ProposalThreshold:                params.RepoProposalThreshold,
			ProposalVetoQuorum:               params.RepoProposalVetoQuorum,
			ProposalVetoOwnersQuorum:         params.RepoProposalVetoOwnersQuorum,
			ProposalFee:                      params.MinProposalFee,
			ProposalFeeRefundType:            0,
		},
	}
}

// BareRepoConfig returns empty repository configurations
func BareRepoConfig() *RepoConfig {
	return &RepoConfig{
		Governace: &RepoConfigGovernance{},
	}
}

// BareRepository returns an empty repository object
func BareRepository() *Repository {
	return &Repository{
		Balance:    "0",
		References: make(map[string]interface{}),
		Owners:     make(map[string]interface{}),
		Proposals:  make(map[string]interface{}),
		Config:     BareRepoConfig(),
	}
}

// Repository represents a git repository.
type Repository struct {
	util.DecoderHelper `json:"-" msgpack:"-" mapstructure:"-"`
	Balance            util.String   `json:"balance" msgpack:"balance" mapstructure:"balance"`
	References         References    `json:"references" msgpack:"references" mapstructure:"references"`
	Owners             RepoOwners    `json:"owners" msgpack:"owners" mapstructure:"owners"`
	Proposals          RepoProposals `json:"proposals" msgpack:"proposals" mapstructure:"proposals"`
	Config             *RepoConfig   `json:"config" msgpack:"config" mapstructure:"config"`
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
	return enc.EncodeMulti(
		r.Balance,
		r.References,
		r.Owners,
		r.Proposals,
		r.Config)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (r *Repository) DecodeMsgpack(dec *msgpack.Decoder) error {
	return r.DecodeMulti(dec,
		&r.Balance,
		&r.References,
		&r.Owners,
		&r.Proposals,
		&r.Config)
}

// Bytes return the bytes equivalent of the account
func (r *Repository) Bytes() []byte {
	return util.ObjectToBytes(r)
}

// NewRepositoryFromBytes decodes bz to Repository
func NewRepositoryFromBytes(bz []byte) (*Repository, error) {

	var repo = BareRepository()
	if err := util.BytesToObject(bz, repo); err != nil {
		return nil, err
	}

	for k, v := range repo.References {
		var ref Reference
		mapstructure.Decode(v, &ref)
		repo.References[k] = &ref
	}

	for k, v := range repo.Owners {
		var owner RepoOwner
		mapstructure.Decode(v, &owner)
		repo.AddOwner(k, &owner)
	}

	for k, v := range repo.Proposals {
		var prop RepoProposal
		mapstructure.Decode(v, &prop)
		repo.Proposals.Add(k, &prop)
	}

	return repo, nil
}
