package push

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/thoas/go-funk"
	"gitlab.com/makeos/mosdef/dht"
	"gitlab.com/makeos/mosdef/remote/plumbing"
	"gitlab.com/makeos/mosdef/remote/policy"
	"gitlab.com/makeos/mosdef/remote/push/types"
	"gitlab.com/makeos/mosdef/remote/repo"
	types2 "gitlab.com/makeos/mosdef/remote/types"
	"gitlab.com/makeos/mosdef/remote/validation"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/util/crypto"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp"

	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/pkgs/logger"
)

type refHandler func(ref string, revertOnly bool) []error
type authorizationHandler func(ur *packp.ReferenceUpdateRequest) error

// Handler provides handles all phases of a push operation
type Handler struct {
	log                  logger.Logger
	op                   string                              // The current git operation
	Repo                 types2.LocalRepo                    // The target repository
	Server               core.RemoteServer                   // The repository remote server
	OldState             types2.BareRepoState                // The old state of the repo before the current push was written
	PushReader           *PushReader                         // The push reader for reading pushed git objects
	NoteID               string                              // The push note unique ID
	ChangeValidator      validation.ChangeValidatorFunc      // Repository state change validator
	Reverter             plumbing.RevertFunc                 // Repository state reverser function
	MergeChecker         validation.MergeComplianceCheckFunc // Merge request checker function
	polEnforcer          policy.EnforcerFunc                 // Authorization policy enforcer function for the repository
	TxDetails            types2.ReferenceTxDetails           // Map of references to their transaction details
	ReferenceHandler     refHandler                          // Pushed reference handler function
	AuthorizationHandler authorizationHandler                // Authorization handler function
	PolicyChecker        policy.PolicyChecker                // Policy checker function
}

// PushHandlerFunc describes a function for creating a push handler
type PushHandlerFunc func(
	targetRepo types2.LocalRepo,
	txDetails []*types2.TxDetail,
	enforcer policy.EnforcerFunc) *Handler

// NewHandler returns an instance of Handler
func NewHandler(
	repo types2.LocalRepo,
	txDetails []*types2.TxDetail,
	polEnforcer policy.EnforcerFunc,
	rMgr core.RemoteServer) *Handler {

	h := &Handler{
		Repo:            repo,
		Server:          rMgr,
		log:             rMgr.Log().Module("push-handler"),
		polEnforcer:     polEnforcer,
		PushReader:      &PushReader{},
		TxDetails:       types2.ToReferenceTxDetails(txDetails),
		ChangeValidator: validation.ValidateChange,
		Reverter:        plumbing.Revert,
		MergeChecker:    validation.CheckMergeCompliance,
		PolicyChecker:   policy.CheckPolicy,
	}
	h.ReferenceHandler = h.HandleReference
	h.AuthorizationHandler = h.HandleAuthorization
	return h
}

// HandleStream processes git push request stream
func (h *Handler) HandleStream(packfile io.Reader, gitReceivePack io.WriteCloser) error {

	var err error

	// Get the repository state and record it as the old state
	if h.OldState == nil {
		h.OldState, err = h.Server.GetRepoState(h.Repo)
		if err != nil {
			return err
		}
	}

	// Create a push reader to read, analyse and extract info.
	// Also, pass the git writer so the pack data is written to it.
	h.PushReader, err = NewPushReader(gitReceivePack, h.Repo)
	if err != nil {
		return errors.Wrap(err, "unable to create push reader")
	}

	// Perform actions that should happen before git consumes the stream.
	// Ex: Pre-push processing authorization
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

// EnsureReferencesHaveTxDetail checks that each pushed reference has a transaction detail
func (h *Handler) EnsureReferencesHaveTxDetail() error {
	for _, ref := range h.PushReader.References.Names() {
		if h.TxDetails.Get(ref) == nil {
			return fmt.Errorf("reference (%s) has no transaction information", ref)
		}
	}
	return nil
}

// enforcePolicy enforces authorization policies against a reference command
func (h *Handler) enforcePolicy(cmd *packp.Command) error {

	ref := cmd.Name.String()
	detail := h.TxDetails.Get(ref)

	// Skip policy check for merge proposal fulfilment
	if detail.MergeProposalID != "" {
		return nil
	}

	pusher := h.TxDetails.GetPushKeyID()
	isContrib := h.Repo.IsContributor(pusher)
	isIssueRef := plumbing.IsIssueReference(ref)
	isMergeReqRef := plumbing.IsMergeRequestReference(ref)
	deleteRef := cmd.New.IsZero()
	refState := h.Repo.GetState().References.Get(ref)
	isRefCreator := !refState.IsNil() && refState.Creator.String() == pusher

	// Default action is set to 'write'
	action := policy.PolicyActionWrite

	// For delete command, set action to 'delete'.
	if deleteRef {
		action = policy.PolicyActionDelete
	}

	// For issue update, set default action to 'issue-write'.
	if isIssueRef {
		action = policy.PolicyActionIssueWrite

		// But if reference to delete is an issue reference, set action to 'issue-delete'
		if deleteRef {
			action = policy.PolicyActionIssueDelete
		}

		// When the push updated an admin field, set action to 'issue-update'. Ignore if reference is new.
		if detail.FlagCheckAdminUpdatePolicy && !refState.IsNil() {
			action = policy.PolicyActionIssueUpdate
		}
	}

	// For merge request update, set default action to 'merge-write'.
	if isMergeReqRef {
		action = policy.PolicyActionMergeRequestWrite

		// But if reference to delete is a merge request reference, set action to 'merge-delete'
		if deleteRef {
			action = policy.PolicyActionMergeRequestDelete
		}

		// When the push updated an admin field, set action to 'merge-update'. Ignore if reference is new.
		if detail.FlagCheckAdminUpdatePolicy && !refState.IsNil() {
			action = policy.PolicyActionMergeRequestUpdate
		}
	}

	err := h.PolicyChecker(h.polEnforcer, ref, isRefCreator, pusher, isContrib, action)
	if err != nil {
		return err
	}

	return nil
}

// DoAuth performs authorization checks on the specified target reference.
// If targetRef is unset, all references are checked.
// If ignorePostRefs is true, post references like issue and merge references are not checked.
func (h *Handler) DoAuth(ur *packp.ReferenceUpdateRequest, targetRef string, ignorePostRefs bool) error {
	for _, cmd := range ur.Commands {
		if targetRef != "" && targetRef != cmd.Name.String() {
			continue
		}

		if ignorePostRefs && (plumbing.IsIssueReference(cmd.Name.String()) ||
			plumbing.IsMergeRequestReference(cmd.Name.String())) {
			continue
		}

		if err := h.enforcePolicy(cmd); err != nil {
			return err
		}
	}
	return nil
}

// HandleAuthorization performs authorization checks
func (h *Handler) HandleAuthorization(ur *packp.ReferenceUpdateRequest) error {
	if err := h.EnsureReferencesHaveTxDetail(); err != nil {
		return err
	}
	return h.DoAuth(ur, "", true)
}

// HandleReferences processes all pushed references
func (h *Handler) HandleReferences() error {

	// Expect old state to have been captured before the push was processed
	if h.OldState == nil {
		return fmt.Errorf("expected old state to have been captured")
	}

	var errs = []error{}
	for _, ref := range h.PushReader.References.Names() {
		// When the previous reference handling returned errors, the only handling operation
		// to perform on the current and future references is a revert operation only
		errs = append(errs, h.ReferenceHandler(ref, len(errs) > 0)...)
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
func (h *Handler) HandleUpdate() error {

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
	if err := h.Server.GetPushPool().Add(note); err != nil {
		return err
	}

	// Announce the pushed objects
	for _, obj := range h.PushReader.Objects {
		h.AnnounceObject(obj.Hash.String())
	}

	// Broadcast the push note
	if err = h.Server.BroadcastPushObjects(note); err != nil {
		h.log.Error("Failed to broadcast push note", "Err", err)
	}

	return nil
}

// createPushNote creates a note that describes a push operation.
func (h *Handler) createPushNote() (*types.PushNote, error) {

	var note = &types.PushNote{
		TargetRepo:      h.Repo,
		PushKeyID:       crypto.MustDecodePushKeyID(h.TxDetails.GetPushKeyID()),
		RepoName:        h.TxDetails.GetRepoName(),
		Namespace:       h.TxDetails.GetRepoNamespace(),
		PusherAcctNonce: h.TxDetails.GetNonce(),
		PusherAddress:   h.Server.GetLogic().PushKeyKeeper().Get(h.TxDetails.GetPushKeyID()).Address,
		Timestamp:       time.Now().Unix(),
		NodePubKey:      h.Server.GetPrivateValidatorKey().PubKey().MustBytes32(),
		References:      types.PushedReferences{},
	}

	// Add references
	for refName, ref := range h.PushReader.References {
		detail := h.TxDetails.Get(refName)
		note.References = append(note.References, &types.PushedReference{
			Name:            refName,
			OldHash:         ref.OldHash,
			NewHash:         ref.NewHash,
			Nonce:           h.Repo.GetState().References.Get(refName).Nonce + 1,
			Objects:         h.PushReader.ObjectsRefs.GetObjectsOf(refName),
			Fee:             h.TxDetails.Get(refName).Fee,
			Value:           h.TxDetails.Get(refName).Value,
			MergeProposalID: h.TxDetails.Get(refName).MergeProposalID,
			PushSig:         h.TxDetails.Get(refName).MustSignatureAsBytes(),
			Data:            detail.ReferenceData,
		})
	}

	// Calculate the size of all pushed objects
	var err error
	objs := funk.Keys(h.PushReader.ObjectsRefs).([]string)
	note.Size, err = repo.GetObjectsSize(h.Repo, objs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get pushed objects size")
	}

	// Sign the push transaction
	note.NodeSig, err = h.Server.GetPrivateValidatorKey().PrivKey().Sign(note.BytesNoCache())
	if err != nil {
		return nil, errors.Wrap(err, "failed to sign push note")
	}

	// Store the push ID to the handler
	h.NoteID = note.ID().String()

	return note, nil
}

// AnnounceObject announces a packed object to DHT peers
func (h *Handler) AnnounceObject(objHash string) error {
	dhtKey := dht.MakeGitObjectKey(h.Repo.GetName(), objHash)
	ctx, c := context.WithTimeout(context.Background(), 60*time.Second)
	defer c()
	if err := h.Server.GetDHT().Announce(ctx, []byte(dhtKey)); err != nil {
		h.log.Warn("unable to announce git object", "Err", err)
		return err
	}
	return nil
}

// HandleReference handles reference update validation and reversion.
// When revertOnly is true, only reversion operation is performed.
func (h *Handler) HandleReference(ref string, revertOnly bool) []error {

	var errs []error
	var detail = h.TxDetails.Get(ref)

	// Get the old version of the reference prior to the push
	// and create a lone state object of the old state
	oldRef := h.OldState.GetReferences().Get(ref)
	oldRefState := plumbing.MakeStateFromItem(oldRef)

	// Get the current state of the repository; limit the query to only the target reference
	curState, err := h.Server.GetRepoState(h.Repo, plumbing.MatchOpt(ref))
	if err != nil {
		errs = append(errs, errors.Wrap(err, "failed to get current state"))
		return errs
	}

	// Now, compute the changes from the target reference old state to its current.
	changes := oldRefState.GetChanges(curState.(*plumbing.State))
	var change *types2.ItemChange
	if len(changes.References.Changes) > 0 {
		change = changes.References.Changes[0]
	}

	if revertOnly {
		goto revert
	}

	// Here, we need to validate the change for non-delete request
	if !plumbing.IsZeroHash(h.PushReader.References[ref].NewHash) {
		oldHash := h.PushReader.References[ref].OldHash
		err = h.ChangeValidator(h.Repo, oldHash, change, detail, h.Server.GetPushKeyGetter())
		if err != nil {
			errs = append(errs, errors.Wrap(err, fmt.Sprintf("validation error (%s)", ref)))
		}
	}

	// So, reference update is valid, next we need to ensure the updates
	// is compliant with the target merge proposal, if a merge proposal id is specified
	if err == nil && detail.MergeProposalID != "" {
		if err := h.MergeChecker(
			h.Repo,
			change,
			oldRef,
			h.TxDetails.Get(ref).MergeProposalID,
			h.TxDetails.GetPushKeyID(),
			h.Server.GetLogic()); err != nil {
			errs = append(errs, errors.Wrap(err, fmt.Sprintf("validation error (%s)", ref)))
		}
	}

	// Re-perform authorization checks for post references that have been flagged for update policy check
	if plumbing.IsPostReference(ref) && detail.FlagCheckAdminUpdatePolicy {
		if err = h.DoAuth(h.PushReader.GetUpdateRequest(), ref, false); err != nil {
			errs = append(errs, errors.Wrap(err, "authorization"))
		}
	}

revert:
	// As with all push operations, we must revert the changes made to the
	// repository since we do not consider them final. Here we attempt to revert
	// the repository to the old reference state. We passed the changes as an
	// option so Revert doesn't recompute it
	changes, err = h.Reverter(h.Repo, oldRefState, plumbing.MatchOpt(ref), plumbing.ChangesOpt(changes))
	if err != nil {
		errs = append(errs, errors.Wrap(err, "failed to revert to old state"))
	}

	return errs
}
