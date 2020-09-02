package keepers

import (
	"bytes"
	"fmt"

	"github.com/make-os/lobe/storage/common"
	storagetypes "github.com/make-os/lobe/storage/types"
	"github.com/make-os/lobe/types/core"
	"github.com/make-os/lobe/util"
)

// DHTKeeper manages DHT operation information.
type DHTKeeper struct {
	db storagetypes.Tx
}

// NewDHTKeyKeeper creates an instance of DHTKeeper
func NewDHTKeyKeeper(db storagetypes.Tx) *DHTKeeper {
	return &DHTKeeper{db: db}
}

// AddToAnnounceList adds a key that will be announced at a later time.
// key is the unique object key.
// objType is the object type.
// announceTime is the unix time when the key should be announced.
func (d *DHTKeeper) AddToAnnounceList(key []byte, repo string, objType int, announceTime int64) error {
	if err := d.RemoveFromAnnounceList(key); err != nil {
		return fmt.Errorf("failed to remove existing key")
	}
	data := &core.AnnounceListEntry{Type: objType, Repo: repo}
	rec := common.NewFromKeyValue(MakeAnnounceListKey(key, announceTime), util.ToBytes(data))
	if err := d.db.Put(rec); err != nil {
		return err
	}
	return nil
}

// RemoveFromAnnounceList removes a scheduled key announcement
func (d *DHTKeeper) RemoveFromAnnounceList(key []byte) error {
	var err error
	d.db.NewTx(true, true).Iterate(MakeQueryAnnounceListKey(), true, func(r *common.Record) bool {
		if !bytes.Equal(key, common.SplitPrefix(r.Prefix)[1]) {
			return false
		}
		err = d.db.NewTx(true, true).Del(r.GetKey())
		if err != nil {
			return true
		}
		return false
	})
	return err
}

// IterateAnnounceList iterates over all announcements entries, passing
// each of them to the provided callback function. Entries with the closest
// announcement time are returned first.
func (d *DHTKeeper) IterateAnnounceList(it func(key []byte, entry *core.AnnounceListEntry)) {
	d.db.NewTx(true, true).Iterate(MakeQueryAnnounceListKey(), true, func(r *common.Record) bool {
		var tr core.AnnounceListEntry
		r.Scan(&tr)
		tr.NextTime = int64(util.DecodeNumber(r.Key))
		var key = append([]byte{}, common.SplitPrefix(r.Prefix)[1]...)
		it(key, &tr)
		return false
	})
}
