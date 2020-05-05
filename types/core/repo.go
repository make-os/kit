package core

import (
	"context"
	"time"

	config2 "gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/crypto"
	types2 "gitlab.com/makeos/mosdef/dht/types"
	"gitlab.com/makeos/mosdef/pkgs/logger"
	"gitlab.com/makeos/mosdef/types/modules"
	"gitlab.com/makeos/mosdef/types/state"

	"github.com/vmihailenco/msgpack"
	"gitlab.com/makeos/mosdef/util"
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
	LiteGit

	// GetName returns the name of the repo
	GetName() string

	// GetNameFromPath returns the name of the repo
	GetNameFromPath() string

	// GetNamespaceName returns the namespace this repo is associated to.
	GetNamespaceName() string

	// GetNamespace returns the repos's namespace
	GetNamespace() *state.Namespace

	// References returns an unsorted ReferenceIter for all references.
	References() (storer.ReferenceIter, error)

	// IsContributor checks whether a push key is a contributor to either
	// the repository or its namespace
	IsContributor(pushKeyID string) bool

	// GetRemoteURLs returns the remote URLS of the repository
	GetRemoteURLs() (urls []string)

	// DeleteObject deletes an object from a repository.
	DeleteObject(hash plumbing.Hash) error

	// Reference returns the reference for a given reference name.
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

	// BlobObject returns a Blob with the given hash.
	BlobObject(h plumbing.Hash) (*object.Blob, error)

	// TagObject returns a Tag with the given hash.
	TagObject(h plumbing.Hash) (*object.Tag, error)

	// Tag returns a tag from the repository.
	Tag(name string) (*plumbing.Reference, error)

	// Config return the repository config
	Config() (*config.Config, error)

	// SetConfig sets the repo config
	SetConfig(cfg *config.Config) error

	// SetPath sets the repository root path
	SetPath(path string)

	// GetPath returns the repository's path
	GetPath() string

	// GetState returns the repository's network state
	GetState() *state.Repository

	// Head returns the reference where HEAD is pointing to.
	Head() (string, error)

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

	// GetHost returns the storage engine of the repository
	GetHost() storage.Storer

	// Prune prunes objects older than the given time
	Prune(olderThan time.Time) error

	// NumIssueBranches counts the number of issues branches
	NumIssueBranches() (count int, err error)

	// GetFreeIssueNum finds an issue number that has not been used
	GetFreeIssueNum(startID int) (int, error)
}

// Commit represents a Commit.
type Commit interface {

	// NumParents returns the number of parents in a commit.
	NumParents() int

	// Parent returns the ith parent of a commit.
	Parent(i int) (Commit, error)

	// IsParent checks whether the specified hash is a parent of the commit
	IsParent(hash string) (bool, Commit)

	// UnWrap returns the underlying commit object
	UnWrap() *object.Commit

	// GetCommitter returns the one performing the commit, might be different from Author
	GetCommitter() *object.Signature

	// GetAuthor returns the original author of the commit.
	GetAuthor() *object.Signature

	// GetTreeHash returns the hash of the root tree of the commit
	GetTreeHash() plumbing.Hash

	// GetHash returns the hash of the commit object
	GetHash() plumbing.Hash

	// GetTree returns the tree from the commit
	GetTree() (*object.Tree, error)
}

type Remote struct {
	Name string
	URLs []string
}

// PushKeyGetter represents a function used for fetching a push key
type PushKeyGetter func(pushKeyID string) (crypto.PublicKey, error)

// PoolGetter returns various pools
type PoolGetter interface {

	// GetPushPool returns the push pool
	GetPushPool() PushPool

	// GetMempool returns the transaction pool
	GetMempool() Mempool
}

// RepoGetter describes an interface for getting a local repository
type RepoGetter interface {

	// Get returns a repo handle
	GetRepo(name string) (BareRepo, error)
}

// RepoUpdater describes an interface for updating a repository from a push transaction
type RepoUpdater interface {
	// UpdateRepoWithTxPush attempts to merge a push transaction to a repository and
	// also update the repository's state tree.
	UpdateRepoWithTxPush(tx *TxPush) error
}

// PushPool represents a pool for holding and ordering git push transactions
type PushPool interface {

	// Register a push transaction to the pool.
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
	Get(noteID string) *PushNote

	// Len returns the number of items in the pool
	Len() int

	// Remove removes a push note
	Remove(pushNote *PushNote)
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

	// GetPusherKeyID returns the pusher push key ID
	GetPusherKeyID() []byte

	// GetPusherKeyIDString is like GetPusherKeyID but returns hex string, prefixed
	// with 0x
	GetPusherKeyIDString() string

	// GetTargetRepo returns the target repository
	GetTargetRepo() BareRepo

	// GetSize returns the total pushed objects size
	GetSize() uint64

	// GetPushedObjects returns all objects from all pushed references without a
	// delete option.
	// ignoreDelRefs cause deleted references' objects to not be include in the result
	GetPushedObjects() (objs []string)

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
	util.SerializerHelper `json:"-" msgpack:"-" mapstructure:"-"`
	Name                  string      `json:"name" msgpack:"name,omitempty"`       // The full name of the reference
	OldHash               string      `json:"oldHash" msgpack:"oldHash,omitempty"` // The hash of the reference before the push
	NewHash               string      `json:"newHash" msgpack:"newHash,omitempty"` // The hash of the reference after the push
	Nonce                 uint64      `json:"nonce" msgpack:"nonce,omitempty"`     // The next repo nonce of the reference
	Objects               []string    `json:"objects" msgpack:"objects,omitempty"` // A list of objects pushed to the reference
	MergeProposalID       string      `json:"mergeID" msgpack:"mergeID,omitempty"` // The merge proposal ID the reference is complaint with.
	Fee                   util.String `json:"fee" msgpack:"fee,omitempty"`         // The merge proposal ID the reference is complaint with.
	PushSig               []byte      `json:"pushSig" msgpack:"pushSig,omitempty"` // The signature of from the push request token
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (pr *PushedReference) EncodeMsgpack(enc *msgpack.Encoder) error {
	return enc.EncodeMulti(
		pr.Name,
		pr.OldHash,
		pr.NewHash,
		pr.Nonce,
		pr.Objects,
		pr.MergeProposalID,
		pr.Fee,
		pr.PushSig)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (pr *PushedReference) DecodeMsgpack(dec *msgpack.Decoder) error {
	return pr.DecodeMulti(dec,
		&pr.Name,
		&pr.OldHash,
		&pr.NewHash,
		&pr.Nonce,
		&pr.Objects,
		&pr.MergeProposalID,
		&pr.Fee,
		&pr.PushSig)
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
	Changes []*ItemChange
}

// BareRepoState represents a repositories state
type BareRepoState interface {
	// GetReferences returns the references.
	GetReferences() Items
	// IsEmpty checks whether the state is empty
	IsEmpty() bool
	// GetChanges summarizes the changes between GetState s and y.
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

// RepoPushEndorsement represents a push endorsement
type RepoPushEndorsement interface {
	// ID returns the hash of the object
	ID() util.Bytes32
	// Bytes returns a serialized version of the object
	Bytes() []byte
	// BytesAndID returns the serialized version of the tx and the id
	BytesAndID() ([]byte, util.Bytes32)
}

// RemoteServer provides functionality for manipulating repositories.
type RemoteServer interface {
	PoolGetter
	RepoGetter
	RepoUpdater

	// Log returns the logger
	Log() logger.Logger

	// Cfg returns the application config
	Cfg() *config2.AppConfig

	// GetRepoState returns the state of the repository at the given path
	// options: Allows the caller to configure how and what state are gathered
	GetRepoState(target BareRepo, options ...KVOption) (BareRepoState, error)

	// GetPushKeyGetter returns getter function for fetching a push key
	GetPushKeyGetter() PushKeyGetter

	// GetLogic returns the application logic provider
	GetLogic() Logic

	// GetPrivateValidatorKey returns the node's private key
	GetPrivateValidatorKey() *crypto.Key

	// Start starts the server
	Start() error

	// Wait can be used by the caller to wait till the server terminates
	Wait()

	// CreateRepository creates a local git repository
	CreateRepository(name string) error

	// BroadcastMsg broadcast messages to peers
	BroadcastMsg(ch byte, msg []byte)

	// BroadcastPushObjects broadcasts repo push note and push endorsement
	BroadcastPushObjects(note RepoPushNote) error

	// SetPGPPubKeyGetter sets the PGP public key query function
	SetPGPPubKeyGetter(pkGetter PushKeyGetter)

	// RegisterAPIHandlers registers server API handlers
	RegisterAPIHandlers(agg modules.ModuleHub)

	// GetPruner returns the repo pruner
	GetPruner() Pruner

	// GetDHT returns the dht service
	GetDHT() types2.DHTNode

	// ExecTxPush applies a push transaction to the local repository.
	// If the node is a validator, only the target reference trees are updated.
	ExecTxPush(tx *TxPush) error

	// Shutdown shuts down the server
	Shutdown(ctx context.Context)

	// Stop implements Reactor
	Stop() error
}

type LiteGit interface {
	RefDelete(refname string) error
	RefUpdate(refname, commitHash string) error
	TagDelete(tagname string) error
	RefGet(refname string) (string, error)
	GetRecentCommitHash() (string, error)
	GetHEAD(short bool) (string, error)
	NumCommits(branch string, noMerges bool) (int, error)
	GetConfig(path string) string
	CreateSignedEmptyCommit(msg, signingKey string, env ...string) error
	CreateTagWithMsg(args []string, msg, signingKey string, env ...string) error
	ListTreeObjects(treename string, recursive bool, env ...string) (map[string]string, error)
	ListTreeObjectsSlice(treename string, recursive, showTrees bool, env ...string) ([]string, error)
	RemoveEntryFromNote(notename, objectHash string, env ...string) error
	AddEntryToNote(notename, objectHash, note string, env ...string) error
	CreateBlob(content string) (string, error)
	UpdateRecentCommitMsg(msg, signingKey string, env ...string) error
	IsAncestor(commitA string, commitB string, env ...string) error
	HasMergeCommits(reference string, env ...string) (bool, error)
	GetMergeCommits(reference string, env ...string) ([]string, error)
	CreateSingleFileCommit(filename, content, commitMsg, parent string) (string, error)
	Checkout(refname string, create, force bool) error
	GetRefRootCommit(ref string) (string, error)
	GetRefCommits(ref string, noMerges bool) ([]string, error)
	Var(name string) (string, error)
}

type CommitTree interface {
	File(path string) (*object.File, error)
	Size(path string) (int64, error)
	Tree(path string) (*object.Tree, error)
	TreeEntryFile(e *object.TreeEntry) (*object.File, error)
	FindEntry(path string) (*object.TreeEntry, error)
	Files() *object.FileIter
	ID() plumbing.Hash
	Type() plumbing.ObjectType
	Decode(o plumbing.EncodedObject) (err error)
	Encode(o plumbing.EncodedObject) (err error)
}
