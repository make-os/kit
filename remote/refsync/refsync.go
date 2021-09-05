package refsync

import (
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"sync"

	plumbing2 "github.com/go-git/go-git/v5/plumbing"
	packfile2 "github.com/go-git/go-git/v5/plumbing/format/packfile"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/make-os/kit/config"
	dht2 "github.com/make-os/kit/net/dht"
	"github.com/make-os/kit/net/dht/announcer"
	"github.com/make-os/kit/pkgs/cache"
	"github.com/make-os/kit/pkgs/logger"
	"github.com/make-os/kit/remote/fetcher"
	"github.com/make-os/kit/remote/plumbing"
	"github.com/make-os/kit/remote/push"
	"github.com/make-os/kit/remote/push/types"
	reftypes "github.com/make-os/kit/remote/refsync/types"
	"github.com/make-os/kit/remote/repo"
	"github.com/make-os/kit/types/core"
	"github.com/make-os/kit/types/txns"
	"github.com/make-os/kit/util/crypto"
	"github.com/make-os/kit/util/identifier"
	"github.com/pkg/errors"
)

var (
	ErrUntracked = fmt.Errorf("untracked repository")
)

// RefSync implements RefSync. It provides the mechanism that synchronizes the state of a
// repository's reference based on push transactions in the blockchain. It reacts to newly
// processed push transactions, processing each references to change the state of a repository.
type RefSync struct {
	cfg                     *config.AppConfig
	log                     logger.Logger
	fetcher                 fetcher.ObjectFetcherService
	announcer               dht2.AnnouncerService
	makeReferenceUpdatePack push.MakeReferenceUpdateRequestPackFunc
	RepoGetter              repo.GetLocalRepoFunc
	UpdateRepoUsingNote     UpdateRepoUsingNoteFunc
	keepers                 core.Keepers
	watcher                 reftypes.Watcher
	queued                  *cache.Cache
	removeRefQueueOnEmpty   bool
	pool                    types.PushPool
	lck                     *sync.Mutex
	FinalizingRefs          map[string]struct{}
	queues                  map[string]chan *reftypes.RefTask
}

// New creates an instance of RefSync
func New(cfg *config.AppConfig,
	pool types.PushPool, fetcher fetcher.ObjectFetcherService,
	announcer dht2.AnnouncerService,
	keepers core.Keepers) *RefSync {

	rs := &RefSync{
		fetcher:                 fetcher,
		lck:                     &sync.Mutex{},
		cfg:                     cfg,
		queues:                  map[string]chan *reftypes.RefTask{},
		log:                     cfg.G().Log.Module("ref-syncer"),
		keepers:                 keepers,
		queued:                  cache.NewCache(10000),
		makeReferenceUpdatePack: push.MakeReferenceUpdateRequestPack,
		RepoGetter:              repo.GetWithGitModule,
		UpdateRepoUsingNote:     UpdateRepoUsingNote,
		pool:                    pool,
		removeRefQueueOnEmpty:   true,
		announcer:               announcer,
	}

	// Create and start the watcher
	rs.watcher = NewWatcher(cfg, rs.OnNewTx, keepers)
	if cfg.Node.Mode != config.ModeTest {
		rs.watcher.Start()
	}

	go func() {
		for evt := range cfg.G().Bus.On(core.EvtTxPushProcessed) {
			rs.OnNewTx(evt.Args[0].(*txns.TxPush), "", evt.Args[2].(int), evt.Args[1].(int64), nil)
		}
	}()

	return rs
}

// CanSync checks whether the target repository of a push transaction can be synchronized.
func (rs *RefSync) CanSync(namespace, repoName string) error {

	// If there are no explicitly tracked repository, all repositories must be synchronized
	var tracked = rs.keepers.RepoSyncInfoKeeper().Tracked()
	if len(tracked) == 0 {
		return nil
	}

	// If the push targets a namespace, resolve the namespace/domain
	if namespace != "" {
		ns := rs.keepers.NamespaceKeeper().Get(crypto.MakeNamespaceHash(namespace))
		if ns.IsNil() {
			return fmt.Errorf("namespace not found")
		}
		target, ok := ns.Domains[repoName]
		if !ok {
			return fmt.Errorf("namespace's domain not found")
		}
		repoName = identifier.GetDomain(target)
	}

	if _, ok := tracked[repoName]; !ok {
		return ErrUntracked
	}

	return nil
}

// Watch adds a repository to the watch queue
func (rs *RefSync) Watch(repo, reference string, startHeight, endHeight uint64) error {
	return rs.watcher.Watch(repo, reference, startHeight, endHeight)
}

type TxHandlerFunc func(*txns.TxPush, string, int, int64, func())

// OnNewTx receives push transactions and adds non-delete
// pushed references to the task queue.
// targetRef is the pushed reference that will be queued. If unset, all references are queued.
// txIndex is the index of the transaction it its containing block.
// height is the block height that contains the transaction.
func (rs *RefSync) OnNewTx(tx *txns.TxPush, targetRef string, txIndex int, height int64, doneCb func()) {

	// Ignore already queued transaction
	if rs.queued.Has(tx.GetNoteID()) {
		return
	}
	rs.queued.Add(tx.GetNoteID(), struct{}{})

	// Check if the repository is allowed to be synchronized.
	if rs.CanSync(tx.Note.GetNamespace(), tx.Note.GetRepoName()) != nil {
		return
	}

	// Track all non-delete pushed references
	for _, ref := range tx.Note.GetPushedReferences() {

		// If target reference is set, skip pushed reference if it does not match it
		if targetRef != "" && targetRef != ref.Name {
			continue
		}

		// Skip delete requesting pushed reference
		if plumbing.IsZeroHash(ref.NewHash) {
			continue
		}

		go rs.addTask(&reftypes.RefTask{
			ID:           tx.GetNoteID(),
			RepoName:     tx.Note.GetRepoName(),
			NoteCreator:  tx.Note.GetCreatorPubKey(),
			Endorsements: tx.Endorsements,
			Ref:          ref,
			Height:       height,
			TxIndex:      txIndex,
			Timestamp:    tx.GetTimestamp(),
			Done: func() {
				rs.queued.Remove(tx.GetNoteID())
				if doneCb != nil {
					doneCb()
				}
			},
		})
	}
}

func (rs *RefSync) addTask(task *reftypes.RefTask) {

	// Get reference queue or create a new one
	rs.lck.Lock()
	queue, ok := rs.queues[task.Ref.Name]
	rs.lck.Unlock()

	if !ok {
		queue = make(chan *reftypes.RefTask, 1000)
		rs.lck.Lock()
		rs.queues[task.Ref.Name] = queue
		rs.lck.Unlock()

		// For a new queue, start a goroutine process it
		go func() {
			for task := range queue {
				if err := rs.do(task); err != nil {
					rs.log.Error(err.Error(), "Repo", task.RepoName, "Ref", task.Ref.Name)
				}

				// If the queue is empty, close it and delete it from queues map
				if len(queue) == 0 && rs.removeRefQueueOnEmpty {
					close(queue)
					rs.lck.Lock()
					delete(rs.queues, task.Ref.Name)
					rs.lck.Unlock()
				}
			}
		}()
	}

	queue <- task
}

// Stop stops the syncer
func (rs *RefSync) Stop() {
	rs.watcher.Stop()
}

// updatedTrackInfo updates the track info height of a task's repo.
// Does nothing if the task's repo is not being tracked.
func (rs *RefSync) updatedTrackInfo(task *reftypes.RefTask) (err error) {
	if rs.keepers.RepoSyncInfoKeeper().GetTracked(task.RepoName) != nil {
		if err = rs.keepers.RepoSyncInfoKeeper().Track(task.RepoName, uint64(task.Height)); err != nil {
			err = errors.Wrap(err, "failed to update tracked repo info")
		}
	}
	return
}

// do takes a pushed reference task and attempts to fetch the objects
// required to update the reference's local state.
func (rs *RefSync) do(task *reftypes.RefTask) error {
	if task.Done != nil {
		defer task.Done()
	}

	// Get the target repo
	repoPath := filepath.Join(rs.cfg.GetRepoRoot(), task.RepoName)
	targetRepo, err := rs.RepoGetter(rs.cfg.Node.GitBinPath, repoPath)
	if err != nil {
		return errors.Wrap(err, "failed to get target repo")
	}

	// Get the local reference
	refName := task.Ref.Name
	localHash, err := targetRepo.RefGet(refName)
	if err != nil && err != plumbing.ErrRefNotFound {
		return errors.Wrap(err, "failed to get reference from target repo")
	}

	// We need to skip this task if the local hash is non-zero and:
	// - Local hash and the incoming new reference hash match.
	// - Local hash is a child of the new reference hash.
	if localHash != "" && (task.Ref.NewHash == localHash || targetRepo.IsAncestor(task.Ref.NewHash, localHash) == nil) {
		return rs.updatedTrackInfo(task)
	}

	// If reference does not exist locally, use zero hash as the local hash.
	if localHash == "" {
		localHash = plumbing2.ZeroHash.String()
	}

	// If the local hash does not match the task's old hash, it means history is missing.
	// This can be due to failure to apply previous push notes. To solve this, we update
	// the task's old hash to point to the local hash so that the missing history objects
	// are fetched and applied along with this task.
	if localHash != task.Ref.OldHash {
		task.Ref.OldHash = localHash
	}

	// Reconstruct a push note
	note := &types.Note{
		TargetRepo:    targetRepo,
		RepoName:      task.RepoName,
		References:    []*types.PushedReference{task.Ref},
		CreatorPubKey: task.NoteCreator,
		Timestamp:     task.Timestamp,
	}

	// doUpdate uses the push note to update the target repo
	doUpdate := func() error {
		err := rs.UpdateRepoUsingNote(rs.cfg.Node.GitBinPath, rs.makeReferenceUpdatePack, note)
		if err != nil {
			rs.log.Error("Failed to update reference using note", "Err", err.Error(), "ID", task.ID)
			return err
		}

		// If the repository is being tracked, update its last update height
		err = rs.updatedTrackInfo(task)

		// Update the reference's last sync height
		if err == nil {
			err = rs.keepers.RepoSyncInfoKeeper().UpdateRefLastSyncHeight(task.RepoName, task.Ref.Name, uint64(task.Height))
			if err != nil {
				err = errors.Wrap(err, "unable to update last reference sync height")
			}
		}

		rs.log.Debug("Successfully updated reference", "Repo", task.RepoName,
			"Ref", refName, "NewHash", task.Ref.NewHash, "OldHash", task.Ref.OldHash)

		return err
	}

	// If the note is in the push pool, it's safe to say the objects
	// already exist locally, therefore no need to fetch them again.
	if !rs.cfg.IsDev() && rs.pool.HasSeen(task.ID) {
		return doUpdate()
	}

	// Announce fetched objects as they are fetched.
	rs.fetcher.OnPackReceived(func(hash string, packfile io.ReadSeeker) {
		plumbing.UnpackPackfile(packfile, func(header *packfile2.ObjectHeader, read func() (object.Object, error)) error {
			obj, _ := read()
			if obj.Type() == plumbing2.CommitObject || obj.Type() == plumbing2.TagObject {
				objHash := obj.ID()
				rs.announcer.Announce(announcer.ObjTypeGit, note.RepoName, objHash[:], nil)
			}
			return nil
		})
	})

	// FetchAsync objects that are required to apply the push note updates successfully.
	// Since fetching is asynchronous, we use an error channel wait for it to complete.
	var errCh = make(chan error, 1)
	rs.fetcher.FetchAsync(note, func(err error) {

		if err != nil {
			rs.log.Error("Failed to fetch push note objects", "Err", err.Error())
			errCh <- err
			return
		}

		if err = doUpdate(); err != nil {
			rs.log.Error("Failed to update repo using push note", "Err", err.Error())
			errCh <- err
			return
		}

		errCh <- nil
	})

	return <-errCh
}

// UpdateRepoUsingNoteFunc describes a function for updating a repo using a push note
type UpdateRepoUsingNoteFunc func(
	gitBinPath string,
	refUpdateMaker push.MakeReferenceUpdateRequestPackFunc,
	note types.PushNote) error

// UpdateRepoUsingNote updates a push note's target repo to match the state
// of the pushed references contained in it.
func UpdateRepoUsingNote(
	gitBinPath string,
	refUpdateMaker push.MakeReferenceUpdateRequestPackFunc,
	note types.PushNote) error {

	// Create a packfile that represents updates described in the note.
	updatePackfile, err := refUpdateMaker(note)
	if err != nil {
		return errors.Wrap(err, "failed to create packfile from push note")
	}

	// Create the git-receive-pack command
	repoPath := note.GetTargetRepo().GetPath()
	cmd := exec.Command(gitBinPath, []string{"receive-pack", "--stateless-rpc", repoPath}...)
	cmd.Dir = repoPath

	// Get the command's stdin pipe
	in, err := cmd.StdinPipe()
	if err != nil {
		return errors.Wrap(err, "failed to get stdin pipe")
	}
	defer in.Close()

	// start the command
	err = cmd.Start()
	if err != nil {
		return errors.Wrap(err, "git-receive-pack failed to start")
	}

	io.Copy(in, updatePackfile)
	return cmd.Wait()
}
