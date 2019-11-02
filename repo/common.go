package repo

import (
	"gopkg.in/src-d/go-git.v4"
)

// ActionType represents a repo altering action
type ActionType int

const (
	// ActionTypeHardReset represents an action to delete a commit and all its children
	ActionTypeHardReset ActionType = iota
	// ActionTypeRefDelete represents an action to delete a reference
	ActionTypeRefDelete
	// ActionTypeRefUpdate represents an action to update a reference
	ActionTypeRefUpdate
	// ActionTypeAnnTagDelete represents an action to delete an annotated tag
	ActionTypeAnnTagDelete
)

// Repo represents a git repository
type Repo struct {
	*git.Repository
	*GitOps
	Path string
}
