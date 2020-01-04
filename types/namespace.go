package types

import (
	"github.com/makeos/mosdef/util"
	"github.com/vmihailenco/msgpack"
)

// Namespace describes an object for storing human-readable names mapping to
// various network resources
type Namespace struct {
	Owner      string           `json:"owner" mapstructure:"owner" msgpack:"owner"`
	GraceEndAt uint64           `json:"graceEndAt" mapstructure:"graceEndAt" msgpack:"graceEndAt"`
	ExpiresAt  uint64           `json:"expiresAt" mapstructure:"expiresAt" msgpack:"expiresAt"`
	Targets    NamespaceTargets `json:"targets" mapstructure:"targets" msgpack:"targets"`
}

// NamespaceTargets represents a map of human-readable names to their original,
// usually unreadable name
type NamespaceTargets map[string]string

// BareNamespace returns an empty namespace object
func BareNamespace() *Namespace {
	return &Namespace{
		Targets: make(map[string]string),
	}
}

// IsNil returns true if the repo fields are set to their nil value
func (ns *Namespace) IsNil() bool {
	return ns.Owner == "" &&
		ns.GraceEndAt == 0 &&
		ns.ExpiresAt == 0 &&
		len(ns.Targets) == 0
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (ns *Namespace) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.EncodeMulti(
		ns.Owner,
		ns.GraceEndAt,
		ns.ExpiresAt,
		ns.Targets)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (ns *Namespace) DecodeMsgpack(dec *msgpack.Decoder) error {
	err := dec.DecodeMulti(
		&ns.Owner,
		&ns.GraceEndAt,
		&ns.ExpiresAt,
		&ns.Targets)
	if err != nil {
		return err
	}
	return nil
}

// Bytes return the bytes equivalent of the account
func (ns *Namespace) Bytes() []byte {
	return util.ObjectToBytes(ns)
}

// NewNamespaceFromBytes decodes bz to Namespace
func NewNamespaceFromBytes(bz []byte) (*Namespace, error) {
	var ns = BareNamespace()
	if err := util.BytesToObject(bz, ns); err != nil {
		return nil, err
	}
	return ns, nil
}
