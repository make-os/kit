package plumbing

import (
	"fmt"
	"strings"
)

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
