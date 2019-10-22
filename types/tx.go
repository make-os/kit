package types

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/fatih/structs"

	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/util"
)

var (
	// TxTypeCoinTransfer represents a tx that moves coin between accounts
	TxTypeCoinTransfer = 0x0

	// TxTypeValidatorTicket represents a transaction purchases validator ticket
	TxTypeValidatorTicket = 0x1

	// TxTypeEpochSecret represents a transaction containing 64 bytes secret
	// for selecting the next epoch block validators.
	TxTypeEpochSecret = 0x2

	// TxTypeSetDelegatorCommission sets the delegator commission
	TxTypeSetDelegatorCommission = 0x3

	// TxTypeStorerTicket represents a transaction to acquire an storer ticket.
	TxTypeStorerTicket = 0x4
)

// Transaction meta keys
var (
	TxMetaKeyInvalidated = "invalidated"
)

// Tx represents a transaction
type Tx interface {
	GetSignature() []byte
	SetSignature(s []byte)
	GetSenderPubKey() util.String
	SetSenderPubKey(pk util.String)
	GetTimestamp() int64
	SetTimestamp(t int64)
	GetNonce() uint64
	GetFee() util.String
	GetValue() util.String
	SetValue(v util.String)
	GetFrom() util.String
	GetTo() util.String
	GetHash() util.Hash
	GetType() int
	GetSecret() []byte
	GetPreviousSecret() []byte
	GetSecretRound() uint64
	GetBytesNoSig() []byte
	GetTicketID() []byte
	Bytes() []byte
	ComputeHash() util.Hash
	GetID() string
	Sign(privKey string) ([]byte, error)
	GetSizeNoFee() int64
	GetSize() int64
	ToMap() map[string]interface{}
	ToHex() string
	GetMeta() map[string]interface{}
	IsInvalidated() bool
	Invalidate()
}

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

	// TxTypeEpochSecret specific field
	Secret         []byte `json:"secret,omitempty" msgpack:"secret,omitempty"`
	PreviousSecret []byte `json:"previousSecret,omitempty" msgpack:"previousSecret,omitempty"`
	SecretRound    uint64 `json:"secretRound,omitempty" msgpack:"secretRound,omitempty"`

	// TxTypeUnbondTicket specific field
	TicketID []byte `json:"ticketID,omitempty" msgpack:"ticketID,omitempty"`

	// meta stores arbitrary data for message passing during tx processing
	meta map[string]interface{}
}

// NewBareTx create an unsigned transaction with zero value for all fields.
func NewBareTx(txType int) *Transaction {
	tx := new(Transaction)
	tx.Type = txType
	tx.Nonce = 0
	tx.To = util.String("")
	tx.SenderPubKey = util.String("")
	tx.Value = util.String("0")
	tx.Fee = util.String("0")
	tx.Timestamp = time.Now().Unix()
	tx.TicketID = []byte{}
	tx.Secret = []byte{}
	tx.PreviousSecret = []byte{}
	tx.SecretRound = 0
	tx.Sig = []byte{}
	tx.meta = map[string]interface{}{}
	return tx
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
	tx.Secret = []byte{}
	tx.PreviousSecret = []byte{}
	tx.SecretRound = 0
	tx.TicketID = []byte{}
	tx.meta = map[string]interface{}{}

	var err error
	tx.Sig, err = SignTx(tx, senderKey.PrivKey().Base58())
	if err != nil {
		panic(err)
	}

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

// GetMeta returns the app meta
func (tx *Transaction) GetMeta() map[string]interface{} {
	return tx.meta
}

// SetMeta set the full meta
func (tx *Transaction) SetMeta(meta map[string]interface{}) {
	tx.meta = meta
}

// IsInvalidated checks whether the transaction has been marked as invalid
func (tx *Transaction) IsInvalidated() bool {
	if tx.meta == nil {
		return false
	}
	return tx.meta[TxMetaKeyInvalidated] != nil
}

// Invalidate sets a TxMetaKeyInvalidated key in the tx map
// indicating that the transaction has been invalidated.
func (tx *Transaction) Invalidate() {
	tx.meta[TxMetaKeyInvalidated] = struct{}{}
}

// GetSenderPubKey gets the sender public key
func (tx *Transaction) GetSenderPubKey() util.String {
	return tx.SenderPubKey
}

// SetSenderPubKey sets the sender public key
func (tx *Transaction) SetSenderPubKey(pk util.String) {
	tx.SenderPubKey = pk
}

// GetFrom returns the address of the sender.
// Panics if sender's public key is invalid
func (tx *Transaction) GetFrom() util.String {
	pk, err := crypto.PubKeyFromBase58(tx.SenderPubKey.String())
	if err != nil {
		panic(err)
	}
	return pk.Addr()
}

// GetTimestamp gets the timestamp
func (tx *Transaction) GetTimestamp() int64 {
	return tx.Timestamp
}

// SetTimestamp set the unix timestamp
func (tx *Transaction) SetTimestamp(t int64) {
	tx.Timestamp = t
}

// GetSecret returns the secret
// FOR: TxTypeEpochSecret
func (tx *Transaction) GetSecret() []byte {
	return tx.Secret
}

// GetPreviousSecret returns the previous secret
// FOR: TxTypeEpochSecret
func (tx *Transaction) GetPreviousSecret() []byte {
	return tx.PreviousSecret
}

// GetSecretRound returns the secret round
// FOR: TxTypeEpochSecret
func (tx *Transaction) GetSecretRound() uint64 {
	return tx.SecretRound
}

// GetTicketID returns the ticket id
// FOR: TxTypeUnbondTicket
func (tx *Transaction) GetTicketID() []byte {
	return tx.TicketID
}

// ToMap decodes the transaction to a map
func (tx *Transaction) ToMap() map[string]interface{} {
	s := structs.New(tx)
	s.TagName = "json"
	return s.Map()
}

// ToHex returns the hex encoded representation of the tx
func (tx *Transaction) ToHex() string {
	return hex.EncodeToString(tx.Bytes())
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
	return tx.ComputeHash()
}

// GetType gets the transaction type
func (tx *Transaction) GetType() int {
	return tx.Type
}

// GetBytesNoSig returns a serialized transaction
// but omits the signature in the result.
func (tx *Transaction) GetBytesNoSig() []byte {
	return util.ObjectToBytes([]interface{}{
		tx.Fee,
		tx.Nonce,
		tx.SenderPubKey,
		tx.Timestamp,
		tx.To,
		tx.Type,
		tx.Value,
		tx.Secret,
		tx.PreviousSecret,
		tx.SecretRound,
		tx.TicketID,
	})
}

// Bytes returns the serializes version of the transaction
func (tx *Transaction) Bytes() []byte {
	return util.ObjectToBytes([]interface{}{
		tx.Fee,
		tx.Nonce,
		tx.SenderPubKey,
		tx.Sig,
		tx.Timestamp,
		tx.To,
		tx.Type,
		tx.Value,
		tx.Secret,
		tx.PreviousSecret,
		tx.SecretRound,
		tx.TicketID,
	})
}

// NewTxFromBytes creates a transaction object from a slice of
// bytes produced by tx.Bytes
func NewTxFromBytes(bs []byte) (*Transaction, error) {
	var fields []interface{}
	if err := util.BytesToObject(bs, &fields); err != nil {
		return nil, err
	}
	var tx Transaction
	tx.meta = make(map[string]interface{})
	tx.Fee = util.String(fields[0].(string))
	tx.Nonce = fields[1].(uint64)
	tx.SenderPubKey = util.String(fields[2].(string))
	tx.Sig = fields[3].([]uint8)
	tx.Timestamp = fields[4].(int64)
	tx.To = util.String(fields[5].(string))
	tx.Type = int(fields[6].(int64))
	tx.Value = util.String(fields[7].(string))
	tx.Secret = fields[8].([]byte)
	tx.PreviousSecret = fields[9].([]byte)
	tx.SecretRound = fields[10].(uint64)
	tx.TicketID = fields[11].([]byte)
	tx.meta = map[string]interface{}{}

	return &tx, nil
}

// GetSizeNoFee returns the virtual size of the transaction
// by summing up the byte size of every fields content except
// the `fee` field. The value does not represent the true size
// of the transaction on disk.
func (tx *Transaction) GetSizeNoFee() int64 {
	return int64(len(util.ObjectToBytes([]interface{}{
		tx.Nonce,
		tx.SenderPubKey,
		tx.Sig,
		tx.Timestamp,
		tx.To,
		tx.Type,
		tx.Value,
		tx.Secret,
		tx.PreviousSecret,
		tx.SecretRound,
		tx.TicketID,
	})))
}

// GetSize returns the size of the transaction
func (tx *Transaction) GetSize() int64 {
	return int64(len(tx.Bytes()))
}

// ComputeHash returns the Blake2-256 hash of the serialized transaction.
func (tx *Transaction) ComputeHash() util.Hash {
	bs := tx.Bytes()
	hash := util.Blake2b256(bs)
	return util.BytesToHash(hash[:])
}

// GetID returns the hex representation of the transaction
func (tx *Transaction) GetID() string {
	return tx.ComputeHash().HexStr()
}

// Sign the transaction
func (tx *Transaction) Sign(privKey string) ([]byte, error) {
	return SignTx(tx, privKey)
}

// VerifyTx checks whether a transaction's signature is valid.
// Expect tx.SenderPubKey and tx.Sig to be set.
func VerifyTx(tx *Transaction) error {

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

	valid, err := pubKey.Verify(tx.GetBytesNoSig(), tx.Sig)
	if err != nil {
		return FieldError("sig", err.Error())
	}

	if !valid {
		return ErrTxVerificationFailed
	}

	return nil
}

// SignTx signs a transaction.
// Expects private key in base58Check encoding.
func SignTx(tx *Transaction, privKey string) ([]byte, error) {

	if tx == nil {
		return nil, fmt.Errorf("nil tx")
	}

	pKey, err := crypto.PrivKeyFromBase58(privKey)
	if err != nil {
		return nil, err
	}

	sig, err := pKey.Sign(tx.GetBytesNoSig())
	if err != nil {
		return nil, err
	}

	return sig, nil
}
