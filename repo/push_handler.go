package repo

import (
	"context"
	"fmt"
	"gitlab.com/makeos/mosdef/types/core"
	"io"
	"time"

	"github.com/pkg/errors"
	"github.com/thoas/go-funk"
	"gitlab.com/makeos/mosdef/pkgs/logger"
	"gitlab.com/makeos/mosdef/util"
)

// PushHandler provides handles all phases of a push operation
type PushHandler struct {
	op         string
	repo       core.BareRepo
	rMgr       core.RepoManager
	oldState   core.BareRepoState
	log        logger.Logger
	pushReader *PushReader
	pushNoteID string
}

// newPushHandler returns an instance of PushHandler
func newPushHandler(repo core.BareRepo, rMgr core.RepoManager) *PushHandler {
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
	if h.oldState == nil {
		h.oldState, err = h.rMgr.GetRepoState(h.repo)
		if err != nil {
			return err
		}
	}

	// Create a push reader to read, analyse and extract info.
	// Also, pass the git writer so the pack data is written to it.
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
func (h *PushHandler) HandleValidateAndRevert() (map[string]*util.TxParams, string, error) {

	// Expect old state to have been captured before the push was processed
	if h.oldState == nil {
		return nil, "", fmt.Errorf("push-handler: expected old state to have been captured")
	}

	var errs []error
	refsTxParams := map[string]*util.TxParams{}

	// Here, we need to validate the changes introduced by the push and also
	// collect the transaction information pushed alongside the references
	for _, ref := range h.pushReader.references.names() {
		txParams, pushErrs := h.handleReference(ref)
		if len(pushErrs) == 0 {
			refsTxParams[ref] = txParams
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
	var pkID = funk.Values(refsTxParams).([]*util.TxParams)[0].PubKeyID
	if len(refsTxParams) > 1 {
		for _, txParams := range refsTxParams {
			if pkID != txParams.PubKeyID {
				errs = append(errs, fmt.Errorf("rejected because the pushed references "+
					"were signed with multiple pgp keys"))
				h.rMgr.GetPruner().Schedule(h.repo.GetName())
				break
			}
		}
	}

	if len(errs) != 0 {
		return nil, "", errs[0]
	}

	return refsTxParams, pkID, nil
}

// HandleUpdate is called after the pushed data have been analysed and
// processed by git-receive-pack. Here, we attempt to determine what changed,
// validate the pushed objects, construct a push transaction and broadcast to
// the rest of the network
func (h *PushHandler) HandleUpdate() error {

	refsTxParams, pkID, err := h.HandleValidateAndRevert()
	if err != nil {
		return err
	}

	// At this point, there are no errors. We need to construct a PushNote
	pushNote, err := h.createPushNote(pkID, refsTxParams)
	if err != nil {
		return err
	}

	h.pushNoteID = pushNote.ID().String()

	// Add the push transaction to the push pool. If an error is returned
	// schedule the repository for pruning
	if err := h.rMgr.GetPushPool().Add(pushNote); err != nil {
		h.rMgr.GetPruner().Schedule(h.repo.GetName())
		return err
	}

	// Announce the pushed objects
	for _, obj := range h.pushReader.objects {
		h.announceObject(obj.Hash.String())
	}

	// Broadcast the push note
	h.rMgr.BroadcastPushObjects(pushNote)

	return nil
}

func (h *PushHandler) createPushNote(
	pkID string,
	refsTxParams map[string]*util.TxParams) (*core.PushNote, error) {

	var pushNote = &core.PushNote{
		TargetRepo:    h.repo,
		RepoName:      h.repo.GetName(),
		PusherKeyID:   util.MustDecodeRSAPubKeyID(pkID),
		PusherAddress: h.rMgr.GetLogic().GPGPubKeyKeeper().GetGPGPubKey(pkID).Address,
		Timestamp:     time.Now().Unix(),
		References:    core.PushedReferences([]*core.PushedReference{}),
		NodePubKey:    h.rMgr.GetPrivateValidatorKey().PubKey().MustBytes32(),
		Fee:           util.ZeroString,
	}

	// Get the total size of the pushed objects
	var err error
	pushNote.Size, err = getObjectsSize(h.repo, funk.Keys(h.pushReader.objectsRefs).([]string))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get pushed objects size")
	}

	accountNonce := uint64(0)
	for _, ref := range h.pushReader.references {

		if accountNonce == 0 {
			accountNonce = refsTxParams[ref.name].Nonce
			pushNote.AccountNonce = accountNonce
		} else if accountNonce != refsTxParams[ref.name].Nonce {
			return nil, fmt.Errorf("varying account nonce in references txparams are not allowed")
		}

		accountNonce = refsTxParams[ref.name].Nonce
		fee := pushNote.Fee.Decimal().Add(refsTxParams[ref.name].Fee.Decimal()).String()
		pushNote.Fee = util.String(fee)
		pushedRef := &core.PushedReference{
			Name:            ref.name,
			OldHash:         ref.oldHash,
			NewHash:         ref.newHash,
			Nonce:           h.repo.State().References.Get(ref.name).Nonce + 1,
			Objects:         h.pushReader.objectsRefs.getObjectsOf(ref.name),
			Delete:          refsTxParams[ref.name].DeleteRef,
			MergeProposalID: refsTxParams[ref.name].MergeProposalID,
		}

		pushNote.References = append(pushNote.References, pushedRef)
	}

	// Sign the push transaction
	pushNote.NodeSig, err = h.rMgr.GetPrivateValidatorKey().PrivKey().Sign(pushNote.Bytes())
	if err != nil {
		return nil, errors.Wrap(err, "failed to sign push note")
	}

	return pushNote, nil
}

// announceObject announces a packed object to DHT peers
func (h *PushHandler) announceObject(objHash string) error {
	dhtKey := MakeRepoObjectDHTKey(h.repo.GetName(), objHash)
	ctx, c := context.WithTimeout(context.Background(), 60*time.Second)
	defer c()
	if err := h.rMgr.GetDHT().Announce(ctx, []byte(dhtKey)); err != nil {
		h.log.Warn("unable to announce git object", "Err", err)
		return err
	}
	return nil
}

// handleReference handles push updates to references.
// The goal of this function is to:
// - Determine what changed as a result of the push.
// - Validate the pushed references transaction information & signature.
// - Revert the changes and delete the new objects if validation failed.
func (h *PushHandler) handleReference(ref string) (*util.TxParams, []error) {

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
	var change *core.ItemChange
	if len(changes.References.Changes) > 0 {
		change = changes.References.Changes[0]
	}

	// Here, we need to validate the change
	txParams, err := validateChange(h.repo, change, h.rMgr.GetPGPPubKeyGetter())
	if err != nil {
		errs = append(errs, errors.Wrap(err, fmt.Sprintf("validation error (%s)", ref)))
	}

	// So, reference update is valid, next we need to ensure the updates
	// is compliant with the target merge proposal, if a merge proposal id is specified
	if err == nil && txParams.MergeProposalID != "" {
		if err := checkMergeCompliance(
			h.repo,
			change,
			oldRef,
			txParams.MergeProposalID,
			txParams.PubKeyID,
			h.rMgr.GetLogic()); err != nil {
			errs = append(errs, errors.Wrap(err, fmt.Sprintf("validation error (%s)", ref)))
		}
	}

	// As with all push operations, we must revert the changes made to the
	// repository since we do not consider them final. Here we attempt to revert
	// the repository to the old reference state. We passed the changes as an
	// option so Revert doesn't recompute it
	changes, err = revert(h.repo, oldRefState, matchOpt(ref), changesOpt(changes))
	if err != nil {
		errs = append(errs, errors.Wrap(err, "failed to revert to pre-push state"))
	}

	// Now, we need to delete the pushed objects if an error has occurred.
	// We schedule the repository for pruning.
	if len(errs) > 0 {
		h.rMgr.GetPruner().Schedule(h.repo.GetName())
	}

	return txParams, errs
}
