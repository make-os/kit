package repo

import (
	"fmt"
	"strings"

	"github.com/asaskevich/govalidator"
	"github.com/k0kubun/pp"
	"github.com/pkg/errors"
)

// Revert reverts the repository from its current state to the previous state.
func (rb *Manager) revert(repo *Repo, prevState *State) error {

	// Get the changes from prev state to the current
	changes := prevState.GetChanges(rb.getRepoState(repo))
	pp.Println(changes)
	var actions []*Action

	// Determine actions required to revert references to previous state
	for _, ref := range changes.References.Changes {
		oldStateRef := findRefInCol(ref.Item.GetName(), prevState.Refs)
		refname := ref.Item.GetName()

		// For branch references
		if isBranch(refname) {
			acts, err := getBranchRevertActions(repo, ref, oldStateRef)
			if err != nil {
				return err
			}
			actions = append(actions, acts...)
		}

		// For tags
		if isTag(refname) {
			acts, err := getTagRevertActions(repo, ref, oldStateRef)
			if err != nil {
				return err
			}
			actions = append(actions, acts...)
		}

	}

	// Execute actions
	if err := execActions(repo, actions); err != nil {
		return errors.Wrap(err, "exec failed")
	}

	return nil
}

// execActions executes the given actions against the repository
func execActions(repo *Repo, actions []*Action) (err error) {
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
		}
	}
	return
}

// findRefInCol finds a reference in a reference collection
func findRefInCol(refname string, refCol *ObjCol) (found Item) {
	refCol.ForEach(func(i Item) bool {
		if i.GetName() == refname {
			found = i
			return true
		}
		return false
	})
	return
}

// isBranch checks whether a reference name indicates a branch
func isBranch(refname string) bool {
	return govalidator.Matches(refname, "^refs/heads/.*$")
}

// isTag checks whether a reference name indicates a tag
func isTag(refname string) bool {
	return govalidator.Matches(refname, "^refs/tags/.*$")
}

// Action describes a repo action to be effected on a repo object
type Action struct {
	Data     string
	DataItem Item
	Type     ActionType
}

// getBranchRevertActions returns a set of actions to be executed against
// repo in other to bring its branch state to a specific target.
//
// repo: The repo whose current state is to be reverted.
// branchRef: The reference that was changed in the repo.
// oldRef: The version of ref that was in the old state (this one we want to
// revert to)
func getBranchRevertActions(repo *Repo, branchRef *ItemChange, oldRef Item) ([]*Action, error) {

	var actions []*Action
	refname := branchRef.Item.GetName()

	switch branchRef.Action {
	case ColChangeTypeUpdate:
		actions = append(actions, &Action{Type: ActionTypeTagRefUpdate, DataItem: oldRef})
	case ColChangeTypeNew:
		actions = append(actions, &Action{Type: ActionTypeBranchDelete, Data: refname})
	case ColChangeTypeRemove:
		actions = append(actions, &Action{Type: ActionTypeBranchUpdate, DataItem: branchRef.Item})
	default:
		return nil, fmt.Errorf("unknown change type")
	}

	return actions, nil
}

// getTagRevertActions returns a set of actions to be executed against
// repo in other to bring its tag state to a specific target.
//
// repo: The repo whose current state is to be reverted.
// tagRef: The reference that was changed in the repo.
// oldRef: The version of ref that was in the old state (this one we want to
// revert to)
func getTagRevertActions(repo *Repo, tagRef *ItemChange, oldRef Item) ([]*Action, error) {

	var actions []*Action
	tagname := tagRef.Item.GetName()
	shortTagName := strings.ReplaceAll(tagname, "refs/tags/", "")

	switch tagRef.Action {
	case ColChangeTypeNew:
		actions = append(actions, &Action{Type: ActionTypeTagDelete, Data: shortTagName})
	case ColChangeTypeUpdate:
		actions = append(actions, &Action{Type: ActionTypeTagRefUpdate, DataItem: oldRef})
	case ColChangeTypeRemove:
		actions = append(actions, &Action{Type: ActionTypeTagRefUpdate, DataItem: tagRef.Item})
	default:
		return nil, fmt.Errorf("unknown change type")
	}

	return actions, nil
}
