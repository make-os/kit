package push

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"time"

	"github.com/make-os/lobe/dht/announcer"
	"github.com/make-os/lobe/params"
	"github.com/make-os/lobe/remote/plumbing"
	"github.com/make-os/lobe/remote/policy"
	"github.com/make-os/lobe/remote/push/types"
	remotetypes "github.com/make-os/lobe/remote/types"
	"github.com/make-os/lobe/remote/validation"
	"github.com/make-os/lobe/types/core"
	"github.com/make-os/lobe/util"
	"github.com/make-os/lobe/util/crypto"
	plumb "gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/format/pktline"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp"

	"github.com/make-os/lobe/pkgs/logger"
	"github.com/pkg/errors"
)

// Handler describes an interface for handling incoming push updates.
type Handler interface {

	// HandleStream starts the process of handling a pushed packfile.
	//
	// It reads the pushed updates from the packfile, extracts useful
	// information and writes the update to gitReceive which is the
	// git-receive-pack process.
	//
	// Access the git-receive-pack process using gitReceiveCmd.
	//
	// pktEnc provides access to the git output encoder.
	HandleStream(packfile io.Reader, gitReceive io.WriteCloser, gitRcvCmd util.Cmd, pktEnc *pktline.Encoder) error

	// EnsureReferencesHaveTxDetail checks that each pushed reference
	// have a transaction detail that provide more information about
	// the transaction.
	EnsureReferencesHaveTxDetail() error

	// DoAuth performs authorization checks on the specified target reference.
	// If targetRef is unset, all references are checked. If ignorePostRefs is
	// true, post references like issue and merge references are not checked.
	DoAuth(ur *packp.ReferenceUpdateRequest, targetRef string, ignorePostRefs bool) error

	// HandleAuthorization performs authorization checks on all pushed references.
	HandleAuthorization(ur *packp.ReferenceUpdateRequest) error

	// HandleReferences process pushed references.
	HandleReferences() error

	// HandleRepoSize performs garbage collection and repo size validation.
	//
	// It will return error if repo size exceeds the allowed maximum.
	//
	// It will also reload the repository handle since GC makes
	// go-git internal state stale.
	HandleGCAndSizeCheck() error

	// HandleUpdate creates a push note to represent the push operation and
	// adds it to the push pool and then have it broadcast to peers.
	HandleUpdate(targetNote types.PushNote) error

	// HandleReference performs validation and update reversion for a single pushed reference.
	// // When revertOnly is true, only reversion operation is performed.
	HandleReference(ref string) []error

	// HandleReversion reverts the pushed references back to their pre-push state.
	HandleReversion() []error

	// HandlePushNote implements Handler by handing incoming push note
	HandlePushNote(note types.PushNote) (err error)
}

// BasicHandler implements Handler. It provides handles all phases of a push operation.
type BasicHandler struct {
	log                  logger.Logger
	op                   string                    // The current git operation
	Repo                 remotetypes.LocalRepo     // The target repository
	Server               core.RemoteServer         // The repository remote server
	OldState             remotetypes.RepoRefsState // The old state of the repo before the current push was written
	PushReader           *Reader                   // The push reader for reading pushed git objects
	NoteID               string                    // The push note unique ID
	reversed             bool
	ChangeValidator      validation.ChangeValidatorFunc      // Repository state change validator
	Reverter             plumbing.RevertFunc                 // Repository state reverser function
	MergeChecker         validation.MergeComplianceCheckFunc // Merge request checker function
	polEnforcer          policy.EnforcerFunc                 // Authorization policy enforcer function for the repository
	TxDetails            remotetypes.ReferenceTxDetails      // Map of references to their transaction details
	ReferenceHandler     HandleReferenceFunc                 // Pushed reference handler function
	AuthorizationHandler HandleAuthorizationFunc             // Authorization handler function
	PolicyChecker        policy.PolicyChecker                // Policy checker function
	pktEnc               *pktline.Encoder
}

// NewHandler returns an instance of BasicHandler
func NewHandler(
	repo remotetypes.LocalRepo,
	txDetails []*remotetypes.TxDetail,
	polEnforcer policy.EnforcerFunc,
	rMgr core.RemoteServer) *BasicHandler {
	h := &BasicHandler{
		Repo:            repo,
		Server:          rMgr,
		log:             rMgr.Log().Module("push-handler"),
		polEnforcer:     polEnforcer,
		PushReader:      &Reader{},
		TxDetails:       remotetypes.ToReferenceTxDetails(txDetails),
		ChangeValidator: validation.ValidateChange,
		Reverter:        plumbing.Revert,
		MergeChecker:    validation.CheckMergeCompliance,
		PolicyChecker:   policy.CheckPolicy,
		pktEnc:          pktline.NewEncoder(ioutil.Discard),
	}
	h.ReferenceHandler = h.HandleReference
	h.AuthorizationHandler = h.HandleAuthorization
	return h
}

// HandleStream implements Handler. It reads the packfile and pipes it to git.
func (h *BasicHandler) HandleStream(packfile io.Reader, gitReceive io.WriteCloser, gitRcvCmd util.Cmd, pktEnc *pktline.Encoder) error {

	if pktEnc != nil {
		h.pktEnc = pktEnc
	}

	// Get the repository state and record it as the old state
	var err error
	if h.OldState == nil {
		h.OldState, err = h.Server.GetRepoState(h.Repo)
		if err != nil {
			return err
		}
	}

	h.PushReader, err = NewPushReader(gitReceive, h.Repo)
	if err != nil {
		return errors.Wrap(err, "unable to create push reader")
	}

	// Register to use the git reference update to perform authorization checks
	h.PushReader.UseReferenceUpdateRequestRead(func(ur *packp.ReferenceUpdateRequest) error {
		h.pktEnc.Encode(plumbing.SidebandInfoln("performing authorization checks"))
		return errors.Wrap(h.AuthorizationHandler(ur), "authorization")
	})

	// Write the packfile to the push reader.
	if _, err := io.Copy(h.PushReader, packfile); err != nil {
		return err
	}

	h.pktEnc.Encode(plumbing.SidebandInfoln("reading objects and references"))

	// Read the packfile objects
	gitRcvErr := bytes.NewBuffer(nil)
	gitRcvCmd.SetStderr(gitRcvErr)
	if err = h.PushReader.Read(); err != nil {
		return err
	}

	// Wait for git-receive-pack to finish
	if err := gitRcvCmd.ProcessWait(); err != nil {
		return fmt.Errorf("git-receive-pack: write error: %s", strings.TrimSpace(gitRcvErr.String()))
	}

	return nil
}

// EnsureReferencesHaveTxDetail implements Handler. It ensures each references
// have a signed push transaction detail.
func (h *BasicHandler) EnsureReferencesHaveTxDetail() error {
	for _, ref := range h.PushReader.References.Names() {
		if h.TxDetails.Get(ref) == nil {
			return fmt.Errorf("reference (%s) has no transaction information", ref)
		}
	}
	return nil
}

// enforcePolicy enforces authorization policies against a reference command
func (h *BasicHandler) enforcePolicy(cmd *packp.Command) error {

	ref := cmd.Name.String()
	detail := h.TxDetails.Get(ref)

	// Skip policy check for merge proposal fulfilment
	if detail.MergeProposalID != "" {
		return nil
	}

	// Default action is 'write'
	action := policy.PolicyActionWrite

	// For delete command, set action to 'delete'.
	if cmd.New.IsZero() {
		action = policy.PolicyActionDelete
	}

	refState := h.Repo.GetState().References.Get(ref)

	// When the push updated an admin field, set action to 'update'. Ignore if reference is new.
	if detail.FlagCheckAdminUpdatePolicy && !refState.IsNil() {
		action = policy.PolicyActionUpdate
	}

	pusher := h.TxDetails.GetPushKeyID()
	err := h.PolicyChecker(
		h.polEnforcer,
		ref,
		!refState.IsNil() && refState.Creator.String() == pusher,
		pusher,
		h.Repo.IsContributor(pusher),
		action)
	if err != nil {
		return err
	}

	return nil
}

// DoAuth implements Handler. It performs access-level checks.
func (h *BasicHandler) DoAuth(ur *packp.ReferenceUpdateRequest, targetRef string, ignorePostRefs bool) error {
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

// HandleAuthorizationFunc describes a function for checking authorization to access a reference
type HandleAuthorizationFunc func(ur *packp.ReferenceUpdateRequest) error

// HandleAuthorization implements Handler
func (h *BasicHandler) HandleAuthorization(ur *packp.ReferenceUpdateRequest) error {
	if err := h.EnsureReferencesHaveTxDetail(); err != nil {
		return err
	}
	return h.DoAuth(ur, "", true)
}

// HandleReferences implements Handler. It processes pushed references.
func (h *BasicHandler) HandleReferences() error {

	if h.OldState == nil {
		return fmt.Errorf("expected old state to have been captured")
	}

	var errs []error
	for _, ref := range h.PushReader.References.Names() {
		errs = append(errs, h.ReferenceHandler(ref)...)
	}

	if len(errs) > 0 {
		return errs[0]
	}

	return nil
}

// HandleRepoSize implements Handler. Performs garbage collection and repo size limit check.
func (h *BasicHandler) HandleGCAndSizeCheck() error {

	// Perform garbage collection to:
	// - pack loose objects
	// - remove unreachable that are at least a day old
	h.pktEnc.Encode(plumbing.SidebandInfoln("running garbage collector"))
	if err := h.Repo.GC("1 day ago"); err != nil {
		return errors.Wrap(err, "failed to run garbage collection")
	}

	// Get and ensure size of repository did not exceed the limit
	size, err := h.Repo.Size()
	if err != nil {
		return errors.Wrap(err, "failed to get repo size")
	} else if size > float64(params.MaxRepoSize) {
		return fmt.Errorf("size error: repository size has exceeded the network limit")
	}

	// Reload the repository. Necessary because after GC, the repo
	// handle's internal indices becomes stale.
	if err := h.Repo.Reload(); err != nil {
		return errors.Wrap(err, "failed to reload repo handle")
	}

	return nil
}

// HandleAnnouncement announces the repository name, pushed commit and tag objects.
// cb is called with the number of failed announcements.
func (h *BasicHandler) HandleAnnouncement(cb func(errCount int)) {
	sess := h.Server.GetDHT().NewAnnouncerSession()
	repoName := h.Repo.GetName()
	sess.Announce(announcer.ObjTypeRepoName, repoName, []byte(repoName))
	for _, obj := range h.PushReader.Objects {
		if obj.Type == plumb.CommitObject || obj.Type == plumb.TagObject {
			sess.Announce(announcer.ObjTypeGit, repoName, obj.Hash[:])
		}
	}
	go sess.OnDone(cb)
}

// HandleRefMismatch handles cases where a reference in the push note differs from
// the hash of its corresponding local or network reference hash.
// We react by attempting resyncing the reference.
func (h *BasicHandler) HandleRefMismatch(note types.PushNote, ref string, netMismatch bool) (err error) {
	err = h.Server.TryScheduleReSync(note, ref, netMismatch)
	if err != nil {
		h.pktEnc.Encode(plumbing.SidebandYellowln(err.Error()))
		return
	}
	h.pktEnc.Encode(plumbing.SidebandYellowln(fmt.Sprintf("%s: mismatched state detected; resynchronizing...", ref)))
	return
}

// HandleUpdate implements Handler
func (h *BasicHandler) HandleUpdate(targetNote types.PushNote) (err error) {

	// Perform garbage collection and repo size limit check.
	// Revert pushed updates on error.
	if err = h.HandleGCAndSizeCheck(); err != nil {
		h.HandleReversion()
		return err
	}

	// Process the pushed references
	// Revert pushed updates on error.
	h.pktEnc.Encode(plumbing.SidebandInfoln("performing repo and references validation"))
	err = h.HandleReferences()
	if err != nil {
		h.HandleReversion()
		return err
	}

	// Revert pushed updates
	h.HandleReversion()

	// Create and sign push notification only if a target note was not provided.
	// Then, validate the push note:
	//  - If we get an error about a pushed reference and its corresponding local/network reference
	//    having a hash mismatch, we need to determine whether to schedule the local reference
	//    for re-sync.
	var note = targetNote
	if note == nil {
		h.pktEnc.Encode(plumbing.SidebandInfoln("creating and validating push note"))
		note, err = h.createPushNote()
		if err != nil {
			return err
		}
		if err = h.Server.CheckNote(note); err != nil {
			if misErr, ok := err.(*util.BadFieldError).Data.(*validation.RefMismatchErr); ok {
				h.HandleRefMismatch(note, misErr.Ref, misErr.MismatchNet)
			}
			return errors.Wrap(err, "failed push note validation")
		}
	}

	if err = h.HandlePushNote(note); err != nil {
		return err
	}

	h.pktEnc.Encode(plumbing.SidebandProgressln("hash: " + h.NoteID))

	return nil
}

// HandlePushNote implements Handler by handing incoming push note
func (h *BasicHandler) HandlePushNote(note types.PushNote) (err error) {

	// Add the push note to the push pool
	h.pktEnc.Encode(plumbing.SidebandInfoln("adding push note to the pushpool"))
	if err = h.Server.GetPushPool().Add(note); err != nil {
		return err
	}

	// Announce the pushed objects (note and endorsement)
	// Broadcast the push note if announcement succeeded without a failure.
	h.HandleAnnouncement(func(errCount int) {
		if errCount > 0 {
			return
		}
		h.pktEnc.Encode(plumbing.SidebandInfoln("broadcasting push note and endorsement"))
		if err = h.Server.BroadcastNoteAndEndorsement(note); err != nil {
			h.log.Error("Failed to broadcast push note", "Err", err)
		}
	})

	return
}

// createPushNote creates a note that describes a push request.
func (h *BasicHandler) createPushNote() (*types.Note, error) {

	var note = &types.Note{
		TargetRepo:      h.Repo,
		PushKeyID:       crypto.MustDecodePushKeyID(h.TxDetails.GetPushKeyID()),
		RepoName:        h.TxDetails.GetRepoName(),
		Namespace:       h.TxDetails.GetRepoNamespace(),
		PusherAcctNonce: h.TxDetails.GetNonce(),
		PusherAddress:   h.Server.GetLogic().PushKeyKeeper().Get(h.TxDetails.GetPushKeyID()).Address,
		Timestamp:       time.Now().Unix(),
		CreatorPubKey:   h.Server.GetPrivateValidatorKey().PubKey().MustBytes32(),
		References:      types.PushedReferences{},
	}

	// Add references
	for refName, ref := range h.PushReader.References {
		detail := h.TxDetails.Get(refName)
		note.References = append(note.References, &types.PushedReference{
			Name:            refName,
			OldHash:         ref.OldHash,
			NewHash:         ref.NewHash,
			Nonce:           h.Repo.GetState().References.Get(refName).Nonce.UInt64() + 1,
			Fee:             h.TxDetails.Get(refName).Fee,
			Value:           h.TxDetails.Get(refName).Value,
			MergeProposalID: h.TxDetails.Get(refName).MergeProposalID,
			PushSig:         h.TxDetails.Get(refName).MustSignatureAsBytes(),
			Data:            detail.ReferenceData,
		})
	}

	var err error

	// Determine the size of the pushed reference objects
	note.Size, err = GetSizeOfObjects(note)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get size of pushed objects")
	}

	// Sign the push transaction
	note.RemoteNodeSig, err = h.Server.GetPrivateValidatorKey().PrivKey().Sign(note.BytesNoCache())
	if err != nil {
		return nil, errors.Wrap(err, "failed to sign push note")
	}

	// Store the push ID to the handler
	h.NoteID = note.ID().String()

	return note, nil
}

// HandleReversion reverts the pushed references back to their pre-push state.
// It will do nothing and return nil if already called successfully.
func (h *BasicHandler) HandleReversion() []error {

	if h.reversed {
		return nil
	}

	var errs []error
	for _, ref := range h.PushReader.References.Names() {
		// Get the old version of the reference prior to the push
		// and create a lone state object of the old state
		oldState := plumbing.MakeStateFromItem(h.OldState.GetReferences().Get(ref))

		// Get the current state of the repository; limit the query to only the target reference
		curState, err := h.Server.GetRepoState(h.Repo, plumbing.MatchOpt(ref))
		if err != nil {
			errs = append(errs, errors.Wrapf(err, "%s: failed to get current state", ref))
			return errs
		}

		// Now, compute the changes from the target reference old state to its current.
		changes := oldState.GetChanges(curState.(*plumbing.State))

		// Revert to old state
		h.pktEnc.Encode(plumbing.SidebandInfoln(fmt.Sprintf("%s: reverting to pre-push state", ref)))
		_, err = h.Reverter(h.Repo, oldState, plumbing.MatchOpt(ref), plumbing.ChangesOpt(changes))
		if err != nil {
			errs = append(errs, errors.Wrapf(err, "%s: failed to revert to old state", ref))
		}
	}

	if len(errs) == 0 {
		h.reversed = true
	}

	return errs
}

// GetChange computes and returns the change made to the given reference
func (h *BasicHandler) GetChange(ref string) (*remotetypes.ItemChange, error) {

	// Get the old version of the reference prior to the push
	// and create a lone state object of the old state
	oldRefState := plumbing.MakeStateFromItem(h.OldState.GetReferences().Get(ref))

	// Get the current state of the repository; limit the query to only the target reference
	curState, err := h.Server.GetRepoState(h.Repo, plumbing.MatchOpt(ref))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get current state")
	}

	// Now, compute the changes from the target reference old state to its current.
	changes := oldRefState.GetChanges(curState.(*plumbing.State))
	var change *remotetypes.ItemChange
	if len(changes.References.Changes) > 0 {
		change = changes.References.Changes[0]
	}

	return change, nil
}

// HandleReferenceFunc describes a function for processing a reference
type HandleReferenceFunc func(ref string) []error

// HandleReference implements Handler
func (h *BasicHandler) HandleReference(ref string) (errs []error) {

	// Get the changes made to the reference
	change, err := h.GetChange(ref)
	if err != nil {
		return []error{err}
	}

	var detail = h.TxDetails.Get(ref)

	// Here, we need to validate the change for non-delete request.
	// If no change, skip validation.
	if change != nil && !plumbing.IsZeroHash(h.PushReader.References[ref].NewHash) {
		oldHash := h.PushReader.References[ref].OldHash
		err = h.ChangeValidator(h.Server.GetLogic(), h.Repo, oldHash, change, detail, h.Server.GetPushKeyGetter())
		if err != nil {
			errs = append(errs, errors.Wrap(err, fmt.Sprintf("validation error (%s)", ref)))
		}
	}

	// So, reference update is valid, next we need to ensure the updates
	// is compliant with the target merge proposal, if a merge proposal id is specified
	if err == nil && detail.MergeProposalID != "" {
		h.pktEnc.Encode(plumbing.SidebandInfoln(fmt.Sprintf("%s: performing merge compliance check", ref)))
		if err := h.MergeChecker(
			h.Repo,
			change,
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

	return errs
}
