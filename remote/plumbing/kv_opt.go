package plumbing

import (
	"github.com/make-os/kit/remote/types"
)

// GetKVOpt finds and returns an option matching the given key
func GetKVOpt(key string, options []types.KVOption) interface{} {
	for _, opt := range options {
		if opt.Key == key {
			return opt.Value
		}
	}
	return nil
}

// MatchOpt creates a KVOption with 'match' as key
func MatchOpt(val string) types.KVOption {
	return types.KVOption{Key: "match", Value: val}
}

// Changes creates a KVOption with 'changes' as key
func ChangesOpt(ch *types.Changes) types.KVOption {
	return types.KVOption{Key: "changes", Value: ch}
}
