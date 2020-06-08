package dht

import (
	"bytes"
	"fmt"
)

const (
	MsgTypeWant = "WANT"
	MsgTypeHave = "HAVE"
	MsgTypeSend = "SEND"
	MsgTypeNope = "NOPE"
	MsgTypePack = "PACK"
)

// MakeCommitKey creates a commit query key
func MakeCommitKey(hash []byte) []byte {
	key := []byte(fmt.Sprintf("%s/", CommitKeyID))
	return append(key, hash...)
}

// parseCommitKey parses a commit query key
func parseCommitKey(key []byte) ([]byte, error) {
	parts := bytes.Split(key, []byte("/"))
	if len(parts) != 3 {
		return nil, fmt.Errorf("malformed commit key")
	}
	return parts[2], nil
}

// MakeWantMsg creates a 'WANT' message
func MakeWantMsg(repoName string, hash []byte) []byte {
	return append([]byte(fmt.Sprintf("%s %s ", MsgTypeWant, repoName)), hash...)
}

// parseWantOrSendMsg parses a 'WANT/SEND' message
func parseWantOrSendMsg(msg []byte) (repoName string, hash []byte, err error) {
	parts := bytes.Fields(msg)
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
