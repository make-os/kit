package repo

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/thoas/go-funk"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp"

	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/pkgs/logger"
	"gitlab.com/makeos/mosdef/util"
)

type refHandler func(ref string) []error
type authorizationHandler func(ur *packp.ReferenceUpdateRequest) error

// PushHandler provides handles all phases of a push operation
type PushHandler struct {
	log                  logger.Logger
	op                   string                   // The current git operation
	repo                 core.BareRepo            // The target repository
	mgr                  core.RepoManager         // The repository manager
	oldState             core.BareRepoState       // The old state of the repo before the current push was written
	pushReader           *PushReader              // The push reader for reading pushed git objects
	noteID               string                   // The push note unique ID
	referenceHandler     refHandler               // Pushed reference handler function
	changeValidator      changeValidator          // Repository state change validator
	reverter             reverter                 // Repository state reverser function
	mergeChecker         mergeComplianceChecker   // Merge request checker function
	polEnforcer          policyEnforcer           // Authorization policy enforcer function for the repository
	txDetails            types.ReferenceTxDetails // Map of references to their transaction details
	authorizationHandler authorizationHandler     // Authorization handler function
	policyChecker        policyChecker            // Policy checker function
}

// pushHandlerCreator describes a function for creating a push handler
type pushHandlerCreator func(
	targetRepo core.BareRepo,
	txDetails []*types.TxDetail,
	enforcer policyEnforcer) *PushHandler

// newPushHandler returns an instance of PushHandler
func newPushHandler(
	repo core.BareRepo,
	txDetails []*types.TxDetail,
	polEnforcer policyEnforcer,
	rMgr core.RepoManager) *PushHandler {

	h := &PushHandler{
		repo:            repo,
		mgr:             rMgr,
		log:             rMgr.Log().Module("push-handler"),
		polEnforcer:     polEnforcer,
		pushReader:      &PushReader{},
		txDetails:       types.SliceOfTxDetailToReferenceTxDetails(txDetails),
		changeValidator: validateChange,
		reverter:        revert,
		mergeChecker:    checkMergeCompliance,
		policyChecker:   checkPolicy,
	}
	h.referenceHandler = h.handleReference
	h.authorizationHandler = h.HandleAuthorization
	return h
}

// createPushHandler creates an instance of PushHandler
func (m *Manager) createPushHandler(
	targetRepo core.BareRepo,
	txDetails []*types.TxDetail,
	enforcer policyEnforcer) *PushHandler {
	return newPushHandler(targetRepo, txDetails, enforcer, m)
}

// HandleStream processes git push request stream
func (h *PushHandler) HandleStream(packfile io.Reader, gitReceivePack io.WriteCloser) error {

	var err error

	// Get the repository state and record it as the old state
	if h.oldState == nil {
		h.oldState, err = h.mgr.GetRepoState(h.repo)
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

	// Perform actions that should happen before git consumes the stream.
	h.pushReader.OnReferenceUpdateRequestRead(func(ur *packp.ReferenceUpdateRequest) error {
		return errors.Wrap(h.authorizationHandler(ur), "authorization")
	})

	// Write the packfile to the push reader and read it
	io.Copy(h.pushReader, packfile)
	if err = h.pushReader.Read(); err != nil {
		return err
	}

	return nil
}

// checkForReferencesTxDetail checks that each pushed reference has a transaction detail
func (h *PushHandler) checkForReferencesTxDetail() error {
	for _, ref := range h.pushReader.references.names() {
		if h.txDetails.Get(ref) == nil {
			return fmt.Errorf("reference (%s) has no transaction information", ref)
		}
	}
	return nil
}

// HandleAuthorization performs authorization checks
func (h *PushHandler) HandleAuthorization(ur *packp.ReferenceUpdateRequest) error {

	// Make sure every pushed references has a tx detail
	if err := h.checkForReferencesTxDetail(); err != nil {
		return err
	}

	// For each commands, check whether the pusher is authorized to perform the actions
	for _, cmd := range ur.Commands {
		pushKeyID := h.txDetails.GetPushKeyID()

		// For delete command, check if their is a policy allowing the pusher to do this.
		if cmd.New.IsZero() {
			if err := h.policyChecker(h.polEnforcer, pushKeyID, cmd.Name.String(), "delete"); err != nil {
				return err
			}
			continue
		}

		// For write command, check if their is a policy allowing the pusher to do this.
		if err := h.policyChecker(h.polEnforcer, pushKeyID, cmd.Name.String(), "update"); err != nil {
			return err
		}
	}

	return nil
}

// HandleReferences processes all pushed references
func (h *PushHandler) HandleReferences() error {

	// Expect old state to have been captured before the push was processed
	if h.oldState == nil {
		return fmt.Errorf("expected old state to have been captured")
	}

	for _, ref := range h.pushReader.references.names() {
		refErrs := h.referenceHandler(ref)
		if len(refErrs) > 0 {
			return refErrs[0]
		}
	}

	return nil
}

// HandleUpdate is called after the pushed data have been analysed and
// processed by git-receive-pack. Here, we attempt to determine what changed,
// validate the pushed objects, construct a push transaction and broadcast to
// the rest of the network
func (h *PushHandler) HandleUpdate() error {

	// Validate and pass the references pushed
	err := h.HandleReferences()
	if err != nil {
		return err
	}

	// Construct a push note
	note, err := h.createPushNote()
	if err != nil {
		return err
	}

	// Add the push note to the push pool
	if err := h.mgr.GetPushPool().Add(note); err != nil {
		return err
	}

	// Announce the pushed objects
	for _, obj := range h.pushReader.objects {
		h.announceObject(obj.Hash.String())
	}

	// Broadcast the push note
	if err = h.mgr.BroadcastPushObjects(note); err != nil {
		h.log.Error("Failed to broadcast push note", "Err", err)
	}

	return nil
}

// createPushNote creates a note that describes a push operation.
func (h *PushHandler) createPushNote() (*core.PushNote, error) {

	var note = &core.PushNote{
		TargetRepo:      h.repo,
		PushKeyID:       util.MustDecodePushKeyID(h.txDetails.GetPushKeyID()),
		RepoName:        h.txDetails.GetRepoName(),
		Namespace:       h.txDetails.GetRepoNamespace(),
		PusherAcctNonce: h.txDetails.GetNonce(),
		PusherAddress:   h.mgr.GetLogic().PushKeyKeeper().Get(h.txDetails.GetPushKeyID()).Address,
		Timestamp:       time.Now().Unix(),
		NodePubKey:      h.mgr.GetPrivateValidatorKey().PubKey().MustBytes32(),
		References:      core.PushedReferences{},
	}

	// Add references
	for refName, ref := range h.pushReader.references {
		note.References = append(note.References, &core.PushedReference{
			Name:            refName,
			OldHash:         ref.oldHash,
			NewHash:         ref.newHash,
			Nonce:           h.repo.State().References.Get(refName).Nonce + 1,
			Objects:         h.pushReader.objectsRefs.getObjectsOf(refName),
			Fee:             h.txDetails.Get(refName).Fee,
			MergeProposalID: h.txDetails.Get(refName).MergeProposalID,
			PushSig:         h.txDetails.Get(refName).MustSignatureAsBytes(),
		})
	}

	// Calculate the size of all pushed objects
	var err error
	objs := funk.Keys(h.pushReader.objectsRefs).([]string)
	note.Size, err = getObjectsSize(h.repo, objs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get pushed objects size")
	}

	// Sign the push transaction
	note.NodeSig, err = h.mgr.GetPrivateValidatorKey().PrivKey().Sign(note.Bytes())
	if err != nil {
		return nil, errors.Wrap(err, "failed to sign push note")
	}

	// Store the push ID to the handler
	h.noteID = note.ID().String()

	return note, nil
}

// announceObject announces a packed object to DHT peers
func (h *PushHandler) announceObject(objHash string) error {
	dhtKey := MakeRepoObjectDHTKey(h.repo.GetName(), objHash)
	ctx, c := context.WithTimeout(context.Background(), 60*time.Second)
	defer c()
	if err := h.mgr.GetDHT().Announce(ctx, []byte(dhtKey)); err != nil {
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
func (h *PushHandler) handleReference(ref string) []error {

	var errs []error

	// Get the old version of the reference prior to the push
	// and create a lone state object of the old state
	oldRef := h.oldState.GetReferences().Get(ref)
	oldRefState := makeStateFromItem(oldRef)

	// Get the current state of the repository; limit the query to only the target reference
	curState, err := h.mgr.GetRepoState(h.repo, matchOpt(ref))
	if err != nil {
		errs = append(errs, errors.Wrap(err, "failed to get current state"))
		return errs
	}

	// Now, compute the changes from the target reference old state to its current.
	changes := oldRefState.GetChanges(curState.(*State))
	var change *core.ItemChange
	if len(changes.References.Changes) > 0 {
		change = changes.References.Changes[0]
	}

	// Here, we need to validate the change for non-delete request
	if !isZeroHash(h.pushReader.references[ref].newHash) {
		err = h.changeValidator(h.repo, change, h.txDetails.Get(ref), h.mgr.GetPushKeyGetter())
		if err != nil {
			errs = append(errs, errors.Wrap(err, fmt.Sprintf("validation error (%s)", ref)))
		}
	}

	// So, reference update is valid, next we need to ensure the updates
	// is compliant with the target merge proposal, if a merge proposal id is specified
	if err == nil && h.txDetails.Get(ref).MergeProposalID != "" {
		if err := h.mergeChecker(
			h.repo,
			change,
			oldRef,
			h.txDetails.Get(ref).MergeProposalID,
			h.txDetails.GetPushKeyID(),
			h.mgr.GetLogic()); err != nil {
			errs = append(errs, errors.Wrap(err, fmt.Sprintf("validation error (%s)", ref)))
		}
	}

	// As with all push operations, we must revert the changes made to the
	// repository since we do not consider them final. Here we attempt to revert
	// the repository to the old reference state. We passed the changes as an
	// option so Revert doesn't recompute it
	changes, err = h.reverter(h.repo, oldRefState, matchOpt(ref), changesOpt(changes))
	if err != nil {
		errs = append(errs, errors.Wrap(err, "failed to revert to old state"))
	}

	return errs
}
