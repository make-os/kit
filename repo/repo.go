package repo

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"

	"gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/storage"

	"github.com/pkg/errors"
	"github.com/thoas/go-funk"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/format/objfile"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

// Repo represents a git repository
type Repo struct {
	*LiteGit
	*git.Repository
	path      string
	name      string
	namespace string
	state     *state.Repository
}

// State returns the repository's network state
func (r *Repo) State() *state.Repository {
	return r.state
}

// Head returns the reference where HEAD is pointing to.
func (r *Repo) Head() (string, error) {
	ref, err := r.Repository.Head()
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

// WrappedCommitObject returns commit that implements types.WrappedCommit interface.
func (r *Repo) WrappedCommitObject(h plumbing.Hash) (core.Commit, error) {
	commit, err := r.CommitObject(h)
	if err != nil {
		return nil, err
	}
	return &WrappedCommit{commit}, nil
}

// SetConfig sets the repo config
func (r *Repo) SetConfig(cfg *config.Config) error {
	return r.Storer.SetConfig(cfg)
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

// GetRemoteURLs returns the remote URLS of the repository
func (r *Repo) GetRemoteURLs() (urls []string) {
	remotes, err := r.Remotes()
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
	return r.Storer
}

// Prune deletes objects older than the given time
func (r *Repo) Prune(olderThan time.Time) error {
	return r.Repository.Prune(git.PruneOptions{
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
		Repository: repo,
		path:       path,
	}, nil
}

func getRepoWithLiteGit(gitBinPath, path string) (core.BareRepo, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, err
	}
	return &Repo{
		LiteGit:    NewLiteGit(gitBinPath, path),
		Repository: repo,
		path:       path,
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
	repo, err := getRepoWithLiteGit(gitBinDir, wd)
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
