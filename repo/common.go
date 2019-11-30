package repo

import (
	"fmt"
)

// ErrRepoNotFound means a repo was not found on the local storage
var ErrRepoNotFound = fmt.Errorf("repo not found")

// KVOption holds key-value structure of options
type KVOption struct {
	Key   string
	Value interface{}
}

func getKVOpt(key string, options []KVOption) interface{} {
	for _, opt := range options {
		if opt.Key == key {
			return opt.Value
		}
	}
	return nil
}

func matchOpt(val string) KVOption {
	return KVOption{Key: "match", Value: val}
}

func changesOpt(ch *Changes) KVOption {
	return KVOption{Key: "changes", Value: ch}
}

// PGPPubKeyGetter represents a function for fetching PGP public key
type PGPPubKeyGetter func(pkId string) (string, error)
