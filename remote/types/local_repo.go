package types

import (
	"time"

	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/storage"
	"github.com/make-os/kit/types/state"
)

type CloneOption struct {
	Bare          bool
	ReferenceName string
	Depth         int
}

// LocalRepo represents a local git repository on disk
type LocalRepo interface {
	GitModule

	// GetName returns the name of the repo
	GetName() string

	// GetNamespaceName returns the namespace this repo is associated to.
	GetNamespaceName() string

	// GetNamespace returns the repos's namespace
	GetNamespace() *state.Namespace

	// References returns an unsorted ReferenceIter for all references.
	References() (storer.ReferenceIter, error)

	// IsContributor checks whether a push key is a contributor to either
	// the repository or its namespace
	IsContributor(pushKeyID string) bool

	// GetGitConfigOption finds and returns git config option value
	GetGitConfigOption(path string) string

	// GetRemoteURLs returns remote URLS of the repository.
	// Use `names` to select specific remotes with matching name.
	GetRemoteURLs(names ...string) (urls []string)

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

	// Tags return all tag references in the repository.
	// If you want to check to see if the tag is an annotated tag, you can call
	// TagObject on the hash Reference
	Tags() (storer.ReferenceIter, error)

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

	// Config return the repository's git config
	Config() (*config.Config, error)

	// SetConfig sets the repo config
	SetConfig(cfg *config.Config) error

	// IsAncestor checks whether commitA is an ancestor to commitB.
	// It returns ErrNotAncestor when not an ancestor.
	IsAncestor(commitA string, commitB string) error

	// SetPath sets the repository root path
	SetPath(path string)

	// GetReferences returns all references in the repo
	GetReferences() (refs []plumbing.ReferenceName, err error)

	// Reload reloads the repository
	Reload() error

	// GetPath returns the repository's path
	GetPath() string

	// Clone clones the repository into a temporary directory.
	// It returns the cloned repo, the temp directory where it was cloned
	// into and a nil error on success.
	//  - bare: When true, a worktree is created, otherwise a bare repo is created.
	//  - referenceName: Optionally limit fetching to the given reference
	Clone(option CloneOption) (LocalRepo, string, error)

	// IsClean checks whether the working directory has no un-tracked, staged or modified files
	IsClean() (bool, error)

	// GetState returns the repository's network state
	GetState() *state.Repository

	// SetState sets the repository's network state
	SetState(s *state.Repository)

	// Head returns the reference where HEAD is pointing to.
	Head() (string, error)

	// HeadObject returns the object of the HEAD reference.
	// Returns plumbing.ErrReferenceNotFound if HEAD was not found.
	HeadObject() (object.Object, error)

	// ObjectExist checks whether an object exist in the target repository
	ObjectExist(objHash string) bool

	// GetObjectSize returns the size of an object
	GetObjectSize(objHash string) (int64, error)

	// ObjectsOfCommit returns a hashes of objects a commit is composed of.
	// This objects a the commit itself, its tree and the tree blobs.
	ObjectsOfCommit(hash string) ([]plumbing.Hash, error)

	// GetObject returns an object
	GetObject(objHash string) (object.Object, error)

	// GetStorer returns the storage engine of the repository
	GetStorer() storage.Storer

	// Prune prunes objects older than the given time
	Prune(olderThan time.Time) error

	// NumIssueBranches counts the number of issues branches
	NumIssueBranches() (count int, err error)

	// GetAncestors returns the ancestors of the given commit up til the ancestor matching the stop hash.
	// The stop hash ancestor is not included in the result.
	// Reverse reverses the result
	GetAncestors(commit *object.Commit, stopHash string, reverse bool) (ancestors []*object.Commit, err error)

	// UpdateRepoConfig updates the 'repocfg' configuration file
	UpdateRepoConfig(cfg *LocalConfig) (err error)

	// GetRepoConfig returns the 'repocfg' config object
	GetRepoConfig() (*LocalConfig, error)

	// ListPath returns a list of entries in a repository's path
	ListPath(ref, path string) (res []ListPathValue, err error)

	// GetFileLines returns the lines of a file
	GetFileLines(ref, path string) (res []string, err error)

	// GetFile returns the file as a string
	GetFile(ref, path string) (res string, err error)

	// GetBranches returns a list of branches
	GetBranches() (branches []string, err error)

	// GetLatestCommit returns information about last commit of a branch
	GetLatestCommit(branch string) (*CommitResult, error)

	// GetCommits returns commits of a branch or commit hash
	//  - ref: The target reference name (branch or commit hash)
	//  - limit: The number of commit to return. 0 means all.
	GetCommits(ref string, limit int) (res []*CommitResult, err error)

	// GetCommit gets a commit by hash
	//  - hash: The commit hash
	GetCommit(hash string) (*CommitResult, error)

	// GetCommitAncestors returns ancestors of a commit with the given hash.
	//  - commitHash: The hash of the commit.
	//  - limit: The number of commit to return. 0 means all.
	GetCommitAncestors(commitHash string, limit int) (res []*CommitResult, err error)

	// GetParentAndChildCommitDiff returns the commit diff output between a
	// child commit and its parent commit(s). If the commit has more than
	// one parent, the diff will be run for all parents.
	//  - commitHash: The child commit hash.
	GetParentAndChildCommitDiff(commitHash string) (*GetCommitDiffResult, error)
}

type GetCommitDiffResult struct {
	Patches []map[string]string `json:"patches"`
}

type CommitSignatory struct {
	Name      string `json:"name"`
	Email     string `json:"email"`
	Timestamp int64  `json:"timestamp"`
}

type CommitResult struct {
	Committer    *CommitSignatory `json:"committer"`
	Author       *CommitSignatory `json:"author"`
	Message      string           `json:"message"`
	Hash         string           `json:"hash"`
	ParentHashes []string         `json:"parents"`
}

type ListPathValue struct {
	Name              string `json:"name"`
	BlobHash          string `json:"blobHash"`
	IsDir             bool   `json:"isDir"`
	Size              int64  `json:"size"`
	IsBinary          bool   `json:"isBinary"`
	LastCommitMessage string `json:"lastCommitMsg"`
	LastCommitHash    string `json:"lastCommitHash"`
	UpdatedAt         int64  `json:"updatedAt"`
}

type LocalConfig struct {
	Tokens map[string][]string `json:"tokens"`
}

// EmptyLocalConfig returns an instance of LocalConfig with fields initialized to zero values.
func EmptyLocalConfig() *LocalConfig {
	return &LocalConfig{Tokens: make(map[string][]string)}
}

type RefFetchArgs struct {
	Remote    string
	RemoteRef string
	LocalRef  string
	Force     bool
	Verbose   bool
}

type GitModule interface {
	RefDelete(refname string) error
	RefUpdate(refname, commitHash string) error
	TagDelete(tagname string) error
	RefGet(refname string) (string, error)
	GetRecentCommitHash() (string, error)
	GetHEAD(short bool) (string, error)
	NumCommits(branch string, noMerges bool) (int, error)
	CreateEmptyCommit(msg, signingKey string, env ...string) error
	CreateTagWithMsg(args []string, msg, signingKey string, env ...string) error
	ListTreeObjects(treename string, recursive bool, env ...string) (map[string]string, error)
	ListTreeObjectsSlice(treename string, recursive, showTrees bool, env ...string) ([]string, error)
	RemoveEntryFromNote(notename, objectHash string, env ...string) error
	AddEntryToNote(notename, objectHash, note string, env ...string) error
	CreateBlob(content string) (string, error)
	AmendRecentCommitWithMsg(msg, signingKey string, env ...string) error
	HasMergeCommits(reference string, env ...string) (bool, error)
	GetMergeCommits(reference string, env ...string) ([]string, error)
	CreateSingleFileCommit(filename, content, commitMsg, parent string) (string, error)
	Checkout(refname string, create, force bool) error
	GetRefRootCommit(ref string) (string, error)
	GetRefCommits(ref string, noMerges bool) ([]string, error)
	Var(name string) (string, error)
	ExpandShortHash(hash string) (string, error)
	RefFetch(args RefFetchArgs) error
	GC(pruneExpire ...string) error
	Size() (size float64, err error)
	GetPathLogInfo(path string, revision ...string) (*PathLogInfo, error)
	DiffCommits(commitA, commitB string) (string, error)
}

type PathLogInfo struct {
	// LastUpdateAt is the time the updated happened
	LastUpdateAt time.Time
	// LastCommitHash is the hash of the commit where the updated occurred
	LastCommitHash string
	// LastCommitMessage is the update commit message
	LastCommitMessage string
}

// Commit represents a Commit.
type Commit interface {
	object.Object

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

	// Tree returns the tree from the commit
	Tree() (*object.Tree, error)

	// File returns the file with the specified "path" in the commit and a
	// nil error if the file exists.
	File(path string) (*object.File, error)
}
