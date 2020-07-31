package repo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	plumbing2 "github.com/themakeos/lobe/remote/plumbing"
	"github.com/themakeos/lobe/remote/types"
	"github.com/themakeos/lobe/types/state"
	config2 "gopkg.in/src-d/go-git.v4/plumbing/format/config"
	"gopkg.in/src-d/go-git.v4/plumbing/storer"

	"gopkg.in/src-d/go-git.v4/config"
	"gopkg.in/src-d/go-git.v4/storage"

	"github.com/pkg/errors"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

var ErrNotAnAncestor = fmt.Errorf("not an ancestor")

// Get opens a local repository and returns a handle.
func Get(path string) (types.LocalRepo, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, err
	}
	return &Repo{
		Repository: repo,
		Path:       path,
	}, nil
}

// GetLocalRepoFunc describes a function for getting a local repository handle
type GetLocalRepoFunc func(gitBinPath, path string) (types.LocalRepo, error)

func GetWithLiteGit(gitBinPath, path string) (types.LocalRepo, error) {
	r, err := Get(path)
	if err != nil {
		return nil, err
	}
	r.(*Repo).LiteGit = NewLiteGit(gitBinPath, path)
	return r, nil
}

// GetAtWorkingDir returns a RepoContext instance pointed to the repository
// in the current working directory.
func GetAtWorkingDir(gitBinDir string) (types.LocalRepo, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get current working directory")
	}

	// Since we expect the working directory to be a git working tree,
	// we need to get a repo instance to verify it
	repo, err := GetWithLiteGit(gitBinDir, wd)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open repository")
	} else if repoCfg, _ := repo.Config(); repoCfg.Core.IsBare {
		return nil, errors.New("expected a working tree. this is a bare repository")
	}

	return repo, nil
}

// GetObjectsSize returns the total size of the given objects.
func GetObjectsSize(repo types.LocalRepo, objects []string) (uint64, error) {
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

// Repo provides functions for accessing and modifying
// a repository loaded by the remote server.
type Repo struct {
	*LiteGit
	*git.Repository
	Path          string
	NamespaceName string
	Namespace     *state.Namespace
	State         *state.Repository
}

// GetState returns the repository's network state
func (r *Repo) GetState() *state.Repository {
	return r.State
}

// Tags return all tag references in the repository.
// If you want to check to see if the tag is an annotated tag, you can call
// TagObject on the hash Reference
func (r *Repo) Tags() (storer.ReferenceIter, error) {
	return r.Repository.Tags()
}

// SetState sets the repository's network state
func (r *Repo) SetState(s *state.Repository) {
	r.State = s
}

// Head returns the reference where HEAD is pointing to.
func (r *Repo) Head() (string, error) {
	ref, err := r.Repository.Head()
	if err != nil {
		return "", err
	}
	return ref.Name().String(), nil
}

// GetPath returns the bare repository path.
func (r *Repo) GetPath() string {
	return r.Path
}

// IsClean checks whether the working directory has no un-tracked, staged or modified files
func (r *Repo) IsClean() (bool, error) {
	wt, err := r.Repository.Worktree()
	if err != nil {
		return false, err
	}
	status, err := wt.Status()
	if err != nil {
		return false, err
	}
	return len(status) == 0, nil
}

// SetPath sets the repository root path
func (r *Repo) SetPath(path string) {
	r.Path = path
}

// GetConfig finds and returns a config value
func (lg *Repo) GetConfig(path string) string {
	cfg, _ := lg.Config()
	if cfg == nil {
		return ""
	}

	pathParts := strings.Split(path, ".")

	// If path does not contain a section and a key (e.g: section.key),
	// return empty result
	if partsLen := len(pathParts); partsLen < 2 || partsLen > 3 {
		return ""
	}

	var sec interface{} = cfg.Raw.Section(pathParts[0])
	for i, part := range pathParts[1:] {
		if i == len(pathParts[1:])-1 {
			if o, ok := sec.(*config2.Subsection); ok {
				return o.Option(part)
			} else {
				return sec.(*config2.Section).Option(part)
			}
		}
		sec = sec.(*config2.Section).Subsection(part)
	}

	return ""
}

// WrappedCommitObject returns commit that implements types.WrappedCommit interface.
func (r *Repo) WrappedCommitObject(h plumbing.Hash) (types.Commit, error) {
	commit, err := r.CommitObject(h)
	if err != nil {
		return nil, err
	}
	return &WrappedCommit{Commit: commit}, nil
}

// SetConfig sets the repo config
func (r *Repo) SetConfig(cfg *config.Config) error {
	return r.Storer.SetConfig(cfg)
}

// IsAncestor checks whether commitA is an ancestor to commitB.
// It returns ErrNotAncestor when not an ancestor.
// It returns ErrObjectNotFound if commit A or B does not exist.
func (r *Repo) IsAncestor(commitA, commitB string) error {
	cA, err := r.CommitObject(plumbing.NewHash(commitA))
	if err != nil {
		return err
	}

	cB, err := r.CommitObject(plumbing.NewHash(commitB))
	if err != nil {
		return err
	}

	yes, err := cA.IsAncestor(cB)
	if err != nil {
		return err
	} else if !yes {
		return ErrNotAnAncestor
	}

	return err
}

// GetReferences returns all references in the repo
func (r *Repo) GetReferences() (refs []plumbing.ReferenceName, err error) {
	itr, err := r.References()
	if err != nil {
		return nil, err
	}
	itr.ForEach(func(reference *plumbing.Reference) error {
		refName := reference.Name()
		refs = append(refs, refName)
		return nil
	})
	return
}

// GetName returns the name of the repo
func (r *Repo) GetName() string {
	return r.getNameFromPath()
}

// getNameFromPath returns the name of the repo
func (r *Repo) getNameFromPath() string {
	_, name := filepath.Split(r.Path)
	return name
}

// GetNamespaceName returns the name of the repo's namespace
func (r *Repo) GetNamespaceName() string {
	return r.NamespaceName
}

// GetNamespace returns the repos's namespace
func (r *Repo) GetNamespace() *state.Namespace {
	return r.Namespace
}

// IsContributor checks whether a push key is a contributor to either
// the repository or its namespace
func (r *Repo) IsContributor(pushKeyID string) (isContrib bool) {
	if s := r.GetState(); s != nil {
		if s.Contributors.Has(pushKeyID) {
			return true
		}
	}
	if ns := r.GetNamespace(); ns != nil {
		return ns.Contributors.Has(pushKeyID)
	}
	return
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

// GetObjectSize returns the size of a decompressed object
func (r *Repo) GetObjectSize(objHash string) (int64, error) {
	return r.Storer.EncodedObjectSize(plumbing.NewHash(objHash))
}

// ObjectsOfCommit returns a hashes of objects a commit is composed of.
// This objects a the commit itself, its tree and the tree blobs.
func (r *Repo) ObjectsOfCommit(hash string) ([]plumbing.Hash, error) {
	commit, err := r.CommitObject(plumbing.NewHash(hash))
	if err != nil {
		return nil, err
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, err
	}

	hashes := []plumbing.Hash{commit.Hash, commit.TreeHash}
	for _, e := range tree.Entries {
		hashes = append(hashes, e.Hash)
	}
	return hashes, nil
}

// GetStorer returns the storage engine of the repository
func (r *Repo) GetStorer() storage.Storer {
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

// NumIssueBranches counts the number of issues branches
func (r *Repo) NumIssueBranches() (count int, err error) {
	refIter, err := r.References()
	if err != nil {
		return 0, err
	}
	refIter.ForEach(func(reference *plumbing.Reference) error {
		if plumbing2.IsIssueReference(reference.Name().String()) {
			count++
		}
		return nil
	})
	return count, nil
}

// GetAncestors returns the ancestors of the given commit up til the ancestor matching the stop hash.
// The stop hash ancestor is not included in the result.
// Reverse reverses the result.
func (r *Repo) GetAncestors(commit *object.Commit, stopHash string, reverse bool) (ancestors []*object.Commit, err error) {
	var next = commit
	for {
		if next.NumParents() == 0 {
			break
		}
		ancestor, err := next.Parent(0)
		if err != nil {
			return nil, err
		}
		if ancestor.Hash.String() == stopHash {
			break
		}
		ancestors = append(ancestors, ancestor)
		next = ancestor
	}

	if reverse {
		for i := len(ancestors)/2 - 1; i >= 0; i-- {
			opp := len(ancestors) - 1 - i
			ancestors[i], ancestors[opp] = ancestors[opp], ancestors[i]
		}
	}

	return
}
