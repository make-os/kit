package plumbing

import (
	"fmt"
	"regexp"

	"gopkg.in/src-d/go-git.v4/plumbing"
)

var IssueBranchPrefix = "issues"

// isBranch checks whether a reference name indicates a branch
func IsBranch(name string) bool {
	return plumbing.ReferenceName(name).IsBranch()
}

// isIssueBranch checks whether a branch is an issue branch
func IsIssueReference(name string) bool {
	return regexp.MustCompile(fmt.Sprintf("^refs/heads/%s/[1-9]+([0-9]+)?$", IssueBranchPrefix)).MatchString(name)
}

// IsIssueReferencePath checks if the specified reference matches an issue reference path
func IsIssueReferencePath(name string) bool {
	return regexp.MustCompile(fmt.Sprintf("^refs/heads/%s(/|$)?", IssueBranchPrefix)).MatchString(name)
}

// isReference checks the given name is a reference path or full reference name
func IsReference(name string) bool {
	return regexp.MustCompile("^refs/(heads|tags|notes)((/[a-z0-9_-]+)+)?$").MatchString(name)
}

// isTag checks whether a reference name indicates a tag
func IsTag(name string) bool {
	return plumbing.ReferenceName(name).IsTag()
}

// isNote checks whether a reference name indicates a tag
func IsNote(name string) bool {
	return plumbing.ReferenceName(name).IsNote()
}

// MakeIssueReference creates an issue reference
func MakeIssueReference(id int) string {
	return fmt.Sprintf("refs/heads/%s/%d", IssueBranchPrefix, id)
}
