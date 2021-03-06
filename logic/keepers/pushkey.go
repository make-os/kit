package keepers

import (
	"github.com/make-os/kit/pkgs/tree"
	"github.com/make-os/kit/storage/common"
	storagetypes "github.com/make-os/kit/storage/types"
	"github.com/make-os/kit/types/state"
	"github.com/pkg/errors"
)

// PushKeyKeeper manages push public keys.
type PushKeyKeeper struct {
	state *tree.SafeTree
	db    storagetypes.Tx
}

// NewPushKeyKeeper creates an instance of PushKeyKeeper
func NewPushKeyKeeper(state *tree.SafeTree, db storagetypes.Tx) *PushKeyKeeper {
	return &PushKeyKeeper{state: state, db: db}
}

// Get finds and returns a push key
//
// ARGS:
// pushKeyID: The unique ID of the public key
// blockNum: The target block to query (Optional. Default: latest)
//
// CONTRACT: It returns an empty Account if the key is not found.
func (g *PushKeyKeeper) Get(pushKeyID string, blockNum ...uint64) *state.PushKey {

	// Get version is provided
	var version uint64
	if len(blockNum) > 0 && blockNum[0] > 0 {
		version = blockNum[0]
	}

	// Query the push pub key. If version is provided,
	// we do a versioned query, otherwise we query the latest.
	key := MakePushKeyKey(pushKeyID)
	var bz []byte
	if version != 0 {
		_, bz = g.state.GetVersioned(key, int64(version))
	} else {
		_, bz = g.state.Get(key)
	}

	// If we don't find the pub key, we return an empty one.
	if bz == nil {
		return state.BarePushKey()
	}

	pushKey, err := state.NewPushKeyFromBytes(bz)
	if err != nil {
		panic(errors.Wrap(err, "failed to decode"))
	}

	return pushKey
}

// Update sets a new value for the given public key id.
// It also adds an address->pubID index search for public keys by address.
//
// ARGS:
// pushKeyID: The public key unique ID
// udp: The updated object to replace the existing object.
func (g *PushKeyKeeper) Update(pushKeyID string, upd *state.PushKey) error {
	g.state.Set(MakePushKeyKey(pushKeyID), upd.Bytes())
	key := MakeAddrPushKeyIDIndexKey(upd.Address.String(), pushKeyID)
	idx := common.NewFromKeyValue(key, []byte{})
	return g.db.Put(idx)
}

// Remove removes a push key by its id
//
// ARGS:
// pushKeyID: The public key unique ID
func (g *PushKeyKeeper) Remove(pushKeyID string) bool {
	key := MakePushKeyKey(pushKeyID)
	return g.state.Remove(key)
}

// GetByAddress returns all public keys associated with the given address
//
// ARGS:
// address: The target address
func (g *PushKeyKeeper) GetByAddress(address string) []string {
	var pushKeyIDs []string
	g.db.NewTx(true, true).Iterate(MakeQueryPushKeyIDsOfAddress(address), true, func(rec *common.Record) bool {
		parts := common.SplitPrefix(rec.Key)
		pushKeyIDs = append(pushKeyIDs, string(parts[len(parts)-1]))
		return false
	})
	return pushKeyIDs
}
