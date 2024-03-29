package txns

import (
	"bytes"
	"fmt"

	"github.com/make-os/kit/crypto/ed25519"
	"github.com/make-os/kit/types"
	"github.com/make-os/kit/util"
	"github.com/make-os/kit/util/identifier"
	"github.com/vmihailenco/msgpack"
	msgpack2 "github.com/vmihailenco/msgpack/v4"
)

// All Transaction type
const (
	TxTypeCoinTransfer                types.TxCode = iota + 1 // For native coin transfer to/between accounts
	TxTypeValidatorTicket                                     // For validator ticket purchase
	TxTypeHostTicket                                          // For purchasing host ticket
	TxTypeSetDelegatorCommission                              // For setting delegator commission
	TxTypeUnbondHostTicket                                    // For unbonding host ticket
	TxTypeRepoCreate                                          // For creating a repository
	TxTypeRegisterPushKey                                     // For adding a push key
	TxTypePush                                                // For pushing updates to a repository
	TxTypeNamespaceRegister                                   // For namespace purchase
	TxTypeNamespaceDomainUpdate                               // For setting namespace domains
	TxTypeRepoProposalUpsertOwner                             // For creating a proposal to add repo owner
	TxTypeRepoProposalVote                                    // For voting on a repo proposal
	TxTypeRepoProposalUpdate                                  // For creating a repo update proposal
	TxTypeRepoProposalSendFee                                 // For native coin transfer to repo as proposal fee
	TxTypeRepoProposalRegisterPushKey                         // For adding push keys to a repo
	TxTypeUpDelPushKey                                        // For updating or deleting a push key
	TxTypeMergeRequestProposalAction                          // For identifying merge request proposal
)

// TxType implements some of BaseTx, it includes type information about a transaction
type TxType struct {
	Type types.TxCode `json:"type" msgpack:"type" mapstructure:"type"`
}

// GetType returns the type of the transaction
func (tx *TxType) GetType() types.TxCode {
	return tx.Type
}

// Is checks if the tx is a given type
func (tx *TxType) Is(txType types.TxCode) bool {
	return tx.Type == txType
}

// FromMap populates tx with a map generated by tx.ToMap.
func (tx *TxType) FromMap(data map[string]interface{}) (err error) {
	return util.DecodeMap(data, &tx)
}

// TxCommon implements some of BaseTx, it includes some common fields and methods
type TxCommon struct {
	util.CodecUtil   `json:"-" msgpack:"-" mapstructure:"-"`
	*types.BasicMeta `json:"-" msgpack:"-" mapstructure:"-"`
	Nonce            uint64            `json:"nonce" msgpack:"nonce" mapstructure:"nonce"`
	Fee              util.String       `json:"fee" msgpack:"fee" mapstructure:"fee"`
	Sig              util.Bytes        `json:"sig" msgpack:"sig" mapstructure:"sig"`
	Timestamp        int64             `json:"timestamp" msgpack:"timestamp" mapstructure:"timestamp"`
	SenderPubKey     ed25519.PublicKey `json:"senderPubKey" msgpack:"senderPubKey" mapstructure:"senderPubKey"`
}

func (tx *TxCommon) EncodeMsgpack(enc *msgpack.Encoder) error {
	return tx.EncodeMulti(enc,
		tx.Nonce,
		tx.Fee,
		tx.Sig,
		tx.Timestamp,
		tx.SenderPubKey.Bytes())
}

func (tx *TxCommon) DecodeMsgpack(dec *msgpack2.Decoder) error {
	return tx.DecodeMulti(dec,
		&tx.Nonce,
		&tx.Fee,
		&tx.Sig,
		&tx.Timestamp,
		&tx.SenderPubKey)
}

// NewBareTxCommon returns an instance of TxCommon with zero values
func NewBareTxCommon() *TxCommon {
	return &TxCommon{
		BasicMeta:    types.NewMeta(),
		Nonce:        0,
		Fee:          "0",
		Timestamp:    0,
		SenderPubKey: ed25519.EmptyPublicKey,
	}
}

// FromMap populates tx with a map generated by tx.ToMap.
func (tx *TxCommon) FromMap(data map[string]interface{}) (err error) {
	return util.DecodeMap(data, &tx)
}

// GetFee returns the transaction nonce
func (tx *TxCommon) GetFee() util.String {
	return tx.Fee
}

// SetFee returns the transaction nonce
func (tx *TxCommon) SetFee(fee util.String) {
	tx.Fee = fee
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
func (tx *TxCommon) GetSenderPubKey() ed25519.PublicKey {
	return tx.SenderPubKey
}

// SetSenderPubKey set the transaction sender public key
func (tx *TxCommon) SetSenderPubKey(pk []byte) {
	tx.SenderPubKey = ed25519.BytesToPublicKey(pk)
}

// GetFrom returns the address of the transaction sender
// Panics if sender's public key is invalid
func (tx *TxCommon) GetFrom() identifier.Address {
	pk, err := ed25519.PubKeyFromBytes(tx.SenderPubKey.Bytes())
	if err != nil {
		panic(err)
	}
	return pk.Addr()
}

// SignTransaction signs a transaction.
// Expects private key in base58Check encoding.
func SignTransaction(tx types.BaseTx, privKey string) ([]byte, error) {
	pKey, err := ed25519.PrivKeyFromBase58(privKey)
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
	To identifier.Address `json:"to" msgpack:"to" mapstructure:"to"`
}

// SetRecipient sets the recipient
func (tx *TxRecipient) SetRecipient(to identifier.Address) {
	tx.To = to
}

// FromMap populates tx with a map generated by tx.ToMap.
func (tx *TxRecipient) FromMap(data map[string]interface{}) (err error) {
	return util.DecodeMap(data, &tx)
}

// TxValue describes a transaction value
type TxValue struct {
	Value util.String `json:"value" msgpack:"value" mapstructure:"value"`
}

// SetValue sets the value
func (tx *TxValue) SetValue(value util.String) {
	tx.Value = value
}

// FromMap populates tx with a map generated by tx.ToMap.
func (tx *TxValue) FromMap(data map[string]interface{}) (err error) {
	return util.DecodeMap(data, &tx)
}

// TxProposalCommon describes fields for a proposal
type TxProposalCommon struct {

	// RepoName is the target repository to create the proposal on.
	RepoName string `json:"name" msgpack:"name" mapstructure:"name"`

	// Value is the sometimes optional proposal fee
	Value util.String `json:"value" msgpack:"value" mapstructure:"value"`

	// ID is the proposal ID
	ID string `json:"id,omitempty" msgpack:"id" mapstructure:"id"`
}

// GetProposalID returns the proposal ID
func (tx *TxProposalCommon) GetProposalID() string {
	return tx.ID
}

// GetProposalValue returns the proposal value
func (tx *TxProposalCommon) GetProposalValue() util.String {
	return tx.Value
}

// GetProposalRepoName returns the target repository name
func (tx *TxProposalCommon) GetProposalRepoName() string {
	return tx.RepoName
}

// FromMap populates tx with a map generated by tx.ToMap.
func (tx *TxProposalCommon) FromMap(data map[string]interface{}) (err error) {
	return util.DecodeMap(data, &tx)
}

// TxDescription describes a transaction
type TxDescription struct {
	Description string `json:"desc" msgpack:"desc" mapstructure:"desc"`
}

func (tx *TxDescription) SetDescription(desc string) {
	tx.Description = desc
}

// FromMap populates tx with a map generated by tx.ToMap.
func (tx *TxDescription) FromMap(data map[string]interface{}) (err error) {
	return util.DecodeMap(data, &tx)
}

// DecodeTxFromMap decodes a user-provided map to a transaction object.
func DecodeTxFromMap(data map[string]interface{}) (types.BaseTx, error) {
	txType := &TxType{}
	if err := txType.FromMap(data); err != nil {
		return nil, err
	}

	txObj, err := getBareTxObject(txType.Type)
	if err != nil {
		return nil, err
	}

	return txObj, txObj.FromMap(data)
}

// DecodeTx decodes msgpack data to transactions.
func DecodeTx(txBz []byte) (types.BaseTx, error) {
	dec := msgpack.NewDecoder(bytes.NewBuffer(txBz))

	// Skip object version
	dec.Skip()

	// Decode transaction type
	txType, err := dec.DecodeInt()
	if err != nil {
		return nil, fmt.Errorf("failed to decode tx type")
	}

	// Get the appropriate object for the transaction type
	tx, err := getBareTxObject(types.TxCode(txType))
	if err != nil {
		return nil, err
	}

	// Decode and return any error
	return tx, util.ToObject(txBz, tx)
}

func getBareTxObject(txType types.TxCode) (types.BaseTx, error) {
	var tx interface{}
	switch txType {
	case TxTypeCoinTransfer:
		tx = NewBareTxCoinTransfer()
	case TxTypeValidatorTicket:
		tx = NewBareTxTicketPurchase(TxTypeValidatorTicket)
	case TxTypeHostTicket:
		tx = NewBareTxTicketPurchase(TxTypeHostTicket)
	case TxTypeSetDelegatorCommission:
		tx = NewBareTxSetDelegateCommission()
	case TxTypeUnbondHostTicket:
		tx = NewBareTxTicketUnbond(TxTypeUnbondHostTicket)
	case TxTypeRepoCreate:
		tx = NewBareTxRepoCreate()
	case TxTypeRegisterPushKey:
		tx = NewBareTxRegisterPushKey()
	case TxTypePush:
		tx = NewBareTxPush()
	case TxTypeNamespaceRegister:
		tx = NewBareTxNamespaceRegister()
	case TxTypeNamespaceDomainUpdate:
		tx = NewBareTxNamespaceDomainUpdate()
	case TxTypeRepoProposalUpsertOwner:
		tx = NewBareRepoProposalUpsertOwner()
	case TxTypeRepoProposalVote:
		tx = NewBareRepoProposalVote()
	case TxTypeRepoProposalUpdate:
		tx = NewBareRepoProposalUpdate()
	case TxTypeRepoProposalSendFee:
		tx = NewBareRepoProposalFeeSend()
	case TxTypeRepoProposalRegisterPushKey:
		tx = NewBareRepoProposalRegisterPushKey()
	case TxTypeUpDelPushKey:
		tx = NewBareTxUpDelPushKey()
	default:
		return nil, fmt.Errorf("unsupported tx type")
	}

	return tx.(types.BaseTx), nil
}
