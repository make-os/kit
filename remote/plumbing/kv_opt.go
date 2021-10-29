package plumbing

// GetKVOpt finds and returns an option matching the given key
func GetKVOpt(key string, options []KVOption) interface{} {
	for _, opt := range options {
		if opt.Key == key {
			return opt.Value
		}
	}
	return nil
}

// MatchOpt creates a KVOption with 'match' as key
func MatchOpt(val string) KVOption {
	return KVOption{Key: "match", Value: val}
}

// Changes creates a KVOption with 'changes' as key
func ChangesOpt(ch *Changes) KVOption {
	return KVOption{Key: "changes", Value: ch}
}
