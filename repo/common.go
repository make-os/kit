package repo

import (
	"fmt"
	"strings"

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

// MakeRepoObjectDHTKey returns a key for announcing a repository object
func MakeRepoObjectDHTKey(repoName, hash string) string {
	return fmt.Sprintf("%s/%s", repoName, hash)
}

// ParseRepoObjectDHTKey parses a dht key for finding repository objects
func ParseRepoObjectDHTKey(key string) (repoName string, hash string, err error) {
	parts := strings.Split(key, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid repo object dht key")
	}
	return parts[0], parts[1], nil
}
