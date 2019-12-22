package repo

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
	"github.com/pkg/errors"
	"github.com/tendermint/tendermint/p2p"
	"gopkg.in/src-d/go-git.v4"
)

// Receive implements Reactor
func (m *Manager) Receive(chID byte, peer p2p.Peer, msgBytes []byte) {
	switch chID {
	case PushTxReactorChannel:
		if err := m.onPushTx(peer, msgBytes); err != nil {
			m.log.Error(err.Error())
		}
	}
}

// onPushTx is the handler for incoming PushTx messages
func (m *Manager) onPushTx(peer p2p.Peer, msgBytes []byte) error {

	// Attempt to decode message to PushTx
	var tx PushTx
	if err := util.BytesToObject(msgBytes, &tx); err != nil {
		return errors.Wrap(err, "failed to decoded message")
	}

	// Add a cache entry that indicates the sender of the push tx
	m.cachePushTxSender(string(peer.ID()), tx.ID().String())

	m.log.Debug("Received push transaction from peer",
		"PeerID", peer.ID(), "TxID", tx.ID().String())

	repoName := tx.GetRepoName()
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

	tx.targetRepo = &Repo{
		name:  repoName,
		git:   repo,
		ops:   NewGitOps(m.gitBinPath, repoPath),
		path:  repoPath,
		db:    NewDBOps(m.repoDBCache, repoName),
		state: repoState,
	}

	// Validate the push tx.
	// if err := checkPushTx(&tx, m.logic, m.dht); err != nil {
	// 	return errors.Wrap(err, "failed push tx validation")
	// }
	if err := m.GetPushPool().Add(&tx); err != nil {
		return errors.Wrap(err, "failed to add push tx to push pool")
	}

	// At this point, we know that the push tx is valid and consistent with the
	// state of the repository, but we need to also check that the pushed
	// references and objects are well signed, have correct
	// transaction information and are compatible with the state of the
	// repository on disk. To do this, we create a packfile from the push
	// tx and attempt to let git-receive-pack process it.

	// Create the pack file
	packfile, err := makeReferenceUpdateRequest(tx.targetRepo, &tx)
	if err != nil {
		return errors.Wrap(err, "failed to create packfile from push tx")
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
	pushHandler := newPushHandler(tx.targetRepo, m)
	if err := pushHandler.HandleStream(packfile, in); err != nil {
		return errors.Wrap(err, "HandleStream error")
	}

	// Handle transaction validation and revert operation changes
	if _, _, err := pushHandler.HandleValidateAndRevert(); err != nil {
		return errors.Wrap(err, "HandleValidateAndRevert error")
	}

	if err := cmd.Wait(); err != nil {
		return errors.Wrap(err, "failed to process packfile derived from push tx")
	}

	// At this point, the transaction has passed all validation and
	// compatibility checks. We can now attempt to add the push tx to the PushPool
	// if err := m.GetPushPool().Add(&tx); err != nil {
	// 	return errors.Wrap(err, "failed to add push tx to push pool")
	// }

	// Announce the objects of the push tx to the dht
	for _, hash := range tx.GetPushedObjects() {
		dhtKey := MakeRepoObjectDHTKey(repoName, hash)
		ctx, c := context.WithTimeout(context.Background(), 60*time.Second)
		defer c()
		if err := m.GetDHT().Annonce(ctx, []byte(dhtKey)); err != nil {
			m.log.Error("unable to announce git object", "Err", err)
			continue
		}
	}

	// Broadcast the push tx to peers
	m.BroadcastPushTx(&tx)

	m.log.Info("Added valid push tx to push pool", "TxID", tx.ID().String())

	return nil
}

// BroadcastPushTx broadcast push transaction to peers.
// It will not send to original sender of the push tx.
func (m *Manager) BroadcastPushTx(pushTx types.PushTx) {
	for _, peer := range m.Switch.Peers().List() {
		bz, id := pushTx.BytesAndID()
		if m.isPushTxSender(string(peer.ID()), id.String()) {
			continue
		}
		if peer.Send(PushTxReactorChannel, bz) {
			m.log.Debug("Sent push transaction to peer", "PeerID", peer.ID(), "TxID", id)
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
		{ID: PushTxReactorChannel, Priority: 5},
	}
}
