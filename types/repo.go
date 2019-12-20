package types

import (
	"context"

	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/util"
	"github.com/makeos/mosdef/util/logger"
	"github.com/vmihailenco/msgpack"
	"gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"
	"gopkg.in/src-d/go-git.v4/storage"
)

// BareRepo represents a local git repository on disk
type BareRepo interface {

	// GetName returns the name of the repo
	GetName() string

	// References returns an unsorted ReferenceIter for all references.
	References() (storer.ReferenceIter, error)

	// RefDelete executes `git update-ref -d <refname>` to delete a reference
	RefDelete(refname string) error

	// RefUpdate executes `git update-ref <refname> <commit hash>` to update/create a reference
	RefUpdate(refname, commitHash string) error

	// RefGet returns the hash content of a reference.
	RefGet(refname string) (string, error)

	// TagDelete executes `git tag -d <tagname>` to delete a tag
	TagDelete(tagname string) error

	// ListTreeObjects executes `git tag -d <tagname>` to delete a tag
	ListTreeObjects(treename string, recursive bool, env ...string) (map[string]string, error)

	// DeleteObject deletes an object from a repository.
	DeleteObject(hash plumbing.Hash) error

	// Reference deletes an object from a repository.
	Reference(name plumbing.ReferenceName, resolved bool) (*plumbing.Reference, error)

	// Object returns an Object with the given hash.
	Object(t plumbing.ObjectType, h plumbing.Hash) (object.Object, error)

	// Objects returns an unsorted ObjectIter with all the objects in the repository.
	Objects() (*object.ObjectIter, error)

	// CommitObjects returns an unsorted ObjectIter with all the objects in the repository.
	CommitObjects() (object.CommitIter, error)

	// CommitObject returns an unsorted ObjectIter with all the objects in the repository.
	CommitObject(h plumbing.Hash) (*object.Commit, error)

	// BlobObject returns a Blob with the given hash.
	BlobObject(h plumbing.Hash) (*object.Blob, error)

	// TagObject returns a Tag with the given hash.
	TagObject(h plumbing.Hash) (*object.Tag, error)

	// Tag returns a tag from the repository.
	Tag(name string) (*plumbing.Reference, error)

	// Config return the repository config
	Config() (*config.Config, error)

	// GetConfig finds and returns a config value
	GetConfig(path string) string

	// GetRecentCommit gets the hash of the recent commit.
	// Returns ErrNoCommits if no commits exist
	GetRecentCommit() (string, error)

	// UpdateRecentCommitMsg updates the recent commit message.
	// msg: The commit message which is passed to the command's stdin.
	// signingKey: The signing key
	// env: Optional environment variables to pass to the command.
	UpdateRecentCommitMsg(msg, signingKey string, env ...string) error

	// CreateTagWithMsg an annotated tag.
	// args: `git tag` options (NOTE: -a and --file=- are added by default)
	// msg: The tag's message which is passed to the command's stdin.
	// signingKey: The signing key to use
	// env: Optional environment variables to pass to the command.
	CreateTagWithMsg(args []string, msg, signingKey string, env ...string) error

	// RemoveEntryFromNote removes a note
	RemoveEntryFromNote(notename, objectHash string, env ...string) error

	// CreateBlob creates a blob object
	CreateBlob(content string) (string, error)

	// AddEntryToNote adds a note
	AddEntryToNote(notename, objectHash, note string, env ...string) error

	// ListTreeObjectsSlice returns a slice containing objects name of tree entries
	ListTreeObjectsSlice(treename string, recursive, showTrees bool,
		env ...string) ([]string, error)

	// SetPath sets the repository root path
	SetPath(path string)

	// Path returns the repository's path
	Path() string

	// State returns the repository's network state
	State() *Repository

	// ObjectExist checks whether an object exist in the target repository
	ObjectExist(objHash string) bool

	// GetObjectSize returns the size of an object
	GetObjectSize(objHash string) (int64, error)

	// GetObjectDiskSize returns the size of the object as it exist on the system
	GetObjectDiskSize(objHash string) (int64, error)

	// GetEncodedObject returns an object
	GetEncodedObject(objHash string) (plumbing.EncodedObject, error)

	// WriteObjectToFile writes an object to the repository's objects directory
	WriteObjectToFile(objectHash string, content []byte) error

	// GetObject returns an object
	GetObject(objHash string) (object.Object, error)

	// GetCompressedObject compressed version of an object
	GetCompressedObject(hash string) ([]byte, error)

	// GetStorer returns the storage engine of the repository
	GetStorer() storage.Storer
}

// PGPPubKeyGetter represents a function for fetching PGP public key
type PGPPubKeyGetter func(pkId string) (string, error)

// RepoManager provides functionality for manipulating repositories.
type RepoManager interface {

	// Log returns the logger
	Log() logger.Logger

	// GetRepoState returns the state of the repository at the given path
	// options: Allows the caller to configure how and what state are gathered
	GetRepoState(target BareRepo, options ...KVOption) (BareRepoState, error)

	// Revert reverts the repository from its current state to the previous state.
	Revert(target BareRepo, prevState BareRepoState,
		options ...KVOption) (*Changes, error)

	// GetPGPPubKeyGetter returns the gpg getter function for finding GPG public
	// keys by their ID
	GetPGPPubKeyGetter() PGPPubKeyGetter

	// GetLogic returns the application logic provider
	GetLogic() Logic

	// GetNodeKey returns the node's private key
	GetNodeKey() *crypto.Key

	// GetPushPool returns the push pool
	GetPushPool() PushPool

	// Start starts the server
	Start() error

	// Wait can be used by the caller to wait till the server terminates
	Wait()

	// CreateRepository creates a local git repository
	CreateRepository(name string) error

	// BroadcastMsg broadcast messages to peers
	BroadcastMsg(ch byte, msg []byte)

	// BroadcastPushTx broadcast push transaction to peers
	BroadcastPushTx(pushTx PushTx)

	// SetPGPPubKeyGetter sets the PGP public key query function
	SetPGPPubKeyGetter(pkGetter PGPPubKeyGetter)

	// AddUnfinalizedObject adds an object to the unfinalized object cache
	AddUnfinalizedObject(repo, objHash string)

	// Remove removes an object from the unfinalized object cache
	RemoveUnfinalizedObject(repo, objHash string)

	// IsUnfinalizedObject checks whether an object exist in the unfinalized object cache
	IsUnfinalizedObject(repo, objHash string) bool

	// GetDHT returns the dht service
	GetDHT() DHT

	// Shutdown shuts down the server
	Shutdown(ctx context.Context)

	// Stop implements Reactor
	Stop() error
}

// UnfinalizedObjectCache keeps track of unfinalized repository objects
type UnfinalizedObjectCache interface {
	// AddUnfinalizedObject adds an object to the unfinalized object cache
	AddUnfinalizedObject(repo, objHash string)

	// Remove removes an object from the unfinalized object cache
	RemoveUnfinalizedObject(repo, objHash string)

	// IsUnfinalizedObject checks whether an object exist in the unfinalized object cache
	IsUnfinalizedObject(repo, objHash string) bool
}

// PushPool represents a pool for holding and ordering git push transactions
type PushPool interface {

	// Add a push transaction to the pool.
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
	Add(tx PushTx) error

	// Full returns true if the pool is full
	Full() bool
}

// PushTx represents a repository push request
type PushTx interface {

	// RepoName returns the name of the repo receiving the push
	GetRepoName() string

	// Bytes returns a serialized version of the object
	Bytes() []byte

	// LenMinusFee returns the length of the serialized tx minus
	// the total length of fee fields.
	LenMinusFee() uint64

	// Len returns the length of the serialized tx
	Len() uint64

	// ID returns the hash of the push tx
	ID() util.Hash

	// TxSize is the size of the transaction
	TxSize() uint

	// BillableSize is the size of the transaction + pushed objects
	BillableSize() uint64

	// TotalFee returns the sum of reference update fees
	TotalFee() util.String

	// GetPushedReferences returns the pushed references
	GetPushedReferences() PushedReferences

	// GetPusherKeyID returns the pusher gpg key ID
	GetPusherKeyID() string

	// GetTargetRepo returns the target repository
	GetTargetRepo() BareRepo

	// GetSize returns the total pushed objects size
	GetSize() uint64

	// GetPushedObjects returns all objects from all pushed references
	GetPushedObjects() (objs []string)

	// BytesAndID returns the serialized version of the tx and the id
	BytesAndID() ([]byte, util.Hash)
}

// PushedReference represents a reference that was pushed by git client
type PushedReference struct {
	Name         string      `json:"name" msgpack:"name"`                 // The full name of the reference
	OldHash      string      `json:"oldHash" msgpack:"oldHash"`           // The hash of the reference before the push
	NewHash      string      `json:"newHash" msgpack:"newHash"`           // The hash of the reference after the push
	Nonce        uint64      `json:"nonce" msgpack:"nonce"`               // The next repo nonce of the reference
	AccountNonce uint64      `json:"accountNonce" msgpack:"accountNonce"` // The pusher's account nonce
	Fee          util.String `json:"fee" msgpack:"fee"`                   // The fee the pusher is willing to pay to validators
	Objects      []string    `json:"objects" msgpack:"objects"`           // A list of objects pushed to the reference
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (pr *PushedReference) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.EncodeMulti(pr.Name, pr.OldHash, pr.NewHash,
		pr.Nonce, pr.AccountNonce, pr.Fee, pr.Objects)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (pr *PushedReference) DecodeMsgpack(dec *msgpack.Decoder) error {
	return dec.DecodeMulti(&pr.Name, &pr.OldHash, &pr.NewHash,
		&pr.Nonce, &pr.AccountNonce, &pr.Fee, &pr.Objects)
}

// PushedReferences represents a collection of pushed references
type PushedReferences []*PushedReference

// GetByName finds a pushed reference by name
func (pf *PushedReferences) GetByName(name string) *PushedReference {
	for _, r := range *pf {
		if r.Name == name {
			return r
		}
	}
	return nil
}

// Names returns the names of the references
func (pf *PushedReferences) Names() (names []string) {
	for _, r := range *pf {
		names = append(names, r.Name)
	}
	return
}

type (
	// ColChangeType describes a change to a collection item
	ColChangeType int
)

const (
	// ChangeTypeNew represents a new, unique item added to a collection
	ChangeTypeNew ColChangeType = iota
	// ChangeTypeRemove represents a removal of a collection item
	ChangeTypeRemove
	// ChangeTypeUpdate represents an update to the value of a collection item
	ChangeTypeUpdate
)

// KVOption holds key-value structure of options
type KVOption struct {
	Key   string
	Value interface{}
}

// ItemChange describes a change event
type ItemChange struct {
	Item   Item
	Action ColChangeType
}

// ChangeResult includes information about changes
type ChangeResult struct {
	SizeChange bool // TODO: remove if no use so far
	Changes    []*ItemChange
}

// BareRepoState represents a repositories state
type BareRepoState interface {
	// GetReferences returns the references.
	GetReferences() Items
	// IsEmpty checks whether the state is empty
	IsEmpty() bool
	// Hash returns the 32-bytes hash of the state
	Hash() util.Hash
	// GetChanges summarizes the changes between State s and y.
	GetChanges(y BareRepoState) *Changes
}

// Changes describes reference changes that happened to a repository
// from a previous state to its current state.
type Changes struct {
	References *ChangeResult
}

// Item represents a git object or reference
type Item interface {
	GetName() string
	Equal(o interface{}) bool
	GetData() string
	GetType() string
}

// Items represents a collection of git objects or references identified by a name
type Items interface {
	Has(name interface{}) bool
	Get(name interface{}) Item
	Equal(o interface{}) bool
	ForEach(func(i Item) bool)
	Len() int64
	Bytes() []byte
	Hash() util.Hash
}
