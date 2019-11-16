package repo

import (
	"fmt"

	"github.com/pkg/errors"
)

// Hook provides lifecycle methods that are
// called during git service execution.
type Hook struct {
	op       string
	repo     *Repo
	rMgr     ReposManager
	oldState *State
}

// NewHook returns an instance of Hook
func NewHook(
	op string,
	repo *Repo,
	stateOp ReposManager) *Hook {
	return &Hook{
		op:   op,
		repo: repo,
		rMgr: stateOp,
	}
}

// BeforeInput is called before the git pack data is sent to the
// git-receive-pack or git-upload-pack input stream
func (h *Hook) BeforeInput() error {

	var err error
	switch h.op {
	case ServiceReceivePack:
		goto push
	default:
		return nil
	}

push:
	// Get the repository state and record it as the old state
	h.oldState, err = h.rMgr.GetRepoState(h.repo.path)
	if err != nil {
		return err
	}

	return nil
}

// BeforeOutput is called before output from git-receive-pack
// or git-upload-pack are sent to the client
// reqRefs: are git references pushed by the client
func (h *Hook) BeforeOutput(reqRefs []string) error {

	switch h.op {
	case ServiceReceivePack:
		goto revert
	default:
		return nil
	}

revert:
	// Panic when old state was not captured
	if h.oldState == nil {
		return fmt.Errorf("hook: expected old state to have been captured")
	}

	var errs []error

	// We should only revert the references that were pushed.
	// Process each references separately
	for _, ref := range reqRefs {

		// Find the old version of the reference in the captured old state.
		oldRef := h.oldState.References.Get(ref)

		// Create a new state specifically for the old reference.
		oldRefState := StateFromItem(oldRef)

		// Get the new changes to the repo
		curState, err := h.rMgr.GetRepoState(h.repo.path, prefixOpt(ref))
		if err != nil {
			errs = append(errs, errors.Wrap(err, "failed to get current state"))
			continue
		}

		// Get the changes from previous state to the current
		changes := oldRefState.GetChanges(curState)

		// For each reference change, we need to verify the commit signature
		// and validate the transaction line
		for _, change := range changes.References.Changes {
			if err := validateChange(h.repo, change, h.rMgr.GetPGPPubKeyGetter()); err != nil {
				errs = append(errs, errors.Wrap(err, fmt.Sprintf("validation error (%s)", ref)))
				continue
			}
		}

		// Attempt to revert the repository to the old reference state.
		// We passed in the changes as an option so Revert doesn't recompute it
		changes, err = h.rMgr.Revert(h.repo.path, oldRefState, prefixOpt(ref), changesOpt(changes))
		if err != nil {
			return errors.Wrap(err, "failed to revert to pre-push state")
		}

		// Calculate the size of the new updates
		// getSizeOfChanges(oldState, changes)
	}

	if len(errs) != 0 {
		return errs[0]
	}

	return nil
}

// AfterOutput is called after the output from git-receive-pack or
// git-upload-pack has been sent to the client
func (h *Hook) AfterOutput() error {
	return nil
}
