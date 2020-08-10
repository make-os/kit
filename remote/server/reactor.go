package server

import (
	"fmt"
	"io"
	"os/exec"

	"github.com/pkg/errors"
	"github.com/tendermint/tendermint/p2p"
	crypto2 "github.com/themakeos/lobe/crypto"
	"github.com/themakeos/lobe/crypto/bls"
	"github.com/themakeos/lobe/params"
	"github.com/themakeos/lobe/remote/plumbing"
	"github.com/themakeos/lobe/remote/policy"
	"github.com/themakeos/lobe/remote/push"
	pushtypes "github.com/themakeos/lobe/remote/push/types"
	rr "github.com/themakeos/lobe/remote/repo"
	remotetypes "github.com/themakeos/lobe/remote/types"
	"github.com/themakeos/lobe/remote/validation"
	"github.com/themakeos/lobe/types/state"
	"github.com/themakeos/lobe/types/txns"
	"github.com/themakeos/lobe/util"
	"github.com/themakeos/lobe/util/crypto"
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

// onPushNoteReceived is the handler for incoming Note messages
func (sv *Server) onPushNoteReceived(peer p2p.Peer, msgBytes []byte) error {

	// Exit if the node is in validator mode
	if sv.cfg.IsValidatorNode() {
		return nil
	}

	// Attempt to decode message to PushNote
	var note pushtypes.Note
	if err := util.ToObject(msgBytes, &note); err != nil {
		return errors.Wrap(err, "failed to decoded message")
	}
	note.FromRemotePeer = true

	peerID := peer.ID()
	repoName := note.GetRepoName()
	repoPath := sv.getRepoPath(repoName)
	repoState := sv.logic.RepoKeeper().Get(repoName)

	sv.log.Debug("Received a push note", "PeerID", peerID, "ID", note.ID().String())

	// Ensure target repository exists
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

	// Reconstruct references transaction details from push note
	txDetails := validation.GetTxDetailsFromNote(&note)

	// Perform authentication check
	polEnforcer, err := sv.authenticate(txDetails, repoState, namespace, sv.logic, validation.CheckTxDetail)
	if err != nil {
		return errors.Wrap(err, "authorization failed")
	}

	// Validate the push note.
	if err := sv.checkPushNote(&note, sv.logic); err != nil {
		return errors.Wrap(err, "failed push note validation")
	}

	// Register a cache entry that indicates the sender of the push note
	sv.registerNoteSender(string(peerID), note.ID().String())

	// For each objects fetched:
	// - Broadcast commit and tag objects.
	sv.objfetcher.OnPackReceived(func(hash string, packfile io.ReadSeeker) {
		plumbing.UnpackPackfile(packfile, func(header *packfile2.ObjectHeader, read func() (object.Object, error)) error {
			obj, _ := read()
			if obj.Type() == plumbing2.CommitObject || obj.Type() == plumbing2.TagObject {
				objHash := obj.ID()
				sv.AnnounceObject(objHash[:], nil)
			}
			return nil
		})
	})

	// Fetch the objects for each references in the push note.
	// The callback is called when all objects have been fetched successfully.
	sv.objfetcher.Fetch(&note, func(err error) {
		sv.onFetch(err, &note, txDetails, polEnforcer)
	})

	return nil
}

// onFetch is called after all objects of the push note have been
// completely fetched or an error occurred while fetching.
func (sv *Server) onFetch(
	err error,
	note pushtypes.PushNote,
	txDetails []*remotetypes.TxDetail,
	polEnforcer policy.EnforcerFunc) error {

	if err != nil {
		sv.log.Error("Failed to fetch all note objects", "ID", note.ID().String(), "Err", err.Error())
		return err
	}

	noteID := note.ID().String()
	repoName := note.GetRepoName()

	// Get the size of the pushed update objects. This is the size of the objects required
	// to bring the local reference up to the state of the note's pushed reference.
	localSize, err := push.GetSizeOfObjects(note)
	if err != nil {
		sv.log.Error("Failed to get size of pushed refs objects", "Err", err.Error(), "Repo", repoName)
		return errors.Wrapf(err, "failed to get pushed refs objects size")
	}
	note.SetLocalSize(localSize)

	// Verify the note's size ensuring it matches the local size
	// TODO: Penalize remote node for the inconsistency
	noteSize := note.GetSize()
	if note.IsFromRemotePeer() && noteSize != localSize {
		sv.log.Error("Note's size does not match local size", "ID",
			noteID, "Size", noteSize, "LocalSize", localSize, "Repo", repoName)
		return fmt.Errorf("note's objects size and local size differs")
	}

	// Attempt to process the push note
	if err = sv.processPushNote(note, txDetails, polEnforcer); err != nil {
		sv.log.Debug("Failed to process push note", "ID", noteID, "Err", err.Error())
		return err
	}

	return nil
}

// PushNoteProcessor is a function for processing a push note
type PushNoteProcessor func(
	note pushtypes.PushNote,
	txDetails []*remotetypes.TxDetail,
	polEnforcer policy.EnforcerFunc) error

// maybeProcessPushNote validates and dry-run the push note.
// It expects the pushed objects to be present in the target repository.
func (sv *Server) maybeProcessPushNote(
	note pushtypes.PushNote,
	txDetails []*remotetypes.TxDetail,
	polEnforcer policy.EnforcerFunc) error {

	// Create a packfile that represents updates described in the note.
	updatePackfile, err := sv.makeReferenceUpdatePack(note)
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

	// Get the command's stdout pipe
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "failed to get stdout pipe")
	}
	defer stdout.Close()

	// start the command
	err = cmd.Start()
	if err != nil {
		return errors.Wrap(err, "failed to start git-receive-pack command")
	}

	// Read, analyse and pass the packfile to git
	pushHandler := sv.makePushHandler(note.GetTargetRepo(), txDetails, polEnforcer)
	pushHandler.SetGitReceivePackCmd(cmd)
	if err := pushHandler.HandleStream(updatePackfile, in); err != nil {
		return errors.Wrap(err, "HandleStream error")
	}

	// Handle transaction validation and revert changes
	err = pushHandler.HandleReferences()
	if err != nil {
		return errors.Wrap(err, "HandleReferences error")
	}

	// Add the note to the push pool
	if err := sv.GetPushPool().Add(note, true); err != nil {
		return errors.Wrap(err, "failed to add push note to push pool")
	}

	sv.log.Info("Added valid push note to push pool", "ID", note.ID().String())

	// Broadcast the push note and pushed objects
	if err = sv.noteAndEndorserBroadcaster(note); err != nil {
		sv.log.Error("Failed to broadcast push note", "Err", err)
	}

	return nil
}

// onEndorsementReceived is the handler for incoming Endorsement messages
func (sv *Server) onEndorsementReceived(peer p2p.Peer, msgBytes []byte) error {

	// Return if in validator mode.
	if sv.cfg.IsValidatorNode() {
		return nil
	}

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

	peerID := peer.ID()
	sv.log.Debug("Received a valid push endorsement", "PeerID", peerID, "ID", endID)

	// Cache the sender so we don't broadcast same Endorsement to it later
	sv.registerEndorsementSender(string(peerID), endID)

	// cache the Endorsement object as an endorsement of the PushNote
	sv.registerEndorsementOfNote(endorsement.NoteID.HexStr(), &endorsement)

	// Attempt to create an send a PushTx to the transaction pool
	if err := sv.makePushTx(endorsement.NoteID.HexStr()); err != nil {
		sv.Log().Debug("Unable to create push transaction", "Reason", err.Error())
	}

	// Broadcast the Endorsement to peers
	sv.endorsementBroadcaster(&endorsement)

	return nil
}

// PushNoteAndEndorsementBroadcaster describes a function for broadcasting a push
// note and an endorsement of it.
type PushNoteAndEndorsementBroadcaster func(note pushtypes.PushNote) error

// BroadcastNoteAndEndorsement broadcasts a push note and an endorsement of it.
// The node has to be a top host to broadcast an endorsement.
func (sv *Server) BroadcastNoteAndEndorsement(note pushtypes.PushNote) error {

	// Broadcast the push note to peers
	sv.noteBroadcaster(note)

	// Get the top hosts
	topHosts, err := sv.logic.GetTicketManager().GetTopHosts(params.NumTopHostsLimit)
	if err != nil {
		return errors.Wrap(err, "failed to get top hosts")
	}

	// Exit with nil if node is not among the top hosts
	if !topHosts.Has(sv.validatorKey.PubKey().MustBytes32()) {
		return nil
	}

	// At this point, the node is a top host, create a signed endorsement
	endorsement, err := sv.endorsementCreator(sv.validatorKey, note)
	if err != nil {
		return err
	}

	// Broadcast the endorsement
	sv.endorsementBroadcaster(endorsement)

	// Cache the Endorsement object as an endorsement of the PushNote so can use it
	// to create a mempool-bound push transaction when enough endorsements are discovered.
	sv.registerEndorsementOfNote(note.ID().String(), endorsement)

	// Attempt to create a PushTx and send to the transaction pool
	if err = sv.makePushTx(endorsement.NoteID.HexStr()); err != nil {
		sv.Log().Debug("Unable to create push transaction", "Reason", err.Error())
	}

	return nil
}

// NoteBroadcaster describes a function for broadcasting a push note
type NoteBroadcaster func(pushNote pushtypes.PushNote)

// broadcastPushNote broadcast push transaction to peers.
// It will not send to original sender of the push note.
func (sv *Server) broadcastPushNote(pushNote pushtypes.PushNote) {
	for _, peer := range sv.Switch.Peers().List() {
		bz, id := pushNote.BytesAndID()
		if sv.isNoteSender(string(peer.ID()), id.String()) {
			continue
		}
		if peer.Send(PushNoteReactorChannel, bz) {
			sv.log.Debug("Sent push note to peer", "PeerID", peer.ID(), "ID", id)
		}
	}
}

// EndorsementBroadcaster describes a function for broadcasting endorsement
type EndorsementBroadcaster func(endorsement pushtypes.Endorsement)

// broadcastEndorsement sends out push endorsements (Endorsement) to peers
func (sv *Server) broadcastEndorsement(endorsement pushtypes.Endorsement) {
	for _, peer := range sv.Switch.Peers().List() {
		bz, id := endorsement.BytesAndID()
		if sv.isEndorsementSender(string(peer.ID()), id.String()) {
			continue
		}
		if peer.Send(PushEndReactorChannel, bz) {
			sv.log.Debug("Sent push endorsement to peer", "PeerID", peer.ID(), "TxID", id)
		}
	}
}

// BroadcastMsg broadcast messages to peers
func (sv *Server) BroadcastMsg(ch byte, msg []byte) {
	for _, peer := range sv.Switch.Peers().List() {
		peer.Send(ch, msg)
	}
}

// GetChannels implements Reactor.
func (sv *Server) GetChannels() []*p2p.ChannelDescriptor {
	return []*p2p.ChannelDescriptor{
		{ID: PushNoteReactorChannel, Priority: 5},
		{ID: PushEndReactorChannel, Priority: 5},
	}
}

// PushTxCreator describes a function that takes a push note and creates
// a push transaction which is then added to the mempool.
type PushTxCreator func(noteID string) error

// createPushTx attempts to create a PushTx from a given push note, only if
// a push note matching the given id exist in the push pool and the push note
// has received a quorum Endorsement.
func (sv *Server) createPushTx(noteID string) error {

	// Get the list of push endorsements received for the push note
	endorsements := sv.endorsementsReceived.Get(noteID)
	if endorsements == nil {
		return fmt.Errorf("no endorsements yet")
	}

	// Ensure there are enough push endorsements
	endorsementIdx := endorsements.(map[string]*pushtypes.PushEndorsement)
	if len(endorsementIdx) < params.PushEndorseQuorumSize {
		return fmt.Errorf("cannot create push transaction; note has %d endorsements, wants %d",
			len(endorsementIdx), params.PushEndorseQuorumSize)
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
	var endorsementsPubKey []*bls.PublicKey
	var endorsementsSig [][]byte
	for i, ed := range noteEndorsements {

		// Get the selected ticket of the endorsers
		selTicket := hosts.Get(ed.EndorserPubKey)
		if selTicket == nil {
			return fmt.Errorf("endorsement[%d]: ticket not found in top hosts list", i)
		}

		// Get their BLS public key from the ticket
		pk, err := bls.BytesToPublicKey(selTicket.Ticket.BLSPubKey)
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
	aggSig, err := bls.AggregateSignatures(endorsementsPubKey, endorsementsSig)
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

// EndorsementCreator describes a function for creating an endorsement for the given push note
type EndorsementCreator func(validatorKey *crypto2.Key, note pushtypes.PushNote) (*pushtypes.PushEndorsement, error)

// createEndorsement creates a push endorsement
func createEndorsement(validatorKey *crypto2.Key, note pushtypes.PushNote) (*pushtypes.PushEndorsement, error) {

	var err error

	e := &pushtypes.PushEndorsement{
		NoteID:         note.ID().Bytes(),
		EndorserPubKey: validatorKey.PubKey().MustBytes32(),
	}

	// Set the hash of the endorsement equal the local hash of the reference
	for _, ref := range note.GetPushedReferences() {
		hash, err := note.GetTargetRepo().RefGet(ref.Name)
		if err != nil && err != plumbing.ErrRefNotFound {
			return nil, errors.Wrap(err, fmt.Sprintf("failed to get hash of reference (%s)", ref.Name))
		}

		ref := &pushtypes.EndorsedReference{}
		e.References = append(e.References, ref)
		if err == nil {
			ref.Hash = util.MustFromHex(hash)
		}
	}

	// Sign the endorsement using our BLS key
	e.SigBLS, err = validatorKey.PrivKey().BLSKey().Sign(e.BytesForBLSSig())
	if err != nil {
		return nil, errors.Wrap(err, "bls signing failed")
	}

	return e, nil
}
