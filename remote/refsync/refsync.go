package refsync

import (
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/make-os/lobe/config"
	nodetypes "github.com/make-os/lobe/node/types"
	"github.com/make-os/lobe/pkgs/logger"
	"github.com/make-os/lobe/pkgs/queue"
	"github.com/make-os/lobe/remote/fetcher"
	"github.com/make-os/lobe/remote/plumbing"
	"github.com/make-os/lobe/remote/push"
	"github.com/make-os/lobe/remote/push/types"
	"github.com/make-os/lobe/remote/repo"
	"github.com/make-os/lobe/types/core"
	"github.com/make-os/lobe/types/txns"
	"github.com/make-os/lobe/util"
	"github.com/make-os/lobe/util/crypto"
	"github.com/make-os/lobe/util/identifier"
	"github.com/pkg/errors"
	"github.com/thoas/go-funk"
	plumbing2 "gopkg.in/src-d/go-git.v4/plumbing"
)

var (
	ErrUntracked = fmt.Errorf("untracked repository")
)

// RefSyncer describes an interface for synchronizing a repository's
// reference local state with the network using information from a
// push transaction.
type RefSyncer interface {
	// OnNewTx is called for every push transaction processed.
	OnNewTx(tx *txns.TxPush)

	// SetFetcher sets the object fetcher
	SetFetcher(fetcher fetcher.ObjectFetcherService)

	// Start starts the syncer.
	// Panics if reference syncer is already started.
	Start()

	// IsRunning checks if the syncer is running.
	IsRunning() bool

	// HasTask checks whether there are one or more unprocessed tasks.
	HasTask() bool

	// QueueSize returns the size of the tasks queue
	QueueSize() int

	// CanSync checks whether the target repository of a push transaction can be synchronized.
	CanSync(namespace, repoName string) error

	// Stops the syncer
	Stop()
}

// Task represents a task
type Task struct {
	// ID is the unique ID of the task
	ID string

	// RepoName is the target repository name
	RepoName string

	// Ref is the pushed reference
	Ref *types.PushedReference

	// Endorsements are the endorsements in the push transaction
	Endorsements txns.PushEndorsements

	// NoteCreator is the public key of the note creator
	NoteCreator util.Bytes32

	// CompatRetryCount is the number of times this task has been retried
	// because it was not yet compatible with the local reference.
	CompatRetryCount int
}

func (t *Task) GetID() interface{} {
	return t.ID
}

// RefSync implements RefSyncer. It provides the mechanism that synchronizes the state of a
// repository's reference based on push transactions in the blockchain. It reacts to newly
// processed push transactions, processing each references to change the state of a repository.
type RefSync struct {
	cfg                     *config.AppConfig
	log                     logger.Logger
	nWorkers                int
	fetcher                 fetcher.ObjectFetcherService
	queue                   *queue.UniqueQueue
	makeReferenceUpdatePack push.MakeReferenceUpdateRequestPackFunc
	RepoGetter              repo.GetLocalRepoFunc
	UpdateRepoUsingNote     UpdateRepoUsingNoteFunc
	keepers                 core.Keepers

	lck            *sync.Mutex
	started        bool
	stopped        bool
	FinalizingRefs map[string]struct{}
}

// New creates an instance of RefSync
func New(cfg *config.AppConfig, nWorkers int, fetcher fetcher.ObjectFetcherService, keepers core.Keepers) *RefSync {
	rs := &RefSync{
		fetcher:                 fetcher,
		nWorkers:                nWorkers,
		lck:                     &sync.Mutex{},
		cfg:                     cfg,
		log:                     cfg.G().Log.Module("ref-syncer"),
		queue:                   queue.NewUnique(),
		FinalizingRefs:          make(map[string]struct{}),
		keepers:                 keepers,
		makeReferenceUpdatePack: push.MakeReferenceUpdateRequestPack,
		RepoGetter:              repo.GetWithLiteGit,
		UpdateRepoUsingNote:     UpdateRepoUsingNote,
	}
	go func() {
		for evt := range cfg.G().Bus.On(nodetypes.EvtTxPushProcessed) {
			rs.OnNewTx(evt.Args[0].(*txns.TxPush))
		}
	}()
	return rs
}

// QueueSize returns the size of the tasks queue
func (rs *RefSync) QueueSize() int {
	return rs.queue.Size()
}

// CanSync checks whether the target repository of a push transaction can be synchronized.
func (rs *RefSync) CanSync(namespace, repoName string) error {

	// If there are no explicitly tracked repository, all repositories must be synchronized
	var tracked = rs.keepers.TrackedRepoKeeper().Tracked()
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

// OnNewTx receives push transactions and adds non-delete
// pushed references to the queue as new tasks.
func (rs *RefSync) OnNewTx(tx *txns.TxPush) {

	if rs.CanSync(tx.Note.GetNamespace(), tx.Note.GetRepoName()) != nil {
		return
	}

	for _, ref := range tx.Note.GetPushedReferences() {
		if plumbing.IsZeroHash(ref.NewHash) {
			continue
		}
		rs.queue.Append(&Task{
			ID:           fmt.Sprintf("%s_%d", ref.Name, ref.Nonce),
			RepoName:     tx.Note.GetRepoName(),
			NoteCreator:  tx.Note.GetCreatorPubKey(),
			Endorsements: tx.Endorsements,
			Ref:          ref,
		})
	}
}

// addToFinalizing adds a reference name to the index of currently finalizing references
func (rs *RefSync) addToFinalizing(refName string) {
	rs.lck.Lock()
	defer rs.lck.Unlock()
	rs.FinalizingRefs[refName] = struct{}{}
}

// isFinalizing checks whether the given reference name is being finalized
func (rs *RefSync) isFinalizing(refName string) bool {
	rs.lck.Lock()
	defer rs.lck.Unlock()
	_, ok := rs.FinalizingRefs[refName]
	return ok
}

// removeFromFinalizing removes the given reference name from the index of currently finalizing references
func (rs *RefSync) removeFromFinalizing(refName string) {
	rs.lck.Lock()
	defer rs.lck.Unlock()
	delete(rs.FinalizingRefs, refName)
}

// HasTask checks whether there are one or more unprocessed tasks.
func (rs *RefSync) HasTask() bool {
	return !rs.queue.Empty()
}

// Start starts the workers.
// Panics if already started.
func (rs *RefSync) Start() {

	rs.lck.Lock()
	started := rs.started
	rs.lck.Unlock()

	if started {
		panic("already started")
	}

	for i := 0; i < rs.nWorkers; i++ {
		go rs.createWorker(i)
	}

	rs.lck.Lock()
	rs.started = true
	rs.lck.Unlock()
}

// IsRunning checks if the syncer is running.
func (rs *RefSync) IsRunning() bool {
	return rs.started
}

// SetFetcher sets the object fetcher
func (rs *RefSync) SetFetcher(fetcher fetcher.ObjectFetcherService) {
	rs.fetcher = fetcher
}

// createWorker creates a worker that performs tasks in the queue
func (rs *RefSync) createWorker(id int) {
	for !rs.hasStopped() {
		task := rs.getTask()
		if task != nil {
			if err := Do(rs, task, id); err != nil {
				rs.log.Error(err.Error(), "Repo", task.RepoName, "Ref", task.Ref.Name)
			}
			continue
		}
		time.Sleep(time.Duration(funk.RandomInt(1, 5)) * time.Second)
	}
}

// hasStopped checks whether the syncer has been stopped
func (rs *RefSync) hasStopped() bool {
	rs.lck.Lock()
	defer rs.lck.Unlock()
	return rs.stopped
}

// getTask returns a task
func (rs *RefSync) getTask() *Task {
	item := rs.queue.Head()
	if item == nil {
		return nil
	}
	return item.(*Task)
}

// Stop stops the syncer
func (rs *RefSync) Stop() {
	rs.lck.Lock()
	rs.stopped = true
	rs.started = false
	rs.lck.Unlock()
}

// Do takes a pushed reference task and attempts to fetch the objects
// required to update the reference's local state.
func Do(rs *RefSync, task *Task, workerID int) error {

	refName := task.Ref.Name

	// If a matching reference is currently being processed by a different worker,
	// put the task back to the queue to be tried another time.
	if rs.isFinalizing(refName) {
		rs.queue.Append(task)
		return nil
	}

	// Add the reference to the finalizing reference index so other workers
	// know not to try to update the reference. Also, remove it from the index
	// when this function exits.
	rs.addToFinalizing(refName)
	defer rs.removeFromFinalizing(refName)

	// Get the target repo
	repoPath := filepath.Join(rs.cfg.GetRepoRoot(), task.RepoName)
	targetRepo, err := rs.RepoGetter(rs.cfg.Node.GitBinPath, repoPath)
	if err != nil {
		return errors.Wrap(err, "failed to get target repo")
	}

	// Get the target reference
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
		if err := rs.UpdateRepoUsingNote(rs.cfg.Node.GitBinPath, rs.makeReferenceUpdatePack, note); err != nil {
			rs.log.Error("Failed to update reference using note", "Err", err.Error(), "ID", task.ID)
			return err
		}
		rs.log.Debug("Successfully updated reference", "Repo", task.RepoName, "Ref", refName)
		return nil
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
		errCh <- err

		if err != nil {
			rs.log.Error("Failed to fetch push note objects", "Err", err.Error())
			return
		}

		if err = doUpdate(); err != nil {
			rs.log.Error("Failed to update repo using push note", "Err", err.Error())
			return
		}
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
