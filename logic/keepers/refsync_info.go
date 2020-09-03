package keepers

import (
	"fmt"
	"strings"

	"github.com/make-os/lobe/pkgs/tree"
	"github.com/make-os/lobe/storage"
	"github.com/make-os/lobe/storage/common"
	storagetypes "github.com/make-os/lobe/storage/types"
	"github.com/make-os/lobe/types/core"
	"github.com/make-os/lobe/util"
	"github.com/make-os/lobe/util/crypto"
	"github.com/make-os/lobe/util/identifier"
	"github.com/pkg/errors"
)

// RepoSyncInfoKeeper manages information about repositories that are being tracked.
type RepoSyncInfoKeeper struct {
	db    storagetypes.Tx
	state *tree.SafeTree
}

// NewRepoSyncInfoKeeper creates an instance of RepoSyncInfoKeeper
func NewRepoSyncInfoKeeper(db storagetypes.Tx, state *tree.SafeTree) *RepoSyncInfoKeeper {
	return &RepoSyncInfoKeeper{db: db, state: state}
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
func (t *RepoSyncInfoKeeper) Track(targets string, height ...uint64) error {

	var final = []string{}
	for _, target := range strings.Split(targets, ",") {
		target = strings.TrimSpace(target)
		if identifier.IsUserURI(target) {
			nsName := identifier.GetNamespace(target)
			nsDomain := identifier.GetDomain(target)
			ns := NewNamespaceKeeper(t.state).Get(crypto.MakeNamespaceHash(nsName))
			if ns.IsNil() {
				return fmt.Errorf("namespace (%s) not found", nsName)
			}
			if nsDomain != "" {
				if _, ok := ns.Domains[nsDomain]; !ok {
					return fmt.Errorf("namespace domain (%s) not found", nsDomain)
				}
			}
			for domain, t := range ns.Domains {
				if nsDomain != "" && nsDomain != domain {
					continue
				}
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
		data := core.TrackedRepo{LastUpdated: util.UInt64(h)}
		rec := common.NewFromKeyValue(MakeTrackedRepoKey(repo), util.ToBytes(data))
		if err := t.db.Put(rec); err != nil {
			return errors.Wrap(err, "failed to add repo")
		}
	}

	return nil
}

// Tracked returns a map of repositories.
func (t *RepoSyncInfoKeeper) Tracked() (res map[string]*core.TrackedRepo) {
	res = make(map[string]*core.TrackedRepo)
	t.db.Discard()
	t.db.NewTx(true, true).Iterate(MakeQueryTrackedRepoKey(), false, func(r *common.Record) bool {
		var tr core.TrackedRepo
		r.Scan(&tr)
		res[string(common.SplitPrefix(r.GetKey())[1])] = &tr
		return false
	})
	return
}

// GetTracked returns a repo.
//
// Returns nil if not found
func (t *RepoSyncInfoKeeper) GetTracked(name string) *core.TrackedRepo {
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
func (t *RepoSyncInfoKeeper) UnTrack(targets string) error {
	var final = []string{}
	for _, target := range strings.Split(targets, ",") {
		target = strings.TrimSpace(target)
		if identifier.IsUserURI(target) {
			nsName := identifier.GetNamespace(target)
			nsDomain := identifier.GetDomain(target)
			ns := NewNamespaceKeeper(t.state).Get(crypto.MakeNamespaceHash(nsName))
			if ns.IsNil() {
				return fmt.Errorf("namespace (%s) not found", nsName)
			}
			if nsDomain != "" {
				if _, ok := ns.Domains[nsDomain]; !ok {
					return fmt.Errorf("namespace domain (%s) not found", nsDomain)
				}
			}
			for domain, t := range ns.Domains {
				if nsDomain != "" && nsDomain != domain {
					continue
				}
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
