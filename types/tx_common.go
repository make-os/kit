package types

import (
	"bytes"
	"fmt"

	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/util"
	"github.com/vmihailenco/msgpack"
)

var (
	// TxTypeCoinTransfer represents a tx that moves coin between accounts
	TxTypeCoinTransfer = 0x0

	// TxTypeValidatorTicket represents a transaction purchases validator ticket
	TxTypeValidatorTicket = 0x01

	// TxTypeEpochSecret represents a transaction containing 64 bytes secret
	// for selecting the next epoch block validators.
	TxTypeEpochSecret = 0x02

	// TxTypeSetDelegatorCommission sets the delegator commission
	TxTypeSetDelegatorCommission = 0x03

	// TxTypeStorerTicket represents a transaction to acquire an storer ticket.
	TxTypeStorerTicket = 0x04

	// TxTypeUnbondStorerTicket represents a transaction to unbond storer stake
	TxTypeUnbondStorerTicket = 0x05

	// TxTypeRepoCreate represents a transaction to create a repository
	TxTypeRepoCreate = 0x06

	// TxTypeAddGPGPubKey represents a transaction to add a GPG public key to an
	// account
	TxTypeAddGPGPubKey = 0x07

	// TxTypePush represents a transaction that describes a git push operation
	TxTypePush = 0x08
)

// Transaction meta keys
var (
	TxMetaKeyInvalidated = "invalidated"
)

// TxMeta stores arbitrary, self-contained state information for a transaction
type TxMeta struct {
	meta map[string]interface{}
}

// IsInvalidated checks whether the transaction has been marked as invalid
func (m *TxMeta) IsInvalidated() bool {
	return m.meta != nil && m.meta[TxMetaKeyInvalidated] != nil
}

// Invalidate sets a TxMetaKeyInvalidated key in the tx map
// indicating that the transaction has been invalidated.
func (m *TxMeta) Invalidate() {
	m.meta[TxMetaKeyInvalidated] = struct{}{}
}

// GetMeta returns the meta information of the transaction
func (m *TxMeta) GetMeta() map[string]interface{} {
	return m.meta
}

// BaseTx describes a base transaction
type BaseTx interface {
	msgpack.CustomEncoder
	msgpack.CustomDecoder
	GetType() int                        // Returns the type of the transaction
	GetSignature() []byte                // Returns the transaction signature
	SetSignature(s []byte)               // Sets the transaction signature
	GetSenderPubKey() string             // Returns the transaction sender public key
	SetSenderPubKey(pk string)           // Set the transaction sender public key
	GetTimestamp() int64                 // Return the transaction creation unix timestamp
	SetTimestamp(t int64)                // Set the transaction creation unix timestamp
	GetNonce() uint64                    // Returns the transaction nonce
	SetNonce(nonce uint64)               // Set the transaction nonce
	GetFee() util.String                 // Returns the transaction fee
	GetFrom() util.String                // Returns the address of the transaction sender
	GetHash() util.Hash                  // Returns the hash of the transaction
	GetBytesNoSig() []byte               // Returns the serialized the tx excluding the signature
	Bytes() []byte                       // Returns the serialized transaction
	ComputeHash() util.Hash              // Computes the hash of the transaction
	GetID() string                       // Returns the id of the transaction (also the hash)
	Sign(privKey string) ([]byte, error) // Signs the transaction
	GetEcoSize() int64                   // Returns the size of the tx for use in proto economics
	GetSize() int64                      // Returns the size of the tx object (excluding nothing)
	ToMap() map[string]interface{}       // Returns a map equivalent of the transaction
	GetMeta() map[string]interface{}     // Returns the meta information of the transaction
	IsInvalidated() bool                 // Checks if the tx has been invalidated by a process
	Invalidate()                         // Invalidates the transaction
}

// TxType implements some of BaseTx, it includes type information about a transaction
type TxType struct {
	Type int `json:"type" msgpack:"type"`
}

// GetType returns the type of the transaction
func (tx *TxType) GetType() int {
	return tx.Type
}

// TxCommon implements some of BaseTx, it includes some common fields and methods
type TxCommon struct {
	*TxMeta      `json:"-" msgpack:"-"`
	Nonce        uint64      `json:"nonce" msgpack:"nonce"`
	Fee          util.String `json:"fee" msgpack:"fee"`
	Sig          []byte      `json:"sig" msgpack:"sig"`
	Timestamp    int64       `json:"timestamp" msgpack:"timestamp"`
	SenderPubKey string      `json:"senderPubKey" msgpack:"senderPubKey"`
}

// NewBareTxCommon returns an instance of TxCommon with zero values
func NewBareTxCommon() *TxCommon {
	return &TxCommon{
		TxMeta:       &TxMeta{meta: make(map[string]interface{})},
		Nonce:        0,
		Fee:          "0",
		Timestamp:    0,
		SenderPubKey: "",
	}
}

// GetFee returns the transaction nonce
func (tx *TxCommon) GetFee() util.String {
	return tx.Fee
}

// GetNonce returns the transaction nonce
func (tx *TxCommon) GetNonce() uint64 {
	return tx.Nonce
}

// SetNonce set the transaction nonce
func (tx *TxCommon) SetNonce(n uint64) {
	tx.Nonce = n
}

// GetSignature returns the transaction signature
func (tx *TxCommon) GetSignature() []byte {
	return tx.Sig
}

// SetSignature sets the transaction signature
func (tx *TxCommon) SetSignature(s []byte) {
	tx.Sig = s
}

// GetTimestamp return the transaction creation unix timestamp
func (tx *TxCommon) GetTimestamp() int64 {
	return tx.Timestamp
}

// SetTimestamp set the transaction creation unix timestamp
func (tx *TxCommon) SetTimestamp(t int64) {
	tx.Timestamp = t
}

// GetSenderPubKey returns the transaction sender public key
func (tx *TxCommon) GetSenderPubKey() string {
	return tx.SenderPubKey
}

// SetSenderPubKey set the transaction sender public key
func (tx *TxCommon) SetSenderPubKey(pk string) {
	tx.SenderPubKey = pk
}

// GetFrom returns the address of the transaction sender
// Panics if sender's public key is invalid
func (tx *TxCommon) GetFrom() util.String {
	pk, err := crypto.PubKeyFromBase58(tx.SenderPubKey)
	if err != nil {
		panic(err)
	}
	return pk.Addr()
}

// SignTransaction signs a transaction.
// Expects private key in base58Check encoding.
func SignTransaction(tx BaseTx, privKey string) ([]byte, error) {
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

// TxRecipient describes a transaction receiver
type TxRecipient struct {
	To util.String `json:"to" msgpack:"to"`
}

// TxValue describes a transaction value
type TxValue struct {
	Value util.String `json:"value" msgpack:"value"`
}

// DecodeTx takes a potential tx byte size and returns the transaction object
// for the given type
func DecodeTx(txBz []byte) (BaseTx, error) {
	dec := msgpack.NewDecoder(bytes.NewBuffer(txBz))
	txType, err := dec.DecodeInt()
	if err != nil {
		return nil, fmt.Errorf("failed to decode tx type")
	}

	var tx interface{}

	switch txType {
	case TxTypeCoinTransfer:
		tx = NewBareTxCoinTransfer()
	case TxTypeValidatorTicket:
		tx = NewBareTxTicketPurchase(TxTypeValidatorTicket)
	case TxTypeStorerTicket:
		tx = NewBareTxTicketPurchase(TxTypeStorerTicket)
	case TxTypeSetDelegatorCommission:
		tx = NewBareTxSetDelegateCommission()
	case TxTypeUnbondStorerTicket:
		tx = NewBareTxTicketUnbond(TxTypeUnbondStorerTicket)
	case TxTypeRepoCreate:
		tx = NewBareTxRepoCreate()
	case TxTypeAddGPGPubKey:
		tx = NewBareTxAddGPGPubKey()
	case TxTypeEpochSecret:
		tx = NewBareTxEpochSecret()
	default:
		return nil, fmt.Errorf("unsupported tx type")
	}

	return tx.(BaseTx), util.BytesToObject(txBz, tx)
}

// NewBaseTx creates a new, signed transaction of a given type
func NewBaseTx(txType int,
	nonce uint64,
	to util.String,
	senderKey *crypto.Key,
	value util.String,
	fee util.String,
	timestamp int64) (baseTx BaseTx) {

	switch txType {
	case TxTypeCoinTransfer:
		tx := NewBareTxCoinTransfer()
		tx.Nonce = nonce
		tx.To = to
		tx.SetSenderPubKey(senderKey.PubKey().Base58())
		tx.Value = value
		tx.Fee = fee
		tx.Timestamp = timestamp
		baseTx = tx
		goto sign
	case TxTypeValidatorTicket:
		tx := NewBareTxTicketPurchase(TxTypeValidatorTicket)
		tx.Nonce = nonce
		tx.SetSenderPubKey(senderKey.PubKey().Base58())
		tx.Value = value
		tx.Fee = fee
		tx.Timestamp = timestamp
		baseTx = tx
		goto sign
	case TxTypeStorerTicket:
		tx := NewBareTxTicketPurchase(TxTypeStorerTicket)
		tx.Nonce = nonce
		tx.SetSenderPubKey(senderKey.PubKey().Base58())
		tx.Value = value
		tx.Fee = fee
		tx.Timestamp = timestamp
		baseTx = tx
		goto sign
	case TxTypeSetDelegatorCommission:
		tx := NewBareTxSetDelegateCommission()
		tx.Nonce = nonce
		tx.SetSenderPubKey(senderKey.PubKey().Base58())
		tx.Fee = fee
		tx.Timestamp = timestamp
		baseTx = tx
		goto sign
	case TxTypeUnbondStorerTicket:
		tx := NewBareTxTicketUnbond(TxTypeUnbondStorerTicket)
		tx.Nonce = nonce
		tx.SetSenderPubKey(senderKey.PubKey().Base58())
		tx.Fee = fee
		tx.Timestamp = timestamp
		baseTx = tx
		goto sign
	case TxTypeRepoCreate:
		tx := NewBareTxRepoCreate()
		tx.Nonce = nonce
		tx.SetSenderPubKey(senderKey.PubKey().Base58())
		tx.Value = value
		tx.Fee = fee
		tx.Timestamp = timestamp
		baseTx = tx
		goto sign
	case TxTypeAddGPGPubKey:
		tx := NewBareTxAddGPGPubKey()
		tx.Nonce = nonce
		tx.SetSenderPubKey(senderKey.PubKey().Base58())
		tx.Fee = fee
		tx.Timestamp = timestamp
		baseTx = tx
		goto sign
	default:
		panic("unsupported tx type")
	}

sign:
	sig, err := baseTx.Sign(senderKey.PrivKey().Base58())
	if err != nil {
		panic(err)
	}
	baseTx.SetSignature(sig)
	return
}
