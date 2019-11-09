package repo

import (
	"fmt"

	"gopkg.in/src-d/go-git.v4/plumbing/format/pktline"
)

// Hook provides lifecycle methods that are
// called during git service execution.
type Hook struct {
	op       string
	repo     *Repo
	stateOp  StateOperator
	oldState *State
}

// NewHook returns an instance of Hook
func NewHook(op string, repo *Repo, stateOp StateOperator) *Hook {
	return &Hook{op: op, repo: repo, stateOp: stateOp}
}

// BeforeInput is called before the git pack data is sent to the
// git-receive-pack or git-upload-pack input stream
func (h *Hook) BeforeInput() error {

	var err error
	switch h.op {
	case ServiceReceivePack:
		goto push
	default:
		return fmt.Errorf("unknown operation")
	}

push:
	// Get the repository state and record it as the old state
	h.oldState, err = h.stateOp.GetRepoState(h.repo.path)
	if err != nil {
		return err
	}

	return nil
}

// BeforeOutput is called before output from git-receive-pack
// or git-upload-pack are sent to the client
// enc: The encode used to write the output to the client
func (h *Hook) BeforeOutput(enc *pktline.Encoder) error {
	return nil
}

// AfterOutput is called after the output from git-receive-pack or
// git-upload-pack has been sent to the client
func (h *Hook) AfterOutput() {

	switch h.op {
	case ServiceReceivePack:
		goto revertOldState
	default:
		return
	}

revertOldState:

	// Panic when old state was not captured
	if h.oldState == nil {
		panic("expected old state to have been captured")
	}

	// Revert old state
	if err := h.stateOp.Revert(h.repo.path, h.oldState); err != nil {
		panic(err)
	}
}
