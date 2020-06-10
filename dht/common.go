package dht

import (
	"bytes"
	"fmt"

	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multihash"
)

const (
	MsgTypeWant = "WANT"
	MsgTypeHave = "HAVE"
	MsgTypeSend = "SEND"
	MsgTypeNope = "NOPE"
	MsgTypePack = "PACK"
)

const (
	ObjectKeyID     = "/o"
	ObjectNamespace = "obj"
)

var (
	ErrObjNotFound = fmt.Errorf("object not found")
)

// MakeObjectKey creates an object key
func MakeObjectKey(hash []byte) []byte {
	key := []byte(fmt.Sprintf("%s/", ObjectKeyID))
	return append(key, hash...)
}

// ParseObjectKey parses an object key
func ParseObjectKey(key []byte) ([]byte, error) {
	parts := bytes.SplitN(key, []byte("/"), 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("malformed commit key")
	}
	return parts[2], nil
}

// MakeWantMsg creates a 'WANT' message
func MakeWantMsg(repoName string, hash []byte) []byte {
	return append([]byte(fmt.Sprintf("%s %s ", MsgTypeWant, repoName)), hash...)
}

// ParseWantOrSendMsg parses a 'WANT/SEND' message
func ParseWantOrSendMsg(msg []byte) (repoName string, hash []byte, err error) {
	parts := bytes.SplitN(msg, []byte(" "), 3)
	if len(parts) != 3 {
		return "", nil, fmt.Errorf("malformed message")
	}
	return string(parts[1]), parts[2], nil
}

// MakeHaveMsg creates a 'HAVE' message
func MakeHaveMsg() []byte {
	return []byte(MsgTypeHave)
}

// MakeNopeMsg creates a 'NOPE' message
func MakeNopeMsg() []byte {
	return []byte(MsgTypeNope)
}

// MakeSendMsg creates a 'SEND' message
func MakeSendMsg(repoName string, hash []byte) []byte {
	return append([]byte(fmt.Sprintf("%s %s ", MsgTypeSend, repoName)), hash...)
}

// MakeCid creates a content ID
func MakeCid(data []byte) (cid.Cid, error) {
	hash, err := multihash.Sum(data, multihash.BLAKE2B_MAX, -1)
	if err != nil {
		return cid.Cid{}, err
	}
	return cid.NewCidV1(cid.Raw, hash), nil
}

// MakeKey returns a key for storing an object
func MakeKey(key string) string {
	return fmt.Sprintf("/%s/%s", ObjectNamespace, key)
}
