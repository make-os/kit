package types

import (
	"fmt"

	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/util"
)

var (
	// TxTypeCoin represents a transaction that moves coin
	// from one source account to a destination account
	TxTypeCoin = 0x0
)

// Transaction represents a transaction
type Transaction struct {
	Type         int         `json:"type" msgpack:"type"`
	Nonce        uint64      `json:"nonce" msgpack:"nonce"`
	To           util.String `json:"to" msgpack:"to"`
	SenderPubKey util.String `json:"senderPubKey" msgpack:"senderPubKey"`
	Value        util.String `json:"value" msgpack:"value"`
	Timestamp    int64       `json:"timestamp" msgpack:"timestamp"`
	Fee          util.String `json:"fee" msgpack:"fee"`
	Sig          []byte      `json:"sig" msgpack:"sig"`
	Hash         util.Hash   `json:"hash" msgpack:"hash"`
}

// NewTx creates a new, signed transaction
func NewTx(txType int,
	nonce uint64,
	to util.String,
	senderKey *crypto.Key,
	value util.String,
	fee util.String,
	timestamp int64) (tx *Transaction) {

	tx = new(Transaction)
	tx.Type = txType
	tx.Nonce = nonce
	tx.To = to
	tx.SenderPubKey = util.String(senderKey.PubKey().Base58())
	tx.Value = value
	tx.Timestamp = timestamp
	tx.Fee = fee
	tx.Hash = tx.ComputeHash()

	sig, err := TxSign(tx, senderKey.PrivKey().Base58())
	if err != nil {
		panic(err)
	}
	tx.Sig = sig

	return
}

// GetSignature gets the signature
func (tx *Transaction) GetSignature() []byte {
	return tx.Sig
}

// SetSignature sets the signature
func (tx *Transaction) SetSignature(s []byte) {
	tx.Sig = s
}

// GetSenderPubKey gets the sender public key
func (tx *Transaction) GetSenderPubKey() util.String {
	return tx.SenderPubKey
}

// SetSenderPubKey sets the sender public key
func (tx *Transaction) SetSenderPubKey(pk util.String) {
	tx.SenderPubKey = pk
}

// GetTimestamp gets the timestamp
func (tx *Transaction) GetTimestamp() int64 {
	return tx.Timestamp
}

// SetTimestamp set the unix timestamp
func (tx *Transaction) SetTimestamp(t int64) {
	tx.Timestamp = t
}

// GetNonce gets the nonce
func (tx *Transaction) GetNonce() uint64 {
	return tx.Nonce
}

// GetFee gets the value
func (tx *Transaction) GetFee() util.String {
	return tx.Fee
}

// GetValue gets the value
func (tx *Transaction) GetValue() util.String {
	return tx.Value
}

// SetValue gets the value
func (tx *Transaction) SetValue(v util.String) {
	tx.Value = v
}

// GetTo gets the address of receiver
func (tx *Transaction) GetTo() util.String {
	return tx.To
}

// GetHash returns the hash of tx
func (tx *Transaction) GetHash() util.Hash {
	return tx.Hash
}

// SetHash sets the hash
func (tx *Transaction) SetHash(h util.Hash) {
	tx.Hash = h
}

// GetType gets the transaction type
func (tx *Transaction) GetType() int {
	return tx.Type
}

// GetBytesNoHashAndSig converts a transaction
// to bytes equivalent but omits the hash and
// signature in the result.
func (tx *Transaction) GetBytesNoHashAndSig() []byte {
	return util.ObjectToBytes([]interface{}{
		tx.Fee,
		tx.Nonce,
		tx.SenderPubKey,
		tx.Timestamp,
		tx.To,
		tx.Type,
		tx.Value,
	})
}

// Bytes converts a transaction to bytes equivalent
func (tx *Transaction) Bytes() []byte {
	return util.ObjectToBytes([]interface{}{
		tx.Fee,
		tx.Hash,
		tx.Nonce,
		tx.SenderPubKey,
		tx.Sig,
		tx.Timestamp,
		tx.To,
		tx.Type,
		tx.Value,
	})
}

// NewTxFromBytes creates a transaction object from a slice of
// bytes produced by tx.Bytes
func NewTxFromBytes(bs []byte) (*Transaction, error) {
	// var tx Transaction
	var fields []interface{}
	if err := util.BytesToObject(bs, &fields); err != nil {
		return nil, err
	}
	var tx Transaction
	tx.Fee = util.String(fields[0].(string))
	tx.Hash = util.BytesToHash(fields[1].([]uint8))
	tx.Nonce = fields[2].(uint64)
	tx.SenderPubKey = util.String(fields[3].(string))
	tx.Sig = fields[4].([]uint8)
	tx.Timestamp = fields[5].(int64)
	tx.To = util.String(fields[6].(string))
	tx.Type = int(fields[7].(int64))
	tx.Value = util.String(fields[8].(string))

	return &tx, nil
}

// GetSizeNoFee returns the virtual size of the transaction
// by summing up the byte size of every fields content except
// the `fee` field. The value does not represent the true size
// of the transaction on disk.
func (tx *Transaction) GetSizeNoFee() int64 {
	return int64(len(util.ObjectToBytes([]interface{}{
		tx.Hash,
		tx.Nonce,
		tx.SenderPubKey,
		tx.Sig,
		tx.Timestamp,
		tx.To,
		tx.Type,
		tx.Value,
	})))
}

// ComputeHash returns the Blake2-256 hash of the transaction.
func (tx *Transaction) ComputeHash() util.Hash {
	bs := tx.GetBytesNoHashAndSig()
	hash := util.Blake2b256(bs)
	return util.BytesToHash(hash[:])
}

// GetID returns the hex representation of the transaction
func (tx *Transaction) GetID() string {
	return tx.ComputeHash().HexStr()
}

// Sign the transaction
func (tx *Transaction) Sign(privKey string) ([]byte, error) {
	return TxSign(tx, privKey)
}

// TxVerify checks whether a transaction's signature is valid.
// Expect tx.SenderPubKey and tx.Sig to be set.
func TxVerify(tx *Transaction) error {

	if tx == nil {
		return fmt.Errorf("nil tx")
	}

	if tx.SenderPubKey == "" {
		return FieldError("senderPubKey", "sender public not set")
	}

	if len(tx.Sig) == 0 {
		return FieldError("sig", "signature not set")
	}

	pubKey, err := crypto.PubKeyFromBase58(string(tx.SenderPubKey))
	if err != nil {
		return FieldError("senderPubKey", err.Error())
	}

	valid, err := pubKey.Verify(tx.GetBytesNoHashAndSig(), tx.Sig)
	if err != nil {
		return FieldError("sig", err.Error())
	}

	if !valid {
		return ErrTxVerificationFailed
	}

	return nil
}

// TxSign signs a transaction.
// Expects private key in base58Check encoding.
func TxSign(tx *Transaction, privKey string) ([]byte, error) {

	if tx == nil {
		return nil, fmt.Errorf("nil tx")
	}

	pKey, err := crypto.PrivKeyFromBase58(privKey)
	if err != nil {
		return nil, err
	}

	sig, err := pKey.Sign(tx.GetBytesNoHashAndSig())
	if err != nil {
		return nil, err
	}

	return sig, nil
}
