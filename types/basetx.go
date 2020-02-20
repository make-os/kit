package types

import (
	"github.com/vmihailenco/msgpack"
	"gitlab.com/makeos/mosdef/util"
)

// BaseTx describes a base transaction
type BaseTx interface {
	msgpack.CustomEncoder
	msgpack.CustomDecoder
	GetType() int                         // Returns the type of the transaction
	GetSignature() []byte                 // Returns the transaction signature
	SetSignature(s []byte)                // Sets the transaction signature
	GetSenderPubKey() util.PublicKey      // Returns the transaction sender public key
	SetSenderPubKey(pk []byte)            // Set the transaction sender public key
	GetTimestamp() int64                  // Return the transaction creation unix timestamp
	SetTimestamp(t int64)                 // Set the transaction creation unix timestamp
	GetNonce() uint64                     // Returns the transaction nonce
	SetNonce(nonce uint64)                // Set the transaction nonce
	SetFee(fee util.String)               // Set the fee
	GetFee() util.String                  // Returns the transaction fee
	GetFrom() util.String                 // Returns the address of the transaction sender
	GetHash() util.Bytes32                // Returns the hash of the transaction
	GetBytesNoSig() []byte                // Returns the serialized the tx excluding the signature
	Bytes() []byte                        // Returns the serialized transaction
	ComputeHash() util.Bytes32            // Computes the hash of the transaction
	GetID() string                        // Returns the id of the transaction (also the hash)
	Sign(privKey string) ([]byte, error)  // Signs the transaction
	GetEcoSize() int64                    // Returns the size of the tx for use in proto economics
	GetSize() int64                       // Returns the size of the tx object (excluding nothing)
	ToMap() map[string]interface{}        // Returns a map equivalent of the transaction
	FromMap(map[string]interface{}) error // Populate the fields from a map
	GetMeta() map[string]interface{}      // Returns the meta information of the transaction
	Is(txType int) bool                   // Checks if the tx is a given type
}

