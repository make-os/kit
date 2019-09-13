package jsmodule

import (
	"fmt"
	"time"

	"github.com/makeos/mosdef/util"

	"github.com/makeos/mosdef/crypto"

	"github.com/makeos/mosdef/node/services"
	"github.com/makeos/mosdef/types"
	"github.com/ncodes/mapstructure"
	"github.com/pkg/errors"
)

// sendCoin sends the native coin from a source account
// to a destination account. It returns an object containing
// the hash of the transaction. It panics when an error occurs.
func (m *Module) sendCoin(txObj interface{}, options ...interface{}) interface{} {

	var err error

	// Decode parameters into a transaction object
	var tx types.Transaction
	if err = mapstructure.Decode(txObj, &tx); err != nil {
		panic(types.ErrArgDecode("object", 0))
	}

	// - Expect options[0] to be the private key (base58 encoded)
	// - options[0] must be a string
	// - options[0] must be a valid key
	var key string
	var ok bool
	if len(options) > 0 {
		key, ok = options[0].(string)
		if !ok {
			panic(types.ErrArgDecode("string", 1))
		} else if err := crypto.IsValidPrivKey(key); err != nil {
			panic(errors.Wrap(err, types.ErrInvalidPrivKey.Error()))
		}
	} else {
		panic(fmt.Errorf("key is required"))
	}

	// Set tx public key
	pk, _ := crypto.PrivKeyFromBase58(key)
	tx.SetSenderPubKey(util.String(crypto.NewKeyFromPrivKey(pk).PubKey().Base58()))

	// Set timestamp if not already set
	if tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}

	// Compute the hash
	tx.SetHash(tx.ComputeHash())

	// Sign the tx
	tx.Sig, err = tx.Sign(key)
	if err != nil {
		panic(errors.Wrap(err, "failed to sign transaction"))
	}

	// Process the transaction
	res, err := m.nodeService.Do(services.SrvNameCoinSend, &tx)
	if err != nil {
		panic(errors.Wrap(err, "failed to send transaction"))
	}

	return res
}
