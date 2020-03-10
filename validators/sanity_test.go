package validators_test

import (
	"io/ioutil"
	"os"
	"strings"
	"time"

	"gitlab.com/makeos/mosdef/types/core"
	"gitlab.com/makeos/mosdef/types/state"

	"github.com/shopspring/decimal"
	"gitlab.com/makeos/mosdef/params"

	"gitlab.com/makeos/mosdef/crypto"
	"gitlab.com/makeos/mosdef/util"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/testutil"
	"gitlab.com/makeos/mosdef/validators"
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
		params.MinProposalFee = float64(0)
	})

	AfterEach(func() {
		err = os.RemoveAll(cfg.DataDir())
		Expect(err).To(BeNil())
	})

	Describe(".CheckRecipient", func() {
		When("recipient address is not set", func() {
			It("should return err", func() {
				tx := core.NewBareTxCoinTransfer()
				err := validators.CheckRecipient(tx.TxRecipient, 0)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("index:0, field:to, msg:recipient address is required"))
			})
		})

		When("recipient address is an invalid base58 encoded address", func() {
			It("should return err", func() {
				tx := core.NewBareTxCoinTransfer()
				tx.To = "abcdef"
				err := validators.CheckRecipient(tx.TxRecipient, 0)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("index:0, field:to, msg:recipient address is not valid"))
			})
		})

		When("recipient address is not base58 encoded but a namespaced address", func() {
			It("should return no error", func() {
				tx := core.NewBareTxCoinTransfer()
				tx.To = "namespace/domain"
				err := validators.CheckRecipient(tx.TxRecipient, 0)
				Expect(err).To(BeNil())
			})
		})

		When("recipient address is not base58 encoded but a prefixed address", func() {
			It("should return no error", func() {
				tx := core.NewBareTxCoinTransfer()
				tx.To = "r/domain"
				err := validators.CheckRecipient(tx.TxRecipient, 0)
				Expect(err).To(BeNil())
			})
		})

		When("recipient address is not base58 encoded but a prefixed account address", func() {
			It("should return err", func() {
				tx := core.NewBareTxCoinTransfer()
				tx.To = "a/abcdef"
				err := validators.CheckRecipient(tx.TxRecipient, 0)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("index:0, field:to, msg:recipient address is not valid"))
			})
		})

		When("recipient address is a base58 encoded address that is valid", func() {
			It("should return no error", func() {
				tx := core.NewBareTxCoinTransfer()
				tx.To = key.Addr()
				err := validators.CheckRecipient(tx.TxRecipient, 0)
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".CheckTxCoinTransfer", func() {
		var tx *core.TxCoinTransfer
		BeforeEach(func() {
			tx = core.NewBareTxCoinTransfer()
			tx.To = key.Addr()
			tx.Fee = "1"
		})

		When("it has invalid fields, it should return error when", func() {
			It("should return error='type is invalid'", func() {
				tx.Type = -10
				err := validators.CheckTxCoinTransfer(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:type, msg:type is invalid"))
			})

			It("has no recipient address", func() {
				tx.To = ""
				err := validators.CheckTxCoinTransfer(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:to, msg:recipient address is required"))
			})

			It("has invalid recipient address", func() {
				tx.To = "invalid"
				err := validators.CheckTxCoinTransfer(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:to, msg:recipient address is not valid"))
			})

			It("has invalid value", func() {
				tx.Value = "invalid"
				err := validators.CheckTxCoinTransfer(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:value, msg:invalid number; must be numeric"))
			})

			It("has no nonce", func() {
				err := validators.CheckTxCoinTransfer(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:nonce, msg:nonce is required"))
			})

			It("has invalid fee", func() {
				tx.Nonce = 1
				tx.Fee = "invalid"
				err := validators.CheckTxCoinTransfer(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:fee, msg:invalid number; must be numeric"))
			})

			It("has low fee", func() {
				tx.Nonce = 1
				tx.Fee = "0"
				err := validators.CheckTxCoinTransfer(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("field:fee, msg:fee cannot be lower than the base price"))
			})

			It("has no timestamp", func() {
				tx.Nonce = 1
				err := validators.CheckTxCoinTransfer(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:timestamp, msg:timestamp is required"))
			})

			It("has no public key", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				err := validators.CheckTxCoinTransfer(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:senderPubKey, msg:sender public key is required"))
			})

			It("has no signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToPublicKey(key.PubKey().MustBytes())
				err := validators.CheckTxCoinTransfer(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, msg:signature is required"))
			})

			It("has invalid signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToPublicKey(key.PubKey().MustBytes())
				tx.Sig = []byte("invalid")
				err := validators.CheckTxCoinTransfer(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, msg:signature is not valid"))
			})
		})

		When("it has no error", func() {
			It("should return no error", func() {
				tx.To = key.Addr()
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToPublicKey(key.PubKey().MustBytes())
				sig, err := tx.Sign(key.PrivKey().Base58())
				Expect(err).To(BeNil())
				tx.Sig = sig
				err = validators.CheckTxCoinTransfer(tx, -1)
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".CheckTxNSAcquire", func() {
		var tx *core.TxNamespaceAcquire
		BeforeEach(func() {
			params.CostOfNamespace = decimal.NewFromFloat(5)
			tx = core.NewBareTxNamespaceAcquire()
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
				Expect(err.Error()).To(Equal("field:type, msg:type is invalid"))
			})

			It("has invalid value", func() {
				tx.Value = "invalid"
				err := validators.CheckTxNSAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:value, msg:invalid number; must be numeric"))
			})

			It("has no name", func() {
				tx.Name = ""
				err := validators.CheckTxNSAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:name, msg:requires a unique name"))
			})

			It("has an invalid name", func() {
				tx.Name = "invalid&"
				err := validators.CheckTxNSAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:name, msg:invalid characters in name. Only alphanumeric, _ and - characters are allowed"))
			})

			It("has transfer repo and account fields set", func() {
				tx.TransferToRepo = "repo"
				tx.TransferToAccount = "account"
				err := validators.CheckTxNSAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:, msg:can only transfer ownership to either an account or a repo"))
			})

			It("has invalid transfer account", func() {
				tx.TransferToAccount = "account"
				err := validators.CheckTxNSAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:toAccount, msg:address is not valid"))
			})

			It("has invalid transfer account", func() {
				tx.TransferToRepo = "re&&^po"
				err := validators.CheckTxNSAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:toRepo, msg:repo name is not valid"))
			})

			It("has value not equal to namespace price", func() {
				tx.Value = "1"
				err := validators.CheckTxNSAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:value, msg:invalid value; has 1, want 5"))
			})

			It("has domain target with invalid format", func() {
				tx.Value = "5"
				tx.Domains["domain"] = "invalid:format"
				err := validators.CheckTxNSAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:domains, msg:domains.domain: target is invalid"))
			})

			It("has domain target with unknown target type", func() {
				tx.Value = "5"
				tx.Domains["domain"] = "unknown_type/name"
				err := validators.CheckTxNSAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:domains, msg:domains.domain: target is invalid"))
			})

			It("has domain target with account target type that has an invalid address", func() {
				tx.Value = "5"
				tx.Domains["domain"] = "a/invalid_addr"
				err := validators.CheckTxNSAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:domains, msg:domains.domain: target is not a valid address"))
			})

			It("has invalid fee", func() {
				tx.Nonce = 1
				tx.Fee = "invalid"
				err := validators.CheckTxNSAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:fee, msg:invalid number; must be numeric"))
			})

			It("has low fee", func() {
				tx.Nonce = 1
				tx.Fee = "0"
				err := validators.CheckTxNSAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("field:fee, msg:fee cannot be lower than the base price"))
			})

			It("has no nonce", func() {
				tx.Nonce = 0
				err := validators.CheckTxNSAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:nonce, msg:nonce is required"))
			})

			It("has no timestamp", func() {
				tx.Nonce = 1
				err := validators.CheckTxNSAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:timestamp, msg:timestamp is required"))
			})

			It("has no public key", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				err := validators.CheckTxNSAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:senderPubKey, msg:sender public key is required"))
			})

			It("has no signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToPublicKey(key.PubKey().MustBytes())
				err := validators.CheckTxNSAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, msg:signature is required"))
			})

			It("has invalid signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToPublicKey(key.PubKey().MustBytes())
				tx.Sig = []byte("invalid")
				err := validators.CheckTxNSAcquire(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, msg:signature is not valid"))
			})
		})

		When("it has no error", func() {
			It("should return no error", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToPublicKey(key.PubKey().MustBytes())
				tx.Domains["domain"] = "r/repo1"
				sig, err := tx.Sign(key.PrivKey().Base58())
				Expect(err).To(BeNil())
				tx.Sig = sig
				err = validators.CheckTxNSAcquire(tx, -1)
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".checkNamespaceDomains", func() {
		When("map include a domain that is not valid", func() {
			It("should return err", func() {
				domains := map[string]string{"goo&": "abc"}
				err := validators.CheckNamespaceDomains(domains, 0)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("index:0, field:domains, msg:domains.goo&: name is invalid"))
			})
		})

		When("map include a domain with a target whose name is not valid", func() {
			It("should return err", func() {
				domains := map[string]string{"google": "xyz"}
				err := validators.CheckNamespaceDomains(domains, 0)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("index:0, field:domains, msg:domains.google: target is invalid"))
			})
		})

		When("map include a domain with an address target that has an invalid address", func() {
			It("should return err", func() {
				domains := map[string]string{"google": "a/xyz"}
				err := validators.CheckNamespaceDomains(domains, 0)
				Expect(err).ToNot(BeNil())
				Expect(err).To(MatchError("index:0, field:domains, msg:domains.google: target is not a valid address"))
			})
		})
	})

	Describe(".CheckTxTicketPurchase", func() {
		var tx *core.TxTicketPurchase
		BeforeEach(func() {
			tx = core.NewBareTxTicketPurchase(core.TxTypeValidatorTicket)
			tx.Fee = "1"
			tx.Value = "1"
		})

		When("it has invalid fields, it should return error when", func() {
			It("should return error='type is invalid'", func() {
				tx.Type = -10
				err := validators.CheckTxTicketPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:type, msg:type is invalid"))
			})

			It("has invalid value", func() {
				tx.Value = "invalid"
				err := validators.CheckTxTicketPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:value, msg:invalid number; must be numeric"))
			})

			It("has type of TxTypeHostTicket and value is lower than minimum stake", func() {
				params.MinHostStake = decimal.NewFromFloat(20)
				tx.Type = core.TxTypeHostTicket
				tx.Value = "10"
				err := validators.CheckTxTicketPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:value, msg:value is lower than minimum host stake"))
			})

			It("has negative or zero value", func() {
				tx.Value = "0"
				err := validators.CheckTxTicketPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:value, msg:value must be a positive number"))
			})

			It("has no nonce", func() {
				err := validators.CheckTxTicketPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:nonce, msg:nonce is required"))
			})

			It("has invalid fee", func() {
				tx.Nonce = 1
				tx.Fee = "invalid"
				err := validators.CheckTxTicketPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:fee, msg:invalid number; must be numeric"))
			})

			It("has no timestamp", func() {
				tx.Nonce = 1
				err := validators.CheckTxTicketPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:timestamp, msg:timestamp is required"))
			})

			It("has no public key", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				err := validators.CheckTxTicketPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:senderPubKey, msg:sender public key is required"))
			})

			It("has no signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToPublicKey(key.PubKey().MustBytes())
				err := validators.CheckTxTicketPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, msg:signature is required"))
			})

			It("has invalid signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToPublicKey(key.PubKey().MustBytes())
				tx.Sig = []byte("invalid")
				err := validators.CheckTxTicketPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, msg:signature is not valid"))
			})

			It("has type of TxTypeHostTicket and BLS public key is unset", func() {
				params.MinHostStake = decimal.NewFromFloat(5)
				tx.Value = "10"
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.Type = core.TxTypeHostTicket
				err := validators.CheckTxTicketPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:blsPubKey, msg:BLS public key is required"))
			})

			It("has type of TxTypeHostTicket and BLS public key has invalid length", func() {
				params.MinHostStake = decimal.NewFromFloat(5)
				tx.Value = "10"
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.Type = core.TxTypeHostTicket
				tx.BLSPubKey = util.RandBytes(32)
				err := validators.CheckTxTicketPurchase(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:blsPubKey, msg:BLS public key length is invalid"))
			})
		})

		When("it has no error", func() {
			It("should return no error", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToPublicKey(key.PubKey().MustBytes())
				sig, err := tx.Sign(key.PrivKey().Base58())
				Expect(err).To(BeNil())
				tx.Sig = sig
				err = validators.CheckTxTicketPurchase(tx, -1)
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".CheckTxUnbondTicket", func() {
		var tx *core.TxTicketUnbond

		BeforeEach(func() {
			tx = core.NewBareTxTicketUnbond(core.TxTypeHostTicket)
			tx.TicketHash = util.StrToBytes32("hash")
			tx.Fee = "1"
		})

		When("it has invalid fields, it should return error when", func() {
			It("should return error='type is invalid'", func() {
				tx.Type = -10
				err := validators.CheckTxUnbondTicket(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:type, msg:type is invalid"))
			})

			It("has no ticket hash", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.TicketHash = util.EmptyBytes32
				err := validators.CheckTxUnbondTicket(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:ticket, msg:ticket id is required"))
			})

			It("has no nonce", func() {
				err := validators.CheckTxUnbondTicket(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:nonce, msg:nonce is required"))
			})

			It("has invalid fee", func() {
				tx.Nonce = 1
				tx.Fee = "invalid"
				err := validators.CheckTxUnbondTicket(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:fee, msg:invalid number; must be numeric"))
			})

			It("has no timestamp", func() {
				tx.Nonce = 1
				err := validators.CheckTxUnbondTicket(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:timestamp, msg:timestamp is required"))
			})

			It("has no public key", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				err := validators.CheckTxUnbondTicket(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:senderPubKey, msg:sender public key is required"))
			})

			It("has no signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToPublicKey(key.PubKey().MustBytes())
				err := validators.CheckTxUnbondTicket(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, msg:signature is required"))
			})

			It("has invalid signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToPublicKey(key.PubKey().MustBytes())
				tx.Sig = []byte("invalid")
				err := validators.CheckTxUnbondTicket(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, msg:signature is not valid"))
			})
		})

		When("it has no error", func() {
			It("should return no error", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToPublicKey(key.PubKey().MustBytes())
				sig, err := tx.Sign(key.PrivKey().Base58())
				Expect(err).To(BeNil())
				tx.Sig = sig
				err = validators.CheckTxUnbondTicket(tx, -1)
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".CheckRepoConfig", func() {
		When("proposee type is unknown", func() {
			It("should return error", func() {
				repoCfg := &state.RepoConfig{
					Governance: &state.RepoConfigGovernance{
						ProposalProposee: 1000,
					},
				}
				err := validators.CheckRepoConfig(repoCfg.ToMap(), -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:config.gov.propProposee, msg:unknown value"))
			})
		})

		When("tally method is unknown", func() {
			It("should return error", func() {
				repoCfg := &state.RepoConfig{
					Governance: &state.RepoConfigGovernance{
						ProposalProposee:    state.ProposeeOwner,
						ProposalTallyMethod: 1000,
					},
				}
				err := validators.CheckRepoConfig(repoCfg.ToMap(), -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:config.gov.propTallyMethod, msg:unknown value"))
			})
		})

		When("quorum is negative", func() {
			It("should return error", func() {
				repoCfg := &state.RepoConfig{
					Governance: &state.RepoConfigGovernance{
						ProposalProposee:    state.ProposeeOwner,
						ProposalTallyMethod: state.ProposalTallyMethodNetStake,
						ProposalQuorum:      -1,
					},
				}
				err := validators.CheckRepoConfig(repoCfg.ToMap(), -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:config.gov.propQuorum, msg:must be a non-negative number"))
			})
		})

		When("threshold is negative", func() {
			It("should return error", func() {
				repoCfg := &state.RepoConfig{
					Governance: &state.RepoConfigGovernance{
						ProposalProposee:    state.ProposeeOwner,
						ProposalTallyMethod: state.ProposalTallyMethodNetStake,
						ProposalQuorum:      1,
						ProposalThreshold:   -1,
					},
				}
				err := validators.CheckRepoConfig(repoCfg.ToMap(), -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:config.gov.propThreshold, msg:must be a non-negative number"))
			})
		})

		When("veto quorum is negative", func() {
			It("should return error", func() {
				repoCfg := &state.RepoConfig{
					Governance: &state.RepoConfigGovernance{
						ProposalProposee:    state.ProposeeOwner,
						ProposalTallyMethod: state.ProposalTallyMethodNetStake,
						ProposalQuorum:      1,
						ProposalThreshold:   1,
						ProposalVetoQuorum:  -1,
					},
				}
				err := validators.CheckRepoConfig(repoCfg.ToMap(), -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:config.gov.propVetoQuorum, msg:must be a non-negative number"))
			})
		})

		When("veto owners quorum is negative", func() {
			It("should return error", func() {
				repoCfg := &state.RepoConfig{
					Governance: &state.RepoConfigGovernance{
						ProposalProposee:         state.ProposeeOwner,
						ProposalTallyMethod:      state.ProposalTallyMethodNetStake,
						ProposalQuorum:           1,
						ProposalThreshold:        1,
						ProposalVetoQuorum:       1,
						ProposalVetoOwnersQuorum: -1,
					},
				}
				err := validators.CheckRepoConfig(repoCfg.ToMap(), -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:config.gov.propVetoOwnersQuorum, msg:must be a non-negative number"))
			})
		})

		When("proposal fee is below network minimum", func() {
			It("should return error", func() {
				params.MinProposalFee = float64(400)
				repoCfg := &state.RepoConfig{
					Governance: &state.RepoConfigGovernance{
						ProposalProposee:         state.ProposeeOwner,
						ProposalTallyMethod:      state.ProposalTallyMethodNetStake,
						ProposalQuorum:           1,
						ProposalThreshold:        1,
						ProposalVetoQuorum:       1,
						ProposalVetoOwnersQuorum: 1,
						ProposalFee:              1,
					},
				}
				err := validators.CheckRepoConfig(repoCfg.ToMap(), -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:config.gov.propFee, msg:cannot be lower " +
					"than network minimum"))
			})
		})

		When("proposee is not ProposeeOwner and tally method is CoinWeighted", func() {
			It("should return error", func() {
				repoCfg := &state.RepoConfig{
					Governance: &state.RepoConfigGovernance{
						ProposalProposee:    state.ProposeeNetStakeholders,
						ProposalTallyMethod: state.ProposalTallyMethodCoinWeighted,
					},
				}
				err := validators.CheckRepoConfig(repoCfg.ToMap(), -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:config, msg:when proposee type " +
					"is not 'ProposeeOwner', tally methods 'CoinWeighted' and 'Identity' are not allowed"))
			})
		})

		When("proposee is not ProposeeOwner and tally method is Identity", func() {
			It("should return error", func() {
				repoCfg := &state.RepoConfig{
					Governance: &state.RepoConfigGovernance{
						ProposalProposee:    state.ProposeeNetStakeholders,
						ProposalTallyMethod: state.ProposalTallyMethodIdentity,
					},
				}
				err := validators.CheckRepoConfig(repoCfg.ToMap(), -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:config, msg:when proposee type " +
					"is not 'ProposeeOwner', tally methods 'CoinWeighted' and 'Identity' are not allowed"))
			})
		})
	})

	Describe(".CheckTxRepoCreate", func() {
		var tx *core.TxRepoCreate
		BeforeEach(func() {
			tx = core.NewBareTxRepoCreate()
			tx.Name = "repo"
			tx.Fee = "1"
		})

		When("it has invalid fields, it should return error when", func() {
			It("should return error='type is invalid'", func() {
				tx.Type = -10
				err := validators.CheckTxRepoCreate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:type, msg:type is invalid"))
			})

			It("has invalid value", func() {
				tx.Value = "invalid"
				err := validators.CheckTxRepoCreate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:value, msg:invalid number; must be numeric"))
			})

			It("has no name", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.Name = ""
				err := validators.CheckTxRepoCreate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:name, msg:requires a unique name"))
			})

			It("has invalid name", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.Name = "org&name#"
				err := validators.CheckTxRepoCreate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:name, msg:invalid characters in name. Only alphanumeric, _ and - characters are allowed"))
			})

			It("has no nonce", func() {
				err := validators.CheckTxRepoCreate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:nonce, msg:nonce is required"))
			})

			It("has invalid fee", func() {
				tx.Nonce = 1
				tx.Fee = "invalid"
				err := validators.CheckTxRepoCreate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:fee, msg:invalid number; must be numeric"))
			})

			It("has no timestamp", func() {
				tx.Nonce = 1
				err := validators.CheckTxRepoCreate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:timestamp, msg:timestamp is required"))
			})

			It("has no public key", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				err := validators.CheckTxRepoCreate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:senderPubKey, msg:sender public key is required"))
			})

			It("has no signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToPublicKey(key.PubKey().MustBytes())
				err := validators.CheckTxRepoCreate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, msg:signature is required"))
			})

			It("has invalid signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToPublicKey(key.PubKey().MustBytes())
				tx.Sig = []byte("invalid")
				err := validators.CheckTxRepoCreate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, msg:signature is not valid"))
			})
		})

		When("it has no error", func() {
			It("should return no error", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToPublicKey(key.PubKey().MustBytes())
				sig, err := tx.Sign(key.PrivKey().Base58())
				Expect(err).To(BeNil())
				tx.Sig = sig
				err = validators.CheckTxRepoCreate(tx, -1)
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".CheckTxRegisterGPGPubKey", func() {
		var tx *core.TxRegisterGPGPubKey
		var gpgKey []byte

		BeforeEach(func() {
			gpgKey, err = ioutil.ReadFile("testdata/gpgkey.pub")
			Expect(err).To(BeNil())
			tx = core.NewBareTxRegisterGPGPubKey()
			tx.PublicKey = string(gpgKey)
			tx.Fee = "2"
		})

		When("it has invalid fields, it should return error when", func() {
			It("should return error='type is invalid'", func() {
				tx.Type = -10
				err := validators.CheckTxRegisterGPGPubKey(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:type, msg:type is invalid"))
			})

			It("has no gpg public key", func() {
				tx.PublicKey = ""
				err := validators.CheckTxRegisterGPGPubKey(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:pubKey, msg:public key is required"))
			})

			It("has invalid gpg public key", func() {
				tx.PublicKey = "invalid"
				err := validators.CheckTxRegisterGPGPubKey(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:pubKey, msg:invalid gpg public key"))
			})

			It("has invalid scopes", func() {
				scopes := []string{
					"r/repo",
					"maker13463exprf3fdq44eth4lkf99dy6z5ajuk4ln4z",
					"a/maker13463exprf3fdq44eth4lkf99dy6z5ajuk4ln4z",
					"repo_&*",
				}
				for _, s := range scopes {
					tx.Scopes = []string{s}
					err := validators.CheckTxRegisterGPGPubKey(tx, -1)
					Expect(err).ToNot(BeNil())
					Expect(err).To(MatchError("field:scopes[0], msg:not an acceptable scope. " +
						"Expects a namespace URI or repository name"))
				}
			})

			It("has invalid fee cap", func() {
				tx.FeeCap = "1a"
				err := validators.CheckTxRegisterGPGPubKey(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:feeCap, msg:invalid number; must be numeric"))
			})

			It("has no nonce", func() {
				err := validators.CheckTxRegisterGPGPubKey(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:nonce, msg:nonce is required"))
			})

			It("has invalid fee", func() {
				tx.Nonce = 1
				tx.Fee = "invalid"
				err := validators.CheckTxRegisterGPGPubKey(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:fee, msg:invalid number; must be numeric"))
			})

			It("has no timestamp", func() {
				tx.Nonce = 1
				err := validators.CheckTxRegisterGPGPubKey(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:timestamp, msg:timestamp is required"))
			})

			It("has no public key", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				err := validators.CheckTxRegisterGPGPubKey(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:senderPubKey, msg:sender public key is required"))
			})

			It("has no signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToPublicKey(key.PubKey().MustBytes())
				err := validators.CheckTxRegisterGPGPubKey(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, msg:signature is required"))
			})

			It("has invalid signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToPublicKey(key.PubKey().MustBytes())
				tx.Sig = []byte("invalid")
				err := validators.CheckTxRegisterGPGPubKey(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, msg:signature is not valid"))
			})
		})

		When("it has no error", func() {
			It("should return no error", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToPublicKey(key.PubKey().MustBytes())
				sig, err := tx.Sign(key.PrivKey().Base58())
				Expect(err).To(BeNil())
				tx.Sig = sig
				err = validators.CheckTxRegisterGPGPubKey(tx, -1)
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".CheckTxSetDelegateCommission", func() {
		var tx *core.TxSetDelegateCommission

		BeforeEach(func() {
			tx = core.NewBareTxSetDelegateCommission()
			tx.Commission = "60"
			tx.Fee = "1"
		})

		When("it has invalid fields, it should return error when", func() {
			It("should return error='type is invalid'", func() {
				tx.Type = -10
				err := validators.CheckTxSetDelegateCommission(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:type, msg:type is invalid"))
			})

			It("has no commission value", func() {
				tx.Commission = ""
				err := validators.CheckTxSetDelegateCommission(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:commission, msg:commission rate is required"))
			})

			It("has no commission value is below minimum", func() {
				params.MinDelegatorCommission = decimal.NewFromFloat(50)
				tx.Commission = "49"
				err := validators.CheckTxSetDelegateCommission(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:commission, msg:rate cannot be below the minimum (50%)"))
			})

			It("has no commission value is above 100", func() {
				params.MinDelegatorCommission = decimal.NewFromFloat(50)
				tx.Commission = "101"
				err := validators.CheckTxSetDelegateCommission(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:commission, msg:commission rate cannot exceed 100%"))
			})

			It("has no nonce", func() {
				err := validators.CheckTxSetDelegateCommission(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:nonce, msg:nonce is required"))
			})

			It("has invalid fee", func() {
				tx.Nonce = 1
				tx.Fee = "invalid"
				err := validators.CheckTxSetDelegateCommission(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:fee, msg:invalid number; must be numeric"))
			})

			It("has no timestamp", func() {
				tx.Nonce = 1
				err := validators.CheckTxSetDelegateCommission(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:timestamp, msg:timestamp is required"))
			})

			It("has no public key", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				err := validators.CheckTxSetDelegateCommission(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:senderPubKey, msg:sender public key is required"))
			})

			It("has no signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToPublicKey(key.PubKey().MustBytes())
				err := validators.CheckTxSetDelegateCommission(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, msg:signature is required"))
			})

			It("has invalid signature", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToPublicKey(key.PubKey().MustBytes())
				tx.Sig = []byte("invalid")
				err := validators.CheckTxSetDelegateCommission(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:sig, msg:signature is not valid"))
			})
		})

		When("it has no error", func() {
			It("should return no error", func() {
				tx.Nonce = 1
				tx.Timestamp = time.Now().Unix()
				tx.SenderPubKey = util.BytesToPublicKey(key.PubKey().MustBytes())
				sig, err := tx.Sign(key.PrivKey().Base58())
				Expect(err).To(BeNil())
				tx.Sig = sig
				err = validators.CheckTxSetDelegateCommission(tx, -1)
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".CheckTxNamespaceDomainUpdate", func() {
		var tx *core.TxNamespaceDomainUpdate

		BeforeEach(func() {
			tx = core.NewBareTxNamespaceDomainUpdate()
			tx.Fee = "1"
		})

		When("it has invalid fields, it should return error when", func() {
			It("should return error='type is invalid'", func() {
				tx.Type = -10
				err := validators.CheckTxNamespaceDomainUpdate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:type, msg:type is invalid"))
			})
		})

		When("name is not set", func() {
			It("should return err", func() {
				err := validators.CheckTxNamespaceDomainUpdate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:name, msg:requires a name"))
			})
		})

		When("name is not valid", func() {
			It("should return err", func() {
				tx.Name = "&name"
				err := validators.CheckTxNamespaceDomainUpdate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:name, msg:invalid characters in name. Only alphanumeric, _ and - characters are allowed"))
			})
		})

		When("name is too short", func() {
			It("should return err", func() {
				tx.Name = "ab"
				err := validators.CheckTxNamespaceDomainUpdate(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:name, msg:name is too short. Must be at least 3 characters long"))
			})
		})
	})

	Describe(".CheckTxPush", func() {
		var tx *core.TxPush

		BeforeEach(func() {
			tx = core.NewBareTxPush()
			tx.Timestamp = time.Now().Unix()
			tx.PushNote.RepoName = "repo1"
			tx.PushNote.PusherKeyID = util.RandBytes(20)
			tx.PushNote.Timestamp = time.Now().Unix()
			tx.PushNote.AccountNonce = 1
			tx.PushNote.Fee = "1"
			tx.PushNote.NodePubKey = key.PubKey().MustBytes32()
			tx.PushNote.NodeSig = key.PrivKey().MustSign(tx.PushNote.Bytes())
		})

		When("it has invalid fields, it should return error when", func() {
			It("should return error='type is invalid'", func() {
				tx.Type = -10
				err := validators.CheckTxPush(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:type, msg:type is invalid"))
			})

			It("has no push note", func() {
				tx.PushNote = nil
				err := validators.CheckTxPush(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:pushNote, msg:push note is required"))
			})

			It("has an invalid push note (with no repo name)", func() {
				tx.PushNote.RepoName = ""
				err := validators.CheckTxPush(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:repoName, msg:repo name is required"))
			})

			It("has low endorsement (not up to quorum)", func() {
				params.PushOKQuorumSize = 1
				err := validators.CheckTxPush(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("field:endorsements, msg:not enough endorsements included"))
			})

			It("has a no push note id", func() {
				params.PushOKQuorumSize = 1
				tx.PushOKs = append(tx.PushOKs, &core.PushOK{})
				tx.SenderPubKey = util.BytesToPublicKey(key.PubKey().MustBytes())
				sig, _ := key.PrivKey().Sign(tx.Bytes())
				tx.Sig = sig
				err := validators.CheckTxPush(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("index:0, field:endorsements.pushNoteID, msg:push note id is required"))
			})

			It("has a PushOK with no sender public key", func() {
				params.PushOKQuorumSize = 1
				tx.PushOKs = append(tx.PushOKs, &core.PushOK{
					PushNoteID:   util.StrToBytes32("id"),
					SenderPubKey: util.EmptyBytes32,
				})
				tx.SenderPubKey = util.BytesToPublicKey(key.PubKey().MustBytes())
				sig, _ := key.PrivKey().Sign(tx.Bytes())
				tx.Sig = sig
				err := validators.CheckTxPush(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("index:0, field:endorsements.senderPubKey, msg:sender public key is required"))
			})

			It("has a PushOK with a push note id that is different from the PushTx.PushNoteID", func() {
				params.PushOKQuorumSize = 1
				tx.PushOKs = append(tx.PushOKs, &core.PushOK{
					PushNoteID:   util.StrToBytes32("id"),
					SenderPubKey: key.PubKey().MustBytes32(),
				})
				tx.SenderPubKey = util.BytesToPublicKey(key.PubKey().MustBytes())
				sig, _ := key.PrivKey().Sign(tx.Bytes())
				tx.Sig = sig
				err := validators.CheckTxPush(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("index:0, field:endorsements.pushNoteID, msg:push note id and push endorsement id must match"))
			})

			It("has multiple PushOKs from same sender", func() {
				params.PushOKQuorumSize = 1

				pushOK1 := &core.PushOK{
					PushNoteID:   tx.PushNote.ID(),
					SenderPubKey: util.BytesToBytes32(key.PubKey().MustBytes()),
				}
				sig, _ := key.PrivKey().Sign(pushOK1.Bytes())
				pushOK1.Sig = util.BytesToBytes64(sig)
				tx.PushOKs = append(tx.PushOKs, pushOK1)

				pushOK2 := &core.PushOK{
					PushNoteID:   tx.PushNote.ID(),
					SenderPubKey: util.BytesToBytes32(key.PubKey().MustBytes()),
				}
				sig, _ = key.PrivKey().Sign(pushOK2.Bytes())
				pushOK2.Sig = util.BytesToBytes64(sig)
				tx.PushOKs = append(tx.PushOKs, pushOK2)

				tx.SenderPubKey = util.BytesToPublicKey(key.PubKey().MustBytes())
				sig, _ = key.PrivKey().Sign(tx.Bytes())
				tx.Sig = sig
				err := validators.CheckTxPush(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("index:1, field:endorsements.senderPubKey, msg:multiple endorsement by a single sender not permitted"))
			})

			It("has PushOKs with different references hash set", func() {
				params.PushOKQuorumSize = 1

				pushOK1 := &core.PushOK{
					PushNoteID:   tx.PushNote.ID(),
					SenderPubKey: util.BytesToBytes32(key.PubKey().MustBytes()),
					ReferencesHash: []*core.ReferenceHash{
						{Hash: util.BytesToBytes32(util.RandBytes(32))},
					},
				}
				sig, _ := key.PrivKey().Sign(pushOK1.Bytes())
				pushOK1.Sig = util.BytesToBytes64(sig)
				tx.PushOKs = append(tx.PushOKs, pushOK1)

				pushOK2 := &core.PushOK{
					PushNoteID:   tx.PushNote.ID(),
					SenderPubKey: util.BytesToBytes32(key2.PubKey().MustBytes()),
					ReferencesHash: []*core.ReferenceHash{
						{Hash: util.BytesToBytes32(util.RandBytes(32))},
					},
				}
				sig, _ = key2.PrivKey().Sign(pushOK2.Bytes())
				pushOK2.Sig = util.BytesToBytes64(sig)
				tx.PushOKs = append(tx.PushOKs, pushOK2)

				tx.SenderPubKey = util.BytesToPublicKey(key.PubKey().MustBytes())
				sig, _ = key.PrivKey().Sign(tx.Bytes())
				tx.Sig = sig
				err := validators.CheckTxPush(tx, -1)
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(Equal("index:1, field:endorsements.refsHash, msg:references of all endorsements must match"))
			})
		})

		When("no error", func() {
			It("should return no error", func() {
				params.PushOKQuorumSize = 1

				pok := &core.PushOK{
					PushNoteID:   tx.PushNote.ID(),
					SenderPubKey: util.BytesToBytes32(key.PubKey().MustBytes()),
				}
				sig, _ := key.PrivKey().Sign(pok.Bytes())
				pok.Sig = util.BytesToBytes64(sig)

				tx.PushOKs = append(tx.PushOKs, pok)
				tx.SenderPubKey = util.BytesToPublicKey(key.PubKey().MustBytes())
				sig, _ = key.PrivKey().Sign(tx.Bytes())
				tx.Sig = sig

				err := validators.CheckTxPush(tx, -1)
				Expect(err).To(BeNil())
			})
		})
	})

	Describe(".CheckTxRepoProposalUpsertOwner", func() {
		var tx *core.TxRepoProposalUpsertOwner

		BeforeEach(func() {
			params.MinProposalFee = 10
			tx = core.NewBareRepoProposalUpsertOwner()
			tx.Timestamp = time.Now().Unix()
			tx.Value = "11"
			tx.ProposalID = "123"
		})

		It("should return error when repo name is not provided", func() {
			err := validators.CheckTxRepoProposalUpsertOwner(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:name, msg:repo name is required"))
		})

		It("should return error when repo name is not valid", func() {
			tx.RepoName = "*&^"
			err := validators.CheckTxRepoProposalUpsertOwner(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:name, msg:invalid characters in name. Only alphanumeric, _ and - characters are allowed"))
		})

		It("should return error when proposal id is unset", func() {
			tx.RepoName = "good-repo"
			tx.ProposalID = ""
			err := validators.CheckTxRepoProposalUpsertOwner(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:id, msg:proposal id is required"))
		})

		It("should return error when proposal id is not valid", func() {
			tx.RepoName = "good-repo"
			tx.ProposalID = "abc123"
			err := validators.CheckTxRepoProposalUpsertOwner(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:id, msg:proposal id is not valid"))
		})

		It("should return error when proposal id length exceeds max", func() {
			tx.RepoName = "good-repo"
			tx.ProposalID = "123456789"
			err := validators.CheckTxRepoProposalUpsertOwner(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:id, msg:proposal id limit of 8 bytes exceeded"))
		})

		It("should return error when value is not provided", func() {
			tx.RepoName = "good-repo"
			tx.Value = ""
			err := validators.CheckTxRepoProposalUpsertOwner(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:value, msg:value is required"))
		})

		It("should return error when value below minimum network proposal fee", func() {
			params.MinProposalFee = 100
			tx.RepoName = "good-repo"
			tx.Value = "1"
			err := validators.CheckTxRepoProposalUpsertOwner(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:value, msg:proposal creation fee cannot be less than network minimum"))
		})

		It("should return error when target address is not provided", func() {
			tx.RepoName = "repo1"
			err := validators.CheckTxRepoProposalUpsertOwner(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:addresses, msg:at least one address is required"))
		})

		It("should return error when target addresses exceed maximum", func() {
			tx.RepoName = "repo1"
			addresses := strings.TrimRight(strings.Repeat("addr1,", 11), ",")
			tx.Addresses = strings.Split(addresses, ",")
			err := validators.CheckTxRepoProposalUpsertOwner(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:addresses, msg:only a maximum of 10 addresses are allowed"))
		})

		It("should return error when target address is not valid", func() {
			tx.RepoName = "repo1"
			tx.Addresses = []string{"invalid_addr"}
			err := validators.CheckTxRepoProposalUpsertOwner(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:addresses[0], msg:address is not valid"))
		})
	})

	Describe(".CheckTxVote", func() {
		var tx *core.TxRepoProposalVote

		BeforeEach(func() {
			tx = core.NewBareRepoProposalVote()
			tx.Timestamp = time.Now().Unix()
		})

		It("should return error when repo name is not provided", func() {
			err := validators.CheckTxVote(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:name, msg:repo name is required"))
		})

		It("should return error when repo name is not valid", func() {
			tx.RepoName = "*&^"
			err := validators.CheckTxVote(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:name, msg:invalid characters in name. Only alphanumeric, _ and - characters are allowed"))
		})

		It("should return error when proposal id is not provided", func() {
			tx.RepoName = "repo1"
			err := validators.CheckTxVote(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:id, msg:proposal id is required"))
		})

		It("should return error when proposal id is not numerical", func() {
			tx.RepoName = "repo1"
			tx.ProposalID = "abc"
			err := validators.CheckTxVote(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:id, msg:proposal id is not valid"))
		})

		It("should return error when vote choice is not between -2 and 1 (inclusive)", func() {
			tx.RepoName = "repo1"
			tx.ProposalID = "1"
			tx.Vote = 2
			err := validators.CheckTxVote(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:vote, msg:vote choice is unknown"))

			tx.Vote = -3
			err = validators.CheckTxVote(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:vote, msg:vote choice is unknown"))

			tx.Vote = -1
			err = validators.CheckTxVote(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).ToNot(MatchError("field:vote, msg:vote choice is unknown"))
		})
	})

	Describe(".CheckTxRepoProposalSendFee", func() {
		var tx *core.TxRepoProposalSendFee

		BeforeEach(func() {
			tx = core.NewBareRepoProposalFeeSend()
			tx.Timestamp = time.Now().Unix()
		})

		It("should return error when repo name is not provided", func() {
			err := validators.CheckTxRepoProposalSendFee(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:name, msg:repo name is required"))
		})

		It("should return error when repo name is not valid", func() {
			tx.RepoName = "*&^"
			err := validators.CheckTxRepoProposalSendFee(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:name, msg:invalid characters in name. Only alphanumeric, _ and - characters are allowed"))
		})

		It("should return error when proposal id is not provided", func() {
			tx.RepoName = "repo1"
			err := validators.CheckTxRepoProposalSendFee(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:id, msg:proposal id is required"))
		})

		It("should return error when proposal id is not numerical", func() {
			tx.RepoName = "repo1"
			tx.ProposalID = "abc"
			err := validators.CheckTxRepoProposalSendFee(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:id, msg:proposal id is not valid"))
		})

		It("should return error when proposal id exceeds max length", func() {
			tx.RepoName = "repo1"
			tx.ProposalID = "1234556789"
			err := validators.CheckTxRepoProposalSendFee(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:id, msg:proposal id limit of 8 bytes exceeded"))
		})

		It("should return error when value is not provided", func() {
			tx.RepoName = "good-repo"
			tx.Value = ""
			tx.ProposalID = "1"
			err := validators.CheckTxRepoProposalSendFee(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:value, msg:value is required"))
		})
	})

	Describe(".CheckTxRepoProposalUpdate", func() {
		var tx *core.TxRepoProposalUpdate

		BeforeEach(func() {
			tx = core.NewBareRepoProposalUpdate()
			tx.Timestamp = time.Now().Unix()
			tx.ProposalID = "123"
		})

		It("should return error when repo name is not provided", func() {
			err := validators.CheckTxRepoProposalUpdate(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:name, msg:repo name is required"))
		})

		It("should return error when repo name is not valid", func() {
			tx.RepoName = "*&^"
			err := validators.CheckTxRepoProposalUpdate(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:name, msg:invalid characters in name. Only alphanumeric, _ and - characters are allowed"))
		})

		It("should return error when proposal id is unset", func() {
			tx.RepoName = "good-repo"
			tx.ProposalID = ""
			err := validators.CheckTxRepoProposalUpdate(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:id, msg:proposal id is required"))
		})

		It("should return error when proposal id is not valid", func() {
			tx.RepoName = "good-repo"
			tx.ProposalID = "abc123"
			err := validators.CheckTxRepoProposalUpdate(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:id, msg:proposal id is not valid"))
		})

		It("should return error when proposal id length exceeds max", func() {
			tx.RepoName = "good-repo"
			tx.ProposalID = "123456789"
			err := validators.CheckTxRepoProposalUpdate(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:id, msg:proposal id limit of 8 bytes exceeded"))
		})

		It("should return error when value is not provided", func() {
			tx.RepoName = "good-repo"
			tx.Value = ""
			err := validators.CheckTxRepoProposalUpdate(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:value, msg:value is required"))
		})

		It("should return error when value below minimum network proposal fee", func() {
			params.MinProposalFee = 100
			tx.RepoName = "good-repo"
			tx.Value = "1"
			err := validators.CheckTxRepoProposalUpdate(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:value, msg:proposal creation fee cannot be less than network minimum"))
		})
	})

	Describe(".CheckTxRepoProposalMergeRequest", func() {
		var tx *core.TxRepoProposalMergeRequest

		BeforeEach(func() {
			tx = core.NewBareRepoProposalMergeRequest()
			tx.Timestamp = time.Now().Unix()
			tx.ProposalID = "123"
		})

		It("should return error='type is invalid'", func() {
			tx.Type = -10
			err := validators.CheckTxRepoProposalMergeRequest(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("field:type, msg:type is invalid"))
		})

		It("should return error when repo name is not provided", func() {
			err := validators.CheckTxRepoProposalMergeRequest(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:name, msg:repo name is required"))
		})

		It("should return error when repo name is not valid", func() {
			tx.RepoName = "*&^"
			err := validators.CheckTxRepoProposalMergeRequest(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:name, msg:invalid characters in name. " +
				"Only alphanumeric, _ and - characters are allowed"))
		})

		It("should return error when proposal id is unset", func() {
			tx.RepoName = "good-repo"
			tx.ProposalID = ""
			err := validators.CheckTxRepoProposalMergeRequest(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:id, msg:proposal id is required"))
		})

		It("should return error when proposal id is not valid", func() {
			tx.RepoName = "good-repo"
			tx.ProposalID = "abc123"
			err := validators.CheckTxRepoProposalMergeRequest(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:id, msg:proposal id is not valid"))
		})

		It("should return error when proposal id length exceeds max", func() {
			tx.RepoName = "good-repo"
			tx.ProposalID = "123456789"
			err := validators.CheckTxRepoProposalMergeRequest(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:id, msg:proposal id limit of 8 bytes exceeded"))
		})

		It("should return error when value is not provided", func() {
			tx.RepoName = "repo1"
			tx.Value = ""
			err := validators.CheckTxRepoProposalMergeRequest(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:value, msg:value is required"))
		})

		It("should return error when base branch is not provided", func() {
			tx.RepoName = "repo1"
			err := validators.CheckTxRepoProposalMergeRequest(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:base, msg:base branch name is required"))
		})

		It("should return error when base branch hash is not valid", func() {
			tx.RepoName = "repo1"
			tx.BaseBranch = "branch_base"
			tx.BaseBranchHash = "invalid"
			err := validators.CheckTxRepoProposalMergeRequest(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:baseHash, msg:base branch hash is not valid"))
		})

		It("should return error when target branch is not provided", func() {
			tx.RepoName = "repo1"
			tx.BaseBranch = "branch_base"
			tx.BaseBranchHash = util.RandString(40)
			err := validators.CheckTxRepoProposalMergeRequest(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:target, msg:target branch name is required"))
		})

		It("should return error when target branch hash is not provided", func() {
			tx.RepoName = "repo1"
			tx.BaseBranch = "branch_base"
			tx.BaseBranchHash = util.RandString(40)
			tx.TargetBranch = "branch_base"
			err := validators.CheckTxRepoProposalMergeRequest(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:targetHash, msg:target branch hash is required"))
		})

		It("should return error when target branch hash is not valid", func() {
			tx.RepoName = "repo1"
			tx.BaseBranch = "branch_base"
			tx.BaseBranchHash = util.RandString(40)
			tx.TargetBranch = "branch_base"
			tx.TargetBranchHash = "invalid"
			err := validators.CheckTxRepoProposalMergeRequest(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:targetHash, msg:target branch hash is not valid"))
		})
	})

	Describe(".CheckTxRepoProposalRegisterGPGKey", func() {
		var tx *core.TxRepoProposalRegisterGPGKey

		BeforeEach(func() {
			tx = core.NewBareRepoProposalRegisterGPGKey()
			tx.Timestamp = time.Now().Unix()
			tx.ProposalID = "123"
		})

		It("should return error='type is invalid'", func() {
			tx.Type = -10
			err := validators.CheckTxRepoProposalRegisterGPGKey(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("field:type, msg:type is invalid"))
		})

		It("should return error when repo name is not provided", func() {
			err := validators.CheckTxRepoProposalRegisterGPGKey(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:name, msg:repo name is required"))
		})

		It("should return error when repo name is not valid", func() {
			tx.RepoName = "*&^"
			err := validators.CheckTxRepoProposalRegisterGPGKey(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:name, msg:invalid characters in name. Only alphanumeric, _ and - characters are allowed"))
		})

		It("should return error when proposal id is unset", func() {
			tx.RepoName = "good-repo"
			tx.ProposalID = ""
			err := validators.CheckTxRepoProposalRegisterGPGKey(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:id, msg:proposal id is required"))
		})

		It("should return error when proposal id is not valid", func() {
			tx.RepoName = "good-repo"
			tx.ProposalID = "abc123"
			err := validators.CheckTxRepoProposalRegisterGPGKey(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:id, msg:proposal id is not valid"))
		})

		It("should return error when proposal id length exceeds max", func() {
			tx.RepoName = "good-repo"
			tx.ProposalID = "123456789"
			err := validators.CheckTxRepoProposalRegisterGPGKey(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:id, msg:proposal id limit of 8 bytes exceeded"))
		})

		It("should return error when value is not provided", func() {
			tx.RepoName = "good-repo"
			tx.Value = ""
			err := validators.CheckTxRepoProposalRegisterGPGKey(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:value, msg:value is required"))
		})

		It("should return error when value below minimum network proposal fee", func() {
			params.MinProposalFee = 100
			tx.RepoName = "good-repo"
			tx.Value = "1"
			err := validators.CheckTxRepoProposalRegisterGPGKey(tx, -1)
			Expect(err).ToNot(BeNil())
		})

		It("should return error when a gpg id is not valid", func() {
			tx.RepoName = "good-repo"
			tx.Value = "1"
			tx.KeyIDs = append(tx.KeyIDs, "gpg1_abc")
			err := validators.CheckTxRepoProposalRegisterGPGKey(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:ids, msg:GPG id (gpg1_abc) is not valid"))
		})

		It("should return error when a gpg id is a duplicate", func() {
			tx.RepoName = "good-repo"
			tx.Value = "1"
			tx.KeyIDs = append(tx.KeyIDs, "gpg1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd")
			tx.KeyIDs = append(tx.KeyIDs, "gpg1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd")
			err := validators.CheckTxRepoProposalRegisterGPGKey(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:ids, msg:GPG id " +
				"(gpg1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd) is a duplicate"))
		})

		It("should return error when fee mode is unknown", func() {
			tx.RepoName = "good-repo"
			tx.Value = "1"
			tx.KeyIDs = append(tx.KeyIDs, "gpg1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd")
			tx.FeeMode = 100
			err := validators.CheckTxRepoProposalRegisterGPGKey(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:feeMode, msg:fee mode is unknown"))
		})

		It("should return error when fee mode is FeeModeRepoCapped but fee cap is unset", func() {
			tx.RepoName = "good-repo"
			tx.Value = "1"
			tx.KeyIDs = append(tx.KeyIDs, "gpg1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd")
			tx.FeeMode = state.FeeModeRepoPaysCapped
			tx.FeeCap = ""
			err := validators.CheckTxRepoProposalRegisterGPGKey(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:feeCap, msg:value is required"))
		})

		It("should return error when fee mode is FeeModeRepoCapped but fee cap is not numeric", func() {
			tx.RepoName = "good-repo"
			tx.Value = "1"
			tx.KeyIDs = append(tx.KeyIDs, "gpg1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd")
			tx.FeeMode = state.FeeModeRepoPaysCapped
			tx.FeeCap = "ten"
			err := validators.CheckTxRepoProposalRegisterGPGKey(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:feeCap, msg:invalid number; must be numeric"))
		})

		It("should return error when fee mode is FeeModeRepoCapped but fee cap is not a positive number", func() {
			tx.RepoName = "good-repo"
			tx.Value = "1"
			tx.KeyIDs = append(tx.KeyIDs, "gpg1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd")
			tx.FeeMode = state.FeeModeRepoPaysCapped
			tx.FeeCap = "-1"
			err := validators.CheckTxRepoProposalRegisterGPGKey(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:feeCap, msg:negative figure not allowed"))
		})

		It("should return error when fee mode is not FeeModeRepoCapped but fee cap is set", func() {
			tx.RepoName = "good-repo"
			tx.Value = "1"
			tx.KeyIDs = append(tx.KeyIDs, "gpg1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd")
			tx.FeeMode = state.FeeModeRepoPays
			tx.FeeCap = "1"
			err := validators.CheckTxRepoProposalRegisterGPGKey(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:feeCap, msg:value not expected for the chosen fee mode"))
		})

		It("should return error when namespace value format is invalid", func() {
			tx.RepoName = "good-repo"
			tx.Value = "10"
			tx.KeyIDs = append(tx.KeyIDs, "gpg1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd")
			tx.FeeMode = state.FeeModeRepoPays
			tx.Namespace = "inv&alid"
			err := validators.CheckTxRepoProposalRegisterGPGKey(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:namespace, msg:value format is not valid"))
		})

		It("should return error when namespace is set but namespaceOnly is also set", func() {
			tx.RepoName = "good-repo"
			tx.Value = "10"
			tx.KeyIDs = append(tx.KeyIDs, "gpg1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd")
			tx.FeeMode = state.FeeModeRepoPays
			tx.Namespace = "ns1"
			tx.NamespaceOnly = "ns2"
			err := validators.CheckTxRepoProposalRegisterGPGKey(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:namespaceOnly, msg:field is not expected because 'namespace' is set"))
		})

		It("should return error when namespaceOnly value format is invalid", func() {
			tx.RepoName = "good-repo"
			tx.Value = "10"
			tx.KeyIDs = append(tx.KeyIDs, "gpg1ntkem0drvtr4a8l25peyr2kzql277nsqpczpfd")
			tx.FeeMode = state.FeeModeRepoPays
			tx.NamespaceOnly = "inv&alid"
			err := validators.CheckTxRepoProposalRegisterGPGKey(tx, -1)
			Expect(err).ToNot(BeNil())
			Expect(err).To(MatchError("field:namespaceOnly, msg:value format is not valid"))
		})
	})
})
