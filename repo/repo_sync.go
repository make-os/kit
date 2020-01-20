package repo

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util/logger"
	"github.com/pkg/errors"
)

// Syncher scans blocks and downloads objects referenced in push
// transactions into their respective repositories.
type Syncher struct {
	gmx             *sync.Mutex
	keepers         types.Keepers
	log             logger.Logger
	tick            *time.Ticker
	dht             types.DHT
	blockGetter     types.BlockGetter
	txPushMerger    types.TxPushMerger
	repoGetter      types.RepoGetter
	isValidatorMode bool
	isSyncing       bool
	lastHeight      uint64
}

// newSyncher creates an instance of Syncher
func newSyncher(
	blockGetter types.BlockGetter,
	repoGetter types.RepoGetter,
	txPushMerger types.TxPushMerger,
	keepers types.Keepers,
	dht types.DHT,
	isValidatorMode bool,
	log logger.Logger) *Syncher {
	return &Syncher{
		gmx:             &sync.Mutex{},
		log:             log,
		keepers:         keepers,
		txPushMerger:    txPushMerger,
		dht:             dht,
		blockGetter:     blockGetter,
		repoGetter:      repoGetter,
		isValidatorMode: isValidatorMode,
	}
}

// Start the repo syncher
func (s *Syncher) Start() {
	s.log.Info("Repo object synchronizer has started")
	s.tick = time.NewTicker(1 * time.Second)
	for range s.tick.C {
		lh, err := s.keepers.ManagedSysKeeper().GetLastRepoObjectsSyncHeight()
		if err != nil {
			panic(err)
		}

		s.lastHeight = lh

		err = s.start()
		if err != nil {
			s.log.Error("object sync error", "Err", err)
		}

		s.setSyncStatus(false)
	}
	return
}

func (s *Syncher) setSyncStatus(status bool) {
	s.gmx.Lock()
	s.isSyncing = status
	s.gmx.Unlock()
}

// IsSynced checks whether the syncher has processed all blocks up to the
// height block on the chain
func (s *Syncher) IsSynced() bool {
	lastSyncedHeight, err := s.keepers.ManagedSysKeeper().
		GetLastRepoObjectsSyncHeight()
	if err != nil {
		panic(err)
	}
	return int64(lastSyncedHeight) == s.blockGetter.GetChainHeight()
}

func (s *Syncher) start() error {
	defer s.setSyncStatus(false)

	startHeight := s.lastHeight

	for {

		// Get the next block
		block := s.blockGetter.GetBlock(int64(startHeight + 1))
		if block == nil {
			break
		}

		// Increment the height tracker and set sync status to active
		startHeight++
		s.setSyncStatus(true)

		// For each push transactions, decode it and attempt to fetch their
		// objects and update the target repo state
		processed := 0
		for _, txBz := range block.Txs {
			tx, err := types.DecodeTx(txBz)
			if err != nil {
				return fmt.Errorf("failed to decode tx")
			}

			if !tx.Is(types.TxTypePush) {
				continue
			}

			if err := s.syncTx(tx.(*types.TxPush)); err != nil {
				return err
			}

			processed++
		}

		if processed > 0 {
			s.log.Debug("Processed push transactions in block",
				"Height", startHeight, "NumTxPush", processed)
		}
	}

	if s.lastHeight < startHeight && startHeight > 1 {
		if err := s.keepers.ManagedSysKeeper().
			SetLastRepoObjectsSyncHeight(startHeight); err != nil {
			return err
		}
	}

	return nil
}

func (s *Syncher) syncTx(tx *types.TxPush) error {

	repoName := tx.PushNote.RepoName
	repo, err := s.repoGetter.GetRepo(repoName)
	if err != nil {
		return errors.Wrap(err, "unable to find repo locally")
	}

	// Do not download pushed objects in validator mode
	if s.isValidatorMode {
		goto update
	}

	// Download pushed objects
	for _, objHash := range tx.PushNote.GetPushedObjects(false) {
		if repo.ObjectExist(objHash) {
			continue
		}

		// Fetch from the dht
		dhtKey := MakeRepoObjectDHTKey(repoName, objHash)
		ctx, cn := context.WithTimeout(context.Background(), 60*time.Second)
		defer cn()
		objValue, err := s.dht.GetObject(ctx, &types.DHTObjectQuery{
			Module:    RepoObjectModule,
			ObjectKey: []byte(dhtKey),
		})
		if err != nil {
			msg := fmt.Sprintf("failed to fetch object '%s'", objHash)
			return errors.Wrap(err, msg)
		}

		// Write fetched object to the repo
		if err = repo.WriteObjectToFile(objHash, objValue); err != nil {
			msg := fmt.Sprintf("failed to write fetched object '%s' to disk",
				objHash)
			return errors.Wrap(err, msg)
		}

		// Annonce ourselves as the newest provider of the object
		if err := s.dht.Annonce(ctx, []byte(dhtKey)); err != nil {
			s.log.Warn("unable to announce git object", "Err", err)
			continue
		}

		s.log.Debug("Fetched object for repo", "ObjHash", objHash,
			"RepoName", repoName)
	}

update:
	// Attempt to merge the push transaction to the target repo
	if err = s.txPushMerger.UpdateRepoWithTxPush(tx); err != nil {
		return err
	}

	// For any pushed reference that has a delete directive, remove the
	// reference from the repo and also its tree.
	for _, ref := range tx.PushNote.GetPushedReferences() {
		if ref.Delete {
			if !s.isValidatorMode {
				if err = repo.RefDelete(ref.Name); err != nil {
					return errors.Wrapf(err, "failed to delete reference (%s)", ref.Name)
				}
			}
			if err := deleteReferenceTree(repo.Path(), ref.Name); err != nil {
				return errors.Wrapf(err, "failed to delete reference (%s) tree", ref.Name)
			}
		}
	}

	return nil
}

// Syncing checks whether the syncher is currently running
func (s *Syncher) Syncing() bool {
	s.gmx.Lock()
	status := s.isSyncing
	s.gmx.Unlock()
	return status
}

// Stop the syncher
func (s *Syncher) Stop() {
	if s.tick != nil {
		s.tick.Stop()
	}
}
