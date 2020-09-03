package refsync

import (
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/make-os/lobe/config"
	nodetypes "github.com/make-os/lobe/node/types"
	"github.com/make-os/lobe/pkgs/cache"
	"github.com/make-os/lobe/pkgs/logger"
	"github.com/make-os/lobe/remote/fetcher"
	"github.com/make-os/lobe/remote/plumbing"
	"github.com/make-os/lobe/remote/push"
	"github.com/make-os/lobe/remote/push/types"
	reftypes "github.com/make-os/lobe/remote/refsync/types"
	"github.com/make-os/lobe/remote/repo"
	"github.com/make-os/lobe/types/core"
	"github.com/make-os/lobe/types/txns"
	"github.com/make-os/lobe/util/crypto"
	"github.com/make-os/lobe/util/identifier"
	"github.com/pkg/errors"
	plumbing2 "gopkg.in/src-d/go-git.v4/plumbing"
)

var (
	ErrUntracked = fmt.Errorf("untracked repository")
)

// RefSync implements RefSync. It provides the mechanism that synchronizes the state of a
// repository's reference based on push transactions in the blockchain. It reacts to newly
// processed push transactions, processing each references to change the state of a repository.
type RefSync struct {
	cfg     *config.AppConfig
	log     logger.Logger
	fetcher fetcher.ObjectFetcherService

	makeReferenceUpdatePack push.MakeReferenceUpdateRequestPackFunc
	RepoGetter              repo.GetLocalRepoFunc
	UpdateRepoUsingNote     UpdateRepoUsingNoteFunc
	keepers                 core.Keepers
	watcher                 reftypes.Watcher
	queued                  *cache.Cache
	removeRefQueueOnEmpty   bool

	lck            *sync.Mutex
	FinalizingRefs map[string]struct{}
	queues         map[string]chan *reftypes.RefTask
}

// New creates an instance of RefSync
func New(cfg *config.AppConfig, fetcher fetcher.ObjectFetcherService, keepers core.Keepers) *RefSync {
	rs := &RefSync{
		fetcher:                 fetcher,
		lck:                     &sync.Mutex{},
		cfg:                     cfg,
		queues:                  map[string]chan *reftypes.RefTask{},
		log:                     cfg.G().Log.Module("ref-syncer"),
		keepers:                 keepers,
		queued:                  cache.NewCache(1000),
		makeReferenceUpdatePack: push.MakeReferenceUpdateRequestPack,
		RepoGetter:              repo.GetWithLiteGit,
		UpdateRepoUsingNote:     UpdateRepoUsingNote,
		removeRefQueueOnEmpty:   true,
	}

	rs.watcher = NewWatcher(cfg, rs.OnNewTx, keepers)

	go func() {
		for evt := range cfg.G().Bus.On(nodetypes.EvtTxPushProcessed) {
			rs.OnNewTx(evt.Args[0].(*txns.TxPush), evt.Args[2].(int), evt.Args[1].(int64))
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

type TxHandlerFunc func(*txns.TxPush, int, int64)

// OnNewTx receives push transactions and adds non-delete
// pushed references to the task queue.
// txIndex is the index of the transaction it its containing block.
// height is the block height that contains the transaction.
func (rs *RefSync) OnNewTx(tx *txns.TxPush, txIndex int, height int64) {

	// Ignore already queued transaction
	if rs.queued.Has(tx.GetID()) {
		return
	}
	rs.queued.Add(tx.GetID(), struct{}{})

	// Check if the repository is allowed to be synchronized.
	if rs.CanSync(tx.Note.GetNamespace(), tx.Note.GetRepoName()) != nil {
		return
	}

	// Track all non-delete pushed references
	for _, ref := range tx.Note.GetPushedReferences() {
		if plumbing.IsZeroHash(ref.NewHash) {
			continue
		}
		rs.addTask(&reftypes.RefTask{
			ID:           fmt.Sprintf("%s_%d", ref.Name, ref.Nonce),
			RepoName:     tx.Note.GetRepoName(),
			NoteCreator:  tx.Note.GetCreatorPubKey(),
			Endorsements: tx.Endorsements,
			Ref:          ref,
			Height:       height,
			TxIndex:      txIndex,
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

// do takes a pushed reference task and attempts to fetch the objects
// required to update the reference's local state.
func (rs *RefSync) do(task *reftypes.RefTask) error {

	// Get the target repo
	repoPath := filepath.Join(rs.cfg.GetRepoRoot(), task.RepoName)
	targetRepo, err := rs.RepoGetter(rs.cfg.Node.GitBinPath, repoPath)
	if err != nil {
		return errors.Wrap(err, "failed to get target repo")
	}

	// Get the target reference
	refName := task.Ref.Name
	localHash, err := targetRepo.RefGet(refName)
	if err != nil {
		if err != plumbing.ErrRefNotFound {
			return errors.Wrap(err, "failed to get reference from target repo")
		}
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
	}

	// doUpdate uses the push note to update the target repo
	doUpdate := func() error {
		err := rs.UpdateRepoUsingNote(rs.cfg.Node.GitBinPath, rs.makeReferenceUpdatePack, note)
		if err != nil {
			rs.log.Error("Failed to update reference using note", "Err", err.Error(), "ID", task.ID)
			return err
		}

		// If the repository is being tracked, update its last update height
		if trackInfo := rs.keepers.RepoSyncInfoKeeper().GetTracked(task.RepoName); trackInfo != nil {
			if err = rs.keepers.RepoSyncInfoKeeper().Track(task.RepoName, uint64(task.Height)); err != nil {
				err = errors.Wrap(err, "failed to update tracked repo info")
			}
		}

		rs.log.Debug("Successfully updated reference", "Repo", task.RepoName,
			"Ref", refName, "NewHash", task.Ref.NewHash, "OldHash", task.Ref.OldHash)

		return err
	}

	// If the push note was signed by the current node, we don't need to fetch
	// anything as we expect this node to have the objects already. Instead,
	// update the repo using the push note.
	valKey, _ := rs.cfg.G().PrivVal.GetKey()
	if note.CreatorPubKey.Equal(valKey.PubKey().MustBytes32()) {
		return doUpdate()
	}

	// If the push note was endorsed by the current node, we don't need to fetch
	// anything as we expect this node to have the objects already. Instead,
	// update the repo using the push note.
	for _, end := range task.Endorsements {
		if end.EndorserPubKey.Equal(valKey.PubKey().MustBytes32()) {
			return doUpdate()
		}
	}

	// FetchAsync objects required to apply the push note updates successfully.
	// Since fetching is asynchronous, we use an error channel to learn about
	// problems and also to wait for the fetch operation to finish.
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
		return errors.Wrap(err, "failed to start git-receive-pack command")
	}

	io.Copy(in, updatePackfile)
	return cmd.Wait()
}
