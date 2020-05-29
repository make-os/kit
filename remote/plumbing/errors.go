package plumbing

import "fmt"

var (
	ErrRefNotFound = fmt.Errorf("reference not found")
	ErrNoCommits   = fmt.Errorf("no commits")
)
