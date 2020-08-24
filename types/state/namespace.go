package state

import (
	"github.com/make-os/lobe/util"
	"github.com/vmihailenco/msgpack"
)

// Namespace describes an object for storing human-readable names mapping to
// various network resources
type Namespace struct {
	util.CodecUtil `json:"-" msgpack:"-"`
	Owner          string           `json:"owner" mapstructure:"owner" msgpack:"owner"`
	GraceEndAt     util.UInt64      `json:"graceEndAt" mapstructure:"graceEndAt" msgpack:"graceEndAt"`
	ExpiresAt      util.UInt64      `json:"expiresAt" mapstructure:"expiresAt" msgpack:"expiresAt"`
	Domains        NamespaceDomains `json:"domains" mapstructure:"domains" msgpack:"domains"`
	Contributors   BaseContributors `json:"contributors" msgpack:"contributors" mapstructure:"contributors"`
}

// NamespaceDomains represents a map of human-readable names to their original,
// usually unreadable name
type NamespaceDomains map[string]string

// Get the target of a domain
func (nd *NamespaceDomains) Get(domain string) string {
	return (*nd)[domain]
}

// BareNamespace returns an empty namespace object
func BareNamespace() *Namespace {
	return &Namespace{
		Domains:      make(map[string]string),
		Contributors: map[string]*BaseContributor{},
	}
}

// IsNil returns true if the repo fields are set to their nil value
func (ns *Namespace) IsNil() bool {
	return ns.Owner == "" &&
		ns.GraceEndAt == 0 &&
		ns.ExpiresAt == 0 &&
		len(ns.Domains) == 0 &&
		len(ns.Contributors) == 0
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (ns *Namespace) EncodeMsgpack(enc *msgpack.Encoder) error {
	return ns.EncodeMulti(enc,
		ns.Owner,
		ns.GraceEndAt,
		ns.ExpiresAt,
		ns.Domains,
		ns.Contributors)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (ns *Namespace) DecodeMsgpack(dec *msgpack.Decoder) error {
	return ns.DecodeMulti(dec,
		&ns.Owner,
		&ns.GraceEndAt,
		&ns.ExpiresAt,
		&ns.Domains,
		&ns.Contributors)
}

// Bytes return the bytes equivalent of the account
func (ns *Namespace) Bytes() []byte {
	return util.ToBytes(ns)
}

// NewNamespaceFromBytes decodes bz to Namespace
func NewNamespaceFromBytes(bz []byte) (*Namespace, error) {
	var ns = BareNamespace()
	if err := util.ToObject(bz, ns); err != nil {
		return nil, err
	}
	return ns, nil
}
