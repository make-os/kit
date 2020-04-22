package manager

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/thoas/go-funk"
	"gitlab.com/makeos/mosdef/repo/plumbing"
	"gitlab.com/makeos/mosdef/repo/policy"
	"gitlab.com/makeos/mosdef/repo/repo"
	"gitlab.com/makeos/mosdef/repo/validator"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp"

	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/pkgs/logger"
	"gitlab.com/makeos/mosdef/util"
)

type refHandler func(ref string, revertOnly bool) []error
type authorizationHandler func(ur *packp.ReferenceUpdateRequest) error

// PushHandler provides handles all phases of a push operation
type PushHandler struct {
	log                  logger.Logger
	op                   string                             // The current git operation
	Repo                 core.BareRepo                      // The target repository
	Mgr                  core.RepoManager                   // The repository manager
	OldState             core.BareRepoState                 // The old state of the repo before the current push was written
	PushReader           *PushReader                        // The push reader for reading pushed git objects
	NoteID               string                             // The push note unique ID
	changeValidator      validator.ChangeValidatorFunc      // Repository state change validator
	reverter             plumbing.RevertFunc                // Repository state reverser function
	mergeChecker         validator.MergeComplianceCheckFunc // Merge request checker function
	polEnforcer          policy.EnforcerFunc                // Authorization policy enforcer function for the repository
	txDetails            types.ReferenceTxDetails           // Map of references to their transaction details
	ReferenceHandler     refHandler                         // Pushed reference handler function
	AuthorizationHandler authorizationHandler               // Authorization handler function
	policyChecker        policy.PolicyChecker               // Policy checker function
}

// PushHandlerFunc describes a function for creating a push handler
type PushHandlerFunc func(
	targetRepo core.BareRepo,
	txDetails []*types.TxDetail,
	enforcer policy.EnforcerFunc) *PushHandler

// NewHandler returns an instance of PushHandler
func NewHandler(
	repo core.BareRepo,
	txDetails []*types.TxDetail,
	polEnforcer policy.EnforcerFunc,
	rMgr core.RepoManager) *PushHandler {

	h := &PushHandler{
		Repo:            repo,
		Mgr:             rMgr,
		log:             rMgr.Log().Module("push-handler"),
		polEnforcer:     polEnforcer,
		PushReader:      &PushReader{},
		txDetails:       types.SliceOfTxDetailToReferenceTxDetails(txDetails),
		changeValidator: validator.ValidateChange,
		reverter:        plumbing.Revert,
		mergeChecker:    validator.CheckMergeCompliance,
		policyChecker:   policy.CheckPolicy,
	}
	h.ReferenceHandler = h.handleReference
	h.AuthorizationHandler = h.HandleAuthorization
	return h
}

// HandleStream processes git push request stream
func (h *PushHandler) HandleStream(packfile io.Reader, gitReceivePack io.WriteCloser) error {

	var err error

	// Get the repository state and record it as the old state
	if h.OldState == nil {
		h.OldState, err = h.Mgr.GetRepoState(h.Repo)
		if err != nil {
			return err
		}
	}

	// Create a push reader to read, analyse and extract info.
	// Also, pass the git writer so the pack data is written to it.
	h.PushReader, err = newPushReader(gitReceivePack, h.Repo)
	if err != nil {
		return errors.Wrap(err, "unable to create push reader")
	}

	// Perform actions that should happen before git consumes the stream.
	h.PushReader.OnReferenceUpdateRequestRead(func(ur *packp.ReferenceUpdateRequest) error {
		return errors.Wrap(h.AuthorizationHandler(ur), "authorization")
	})

	// Write the packfile to the push reader and read it
	io.Copy(h.PushReader, packfile)
	if err = h.PushReader.Read(); err != nil {
		return err
	}

	return nil
}

// checkForReferencesTxDetail checks that each pushed reference has a transaction detail
func (h *PushHandler) checkForReferencesTxDetail() error {
	for _, ref := range h.PushReader.references.names() {
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

	// For each push details, check whether the pusher is authorized
	for _, cmd := range ur.Commands {
		detail := h.txDetails.Get(cmd.Name.String())

		// For delete command, check if there is a policy allowing the pusher to do it.
		if cmd.New.IsZero() {
			if err := h.policyChecker(h.polEnforcer, detail.PushKeyID, cmd.Name.String(),
				"delete"); err != nil {
				return err
			}
			continue
		}

		// For merge update, check if there is a policy allowing the pusher to do it.
		if detail.MergeProposalID != "" {
			if err := h.policyChecker(h.polEnforcer, detail.PushKeyID, cmd.Name.String(),
				"merge-update"); err != nil {
				return err
			}
			continue
		}

		// For merge update, check if there is a policy allowing the pusher to do it.
		if plumbing.IsIssueBranch(detail.Reference) {
			if err := h.policyChecker(h.polEnforcer, detail.PushKeyID, cmd.Name.String(),
				"issue-update"); err != nil {
				return err
			}
			continue
		}

		// For write command, check if there is a policy allowing the pusher to do it.
		if err := h.policyChecker(h.polEnforcer, detail.PushKeyID, cmd.Name.String(),
			"update"); err != nil {
			return err
		}
	}

	return nil
}

// HandleReferences processes all pushed references
func (h *PushHandler) HandleReferences() error {

	// Expect old state to have been captured before the push was processed
	if h.OldState == nil {
		return fmt.Errorf("expected old state to have been captured")
	}

	var errs = []error{}
	for _, ref := range h.PushReader.references.names() {

		// When the previous reference handling failed, the only handling operation
		// to perform on the current reference is a revert of new changes introduced
		revertOnly := false
		if len(errs) > 0 {
			revertOnly = true
		}

		errs = append(errs, h.ReferenceHandler(ref, revertOnly)...)
	}

	if len(errs) > 0 {
		return errs[0]
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
	if err := h.Mgr.GetPushPool().Add(note); err != nil {
		return err
	}

	// Announce the pushed objects
	for _, obj := range h.PushReader.objects {
		h.announceObject(obj.Hash.String())
	}

	// Broadcast the push note
	if err = h.Mgr.BroadcastPushObjects(note); err != nil {
		h.log.Error("Failed to broadcast push note", "Err", err)
	}

	return nil
}

// createPushNote creates a note that describes a push operation.
func (h *PushHandler) createPushNote() (*core.PushNote, error) {

	var note = &core.PushNote{
		TargetRepo:      h.Repo,
		PushKeyID:       util.MustDecodePushKeyID(h.txDetails.GetPushKeyID()),
		RepoName:        h.txDetails.GetRepoName(),
		Namespace:       h.txDetails.GetRepoNamespace(),
		PusherAcctNonce: h.txDetails.GetNonce(),
		PusherAddress:   h.Mgr.GetLogic().PushKeyKeeper().Get(h.txDetails.GetPushKeyID()).Address,
		Timestamp:       time.Now().Unix(),
		NodePubKey:      h.Mgr.GetPrivateValidatorKey().PubKey().MustBytes32(),
		References:      core.PushedReferences{},
	}

	// Add references
	for refName, ref := range h.PushReader.references {
		note.References = append(note.References, &core.PushedReference{
			Name:            refName,
			OldHash:         ref.oldHash,
			NewHash:         ref.newHash,
			Nonce:           h.Repo.GetState().References.Get(refName).Nonce + 1,
			Objects:         h.PushReader.objectsRefs.getObjectsOf(refName),
			Fee:             h.txDetails.Get(refName).Fee,
			MergeProposalID: h.txDetails.Get(refName).MergeProposalID,
			PushSig:         h.txDetails.Get(refName).MustSignatureAsBytes(),
		})
	}

	// Calculate the size of all pushed objects
	var err error
	objs := funk.Keys(h.PushReader.objectsRefs).([]string)
	note.Size, err = repo.GetObjectsSize(h.Repo, objs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get pushed objects size")
	}

	// Sign the push transaction
	note.NodeSig, err = h.Mgr.GetPrivateValidatorKey().PrivKey().Sign(note.Bytes())
	if err != nil {
		return nil, errors.Wrap(err, "failed to sign push note")
	}

	// Store the push ID to the handler
	h.NoteID = note.ID().String()

	return note, nil
}

// announceObject announces a packed object to DHT peers
func (h *PushHandler) announceObject(objHash string) error {
	dhtKey := plumbing.MakeRepoObjectDHTKey(h.Repo.GetName(), objHash)
	ctx, c := context.WithTimeout(context.Background(), 60*time.Second)
	defer c()
	if err := h.Mgr.GetDHT().Announce(ctx, []byte(dhtKey)); err != nil {
		h.log.Warn("unable to announce git object", "Err", err)
		return err
	}
	return nil
}

// handleReference handles reference update validation and reversion.
// When revertOnly is true, only reversion operation is performed.
func (h *PushHandler) handleReference(ref string, revertOnly bool) []error {

	var errs []error

	// Get the old version of the reference prior to the push
	// and create a lone state object of the old state
	oldRef := h.OldState.GetReferences().Get(ref)
	oldRefState := plumbing.MakeStateFromItem(oldRef)

	// Get the current state of the repository; limit the query to only the target reference
	curState, err := h.Mgr.GetRepoState(h.Repo, plumbing.MatchOpt(ref))
	if err != nil {
		errs = append(errs, errors.Wrap(err, "failed to get current state"))
		return errs
	}

	// Now, compute the changes from the target reference old state to its current.
	changes := oldRefState.GetChanges(curState.(*plumbing.State))
	var change *core.ItemChange
	if len(changes.References.Changes) > 0 {
		change = changes.References.Changes[0]
	}

	if revertOnly {
		goto revert
	}

	// Here, we need to validate the change for non-delete request
	if !plumbing.IsZeroHash(h.PushReader.references[ref].newHash) {
		oldHash := h.PushReader.references[ref].oldHash
		err = h.changeValidator(h.Repo, oldHash, change, h.txDetails.Get(ref), h.Mgr.GetPushKeyGetter())
		if err != nil {
			errs = append(errs, errors.Wrap(err, fmt.Sprintf("validation error (%s)", ref)))
		}
	}

	// So, reference update is valid, next we need to ensure the updates
	// is compliant with the target merge proposal, if a merge proposal id is specified
	if err == nil && h.txDetails.Get(ref).MergeProposalID != "" {
		if err := h.mergeChecker(
			h.Repo,
			change,
			oldRef,
			h.txDetails.Get(ref).MergeProposalID,
			h.txDetails.GetPushKeyID(),
			h.Mgr.GetLogic()); err != nil {
			errs = append(errs, errors.Wrap(err, fmt.Sprintf("validation error (%s)", ref)))
		}
	}

revert:
	// As with all push operations, we must revert the changes made to the
	// repository since we do not consider them final. Here we attempt to revert
	// the repository to the old reference state. We passed the changes as an
	// option so Revert doesn't recompute it
	changes, err = h.reverter(h.Repo, oldRefState, plumbing.MatchOpt(ref), plumbing.ChangesOpt(changes))
	if err != nil {
		errs = append(errs, errors.Wrap(err, "failed to revert to old state"))
	}

	return errs
}
