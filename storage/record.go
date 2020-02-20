package storage

import (
	"bytes"
	"fmt"

	"gitlab.com/makeos/mosdef/util"
)

// ErrRecordNotFound indicates that a record was not found
var ErrRecordNotFound = fmt.Errorf("record not found")

const (
	// KeyPrefixSeparator is used to separate prefix and key
	KeyPrefixSeparator = ";"
	prefixSeparator    = ":"
)

// Record represents an item in the database
type Record struct {
	Key    []byte `json:"key"`
	Value  []byte `json:"value"`
	Prefix []byte `json:"prefix"`
}

// IsEmpty checks whether the object is empty
func (r *Record) IsEmpty() bool {
	return len(r.Key) == 0 && len(r.Value) == 0
}

// Scan marshals the value into dest
func (r *Record) Scan(dest interface{}) error {
	return util.BytesToObject(r.Value, &dest)
}

// MakePrefix creates a prefix string
func MakePrefix(prefixes ...[]byte) (result []byte) {
	return bytes.Join(prefixes, []byte(prefixSeparator))
}

// SplitPrefix splits joined prefixes into their individual parts
func SplitPrefix(prefixes []byte) [][]byte {
	return bytes.Split(prefixes, []byte(prefixSeparator))
}

// MakeKey constructs a key from the key and prefixes
func MakeKey(key []byte, prefixes ...[]byte) []byte {
	var prefix = MakePrefix(prefixes...)
	var sep = []byte(KeyPrefixSeparator)
	if len(key) == 0 || len(prefix) == 0 {
		sep = []byte{}
	}
	return append(prefix, append(sep, key...)...)
}

// GetKey creates and returns the key
func (r *Record) GetKey() []byte {
	return MakeKey(r.Key, r.Prefix)
}

// Equal performs equality check with another Record
func (r *Record) Equal(other *Record) bool {
	return bytes.Equal(r.Key, other.Key) && bytes.Equal(r.Value, other.Value)
}

// NewRecord creates a key value object.
// The prefixes provided is joined together and prepended to the key before insertion.
func NewRecord(key, value []byte, prefixes ...[]byte) *Record {
	return &Record{Key: key, Value: value, Prefix: MakePrefix(prefixes...)}
}

// NewFromKeyValue takes a key and creates a Record
func NewFromKeyValue(key []byte, value []byte) *Record {

	var k, p []byte

	// break down the key to determine the prefix and the original key.
	parts := bytes.SplitN(key, []byte(KeyPrefixSeparator), 2)

	// If there are more than 2 parts, it is an invalid key.
	// If there are only two parts, then the 0 index is the prefix while 1 is the key.
	// It there are only one part, the 0 part is considered the key.
	partsLen := len(parts)
	if partsLen > 2 {
		panic("invalid key format: " + string(key))
	} else if partsLen == 2 {
		k = parts[1]
		p = parts[0]
	} else if partsLen == 1 {
		k = parts[0]
	}

	return &Record{
		Key:    k,
		Value:  value,
		Prefix: p,
	}
}
