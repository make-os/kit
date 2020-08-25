package server

import (
	"github.com/make-os/lobe/params"
	pushtypes "github.com/make-os/lobe/remote/push/types"
	"github.com/pkg/errors"
	"github.com/tendermint/tendermint/p2p"
)

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
	sv.makePushTx(endorsement.NoteID.HexStr())

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
