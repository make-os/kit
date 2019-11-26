package repo

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/thoas/go-funk"
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

// BeforePush is called before the push request packfile is written to the repository
func (h *Hook) BeforePush() error {

	var err error
	switch h.op {
	case ServiceReceivePack:
		goto push
	default:
		return nil
	}

push:
	// Get the repository state and record it as the old state
	h.oldState, err = h.rMgr.GetRepoState(h.repo)
	if err != nil {
		return err
	}

	return nil
}

// AfterPush is called after the pushed data have been processed by git.
// targetRefs: are git references pushed by the client
// pi: provides information about the recent push operation
func (h *Hook) AfterPush(pi *PushInspector) error {

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

	// pp.Println(pi.objectRefMap)

	// Process each pushed references
	for _, ref := range pi.references.names() {
		errs = append(errs, h.onPushReference(ref, pi)...)
	}

	// pp.Println(pi.objectRefMap)

	// If we got errors, return the first
	if len(errs) != 0 {
		return errs[0]
	}

	return nil
}

// onPushReference handles push updates to references.
// The goal of this function is to:
// - Determine what changed as a result of the recent push.
// - Validate the update references their current state meet protocol rules.
// - Revert the changes and delete the new objects if validation failed.
func (h *Hook) onPushReference(ref string, pi *PushInspector) []error {

	var errs = []error{}

	// Find the old version of the reference prior to the push
	// and create a lone state object of the old state
	oldRef := h.oldState.References.Get(ref)
	oldRefState := StateFromItem(oldRef)

	// Get the current state of the repository; limit the query to only the
	// target reference
	curState, err := h.rMgr.GetRepoState(h.repo, matchOpt(ref))
	if err != nil {
		errs = append(errs, errors.Wrap(err, "failed to get current state"))
		return errs
	}

	// Now, compute the changes from the target reference old state to its current.
	changes := oldRefState.GetChanges(curState)
	var change *ItemChange
	if len(changes.References.Changes) > 0 {
		change = changes.References.Changes[0]
	}

	// Here, we need to validate the change
	if err := validateChange(h.repo, change, h.rMgr.GetPGPPubKeyGetter()); err != nil {
		errs = append(errs, errors.Wrap(err, fmt.Sprintf("validation error (%s)", ref)))
	}

	// As with all push operations, we must revert the changes made to the
	// repository since we do not consider them final. Here we attempt to revert
	// the repository to the old reference state. We passed the changes as an
	// option so Revert doesn't recompute it
	changes, err = h.rMgr.Revert(h.repo, oldRefState, matchOpt(ref), changesOpt(changes))
	if err != nil {
		errs = append(errs, errors.Wrap(err, "failed to revert to pre-push state"))
	}

	// Now, we need to delete the pushed objects if an error has occurred.
	// We are only able to delete the object if it is related to only the
	// current ref. If it is not, we simply remove the current refer from the
	// list of related refs and let the next refs decide what to do with it.
	if len(errs) > 0 {
		for _, obj := range pi.objects {
			relatedRefs := pi.objectRefMap[obj.Hash.String()]
			if len(relatedRefs) == 1 && funk.ContainsString(relatedRefs, ref) {
				if err := h.repo.DeleteObject(obj.Hash); err != nil {
					errs = append(errs, err)
				}
			}
			pi.objectRefMap.removeRef(obj.Hash.String(), ref)
		}
	}

	return errs
}
