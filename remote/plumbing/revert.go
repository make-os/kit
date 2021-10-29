package plumbing

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
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
	// ActionTypeNoteDelete represents an action to delete a note
	ActionTypeNoteDelete
	// ActionTypeNoteUpdate represents an action to update a note reference
	ActionTypeNoteUpdate
)

type RevertFunc func(repo LocalRepo, prevState RepoRefsState, options ...KVOption) (*Changes, error)

// Revert reverts the repository from its current state to the previous state.
// options: Additional options. prefixOpt forces the operation to ignore
// any reference that does not contain the provided prefix.
func Revert(
	repo LocalRepo,
	prevState RepoRefsState,
	options ...KVOption) (*Changes, error) {

	var actions []*Action
	changes := GetKVOpt("changes", options)

	// Get the changes from previous state to the current
	if changes == nil {
		changes = prevState.GetChanges(GetRepoState(repo, options...))
	}

	// Determine actions required to Revert references to previous state
	for _, ref := range changes.(*Changes).References.Changes {
		oldStateRef := findRefInCol(ref.Item.GetName(), prevState.GetReferences())
		refname := ref.Item.GetName()

		// For branch references
		if IsBranch(refname) {
			acts, err := GetBranchRevertActions(ref, oldStateRef)
			if err != nil {
				return nil, err
			}
			actions = append(actions, acts...)
		}

		// For tags
		if IsTag(refname) {
			acts, err := GetTagRevertActions(ref, oldStateRef)
			if err != nil {
				return nil, err
			}
			actions = append(actions, acts...)
		}

		// For notes
		if IsNote(refname) {
			acts, err := GetNoteRevertActions(ref, oldStateRef)
			if err != nil {
				return nil, err
			}
			actions = append(actions, acts...)
		}
	}

	// Execute all actions to Revert the state of the repository.
	if err := execActions(repo, actions); err != nil {
		return nil, errors.Wrap(err, "exec failed")
	}

	return changes.(*Changes), nil
}

// execActions executes the given actions against the repository
// CONTRACT: Git objects of older state are not altered/removed, they remain as
// loose objects till garbage collection is performed.
func execActions(repo LocalRepo, actions []*Action) (err error) {
	for _, action := range actions {
		switch action.Type {
		case ActionTypeBranchDelete:
			err = repo.RefDelete(action.Data)
		case ActionTypeBranchUpdate:
			err = repo.RefUpdate(action.DataItem.GetName(), action.DataItem.GetData())
		case ActionTypeTagDelete:
			err = repo.TagDelete(action.Data)
		case ActionTypeTagRefUpdate:
			err = repo.RefUpdate(action.DataItem.GetName(), action.DataItem.GetData())
		case ActionTypeNoteDelete:
			err = repo.RefDelete(action.Data)
		case ActionTypeNoteUpdate:
			err = repo.RefUpdate(action.DataItem.GetName(), action.DataItem.GetData())
		}
	}
	return
}

// findRefInCol finds a reference in a reference collection
func findRefInCol(refname string, refCol Items) (found Item) {
	refCol.ForEach(func(i Item) bool {
		if i.GetName() == refname {
			found = i
			return true
		}
		return false
	})
	return
}

// Action describes a repo action to be effected on a repo object
type Action struct {
	Data     string
	DataItem Item
	Type     ActionType
}

// GetBranchRevertActions returns a set of actions to be executed against
// repo in other to bring its branch state to a specific target.
//
// branchRef: The reference that was changed in the repo.
// oldRef: The version of ref that was in the old state (this one we want to
// Revert to)
func GetBranchRevertActions(branchRef *ItemChange, oldRef Item) ([]*Action, error) {

	var actions []*Action
	refname := branchRef.Item.GetName()

	switch branchRef.Action {
	case ChangeTypeUpdate:
		actions = append(actions, &Action{Type: ActionTypeTagRefUpdate, DataItem: oldRef})
	case ChangeTypeNew:
		actions = append(actions, &Action{Type: ActionTypeBranchDelete, Data: refname})
	case ChangeTypeRemove:
		actions = append(actions, &Action{Type: ActionTypeBranchUpdate, DataItem: branchRef.Item})
	default:
		return nil, fmt.Errorf("unknown change type")
	}

	return actions, nil
}

// GetTagRevertActions returns a set of actions to be executed against
// repo in other to bring its tag state to a specific target.
//
// tagRef: The reference that was changed in the repo.
// oldRef: The version of ref that was in the old state (this one we want to
// Revert to)
func GetTagRevertActions(tagRef *ItemChange, oldRef Item) ([]*Action, error) {

	var actions []*Action
	tagname := tagRef.Item.GetName()

	switch tagRef.Action {
	case ChangeTypeNew:
		shortTagName := strings.ReplaceAll(tagname, "refs/tags/", "")
		actions = append(actions, &Action{Type: ActionTypeTagDelete, Data: shortTagName})
	case ChangeTypeUpdate:
		actions = append(actions, &Action{Type: ActionTypeTagRefUpdate, DataItem: oldRef})
	case ChangeTypeRemove:
		actions = append(actions, &Action{Type: ActionTypeTagRefUpdate, DataItem: tagRef.Item})
	default:
		return nil, fmt.Errorf("unknown change type")
	}

	return actions, nil
}

// GetNoteRevertActions returns actions that represent instruction on how to
// Revert a repo to a previous state
//
// noteRef: The note reference that was changed in the repo.
// oldRef: The version of ref that was in the old state (this one we want to
// Revert to)
func GetNoteRevertActions(noteRef *ItemChange, oldRef Item) ([]*Action, error) {

	var actions []*Action
	tagname := noteRef.Item.GetName()

	switch noteRef.Action {
	case ChangeTypeNew:
		actions = append(actions, &Action{Type: ActionTypeNoteDelete, Data: tagname})
	case ChangeTypeUpdate:
		actions = append(actions, &Action{Type: ActionTypeNoteUpdate, DataItem: oldRef})
	case ChangeTypeRemove:
		actions = append(actions, &Action{Type: ActionTypeNoteUpdate, DataItem: noteRef.Item})
	default:
		return nil, fmt.Errorf("unknown change type")
	}

	return actions, nil
}
