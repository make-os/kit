package push

import (
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"time"

	"github.com/make-os/lobe/params"
	"github.com/make-os/lobe/remote/plumbing"
	"github.com/make-os/lobe/remote/policy"
	"github.com/make-os/lobe/remote/push/types"
	types2 "github.com/make-os/lobe/remote/types"
	"github.com/make-os/lobe/remote/validation"
	"github.com/make-os/lobe/types/core"
	"github.com/make-os/lobe/util/crypto"
	plumb "gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/format/pktline"
	"gopkg.in/src-d/go-git.v4/plumbing/protocol/packp"

	"github.com/make-os/lobe/pkgs/logger"
	"github.com/pkg/errors"
)

// Handler describes an interface for handling push updates,
// pre-consensus dry run and authorization checks.
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
	HandleStream(packfile io.Reader, gitReceive io.WriteCloser, gitReceiveCmd *exec.Cmd, pktEnc *pktline.Encoder) error

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

	// HandleReferences validates all pushed references and reverts changes
	// introduced by HandleStream's processing of the references.
	HandleReferences() error

	// HandleRepoSize performs garbage collection and repo size validation.
	// It will return error if repo size exceeds the allowed maximum.
	HandleRepoSize() error

	// HandleUpdate creates a push note to represent the push operation and
	// adds it to the push pool and then have it broadcast to peers.
	HandleUpdate() error

	// HandleReference performs validation and update reversion for a single pushed reference.
	// // When revertOnly is true, only reversion operation is performed.
	HandleReference(ref string, revertOnly bool) []error
}

// BasicHandler implements Handler. It provides handles all phases of a push operation.
type BasicHandler struct {
	log                  logger.Logger
	op                   string                              // The current git operation
	Repo                 types2.LocalRepo                    // The target repository
	Server               core.RemoteServer                   // The repository remote server
	OldState             types2.BareRepoRefsState            // The old state of the repo before the current push was written
	PushReader           *Reader                             // The push reader for reading pushed git objects
	NoteID               string                              // The push note unique ID
	ChangeValidator      validation.ChangeValidatorFunc      // Repository state change validator
	Reverter             plumbing.RevertFunc                 // Repository state reverser function
	MergeChecker         validation.MergeComplianceCheckFunc // Merge request checker function
	polEnforcer          policy.EnforcerFunc                 // Authorization policy enforcer function for the repository
	TxDetails            types2.ReferenceTxDetails           // Map of references to their transaction details
	ReferenceHandler     RefHandler                          // Pushed reference handler function
	AuthorizationHandler AuthorizationHandler                // Authorization handler function
	PolicyChecker        policy.PolicyChecker                // Policy checker function
	pktEnc               *pktline.Encoder
}

// NewHandler returns an instance of BasicHandler
func NewHandler(
	repo types2.LocalRepo,
	txDetails []*types2.TxDetail,
	polEnforcer policy.EnforcerFunc,
	rMgr core.RemoteServer) *BasicHandler {
	h := &BasicHandler{
		Repo:            repo,
		Server:          rMgr,
		log:             rMgr.Log().Module("push-handler"),
		polEnforcer:     polEnforcer,
		PushReader:      &Reader{},
		TxDetails:       types2.ToReferenceTxDetails(txDetails),
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

// HandleStream implements Handler
func (h *BasicHandler) HandleStream(packfile io.Reader, gitReceive io.WriteCloser, gitReceiveCmd *exec.Cmd, pktEnc *pktline.Encoder) error {

	var err error

	if pktEnc != nil {
		h.pktEnc = pktEnc
	}

	// Get the repository state and record it as the old state
	if h.OldState == nil {
		h.OldState, err = h.Server.GetRepoState(h.Repo)
		if err != nil {
			return err
		}
	}

	// Create a push reader to read, analyse and extract info.
	// Also, pass the git writer so the pack data is written to it.
	h.PushReader, err = NewPushReader(gitReceive, h.Repo)
	if err != nil {
		return errors.Wrap(err, "unable to create push reader")
	}

	// Perform actions that should happen before git consumes the stream.
	// - Authorization
	h.PushReader.OnReferenceUpdateRequestRead(func(ur *packp.ReferenceUpdateRequest) error {
		h.pktEnc.Encode(plumbing.SidebandInfo("performing authorization checks"))
		return errors.Wrap(h.AuthorizationHandler(ur), "authorization")
	})

	// Write the packfile to the push reader.
	if _, err := io.Copy(h.PushReader, packfile); err != nil {
		return err
	}

	h.pktEnc.Encode(plumbing.SidebandInfo("reading objects and references"))

	// Read the packfile objects
	if err = h.PushReader.Read(gitReceiveCmd); err != nil {
		return err
	}

	return nil
}

// EnsureReferencesHaveTxDetail implements Handler
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

// DoAuth implements Handler.
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

// AuthorizationHandler describes a function for checking authorization to access a reference
type AuthorizationHandler func(ur *packp.ReferenceUpdateRequest) error

// HandleAuthorization implements Handler
func (h *BasicHandler) HandleAuthorization(ur *packp.ReferenceUpdateRequest) error {
	if err := h.EnsureReferencesHaveTxDetail(); err != nil {
		return err
	}
	return h.DoAuth(ur, "", true)
}

// HandleReferences implements Handler
func (h *BasicHandler) HandleReferences() error {

	// Expect old state to have been captured before the push was processed
	if h.OldState == nil {
		return fmt.Errorf("expected old state to have been captured")
	}

	var errs []error
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

// HandleRepoSize implements Handler
func (h *BasicHandler) HandleRepoSize() error {

	// Perform garbage collection to:
	// - pack loose objects
	// - remove unreachable that are at least a day old
	h.pktEnc.Encode(plumbing.SidebandInfo("running garbage collector"))
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

	return nil
}

// HandleAnnouncement announces the repository name, pushed commit and tag objects.
func (h *BasicHandler) HandleAnnouncement() {
	h.Server.GetDHT().Announce([]byte(h.Repo.GetName()), nil)
	for _, obj := range h.PushReader.Objects {
		if obj.Type == plumb.CommitObject || obj.Type == plumb.TagObject {
			h.Server.GetDHT().Announce(obj.Hash[:], nil)
		}
	}
}

// HandleUpdate implements Handler
func (h *BasicHandler) HandleUpdate() error {

	h.pktEnc.Encode(plumbing.SidebandInfo("performing repo and references validation"))

	// Validate the pushed references
	err := h.HandleReferences()
	if err != nil {
		return err
	}

	// Check repository size check
	if err := h.HandleRepoSize(); err != nil {
		return err
	}

	// Construct a push note
	h.pktEnc.Encode(plumbing.SidebandInfo("creating push note"))
	note, err := h.createPushNote()
	if err != nil {
		return err
	}

	// Add the push note to the push pool
	h.pktEnc.Encode(plumbing.SidebandInfo("adding push note to the pushpool"))
	if err := h.Server.GetPushPool().Add(note); err != nil {
		return err
	}

	// Announce the pushed objects to the DHT
	h.HandleAnnouncement()

	// Broadcast the push note
	h.pktEnc.Encode(plumbing.SidebandInfo("broadcasting push note and endorsement"))
	if err = h.Server.BroadcastNoteAndEndorsement(note); err != nil {
		h.log.Error("Failed to broadcast push note", "Err", err)
	}

	h.pktEnc.Encode(plumbing.SidebandProgress("hash: " + h.NoteID))

	return nil
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

// RefHandler describes a function for processing a reference
type RefHandler func(ref string, revertOnly bool) []error

// HandleReference implements Handler
func (h *BasicHandler) HandleReference(ref string, revertOnly bool) []error {

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

	// Jump to revert label if only reversion is requested.
	if revertOnly {
		goto revert
	}

	// Here, we need to validate the change for non-delete request.
	if !plumbing.IsZeroHash(h.PushReader.References[ref].NewHash) {
		oldHash := h.PushReader.References[ref].OldHash
		err = h.ChangeValidator(h.Server.GetLogic(), h.Repo, oldHash, change, detail, h.Server.GetPushKeyGetter())
		if err != nil {
			errs = append(errs, errors.Wrap(err, fmt.Sprintf("validation error (%s)", ref)))
		}
	}

	// So, reference update is valid, next we need to ensure the updates
	// is compliant with the target merge proposal, if a merge proposal id is specified
	if err == nil && detail.MergeProposalID != "" {
		h.pktEnc.Encode(plumbing.SidebandInfo(fmt.Sprintf("%s: performing merge compliance check", ref)))
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

revert:
	// As with all push operations, we must revert the changes made to the
	// repository since we do not consider them final. Here we attempt to revert
	// the repository to the old reference state. We passed the changes as an
	// option so Revert doesn't recompute it
	h.pktEnc.Encode(plumbing.SidebandInfo(fmt.Sprintf("%s: reverting to pre-push state", ref)))
	changes, err = h.Reverter(h.Repo, oldRefState, plumbing.MatchOpt(ref), plumbing.ChangesOpt(changes))
	if err != nil {
		errs = append(errs, errors.Wrap(err, "failed to revert to old state"))
	}

	return errs
}
