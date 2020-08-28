package keepers

import (
	"fmt"
	"strings"

	"github.com/make-os/lobe/pkgs/tree"
	"github.com/make-os/lobe/storage"
	"github.com/make-os/lobe/types/core"
	"github.com/make-os/lobe/util"
	"github.com/make-os/lobe/util/crypto"
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
		target = strings.TrimSpace(target)
		if identifier.IsUserURI(target) {
			nsName := identifier.GetNamespace(target)
			ns := NewNamespaceKeeper(t.state).Get(crypto.MakeNamespaceHash(nsName))
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
		data := core.TrackedRepo{LastHeight: util.UInt64(h)}
		rec := storage.NewFromKeyValue(MakeTrackedRepoKey(repo), util.ToBytes(data))
		if err := t.db.Put(rec); err != nil {
			return errors.Wrap(err, "failed to add repo")
		}
	}

	return nil
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

// Remove removes repositories from the track list.
//
// Target can be one or more comma-separated list of repositories or user namespaces.
//
// If a user namespace is provided, all repository targets are removed.
func (t *TrackListKeeper) Remove(targets string) error {
	var final = []string{}
	for _, target := range strings.Split(targets, ",") {
		target = strings.TrimSpace(target)
		if identifier.IsUserURI(target) {
			nsName := identifier.GetNamespace(target)
			ns := NewNamespaceKeeper(t.state).Get(crypto.MakeNamespaceHash(nsName))
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

	for _, name := range final {
		if err := t.db.Del(MakeTrackedRepoKey(name)); err != nil {
			return err
		}
	}

	return nil
}
