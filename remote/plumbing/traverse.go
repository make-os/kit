package plumbing

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/themakeos/lobe/remote/types"
	types2 "github.com/themakeos/lobe/types"
	"github.com/thoas/go-funk"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

// WalkBack traverses the history of a commit and returns all objects discovered up til
// the given end object.
// If start object is not a commit, no traversing operation happens
// but the object, along with its related objects (trees, blobs or target) are returned.
// End object must exist locally; If it is a tag, its target must be a commit.
func WalkBack(localRepo types.LocalRepo, startHash, endHash string, cb func(hash string) error) error {
start_hash_check:

	// Unset end hash if it is a zero hash.
	if IsZeroHash(endHash) {
		endHash = ""
	}

	// Get the start object
	start, err := localRepo.GetObject(startHash)
	if err != nil {
		return errors.Wrap(err, "failed to get start object")
	}

	// Set noop callback if callback is unset
	if cb == nil {
		cb = func(string) error { return nil }
	}

	// When the start object is a tag, the target is expected to be a commit.
	// If target is another tag, we need to dereference the tag and get the target.
	// If target is a commit, use it as the start hash.
	// If target is not a commit, return the target and its packable objects, then exit immediately.
	switch start.Type() {
	case plumbing.TagObject:
		tag := start.(*object.Tag)
		if err := cb(tag.Hash.String()); err != nil {
			if err == types2.ErrExit {
				return nil
			}
			return err
		}
		switch tag.TargetType {
		case plumbing.TagObject, plumbing.CommitObject:
			startHash = tag.Target.String()
			goto start_hash_check
		default:
			target, err := localRepo.GetObject(tag.Target.String())
			if err != nil {
				return err
			}
			objs, err := GetPackableObjects(localRepo, target)
			if err != nil {
				return err
			}
			for _, hash := range objs {
				obj, _ := localRepo.GetObject(hash.String())
				if err := cb(obj.ID().String()); err != nil {
					if err == types2.ErrExit {
						return nil
					}
					return err
				}
			}
			return nil
		}
	case plumbing.CommitObject:
	default:
		objs, err := GetPackableObjects(localRepo, start)
		if err != nil {
			return err
		}
		for _, hash := range objs {
			obj, _ := localRepo.GetObject(hash.String())
			if err := cb(obj.ID().String()); err != nil {
				if err == types2.ErrExit {
					return nil
				}
				return err
			}
		}
		return nil
	}

	// If end hash is set and its a tag, we need to dereference it and use its target as the end object.
	// - If the tag's target is a commit, use the commit as the end hash.
	//   e.g [c1]-[c2]-[c3]   - c1-c3 are commits
	//				\__[t1]   - t1 is a tag that is also the end hash.
	//   If [t1] is the end hash and [c3] the start hash, it will cause the walk algorithm
	// 	 to traverse over all commits non-stop because [t1] is not an ancestor. To solve
	// 	 this problem, we need to dereference the tag till we find a commit [c2] to use
	// 	 as the end hash.
	// - If the tag's target is another tag, we dereference the tag recursively.
	// - If the end object or tag's target is not a tag or a commit, we return error as we can't
	//   walk back to such objects.
	var end object.Object
	if endHash != "" {
	end_hash_check:
		end, err = localRepo.GetObject(endHash)
		if err != nil {
			return errors.Wrap(err, "failed to get end object")
		}
		if end.Type() == plumbing.TagObject {
			tag := end.(*object.Tag)
			switch tag.TargetType {
			case plumbing.TagObject, plumbing.CommitObject:
				endHash = tag.Target.String()
				goto end_hash_check
			default:
				return fmt.Errorf("end object must be a tag or a commit")
			}
		}
		if end.Type() != plumbing.CommitObject && end.Type() != plumbing.TagObject {
			return fmt.Errorf("end object must be a tag or a commit")
		}
	}

	walkerArgs := &WalkCommitHistoryArgs{
		Commit: start.(*object.Commit),
		Res: func(objHash string) error {
			return cb(objHash)
		},
	}
	if end != nil {
		walkerArgs.StopHash = end.ID().String()
	}
	err = WalkCommitHistoryWithIteratee(localRepo, walkerArgs)
	if err != nil {
		return err
	}

	return nil
}

// GetTreeEntries returns all entries in a tree.
func GetTreeEntries(repo types.LocalRepo, treeHash string) ([]string, error) {
	entries, err := repo.ListTreeObjectsSlice(treeHash, true, true)
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// WalkCommitHistory walks back a commit's history,
// returning its ancestors and other git objects they contain.
// stopHash stops the walk when an object matching the hash is found.
// Ancestors of the stop object will not be traversed.
func WalkCommitHistory(repo types.LocalRepo, commit *object.Commit, stopHash string) (hashes []string, err error) {
	err = WalkCommitHistoryWithIteratee(repo, &WalkCommitHistoryArgs{
		Commit:   commit,
		StopHash: stopHash,
		Res: func(objHash string) error {
			hashes = append(hashes, objHash)
			return nil
		},
	})
	return funk.UniqString(hashes), err
}

// WalkCommitHistoryArgs contains arguments for WalkCommitHistoryWithIteratee
type WalkCommitHistoryArgs struct {
	Commit   *object.Commit
	StopHash string
	Res      func(objHash string) error
}

// WalkCommitHistoryWithIteratee walks back a commit's history,
// returning its ancestors and other git objects they contain.
// repo: The target repository
// args.Commit: The target/start commit
// args.StopHash: Stops the walk when an object matching the hash is found. Ancestors
//  of the stop object will not be traversed.
// args.Res: A callback that will be called with any found object. Return ErrExit
// to stop immediately without error or return other error types to end immediately
// with the specified error.
func WalkCommitHistoryWithIteratee(repo types.LocalRepo, args *WalkCommitHistoryArgs) error {

	// Get stop object
	stopObject, err := repo.GetObject(args.StopHash)
	if err != nil && err != plumbing.ErrObjectNotFound {
		return err
	}

	// Stop if commit hash matches the stop hash
	if args.Commit.Hash.String() == args.StopHash {
		return nil
	}

	// If stop object exist and it is a descendant of the commit, exit with nil error.
	// This prevents us from walking over objects that are ancestors of the stop object.
	if stopObject != nil && repo.IsAncestor(args.Commit.Hash.String(), args.StopHash) == nil {
		return nil
	}

	// Set noop callback if response callback is unset
	if args.Res == nil {
		args.Res = func(string) error { return nil }
	}

	// Collect the commit and the tree hash
	for _, hash := range append([]string{}, args.Commit.Hash.String(), args.Commit.TreeHash.String()) {
		if hash == args.StopHash {
			return nil
		}
		if err := args.Res(hash); err != nil {
			if err == types2.ErrExit {
				return nil
			}
			return err
		}
	}

	// Get entries of the tree (blobs and sub-trees)
	entries, err := GetTreeEntries(repo, args.Commit.TreeHash.String())
	if err != nil {
		return err
	} else {
		for _, entry := range entries {
			if entry == args.StopHash {
				return nil
			}
			if err := args.Res(entry); err != nil {
				if err == types2.ErrExit {
					return nil
				}
				return err
			}
		}
	}

	// Perform same operation on the parents of the commit
	err = args.Commit.Parents().ForEach(func(parent *object.Commit) error {
		args.Commit = parent
		err := WalkCommitHistoryWithIteratee(repo, args)
		if err != nil {
			return err
		}
		return nil
	})

	return err
}
