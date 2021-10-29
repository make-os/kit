package types

import (
	"io"

	"github.com/go-git/go-git/v5/plumbing/format/pktline"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp"
	"github.com/make-os/kit/remote/plumbing"
	coretypes "github.com/make-os/kit/types"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/identifier"
	"github.com/vmihailenco/msgpack"
)

// Handler describes an interface for handling incoming push updates.
type Handler interface {

	// WaitForPushTx waits for the final push transaction to be created and added to the mempool.
	//  - It will return error if the tx was rejected.
	//  - An error is returned if the tx was not successfully added to the pool after 15 minutes.
	WaitForPushTx() chan interface{}

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

	// HandleGCAndSizeCheck  performs garbage collection and repo size validation.
	//
	// It will return error if repo size exceeds the allowed maximum.
	//
	// It will also reload the repository handle since GC makes
	// go-git internal state stale.
	HandleGCAndSizeCheck() error

	// HandleUpdate creates a push note to represent the push operation and
	// adds it to the push pool and then have it broadcast to peers.
	HandleUpdate(targetNote PushNote) error

	// HandleReference performs validation and update reversion for a single pushed reference.
	// // When revertOnly is true, only reversion operation is performed.
	HandleReference(ref string) []error

	// HandleReversion reverts the pushed references back to their pre-push state.
	HandleReversion() []error

	// HandlePushNote implements Handler by handing incoming push note
	HandlePushNote(note PushNote) (err error)
}

// PushPool represents a pool for ordering git push transactions
type PushPool interface {

	// Add Register a push transaction to the pool.
	//
	// Check all the references to ensure there are no identical (same repo,
	// reference and nonce) references with same nonce in the pool. A valid
	// reference is one which has no identical reference with a higher fee rate in
	// the pool. If an identical reference exist in the pool with an inferior fee
	// rate, the existing tx holding the reference is eligible for replacable by tx
	// holding the reference with a superior fee rate. In cases where more than one
	// reference of tx is superior to multiple references in multiple transactions,
	// replacement will only happen if the fee rate of tx is higher than the
	// combined fee rate of the replaceable transactions.
	//
	// noValidation disables tx validation
	Add(note PushNote) error

	// Full returns true if the pool is full
	Full() bool

	// Get finds and returns a push note
	Get(noteID string) *Note

	// Len returns the number of items in the pool
	Len() int

	// Remove removes a push note
	Remove(pushNote PushNote)

	// HasSeen checks whether a note with the given ID was recently added
	HasSeen(noteID string) bool
}

type PushNote interface {
	coretypes.Meta
	GetTargetRepo() plumbing.LocalRepo
	SetTargetRepo(repo plumbing.LocalRepo)
	GetPusherKeyID() []byte
	GetPusherAddress() identifier.Address
	GetPusherAccountNonce() uint64
	GetPusherKeyIDString() string
	EncodeMsgpack(enc *msgpack.Encoder) error
	DecodeMsgpack(dec *msgpack.Decoder) error
	Bytes(recompute ...bool) []byte
	BytesNoCache() []byte
	BytesNoSig() []byte
	GetEcoSize() uint64
	GetCreatorPubKey() util.Bytes32
	GetNodeSignature() []byte
	GetRepoName() string
	GetNamespace() string
	GetTimestamp() int64
	GetPushedReferences() PushedReferences
	Len() uint64
	ID(recompute ...bool) util.Bytes32
	BytesAndID(recompute ...bool) ([]byte, util.Bytes32)
	TxSize() uint
	SizeForFeeCal() uint64
	GetSize() uint64
	GetFee() util.String
	GetValue() util.String
	IsFromRemotePeer() bool
}
