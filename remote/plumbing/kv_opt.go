package plumbing

import "gitlab.com/makeos/mosdef/types/core"

// GetKVOpt finds and returns an option matching the given key
func GetKVOpt(key string, options []core.KVOption) interface{} {
	for _, opt := range options {
		if opt.Key == key {
			return opt.Value
		}
	}
	return nil
}

// MatchOpt creates a KVOption with 'match' as key
func MatchOpt(val string) core.KVOption {
	return core.KVOption{Key: "match", Value: val}
}

// Changes creates a KVOption with 'changes' as key
func ChangesOpt(ch *core.Changes) core.KVOption {
	return core.KVOption{Key: "changes", Value: ch}
}
