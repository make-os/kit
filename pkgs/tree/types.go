package tree

// Tree represents a merkle tree
type Tree interface {
	Version() int64
	GetVersioned(key []byte, version int64) (index int64, value []byte)
	Get(key []byte) (index int64, value []byte)
	Set(key, value []byte) bool
	Remove(key []byte) bool
	SaveVersion() ([]byte, int64, error)
	Load() (int64, error)
	WorkingHash() []byte
	Hash() []byte
}
