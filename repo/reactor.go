package repo

import (
	"context"
	"fmt"
	"gitlab.com/makeos/mosdef/dht/types"
	"gitlab.com/makeos/mosdef/types/core"
	"io"
	"os/exec"
	"time"

	"github.com/thoas/go-funk"
	"gitlab.com/makeos/mosdef/crypto/bls"
	"gitlab.com/makeos/mosdef/params"

	"github.com/pkg/errors"
	"github.com/tendermint/tendermint/p2p"
	"gitlab.com/makeos/mosdef/util"
	"gopkg.in/src-d/go-git.v4"
)

// Receive implements Reactor
func (m *Manager) Receive(chID byte, peer p2p.Peer, msgBytes []byte) {
	switch chID {
	case PushNoteReactorChannel:
		if err := m.onPushNote(peer, msgBytes); err != nil {
			m.log.Error(err.Error())
		}
	case PushOKReactorChannel:
		if err := m.onPushOK(peer, msgBytes); err != nil {
			m.log.Error(err.Error())
		}
	}
}

// onPushNote is the handler for incoming PushNote messages
func (m *Manager) onPushNote(peer p2p.Peer, msgBytes []byte) error {

	// Exit if the node is in validator mode
	if m.cfg.IsValidatorNode() {
		return nil
	}

	// Attempt to decode message to PushNote
	var pn core.PushNote
	if err := util.BytesToObject(msgBytes, &pn); err != nil {
		return errors.Wrap(err, "failed to decoded message")
	}

	m.log.Debug("Received push notification from peer",
		"PeerID", peer.ID(), "TxID", pn.ID().String())

	repoName := pn.GetRepoName()
	repoPath := m.getRepoPath(repoName)

	// Get the repository's state object
	repoState := m.logic.RepoKeeper().GetRepo(repoName)
	if repoState.IsNil() {
		return fmt.Errorf("repo '%s' not found", repoName)
	}

	// Open the repo
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to open repo '%s'", repoName))
	}

	// Add a cache entry that indicates the sender of the push note
	m.cachePushNoteSender(string(peer.ID()), pn.ID().String())

	// Set the target repository object
	pn.TargetRepo = &Repo{
		name:  repoName,
		git:   repo,
		ops:   NewGitOps(m.gitBinPath, repoPath),
		path:  repoPath,
		state: repoState,
	}

	defer m.pruner.Schedule(pn.GetRepoName())

	// Validate the push note.
	// Downloads the git objects, performs sanity and consistency checks on the
	// push note. Does not check if the push note can extend the repository
	// without issue.
	// NOTE: If in validator mode, only sanity check is performed.
	if err := checkPushNote(&pn, m.dht, m.logic); err != nil {
		return errors.Wrap(err, "failed push note validation")
	}

	// ------------------------------------------------------------------------
	// At this point, we know that the push note is valid and its proposed
	// reference updates are consistent with the state of the repository,
	// but we need to also check that the proposed references and objects are
	// well signed, have correct transaction information and can update the
	// state of the repository on disk without issue.
	// To do this, we create a packfile from the push tx and attempt to let
	// git-receive-pack process it.
	// ------------------------------------------------------------------------

	// Create the pack file
	packfile, err := makeReferenceUpdateRequest(pn.TargetRepo, &pn)
	if err != nil {
		return errors.Wrap(err, "failed to create packfile from push note")
	}

	// Create the git-receive-pack command
	args := []string{"receive-pack", "--stateless-rpc", repoPath}
	cmd := exec.Command(m.gitBinPath, args...)
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
	pushHandler := newPushHandler(pn.TargetRepo, m)
	if err := pushHandler.HandleStream(packfile, in); err != nil {
		return errors.Wrap(err, "HandleStream error")
	}

	// Handle transaction validation and revert pre-commit changes
	refsTxParams, _, err := pushHandler.HandleValidateAndRevert()
	if err != nil {
		return errors.Wrap(err, "HandleValidateAndRevert error")
	}

	if err := cmd.Wait(); err != nil {
		return errors.Wrap(err, "failed to process packfile derived from push note")
	}

	// Verify that push note is consistent with the txparamss
	if err := checkPushNoteAgainstTxParamss(&pn, refsTxParams); err != nil {
		return errors.Wrapf(err, "push note and txparams conflict")
	}

	// At this point, the transaction has passed all validation and
	// compatibility checks. We can now attempt to add the push note to the
	// PushPool without validation
	if err := m.GetPushPool().Add(&pn, true); err != nil {
		return errors.Wrap(err, "failed to add push note to push pool")
	}

	// Announce the objects of the push note to the dht
	for _, hash := range pn.GetPushedObjects(false) {
		dhtKey := MakeRepoObjectDHTKey(repoName, hash)
		ctx, c := context.WithTimeout(context.Background(), 60*time.Second)
		defer c()
		if err := m.GetDHT().Announce(ctx, []byte(dhtKey)); err != nil {
			m.log.Warn("unable to announce git object", "Err", err)
			continue
		}
	}

	if err = m.BroadcastPushObjects(&pn); err != nil {
		m.log.Error("Error broadcasting push objects", "Err", err)
	}

	m.log.Info("Added valid push note to push pool", "TxID", pn.ID().String())

	return nil
}

// onPushOK is the handler for incoming PushOK messages
func (m *Manager) onPushOK(peer p2p.Peer, msgBytes []byte) error {

	// Return if in validator mode.
	if m.cfg.IsValidatorNode() {
		return nil
	}

	// Attempt to decode message to PushOK object
	var pok core.PushOK
	if err := util.BytesToObject(msgBytes, &pok); err != nil {
		return errors.Wrap(err, "failed to decoded message")
	}

	// Validate the PushOK object
	pokID := pok.ID().String()
	if err := checkPushOK(&pok, m.logic, -1); err != nil {
		m.log.Debug("Received an invalid push endorsement", "ID", pokID, "Err", err)
		return err
	}

	m.log.Debug("Received a valid push endorsement", "PeerID", peer.ID(), "ID", pokID)

	// Cache the sender so we don't broadcast same PushOK to it later
	m.cachePushOkSender(string(peer.ID()), pokID)

	// cache the PushOK object as an endorsement of the PushNote
	m.addPushNoteEndorsement(pok.PushNoteID.HexStr(), &pok)

	// Attempt to create an send a PushTx to the transaction pool
	if err := m.MaybeCreatePushTx(pok.PushNoteID.HexStr()); err != nil {
		m.Log().Debug(err.Error())
	}

	// Broadcast the PushOK to peers
	m.broadcastPushOK(&pok)

	return nil
}

// BroadcastPushObjects broadcasts repo push notes and PushOK; PushOK is only
// created and broadcast only if the node is a top storer.
func (m *Manager) BroadcastPushObjects(pushNote core.RepoPushNote) error {

	// Broadcast the push note to peers
	m.broadcastPushNote(pushNote)

	// Get the top storers
	topStorers, err := m.logic.GetTicketManager().GetTopStorers(params.NumTopStorersLimit)
	if err != nil {
		return errors.Wrap(err, "failed to get top storers")
	}

	// Exit with nil if node is not among the top storers
	if !topStorers.Has(m.privValidatorKey.PubKey().MustBytes32()) {
		return nil
	}

	// At this point, the node is a top storer, so we create, sign and broadcast a PushOK
	pok, err := m.createPushOK(pushNote)
	if err != nil {
		return err
	}
	m.broadcastPushOK(pok)

	// Cache the PushOK object as an endorsement of the PushNote so can use it
	// to create a PushTx when enough PushOKs are discovered.
	m.addPushNoteEndorsement(pushNote.ID().String(), pok)

	// Attempt to create a PushTx and send to the transaction pool
	if err = m.MaybeCreatePushTx(pok.PushNoteID.HexStr()); err != nil {
		m.Log().Debug(err.Error())
	}

	return nil
}

// createPushOK creates a PushOK for a push note
func (m *Manager) createPushOK(pushNote core.RepoPushNote) (*core.PushOK, error) {

	pok := &core.PushOK{}
	pok.PushNoteID = pushNote.ID()
	pok.SenderPubKey = util.BytesToBytes32(m.privValidatorKey.PubKey().MustBytes())

	repo, err := GetRepo(m.getRepoPath(pushNote.GetRepoName()))
	if err != nil {
		return nil, err
	}

	// Set the state hash for every reference
	for _, pushedRef := range pushNote.GetPushedReferences() {
		refHash := &core.ReferenceHash{}
		refHash.Hash, err = repo.TreeRoot(pushedRef.Name)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("failed to get reference (%s) state hash",
				pushedRef.Name))
		}
		pok.ReferencesHash = append(pok.ReferencesHash, refHash)
	}

	// Sign the endorsement using our BLS key
	blsSig, _ := m.privValidatorKey.PrivKey().BLSKey().Sign(pok.BytesNoSigAndSenderPubKey())
	pok.Sig = util.BytesToBytes64(blsSig)

	return pok, nil
}

// broadcastPushNote broadcast push transaction to peers.
// It will not send to original sender of the push note.
func (m *Manager) broadcastPushNote(pushNote core.RepoPushNote) {
	for _, peer := range m.Switch.Peers().List() {
		bz, id := pushNote.BytesAndID()
		if m.isPushNoteSender(string(peer.ID()), id.String()) {
			continue
		}
		if peer.Send(PushNoteReactorChannel, bz) {
			m.log.Debug("Sent push notification to peer", "PeerID", peer.ID(), "TxID", id)
		}
	}
}

// broadcastPushOK sends out push endorsements (PushOK) to peers
func (m *Manager) broadcastPushOK(pushOk core.RepoPushOK) {
	for _, peer := range m.Switch.Peers().List() {
		bz, id := pushOk.BytesAndID()
		if m.isPushOKSender(string(peer.ID()), id.String()) {
			continue
		}
		if peer.Send(PushOKReactorChannel, bz) {
			m.log.Debug("Sent push endorsement to peer", "PeerID", peer.ID(), "TxID", id)
		}
	}
}

// BroadcastMsg broadcast messages to peers
func (m *Manager) BroadcastMsg(ch byte, msg []byte) {
	for _, peer := range m.Switch.Peers().List() {
		peer.Send(ch, msg)
	}
}

// GetChannels implements Reactor.
func (m *Manager) GetChannels() []*p2p.ChannelDescriptor {
	return []*p2p.ChannelDescriptor{
		{ID: PushNoteReactorChannel, Priority: 5},
		{ID: PushOKReactorChannel, Priority: 5},
	}
}

// MaybeCreatePushTx attempts to create a PushTx from a given push note, only if
// a push note matching the given id exist in the push pool and the push note
// has received a quorum PushOK.
func (m *Manager) MaybeCreatePushTx(pushNoteID string) error {

	// Get the list of PushOKs received for the push note
	notePushOKs := m.pushNoteEndorsements.Get(pushNoteID)
	if notePushOKs == nil {
		return fmt.Errorf("no endorsements yet")
	}

	// Ensure there are enough PushOKs
	pushOKIdx := notePushOKs.(map[string]*core.PushOK)
	if len(pushOKIdx) < params.PushOKQuorumSize {
		return fmt.Errorf("Not enough push endorsements to satisfy quorum size")
	}

	// Get the push note from the push pool
	note := m.GetPushPool().Get(pushNoteID)
	if note == nil {
		return fmt.Errorf("push note not found in pool")
	}

	storers, err := m.logic.GetTicketManager().GetTopStorers(params.NumTopStorersLimit)
	if err != nil {
		return errors.Wrap(err, "failed to get top storers")
	}

	// Collect the BLS public keys of all PushOK senders.
	// We need them for the construction of BLS aggregated signature.
	pushOKs := funk.Values(pushOKIdx).([]*core.PushOK)
	pokPubKeys := []*bls.PublicKey{}
	pokSigs := [][]byte{}
	for i, pok := range pushOKs {
		selTicket := storers.Get(pok.SenderPubKey)
		if selTicket == nil {
			return fmt.Errorf("endorsement[%d]: ticket not found in top storers list", i)
		}

		pk, err := bls.BytesToPublicKey(selTicket.Ticket.BLSPubKey)
		if err != nil {
			return errors.Wrapf(err, "endorsement[%d]: bls public key is invalid", i)
		}

		pokPubKeys = append(pokPubKeys, pk)
		pokSigs = append(pokSigs, pok.Sig.Bytes())
		pushOKs[i] = pok.Clone()
		pushOKs[i].Sig = util.EmptyBytes64
	}

	pushTx := core.NewBareTxPush()
	pushTx.PushNote = note
	pushTx.PushOKs = pushOKs

	// Generate aggregated BLS signature
	aggSig, err := bls.AggregateSignatures(pokPubKeys, pokSigs)
	if err != nil {
		return errors.Wrap(err, "unable to create aggregated signature")
	}
	pushTx.AggPushOKsSig = aggSig

	// Add push to mempool
	if err := m.GetMempool().Add(pushTx); err != nil {
		return errors.Wrap(err, "failed to add push tx to mempool")
	}

	pushTx.PushNote.TargetRepo = nil

	return nil
}

// mergeTxPush takes a push transaction and attempts to merge it into the
// target repository
func (m *Manager) mergeTxPush(tx *core.TxPush) error {

	repoPath := m.getRepoPath(tx.PushNote.GetRepoName())

	// Get the repository
	repo, err := GetRepo(repoPath)
	if err != nil {
		return err
	}

	// Create a reference update request packfile from the push note
	packfile, err := makeReferenceUpdateRequest(repo, tx.PushNote)
	if err != nil {
		return errors.Wrap(err, "failed to create packfile")
	}

	// Create the git-receive-pack command
	args := []string{"receive-pack", "--stateless-rpc", repoPath}
	cmd := exec.Command(m.gitBinPath, args...)
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

	m.Log().Debug("Committed pushed tx to repository permanently",
		"Repo", tx.PushNote.RepoName)

	return nil
}

// UpdateRepoWithTxPush attempts to merge a push transaction to a repository and
// also update the repository's state tree.
func (m *Manager) UpdateRepoWithTxPush(tx *core.TxPush) error {

	// Merge the push transaction to the repo only if the node is not in
	// validator mode.
	if !m.cfg.IsValidatorNode() {
		if err := m.mergeTxPush(tx); err != nil {
			m.Log().Error("failed to process push transaction", "Err", err)
			return nil
		}
	}

	// Update the repository's reference tree(s)
	pn := tx.PushNote
	refHashes, err := updateReferencesTree(pn.References, m.getRepoPath(pn.RepoName))
	if err != nil {
		m.Log().Error("Error updating repo tree", "RepoName", pn.RepoName, "Err", err)
		return err
	}

	for ref, hash := range refHashes {
		m.Log().Info("Reference state updated", "RepoName", pn.RepoName,
			"Ref", ref, "StateHash", hash)
	}

	return nil
}

// ExecTxPush executes a push transaction
func (m *Manager) ExecTxPush(tx *core.TxPush) error {
	return execTxPush(m, tx)
}

// execTxPush executes a push transaction coming from a block that is currently
// being processed.
func execTxPush(m core.RepoManager, tx *core.TxPush) error {

	repoName := tx.PushNote.RepoName
	repo, err := m.GetRepo(repoName)
	if err != nil {
		return errors.Wrap(err, "unable to find repo locally")
	}

	defer m.GetPruner().Schedule(repoName)

	// Do not download pushed objects in validator mode
	cfg := m.Cfg()
	if cfg.IsValidatorNode() {
		goto update
	}

	// Download pushed objects
	for _, objHash := range tx.PushNote.GetPushedObjects(false) {
		if repo.ObjectExist(objHash) {
			continue
		}

		// Fetch from the dht
		dhtKey := MakeRepoObjectDHTKey(repoName, objHash)
		ctx, cn := context.WithTimeout(context.Background(), 60*time.Second)
		defer cn()
		objValue, err := m.GetDHT().GetObject(ctx, &types.DHTObjectQuery{
			Module:    core.RepoObjectModule,
			ObjectKey: []byte(dhtKey),
		})
		if err != nil {
			msg := fmt.Sprintf("failed to fetch object '%s'", objHash)
			return errors.Wrap(err, msg)
		}

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

		m.Log().Debug("Fetched object for repo", "ObjHash", objHash,
			"RepoName", repoName)
	}

update:
	// Attempt to merge the push transaction to the target repo
	if err = m.UpdateRepoWithTxPush(tx); err != nil {
		return err
	}

	// For any pushed reference that has a delete directive, remove the
	// reference from the repo and also its tree.
	for _, ref := range tx.PushNote.GetPushedReferences() {
		if ref.Delete {
			if !cfg.IsValidatorNode() {
				if err = repo.RefDelete(ref.Name); err != nil {
					return errors.Wrapf(err, "failed to delete reference (%s)", ref.Name)
				}
			}
			if err := deleteReferenceTree(repo.Path(), ref.Name); err != nil {
				return errors.Wrapf(err, "failed to delete reference (%s) tree", ref.Name)
			}
		}
	}

	return nil
}
