package plumbing

import (
	"regexp"

	"gopkg.in/src-d/go-git.v4/plumbing"
)

// isBranch checks whether a reference name indicates a branch
func IsBranch(name string) bool {
	return plumbing.ReferenceName(name).IsBranch()
}

// isIssueBranch checks whether a branch is an issue branch
func IsIssueBranch(name string) bool {
	return regexp.MustCompile("^refs/heads/issues/.*").MatchString(name)
}

// isReference checks the given name is a reference path or full reference name
func IsReference(name string) bool {
	m, _ := regexp.MatchString("^refs/(heads|tags|notes)((/[a-z0-9_-]+)+)?$", name)
	return m
}

// isTag checks whether a reference name indicates a tag
func IsTag(name string) bool {
	return plumbing.ReferenceName(name).IsTag()
}

// isNote checks whether a reference name indicates a tag
func IsNote(name string) bool {
	return plumbing.ReferenceName(name).IsNote()
}
