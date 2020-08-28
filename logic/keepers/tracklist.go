package keepers

import (
	"fmt"
	"strings"

	"github.com/make-os/lobe/pkgs/tree"
	"github.com/make-os/lobe/storage"
	"github.com/make-os/lobe/types/core"
	"github.com/make-os/lobe/util"
	"github.com/make-os/lobe/util/identifier"
	"github.com/pkg/errors"
)

// TrackListKeeper manages information about repositories that the node has subscribed to.
type TrackListKeeper struct {
	db    storage.Tx
	state *tree.SafeTree
}

// NewTrackListKeeper creates an instance of TrackListKeeper
func NewTrackListKeeper(db storage.Tx, state *tree.SafeTree) *TrackListKeeper {
	return &TrackListKeeper{db: db, state: state}
}

// Add adds repositories to the track list.
//
// Target can be one or more comma-separated list of repositories or user namespaces.
//
// If a user namespace is provided, all repository targets are added.
//
// If height is provided, it will be used as the last update height.
//
// If will not re-add an already repo
func (t *TrackListKeeper) Add(targets string, height ...uint64) error {

	var final = []string{}
	for _, target := range strings.Split(targets, ",") {
		// If target is a user namespace, get the namespace and
		// add all repository target in the track list.
		target = strings.TrimSpace(target)
		if identifier.IsUserURI(target) {
			nsName := identifier.GetNamespace(target)
			ns := NewNamespaceKeeper(t.state).Get(nsName)
			if ns.IsNil() {
				return fmt.Errorf("namespace (%s) not found", nsName)
			}
			for _, t := range ns.Domains {
				if identifier.IsWholeNativeRepoURI(t) {
					final = append(final, identifier.GetDomain(t))
				}
			}
			continue
		}

		if err := identifier.IsValidResourceName(target); err != nil {
			return fmt.Errorf("target (%s) is not a valid repo identifier", target)
		}
		final = append(final, target)
	}

	var h = uint64(0)
	if len(height) > 0 {
		h = height[0]
	}

	for _, repo := range final {
		if t.Get(repo) != nil {
			continue
		}

		data := core.TrackedRepo{LastHeight: h}
		rec := storage.NewFromKeyValue(MakeTrackedRepoKey(repo), util.ToBytes(data))
		if err := t.db.Put(rec); err != nil {
			return errors.Wrap(err, "failed to add repo")
		}
	}

	return nil
}

// UpdateLastHeight resets the last update height of a repository.
// Returns error if repository is not being tracked.
func (t *TrackListKeeper) UpdateLastHeight(name string, height uint64) error {
	rec, err := t.db.Get(MakeTrackedRepoKey(name))
	if err != nil {
		if err == storage.ErrRecordNotFound {
			return fmt.Errorf("repo not tracked")
		}
		return err
	}

	var tr core.TrackedRepo
	if err = rec.Scan(&tr); err != nil {
		return err
	}

	tr.LastHeight = height
	return t.db.Put(storage.NewFromKeyValue(MakeTrackedRepoKey(name), util.ToBytes(tr)))
}

// Tracked returns a map of repositories.
func (t *TrackListKeeper) Tracked() (res map[string]*core.TrackedRepo) {
	res = make(map[string]*core.TrackedRepo)
	t.db.Iterate(MakeQueryTrackedRepoKey(), false, func(r *storage.Record) bool {
		var tr core.TrackedRepo
		r.Scan(&tr)
		res[string(storage.SplitPrefix(r.GetKey())[1])] = &tr
		return false
	})
	return
}

// Get returns a repo.
//
// Returns nil if not found
func (t *TrackListKeeper) Get(name string) *core.TrackedRepo {
	rec, err := t.db.Get(MakeTrackedRepoKey(name))
	if err != nil {
		if err == storage.ErrRecordNotFound {
			return nil
		}
		return nil
	}
	var tr core.TrackedRepo
	rec.Scan(&tr)
	return &tr
}

// Remove removes a repo
func (t *TrackListKeeper) Remove(name string) error {
	return t.db.Del(MakeTrackedRepoKey(name))
}
