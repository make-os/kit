package mempool

import (
	"fmt"
	"math"

	"gitlab.com/makeos/mosdef/types"
	"gitlab.com/makeos/mosdef/types/core"

	"gitlab.com/makeos/mosdef/config"

	"gitlab.com/makeos/mosdef/pkgs/cache"
	"gitlab.com/makeos/mosdef/pkgs/logger"

	"gitlab.com/makeos/mosdef/util"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/p2p"
)

const (
	MempoolChannel = byte(0x30)

	aminoOverheadForTxMessage = 8

	peerCatchupSleepIntervalMS = 100 // If peer is behind, sleep this amount

	// UnknownPeerID is the peer ID to use when running CheckTx when there is
	// no peer (e.g. RPC)
	UnknownPeerID uint16 = 0

	maxActiveIDs = math.MaxUint16
)

// Reactor handles mempool tx broadcasting amongst peers.
// It maintains a map from peer ID to counter, to prevent gossiping txs to the
// peers you received it from.
type Reactor struct {
	p2p.BaseReactor
	config  *cfg.MempoolConfig
	mempool *Mempool
	cache   *cache.Cache
	log     logger.Logger
}

// NewReactor returns a new Reactor with the given config and mempool.
func NewReactor(cfg *config.AppConfig, mempool *Mempool) *Reactor {
	r := &Reactor{
		config:  cfg.G().TMConfig.Mempool,
		mempool: mempool,
		cache:   cache.NewCache(cfg.Mempool.CacheSize),
		log:     cfg.G().Log.Module("mempool/reactor"),
	}
	r.BaseReactor = *p2p.NewBaseReactor("Reactor", r)

	return r
}

// OnStart implements p2p.BaseReactor.
func (r *Reactor) OnStart() error {
	return nil
}

// GetChannels implements Reactor.
// It returns the list of channels for this reactor.
func (r *Reactor) GetChannels() []*p2p.ChannelDescriptor {
	return []*p2p.ChannelDescriptor{
		{ID: MempoolChannel, Priority: 5},
	}
}

// AddPeer implements Reactor.
// It starts a broadcast routine ensuring all txs are forwarded to the given peer.
func (r *Reactor) AddPeer(peer p2p.Peer) {}

// RemovePeer implements Reactor.
func (r *Reactor) RemovePeer(peer p2p.Peer, reason interface{}) {}

// Receive implements Reactor.
// It adds any received transactions to the mempool.
func (r *Reactor) Receive(chID byte, src p2p.Peer, msgBytes []byte) {

	tx, err := core.DecodeTx(msgBytes)
	if err != nil {
		r.log.Error("Failed to decode received transaction", "Err", err)
		return
	}

	// Register the peer as a sender of the tx so we don't
	// broadcast the tx back to it
	r.addSender(tx.GetHash().HexStr(), string(src.ID()))

	// Register the peer to the pool
	_, err = r.AddTx(tx)
	if err != nil {
		return
	}

	r.log.Debug("Received and added a transaction",
		"TxHash", tx.GetHash().HexStr(), "PeerID", src.ID())
}

// GetPoolSize returns the size information of the pool
func (r *Reactor) GetPoolSize() *core.PoolSizeInfo {
	return &core.PoolSizeInfo{
		TotalTxSize: r.mempool.TxsBytes(),
		TxCount:     r.mempool.Size(),
	}
}

// GetTop returns the top n transactions in the pool.
// It will return all transactions if n is zero or negative.
func (r *Reactor) GetTop(n int) []types.BaseTx {
	var txs []types.BaseTx
	r.mempool.pool.Find(func(tx types.BaseTx) bool {
		txs = append(txs, tx)
		if n > 0 && len(txs) == n {
			return true
		}
		return false
	})
	return txs
}

// AddTx adds a transaction to the tx pool and broadcasts it.
func (r *Reactor) AddTx(tx types.BaseTx) (hash util.Bytes32, err error) {
	err = r.mempool.Add(tx)
	if err != nil {
		return util.Bytes32{}, err
	}

	r.broadcastTx(tx)

	return tx.GetHash(), nil
}

// broadcastTx sends a valid transaction to all known peers.
// It will not resend the transaction to peers that have previously
// sent the same transaction
func (r *Reactor) broadcastTx(tx types.BaseTx) {
	txHash := tx.GetHash().HexStr()
	txBytes := tx.Bytes()
	for _, peer := range r.Switch.Peers().List() {
		if r.isSender(txHash, string(peer.ID())) {
			continue
		}
		go peer.Send(MempoolChannel, txBytes)
	}
}

// addSender caches a peer as a sender of a tx
func (r *Reactor) addSender(txHash string, peerID string) {
	key := fmt.Sprintf("s:%s:%s", txHash, peerID)
	if !r.cache.Has(key) {
		r.cache.Add(key, struct{}{})
	}
}

// removeSender removes a peer sender key from the cache
func (r *Reactor) removeSender(txHash string, peerID string) {
	key := fmt.Sprintf("s:%s:%s", txHash, peerID)
	r.cache.Remove(key)
}

// isSender checks whether a peer has previously sent a tx with the given txHash
func (r *Reactor) isSender(txHash string, peerID string) bool {
	key := fmt.Sprintf("s:%s:%s", txHash, peerID)
	return r.cache.Has(key)
}
