package repo

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"time"

	"github.com/makeos/mosdef/params"

	"github.com/makeos/mosdef/crypto"

	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
	"github.com/pkg/errors"
	"github.com/tendermint/tendermint/p2p"
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

	// Attempt to decode message to PushNote
	var pn types.PushNote
	if err := util.BytesToObject(msgBytes, &pn); err != nil {
		return errors.Wrap(err, "failed to decoded message")
	}

	m.log.Debug("Received push transaction from peer",
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
		db:    NewDBOps(m.repoDBCache, repoName),
		state: repoState,
	}

	defer m.pruner.Schedule(pn.GetRepoName())

	// Validate the push note.
	// Downloads the git objects, performs sanity and consistency checks on the
	// push note. Does not check if the push note can extend the repository without issue
	if err := checkPushNote(&pn, m.logic, m.dht); err != nil {
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
	if _, _, err := pushHandler.HandleValidateAndRevert(); err != nil {
		return errors.Wrap(err, "HandleValidateAndRevert error")
	}

	if err := cmd.Wait(); err != nil {
		return errors.Wrap(err, "failed to process packfile derived from push note")
	}

	// At this point, the transaction has passed all validation and
	// compatibility checks. We can now attempt to add the push note to the
	// PushPool without validation
	if err := m.GetPushPool().Add(&pn, true); err != nil {
		return errors.Wrap(err, "failed to add push note to push pool")
	}

	// Announce the objects of the push note to the dht
	for _, hash := range pn.GetPushedObjects() {
		dhtKey := MakeRepoObjectDHTKey(repoName, hash)
		ctx, c := context.WithTimeout(context.Background(), 60*time.Second)
		defer c()
		if err := m.GetDHT().Annonce(ctx, []byte(dhtKey)); err != nil {
			m.log.Error("unable to announce git object", "Err", err)
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

	// Attempt to decode message to PushOK
	var pok types.PushOK
	if err := util.BytesToObject(msgBytes, &pok); err != nil {
		return errors.Wrap(err, "failed to decoded message")
	}

	m.log.Debug("Received push OK from peer", "PeerID", peer.ID(), "TxID", pok.ID().String())

	m.cachePushOkSender(string(peer.ID()), pok.ID().String())

	// Verify the signature
	spk, err := crypto.PubKeyFromBytes(pok.SenderPubKey.Bytes())
	if err != nil {
		return errors.Wrap(err, "push ok sender public key is invalid")
	}
	ok, err := spk.Verify(pok.BytesNoSig(), pok.Sig.Bytes())
	if err != nil || !ok {
		if err == nil {
			err = fmt.Errorf("invalid signature")
		}
		return errors.Wrap(err, "push ok signature failed verification")
	}

	// cache the PushOK object as an endorsement of the PushNote
	m.addPushNoteEndorsement(pok.PushNoteID.HexStr(), &pok)

	// Attempt to create a PushTx and send to the transaction pool
	if err = m.MaybeCreatePushTx(pok.PushNoteID.HexStr()); err != nil {
		m.Log().Debug(err.Error())
	}

	// Broadcast the PushOK to peers
	m.broadcastPushOK(&pok)

	return nil
}

// BroadcastPushObjects broadcasts repo push notes and PushOK; PushOK is only
// created and broadcast only if the node is a top storer.
func (m *Manager) BroadcastPushObjects(pushNote types.RepoPushNote) error {

	// Broadcast the push note to peers
	m.broadcastPushNote(pushNote)

	// Get the top storers
	topStorers, err := m.logic.GetTicketManager().GetTopStorers(params.NumTopStorersLimit)
	if err != nil {
		return errors.Wrap(err, "failed to get top storers")
	}

	// Exit with nil if node is not among the top storers
	if !topStorers.Has(m.privValidatorKey.PubKey().Base58()) {
		return nil
	}

	// At this point, the node is a top storer, so we create,
	// sign and broadcast a PushOK
	pok := &types.PushOK{}
	pok.PushNoteID = pushNote.ID()
	pok.SenderPubKey = util.BytesToHash(m.privValidatorKey.PubKey().MustBytes())
	pok.Sig = util.BytesToSig(m.privValidatorKey.PrivKey().MustSign(pok.Bytes()))
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

// broadcastPushNote broadcast push transaction to peers.
// It will not send to original sender of the push note.
func (m *Manager) broadcastPushNote(pushNote types.RepoPushNote) {
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
func (m *Manager) broadcastPushOK(pushOk types.RepoPushOK) {
	for _, peer := range m.Switch.Peers().List() {
		bz, id := pushOk.BytesAndID()
		if m.isPushOKSender(string(peer.ID()), id.String()) {
			continue
		}
		if peer.Send(PushOKReactorChannel, bz) {
			m.log.Debug("Sent push OK to peer", "PeerID", peer.ID(), "TxID", id)
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
	pushOKIndex := notePushOKs.(map[string]*types.PushOK)
	if len(pushOKIndex) < params.PushOKQuorumSize {
		return fmt.Errorf("Not enough PushOKs to satisfy quorum size")
	}

	// Get the push note from the push pool
	note := m.GetPushPool().Get(pushNoteID)
	if note == nil {
		return fmt.Errorf("push note not found in pool")
	}

	pushOKs := []*types.PushOK{}
	for _, v := range pushOKIndex {
		pushOKs = append(pushOKs, v)
	}

	pushTx := types.NewBareTxPush()
	pushTx.PushNote = note
	pushTx.PushOKs = pushOKs

	// Sort PushOKs to promote determinism with other nodes that are going to
	// construct their own PushTx (we need the final ID to be same network-wide)
	sort.Slice(pushTx.PushOKs, func(i, j int) bool {
		pnI := pushTx.PushOKs[i]
		pnJ := pushTx.PushOKs[j]
		return pnI.SenderPubKey.Big().Cmp(pnJ.SenderPubKey.Big()) == -1
	})

	// Add push to mempool
	if err := m.GetMempool().Add(pushTx); err != nil {
		return errors.Wrap(err, "failed to add push tx to mempool")
	}

	pushTx.PushNote.TargetRepo = nil

	return nil
}

// onCommittedTxPush handles committed push transactions.
func (m *Manager) onCommittedTxPush(tx *types.TxPush) error {

	repoPath := m.getRepoPath(tx.PushNote.GetRepoName())

	// Get the repository
	repo, err := getRepo(repoPath)
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
