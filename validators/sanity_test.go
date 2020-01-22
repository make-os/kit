package validators_test

import (
	"io/ioutil"
	"os"
	"time"

	"github.com/makeos/mosdef/params"
	"github.com/shopspring/decimal"

	"github.com/makeos/mosdef/crypto"
	"github.com/makeos/mosdef/util"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/testutil"
	"github.com/makeos/mosdef/types"
	"github.com/makeos/mosdef/validators"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("TxValidator", func() {
	var err error
	var cfg *config.AppConfig
	var key = crypto.NewKeyFromIntSeed(1)
	var key2 = crypto.NewKeyFromIntSeed(2)

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		params.FeePerByte = decimal.NewFromFloat(0.001)
	})

	AfterEach(func() {
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".CheckTxCoinTransfer", func() {
		var tx *types.TxCoinTransfer
		BeforeEach(func() {
			tx = types.NewBareTxCoinTransfer()
			tx.To = key.Addr()
			tx.Fee = "1"
		})

		When("it has invalid fields, it should return error when", func() {
			It("should return error='type is invalid'", func() {
				tx.Type = -10
				err := validators.CheckTxCoinTransfer(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:type, error:type is invalid"))
			})

			It("has no recipient address", func() {
				tx.To = ""
				err := validators.CheckTxCoinTransfer(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:to, error:recipient address is required"))
			})

			It("has invalid recipient address", func() {
				tx.To = "invalid"
				err := validators.CheckTxCoinTransfer(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:to, error:recipient address is not valid"))
			})

			It("has invalid value", func() {
				tx.Value = "invalid"
				err := validators.CheckTxCoinTransfer(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:value, error:invalid number; must be numeric"))
			})

			It("has no nonce", func() {
				err := validators.CheckTxCoinTransfer(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:nonce, error:nonce is required"))
			})

			It("has invalid fee", func() {
				tx.Nonce = 1
				tx.Fee = "invalid"
				err := validators.CheckTxCoinTransfer(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:fee, error:invalid number; must be numeric"))
			})

			It("has low fee", func() {
				tx.Nonce = 1
				tx.Fee = "0"
				err := validators.CheckTxCoinTransfer(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("field:fee, error:fee cannot be lower than the base price"))
			})

			It("has no timestamp", func() {
				tx.Nonce = 1
				err := validators.CheckTxCoinTransfer(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:timestamp, error:timestamp is required"))
			})

			It("has no public key", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				err := validators.CheckTxCoinTransfer(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:senderPubKey, error:sender public key is required"))
			})

			It("has no signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToBytes32(key.PubKey().MustBytes())
				err := validators.CheckTxCoinTransfer(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, error:signature is required"))
			})

			It("has invalid signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToBytes32(key.PubKey().MustBytes())
				tx.Sig = []byte("invalid")
				err := validators.CheckTxCoinTransfer(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, error:signature is not valid"))
			})
		})

		When("it has no error", func() {
			It("should return no error", func() {
				tx.To = key.Addr()
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToBytes32(key.PubKey().MustBytes())
				sig, err := tx.Sign(key.PrivKey().Base58())
				Expect(err).To(BeNil())
				tx.Sig = sig
				err = validators.CheckTxCoinTransfer(tx, -1)
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".CheckTxNSAcquire", func() {
		var tx *types.TxNamespaceAcquire
		BeforeEach(func() {
			params.CostOfNamespace = decimal.NewFromFloat(5)
			tx = types.NewBareTxNamespaceAcquire()
			tx.Fee = "1"
			tx.Name = "namespace"
			tx.Value = util.String(params.CostOfNamespace.String())
			tx.Nonce = 1
		})

		When("it has invalid fields, it should return error when", func() {
			It("should return error='type is invalid'", func() {
				tx.Type = -10
				err := validators.CheckTxNSAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:type, error:type is invalid"))
			})

			It("has invalid value", func() {
				tx.Value = "invalid"
				err := validators.CheckTxNSAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:value, error:invalid number; must be numeric"))
			})

			It("has no name", func() {
				tx.Name = ""
				err := validators.CheckTxNSAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:name, error:requires a unique name"))
			})

			It("has an invalid name", func() {
				tx.Name = "invalid&"
				err := validators.CheckTxNSAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:name, error:invalid characters in name. Only alphanumeric, _ and - characters are allowed"))
			})

			It("has transfer repo and account fields set", func() {
				tx.TransferToRepo = "repo"
				tx.TransferToAccount = "account"
				err := validators.CheckTxNSAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:, error:can only transfer ownership to either an account or a repo"))
			})

			It("has invalid transfer account", func() {
				tx.TransferToAccount = "account"
				err := validators.CheckTxNSAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:transferToAccount, error:address is not valid"))
			})

			It("has value not equal to namespace price", func() {
				tx.Value = "1"
				err := validators.CheckTxNSAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:value, error:invalid value; has 1, want 5"))
			})

			It("has domain target with invalid format", func() {
				tx.Value = "5"
				tx.Domains["domain"] = "invalid:format"
				err := validators.CheckTxNSAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:domains, error:domains.domain target format is invalid"))
			})

			It("has domain target with unknown target type", func() {
				tx.Value = "5"
				tx.Domains["domain"] = "unknown_type/name"
				err := validators.CheckTxNSAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:domains, error:domains.domain has unknown target type"))
			})

			It("has domain target with account target type that has an invalid address", func() {
				tx.Value = "5"
				tx.Domains["domain"] = "a/invalid_addr"
				err := validators.CheckTxNSAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:domains, error:domains.domain has invalid address"))
			})

			It("has invalid fee", func() {
				tx.Nonce = 1
				tx.Fee = "invalid"
				err := validators.CheckTxNSAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:fee, error:invalid number; must be numeric"))
			})

			It("has low fee", func() {
				tx.Nonce = 1
				tx.Fee = "0"
				err := validators.CheckTxNSAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("field:fee, error:fee cannot be lower than the base price"))
			})

			It("has no nonce", func() {
				tx.Nonce = 0
				err := validators.CheckTxNSAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:nonce, error:nonce is required"))
			})

			It("has no timestamp", func() {
				tx.Nonce = 1
				err := validators.CheckTxNSAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:timestamp, error:timestamp is required"))
			})

			It("has no public key", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				err := validators.CheckTxNSAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:senderPubKey, error:sender public key is required"))
			})

			It("has no signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToBytes32(key.PubKey().MustBytes())
				err := validators.CheckTxNSAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, error:signature is required"))
			})

			It("has invalid signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToBytes32(key.PubKey().MustBytes())
				tx.Sig = []byte("invalid")
				err := validators.CheckTxNSAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, error:signature is not valid"))
			})
		})

		When("it has no error", func() {
			It("should return no error", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToBytes32(key.PubKey().MustBytes())
				tx.Domains["domain"] = "r/repo1"
				sig, err := tx.Sign(key.PrivKey().Base58())
				Expect(err).To(BeNil())
				tx.Sig = sig
				err = validators.CheckTxNSAcquire(tx, -1)
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".CheckTxTicketPurchase", func() {
		var tx *types.TxTicketPurchase
		BeforeEach(func() {
			tx = types.NewBareTxTicketPurchase(types.TxTypeValidatorTicket)
			tx.Fee = "1"
			tx.VRFPubKey = util.BytesToBytes32(util.RandBytes(32))
		})

		When("it has invalid fields, it should return error when", func() {
			It("should return error='type is invalid'", func() {
				tx.Type = -10
				err := validators.CheckTxTicketPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:type, error:type is invalid"))
			})

			It("has invalid value", func() {
				tx.Value = "invalid"
				err := validators.CheckTxTicketPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:value, error:invalid number; must be numeric"))
			})

			It("has type of TxTypeStorerTicket and value is lower than minimum stake", func() {
				params.MinStorerStake = decimal.NewFromFloat(20)
				tx.Type = types.TxTypeStorerTicket
				tx.Value = "10"
				err := validators.CheckTxTicketPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:value, error:value is lower than minimum storer stake"))
			})

			It("has no nonce", func() {
				// tx.BLSPubKey = util.RandBytes(128)
				err := validators.CheckTxTicketPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:nonce, error:nonce is required"))
			})

			It("has invalid fee", func() {
				tx.Nonce = 1
				tx.Fee = "invalid"
				err := validators.CheckTxTicketPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:fee, error:invalid number; must be numeric"))
			})

			It("has no timestamp", func() {
				tx.Nonce = 1
				err := validators.CheckTxTicketPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:timestamp, error:timestamp is required"))
			})

			It("has no public key", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				err := validators.CheckTxTicketPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:senderPubKey, error:sender public key is required"))
			})

			It("has no signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToBytes32(key.PubKey().MustBytes())
				err := validators.CheckTxTicketPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, error:signature is required"))
			})

			It("has invalid signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToBytes32(key.PubKey().MustBytes())
				tx.Sig = []byte("invalid")
				err := validators.CheckTxTicketPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, error:signature is not valid"))
			})

			It("has type of TxTypeValidatorTicket and VRF public key is unset", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.VRFPubKey = util.EmptyBytes32
				err := validators.CheckTxTicketPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:vrfPubKey, error:VRF public key is required"))
			})

			It("has type of TxTypeStorerTicket and BLS public key is unset", func() {
				params.MinStorerStake = decimal.NewFromFloat(5)
				tx.Value = "10"
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.Type = types.TxTypeStorerTicket
				err := validators.CheckTxTicketPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:blsPubKey, error:BLS public key is required"))
			})

			It("has type of TxTypeStorerTicket and BLS public key has invalid length", func() {
				params.MinStorerStake = decimal.NewFromFloat(5)
				tx.Value = "10"
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.Type = types.TxTypeStorerTicket
				tx.BLSPubKey = util.RandBytes(32)
				err := validators.CheckTxTicketPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:blsPubKey, error:BLS public key length is invalid"))
			})
		})

		When("it has no error", func() {
			It("should return no error", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToBytes32(key.PubKey().MustBytes())
				sig, err := tx.Sign(key.PrivKey().Base58())
				Expect(err).To(BeNil())
				tx.Sig = sig
				err = validators.CheckTxTicketPurchase(tx, -1)
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".CheckTxUnbondTicket", func() {
		var tx *types.TxTicketUnbond

		BeforeEach(func() {
			tx = types.NewBareTxTicketUnbond(types.TxTypeStorerTicket)
			tx.TicketHash = util.StrToBytes32("hash")
			tx.Fee = "1"
		})

		When("it has invalid fields, it should return error when", func() {
			It("should return error='type is invalid'", func() {
				tx.Type = -10
				err := validators.CheckTxUnbondTicket(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:type, error:type is invalid"))
			})

			It("has no ticket hash", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.TicketHash = util.EmptyBytes32
				err := validators.CheckTxUnbondTicket(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:ticket, error:ticket id is required"))
			})

			It("has no nonce", func() {
				err := validators.CheckTxUnbondTicket(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:nonce, error:nonce is required"))
			})

			It("has invalid fee", func() {
				tx.Nonce = 1
				tx.Fee = "invalid"
				err := validators.CheckTxUnbondTicket(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:fee, error:invalid number; must be numeric"))
			})

			It("has no timestamp", func() {
				tx.Nonce = 1
				err := validators.CheckTxUnbondTicket(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:timestamp, error:timestamp is required"))
			})

			It("has no public key", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				err := validators.CheckTxUnbondTicket(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:senderPubKey, error:sender public key is required"))
			})

			It("has no signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToBytes32(key.PubKey().MustBytes())
				err := validators.CheckTxUnbondTicket(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, error:signature is required"))
			})

			It("has invalid signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToBytes32(key.PubKey().MustBytes())
				tx.Sig = []byte("invalid")
				err := validators.CheckTxUnbondTicket(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, error:signature is not valid"))
			})
		})

		When("it has no error", func() {
			It("should return no error", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToBytes32(key.PubKey().MustBytes())
				sig, err := tx.Sign(key.PrivKey().Base58())
				Expect(err).To(BeNil())
				tx.Sig = sig
				err = validators.CheckTxUnbondTicket(tx, -1)
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".CheckTxRepoCreate", func() {
		var tx *types.TxRepoCreate
		BeforeEach(func() {
			tx = types.NewBareTxRepoCreate()
			tx.Name = "repo"
			tx.Fee = "1"
		})

		When("it has invalid fields, it should return error when", func() {
			It("should return error='type is invalid'", func() {
				tx.Type = -10
				err := validators.CheckTxRepoCreate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:type, error:type is invalid"))
			})

			It("has invalid value", func() {
				tx.Value = "invalid"
				err := validators.CheckTxRepoCreate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:value, error:invalid number; must be numeric"))
			})

			It("has no name", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.Name = ""
				err := validators.CheckTxRepoCreate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:name, error:requires a unique name"))
			})

			It("has invalid name", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.Name = "org&name#"
				err := validators.CheckTxRepoCreate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:name, error:invalid characters in name. Only alphanumeric, _ and - characters are allowed"))
			})

			It("has no nonce", func() {
				err := validators.CheckTxRepoCreate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:nonce, error:nonce is required"))
			})

			It("has invalid fee", func() {
				tx.Nonce = 1
				tx.Fee = "invalid"
				err := validators.CheckTxRepoCreate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:fee, error:invalid number; must be numeric"))
			})

			It("has no timestamp", func() {
				tx.Nonce = 1
				err := validators.CheckTxRepoCreate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:timestamp, error:timestamp is required"))
			})

			It("has no public key", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				err := validators.CheckTxRepoCreate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:senderPubKey, error:sender public key is required"))
			})

			It("has no signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToBytes32(key.PubKey().MustBytes())
				err := validators.CheckTxRepoCreate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, error:signature is required"))
			})

			It("has invalid signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToBytes32(key.PubKey().MustBytes())
				tx.Sig = []byte("invalid")
				err := validators.CheckTxRepoCreate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, error:signature is not valid"))
			})
		})

		When("it has no error", func() {
			It("should return no error", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToBytes32(key.PubKey().MustBytes())
				sig, err := tx.Sign(key.PrivKey().Base58())
				Expect(err).To(BeNil())
				tx.Sig = sig
				err = validators.CheckTxRepoCreate(tx, -1)
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".CheckTxEpochSeed", func() {
		var tx *types.TxEpochSeed
		BeforeEach(func() {
			tx = types.NewBareTxEpochSeed()
		})

		When("it has invalid fields, it should return error when", func() {
			It("should return error='type is invalid'", func() {
				tx.Type = -10
				err := validators.CheckTxEpochSeed(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:type, error:type is invalid"))
			})

			It("has no vrf output", func() {
				err := validators.CheckTxEpochSeed(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:output, error:output is required"))
			})

			It("has no vrf proof", func() {
				tx.Output = util.BytesToBytes32(util.RandBytes(32))
				err := validators.CheckTxEpochSeed(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:proof, error:proof is required"))
			})

			It("has vrf proof length not equal to 96 bytes", func() {
				tx.Output = util.BytesToBytes32(util.RandBytes(32))
				tx.Proof = []byte("invalid_length")
				err := validators.CheckTxEpochSeed(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:proof, error:proof length is invalid"))
			})

		})

		When("it has no error", func() {
			It("should return no error", func() {
				tx.Output = util.BytesToBytes32(util.RandBytes(32))
				tx.Proof = util.RandBytes(96)
				err := validators.CheckTxEpochSeed(tx, -1)
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".CheckTxAddGPGPubKey", func() {
		var tx *types.TxAddGPGPubKey
		var gpgKey []byte

		BeforeEach(func() {
			gpgKey, err = ioutil.ReadFile("testdata/gpgkey.pub")
			Expect(err).To(BeNil())
			tx = types.NewBareTxAddGPGPubKey()
			tx.PublicKey = string(gpgKey)
			tx.Fee = "2"
		})

		When("it has invalid fields, it should return error when", func() {
			It("should return error='type is invalid'", func() {
				tx.Type = -10
				err := validators.CheckTxAddGPGPubKey(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:type, error:type is invalid"))
			})

			It("has no gpg public key", func() {
				tx.PublicKey = ""
				err := validators.CheckTxAddGPGPubKey(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:pubKey, error:public key is required"))
			})

			It("has invalid gpg public key", func() {
				tx.PublicKey = "invalid"
				err := validators.CheckTxAddGPGPubKey(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:pubKey, error:invalid gpg public key"))
			})

			It("has no nonce", func() {
				err := validators.CheckTxAddGPGPubKey(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:nonce, error:nonce is required"))
			})

			It("has invalid fee", func() {
				tx.Nonce = 1
				tx.Fee = "invalid"
				err := validators.CheckTxAddGPGPubKey(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:fee, error:invalid number; must be numeric"))
			})

			It("has no timestamp", func() {
				tx.Nonce = 1
				err := validators.CheckTxAddGPGPubKey(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:timestamp, error:timestamp is required"))
			})

			It("has no public key", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				err := validators.CheckTxAddGPGPubKey(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:senderPubKey, error:sender public key is required"))
			})

			It("has no signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToBytes32(key.PubKey().MustBytes())
				err := validators.CheckTxAddGPGPubKey(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, error:signature is required"))
			})

			It("has invalid signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToBytes32(key.PubKey().MustBytes())
				tx.Sig = []byte("invalid")
				err := validators.CheckTxAddGPGPubKey(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, error:signature is not valid"))
			})
		})

		When("it has no error", func() {
			It("should return no error", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToBytes32(key.PubKey().MustBytes())
				sig, err := tx.Sign(key.PrivKey().Base58())
				Expect(err).To(BeNil())
				tx.Sig = sig
				err = validators.CheckTxAddGPGPubKey(tx, -1)
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".CheckTxSetDelegateCommission", func() {
		var tx *types.TxSetDelegateCommission

		BeforeEach(func() {
			tx = types.NewBareTxSetDelegateCommission()
			tx.Commission = "60"
			tx.Fee = "1"
		})

		When("it has invalid fields, it should return error when", func() {
			It("should return error='type is invalid'", func() {
				tx.Type = -10
				err := validators.CheckTxSetDelegateCommission(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:type, error:type is invalid"))
			})

			It("has no commission value", func() {
				tx.Commission = ""
				err := validators.CheckTxSetDelegateCommission(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:commission, error:commission rate is required"))
			})

			It("has no commission value is below minimum", func() {
				params.MinDelegatorCommission = decimal.NewFromFloat(50)
				tx.Commission = "49"
				err := validators.CheckTxSetDelegateCommission(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:commission, error:rate cannot be below the minimum (50%)"))
			})

			It("has no commission value is above 100", func() {
				params.MinDelegatorCommission = decimal.NewFromFloat(50)
				tx.Commission = "101"
				err := validators.CheckTxSetDelegateCommission(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:commission, error:commission rate cannot exceed 100%"))
			})

			It("has no nonce", func() {
				err := validators.CheckTxSetDelegateCommission(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:nonce, error:nonce is required"))
			})

			It("has invalid fee", func() {
				tx.Nonce = 1
				tx.Fee = "invalid"
				err := validators.CheckTxSetDelegateCommission(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:fee, error:invalid number; must be numeric"))
			})

			It("has no timestamp", func() {
				tx.Nonce = 1
				err := validators.CheckTxSetDelegateCommission(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:timestamp, error:timestamp is required"))
			})

			It("has no public key", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				err := validators.CheckTxSetDelegateCommission(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:senderPubKey, error:sender public key is required"))
			})

			It("has no signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToBytes32(key.PubKey().MustBytes())
				err := validators.CheckTxSetDelegateCommission(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, error:signature is required"))
			})

			It("has invalid signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToBytes32(key.PubKey().MustBytes())
				tx.Sig = []byte("invalid")
				err := validators.CheckTxSetDelegateCommission(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, error:signature is not valid"))
			})
		})

		When("it has no error", func() {
			It("should return no error", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToBytes32(key.PubKey().MustBytes())
				sig, err := tx.Sign(key.PrivKey().Base58())
				Expect(err).To(BeNil())
				tx.Sig = sig
				err = validators.CheckTxSetDelegateCommission(tx, -1)
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".CheckTxNamespaceDomainUpdate", func() {
		var tx *types.TxNamespaceDomainUpdate

		BeforeEach(func() {
			tx = types.NewBareTxNamespaceDomainUpdate()
			tx.Fee = "1"
		})

		When("it has invalid fields, it should return error when", func() {
			It("should return error='type is invalid'", func() {
				tx.Type = -10
				err := validators.CheckTxNamespaceDomainUpdate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:type, error:type is invalid"))
			})
		})

		When("name is not set", func() {
			It("should return err", func() {
				err := validators.CheckTxNamespaceDomainUpdate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:name, error:requires a name"))
			})
		})

		When("name is not valid", func() {
			It("should return err", func() {
				tx.Name = "&name"
				err := validators.CheckTxNamespaceDomainUpdate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:name, error:invalid characters in name. Only alphanumeric, _ and - characters are allowed"))
			})
		})

		When("name is too short", func() {
			It("should return err", func() {
				tx.Name = "ab"
				err := validators.CheckTxNamespaceDomainUpdate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:name, error:name is too short. Must be at least 3 characters long"))
			})
		})
	})

	Describe(".CheckTxPush", func() {
		var tx *types.TxPush

		BeforeEach(func() {
			tx = types.NewBareTxPush()
			tx.Timestamp = time.Now().Unix()
			tx.PushNote.RepoName = "repo1"
			tx.PushNote.PusherKeyID = util.RandBytes(20)
			tx.PushNote.Timestamp = time.Now().Unix()
			tx.PushNote.NodePubKey = key.PubKey().MustBytes32()
			tx.PushNote.NodeSig = key.PrivKey().MustSign(tx.PushNote.Bytes())
		})

		When("it has invalid fields, it should return error when", func() {
			It("should return error='type is invalid'", func() {
				tx.Type = -10
				err := validators.CheckTxPush(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:type, error:type is invalid"))
			})

			It("has no push note", func() {
				tx.PushNote = nil
				err := validators.CheckTxPush(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:pushNote, error:push note is required"))
			})

			It("has an invalid push note (with no repo name)", func() {
				tx.PushNote.RepoName = ""
				err := validators.CheckTxPush(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:repoName, error:repo name is required"))
			})

			It("has low endorsement (not up to quorum)", func() {
				params.PushOKQuorumSize = 1
				err := validators.CheckTxPush(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:endorsements, error:not enough endorsements included"))
			})

			It("has a no push note id", func() {
				params.PushOKQuorumSize = 1
				tx.PushOKs = append(tx.PushOKs, &types.PushOK{})
				tx.SenderPubKey = util.BytesToBytes32(key.PubKey().MustBytes())
				sig, _ := key.PrivKey().Sign(tx.Bytes())
				tx.Sig = sig
				err := validators.CheckTxPush(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("index:0, field:endorsements.pushNoteID, error:push note id is required"))
			})

			It("has a PushOK with no sender public key", func() {
				params.PushOKQuorumSize = 1
				tx.PushOKs = append(tx.PushOKs, &types.PushOK{
					PushNoteID:   util.StrToBytes32("id"),
					SenderPubKey: util.EmptyBytes32,
				})
				tx.SenderPubKey = util.BytesToBytes32(key.PubKey().MustBytes())
				sig, _ := key.PrivKey().Sign(tx.Bytes())
				tx.Sig = sig
				err := validators.CheckTxPush(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("index:0, field:endorsements.senderPubKey, error:sender public key is required"))
			})

			It("has a PushOK with a push note id that is different from the PushTx.PushNoteID", func() {
				params.PushOKQuorumSize = 1
				tx.PushOKs = append(tx.PushOKs, &types.PushOK{
					PushNoteID:   util.StrToBytes32("id"),
					SenderPubKey: key.PubKey().MustBytes32(),
				})
				tx.SenderPubKey = util.BytesToBytes32(key.PubKey().MustBytes())
				sig, _ := key.PrivKey().Sign(tx.Bytes())
				tx.Sig = sig
				err := validators.CheckTxPush(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("index:0, field:endorsements.pushNoteID, error:push note id and push endorsement id must match"))
			})

			It("has multiple PushOKs from same sender", func() {
				params.PushOKQuorumSize = 1

				pushOK1 := &types.PushOK{
					PushNoteID:   tx.PushNote.ID(),
					SenderPubKey: util.BytesToBytes32(key.PubKey().MustBytes()),
				}
				sig, _ := key.PrivKey().Sign(pushOK1.Bytes())
				pushOK1.Sig = util.BytesToBytes64(sig)
				tx.PushOKs = append(tx.PushOKs, pushOK1)

				pushOK2 := &types.PushOK{
					PushNoteID:   tx.PushNote.ID(),
					SenderPubKey: util.BytesToBytes32(key.PubKey().MustBytes()),
				}
				sig, _ = key.PrivKey().Sign(pushOK2.Bytes())
				pushOK2.Sig = util.BytesToBytes64(sig)
				tx.PushOKs = append(tx.PushOKs, pushOK2)

				tx.SenderPubKey = util.BytesToBytes32(key.PubKey().MustBytes())
				sig, _ = key.PrivKey().Sign(tx.Bytes())
				tx.Sig = sig
				err := validators.CheckTxPush(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("index:1, field:endorsements.senderPubKey, error:multiple endorsement by a single sender not permitted"))
			})

			It("has PushOKs with different references hash set", func() {
				params.PushOKQuorumSize = 1

				pushOK1 := &types.PushOK{
					PushNoteID:   tx.PushNote.ID(),
					SenderPubKey: util.BytesToBytes32(key.PubKey().MustBytes()),
					ReferencesHash: []*types.ReferenceHash{
						{Hash: util.BytesToBytes32(util.RandBytes(32))},
					},
				}
				sig, _ := key.PrivKey().Sign(pushOK1.Bytes())
				pushOK1.Sig = util.BytesToBytes64(sig)
				tx.PushOKs = append(tx.PushOKs, pushOK1)

				pushOK2 := &types.PushOK{
					PushNoteID:   tx.PushNote.ID(),
					SenderPubKey: util.BytesToBytes32(key2.PubKey().MustBytes()),
					ReferencesHash: []*types.ReferenceHash{
						{Hash: util.BytesToBytes32(util.RandBytes(32))},
					},
				}
				sig, _ = key2.PrivKey().Sign(pushOK2.Bytes())
				pushOK2.Sig = util.BytesToBytes64(sig)
				tx.PushOKs = append(tx.PushOKs, pushOK2)

				tx.SenderPubKey = util.BytesToBytes32(key.PubKey().MustBytes())
				sig, _ = key.PrivKey().Sign(tx.Bytes())
				tx.Sig = sig
				err := validators.CheckTxPush(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("index:1, field:endorsements.refsHash, error:references of all endorsements must match"))
			})
		})

		When("no error", func() {
			It("should return no error", func() {
				params.PushOKQuorumSize = 1

				pok := &types.PushOK{
					PushNoteID:   tx.PushNote.ID(),
					SenderPubKey: util.BytesToBytes32(key.PubKey().MustBytes()),
				}
				sig, _ := key.PrivKey().Sign(pok.Bytes())
				pok.Sig = util.BytesToBytes64(sig)

				tx.PushOKs = append(tx.PushOKs, pok)
				tx.SenderPubKey = util.BytesToBytes32(key.PubKey().MustBytes())
				sig, _ = key.PrivKey().Sign(tx.Bytes())
				tx.Sig = sig

				err := validators.CheckTxPush(tx, -1)
				Expect(err).To(BeNil())
			})
		})
	})
})
