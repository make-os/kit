package repo

import (
	"fmt"
	"gitlab.com/makeos/mosdef/types/core"
	"path/filepath"
	"sync"
	"time"

	"github.com/pkg/errors"
	"gitlab.com/makeos/mosdef/params"
)

// Pruner runs a scheduler that prunes repositories to remove unreachable or dangling
// objects. A repository will only be pruned if there are no pending
// transactions in both the transaction and push pools.
type Pruner struct {
	gmx        *sync.Mutex
	poolGetter core.PoolGetter
	reposDir   string
	targets    map[string]struct{}
	tick       *time.Ticker
}

// newPruner creates an instance of pruner
func newPruner(poolGetter core.PoolGetter, reposDir string) *Pruner {
	p := &Pruner{
		gmx:        &sync.Mutex{},
		reposDir:   reposDir,
		poolGetter: poolGetter,
		tick:       time.NewTicker(params.RepoPrunerTickDur),
		targets:    make(map[string]struct{}),
	}
	return p
}

// Schedule schedules a repository for pruning
func (p *Pruner) Schedule(repoName string) {
	p.gmx.Lock()
	p.targets[repoName] = struct{}{}
	p.gmx.Unlock()
}

// Prune prunes a repository only if it has no incoming transactions in both the transaction
// and push pool. If force is set to true, the repo will be pruned regardless of
// the existence of transactions in the pools.
// TODO: Requires smarter implementation that does not delete objects already
// referenced in a previous block.
func (p *Pruner) Prune(repoName string, force bool) error {
	// p.gmx.Lock()
	// defer p.gmx.Unlock()
	// return p.doPrune(repoName, force)
	return nil
}

// Prune prunes a repository only if it has no transactions in the transaction
// and push pool. If force is set to true, the repo will be pruned regardless of
// the existence of transactions in the pools.
// Note: Not thread safe
func (p *Pruner) doPrune(repoName string, force bool) error {

	// Abort if repo has a tx in the push pool
	if p.poolGetter.GetPushPool().RepoHasPushNote(repoName) && !force {
		return fmt.Errorf("refused because repo still has transactions in the push pool")
	}

	repo, err := GetRepo(filepath.Join(p.reposDir, repoName))
	if err != nil {
		return err
	}

	if err := repo.Prune(time.Time{}); err != nil {
		return errors.Wrap(err, "failed to prune")
	}

	delete(p.targets, repoName)
	return nil
}

// Start starts the pruner
func (p *Pruner) Start() {
	for range p.tick.C {
		p.gmx.Lock()
		for repoName := range p.targets {
			p.doPrune(repoName, false)
		}
		p.gmx.Unlock()
	}
}

// Stop stops the pruner
func (p *Pruner) Stop() {
	p.tick.Stop()
}
