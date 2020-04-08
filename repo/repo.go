package repo

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"gitlab.com/makeos/mosdef/types/core"
	state2 "gitlab.com/makeos/mosdef/types/state"

	"gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/storage"

	"github.com/thoas/go-funk"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/format/objfile"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"

	"github.com/pkg/errors"
)

// Repo represents a git repository
type Repo struct {
	git       *git.Repository
	path      string
	name      string
	namespace string
	ops       *GitOps
	state     *state2.Repository
}

// State returns the repository's network state
func (r *Repo) State() *state2.Repository {
	return r.state
}

// Head returns the reference where HEAD is pointing to.
func (r *Repo) Head() (string, error) {
	ref, err := r.git.Head()
	if err != nil {
		return "", err
	}
	return ref.Name().String(), nil
}

// Path returns the repository's path
func (r *Repo) Path() string {
	return r.path
}

// SetPath sets the repository root path
func (r *Repo) SetPath(path string) {
	r.path = path
}

// References returns an unsorted ReferenceIter for all references.
func (r *Repo) References() (storer.ReferenceIter, error) {
	return r.git.References()
}

// RefDelete executes `git update-ref -d <refname>` to delete a reference
func (r *Repo) RefDelete(refname string) error {
	return r.ops.RefDelete(refname)
}

// RefUpdate executes `git update-ref <refname> <commit hash>` to update/create a reference
func (r *Repo) RefUpdate(refname, commitHash string) error {
	return r.ops.RefUpdate(refname, commitHash)
}

// RefGet returns the hash content of a reference.
func (r *Repo) RefGet(refname string) (string, error) {
	return r.ops.RefGet(refname)
}

// TagDelete executes `git tag -d <tagname>` to delete a tag
func (r *Repo) TagDelete(tagname string) error {
	return r.ops.TagDelete(tagname)
}

// ListTreeObjects executes `git tag -d <tagname>` to delete a tag
func (r *Repo) ListTreeObjects(treename string, recursive bool, env ...string) (map[string]string,
	error) {
	return r.ops.ListTreeObjects(treename, recursive, env...)
}

// DeleteObject deletes an object from a repository.
func (r *Repo) DeleteObject(hash plumbing.Hash) error {
	return r.git.DeleteObject(hash)
}

// Reference returns the reference for a given reference name.
func (r *Repo) Reference(name plumbing.ReferenceName, resolved bool) (*plumbing.Reference, error) {
	return r.git.Reference(name, resolved)
}

// Object returns an Object with the given hash.
func (r *Repo) Object(t plumbing.ObjectType, h plumbing.Hash) (object.Object, error) {
	return r.git.Object(t, h)
}

// Objects returns an unsorted ObjectIter with all the objects in the repository.
func (r *Repo) Objects() (*object.ObjectIter, error) {
	return r.git.Objects()
}

// CommitObjects returns an unsorted ObjectIter with all the objects in the repository.
func (r *Repo) CommitObjects() (object.CommitIter, error) {
	return r.git.CommitObjects()
}

// CommitObject returns an unsorted ObjectIter with all the objects in the repository.
func (r *Repo) CommitObject(h plumbing.Hash) (*object.Commit, error) {
	return r.git.CommitObject(h)
}

// WrappedCommitObject returns commit that implements types.Commit interface.
func (r *Repo) WrappedCommitObject(h plumbing.Hash) (core.Commit, error) {
	commit, err := r.git.CommitObject(h)
	if err != nil {
		return nil, err
	}
	return &WrappedCommit{commit}, nil
}

// BlobObject returns a Blob with the given hash.
func (r *Repo) BlobObject(h plumbing.Hash) (*object.Blob, error) {
	return r.git.BlobObject(h)
}

// TagObject returns a Tag with the given hash.
func (r *Repo) TagObject(h plumbing.Hash) (*object.Tag, error) {
	return r.git.TagObject(h)
}

// Tag returns a tag from the repository.
func (r *Repo) Tag(name string) (*plumbing.Reference, error) {
	return r.git.Tag(name)
}

// Config return the repository config
func (r *Repo) Config() (*config.Config, error) {
	return r.git.Config()
}

// SetConfig sets the repo config
func (r *Repo) SetConfig(cfg *config.Config) error {
	return r.git.Storer.SetConfig(cfg)
}

// GetConfig finds and returns a config value
func (r *Repo) GetConfig(path string) string {
	return r.ops.GetConfig(path)
}

// GetRecentCommit gets the hash of the recent commit.
// Returns ErrNoCommits if no commits exist
func (r *Repo) GetRecentCommit() (string, error) {
	return r.ops.GetRecentCommit()
}

// CreateAndOrSignQuietCommit creates and optionally sign a quiet commit.
// msg: The commit message.
// signingKey: The optional signing key. If provided, the commit is signed
// env: Optional environment variables to pass to the command.
func (r *Repo) CreateAndOrSignQuietCommit(msg, signingKey string, env ...string) error {
	return r.ops.CreateAndOrSignQuietCommit(msg, signingKey, env...)
}

// CreateTagWithMsg an annotated tag.
// args: `git tag` options (NOTE: -a and --file=- are added by default)
// msg: The tag's message which is passed to the command's stdin.
// signingKey: The signing key to use
// env: Optional environment variables to pass to the command.
func (r *Repo) CreateTagWithMsg(args []string, msg, signingKey string, env ...string) error {
	return r.ops.CreateTagWithMsg(args, msg, signingKey, env...)
}

// RemoveEntryFromNote removes a note
func (r *Repo) RemoveEntryFromNote(notename, objectHash string, env ...string) error {
	return r.ops.RemoveEntryFromNote(notename, objectHash, env...)
}

// CreateBlob creates a blob object
func (r *Repo) CreateBlob(content string) (string, error) {
	return r.ops.CreateBlob(content)
}

// AddEntryToNote adds a note
func (r *Repo) AddEntryToNote(notename, objectHash, note string, env ...string) error {
	return r.ops.AddEntryToNote(notename, objectHash, note, env...)
}

// ListTreeObjectsSlice returns a slice containing objects name of tree entries
func (r *Repo) ListTreeObjectsSlice(treename string, recursive, showTrees bool,
	env ...string) ([]string, error) {
	return r.ops.ListTreeObjectsSlice(treename, recursive, showTrees, env...)
}

// GetName returns the name of the repo
func (r *Repo) GetName() string {
	return r.name
}

// GetNameFromPath returns the name of the repo
func (r *Repo) GetNameFromPath() string {
	_, name := filepath.Split(r.path)
	return name
}

// GetNamespace returns the namespace this repo is associated to.
func (r *Repo) GetNamespace() string {
	return r.namespace
}

// UpdateRecentCommitMsg updates the recent commit message.
// msg: The commit message which is passed to the command's stdin.
// signingKey: The signing key
// env: Optional environment variables to pass to the command.
func (r *Repo) UpdateRecentCommitMsg(msg, signingKey string, env ...string) error {
	return r.ops.UpdateRecentCommitMsg(msg, signingKey, env...)
}

// GetRemoteURLs returns the remote URLS of the repository
func (r *Repo) GetRemoteURLs() (urls []string) {
	remotes, err := r.git.Remotes()
	if err != nil {
		return
	}
	for _, r := range remotes {
		urls = append(urls, r.Config().URLs...)
	}
	return
}

// ObjectExist checks whether an object exist in the target repository
func (r *Repo) ObjectExist(objHash string) bool {
	_, err := r.Object(plumbing.AnyObject, plumbing.NewHash(objHash))
	return err == nil
}

// GetObject returns an object
func (r *Repo) GetObject(objHash string) (object.Object, error) {
	obj, err := r.Object(plumbing.AnyObject, plumbing.NewHash(objHash))
	if err != nil {
		return nil, err
	}
	return obj, nil
}

// GetEncodedObject returns an object in decompressed state
func (r *Repo) GetEncodedObject(objHash string) (plumbing.EncodedObject, error) {
	obj, err := r.Object(plumbing.AnyObject, plumbing.NewHash(objHash))
	if err != nil {
		return nil, err
	}
	encoded := &plumbing.MemoryObject{}
	if err = obj.Encode(encoded); err != nil {
		return nil, err
	}
	return encoded, nil
}

// GetObjectSize returns the size of a decompressed object
func (r *Repo) GetObjectSize(objHash string) (int64, error) {
	obj, err := r.GetEncodedObject(objHash)
	if err != nil {
		return 0, err
	}
	return obj.Size(), nil
}

// GetObjectDiskSize returns the size of the object as it exist on the system
func (r *Repo) GetObjectDiskSize(objHash string) (int64, error) {
	path := filepath.Join(r.path, "objects", objHash[:2], objHash[2:])
	fi, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

// WriteObjectToFile writes an object to the repository's objects store
func (r *Repo) WriteObjectToFile(objectHash string, content []byte) error {

	objDir := filepath.Join(r.path, "objects", objectHash[:2])
	os.MkdirAll(objDir, 0700)

	fullPath := filepath.Join(objDir, objectHash[2:])
	err := ioutil.WriteFile(fullPath, content, 0644)
	if err != nil {
		return errors.Wrap(err, "failed to write object")
	}

	return nil
}

// GetCompressedObject compressed version of an object
func (r *Repo) GetCompressedObject(hash string) ([]byte, error) {
	obj, err := r.GetEncodedObject(hash)
	if err != nil {
		return nil, err
	}

	rdr, err := obj.Reader()
	if err != nil {
		return nil, err
	}

	var buf = bytes.NewBuffer(nil)
	objW := objfile.NewWriter(buf)
	defer objW.Close()
	if err := objW.WriteHeader(obj.Type(), obj.Size()); err != nil {
		return nil, err
	}

	if _, err := io.Copy(objW, rdr); err != nil {
		return nil, err
	}

	objW.Close()

	return buf.Bytes(), nil
}

// GetHost returns the storage engine of the repository
func (r *Repo) GetHost() storage.Storer {
	return r.git.Storer
}

// Prune deletes objects older than the given time
func (r *Repo) Prune(olderThan time.Time) error {
	return r.git.Prune(git.PruneOptions{
		OnlyObjectsOlderThan: olderThan,
		Handler: func(hash plumbing.Hash) error {
			return r.DeleteObject(hash)
		},
	})
}

// Get returns a repository
func GetRepo(path string) (core.BareRepo, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, err
	}
	return &Repo{
		git:  repo,
		path: path,
	}, nil
}

func getRepoWithGitOpt(gitBinPath, path string) (core.BareRepo, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, err
	}
	return &Repo{
		ops:  NewGitOps(gitBinPath, path),
		git:  repo,
		path: path,
	}, nil
}

// GetCurrentWDRepo returns a Repo instance pointed to the repository
// in the current working directory.
func GetCurrentWDRepo(gitBinDir string) (core.BareRepo, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get current working directory")
	}

	// Since we expect the working directory to be a git working tree,
	// we need to get a repo instance to verify it
	repo, err := getRepoWithGitOpt(gitBinDir, wd)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open repository")
	} else if repoCfg, _ := repo.Config(); repoCfg.Core.IsBare {
		return nil, errors.New("expected a working tree. this is a bare repository")
	}

	return repo, nil
}

// getTreeEntries returns all entries in a tree.
func getTreeEntries(repo core.BareRepo, treeHash string) ([]string, error) {
	entries, err := repo.ListTreeObjectsSlice(treeHash, true, true)
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// getCommitHistory gets all objects that led/make up to the given commit, such as
// parent commits, trees and blobs.
// repo: The target repository
// commit: The target commit
// stopCommitHash: A commit hash that when found triggers the end of the search.
func getCommitHistory(repo core.BareRepo, commit *object.Commit, stopCommitHash string) ([]string, error) {
	var hashes []string

	// Stop if commit hash matches the stop hash
	if commit.Hash.String() == stopCommitHash {
		return hashes, nil
	}

	// Register the commit and the tree hash
	hashes = append(hashes, commit.Hash.String())
	hashes = append(hashes, commit.TreeHash.String())

	// Get entries of the tree (blobs and sub-trees)
	entries, err := getTreeEntries(repo, commit.TreeHash.String())
	if err != nil {
		return nil, err
	}
	hashes = append(hashes, entries...)

	// Perform same operation on the parents of the commit
	err = commit.Parents().ForEach(func(parent *object.Commit) error {
		childHashes, err := getCommitHistory(repo, parent, stopCommitHash)
		if err != nil {
			return err
		}
		hashes = append(hashes, childHashes...)
		return nil
	})

	return funk.UniqString(hashes), err
}

// getObjectsSize returns the total size of the given objects.
func getObjectsSize(repo core.BareRepo, objects []string) (uint64, error) {
	var size int64
	for _, hash := range objects {
		objSize, err := repo.GetObjectSize(hash)
		if err != nil {
			return 0, err
		}
		size += objSize
	}
	return uint64(size), nil
}
