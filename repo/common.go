package repo

import (
	"fmt"
)

// ErrRepoNotFound means a repo was not found on the local storage
var ErrRepoNotFound = fmt.Errorf("repo not found")

type kvOption struct {
	Key   string
	Value interface{}
}

func getKVOpt(key string, options []kvOption) interface{} {
	for _, opt := range options {
		if opt.Key == key {
			return opt.Value
		}
	}
	return nil
}

func matchOpt(val string) kvOption {
	return kvOption{Key: "match", Value: val}
}

func changesOpt(ch *Changes) kvOption {
	return kvOption{Key: "changes", Value: ch}
}

// PGPPubKeyGetter represents a function for fetching PGP public key
type PGPPubKeyGetter func(pkId string) (string, error)
