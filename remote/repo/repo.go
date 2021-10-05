package repo

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	config2 "github.com/go-git/go-git/v5/plumbing/format/config"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/storage"
	plumbing2 "github.com/make-os/kit/remote/plumbing"
	"github.com/make-os/kit/remote/types"
	"github.com/make-os/kit/types/state"
	"github.com/make-os/kit/util"
	"github.com/pkg/errors"
	"github.com/thoas/go-funk"
)

var (
	ErrNotAnAncestor = fmt.Errorf("not an ancestor")
	ErrPathNotFound  = fmt.Errorf("path not found")
	ErrPathNotAFile  = fmt.Errorf("path is not a file")
)

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

func GetWithGitModule(gitBinPath, path string) (types.LocalRepo, error) {
	r, err := Get(path)
	if err != nil {
		return nil, err
	}
	r.(*Repo).GitModule = NewGitModule(gitBinPath, path)
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
	repo, err := GetWithGitModule(gitBinDir, wd)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open repository")
	} else if repoCfg, _ := repo.Config(); repoCfg.Core.IsBare {
		return nil, errors.New("expected a working tree. this is a bare repository")
	}

	return repo, nil
}

// Repo provides functions for accessing and modifying a local repository.
type Repo struct {
	*GitModule
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

// HeadObject returns the object of the HEAD reference.
// Returns plumbing.ErrReferenceNotFound if HEAD was not found.
func (r *Repo) HeadObject() (object.Object, error) {
	ref, err := r.Repository.Head()
	if err != nil {
		return nil, err
	}
	return r.Repository.Object(plumbing.AnyObject, ref.Hash())
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

// GetGitConfigOption finds and returns git config option value
func (r *Repo) GetGitConfigOption(path string) string {
	cfg, _ := r.Config()
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

// Reload reloads the repository
func (r *Repo) Reload() error {
	repo, err := Get(r.path)
	if err != nil {
		return err
	}
	r.Repository = repo.(*Repo).Repository
	return nil
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

// GetNamespace returns the repo's namespace
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

// GetRemoteURLs returns the remote URLS of the repository.
// Use `names` to select specific remotes with matching name.
func (r *Repo) GetRemoteURLs(names ...string) (urls []string) {
	remotes, err := r.Remotes()
	if err != nil {
		return
	}
	for _, r := range remotes {
		if len(names) > 0 && !funk.Contains(names, r.Config().Name) {
			continue
		}
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

// UpdateRepoConfig updates the repo's 'repocfg' configuration file
func (r *Repo) UpdateRepoConfig(cfg *types.LocalConfig) (err error) {

	var f *os.File
	cfgFile := filepath.Join(r.Path, ".git", "repocfg")
	if !util.IsFileOk(cfgFile) {
		f, err = os.Create(cfgFile)
		if err != nil {
			return errors.Wrap(err, "failed to create repo config file")
		}
		defer f.Close()
	}

	if f == nil {
		f, err = os.OpenFile(cfgFile, os.O_RDWR, 0644)
		if err != nil {
			return errors.Wrap(err, "failed to open repo config file")
		}
		defer f.Close()
	}

	return json.NewEncoder(f).Encode(cfg)
}

// GetRepoConfig returns the repo's 'repocfg' config object.
// Returns an empty LocalConfig and nil if no repo config file was found
func (r *Repo) GetRepoConfig() (*types.LocalConfig, error) {

	cfgFile := filepath.Join(r.Path, ".git", "repocfg")
	if !util.IsFileOk(cfgFile) {
		return types.EmptyLocalConfig(), nil
	}

	bz, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		return nil, err
	}

	var cfg = types.EmptyLocalConfig()
	if err := json.Unmarshal(bz, cfg); err != nil {
		return nil, err
	}

	if cfg.Tokens == nil {
		cfg.Tokens = map[string][]string{}
	}

	return cfg, nil
}

// ListPath lists entries in a given path on the given reference.
func (r *Repo) ListPath(ref, path string) (res []types.ListPathValue, err error) {

	reference, err := r.Reference(plumbing.ReferenceName(ref), true)
	if err != nil {
		return nil, err
	}

	commit, err := r.CommitObject(reference.Hash())
	if err != nil {
		return nil, err
	}

	var targetEntry *object.TreeEntry

	tree, err := commit.Tree()
	if err != nil {
		return nil, err
	}
	if path == "." || path == "" {
		targetEntry = &object.TreeEntry{Mode: filemode.Dir}
		path = "."
		goto handleEntry
	}

	targetEntry, err = tree.FindEntry(path)
	if err != nil {
		if err == object.ErrEntryNotFound {
			return nil, ErrPathNotFound
		}
		return nil, err
	} else if targetEntry.Mode == filemode.Dir {
		tree, _ = tree.Tree(path)
	} else {
		path, _ = filepath.Split(path)
		if path != "" {
			tree, err = tree.Tree(filepath.Clean(path))
		}
	}

handleEntry:
	processEntry := func(entry object.TreeEntry, tree *object.Tree) {
		item := types.ListPathValue{}
		item.Name = entry.Name
		item.IsDir = entry.Mode == filemode.Dir
		item.BlobHash = entry.Hash.String()
		if entry.Mode != filemode.Dir {
			var file *object.File
			file, err = tree.File(entry.Name)
			if err != nil {
				return
			}
			item.IsBinary, _ = file.IsBinary()
			item.Size = file.Size

			t, err2 := r.GetPathLogInfo(filepath.Join(path, entry.Name), ref)
			if err2 != nil {
				err = err2
				return
			}

			item.LastCommitMessage = t.LastCommitMessage
			item.LastCommitHash = t.LastCommitHash
			if !t.LastUpdateAt.IsZero() {
				item.UpdatedAt = t.LastUpdateAt.Unix()
			}
		} else {
			t, err2 := r.GetPathLogInfo(filepath.Join(path, entry.Name), ref)
			if err2 != nil {
				err = err2
				return
			}
			item.LastCommitMessage = t.LastCommitMessage
			item.LastCommitHash = t.LastCommitHash
			if !t.LastUpdateAt.IsZero() {
				item.UpdatedAt = t.LastUpdateAt.Unix()
			}
		}
		res = append(res, item)
	}

	switch targetEntry.Mode {
	case filemode.Dir:
		for _, entry := range tree.Entries {
			processEntry(entry, tree)
		}
	case filemode.Regular, filemode.Executable:
		processEntry(*targetEntry, tree)
	}

	return
}

// GetFileLines returns the lines of a file
//  - ref: A full reference name or commit hash
//  - path: The case-sensitive file path
func (r *Repo) GetFileLines(ref, path string) (res []string, err error) {

	var hash plumbing.Hash
	if plumbing.IsHash(ref) && !strings.HasPrefix(strings.ToLower(ref), "refs") {
		hash = plumbing.NewHash(ref)
	} else {
		reference, err := r.Reference(plumbing.ReferenceName(ref), true)
		if err != nil {
			return nil, err
		}
		hash = reference.Hash()
	}

	commit, err := r.CommitObject(hash)
	if err != nil {
		return nil, err
	}

	tree, err := commit.Tree()
	if err != nil {
		return nil, err
	}

	targetEntry, err := tree.FindEntry(path)
	if err != nil {
		if err == object.ErrEntryNotFound {
			return nil, ErrPathNotFound
		}
		return nil, err
	} else if targetEntry.Mode == filemode.Dir {
		return nil, ErrPathNotAFile
	}

	file, err := tree.TreeEntryFile(targetEntry)
	if err != nil {
		return nil, err
	}

	return file.Lines()
}

// GetFile returns the lines of a file
//  - ref: A full reference name or commit hash
//  - path: The case-sensitive file path
func (r *Repo) GetFile(ref, path string) (res string, err error) {

	var hash plumbing.Hash
	if plumbing.IsHash(ref) && !strings.HasPrefix(strings.ToLower(ref), "refs") {
		hash = plumbing.NewHash(ref)
	} else {
		reference, err := r.Reference(plumbing.ReferenceName(ref), true)
		if err != nil {
			return "", err
		}
		hash = reference.Hash()
	}

	commit, err := r.CommitObject(hash)
	if err != nil {
		return "", err
	}

	tree, err := commit.Tree()
	if err != nil {
		return "", err
	}

	targetEntry, err := tree.FindEntry(path)
	if err != nil {
		if err == object.ErrEntryNotFound {
			return "", ErrPathNotFound
		}
		return "", err
	} else if targetEntry.Mode == filemode.Dir {
		return "", ErrPathNotAFile
	}

	file, err := tree.TreeEntryFile(targetEntry)
	if err != nil {
		return "", err
	}

	return file.Contents()
}

// GetBranches returns a list of branches
func (r *Repo) GetBranches() (branches []string, err error) {
	itr, err := r.Branches()
	if err != nil {
		return nil, err
	}
	branches = []string{}
	for {
		var ref *plumbing.Reference
		ref, err = itr.Next()
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			break
		}
		branches = append(branches, ref.Name().Short())
	}
	return
}

// GetLatestCommit returns the recent commit of a branch
func (r *Repo) GetLatestCommit(branch string) (*types.BranchCommit, error) {

	branch = strings.ToLower(branch)
	var refname = plumbing.ReferenceName("refs/heads/" + branch)
	if strings.HasPrefix(branch, "refs/heads/") {
		refname = plumbing.ReferenceName(branch)
	}

	ref, err := r.Reference(refname, true)
	if err != nil {
		return nil, err
	}

	commit, err := r.CommitObject(ref.Hash())
	if err != nil {
		return nil, err
	}

	bc := &types.BranchCommit{
		Message: strings.Trim(strings.TrimSpace(commit.Message), "\n"),
		Hash:    commit.Hash.String(),
	}
	if commit.Committer != (object.Signature{}) {
		bc.Committer = &types.CommitSignatory{
			Name:      commit.Committer.Name,
			Email:     commit.Committer.Email,
			Timestamp: commit.Committer.When.Unix(),
		}
	}
	if commit.Author != (object.Signature{}) {
		bc.Author = &types.CommitSignatory{
			Name:      commit.Author.Name,
			Email:     commit.Author.Email,
			Timestamp: commit.Author.When.Unix(),
		}
	}

	for _, parent := range commit.ParentHashes {
		bc.ParentHashes = append(bc.ParentHashes, parent.String())
	}

	return bc, nil
}

// GetCommits returns commits of a branch
//  - branch: The target branch.
//  - limit: The number of commit to return. 0 means all.
func (r *Repo) GetCommits(branch string, limit int) (res []*types.BranchCommit, err error) {

	branch = strings.ToLower(branch)
	var refname = plumbing.ReferenceName("refs/heads/" + branch)
	if strings.HasPrefix(branch, "refs/heads/") {
		refname = plumbing.ReferenceName(branch)
	}

	ref, err := r.Reference(refname, true)
	if err != nil {
		return nil, err
	}

	commit, err := r.CommitObject(ref.Hash())
	if err != nil {
		return nil, err
	}

	res, err = iterCommit(commit, limit, nil, nil)
	if err != nil {
		return nil, err
	}

	return
}

// GetCommitAncestors returns ancestors of a commit with the given hash.
//  - commitHash: The hash of the commit.
//  - limit: The number of commit to return. 0 means all.
func (r *Repo) GetCommitAncestors(commitHash string, limit int) (res []*types.BranchCommit, err error) {
	commit, err := r.CommitObject(plumbing.NewHash(commitHash))
	if err != nil {
		return nil, err
	}

	res, err = iterCommit(commit, limit, nil, []plumbing.Hash{commit.Hash})
	if err != nil {
		return nil, err
	}

	return
}

// iterCommit walks the history of a commit.
//  - commit: The commit whose history will be iterated.
// 	- limit: The max. number of commit to return and iterate.
// 	- ignore: A list of commit that we do not want iterated.
//  - skip: A list of commit that will be iterated by not included in the result.
func iterCommit(
	commit *object.Commit,
	limit int,
	ignore []plumbing.Hash,
	skip []plumbing.Hash,
) (res []*types.BranchCommit, err error) {
	itr := object.NewCommitIterCTime(commit, nil, ignore)
	for {
		next, err := itr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		if funk.Contains(skip, next.Hash) {
			continue
		}

		bc := &types.BranchCommit{Message: next.Message, Hash: next.Hash.String()}
		if next.Committer != (object.Signature{}) {
			bc.Committer = &types.CommitSignatory{
				Name:      next.Committer.Name,
				Email:     next.Committer.Email,
				Timestamp: next.Committer.When.Unix(),
			}
		}
		if next.Author != (object.Signature{}) {
			bc.Author = &types.CommitSignatory{
				Name:      next.Author.Name,
				Email:     next.Author.Email,
				Timestamp: next.Author.When.Unix(),
			}
		}

		for _, parent := range next.ParentHashes {
			bc.ParentHashes = append(bc.ParentHashes, parent.String())
		}

		res = append(res, bc)

		if limit > 0 && len(res) == limit {
			break
		}
	}
	return res, nil
}
