package types

import (
	"github.com/make-os/lobe/crypto"
	"github.com/make-os/lobe/util"
	"github.com/make-os/lobe/util/identifier"
	"github.com/vmihailenco/msgpack"
)

type TxCode int

// BaseTx describes a base transaction
type BaseTx interface {
	msgpack.CustomEncoder
	msgpack.CustomDecoder

	// GetType returns the type of the transaction
	GetType() TxCode

	// GetSignature returns the transaction signature
	GetSignature() []byte

	// SetSignature sets the transaction signature
	SetSignature(s []byte)

	// GetSenderPubKey returns the transaction sender public key
	GetSenderPubKey() crypto.PublicKey

	// SetSenderPubKey sets the transaction sender public key
	SetSenderPubKey(pk []byte)

	// GetTimestamp return the transaction creation unix timestamp
	GetTimestamp() int64

	// SetTimestamp sets the transaction creation unix timestamp
	SetTimestamp(t int64)

	// GetNonce returns the transaction nonce
	GetNonce() uint64

	// SetNonce set the transaction nonce
	SetNonce(nonce uint64)

	// SetFee sets the transaction fee
	SetFee(fee util.String)

	// GetFee returns the transaction fee
	GetFee() util.String

	// GetFrom returns the address of the transaction sender
	GetFrom() identifier.Address

	// GetHash returns the hash of the transaction
	GetHash() util.HexBytes

	// GetBytesNoSig returns the serialized the tx excluding the signature
	GetBytesNoSig() []byte

	//  Bytes Returns the serialized transaction
	Bytes() []byte

	// ComputeHash computes the hash of the transaction
	ComputeHash() util.Bytes32

	// GetID returns the id of the transaction (also the hash)
	GetID() string

	// Sign signs the transaction
	Sign(privKey string) ([]byte, error)

	// GetEcoSize returns the size of the tx for use in fee calculation.
	// Size returned here may not be the actual tx size.
	GetEcoSize() int64

	// GetSize returns the size of the tx object (excluding nothing)
	GetSize() int64

	// ToBasicMap returns a map equivalent of the transaction
	ToMap() map[string]interface{}

	// FromMap populate the fields from a map
	FromMap(map[string]interface{}) error

	// GetMeta returns the meta information of the transaction
	GetMeta() map[string]interface{}

	// Id checks if the tx is a given type
	Is(txType TxCode) bool
}

// ProposalTx describes a proposal creating transaction
type ProposalTx interface {
	GetProposalID() string
	GetProposalRepoName() string
	GetProposalValue() util.String
}
