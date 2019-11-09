package repo

import (
	"gopkg.in/src-d/go-git.v4"
)

// ActionType represents a repo altering action
type ActionType int

const (
	// ActionTypeBranchDelete represents an action to delete a branch reference
	ActionTypeBranchDelete ActionType = iota
	// ActionTypeBranchUpdate represents an action to update a branch reference
	ActionTypeBranchUpdate
	// ActionTypeTagDelete represents an action to delete an annotated tag
	ActionTypeTagDelete
	// ActionTypeTagRefUpdate represents an action to update a tag's reference hash
	ActionTypeTagRefUpdate
)

// Repo represents a git repository
type Repo struct {
	*git.Repository
	*GitOps
	Path string
}
