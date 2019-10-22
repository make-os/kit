package jsmodules

import (
	"fmt"
	"time"

	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/util"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
)

func simpleTx(
	service types.Service,
	txType int,
	txObj interface{}, options ...interface{}) interface{} {

	var err error
	tx, key := processTxArgs(txObj, options...)
	tx.Type = txType

	// Set tx public key
	pk, _ := crypto.PrivKeyFromBase58(key)
	tx.SetSenderPubKey(util.String(crypto.NewKeyFromPrivKey(pk).PubKey().Base58()))

	// Set timestamp if not already set
	if tx.Timestamp == 0 {
		tx.Timestamp = time.Now().Unix()
	}

	// Set nonce if nonce is not provided
	if tx.Nonce == 0 {
		nonce, err := service.GetNonce(tx.GetFrom())
		if err != nil {
			panic(errors.Wrap(err, "failed to get sender's nonce"))
		}
		tx.Nonce = nonce + 1
	}

	// Sign the tx
	tx.Sig, err = tx.Sign(key)
	if err != nil {
		panic(errors.Wrap(err, "failed to sign transaction"))
	}

	// Process the transaction
	hash, err := service.SendTx(tx)
	if err != nil {
		panic(errors.Wrap(err, "failed to send transaction"))
	}

	return util.EncodeForJS(map[string]interface{}{
		"hash": hash,
	})
}

func processTxArgs(txObj interface{}, options ...interface{}) (*types.Transaction, string) {
	var err error

	// Decode parameters into a transaction object
	var tx = types.NewBareTx(types.TxTypeCoinTransfer)
	if err = mapstructure.Decode(txObj, tx); err != nil {
		panic(errors.Wrap(err, types.ErrArgDecode("types.Transaction", 0).Error()))
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

	return tx, key
}
