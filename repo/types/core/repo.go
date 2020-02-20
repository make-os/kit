package core

import (
	mempooltypes "gitlab.com/makeos/mosdef/mempool/types"
	repomsgs "gitlab.com/makeos/mosdef/repo/types/msgs"
	"gitlab.com/makeos/mosdef/types/msgs"
	"gitlab.com/makeos/mosdef/types/state"
	"time"

	"gitlab.com/makeos/mosdef/storage/tree"
	"gitlab.com/makeos/mosdef/util"
	"github.com/vmihailenco/msgpack"
	"gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"
	"gopkg.in/src-d/go-git.v4/storage"
)

// Constants
const (
	RepoObjectModule = "repo-object"
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

	// CommitObject returns a commit.
	CommitObject(h plumbing.Hash) (*object.Commit, error)

	// WrappedCommitObject returns commit that implements types.Commit interface.
	WrappedCommitObject(h plumbing.Hash) (Commit, error)

	// MergeBranch merges target branch into base
	MergeBranch(base, target, targetRepoDir string) error

	// TryMergeBranch merges target branch into base and reverses it
	TryMergeBranch(base, target, targetRepoDir string) error

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

	// MakeSignableCommit sign and commit staged changes
	// msg: The commit message.
	// signingKey: The signing key
	// env: Optional environment variables to pass to the command.
	MakeSignableCommit(msg, signingKey string, env ...string) error

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

	// UpdateRecentCommitMsg updates the recent commit message.
	// msg: The commit message which is passed to the command's stdin.
	// signingKey: The signing key
	// env: Optional environment variables to pass to the command.
	UpdateRecentCommitMsg(msg, signingKey string, env ...string) error

	// UpdateTree updates the state tree
	UpdateTree(ref string, updater func(tree *tree.SafeTree) error) ([]byte, int64, error)

	// TreeRoot returns the state root of the repository
	TreeRoot(ref string) (util.Bytes32, error)

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
	State() *state.Repository

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

	// Prune prunes objects older than the given time
	Prune(olderThan time.Time) error
}

// Commit represents a Commit.
type Commit interface {

	// NumParents returns the number of parents in a commit.
	NumParents() int

	// Parent returns the ith parent of a commit.
	Parent(i int) (Commit, error)

	// GetCommitter returns the one performing the commit, might be different from Author
	GetCommitter() *object.Signature

	// GetAuthor returns the original author of the commit.
	GetAuthor() *object.Signature

	// GetTreeHash returns the hash of the root tree of the commit
	GetTreeHash() plumbing.Hash

	// GetHash returns the hash of the commit object
	GetHash() plumbing.Hash
}

// PGPPubKeyGetter represents a function for fetching PGP public key
type PGPPubKeyGetter func(pkId string) (string, error)

// PoolGetter returns various pools
type PoolGetter interface {

	// GetPushPool returns the push pool
	GetPushPool() PushPool

	// GetMempool returns the transaction pool
	GetMempool() mempooltypes.Mempool
}

// RepoGetter describes an interface for getting a local repository
type RepoGetter interface {

	// GetRepo returns a repo handle
	GetRepo(name string) (BareRepo, error)
}

// TxPushMerger describes an interface for merging push transaction to a repository
type TxPushMerger interface {
	// UpdateRepoWithTxPush attempts to merge a push transaction to a repository and
	// also update the repository's state tree.
	UpdateRepoWithTxPush(tx *msgs.TxPush) error
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
	//
	// noValidation disables tx validation
	Add(tx RepoPushNote, noValidation ...bool) error

	// Full returns true if the pool is full
	Full() bool

	// RepoHasPushNote returns true if the given repo has a transaction in the pool
	RepoHasPushNote(repo string) bool

	// Get finds and returns a push note
	Get(noteID string) *repomsgs.PushNote

	// Len returns the number of items in the pool
	Len() int

	// Remove removes a push note
	Remove(pushNote *repomsgs.PushNote)
}

// RepoPushNote represents a repository push request
type RepoPushNote interface {

	// RepoName returns the name of the repo receiving the push
	GetRepoName() string

	// Bytes returns a serialized version of the object
	Bytes() []byte

	// GetEcoSize returns the length of the serialized tx minus
	// the total length of fee fields.
	GetEcoSize() uint64

	// Len returns the length of the serialized tx
	Len() uint64

	// ID returns the hash of the push note
	ID() util.Bytes32

	// TxSize is the size of the transaction
	TxSize() uint

	// BillableSize is the size of the transaction + pushed objects
	BillableSize() uint64

	// Fee returns the sum of reference update fees
	GetFee() util.String

	// GetPushedReferences returns the pushed references
	GetPushedReferences() PushedReferences

	// GetPusherKeyID returns the pusher gpg key ID
	GetPusherKeyID() []byte

	// GetPusherKeyIDString is like GetPusherKeyID but returns hex string, prefixed
	// with 0x
	GetPusherKeyIDString() string

	// GetTargetRepo returns the target repository
	GetTargetRepo() BareRepo

	// GetSize returns the total pushed objects size
	GetSize() uint64

	// GetPushedObjects returns all objects from all pushed references without a
	// delete directive.
	// ignoreDelRefs cause deleted references' objects to not be include in the result
	GetPushedObjects(ignoreDelRefs bool) (objs []string)

	// BytesAndID returns the serialized version of the tx and the id
	BytesAndID() ([]byte, util.Bytes32)
}

// Pruner provides repository pruning functionality
type Pruner interface {

	// Start starts the pruner
	Start()

	// Schedule schedules a repository for pruning
	Schedule(repoName string)

	// Prune prunes a repository only if it has no transactions in the transaction
	// and push pool. If force is set to true, the repo will be pruned regardless of
	// the existence of transactions in the pools.
	Prune(repoName string, force bool) error

	// Stop stops the pruner
	Stop()
}

// PushedReference represents a reference that was pushed by git client
type PushedReference struct {
	util.DecoderHelper `json:",flatten" msgpack:"-" mapstructure:"-"`
	Name               string   `json:"name" msgpack:"name"`       // The full name of the reference
	OldHash            string   `json:"oldHash" msgpack:"oldHash"` // The hash of the reference before the push
	NewHash            string   `json:"newHash" msgpack:"newHash"` // The hash of the reference after the push
	Nonce              uint64   `json:"nonce" msgpack:"nonce"`     // The next repo nonce of the reference
	Objects            []string `json:"objects" msgpack:"objects"` // A list of objects pushed to the reference
	Delete             bool     `json:"delete" msgpack:"delete"`   // Delete indicates that the reference should be deleted from the repo
	MergeProposalID    string   `json:"mergeId" msgpack:"mergeId"` // The merge proposal ID the reference is complaint with.
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (pr *PushedReference) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.EncodeMulti(
		pr.Name,
		pr.OldHash,
		pr.NewHash,
		pr.Nonce,
		pr.Objects,
		pr.Delete,
		pr.MergeProposalID)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (pr *PushedReference) DecodeMsgpack(dec *msgpack.Decoder) error {
	return pr.DecodeMulti(dec,
		&pr.Name,
		&pr.OldHash,
		&pr.NewHash,
		&pr.Nonce,
		&pr.Objects,
		&pr.Delete,
		&pr.MergeProposalID)
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
	Hash() util.Bytes32
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
	Hash() util.Bytes32
}

// RepoPushOK represents a push endorsement
type RepoPushOK interface {
	// ID returns the hash of the object
	ID() util.Bytes32
	// Bytes returns a serialized version of the object
	Bytes() []byte
	// BytesAndID returns the serialized version of the tx and the id
	BytesAndID() ([]byte, util.Bytes32)
}
