package plumbing

import "fmt"

var (
	ErrRefNotFound = fmt.Errorf("ref not found")
	ErrNoCommits   = fmt.Errorf("no commits")
)
