package server

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"

	crypto2 "github.com/make-os/lobe/crypto"
	"github.com/make-os/lobe/crypto/bdn"
	"github.com/make-os/lobe/dht/announcer"
	"github.com/make-os/lobe/params"
	"github.com/make-os/lobe/remote/plumbing"
	"github.com/make-os/lobe/remote/policy"
	"github.com/make-os/lobe/remote/push"
	pushtypes "github.com/make-os/lobe/remote/push/types"
	rr "github.com/make-os/lobe/remote/repo"
	remotetypes "github.com/make-os/lobe/remote/types"
	"github.com/make-os/lobe/remote/validation"
	"github.com/make-os/lobe/types/state"
	"github.com/make-os/lobe/types/txns"
	"github.com/make-os/lobe/util"
	"github.com/make-os/lobe/util/crypto"
	"github.com/pkg/errors"
	"github.com/tendermint/tendermint/p2p"
	"github.com/thoas/go-funk"
	"gopkg.in/src-d/go-git.v4"
	plumbing2 "gopkg.in/src-d/go-git.v4/plumbing"
	packfile2 "gopkg.in/src-d/go-git.v4/plumbing/format/packfile"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

// Receive implements Reactor
func (sv *Server) Receive(chID byte, peer p2p.Peer, msgBytes []byte) {
	switch chID {
	case PushNoteReactorChannel:
		if err := sv.onPushNoteReceived(peer, msgBytes); err != nil {
			sv.log.Error("failed to handle push note", "Err", err.Error())
		}
	case PushEndReactorChannel:
		if err := sv.onEndorsementReceived(peer, msgBytes); err != nil {
			sv.log.Error("failed to handle push endorsement", "Err", err.Error())
		}
	}
}

type ScheduleReSyncFunc func(note pushtypes.PushNote, ref string) error

// maybeScheduleReSync checks whether the local repository needs to be scheduled for synchronized.
// note is the note containing the problematic ref.
// ref is the name of the reference that may be out of sync.
func (sv *Server) maybeScheduleReSync(note pushtypes.PushNote, ref string) error {

	localRef, err := note.GetTargetRepo().Reference(plumbing2.ReferenceName(ref), false)
	if err != nil {
		return err
	}
	localRefHash := localRef.Hash()

	// Check if the reference's local and network hash match.
	// If yes, no resync needs to happen.
	repoState := note.GetTargetRepo().GetState()
	if bytes.Equal(repoState.References.Get(ref).Hash.Bytes(), localRefHash[:]) {
		return nil
	}

	// Get last synchronized
	refLastSyncHeight, err := sv.logic.RepoSyncInfoKeeper().GetRefLastSyncHeight(note.GetRepoName(), ref)
	if err != nil {
		return err
	}

	// If the last successful synced reference height equal the last successful synced
	// height for the entire repo, it means something unnatural/external messed the repo
	// history. We react by resyncing the repo from height 0
	repoLastUpdated := repoState.UpdatedAt.UInt64()
	if refLastSyncHeight == repoLastUpdated {
		refLastSyncHeight = 0
	}

	// Add the repo to the refsync watcher
	sv.refSyncer.Watch(note.GetRepoName(), ref, refLastSyncHeight, repoLastUpdated)

	return nil
}

// onPushNoteReceived handles incoming Note messages
func (sv *Server) onPushNoteReceived(peer p2p.Peer, msgBytes []byte) error {

	// Attempt to decode message to a PushNote
	var note = pushtypes.Note{FromRemotePeer: true}
	if err := util.ToObject(msgBytes, &note); err != nil {
		return errors.Wrap(err, "failed to decoded message")
	}

	// Ignore note if previously seen or mark note as 'seen'
	noteID := note.ID().String()
	if sv.isNoteSeen(noteID) {
		return nil
	}
	sv.markNoteAsSeen(noteID)

	peerID, repoName := peer.ID(), note.GetRepoName()
	sv.log.Debug("Received a push note", "PeerID", peerID, "ID", noteID)

	// Ensure target repository exists
	repoPath, repoState := sv.getRepoPath(repoName), sv.logic.RepoKeeper().Get(repoName)
	if repoState.IsNil() {
		return fmt.Errorf("repo '%s' not found", repoName)
	}

	// If namespace is set, get it and ensure it exists
	var namespace *state.Namespace
	if note.Namespace != "" {
		namespace = sv.logic.NamespaceKeeper().Get(crypto.MakeNamespaceHash(note.Namespace))
		if namespace.IsNil() {
			return fmt.Errorf("namespace '%s' not found", note.Namespace)
		}
	}

	// Reconstruct references transaction details from push note
	txDetails := validation.GetTxDetailsFromNote(&note)

	// Perform authentication check
	polEnforcer, err := sv.authenticate(txDetails, repoState, namespace, sv.logic, validation.CheckTxDetail)
	if err != nil {
		return errors.Wrap(err, "authorization failed")
	}

	// If the node is in validator mode or the target repository cannot
	// be synced, we can only validate and broadcast the node.
	if err := sv.refSyncer.CanSync(note.Namespace, note.RepoName); err != nil || sv.cfg.IsValidatorNode() {
		sv.log.Info("Partially processing received push note",
			"ID", noteID, "IsValidator", sv.cfg.IsValidatorNode(), "IsUntrackedRepo", err != nil)

		if err := sv.checkPushNote(&note, sv.logic); err != nil {
			return errors.Wrap(err, "failed push note validation")
		}

		sv.registerNoteSender(string(peerID), noteID)
		sv.noteBroadcaster(&note)
		return nil
	}

	// Get a reference to the local repository
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to open repo '%s'", repoName))
	}

	// Set the target repository object
	note.TargetRepo = &rr.Repo{
		Repository:    repo,
		LiteGit:       rr.NewLiteGit(sv.gitBinPath, repoPath),
		Path:          repoPath,
		NamespaceName: note.Namespace,
		State:         repoState,
		Namespace:     namespace,
	}

	// Validate the push note.
	if err := sv.checkPushNote(&note, sv.logic); err != nil {

		// If we get an error about a pushed reference and local reference mismatch,
		// we need to determine whether to schedule the repository for a resynchronization.
		if funk.ContainsString(err.(*util.BadFieldError).Data, validation.ErrCodePushRefAndLocalHashMismatch) {
			sv.scheduleReSync(&note, err.(*util.BadFieldError).Data[1])
		}

		return errors.Wrap(err, "failed push note validation")
	}

	// Register a cache entry that indicates the sender of the push note
	sv.registerNoteSender(string(peerID), noteID)

	// For each packfile fetched, announce commit and tag objects.
	sv.objfetcher.OnPackReceived(func(hash string, packfile io.ReadSeeker) {
		plumbing.UnpackPackfile(packfile, func(header *packfile2.ObjectHeader, read func() (object.Object, error)) error {
			obj, _ := read()
			if obj.Type() == plumbing2.CommitObject || obj.Type() == plumbing2.TagObject {
				objHash := obj.ID()
				sv.Announce(announcer.ObjTypeGit, repoName, objHash[:], nil)
			}
			return nil
		})
	})

	// FetchAsync the objects for each references in the push note.
	// The callback is called when all objects have been fetched successfully.
	sv.objfetcher.FetchAsync(&note, func(err error) {
		sv.onObjectsFetched(err, &note, txDetails, polEnforcer)
	})

	return nil
}

// onObjectsFetched is called after all objects of the push note have been
// completely fetched or an error occurred while fetching.
func (sv *Server) onObjectsFetched(
	err error,
	note pushtypes.PushNote,
	txDetails []*remotetypes.TxDetail,
	polEnforcer policy.EnforcerFunc) error {

	if err != nil {
		sv.log.Error("Failed to fetch all note objects", "ID", note.ID().String(), "Err", err.Error())
		return err
	}

	// Reload repository handle because the handle's internal reference
	// become stale after new objects where written to the repository.
	if err = note.GetTargetRepo().Reload(); err != nil {
		return errors.Wrap(err, "failed to reload repo handle")
	}

	// Get the size of the pushed update objects. This is the size of the objects required
	// to bring the local reference up to the state of the note's pushed reference.
	repoName := note.GetRepoName()
	localSize, err := push.GetSizeOfObjects(note)
	if err != nil {
		sv.log.Error("Failed to get size of pushed refs objects", "Err", err.Error(), "Repo", repoName)
		return errors.Wrapf(err, "failed to get pushed refs objects size")
	}

	// Verify the note's size ensuring it matches the local size
	// TODO: Penalize remote node for the inconsistency
	if noteSize := note.GetSize(); note.IsFromRemotePeer() && noteSize != localSize {
		sv.log.Error("push note size and local size mismatch", "Size", noteSize,
			"LocalSize", localSize, "Repo", repoName)
		return fmt.Errorf("note's objects size and local size differs")
	}

	// Attempt to process the push note
	if err = sv.processPushNote(note, txDetails, polEnforcer); err != nil {
		sv.log.Error("Failed to process push note", "ID", note.ID().String(), "Err", err.Error())
		return err
	}

	// Announce interest in providing the repository objects
	sv.Announce(announcer.ObjTypeRepoName, repoName, []byte(repoName), nil)

	return nil
}

// MaybeProcessPushNoteFunc is a function for processing a push note
type MaybeProcessPushNoteFunc func(note pushtypes.PushNote,
	txDetails []*remotetypes.TxDetail,
	polEnforcer policy.EnforcerFunc) error

// maybeProcessPushNote validates and dry-run the push note.
// It expects the pushed objects to be present in the target repository.
func (sv *Server) maybeProcessPushNote(
	note pushtypes.PushNote,
	txDetails []*remotetypes.TxDetail,
	polEnforcer policy.EnforcerFunc) error {

	// Create a packfile that represents updates described in the note.
	packfile, err := sv.makeReferenceUpdatePack(note)
	if err != nil {
		return errors.Wrap(err, "failed to create packfile from push note")
	}

	// Create the git-receive-pack command
	repoPath := note.GetTargetRepo().GetPath()
	cmd := exec.Command(sv.gitBinPath, []string{"receive-pack", "--stateless-rpc", repoPath}...)
	cmd.Dir = repoPath

	// Get the command's stdin pipe
	in, err := cmd.StdinPipe()
	if err != nil {
		return errors.Wrap(err, "failed to get stdin pipe")
	}

	// Start the command
	err = cmd.Start()
	if err != nil {
		return errors.Wrap(err, "git-receive-pack failed to start")
	}

	// Read, analyse and pass the packfile to git
	handler := sv.makePushHandler(note.GetTargetRepo(), txDetails, polEnforcer)
	if err := handler.HandleStream(packfile, in, util.NewWrappedCmd(cmd), nil); err != nil {
		return errors.Wrap(err, "HandleStream error")
	}

	if err := handler.HandleUpdate(note); err != nil {
		return errors.Wrap(err, "HandleUpdate error")
	}

	return nil
}

// onEndorsementReceived handles incoming Endorsement messages
func (sv *Server) onEndorsementReceived(peer p2p.Peer, msgBytes []byte) error {

	var peerID = peer.ID()

	// Decode the endorsement
	var endorsement pushtypes.PushEndorsement
	if err := util.ToObject(msgBytes, &endorsement); err != nil {
		return errors.Wrap(err, "failed to decode endorsement")
	}

	// Validate the endorsement
	endID := endorsement.ID().String()
	if err := sv.checkEndorsement(&endorsement, sv.logic, -1); err != nil {
		sv.log.Debug("Received endorsement failed validation", "ID", endID, "Err", err)
		return errors.Wrap(err, "endorsement validation failed")
	}

	sv.log.Debug("Received a valid push endorsement",
		"PeerID", peerID, "ID", endID, "Endorser", crypto2.ToBase58PubKey(endorsement.EndorserPubKey))

	// Cache the sender so we don't broadcast same Endorsement to it later
	sv.registerEndorsementSender(string(peerID), endID)

	noteID := endorsement.NoteID.HexStr()

	// If in validator mode, next step is to broadcast
	if sv.cfg.IsValidatorNode() {
		goto broadcast
	}

	// cache the Endorsement object as an endorsement of the PushNote
	sv.registerNoteEndorsement(noteID, &endorsement)

	// Attempt to create an send a PushTx to the transaction pool
	sv.makePushTx(noteID)

broadcast:
	// Broadcast the Endorsement to peers
	sv.endorsementBroadcaster(&endorsement)

	return nil
}

// CreatePushTxFunc describes a function that takes a push note and creates
// a push transaction which is then added to the mempool.
type CreatePushTxFunc func(noteID string) error

// createPushTx attempts to create a PushTx from a given push note, only if
// a push note matching the given id exist in the push pool and the push note
// has received a quorum Endorsement.
func (sv *Server) createPushTx(noteID string) error {

	// Get the list of push endorsements received for the push note
	endorsements := sv.endorsements.Get(noteID)
	if endorsements == nil {
		return fmt.Errorf("no endorsements yet")
	}

	// Ensure there are enough push endorsements
	endorsementIdx := endorsements.(map[string]*pushtypes.PushEndorsement)
	if len(endorsementIdx) < params.PushEndorseQuorumSize {
		msg := "cannot create push transaction; note has %d endorsements, wants %d"
		return fmt.Errorf(msg, len(endorsementIdx), params.PushEndorseQuorumSize)
	}

	// Get the push note from the push pool
	note := sv.GetPushPool().Get(noteID)
	if note == nil {
		return fmt.Errorf("push note not found in pool")
	}

	// Get the top hosts
	hosts, err := sv.logic.GetTicketManager().GetTopHosts(params.NumTopHostsLimit)
	if err != nil {
		return errors.Wrap(err, "failed to get top hosts")
	}

	// Collect the BLS public keys of all Endorsement senders.
	// We need them for the construction of BLS aggregated signature.
	noteEndorsements := funk.Values(endorsementIdx).([]*pushtypes.PushEndorsement)
	var endorsementsPubKey []*bdn.PublicKey
	var endorsementsSig [][]byte
	for i, ed := range noteEndorsements {

		// Get the selected ticket of the endorsers
		selTicket := hosts.Get(ed.EndorserPubKey)
		if selTicket == nil {
			return fmt.Errorf("endorsement[%d]: ticket not found in top hosts list", i)
		}

		// Get their BLS public key from the ticket
		pk, err := bdn.BytesToPublicKey(selTicket.Ticket.BLSPubKey)
		if err != nil {
			return errors.Wrapf(err, "endorsement[%d]: bls public key is invalid", i)
		}

		// Collect the public key and signature for later generation of aggregated signature
		endorsementsPubKey = append(endorsementsPubKey, pk)
		endorsementsSig = append(endorsementsSig, ed.SigBLS)

		// Clone the endorsement and replace endorsement at i.
		// Clear the BLS signature and Note ID fields to reduce serialized message size.
		noteEndorsements[i] = ed.Clone()
		noteEndorsements[i].SigBLS = nil
		noteEndorsements[i].NoteID = nil

		// Similarly, clear references information from all endorsement except the 0-index reference.
		// No need keeping repeating information that can be deduced from the 0-index reference
		// considering all endorsement endorse same piece of data.
		if i != 0 {
			noteEndorsements[i].References = nil
		}
	}

	// Create a new push transaction
	pushTx := txns.NewBareTxPush()

	// Set push note and endorsements
	pushTx.Note = note
	pushTx.Endorsements = noteEndorsements

	// Generate and set aggregated BLS signature
	aggSig, err := bdn.AggregateSignatures(endorsementsPubKey, endorsementsSig)
	if err != nil {
		return errors.Wrap(err, "unable to create aggregated signature")
	}
	pushTx.AggregatedSig = aggSig

	// Register push transaction to mempool
	if err := sv.GetMempool().Add(pushTx); err != nil {
		return errors.Wrap(err, "failed to add push tx to mempool")
	}

	pushTx.Note.SetTargetRepo(nil)

	return nil
}

// CreateEndorsementFunc describes a function for creating an endorsement for the given push note
type CreateEndorsementFunc func(validatorKey *crypto2.Key, note pushtypes.PushNote) (*pushtypes.PushEndorsement, error)

// createEndorsement creates a push endorsement
func createEndorsement(validatorKey *crypto2.Key, note pushtypes.PushNote) (*pushtypes.PushEndorsement, error) {

	var err error

	e := &pushtypes.PushEndorsement{
		NoteID:         note.ID().Bytes(),
		EndorserPubKey: validatorKey.PubKey().MustBytes32(),
	}

	// Set the hash of the endorsement equal the local hash of the reference
	for _, ref := range note.GetPushedReferences() {
		end := &pushtypes.EndorsedReference{}
		end.Hash = util.MustFromHex(ref.OldHash)
		e.References = append(e.References, end)
	}

	// Sign the endorsement using our BLS key
	e.SigBLS, err = validatorKey.PrivKey().BLSKey().Sign(e.BytesForBLSSig())
	if err != nil {
		return nil, errors.Wrap(err, "bls signing failed")
	}

	return e, nil
}
