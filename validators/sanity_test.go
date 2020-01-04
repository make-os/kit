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
			It("has invalid type", func() {
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

	Describe(".CheckTxNSPurchase", func() {
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
			It("has invalid type", func() {
				tx.Type = -10
				err := validators.CheckTxNSPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:type, error:type is invalid"))
			})

			It("has invalid value", func() {
				tx.Value = "invalid"
				err := validators.CheckTxNSPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:value, error:invalid number; must be numeric"))
			})

			It("has no name", func() {
				tx.Name = ""
				err := validators.CheckTxNSPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:name, error:requires a unique name"))
			})

			It("has an invalid name", func() {
				tx.Name = "invalid&"
				err := validators.CheckTxNSPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:name, error:invalid characters in name. Only alphanumeric, _ and - characters are allowed"))
			})

			It("has transfer repo and account fields set", func() {
				tx.TransferToRepo = "repo"
				tx.TransferToAccount = "account"
				err := validators.CheckTxNSPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:, error:can only transfer ownership to either an account or a repo"))
			})

			It("has invalid transfer account", func() {
				tx.TransferToAccount = "account"
				err := validators.CheckTxNSPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:transferToAccount, error:address is not valid"))
			})

			It("has value not equal to namespace price", func() {
				tx.Value = "1"
				err := validators.CheckTxNSPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:value, error:invalid value; has 1, want 5"))
			})

			It("has domain target with invalid format", func() {
				tx.Value = "5"
				tx.Domains["domain"] = "invalid:format"
				err := validators.CheckTxNSPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:domains, error:domains.domain target format is invalid"))
			})

			It("has domain target with unknown target type", func() {
				tx.Value = "5"
				tx.Domains["domain"] = "unknown_type/name"
				err := validators.CheckTxNSPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:domains, error:domains.domain has unknown target type"))
			})

			It("has domain target with account target type that has an invalid address", func() {
				tx.Value = "5"
				tx.Domains["domain"] = "a/invalid_addr"
				err := validators.CheckTxNSPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:domains, error:domains.domain has invalid address"))
			})

			It("has invalid fee", func() {
				tx.Nonce = 1
				tx.Fee = "invalid"
				err := validators.CheckTxNSPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:fee, error:invalid number; must be numeric"))
			})

			It("has low fee", func() {
				tx.Nonce = 1
				tx.Fee = "0"
				err := validators.CheckTxNSPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("field:fee, error:fee cannot be lower than the base price"))
			})

			It("has no nonce", func() {
				tx.Nonce = 0
				err := validators.CheckTxNSPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:nonce, error:nonce is required"))
			})

			It("has no timestamp", func() {
				tx.Nonce = 1
				err := validators.CheckTxNSPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:timestamp, error:timestamp is required"))
			})

			It("has no public key", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				err := validators.CheckTxNSPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:senderPubKey, error:sender public key is required"))
			})

			It("has no signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToBytes32(key.PubKey().MustBytes())
				err := validators.CheckTxNSPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, error:signature is required"))
			})

			It("has invalid signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToBytes32(key.PubKey().MustBytes())
				tx.Sig = []byte("invalid")
				err := validators.CheckTxNSPurchase(tx, -1)
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
				err = validators.CheckTxNSPurchase(tx, -1)
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".CheckTxTicketPurchase", func() {
		var tx *types.TxTicketPurchase
		BeforeEach(func() {
			tx = types.NewBareTxTicketPurchase(types.TxTypeValidatorTicket)
			tx.Fee = "1"
		})

		When("it has invalid fields, it should return error when", func() {
			It("has invalid type", func() {
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
			tx.TicketHash = "hash"
			tx.Fee = "1"
		})

		When("it has invalid fields, it should return error when", func() {
			It("has invalid type", func() {
				tx.Type = -10
				err := validators.CheckTxUnbondTicket(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:type, error:type is invalid"))
			})

			It("has no ticket hash", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.TicketHash = ""
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
			It("has invalid type", func() {
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

	Describe(".CheckTxEpochSecret", func() {
		var tx *types.TxEpochSecret
		BeforeEach(func() {
			tx = types.NewBareTxEpochSecret()
			tx.Fee = "1"
		})

		When("it has invalid fields, it should return error when", func() {
			It("has invalid type", func() {
				tx.Type = -10
				err := validators.CheckTxEpochSecret(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:type, error:type is invalid"))
			})

			It("has no secret", func() {
				err := validators.CheckTxEpochSecret(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:secret, error:secret is required"))
			})

			It("has less than 64 bytes secret", func() {
				tx.Secret = []byte("invalid length")
				err := validators.CheckTxEpochSecret(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:secret, error:invalid length; expected 64 bytes"))
			})

			It("has no previous secret", func() {
				tx.Secret = util.RandBytes(64)
				err := validators.CheckTxEpochSecret(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:previousSecret, error:previous secret is required"))
			})

			It("has less than 64 bytes previous secret", func() {
				tx.Secret = util.RandBytes(64)
				tx.PreviousSecret = []byte("invalid length")
				err := validators.CheckTxEpochSecret(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:previousSecret, error:invalid length; expected 64 bytes"))
			})

			It("has no secret round", func() {
				tx.Secret = util.RandBytes(64)
				tx.PreviousSecret = util.RandBytes(64)
				err := validators.CheckTxEpochSecret(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:secretRound, error:secret round is required"))
			})

			It("has no public key", func() {
				tx.Secret = util.RandBytes(64)
				tx.PreviousSecret = util.RandBytes(64)
				tx.SecretRound = 1
				err := validators.CheckTxEpochSecret(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:senderPubKey, error:sender public key is required"))
			})

			It("has no signature", func() {
				tx.Secret = util.RandBytes(64)
				tx.PreviousSecret = util.RandBytes(64)
				tx.SecretRound = 1
				tx.SenderPubKey = util.BytesToBytes32(key.PubKey().MustBytes())
				err := validators.CheckTxEpochSecret(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, error:signature is required"))
			})

			It("has invalid signature", func() {
				tx.Secret = util.RandBytes(64)
				tx.PreviousSecret = util.RandBytes(64)
				tx.SecretRound = 1
				tx.SenderPubKey = util.BytesToBytes32(key.PubKey().MustBytes())
				tx.Sig = []byte("invalid")
				err := validators.CheckTxEpochSecret(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, error:signature is not valid"))
			})
		})

		When("it has no error", func() {
			It("should return no error", func() {
				tx.Secret = util.RandBytes(64)
				tx.PreviousSecret = util.RandBytes(64)
				tx.SecretRound = 1
				tx.SenderPubKey = util.BytesToBytes32(key.PubKey().MustBytes())
				sig, _ := tx.Sign(key.PrivKey().Base58())
				tx.Sig = sig
				err := validators.CheckTxEpochSecret(tx, -1)
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
			It("has invalid type", func() {
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
			It("has invalid type", func() {
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
			It("has invalid type", func() {
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

			It("has a PushOK with a push note id that is different from the PushTx.PushNoteID", func() {
				params.PushOKQuorumSize = 1
				tx.PushOKs = append(tx.PushOKs, &types.PushOK{})
				tx.SenderPubKey = util.BytesToBytes32(key.PubKey().MustBytes())
				sig, _ := key.PrivKey().Sign(tx.Bytes())
				tx.Sig = sig
				err := validators.CheckTxPush(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:endorsements.pushNoteID, error:value does not match push tx note id"))
			})

			It("has a PushOK with an invalid signature", func() {
				params.PushOKQuorumSize = 1
				tx.PushOKs = append(tx.PushOKs, &types.PushOK{
					PushNoteID:   tx.PushNote.ID(),
					SenderPubKey: util.BytesToBytes32(key.PubKey().MustBytes()),
					Sig:          util.BytesToBytes64([]byte("invalid sig")),
				})
				tx.SenderPubKey = util.BytesToBytes32(key.PubKey().MustBytes())
				sig, _ := key.PrivKey().Sign(tx.Bytes())
				tx.Sig = sig
				err := validators.CheckTxPush(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:endorsements.sig, error:signature is invalid"))
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
				Expect(err.Error()).To(Equal("field:endorsements.senderPubKey, error:multiple " +
					"endorsement by a single sender not permitted"))
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
