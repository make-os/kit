package server

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/thoas/go-funk"
	"gitlab.com/makeos/mosdef/crypto/bls"
	dhttypes "gitlab.com/makeos/mosdef/dht/types"
	"gitlab.com/makeos/mosdef/params"
	"gitlab.com/makeos/mosdef/remote/plumbing"
	"gitlab.com/makeos/mosdef/remote/push"
	pushtypes "gitlab.com/makeos/mosdef/remote/push/types"
	rr "gitlab.com/makeos/mosdef/remote/repo"
	"gitlab.com/makeos/mosdef/remote/validation"
	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/types/txns"

	"github.com/pkg/errors"
	"github.com/tendermint/tendermint/p2p"
	"gitlab.com/makeos/mosdef/util"
	"gopkg.in/src-d/go-git.v4"
)

// Receive implements Reactor
func (sv *Server) Receive(chID byte, peer p2p.Peer, msgBytes []byte) {
	switch chID {
	case PushNoteReactorChannel:
		if err := sv.onPushNote(peer, msgBytes); err != nil {
			sv.log.Error("failed to handle push note", "Err", err.Error())
		}
	case PushEndReactorChannel:
		if err := sv.onPushEnd(peer, msgBytes); err != nil {
			sv.log.Error("failed to handle push endorsement", "Err", err.Error())
		}
	}
}

// onPushNote is the handler for incoming PushNotice messages
func (sv *Server) onPushNote(peer p2p.Peer, msgBytes []byte) error {

	// Exit if the node is in validator mode
	if sv.cfg.IsValidatorNode() {
		return nil
	}

	// Attempt to decode message to PushNotice
	var note pushtypes.PushNote
	if err := util.ToObject(msgBytes, &note); err != nil {
		return errors.Wrap(err, "failed to decoded message")
	}

	peerID := peer.ID()
	sv.log.Debug("Received a push note", "PeerID", peerID, "ID", note.ID().String())

	repoName := note.GetRepoName()
	repoPath := sv.getRepoPath(repoName)
	repoState := sv.logic.RepoKeeper().Get(repoName)

	// Ensure target repository exists
	if repoState.IsNil() {
		return fmt.Errorf("repo '%s' not found", repoName)
	}

	// If namespace is set, get it and ensure it exists
	var namespace *state.Namespace
	if note.Namespace != "" {
		namespace = sv.logic.NamespaceKeeper().Get(util.HashNamespace(note.Namespace))
		if namespace.IsNil() {
			return fmt.Errorf("namespace '%s' not found", note.Namespace)
		}
	}

	// Reconstruct references transaction details from push note
	txDetails := validation.GetTxDetailsFromNote(&note)

	// Perform authorization check
	polEnforcer, err := sv.authenticate(txDetails, repoState, namespace, sv.logic, validation.CheckTxDetail)
	if err != nil {
		return errors.Wrap(err, "authorization failed")
	}

	// Open the repo
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to open repo '%s'", repoName))
	}

	// Register a cache entry that indicates the sender of the push note
	sv.cacheNoteSender(string(peerID), note.ID().String())

	// Set the target repository object
	note.TargetRepo = &rr.Repo{
		Name:          repoName,
		Repository:    repo,
		LiteGit:       rr.NewLiteGit(sv.gitBinPath, repoPath),
		Path:          repoPath,
		NamespaceName: note.Namespace,
		State:         repoState,
		Namespace:     namespace,
	}

	// Validate the push note.
	// Downloads the git objects, performs sanity and consistency checks on the
	// push note. Does not check if the push note can extend the repository
	// without issue.
	if err := sv.checkPushNote(&note, sv.dht, sv.logic); err != nil {
		return errors.Wrap(err, "failed push note validation")
	}

	// Create the packfile
	packfile, err := sv.packfileMaker(note.TargetRepo, &note)
	if err != nil {
		return errors.Wrap(err, "failed to create packfile from push note")
	}

	// Create the git-receive-pack command
	args := []string{"receive-pack", "--stateless-rpc", repoPath}
	cmd := exec.Command(sv.gitBinPath, args...)
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
	pushHandler := sv.makePushHandler(note.TargetRepo, txDetails, polEnforcer)
	if err := pushHandler.HandleStream(packfile, in); err != nil {
		return errors.Wrap(err, "HandleStream error")
	}

	// Handle transaction validation and revert changes
	err = pushHandler.HandleReferences()
	if err != nil {
		return errors.Wrap(err, "HandleReferences error")
	}

	if err := cmd.Wait(); err != nil {
		return errors.Wrap(err, "failed to process packfile derived from push note")
	}

	// Add the note to the push pool
	if err := sv.GetPushPool().Add(&note, true); err != nil {
		return errors.Wrap(err, "failed to add push note to push pool")
	}

	// Announce the objects and push note
	sv.log.Info("Added valid push note to push pool", "ID", note.ID().String())

	// Broadcast the push note and pushed objects
	go sv.pushedObjectsBroadcaster(&note)

	return nil
}

// pushedObjectsBroadcaster describes an object for broadcasting pushed objects
type pushedObjectsBroadcaster func(pn *pushtypes.PushNote) (err error)

func (sv *Server) broadcastPushedObjects(pn *pushtypes.PushNote) (err error) {

	// Announce all pushed objects to the DHT
	for _, hash := range pn.GetPushedObjects() {
		dhtKey := plumbing.MakeRepoObjectDHTKey(pn.RepoName, hash)
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		if err := sv.GetDHT().Announce(ctx, []byte(dhtKey)); err != nil {
			err = fmt.Errorf("unable to announce git object")
			sv.log.Warn(err.Error())
			cancel()
			continue
		}
		cancel()
	}

	// Broadcast the push note and an endorse if this note is a host
	if err = sv.BroadcastPushObjects(pn); err != nil {
		sv.log.Error("Failed to broadcast push note", "Err", err)
	}

	return
}

// onPushEnd is the handler for incoming PushEndorsement messages
func (sv *Server) onPushEnd(peer p2p.Peer, msgBytes []byte) error {

	// Return if in validator mode.
	if sv.cfg.IsValidatorNode() {
		return nil
	}

	// Attempt to decode message to PushEndorsement object
	var pushEnd pushtypes.PushEndorsement
	if err := util.ToObject(msgBytes, &pushEnd); err != nil {
		return errors.Wrap(err, "failed to decoded message")
	}

	// Validate the PushEndorsement object
	pushEndID := pushEnd.ID().String()
	if err := validation.CheckPushEnd(&pushEnd, sv.logic, -1); err != nil {
		sv.log.Debug("Received an invalid push endorsement", "ID", pushEndID, "Err", err)
		return err
	}

	sv.log.Debug("Received a valid push endorsement", "PeerID", peer.ID(), "ID", pushEndID)

	// Cache the sender so we don't broadcast same PushEndorsement to it later
	sv.cachePushEndSender(string(peer.ID()), pushEndID)

	// cache the PushEndorsement object as an endorsement of the PushNotice
	sv.addPushNoteEndorsement(pushEnd.NoteID.HexStr(), &pushEnd)

	// Attempt to create an send a PushTx to the transaction pool
	if err := sv.MaybeCreatePushTx(pushEnd.NoteID.HexStr()); err != nil {
		sv.Log().Debug(err.Error())
	}

	// Broadcast the PushEndorsement to peers
	sv.broadcastPushEnd(&pushEnd)

	return nil
}

// BroadcastPushObjects broadcasts repo push notes and PushEndorsement; PushEndorsement is only
// created and broadcast only if the node is a top host.
func (sv *Server) BroadcastPushObjects(note pushtypes.PushNotice) error {

	// Broadcast the push note to peers
	sv.broadcastPushNote(note)

	// Get the top hosts
	topHosts, err := sv.logic.GetTicketManager().GetTopHosts(params.NumTopHostsLimit)
	if err != nil {
		return errors.Wrap(err, "failed to get top hosts")
	}

	// Exit with nil if node is not among the top hosts
	if !topHosts.Has(sv.privValidatorKey.PubKey().MustBytes32()) {
		return nil
	}

	// At this point, the node is a top host, so we create, sign and broadcast a PushEndorsement
	pushEnd, err := sv.createEndorsement(note)
	if err != nil {
		return err
	}
	sv.broadcastPushEnd(pushEnd)

	// Cache the PushEndorsement object as an endorsement of the PushNotice so can use it
	// to create a PushTx when enough push endorsements are discovered.
	sv.addPushNoteEndorsement(note.ID().String(), pushEnd)

	// Attempt to create a PushTx and send to the transaction pool
	if err = sv.MaybeCreatePushTx(pushEnd.NoteID.HexStr()); err != nil {
		sv.Log().Debug(err.Error())
	}

	return nil
}

// createEndorsement creates a PushEndorsement for a push note
func (sv *Server) createEndorsement(note pushtypes.PushNotice) (*pushtypes.PushEndorsement, error) {

	pe := &pushtypes.PushEndorsement{}
	pe.NoteID = note.ID()
	pe.EndorserPubKey = util.BytesToBytes32(sv.privValidatorKey.PubKey().MustBytes())

	// Set the hash of the endorsement equal the local hash of the reference
	for _, pushedRef := range note.GetPushedReferences() {
		endorsement := &pushtypes.EndorsedReference{}

		// Get the current reference hash
		refHash, err := note.GetTargetRepo().RefGet(pushedRef.Name)
		if err != nil && err != plumbing.ErrRefNotFound {
			return nil, errors.Wrap(err, fmt.Sprintf("failed to get hash of reference (%s)", pushedRef.Name))
		}

		if err == nil {
			endorsement.Hash = util.MustFromHex(refHash)
		}

		pe.References = append(pe.References, endorsement)
	}

	// Sign the endorsement using our BLS key
	sig, _ := sv.privValidatorKey.PrivKey().BLSKey().Sign(pe.BytesNoSigAndSenderPubKey())
	pe.Sig = util.BytesToBytes64(sig)

	return pe, nil
}

// broadcastPushNote broadcast push transaction to peers.
// It will not send to original sender of the push note.
func (sv *Server) broadcastPushNote(pushNote pushtypes.PushNotice) {
	for _, peer := range sv.Switch.Peers().List() {
		bz, id := pushNote.BytesAndID()
		if sv.isPushNoteSender(string(peer.ID()), id.String()) {
			continue
		}
		if peer.Send(PushNoteReactorChannel, bz) {
			sv.log.Debug("Sent push notification to peer", "PeerID", peer.ID(), "TxID", id)
		}
	}
}

// broadcastPushEnd sends out push endorsements (PushEndorsement) to peers
func (sv *Server) broadcastPushEnd(pushEnd core.RepoPushEndorsement) {
	for _, peer := range sv.Switch.Peers().List() {
		bz, id := pushEnd.BytesAndID()
		if sv.isPushEndSender(string(peer.ID()), id.String()) {
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

// MaybeCreatePushTx attempts to create a PushTx from a given push note, only if
// a push note matching the given id exist in the push pool and the push note
// has received a quorum PushEndorsement.
func (sv *Server) MaybeCreatePushTx(noteID string) error {

	// Get the list of push endorsements received for the push note
	endorsements := sv.pushEndorsements.Get(noteID)
	if endorsements == nil {
		return fmt.Errorf("no endorsements yet")
	}

	// Ensure there are enough push endorsements
	endorsementIdx := endorsements.(map[string]*pushtypes.PushEndorsement)
	if len(endorsementIdx) < params.PushEndorseQuorumSize {
		return fmt.Errorf("not enough push endorsements to satisfy quorum size")
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

	// Collect the BLS public keys of all PushEndorsement senders.
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
		endorsementsSig = append(endorsementsSig, ed.Sig.Bytes())

		// Clone the endorsement and replace endorsement at i.
		// Also clear the signature as it will no longer be useful
		noteEndorsements[i] = ed.Clone()
		noteEndorsements[i].Sig = util.EmptyBytes64
	}

	// Create a new push transaction
	pushTx := txns.NewBareTxPush()

	// Set push note and endorsements
	pushTx.PushNote = note
	pushTx.PushEnds = noteEndorsements

	// Generate and set aggregated BLS signature
	aggSig, err := bls.AggregateSignatures(endorsementsPubKey, endorsementsSig)
	if err != nil {
		return errors.Wrap(err, "unable to create aggregated signature")
	}
	pushTx.AggPushEndsSig = aggSig

	// Register push transaction to mempool
	if err := sv.GetMempool().Add(pushTx); err != nil {
		return errors.Wrap(err, "failed to add push tx to mempool")
	}

	pushTx.PushNote.SetTargetRepo(nil)

	return nil
}

// updateWithPushTx updates a repository using a push transaction
func (sv *Server) updateWithPushTx(tx *txns.TxPush) error {

	repoPath := sv.getRepoPath(tx.PushNote.GetRepoName())

	// Get the repository
	repo, err := rr.Get(repoPath)
	if err != nil {
		return err
	}

	// Create a reference update request packfile from the push note
	packfile, err := push.MakeReferenceUpdateRequest(repo, tx.PushNote)
	if err != nil {
		return errors.Wrap(err, "failed to create packfile")
	}

	// Create the git-receive-pack command
	args := []string{"receive-pack", "--stateless-rpc", repoPath}
	cmd := exec.Command(sv.gitBinPath, args...)
	cmd.Dir = repoPath

	// Get the command's stdin pipe
	in, err := cmd.StdinPipe()
	if err != nil {
		return errors.Wrap(err, "failed to get stdin pipe")
	}
	defer in.Close()

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

	io.Copy(in, packfile)

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("git exec failed")
	}

	sv.Log().Debug("Updated repo with push transaction",
		"RepoContext", tx.PushNote.GetRepoName(), "TxID", tx.GetID())

	return nil
}

// UpdateRepoWithTxPush attempts to update a repository using a push transaction
func (sv *Server) UpdateRepoWithTxPush(tx types.BaseTx) error {

	if sv.cfg.IsValidatorNode() {
		return nil
	}

	if err := sv.updateWithPushTx(tx.(*txns.TxPush)); err != nil {
		sv.Log().Error("failed to process push transaction", "Err", err)
		return err
	}

	return nil
}

// ExecTxPush executes a push transaction
func (sv *Server) ExecTxPush(tx types.BaseTx) error {
	return execTxPush(sv, tx.(*txns.TxPush))
}

// execTxPush executes a push transaction
func execTxPush(m core.RemoteServer, tx *txns.TxPush) error {

	if m.Cfg().IsValidatorNode() {
		return nil
	}

	repoName := tx.PushNote.GetRepoName()
	repo, err := m.GetRepo(repoName)
	if err != nil {
		return errors.Wrap(err, "unable to find repo locally")
	}

	for _, objHash := range tx.PushNote.GetPushedObjects() {
		if repo.ObjectExist(objHash) {
			continue
		}

		// Fetch the object from the dht
		dhtKey := plumbing.MakeRepoObjectDHTKey(repoName, objHash)
		ctx, cn := context.WithTimeout(context.Background(), 60*time.Second)
		query := &dhttypes.DHTObjectQuery{Module: core.RepoObjectModule, ObjectKey: []byte(dhtKey)}
		objValue, err := m.GetDHT().GetObject(ctx, query)
		if err != nil {
			cn()
			msg := fmt.Sprintf("failed to fetch object '%s'", objHash)
			return errors.Wrap(err, msg)
		}
		cn()

		// Write fetched object to the repo
		if err = repo.WriteObjectToFile(objHash, objValue); err != nil {
			msg := fmt.Sprintf("failed to write fetched object '%s' to disk",
				objHash)
			return errors.Wrap(err, msg)
		}

		// Announce ourselves as the newest provider of the object
		if err := m.GetDHT().Announce(ctx, []byte(dhtKey)); err != nil {
			m.Log().Warn("unable to announce git object", "Err", err)
			continue
		}

		m.Log().Debug("Fetched object for repo", "ObjHash", objHash, "RepoName", repoName)
	}

	// Attempt to update the local repository using the push transaction
	if err = m.UpdateRepoWithTxPush(tx); err != nil {
		return err
	}

	// If delete request and we are not a validator, delete the reference from the local repo.
	for _, ref := range tx.PushNote.GetPushedReferences() {
		if plumbing.IsZeroHash(ref.NewHash) {
			if err = repo.RefDelete(ref.Name); err != nil {
				return errors.Wrapf(err, "failed to delete reference (%s)", ref.Name)
			}
		}
	}

	return nil
}
