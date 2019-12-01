package repo

import (
	"fmt"

	"github.com/makeos/mosdef/types"
)

// ErrRepoNotFound means a repo was not found on the local storage
var ErrRepoNotFound = fmt.Errorf("repo not found")

func getKVOpt(key string, options []types.KVOption) interface{} {
	for _, opt := range options {
		if opt.Key == key {
			return opt.Value
		}
	}
	return nil
}

func matchOpt(val string) types.KVOption {
	return types.KVOption{Key: "match", Value: val}
}

func changesOpt(ch *types.Changes) types.KVOption {
	return types.KVOption{Key: "changes", Value: ch}
}
