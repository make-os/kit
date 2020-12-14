package dht

import (
	"bytes"
	"fmt"
	"io"

	"github.com/ipfs/go-cid"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/identifier"
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
	ObjectNamespace = "obj"
)

var (
	ErrObjNotFound = fmt.Errorf("object not found")
	MsgTypeLen     = 4
)

// ParseObjectKeyToHex parses an object key to an hex-encoded version
func ParseObjectKeyToHex(key []byte) (string, error) {
	return util.ToHex(key, true), nil
}

// MakeWantMsg creates a 'WANT' message.
//  - Format: WANT <reponame> <20 bytes hash>
//  - <reponame>: Length varies but not more than MaxResourceNameLength
func MakeWantMsg(repoName string, hash []byte) []byte {
	return append([]byte(fmt.Sprintf("%s %s ", MsgTypeWant, repoName)), hash...)
}

// MakeSendMsg creates a 'SEND' message
//  - Format: SEND <reponame> <20 bytes hash>
//  - <reponame>: Length varies but not more than MaxResourceNameLength
func MakeSendMsg(repoName string, hash []byte) []byte {
	return append([]byte(fmt.Sprintf("%s %s ", MsgTypeSend, repoName)), hash...)
}

// ParseWantOrSendMsg parses a 'WANT/SEND' message
func ParseWantOrSendMsg(msg []byte) (typ string, repoName string, hash []byte, err error) {
	parts := bytes.SplitN(msg, []byte(" "), 3)
	if len(parts) != 3 {
		return "", "", nil, fmt.Errorf("malformed message")
	}
	return string(parts[0]), string(parts[1]), parts[2][:20], nil
}

// ReadWantOrSendMsg reads WANT or SEND message from the reader
func ReadWantOrSendMsg(r io.Reader) (typ string, repoName string, hash []byte, err error) {
	var buf = make([]byte, MsgTypeLen+identifier.MaxResourceNameLength+20)
	_, err = r.Read(buf)
	if err != nil && err != io.EOF {
		return "", "", nil, err
	}
	return ParseWantOrSendMsg(buf)
}

// MakeHaveMsg creates a 'HAVE' message
func MakeHaveMsg() []byte {
	return []byte(MsgTypeHave)
}

// MakeNopeMsg creates a 'NOPE' message
func MakeNopeMsg() []byte {
	return []byte(MsgTypeNope)
}

// MakeCID creates a content ID
func MakeCID(data []byte) (cid.Cid, error) {
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
