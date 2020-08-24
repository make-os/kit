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
	"github.com/make-os/lobe/types/txns"
	"github.com/make-os/lobe/util"
	"github.com/pkg/errors"
	plumbing2 "gopkg.in/src-d/go-git.v4/plumbing"
)

var (
	MaxCompatRetries = 5
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

	// NextRunTime indicates a future time when this task can be processed.
	// If a worker receives this task before the specified time, it must put
	// it back into the queue.
	NextRunTime time.Time
}

func (t *Task) GetID() interface{} {
	return t.ID
}

// RefSync implements RefSyncer. It provides the mechanism that synchronizes the state of a
// repository's reference based on push transactions in the blockchain. It reacts to push
// transactions to change the state of a repository's reference state.
type RefSync struct {
	cfg                     *config.AppConfig
	log                     logger.Logger
	nWorkers                int
	fetcher                 fetcher.ObjectFetcherService
	queue                   *queue.UniqueQueue
	lck                     *sync.Mutex
	started                 bool
	stopped                 bool
	FinalizingRefs          map[string]struct{}
	makeReferenceUpdatePack push.ReferenceUpdateRequestPackMaker
	RepoGetter              repo.GetLocalRepoFunc
	UpdateRepoUsingNote     UpdateRepoUsingNoteFunc
}

// New creates an instance of RefSync
func New(cfg *config.AppConfig, fetcher fetcher.ObjectFetcherService, nWorkers int) *RefSync {
	rs := &RefSync{
		fetcher:                 fetcher,
		nWorkers:                nWorkers,
		lck:                     &sync.Mutex{},
		cfg:                     cfg,
		log:                     cfg.G().Log.Module("ref-syncer"),
		queue:                   queue.NewUnique(),
		FinalizingRefs:          make(map[string]struct{}),
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

// OnNewTx receives push transactions and adds non-delete
// pushed references to the queue as new tasks.
func (rs *RefSync) OnNewTx(tx *txns.TxPush) {
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
		time.Sleep(5 * time.Second)
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

	// When next run time is a future time, put task back in the queue
	if task.NextRunTime.After(time.Now()) {
		rs.queue.Append(task)
		return nil
	}

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
	localRefHash, err := targetRepo.RefGet(refName)
	if err != nil {
		if err != plumbing.ErrRefNotFound {
			return errors.Wrap(err, "failed to get reference from target repo")
		}
	}

	// If reference does not exist locally, reset ref hash is zero hash.
	if localRefHash == "" {
		localRefHash = plumbing2.ZeroHash.String()
	}

	// If the local reference hash does not match the task's old reference hash,
	// they are not compatible yet. It may be that there is another reference in
	// the queue capable of filling in the missing history. With that, we add
	// the task back to the queue to be re-processed later.
	// However, if we reach the compatibility retry limit, we attempt to alter
	// the task to point to the current local reference hash and re-queue in
	// hopes that the adjusted task completes the missing history.
	if localRefHash != task.Ref.OldHash {

		task.CompatRetryCount++
		if task.CompatRetryCount >= MaxCompatRetries {
			task.Ref.OldHash = localRefHash
			rs.queue.Append(task)
			return fmt.Errorf("reference is not compatible with local state")
		}

		// Put the task back in the queue and set future run time.
		task.NextRunTime = time.Now().Add(15 * time.Second)
		rs.queue.Append(task)
		return nil
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

	// Fetch objects required to apply push note successfully
	rs.fetcher.Fetch(note, func(err error) {
		if err != nil {
			rs.log.Error("Failed to fetch reference object", "Err", err.Error())
			return
		}
		if err = doUpdate(); err != nil {
			rs.log.Error("Failed to update repo using push note", "Err", err.Error())
			return
		}
	})

	return nil
}

// UpdateRepoUsingNoteFunc describes a function for updating a repo using a push note
type UpdateRepoUsingNoteFunc func(
	gitBinPath string,
	refUpdateMaker push.ReferenceUpdateRequestPackMaker,
	note types.PushNote) error

// UpdateRepoUsingNote updates a push note's target repo to match the state
// of the pushed references contained in it.
func UpdateRepoUsingNote(
	gitBinPath string,
	refUpdateMaker push.ReferenceUpdateRequestPackMaker,
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
