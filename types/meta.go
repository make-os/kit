package types

const TxMetaKeyAllowNonceGap = "allowNonceGap"

// Meta stores arbitrary, self-contained state information for a transaction
type Meta struct {
	meta map[string]interface{}
}

// NewMeta creates an instance of Meta
func NewMeta() *Meta {
	return &Meta{map[string]interface{}{}}
}

// HasMetaKey returns true if the given key exist in the meta map
func (m *Meta) HasMetaKey(key string) bool {
	if m == nil {
		return false
	}
	return m.meta[key] != nil
}

// GetMeta returns the meta information of the transaction
func (m *Meta) GetMeta() map[string]interface{} {
	if m == nil {
		return make(map[string]interface{})
	}
	return m.meta
}

// SetMeta set key and value
func (m *Meta) SetMeta(key string, val interface{}) {
	m.meta[key] = val
}
