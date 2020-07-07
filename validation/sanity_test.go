package validation_test

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gitlab.com/makeos/mosdef/remote/push/types"
	"gitlab.com/makeos/mosdef/types/state"
	"gitlab.com/makeos/mosdef/types/txns"

	"github.com/shopspring/decimal"
	"gitlab.com/makeos/mosdef/params"

	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/util"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/testutil"
	"gitlab.com/makeos/mosdef/validation"
)

var _ = Describe("TxValidator", func() {
	var err error
	var cfg *config.AppConfig
	var key = crypto.NewKeyFromIntSeed(1)

	BeforeEach(func() {
		cfg, err = testutil.SetTestCfg()
		Expect(err).To(BeNil())
		params.FeePerByte = decimal.NewFromFloat(0.001)
		params.MinProposalFee = float64(0)
	})

	AfterEach(func() {
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".CheckRecipient", func() {
		When("recipient address is not set", func() {
			It("should return err", func() {
				tx := txns.NewBareTxCoinTransfer()
				err := validation.CheckRecipient(tx.TxRecipient, 0)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("index:0, field:to, msg:recipient address is required"))
			})
		})

		When("recipient address is an invalid base58 encoded address", func() {
			It("should return err", func() {
				tx := txns.NewBareTxCoinTransfer()
				tx.To = "abcdef"
				err := validation.CheckRecipient(tx.TxRecipient, 0)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("index:0, field:to, msg:recipient address is not valid"))
			})
		})

		When("recipient address is not base58 encoded but a namespaced address", func() {
			It("should return no error", func() {
				tx := txns.NewBareTxCoinTransfer()
				tx.To = "namespace/domain"
				err := validation.CheckRecipient(tx.TxRecipient, 0)
				Expect(err).To(BeNil())
			})
		})

		When("recipient address is not base58 encoded but a prefixed address", func() {
			It("should return no error", func() {
				tx := txns.NewBareTxCoinTransfer()
				tx.To = "r/domain"
				err := validation.CheckRecipient(tx.TxRecipient, 0)
				Expect(err).To(BeNil())
			})
		})

		When("recipient address is not base58 encoded but a prefixed account address", func() {
			It("should return err", func() {
				tx := txns.NewBareTxCoinTransfer()
				tx.To = "a/abcdef"
				err := validation.CheckRecipient(tx.TxRecipient, 0)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("index:0, field:to, msg:recipient address is not valid"))
			})
		})

		When("recipient address is a base58 encoded address that is valid", func() {
			It("should return no error", func() {
				tx := txns.NewBareTxCoinTransfer()
				tx.To = key.Addr()
				err := validation.CheckRecipient(tx.TxRecipient, 0)
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".CheckTxCoinTransfer", func() {
		var tx *txns.TxCoinTransfer
		BeforeEach(func() {
			tx = txns.NewBareTxCoinTransfer()
			tx.To = key.Addr()
			tx.Fee = "1"
		})

		When("it has invalid fields, it should return error when", func() {
			It("should return error='type is invalid'", func() {
				tx.Type = -10
				err := validation.CheckTxCoinTransfer(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:type, msg:type is invalid"))
			})

			It("has no recipient address", func() {
				tx.To = ""
				err := validation.CheckTxCoinTransfer(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:to, msg:recipient address is required"))
			})

			It("has invalid recipient address", func() {
				tx.To = "invalid"
				err := validation.CheckTxCoinTransfer(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:to, msg:recipient address is not valid"))
			})

			It("has invalid value", func() {
				tx.Value = "invalid"
				err := validation.CheckTxCoinTransfer(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:value, msg:invalid number; must be numeric"))
			})

			It("has no nonce", func() {
				err := validation.CheckTxCoinTransfer(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:nonce, msg:nonce is required"))
			})

			It("has invalid fee", func() {
				tx.Nonce = 1
				tx.Fee = "invalid"
				err := validation.CheckTxCoinTransfer(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:fee, msg:invalid number; must be numeric"))
			})

			It("has low fee", func() {
				tx.Nonce = 1
				tx.Fee = "0"
				err := validation.CheckTxCoinTransfer(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("field:fee, msg:fee cannot be lower than the base price"))
			})

			It("has no timestamp", func() {
				tx.Nonce = 1
				err := validation.CheckTxCoinTransfer(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:timestamp, msg:timestamp is required"))
			})

			It("has no public key", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				err := validation.CheckTxCoinTransfer(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:senderPubKey, msg:sender public key is required"))
			})

			It("has no signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				err := validation.CheckTxCoinTransfer(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, msg:signature is required"))
			})

			It("has invalid signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				tx.Sig = []byte("invalid")
				err := validation.CheckTxCoinTransfer(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, msg:signature is not valid"))
			})
		})

		When("it has no error", func() {
			It("should return no error", func() {
				tx.To = key.Addr()
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				sig, err := tx.Sign(key.PrivKey().Base58())
				Expect(err).To(BeNil())
				tx.Sig = sig
				err = validation.CheckTxCoinTransfer(tx, -1)
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".CheckTxNamespaceAcquire", func() {
		var tx *txns.TxNamespaceRegister
		BeforeEach(func() {
			params.CostOfNamespace = decimal.NewFromFloat(5)
			tx = txns.NewBareTxNamespaceRegister()
			tx.Fee = "1"
			tx.Name = "namespace"
			tx.Value = util.String(params.CostOfNamespace.String())
			tx.Nonce = 1
		})

		When("it has invalid fields, it should return error when", func() {
			It("should return error='type is invalid'", func() {
				tx.Type = -10
				err := validation.CheckTxNamespaceAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:type, msg:type is invalid"))
			})

			It("has invalid value", func() {
				tx.Value = "invalid"
				err := validation.CheckTxNamespaceAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:value, msg:invalid number; must be numeric"))
			})

			It("has no name", func() {
				tx.Name = ""
				err := validation.CheckTxNamespaceAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:name, msg:requires a unique name"))
			})

			It("has an invalid name", func() {
				tx.Name = "invalid&"
				err := validation.CheckTxNamespaceAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:name, msg:invalid identifier; only alphanumeric, _, and - characters are allowed"))
			})

			It("has invalid transfer destination", func() {
				tx.TransferTo = "re&&^po"
				err := validation.CheckTxNamespaceAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:to, msg:invalid value. Expected an address or a repository name"))
			})

			It("has value not equal to namespace price", func() {
				tx.Value = "1"
				err := validation.CheckTxNamespaceAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:value, msg:invalid value; has 1, want 5"))
			})

			It("has domain target with invalid format", func() {
				tx.Value = "5"
				tx.Domains["domain"] = "invalid:format"
				err := validation.CheckTxNamespaceAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:domains, msg:domains.domain: target is invalid"))
			})

			It("has domain target with unknown target type", func() {
				tx.Value = "5"
				tx.Domains["domain"] = "unknown_type/name"
				err := validation.CheckTxNamespaceAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:domains, msg:domains.domain: target is invalid"))
			})

			It("has domain target with account target type that has an invalid address", func() {
				tx.Value = "5"
				tx.Domains["domain"] = "a/invalid_addr"
				err := validation.CheckTxNamespaceAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:domains, msg:domains.domain: target is not a valid address"))
			})

			It("has invalid fee", func() {
				tx.Nonce = 1
				tx.Fee = "invalid"
				err := validation.CheckTxNamespaceAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:fee, msg:invalid number; must be numeric"))
			})

			It("has low fee", func() {
				tx.Nonce = 1
				tx.Fee = "0"
				err := validation.CheckTxNamespaceAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("field:fee, msg:fee cannot be lower than the base price"))
			})

			It("has no nonce", func() {
				tx.Nonce = 0
				err := validation.CheckTxNamespaceAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:nonce, msg:nonce is required"))
			})

			It("has no timestamp", func() {
				tx.Nonce = 1
				err := validation.CheckTxNamespaceAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:timestamp, msg:timestamp is required"))
			})

			It("has no public key", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				err := validation.CheckTxNamespaceAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:senderPubKey, msg:sender public key is required"))
			})

			It("has no signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				err := validation.CheckTxNamespaceAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, msg:signature is required"))
			})

			It("has invalid signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				tx.Sig = []byte("invalid")
				err := validation.CheckTxNamespaceAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, msg:signature is not valid"))
			})
		})

		When("it has no error", func() {
			It("should return no error", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				tx.Domains["domain"] = "r/repo1"
				sig, err := tx.Sign(key.PrivKey().Base58())
				Expect(err).To(BeNil())
				tx.Sig = sig
				err = validation.CheckTxNamespaceAcquire(tx, -1)
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".checkNamespaceDomains", func() {
		When("map include a domain that is not valid", func() {
			It("should return err", func() {
				domains := map[string]string{"goo&": "abc"}
				err := validation.CheckNamespaceDomains(domains, 0)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("index:0, field:domains, msg:domains.goo&: name is invalid"))
			})
		})

		When("map include a domain with a target whose name is not valid", func() {
			It("should return err", func() {
				domains := map[string]string{"google": "xyz"}
				err := validation.CheckNamespaceDomains(domains, 0)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("index:0, field:domains, msg:domains.google: target is invalid"))
			})
		})

		When("map include a domain with an address target that has an invalid address", func() {
			It("should return err", func() {
				domains := map[string]string{"google": "a/xyz"}
				err := validation.CheckNamespaceDomains(domains, 0)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("index:0, field:domains, msg:domains.google: target is not a valid address"))
			})
		})
	})

	Describe(".CheckTxTicketPurchase", func() {
		var tx *txns.TxTicketPurchase
		BeforeEach(func() {
			tx = txns.NewBareTxTicketPurchase(txns.TxTypeValidatorTicket)
			tx.Fee = "1"
			tx.Value = "1"
		})

		When("it has invalid fields, it should return error when", func() {
			It("should return error='type is invalid'", func() {
				tx.Type = -10
				err := validation.CheckTxTicketPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:type, msg:type is invalid"))
			})

			It("has invalid value", func() {
				tx.Value = "invalid"
				err := validation.CheckTxTicketPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:value, msg:invalid number; must be numeric"))
			})

			It("has type of TxTypeHostTicket and value is lower than minimum stake", func() {
				params.MinHostStake = decimal.NewFromFloat(20)
				tx.Type = txns.TxTypeHostTicket
				tx.Value = "10"
				err := validation.CheckTxTicketPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:value, msg:value is lower than minimum host stake"))
			})

			It("has negative or zero value", func() {
				tx.Value = "0"
				err := validation.CheckTxTicketPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:value, msg:value must be a positive number"))
			})

			It("has no nonce", func() {
				err := validation.CheckTxTicketPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:nonce, msg:nonce is required"))
			})

			It("has invalid fee", func() {
				tx.Nonce = 1
				tx.Fee = "invalid"
				err := validation.CheckTxTicketPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:fee, msg:invalid number; must be numeric"))
			})

			It("has no timestamp", func() {
				tx.Nonce = 1
				err := validation.CheckTxTicketPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:timestamp, msg:timestamp is required"))
			})

			It("has no public key", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				err := validation.CheckTxTicketPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:senderPubKey, msg:sender public key is required"))
			})

			It("has no signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				err := validation.CheckTxTicketPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, msg:signature is required"))
			})

			It("has invalid signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				tx.Sig = []byte("invalid")
				err := validation.CheckTxTicketPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, msg:signature is not valid"))
			})

			It("has type of TxTypeHostTicket and BLS public key is unset", func() {
				params.MinHostStake = decimal.NewFromFloat(5)
				tx.Value = "10"
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.Type = txns.TxTypeHostTicket
				err := validation.CheckTxTicketPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:blsPubKey, msg:BLS public key is required"))
			})

			It("has type of TxTypeHostTicket and BLS public key has invalid length", func() {
				params.MinHostStake = decimal.NewFromFloat(5)
				tx.Value = "10"
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.Type = txns.TxTypeHostTicket
				tx.BLSPubKey = util.RandBytes(32)
				err := validation.CheckTxTicketPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:blsPubKey, msg:BLS public key length is invalid"))
			})
		})

		When("it has no error", func() {
			It("should return no error", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				sig, err := tx.Sign(key.PrivKey().Base58())
				Expect(err).To(BeNil())
				tx.Sig = sig
				err = validation.CheckTxTicketPurchase(tx, -1)
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".CheckTxUnbondTicket", func() {
		var tx *txns.TxTicketUnbond

		BeforeEach(func() {
			tx = txns.NewBareTxTicketUnbond(txns.TxTypeHostTicket)
			tx.TicketHash = util.StrToHexBytes("hash")
			tx.Fee = "1"
		})

		When("it has invalid fields, it should return error when", func() {
			It("should return error='type is invalid'", func() {
				tx.Type = -10
				err := validation.CheckTxUnbondTicket(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:type, msg:type is invalid"))
			})

			It("has no ticket hash", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.TicketHash = []byte{}
				err := validation.CheckTxUnbondTicket(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:ticket, msg:ticket id is required"))
			})

			It("has no nonce", func() {
				err := validation.CheckTxUnbondTicket(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:nonce, msg:nonce is required"))
			})

			It("has invalid fee", func() {
				tx.Nonce = 1
				tx.Fee = "invalid"
				err := validation.CheckTxUnbondTicket(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:fee, msg:invalid number; must be numeric"))
			})

			It("has no timestamp", func() {
				tx.Nonce = 1
				err := validation.CheckTxUnbondTicket(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:timestamp, msg:timestamp is required"))
			})

			It("has no public key", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				err := validation.CheckTxUnbondTicket(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:senderPubKey, msg:sender public key is required"))
			})

			It("has no signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				err := validation.CheckTxUnbondTicket(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, msg:signature is required"))
			})

			It("has invalid signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				tx.Sig = []byte("invalid")
				err := validation.CheckTxUnbondTicket(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, msg:signature is not valid"))
			})
		})

		When("it has no error", func() {
			It("should return no error", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				sig, err := tx.Sign(key.PrivKey().Base58())
				Expect(err).To(BeNil())
				tx.Sig = sig
				err = validation.CheckTxUnbondTicket(tx, -1)
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".CheckRepoConfig", func() {
		var cases = []map[string]interface{}{
			{
				"desc": "unexpected governance.propVoter field type",
				"err":  "dry merge failed: cannot append two different types (string, float64)",
				"data": map[string]interface{}{"governance": map[string]interface{}{"propVoter": "1"}},
			},
			{
				"desc": "invalid governance.propVoter value",
				"err":  "field:governance.propVoter, msg:unknown value",
				"data": map[string]interface{}{"governance": map[string]interface{}{"propVoter": 100}},
			},
			{
				"desc": "invalid governance.propCreator value",
				"err":  "field:governance.propCreator, msg:unknown value",
				"data": map[string]interface{}{"governance": map[string]interface{}{
					"propVoter":   state.VoterOwner,
					"propCreator": 1000,
				}},
			},
			{
				"desc": "invalid governance.propTallyMethod value",
				"err":  "field:governance.propTallyMethod, msg:unknown value",
				"data": map[string]interface{}{"governance": map[string]interface{}{
					"propVoter":       state.VoterOwner,
					"propCreator":     state.ProposalCreatorAny,
					"propTallyMethod": 1000,
				}},
			},
			{
				"desc": "proposal quorum has negative value",
				"err":  "field:governance.propQuorum, msg:must be a non-negative number",
				"data": map[string]interface{}{"governance": map[string]interface{}{
					"propVoter":       state.VoterOwner,
					"propCreator":     state.ProposalCreatorAny,
					"propTallyMethod": state.ProposalTallyMethodNetStake,
					"propQuorum":      -1,
				}},
			},
			{
				"desc": "proposal threshold has negative value",
				"err":  "field:governance.propThreshold, msg:must be a non-negative number",
				"data": map[string]interface{}{"governance": map[string]interface{}{
					"propVoter":       state.VoterOwner,
					"propCreator":     state.ProposalCreatorAny,
					"propTallyMethod": state.ProposalTallyMethodNetStake,
					"propThreshold":   -1,
				}},
			},
			{
				"desc": "proposal veto quorum has negative value",
				"err":  "field:governance.propVetoQuorum, msg:must be a non-negative number",
				"data": map[string]interface{}{"governance": map[string]interface{}{
					"propVoter":       state.VoterOwner,
					"propCreator":     state.ProposalCreatorAny,
					"propTallyMethod": state.ProposalTallyMethodNetStake,
					"propVetoQuorum":  -1,
				}},
			},
			{
				"desc": "proposal veto owners quorum has negative value",
				"err":  "field:governance.propVetoOwnersQuorum, msg:must be a non-negative number",
				"data": map[string]interface{}{"governance": map[string]interface{}{
					"propVoter":            state.VoterOwner,
					"propCreator":          state.ProposalCreatorAny,
					"propTallyMethod":      state.ProposalTallyMethodNetStake,
					"propVetoOwnersQuorum": -1,
				}},
			},
			{
				"desc": "proposal fee is below minimum value",
				"err":  "field:governance.propFee, msg:cannot be lower than network minimum",
				"before": func() {
					params.MinProposalFee = float64(400)
				},
				"data": map[string]interface{}{"governance": map[string]interface{}{
					"propVoter":       state.VoterOwner,
					"propCreator":     state.ProposalCreatorAny,
					"propTallyMethod": state.ProposalTallyMethodNetStake,
					"propFee":         100,
				}},
			},
			{
				"desc": "when voter type is not ProposerOwner and tally method is CoinWeighted",
				"err":  "field:config, msg:when proposer type is not 'ProposerOwner', tally methods 'CoinWeighted' and 'Identity' are not allowed",
				"data": map[string]interface{}{"governance": map[string]interface{}{
					"propVoter":       state.VoterNetStakers,
					"propTallyMethod": state.ProposalTallyMethodCoinWeighted,
				}},
			},
			{
				"desc": "when voter is not ProposerOwner and tally method is Identity",
				"err":  "field:config, msg:when proposer type is not 'ProposerOwner', tally methods 'CoinWeighted' and 'Identity' are not allowed",
				"data": map[string]interface{}{"governance": map[string]interface{}{
					"propVoter":       state.VoterNetStakers,
					"propTallyMethod": state.ProposalTallyMethodIdentity,
				}},
			},
		}

		for index, c := range cases {
			i := index
			cur := c
			It(fmt.Sprintf("case %d: ", i)+cur["desc"].(string), func() {
				if before, ok := cur["before"]; ok {
					before.(func())()
				}
				err := validation.CheckRepoConfig(cur["data"].(map[string]interface{}), -1)
				if cur["err"].(string) == "" {
					Expect(err).To(BeNil())
				} else {
					Expect(err).ToNot(BeNil())
					Expect(err.Error()).To(Equal(cur["err"].(string)))
				}
			})
		}
	})

	Describe(".CheckTxRepoCreate", func() {
		var tx *txns.TxRepoCreate
		BeforeEach(func() {
			tx = txns.NewBareTxRepoCreate()
			tx.Name = "repo"
			tx.Fee = "1"
		})

		When("it has invalid fields, it should return error when", func() {
			It("should return error='type is invalid'", func() {
				tx.Type = -10
				err := validation.CheckTxRepoCreate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:type, msg:type is invalid"))
			})

			It("has invalid value", func() {
				tx.Value = "invalid"
				err := validation.CheckTxRepoCreate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:value, msg:invalid number; must be numeric"))
			})

			It("has no name", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.Name = ""
				err := validation.CheckTxRepoCreate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:name, msg:requires a unique name"))
			})

			It("has invalid name", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.Name = "org&name#"
				err := validation.CheckTxRepoCreate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:name, msg:invalid identifier; only alphanumeric, _, and - characters are allowed"))
			})

			It("has invalid repo config (propVoter)", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.Name = "repo1"
				tx.Config["governance"] = map[string]interface{}{
					"propVoter": -1,
				}
				err := validation.CheckTxRepoCreate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:governance.propVoter, msg:unknown value"))
			})

			It("has no nonce", func() {
				err := validation.CheckTxRepoCreate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:nonce, msg:nonce is required"))
			})

			It("has invalid fee", func() {
				tx.Nonce = 1
				tx.Fee = "invalid"
				err := validation.CheckTxRepoCreate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:fee, msg:invalid number; must be numeric"))
			})

			It("has no timestamp", func() {
				tx.Nonce = 1
				err := validation.CheckTxRepoCreate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:timestamp, msg:timestamp is required"))
			})

			It("has no public key", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				err := validation.CheckTxRepoCreate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:senderPubKey, msg:sender public key is required"))
			})

			It("has no signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				err := validation.CheckTxRepoCreate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, msg:signature is required"))
			})

			It("has invalid signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				tx.Sig = []byte("invalid")
				err := validation.CheckTxRepoCreate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, msg:signature is not valid"))
			})
		})

		When("it has no error", func() {
			It("should return no error", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				sig, err := tx.Sign(key.PrivKey().Base58())
				Expect(err).To(BeNil())
				tx.Sig = sig
				err = validation.CheckTxRepoCreate(tx, -1)
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".CheckTxRegisterPushKey", func() {
		var tx *txns.TxRegisterPushKey

		BeforeEach(func() {
			pushKey, err := crypto.NewKey(nil)
			Expect(err).To(BeNil())
			tx = txns.NewBareTxRegister()
			tx.PublicKey = crypto.BytesToPublicKey(pushKey.PubKey().MustBytes())
			tx.Fee = "2"
		})

		When("it has invalid fields, it should return error when", func() {
			It("should return error='type is invalid'", func() {
				tx.Type = -10
				err := validation.CheckTxRegisterPushKey(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:type, msg:type is invalid"))
			})

			It("has no public key", func() {
				tx.PublicKey = crypto.EmptyPublicKey
				err := validation.CheckTxRegisterPushKey(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:pubKey, msg:public key is required"))
			})

			It("has invalid scopes", func() {
				scopes := []string{
					"maker13463exprf3fdq44eth4lkf99dy6z5ajuk4ln4z",
					"a/maker13463exprf3fdq44eth4lkf99dy6z5ajuk4ln4z",
					"repo_&*",
				}
				for _, s := range scopes {
					tx.Scopes = []string{s}
					err := validation.CheckTxRegisterPushKey(tx, -1)
					Expect(err).ToNot(BeNil())
					Expect(err).To(MatchError("field:scopes[0], msg:not an acceptable scope. " +
						"Expects a namespace URI or repository name"))
				}
			})

			It("has invalid fee cap", func() {
				tx.FeeCap = "1a"
				err := validation.CheckTxRegisterPushKey(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:feeCap, msg:invalid number; must be numeric"))
			})

			It("has no nonce", func() {
				err := validation.CheckTxRegisterPushKey(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:nonce, msg:nonce is required"))
			})

			It("has invalid fee", func() {
				tx.Nonce = 1
				tx.Fee = "invalid"
				err := validation.CheckTxRegisterPushKey(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:fee, msg:invalid number; must be numeric"))
			})

			It("has no timestamp", func() {
				tx.Nonce = 1
				err := validation.CheckTxRegisterPushKey(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:timestamp, msg:timestamp is required"))
			})

			It("has no public key", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				err := validation.CheckTxRegisterPushKey(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:senderPubKey, msg:sender public key is required"))
			})

			It("has no signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				err := validation.CheckTxRegisterPushKey(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, msg:signature is required"))
			})

			It("has invalid signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				tx.Sig = []byte("invalid")
				err := validation.CheckTxRegisterPushKey(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, msg:signature is not valid"))
			})
		})

		When("it has no error", func() {
			It("should return no error", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				sig, err := tx.Sign(key.PrivKey().Base58())
				Expect(err).To(BeNil())
				tx.Sig = sig
				err = validation.CheckTxRegisterPushKey(tx, -1)
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".CheckTxUpDelPushKey", func() {
		var tx *txns.TxUpDelPushKey

		BeforeEach(func() {
			tx = txns.NewBareTxUpDelPushKey()
			tx.Fee = "2"
			tx.ID = "push1wfx7vp8qfyv98cctvamqwec5xjrj48tpxaa77t"
		})

		When("it has invalid fields, it should return error when", func() {
			It("should return error='type is invalid'", func() {
				tx.Type = -10
				err := validation.CheckTxUpDelPushKey(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:type, msg:type is invalid"))
			})

			It("has no id", func() {
				tx.ID = ""
				err := validation.CheckTxUpDelPushKey(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:id, msg:push key id is required"))
			})

			It("has invalid id", func() {
				tx.ID = "push_abc_invalid"
				err := validation.CheckTxUpDelPushKey(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:id, msg:push key id is not valid"))
			})

			It("has invalid entry in addScopes", func() {
				tx.AddScopes = []string{"inv*alid"}
				err := validation.CheckTxUpDelPushKey(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:scopes[0], msg:not an acceptable scope. Expects a namespace URI or repository name"))
			})

			It("has invalid entry in addScopes", func() {
				tx.AddScopes = []string{"inv*alid"}
				err := validation.CheckTxUpDelPushKey(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:scopes[0], msg:not an acceptable scope. Expects a namespace URI or repository name"))
			})

			It("has invalid fee cap", func() {
				tx.FeeCap = "1a"
				err := validation.CheckTxUpDelPushKey(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:feeCap, msg:invalid number; must be numeric"))
			})
		})

		When("it has no error", func() {
			It("should return no error", func() {
				tx.Nonce = 1
				tx.AddScopes = []string{"repo1"}
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				sig, err := tx.Sign(key.PrivKey().Base58())
				Expect(err).To(BeNil())
				tx.Sig = sig
				err = validation.CheckTxUpDelPushKey(tx, -1)
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".CheckTxSetDelegateCommission", func() {
		var tx *txns.TxSetDelegateCommission

		BeforeEach(func() {
			tx = txns.NewBareTxSetDelegateCommission()
			tx.Commission = "60"
			tx.Fee = "1"
		})

		When("it has invalid fields, it should return error when", func() {
			It("should return error='type is invalid'", func() {
				tx.Type = -10
				err := validation.CheckTxSetDelegateCommission(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:type, msg:type is invalid"))
			})

			It("has no commission value", func() {
				tx.Commission = ""
				err := validation.CheckTxSetDelegateCommission(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:commission, msg:commission rate is required"))
			})

			It("has no commission value is below minimum", func() {
				params.MinDelegatorCommission = decimal.NewFromFloat(50)
				tx.Commission = "49"
				err := validation.CheckTxSetDelegateCommission(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:commission, msg:rate cannot be below the minimum (50%)"))
			})

			It("has no commission value is above 100", func() {
				params.MinDelegatorCommission = decimal.NewFromFloat(50)
				tx.Commission = "101"
				err := validation.CheckTxSetDelegateCommission(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:commission, msg:commission rate cannot exceed 100%"))
			})

			It("has no nonce", func() {
				err := validation.CheckTxSetDelegateCommission(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:nonce, msg:nonce is required"))
			})

			It("has invalid fee", func() {
				tx.Nonce = 1
				tx.Fee = "invalid"
				err := validation.CheckTxSetDelegateCommission(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:fee, msg:invalid number; must be numeric"))
			})

			It("has no timestamp", func() {
				tx.Nonce = 1
				err := validation.CheckTxSetDelegateCommission(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:timestamp, msg:timestamp is required"))
			})

			It("has no public key", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				err := validation.CheckTxSetDelegateCommission(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:senderPubKey, msg:sender public key is required"))
			})

			It("has no signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				err := validation.CheckTxSetDelegateCommission(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, msg:signature is required"))
			})

			It("has invalid signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				tx.Sig = []byte("invalid")
				err := validation.CheckTxSetDelegateCommission(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, msg:signature is not valid"))
			})
		})

		When("it has no error", func() {
			It("should return no error", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
				sig, err := tx.Sign(key.PrivKey().Base58())
				Expect(err).To(BeNil())
				tx.Sig = sig
				err = validation.CheckTxSetDelegateCommission(tx, -1)
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".CheckTxNamespaceDomainUpdate", func() {
		var tx *txns.TxNamespaceDomainUpdate

		BeforeEach(func() {
			tx = txns.NewBareTxNamespaceDomainUpdate()
			tx.Fee = "1"
		})

		When("it has invalid fields, it should return error when", func() {
			It("should return error='type is invalid'", func() {
				tx.Type = -10
				err := validation.CheckTxNamespaceDomainUpdate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:type, msg:type is invalid"))
			})
		})

		When("name is not set", func() {
			It("should return err", func() {
				err := validation.CheckTxNamespaceDomainUpdate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:name, msg:requires a name"))
			})
		})

		When("name is not valid", func() {
			It("should return err", func() {
				tx.Name = "&name"
				err := validation.CheckTxNamespaceDomainUpdate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:name, msg:invalid identifier; only alphanumeric, _, and - characters are allowed"))
			})
		})

		When("name is too short", func() {
			It("should return err", func() {
				tx.Name = "ab"
				err := validation.CheckTxNamespaceDomainUpdate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:name, msg:name is too short. Must be at least 3 characters long"))
			})
		})

		When("a domain is not valid", func() {
			It("should return err", func() {
				tx.Name = "name1"
				tx.Domains = map[string]string{"domain": "invalid-target"}
				err := validation.CheckTxNamespaceDomainUpdate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:domains, msg:domains.domain: target is invalid"))
			})
		})
	})

	Describe(".CheckTxPush", func() {
		var tx *txns.TxPush

		BeforeEach(func() {
			tx = txns.NewBareTxPush()
			tx.Timestamp = time.Now().Unix()
			tx.Note.(*types.Note).RepoName = "repo1"
			tx.Note.(*types.Note).PushKeyID = util.RandBytes(20)
			tx.Note.(*types.Note).Timestamp = time.Now().Unix()
			tx.Note.(*types.Note).PusherAcctNonce = 1
			tx.Note.(*types.Note).CreatorPubKey = key.PubKey().MustBytes32()
			tx.Note.(*types.Note).RemoteNodeSig = key.PrivKey().MustSign(tx.Note.Bytes())
		})

		It("should return error when type is invalid", func() {
			tx.Type = -10
			err := validation.CheckTxPush(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("field:type, msg:type is invalid"))
		})

		It("should return error when it has no push note", func() {
			tx.Note = nil
			err := validation.CheckTxPush(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("field:note, msg:push note is required"))
		})

		It("should return error when it has an invalid push note (with no repo name)", func() {
			tx.Note.(*types.Note).RepoName = ""
			err := validation.CheckTxPush(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("field:repo, msg:repo name is required"))
		})

		It("should return error when it has low endorsement (not up to quorum)", func() {
			err := validation.CheckTxPush(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("field:endorsements, msg:not enough endorsements included"))
		})

		It("should return error when it has a push endorsement with no sender public key", func() {
			params.PushEndorseQuorumSize = 1
			tx.Endorsements = append(tx.Endorsements, &types.PushEndorsement{
				EndorserPubKey: util.EmptyBytes32,
			})
			tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
			sig, _ := key.PrivKey().Sign(tx.Bytes())
			tx.Sig = sig
			err := validation.CheckTxPush(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("index:0, field:endorsements.pubKey, msg:endorser's public key is required"))
		})

		It("should return error when it has multiple push endorsements from same sender", func() {
			params.PushEndorseQuorumSize = 1

			end1 := &types.PushEndorsement{
				EndorserPubKey: util.BytesToBytes32(key.PubKey().MustBytes()),
				References:     []*types.EndorsedReference{{}},
			}
			tx.Endorsements = append(tx.Endorsements, end1)

			end2 := &types.PushEndorsement{
				EndorserPubKey: util.BytesToBytes32(key.PubKey().MustBytes()),
				References:     []*types.EndorsedReference{},
			}
			tx.Endorsements = append(tx.Endorsements, end2)

			tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
			sig, _ := key.PrivKey().Sign(tx.Bytes())
			tx.Sig = sig
			err := validation.CheckTxPush(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("index:1, field:endorsements.pubKey, " +
				"msg:multiple endorsement by a single sender not permitted"))
		})

		It("should return no error when endorsement is valid", func() {
			params.PushEndorseQuorumSize = 1

			end := &types.PushEndorsement{
				EndorserPubKey: util.BytesToBytes32(key.PubKey().MustBytes()),
				References:     []*types.EndorsedReference{{}},
			}
			tx.Endorsements = append(tx.Endorsements, end)

			tx.SenderPubKey = crypto.BytesToPublicKey(key.PubKey().MustBytes())
			sig, _ := key.PrivKey().Sign(tx.Bytes())
			tx.Sig = sig

			err := validation.CheckTxPush(tx, -1)
			Expect(err).To(BeNil())
		})
	})

	Describe(".CheckTxRepoProposalUpsertOwner", func() {
		var tx *txns.TxRepoProposalUpsertOwner

		BeforeEach(func() {
			params.MinProposalFee = 10
			tx = txns.NewBareRepoProposalUpsertOwner()
			tx.Timestamp = time.Now().Unix()
			tx.Value = "11"
			tx.ID = "123"
		})

		It("should return error when repo name is not provided", func() {
			err := validation.CheckTxRepoProposalUpsertOwner(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:name, msg:repo name is required"))
		})

		It("should return error when repo name is not valid", func() {
			tx.RepoName = "*&^"
			err := validation.CheckTxRepoProposalUpsertOwner(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:name, msg:invalid identifier; only alphanumeric, _, and - characters are allowed"))
		})

		It("should return error when proposal id is unset", func() {
			tx.RepoName = "good-repo"
			tx.ID = ""
			err := validation.CheckTxRepoProposalUpsertOwner(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:id, msg:proposal id is required"))
		})

		It("should return error when proposal id is not valid", func() {
			tx.RepoName = "good-repo"
			tx.ID = "abc123"
			err := validation.CheckTxRepoProposalUpsertOwner(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:id, msg:proposal id is not valid"))
		})

		It("should return error when proposal id length exceeds max", func() {
			tx.RepoName = "good-repo"
			tx.ID = "123456789"
			err := validation.CheckTxRepoProposalUpsertOwner(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:id, msg:proposal id limit of 8 bytes exceeded"))
		})

		It("should return error when value is not provided", func() {
			tx.RepoName = "good-repo"
			tx.Value = ""
			err := validation.CheckTxRepoProposalUpsertOwner(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:value, msg:value is required"))
		})

		It("should return error when value below minimum network proposal fee", func() {
			params.MinProposalFee = 100
			tx.RepoName = "good-repo"
			tx.Value = "1"
			err := validation.CheckTxRepoProposalUpsertOwner(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:value, msg:proposal creation fee cannot be less than network minimum"))
		})

		It("should return error when target address is not provided", func() {
			tx.RepoName = "repo1"
			err := validation.CheckTxRepoProposalUpsertOwner(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:addresses, msg:at least one address is required"))
		})

		It("should return error when target addresses exceed maximum", func() {
			tx.RepoName = "repo1"
			addresses := strings.TrimRight(strings.Repeat("addr1,", 11), ",")
			tx.Addresses = strings.Split(addresses, ",")
			err := validation.CheckTxRepoProposalUpsertOwner(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:addresses, msg:only a maximum of 10 addresses are allowed"))
		})

		It("should return error when target address is not valid", func() {
			tx.RepoName = "repo1"
			tx.Addresses = []string{"invalid_addr"}
			err := validation.CheckTxRepoProposalUpsertOwner(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:addresses[0], msg:address is not valid"))
		})
	})

	Describe(".CheckTxVote", func() {
		var tx *txns.TxRepoProposalVote

		BeforeEach(func() {
			tx = txns.NewBareRepoProposalVote()
			tx.Timestamp = time.Now().Unix()
		})

		It("should return error when repo name is not provided", func() {
			err := validation.CheckTxVote(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:name, msg:repo name is required"))
		})

		It("should return error when repo name is not valid", func() {
			tx.RepoName = "*&^"
			err := validation.CheckTxVote(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:name, msg:invalid identifier; only alphanumeric, _, and - characters are allowed"))
		})

		It("should return error when proposal id is not provided", func() {
			tx.RepoName = "repo1"
			err := validation.CheckTxVote(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:id, msg:proposal id is required"))
		})

		It("should return error when proposal id is not numerical", func() {
			tx.RepoName = "repo1"
			tx.ProposalID = "abc"
			err := validation.CheckTxVote(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:id, msg:proposal id is not valid"))
		})

		It("should return error when vote choice is not between -2 and 1 (inclusive)", func() {
			tx.RepoName = "repo1"
			tx.ProposalID = "1"
			tx.Vote = 2
			err := validation.CheckTxVote(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:vote, msg:vote choice is unknown"))

			tx.Vote = -3
			err = validation.CheckTxVote(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:vote, msg:vote choice is unknown"))

			tx.Vote = -1
			err = validation.CheckTxVote(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).ToNot(MatchError("field:vote, msg:vote choice is unknown"))
		})
	})

	Describe(".CheckTxRepoProposalSendFee", func() {
		var tx *txns.TxRepoProposalSendFee

		BeforeEach(func() {
			tx = txns.NewBareRepoProposalFeeSend()
			tx.Timestamp = time.Now().Unix()
		})

		It("should return error when repo name is not provided", func() {
			err := validation.CheckTxRepoProposalSendFee(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:name, msg:repo name is required"))
		})

		It("should return error when repo name is not valid", func() {
			tx.RepoName = "*&^"
			err := validation.CheckTxRepoProposalSendFee(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:name, msg:invalid identifier; only alphanumeric, _, and - characters are allowed"))
		})

		It("should return error when proposal id is not provided", func() {
			tx.RepoName = "repo1"
			err := validation.CheckTxRepoProposalSendFee(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:id, msg:proposal id is required"))
		})

		It("should return error when proposal id is not numerical", func() {
			tx.RepoName = "repo1"
			tx.ID = "abc"
			err := validation.CheckTxRepoProposalSendFee(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:id, msg:proposal id is not valid"))
		})

		It("should return error when proposal id exceeds max length", func() {
			tx.RepoName = "repo1"
			tx.ID = "1234556789"
			err := validation.CheckTxRepoProposalSendFee(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:id, msg:proposal id limit of 8 bytes exceeded"))
		})

		It("should return error when value is not provided", func() {
			tx.RepoName = "good-repo"
			tx.Value = ""
			tx.ID = "1"
			err := validation.CheckTxRepoProposalSendFee(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:value, msg:value is required"))
		})
	})

	Describe(".CheckTxRepoProposalUpdate", func() {
		var tx *txns.TxRepoProposalUpdate

		BeforeEach(func() {
			tx = txns.NewBareRepoProposalUpdate()
			tx.Timestamp = time.Now().Unix()
			tx.ID = "123"
		})

		It("should return error when repo name is not provided", func() {
			err := validation.CheckTxRepoProposalUpdate(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:name, msg:repo name is required"))
		})

		It("should return error when repo name is not valid", func() {
			tx.RepoName = "*&^"
			err := validation.CheckTxRepoProposalUpdate(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:name, msg:invalid identifier; only alphanumeric, _, and - characters are allowed"))
		})

		It("should return error when proposal id is unset", func() {
			tx.RepoName = "good-repo"
			tx.ID = ""
			err := validation.CheckTxRepoProposalUpdate(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:id, msg:proposal id is required"))
		})

		It("should return error when proposal id is not valid", func() {
			tx.RepoName = "good-repo"
			tx.ID = "abc123"
			err := validation.CheckTxRepoProposalUpdate(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:id, msg:proposal id is not valid"))
		})

		It("should return error when proposal id length exceeds max", func() {
			tx.RepoName = "good-repo"
			tx.ID = "123456789"
			err := validation.CheckTxRepoProposalUpdate(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:id, msg:proposal id limit of 8 bytes exceeded"))
		})

		It("should return error when value is not provided", func() {
			tx.RepoName = "good-repo"
			tx.Value = ""
			err := validation.CheckTxRepoProposalUpdate(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:value, msg:value is required"))
		})

		It("should return error when value below minimum network proposal fee", func() {
			params.MinProposalFee = 100
			tx.RepoName = "good-repo"
			tx.Value = "1"
			err := validation.CheckTxRepoProposalUpdate(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:value, msg:proposal creation fee cannot be less than network minimum"))
		})
	})

	Describe(".CheckTxRepoProposalRegisterPushKey", func() {
		var tx *txns.TxRepoProposalRegisterPushKey

		BeforeEach(func() {
			tx = txns.NewBareRepoProposalRegisterPushKey()
			tx.Timestamp = time.Now().Unix()
			tx.ID = "123"
		})

		It("should return error='type is invalid'", func() {
			tx.Type = -10
			err := validation.CheckTxRepoProposalRegisterPushKey(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("field:type, msg:type is invalid"))
		})

		It("should return error when repo name is not provided", func() {
			err := validation.CheckTxRepoProposalRegisterPushKey(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:name, msg:repo name is required"))
		})

		It("should return error when repo name is not valid", func() {
			tx.RepoName = "*&^"
			err := validation.CheckTxRepoProposalRegisterPushKey(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:name, msg:invalid identifier; only alphanumeric, _, and - characters are allowed"))
		})

		It("should return error when proposal id is unset", func() {
			tx.RepoName = "good-repo"
			tx.ID = ""
			err := validation.CheckTxRepoProposalRegisterPushKey(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:id, msg:proposal id is required"))
		})

		It("should return error when proposal id is not valid", func() {
			tx.RepoName = "good-repo"
			tx.ID = "abc123"
			err := validation.CheckTxRepoProposalRegisterPushKey(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:id, msg:proposal id is not valid"))
		})

		It("should return error when proposal id length exceeds max", func() {
			tx.RepoName = "good-repo"
			tx.ID = "123456789"
			err := validation.CheckTxRepoProposalRegisterPushKey(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:id, msg:proposal id limit of 8 bytes exceeded"))
		})

		It("should return error when value is not provided", func() {
			tx.RepoName = "good-repo"
			tx.Value = ""
			err := validation.CheckTxRepoProposalRegisterPushKey(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:value, msg:value is required"))
		})

		It("should return error when value below minimum network proposal fee", func() {
			params.MinProposalFee = 100
			tx.RepoName = "good-repo"
			tx.Value = "1"
			err := validation.CheckTxRepoProposalRegisterPushKey(tx, -1)
			Expect(err).ToNot(BeNil())
		})

		It("should return error when a push key id is not valid", func() {
			tx.RepoName = "good-repo"
			tx.Value = "1"
			tx.KeyIDs = append(tx.KeyIDs, "push1_abc")
			err := validation.CheckTxRepoProposalRegisterPushKey(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:ids, msg:push key id (push1_abc) is not valid"))
		})

		It("should return error when a push id is a duplicate", func() {
			tx.RepoName = "good-repo"
			tx.Value = "1"
			tx.KeyIDs = append(tx.KeyIDs, "push1wfx7vp8qfyv98cctvamqwec5xjrj48tpxaa77t")
			tx.KeyIDs = append(tx.KeyIDs, "push1wfx7vp8qfyv98cctvamqwec5xjrj48tpxaa77t")
			err := validation.CheckTxRepoProposalRegisterPushKey(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:ids, msg:push key id " +
				"(push1wfx7vp8qfyv98cctvamqwec5xjrj48tpxaa77t) is a duplicate"))
		})

		It("should return error when fee mode is unknown", func() {
			tx.RepoName = "good-repo"
			tx.Value = "1"
			tx.KeyIDs = append(tx.KeyIDs, "push1wfx7vp8qfyv98cctvamqwec5xjrj48tpxaa77t")
			tx.FeeMode = 100
			err := validation.CheckTxRepoProposalRegisterPushKey(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:feeMode, msg:fee mode is unknown"))
		})

		It("should return error when fee mode is FeeModeRepoCapped but fee cap is unset", func() {
			tx.RepoName = "good-repo"
			tx.Value = "1"
			tx.KeyIDs = append(tx.KeyIDs, "push1wfx7vp8qfyv98cctvamqwec5xjrj48tpxaa77t")
			tx.FeeMode = state.FeeModeRepoPaysCapped
			tx.FeeCap = ""
			err := validation.CheckTxRepoProposalRegisterPushKey(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:feeCap, msg:value is required"))
		})

		It("should return error when fee mode is FeeModeRepoCapped but fee cap is not numeric", func() {
			tx.RepoName = "good-repo"
			tx.Value = "1"
			tx.KeyIDs = append(tx.KeyIDs, "push1wfx7vp8qfyv98cctvamqwec5xjrj48tpxaa77t")
			tx.FeeMode = state.FeeModeRepoPaysCapped
			tx.FeeCap = "ten"
			err := validation.CheckTxRepoProposalRegisterPushKey(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:feeCap, msg:invalid number; must be numeric"))
		})

		It("should return error when fee mode is FeeModeRepoCapped but fee cap is not a positive number", func() {
			tx.RepoName = "good-repo"
			tx.Value = "1"
			tx.KeyIDs = append(tx.KeyIDs, "push1wfx7vp8qfyv98cctvamqwec5xjrj48tpxaa77t")
			tx.FeeMode = state.FeeModeRepoPaysCapped
			tx.FeeCap = "-1"
			err := validation.CheckTxRepoProposalRegisterPushKey(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:feeCap, msg:negative figure not allowed"))
		})

		It("should return error when fee mode is not FeeModeRepoCapped but fee cap is set", func() {
			tx.RepoName = "good-repo"
			tx.Value = "1"
			tx.KeyIDs = append(tx.KeyIDs, "push1wfx7vp8qfyv98cctvamqwec5xjrj48tpxaa77t")
			tx.FeeMode = state.FeeModeRepoPays
			tx.FeeCap = "1"
			err := validation.CheckTxRepoProposalRegisterPushKey(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:feeCap, msg:value not expected for the chosen fee mode"))
		})

		It("should return error when namespace value format is invalid", func() {
			tx.RepoName = "good-repo"
			tx.Value = "10"
			tx.KeyIDs = append(tx.KeyIDs, "push1wfx7vp8qfyv98cctvamqwec5xjrj48tpxaa77t")
			tx.FeeMode = state.FeeModeRepoPays
			tx.Namespace = "inv&alid"
			err := validation.CheckTxRepoProposalRegisterPushKey(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:namespace, msg:value format is not valid"))
		})

		It("should return error when namespace is set but namespaceOnly is also set", func() {
			tx.RepoName = "good-repo"
			tx.Value = "10"
			tx.KeyIDs = append(tx.KeyIDs, "push1wfx7vp8qfyv98cctvamqwec5xjrj48tpxaa77t")
			tx.FeeMode = state.FeeModeRepoPays
			tx.Namespace = "ns1"
			tx.NamespaceOnly = "ns2"
			err := validation.CheckTxRepoProposalRegisterPushKey(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:namespaceOnly, msg:field is not expected because 'namespace' is set"))
		})

		It("should return error when namespaceOnly value format is invalid", func() {
			tx.RepoName = "good-repo"
			tx.Value = "10"
			tx.KeyIDs = append(tx.KeyIDs, "push1wfx7vp8qfyv98cctvamqwec5xjrj48tpxaa77t")
			tx.FeeMode = state.FeeModeRepoPays
			tx.NamespaceOnly = "inv&alid"
			err := validation.CheckTxRepoProposalRegisterPushKey(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:namespaceOnly, msg:value format is not valid"))
		})
	})
})
