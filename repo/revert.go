package repo

import (
	"fmt"

	"github.com/asaskevich/govalidator"
	"github.com/k0kubun/pp"
	"github.com/pkg/errors"
)

// Revert reverts the repository from its current state to the previous state.
func (rb *Manager) revert(repo *Repo, prevState *State) error {

	// Get the changes from prev state to the current
	changes := prevState.GetChanges(rb.getRepoState(repo))

	var actions []*Action

	// Determine actions required to revert references to previous state
	for _, ref := range changes.RefChange.Changes {
		oldStateRef := findRefInCol(ref.Item.GetName(), prevState.Refs)
		revertActs, err := getBranchRevertActions(repo, ref, oldStateRef)
		if err != nil {
			return err
		}
		actions = append(actions, revertActs...)
	}

	// Determine actions required to revert annotated tags to previous state
	for _, tag := range changes.AnnTagChange.Changes {
		oldStateTag := findRefInCol(tag.Item.GetName(), prevState.Tags)
		revertActs, err := getAnnotatedTagRevertActions(repo, tag, oldStateTag)
		if err != nil {
			return err
		}
		actions = append(actions, revertActs...)
	}

	// Execute actions
	if err := execActions(repo, actions); err != nil {
		return errors.Wrap(err, "exec failed")
	}

	// TODO: ensure new state matches old state

	// pp.Println("Changes:", changes)
	// pp.Println("Actions", actions)

	return nil
}

// execActions executes the given actions against the repository
func execActions(repo *Repo, actions []*Action) (err error) {
	for _, action := range actions {
		switch action.Type {
		case ActionTypeHardReset:
			err = repo.HardReset(action.Data)
		case ActionTypeRefDelete:
			err = repo.RefDelete(action.Data)
		case ActionTypeRefUpdate:
			ref := action.DataAny.(Item)
			err = repo.RefUpdate(ref.GetName(), ref.GetData())
		case ActionTypeAnnTagDelete:
			pp.Println("DELELE", action.Data)
			err = repo.TagDelete(action.Data)
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

// isBranch checks whether a reference name belong to a branch
func isBranch(refname string) bool {
	return govalidator.Matches(refname, "^refs/heads/.*$")
}

// Action describes a repo action to be effected on a repo object
type Action struct {
	Data    string
	DataAny interface{}
	Type    ActionType
}

// getBranchRevertActions returns a set of actions to be executed against
// repo in other to bring its branch state to a specific target
// repo: The repo whose current state is to be reverted.
// ref: The reference that was changed in the repo.
// oldRef: The version of ref that was in the old state (this one we want to
// revert to)
func getBranchRevertActions(repo *Repo, ref *ItemChange, oldRef Item) ([]*Action, error) {

	var actions []*Action
	refname := ref.Item.GetName()

	// Do nothing if the reference type is not a branch
	if !isBranch(refname) {
		return actions, nil
	}

	switch ref.Action {
	case ColChangeTypeUpdate:
		actions = append(actions, &Action{Type: ActionTypeHardReset, Data: oldRef.GetData()})
	case ColChangeTypeNew:
		actions = append(actions, &Action{Type: ActionTypeRefDelete, Data: refname})
	case ColChangeTypeRemove:
		actions = append(actions, &Action{Type: ActionTypeRefUpdate, DataAny: ref.Item})
	default:
		return nil, fmt.Errorf("unknown change type")
	}

	return actions, nil
}

// getAnnotatedTagRevertActions returns a set of actions to be executed against
// repo in other to bring its annotated tag state to a specific target
// repo: The repo whose current state is to be reverted.
// ref: The reference that was changed in the repo.
// oldRef: The version of ref that was in the old state (this one we want to
// revert to)
func getAnnotatedTagRevertActions(repo *Repo, tag *ItemChange, oldRef Item) ([]*Action, error) {

	var actions []*Action
	tagname := tag.Item.GetName()

	switch tag.Action {
	case ColChangeTypeNew:
		actions = append(actions, &Action{Type: ActionTypeAnnTagDelete, Data: tagname})
	}

	return actions, nil
}
