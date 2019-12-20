package repo

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
	"github.com/makeos/mosdef/util/logger"
	"github.com/pkg/errors"
	"github.com/thoas/go-funk"
)

// PushHandler provides handles all phases of a push operation
type PushHandler struct {
	op         string
	repo       types.BareRepo
	rMgr       types.RepoManager
	oldState   types.BareRepoState
	log        logger.Logger
	pushReader *PushReader
}

// newPushHandler returns an instance of PushHandler
func newPushHandler(repo types.BareRepo, rMgr types.RepoManager) *PushHandler {
	return &PushHandler{
		repo: repo,
		rMgr: rMgr,
		log:  rMgr.Log().Module("push-handler"),
	}
}

// HandleStream processes git push request stream
func (h *PushHandler) HandleStream(
	packfile io.Reader,
	gitReceivePack io.WriteCloser) error {

	var err error

	// Get the repository state and record it as the old state
	h.oldState, err = h.rMgr.GetRepoState(h.repo)
	if err != nil {
		return err
	}

	// Create a push reader to read, analyse and extract info.
	// Also, pass the git input pipe so the pack data is written to it.
	h.pushReader, err = newPushReader(gitReceivePack, h.repo)
	if err != nil {
		return errors.Wrap(err, "unable to create push reader")
	}

	// Write the packfile to the push reader and read it
	io.Copy(h.pushReader, packfile)
	if err = h.pushReader.Read(); err != nil {
		return errors.Wrap(err, "failed to read pushed update")
	}

	return nil
}

// HandleValidateAndRevert validates the transaction information and signatures
// that must accompany pushed references afterwhich the changes introduced by
// the push are reverted.
func (h *PushHandler) HandleValidateAndRevert() (map[string]*util.TxLine, string, error) {

	// Expect old state to have been captured before the push was processed
	if h.oldState == nil {
		return nil, "", fmt.Errorf("push-handler: expected old state to have been captured")
	}

	var errs []error
	refsTxLine := map[string]*util.TxLine{}

	// Here, we need to validate the changes introduced by the push and also
	// collect the transaction information pushed alongside the references
	for _, ref := range h.pushReader.references.names() {
		txLine, pushErrs := h.handleReference(ref)
		if len(pushErrs) == 0 {
			refsTxLine[ref] = txLine
			continue
		}
		errs = append(errs, pushErrs...)
	}

	// If we got errors, return the first
	if len(errs) != 0 {
		return nil, "", errs[0]
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
				errs = append(errs, removeObjsRelatedToRefs(
					h.repo,
					h.pushReader,
					h.rMgr,
					h.pushReader.references.names())...)
				break
			}
		}
	} else {
		pkID = funk.Values(refsTxLine).([]*util.TxLine)[0].PubKeyID
	}

	if len(errs) != 0 {
		return nil, "", errs[0]
	}

	return refsTxLine, pkID, nil
}

// HandleUpdate is called after the pushed data have been analysed and
// processed by git-receive-pack. Here, we attempt to determine what changed,
// validate the pushed objects, construct a push transaction and broadcast to
// the rest of the network
func (h *PushHandler) HandleUpdate() error {

	refsTxLine, pkID, err := h.HandleValidateAndRevert()
	if err != nil {
		return err
	}

	// At this point, there are no errors. We need to construct a PushTx
	pushTx, err := h.createPushTx(pkID, refsTxLine)
	if err != nil {
		return err
	}

	// Add the push transaction to the push pool. If an error is returned we
	// will attempt to remove the pushed objects from the references and node
	// and return an error.
	if err := h.rMgr.GetPushPool().Add(pushTx); err != nil {
		if errs := removeObjsRelatedToRefs(
			h.repo,
			h.pushReader,
			h.rMgr,
			h.pushReader.references.names()); len(errs) > 0 {
			return errors.Wrap(errs[0], "failed to remove packed objects from ref")
		}
		return err
	}

	// Add the objects to unfinalized cache to prevent premature deletion
	// Announce the objects to the dht
	for _, obj := range h.pushReader.objects {
		h.rMgr.AddUnfinalizedObject(h.repo.GetName(), obj.Hash.String())
		if err := h.announceObject(obj); err != nil {
			continue
		}
	}

	// Broadcast the push tx
	h.rMgr.BroadcastMsg(PushTxReactorChannel, pushTx.Bytes())

	return nil
}

func (h *PushHandler) createPushTx(pkID string, refsTxLine map[string]*util.TxLine) (*PushTx, error) {

	var err error
	var pushTx = &PushTx{
		targetRepo:  h.repo,
		RepoName:    h.repo.GetName(),
		PusherKeyID: pkID,
		Timestamp:   time.Now().Unix(),
		References:  types.PushedReferences([]*types.PushedReference{}),
		NodePubKey:  h.rMgr.GetNodeKey().PubKey().Base58(),
	}

	// Get the total size of the pushed objects
	pushTx.Size, err = getObjectsSize(h.repo, funk.Keys(h.pushReader.objectsRefs).([]string))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get pushed objects size")
	}

	for _, ref := range h.pushReader.references {
		pushTx.References = append(pushTx.References, &types.PushedReference{
			Name:         ref.name,
			OldHash:      ref.oldHash,
			NewHash:      ref.newHash,
			Nonce:        h.repo.State().References.Get(ref.name).Nonce + 1,
			Fee:          refsTxLine[ref.name].Fee,
			AccountNonce: refsTxLine[ref.name].Nonce,
			Objects:      h.pushReader.objectsRefs.getObjectsOf(ref.name),
		})
	}

	// Sign the push transaction
	pushTx.NodeSig, err = h.rMgr.GetNodeKey().PrivKey().Sign(pushTx.Bytes())
	if err != nil {
		return nil, errors.Wrap(err, "failed to sign push tx")
	}

	return pushTx, nil
}

func (h *PushHandler) announceObject(obj *packObject) error {
	dhtKey := MakeRepoObjectDHTKey(h.repo.GetName(), obj.Hash.String())
	ctx, c := context.WithTimeout(context.Background(), 60*time.Second)
	defer c()
	if err := h.rMgr.GetDHT().Annonce(ctx, []byte(dhtKey)); err != nil {
		h.log.Error("unable to announce git object", "Err", err)
		return err
	}
	return nil
}

// handleReference handles push updates to references.
// The goal of this function is to:
// - Determine what changed as a result of the push.
// - Validate the pushed references transaction information & signature.
// - Revert the changes and delete the new objects if validation failed.
func (h *PushHandler) handleReference(ref string) (*util.TxLine, []error) {

	var errs = []error{}

	// Find the old version of the reference prior to the push
	// and create a lone state object of the old state
	oldRef := h.oldState.GetReferences().Get(ref)
	oldRefState := StateFromItem(oldRef)

	// Get the current state of the repository; limit the query to only the
	// target reference
	curState, err := h.rMgr.GetRepoState(h.repo, matchOpt(ref))
	if err != nil {
		errs = append(errs, errors.Wrap(err, "failed to get current state"))
		return nil, errs
	}

	// Now, compute the changes from the target reference old state to its current.
	changes := oldRefState.GetChanges(curState.(*State))
	var change *types.ItemChange
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
	// current ref. If it is not, we simply remove the current ref from the
	// list of related refs and let the next refs decide what to do with it.
	if len(errs) > 0 {
		errs = append(errs, removeObjRelatedToOnlyRef(h.repo, h.pushReader, h.rMgr, ref)...)
	}

	return txLine, errs
}

// removeObjRelatedToOnlyRef deletes pushed objects associated with only the given ref.
// Objects that are linked to other references are not deleted.
// Also, objects that have been cached in the unfinalized object cache are not deleted.
// ref: The target ref whose contained object are to be deleted.
// repo: The repository where this object exist.
// pr: Push reader object
// unfinalized: A cache containing pushed objects that should not be deleted
func removeObjRelatedToOnlyRef(
	repo types.BareRepo,
	pr *PushReader,
	unfinalized types.UnfinalizedObjectCache,
	ref string) (errs []error) {

	for _, obj := range pr.objects {

		if unfinalized.IsUnfinalizedObject(repo.GetName(), obj.Hash.String()) {
			continue
		}

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

// removeObjsRelatedToRefs deletes pushed objects contained in the given refs;
// Objects that are also linked to other references are not deleted.
// refs: A list of refs whose contained object are to be deleted.
// repo: The repository where this object exist.
// pr: Push inspector object
func removeObjsRelatedToRefs(
	repo types.BareRepo,
	pr *PushReader,
	unfinalized types.UnfinalizedObjectCache,
	refs []string) (errs []error) {
	for _, ref := range refs {
		errs = append(errs, removeObjRelatedToOnlyRef(repo, pr, unfinalized, ref)...)
	}
	return
}
