package txns

import (
	"fmt"

	"github.com/fatih/structs"
	"github.com/stretchr/objx"
	"github.com/vmihailenco/msgpack"
	"gitlab.com/makeos/mosdef/util"
	"gitlab.com/makeos/mosdef/util/crypto"
)

// TxNamespaceAcquire implements BaseTx, it describes a transaction for acquiring a namespace
type TxNamespaceAcquire struct {
	*TxType    `json:",flatten" msgpack:"-" mapstructure:"-"`
	*TxCommon  `json:",flatten" msgpack:"-" mapstructure:"-"`
	*TxValue   `json:",flatten" msgpack:"-" mapstructure:"-"`
	Name       string            `json:"name" msgpack:"name" mapstructure:"name"`          // The name of the namespace
	TransferTo string            `json:"to" msgpack:"to" mapstructure:"to"`                // Name of repo or address that will own the name.
	Domains    map[string]string `json:"domains" msgpack:"domains" mapstructure:"domains"` // Dictionary of namespace domains and their target
}

// NewBareTxNamespaceAcquire returns an instance of TxNamespaceAcquire with zero values
func NewBareTxNamespaceAcquire() *TxNamespaceAcquire {
	return &TxNamespaceAcquire{
		TxType:     &TxType{Type: TxTypeNSAcquire},
		TxCommon:   NewBareTxCommon(),
		TxValue:    &TxValue{Value: "0"},
		Name:       "",
		TransferTo: "",
		Domains:    make(map[string]string),
	}
}

// EncodeMsgpack implements msgpack.CustomEncoder
func (tx *TxNamespaceAcquire) EncodeMsgpack(enc *msgpack.Encoder) error {
	return tx.EncodeMulti(enc,
		tx.Type,
		tx.Nonce,
		tx.Fee,
		tx.Sig,
		tx.Timestamp,
		tx.SenderPubKey,
		tx.Name,
		tx.Value,
		tx.TransferTo,
		tx.Domains)
}

// DecodeMsgpack implements msgpack.CustomDecoder
func (tx *TxNamespaceAcquire) DecodeMsgpack(dec *msgpack.Decoder) error {
	return tx.DecodeMulti(dec,
		&tx.Type,
		&tx.Nonce,
		&tx.Fee,
		&tx.Sig,
		&tx.Timestamp,
		&tx.SenderPubKey,
		&tx.Name,
		&tx.Value,
		&tx.TransferTo,
		&tx.Domains)
}

// Bytes returns the serialized transaction
func (tx *TxNamespaceAcquire) Bytes() []byte {
	return util.ToBytes(tx)
}

// GetBytesNoSig returns the serialized the transaction excluding the signature
func (tx *TxNamespaceAcquire) GetBytesNoSig() []byte {
	sig := tx.Sig
	tx.Sig = nil
	bz := tx.Bytes()
	tx.Sig = sig
	return bz
}

// ComputeHash computes the hash of the transaction
func (tx *TxNamespaceAcquire) ComputeHash() util.Bytes32 {
	return util.BytesToBytes32(crypto.Blake2b256(tx.Bytes()))
}

// GetHash returns the hash of the transaction
func (tx *TxNamespaceAcquire) GetHash() util.Bytes32 {
	return tx.ComputeHash()
}

// GetID returns the id of the transaction (also the hash)
func (tx *TxNamespaceAcquire) GetID() string {
	return tx.ComputeHash().HexStr()
}

// GetEcoSize returns the size of the transaction for use in protocol economics
func (tx *TxNamespaceAcquire) GetEcoSize() int64 {
	return tx.GetSize()
}

// GetSize returns the size of the tx object (excluding nothing)
func (tx *TxNamespaceAcquire) GetSize() int64 {
	return int64(len(tx.Bytes()))
}

// Sign signs the transaction
func (tx *TxNamespaceAcquire) Sign(privKey string) ([]byte, error) {
	return SignTransaction(tx, privKey)
}

// ToMap returns a map equivalent of the transaction
func (tx *TxNamespaceAcquire) ToMap() map[string]interface{} {
	s := structs.New(tx)
	s.TagName = "json"
	return s.Map()
}

// FromMap populates fields from a map.
// Note: Default or zero values may be set for fields that aren't present in the
// map. Also, an error will be returned when unable to convert types in map to
// actual types in the object.
func (tx *TxNamespaceAcquire) FromMap(data map[string]interface{}) error {
	err := tx.TxCommon.FromMap(data)
	err = util.CallOnNilErr(err, func() error { return tx.TxType.FromMap(data) })
	err = util.CallOnNilErr(err, func() error { return tx.TxValue.FromMap(data) })

	o := objx.New(data)

	// Name: expects string type in map
	if nameVal := o.Get("name"); !nameVal.IsNil() {
		if nameVal.IsStr() {
			tx.Name = nameVal.Str()
		} else {
			return util.FieldError("name", fmt.Sprintf("invalid value type: has %T, "+
				"wants string", nameVal.Inter()))
		}
	}

	// TransferTo: expects string type in map
	if transferTo := o.Get("to"); !transferTo.IsNil() {
		if transferTo.IsStr() {
			tx.TransferTo = transferTo.Str()
		} else {
			return util.FieldError("to", fmt.Sprintf("invalid value type: has %T, "+
				"wants string", transferTo.Inter()))
		}
	}

	// Domains: expects map[string]string type in map
	if domainsVal := o.Get("domains"); !domainsVal.IsNil() {
		if domainsVal.IsObjxMap() {
			if tx.Domains == nil {
				tx.Domains = make(map[string]string)
			}
			for k, v := range domainsVal.Inter().(map[string]interface{}) {
				tx.Domains[k] = v.(string)
			}
		} else {
			return util.FieldError("domains", fmt.Sprintf("invalid value type: has %T, "+
				"wants map[string]string", domainsVal.Inter()))
		}
	}

	return err
}
