package keepers

import (
	"github.com/makeos/mosdef/storage"
	"github.com/makeos/mosdef/storage/tree"
	"github.com/makeos/mosdef/types"
	"github.com/pkg/errors"
)

// GPGPubKeyKeeper manages gpg public keys.
type GPGPubKeyKeeper struct {
	state *tree.SafeTree
	db    storage.Tx
}

// NewGPGPubKeyKeeper creates an instance of GPGPubKeyKeeper
func NewGPGPubKeyKeeper(state *tree.SafeTree, db storage.Tx) *GPGPubKeyKeeper {
	return &GPGPubKeyKeeper{state: state, db: db}
}

// GetGPGPubKey returns a GPG public key
//
// ARGS:
// pkID: The unique ID of the public key
// blockNum: The target block to query (Optional. Default: latest)
//
// CONTRACT: It returns an empty Account if no account is found.
func (g *GPGPubKeyKeeper) GetGPGPubKey(pkID string, blockNum ...uint64) *types.GPGPubKey {

	// Get version is provided
	var version uint64
	if len(blockNum) > 0 && blockNum[0] > 0 {
		version = blockNum[0]
	}

	// Query the gpg pub key. If version is provided,
	// we do a versioned query, otherwise we query the latest.
	key := MakeGPGPubKeyKey(pkID)
	var bz []byte
	if version != 0 {
		_, bz = g.state.GetVersioned(key, int64(version))
	} else {
		_, bz = g.state.Get(key)
	}

	// If we don't find the pub key, we return an empty one.
	if bz == nil {
		return types.BareGPGPubKey()
	}

	gpgPubKey, err := types.NewGPGPubKeyFromBytes(bz)
	if err != nil {
		panic(errors.Wrap(err, "failed to decode"))
	}

	return gpgPubKey
}

// Update sets a new value for the given public key id.
// It also adds an address->pubID index search for public keys by address.
//
// ARGS:
// pkID: The public key unique ID
// udp: The updated object to replace the existing object.
func (g *GPGPubKeyKeeper) Update(pkID string, upd *types.GPGPubKey) error {
	g.state.Set(MakeGPGPubKeyKey(pkID), upd.Bytes())
	key := MakeAddrGPGPkIDIndexKey(upd.Address.String(), pkID)
	idx := storage.NewFromKeyValue(key, []byte{})
	return g.db.Put(idx)
}

// GetPubKeyIDs returns all public keys associated with the given address
//
// ARGS:
// address: The target address
func (g *GPGPubKeyKeeper) GetPubKeyIDs(address string) []string {
	pkIDs := []string{}
	g.db.Iterate(MakeQueryPkIDs(address), true, func(rec *storage.Record) bool {
		parts := storage.SplitPrefix(rec.Key)
		pkIDs = append(pkIDs, string(parts[len(parts)-1]))
		return false
	})
	return pkIDs
}
