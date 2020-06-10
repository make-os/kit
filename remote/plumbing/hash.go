package plumbing

import (
	"encoding/hex"

	"gopkg.in/src-d/go-git.v4/plumbing"
)

// MakeCommitHash creates and returns a commit hash from the specified data
func MakeCommitHash(data string) plumbing.Hash {
	return plumbing.ComputeHash(plumbing.CommitObject, []byte(data))
}

// HashToBytes decodes an object hash to bytes
func HashToBytes(hexStr string) []byte {
	bz, err := hex.DecodeString(hexStr)
	if err != nil {
		panic("input is bad hex")
	}
	return bz
}

// BytesToHash converts a byte slice to plumbing.Hash.
func BytesToHash(bz []byte) plumbing.Hash {
	var hash plumbing.Hash
	copy(hash[:], bz[:20])
	return hash
}

// HashToBytes decodes an object hash to bytes
func BytesToHex(bz []byte) string {
	return hex.EncodeToString(bz)
}

// isZeroHash checks whether a given hash is a zero git hash
func IsZeroHash(h string) bool {
	return h == plumbing.ZeroHash.String()
}
