package repo

import (
	"fmt"
	"time"

	"github.com/makeos/mosdef/util"
	"github.com/pkg/errors"
	"github.com/thoas/go-funk"
)

// PushHook provides lifecycle methods that are
// called during git push operation request.
type PushHook struct {
	op       string
	repo     *Repo
	rMgr     RepositoryManager
	oldState *State
}

// newPushHook returns an instance of Hook
func newPushHook(repo *Repo, rMgr RepositoryManager) *PushHook {
	return &PushHook{
		repo: repo,
		rMgr: rMgr,
	}
}

// BeforePush is called before the push request packfile is written to the repository
func (h *PushHook) BeforePush() error {

	var err error

	// Get the repository state and record it as the old state
	h.oldState, err = h.rMgr.GetRepoState(h.repo)
	if err != nil {
		return err
	}

	return nil
}

// AfterPush is called after the pushed data have been processed by git.
// targetRefs: are git references pushed by the client
// pr: push inspector provides information about the push operation
func (h *PushHook) AfterPush(pr *PushReader) error {

	// Panic when old state was not captured
	if h.oldState == nil {
		return fmt.Errorf("hook: expected old state to have been captured")
	}

	var errs []error
	refsTxLine := map[string]*util.TxLine{}

	// Here, we need to validate the changes introduced by the push and also
	// collect the transaction information pushed alongside the references
	for _, ref := range pr.references.names() {
		txLine, pushErrs := h.onPushReference(ref, pr)
		if len(pushErrs) == 0 {
			refsTxLine[ref] = txLine
			continue
		}
		errs = append(errs, pushErrs...)
	}

	// If we got errors, return the first
	if len(errs) != 0 {
		return errs[0]
	}

	// When we have more than one pushed references, we need to ensure they both
	// were signed using same public key id, if not, we return an error and also
	// remove the pushed objects from the references and repository
	var pkID string
	if len(refsTxLine) > 1 {
		for _, txLine := range refsTxLine {
			if pkID == "" {
				pkID = txLine.PubKeyID
				continue
			}
			if pkID != txLine.PubKeyID {
				errs = append(errs, fmt.Errorf("rejected because the pushed references "+
					"were signed with multiple pgp keys"))
				errs = append(errs, removePackedObjectsFromRefs(pr.references.names(),
					h.repo, pr)...)
				break
			}
		}
	} else {
		pkID = funk.Values(refsTxLine).([]*util.TxLine)[0].PubKeyID
	}

	// If we got errors, return the first
	if len(errs) != 0 {
		return errs[0]
	}

	// At this point, there are no errors. We need to construct a PushTx
	var pushTx = &PushTx{
		RepoName:    h.repo.repoName,
		PusherKeyID: pkID,
		Timestamp:   time.Now().Unix(),
		References:  PushedReferences([]*PushedReference{}),
		Size:        getObjectsSize(h.repo, funk.Keys(pr.objectsRefs).([]string)),
		NodePubKey:  h.rMgr.GetNodeKey().PubKey().Base58(),
	}
	for _, ref := range pr.references {
		pushTx.References = append(pushTx.References, &PushedReference{
			Name:         ref.name,
			OldObjectID:  ref.oldHash,
			NewObjectID:  ref.newHash,
			Nonce:        h.repo.state.References.Get(ref.name).Nonce + 1,
			Fee:          refsTxLine[ref.name].Fee,
			Sig:          refsTxLine[ref.name].Signature,
			AccountNonce: refsTxLine[ref.name].Nonce,
			Objects:      pr.objectsRefs.getObjectsOf(ref.name),
		})
	}

	// Sign the push transaction
	var err error
	sigMsg := pushTx.Bytes()
	pushTx.NodeSig, err = h.rMgr.GetNodeKey().PrivKey().Sign(sigMsg)
	if err != nil {
		return errors.Wrap(err, "failed to sign push tx")
	}

	// Add the push transaction to the push pool. If an error is returned we
	// will attempt to remove the pushed objects from the references and node
	// and return an error.
	if err := h.rMgr.GetPushPool().Add(pushTx); err != nil {
		if errs = removePackedObjectsFromRefs(pr.references.names(), h.repo, pr); len(errs) > 0 {
			return errors.Wrap(errs[0], "failed to remove packed objects from ref")
		}
		return err
	}

	return nil
}

// onPushReference handles push updates to references.
// The goal of this function is to:
// - Determine what changed as a result of the recent push.
// - Validate the update references their current state meet protocol rules.
// - Revert the changes and delete the new objects if validation failed.
func (h *PushHook) onPushReference(ref string, pr *PushReader) (*util.TxLine, []error) {

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
		return nil, errs
	}

	// Now, compute the changes from the target reference old state to its current.
	changes := oldRefState.GetChanges(curState)
	var change *ItemChange
	if len(changes.References.Changes) > 0 {
		change = changes.References.Changes[0]
	}

	// Here, we need to validate the change
	txLine, err := validateChange(h.repo, change, h.rMgr.GetPGPPubKeyGetter())
	if err != nil {
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
		errs = append(errs, removePackedObjectsFromRef(ref, h.repo, pr)...)
	}

	return txLine, errs
}

// removePackedObjectsFromRef deletes pushed objects contained in the given ref;
// Objects that are also linked to other references are not deleted.
// ref: The target ref whose contained object are to be deleted.
// repo: The repository where this object exist.
// pr: Push inspector object
func removePackedObjectsFromRef(ref string, repo *Repo, pr *PushReader) (errs []error) {
	for _, obj := range pr.objects {
		relatedRefs := pr.objectsRefs[obj.Hash.String()]
		if len(relatedRefs) == 1 && funk.ContainsString(relatedRefs, ref) {
			if err := repo.DeleteObject(obj.Hash); err != nil {
				errs = append(errs, err)
			}
		}
		pr.objectsRefs.removeRef(obj.Hash.String(), ref)
	}
	return
}

// removePackedObjectsFromRefs deletes pushed objects contained in the given refs;
// Objects that are also linked to other references are not deleted.
// refs: A list of refs whose contained object are to be deleted.
// repo: The repository where this object exist.
// pr: Push inspector object
func removePackedObjectsFromRefs(refs []string, repo *Repo, pr *PushReader) (errs []error) {
	for _, ref := range refs {
		for _, obj := range pr.objects {
			relatedRefs := pr.objectsRefs[obj.Hash.String()]
			if len(relatedRefs) == 1 && funk.ContainsString(relatedRefs, ref) {
				if err := repo.DeleteObject(obj.Hash); err != nil {
					errs = append(errs, err)
				}
			}
			pr.objectsRefs.removeRef(obj.Hash.String(), ref)
		}
	}
	return
}
